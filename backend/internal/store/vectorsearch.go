package store

import (
	"context"
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lifelog/backend/internal/core"
)

// VectorIndexName is the name of the Atlas Vector Search index on the
// collection "chunks".
const VectorIndexName = "chunks_vector"

// vectorIndexDefinition: "embedding" as vector (768 dimensions, cosine),
// "userId" and "module" as filter fields, so the search only looks at the
// user's own chunks of the chosen module. The same file is pasted when
// creating the index in the Atlas UI (backend/README.md, section vector
// index).
//
//go:embed chunks_vector.json
var vectorIndexDefinition []byte

// numCandidates: Atlas considers this many candidates before it picks the
// best MaxHits. More candidates = more accurate but slower; 100 is plenty
// for a few thousand chunks.
const numCandidates = 100

// Results of CreateVectorIndex besides the Atlas status.
const (
	VectorIndexCreated = "CREATED"
	VectorIndexUpdated = "UPDATED"
	VectorIndexReady   = "READY"
)

// indexField is one field of a vector index definition.
type indexField struct {
	Type string `bson:"type"`
	Path string `bson:"path"`
}

type indexDefinition struct {
	Fields []indexField `bson:"fields"`
}

// searchIndexDoc is one entry of listSearchIndexes.
type searchIndexDoc struct {
	Status           string          `bson:"status"`
	LatestDefinition indexDefinition `bson:"latestDefinition"`
}

// CreateVectorIndex makes sure the vector index exists with the current
// definition and returns its status:
//
//   - missing: create it, "CREATED";
//   - definition without a filter field we need (e.g. "userId" in an index
//     from before 2026-11-18): update it, "UPDATED";
//   - otherwise the Atlas status, e.g. "PENDING", "BUILDING", "READY".
//
// Atlas builds a new or updated index in the background. Until it is
// "READY", questions do not work (POST /questions answers 503). If creating
// or updating fails (e.g. missing permissions), the function returns the
// error; the server still starts, and the index can be changed in the
// Atlas UI.
//
// Precondition: the collection "chunks" exists. CreateIndexes creates it
// with its indexes, so call that first.
func (s *Store) CreateVectorIndex(ctx context.Context) (status string, err error) {
	var want bson.D
	if err := bson.UnmarshalExtJSON(vectorIndexDefinition, false, &want); err != nil {
		return "", fmt.Errorf("vector index definition not readable: %w", err)
	}
	wantFields, err := definitionFields(vectorIndexDefinition)
	if err != nil {
		return "", err
	}

	current, found, err := s.vectorIndex(ctx)
	if err != nil {
		return "", err
	}
	if !found {
		_, err = s.chunks.SearchIndexes().CreateOne(ctx, mongo.SearchIndexModel{
			Definition: want,
			Options:    options.SearchIndexes().SetName(VectorIndexName).SetType("vectorSearch"),
		})
		if err != nil {
			return "", fmt.Errorf("create vector index: %w", err)
		}
		return VectorIndexCreated, nil
	}
	if needsUpdate(current.LatestDefinition.Fields, wantFields) {
		if err := s.chunks.SearchIndexes().UpdateOne(ctx, VectorIndexName, want); err != nil {
			return "", fmt.Errorf("update vector index: %w", err)
		}
		return VectorIndexUpdated, nil
	}
	return current.Status, nil
}

// VectorIndexStatus returns the status of the vector index, if it exists.
func (s *Store) VectorIndexStatus(ctx context.Context) (status string, found bool, err error) {
	idx, found, err := s.vectorIndex(ctx)
	return idx.Status, found, err
}

func (s *Store) vectorIndex(ctx context.Context) (searchIndexDoc, bool, error) {
	cur, err := s.chunks.SearchIndexes().List(ctx, options.SearchIndexes().SetName(VectorIndexName))
	if err != nil {
		return searchIndexDoc{}, false, fmt.Errorf("check vector index: %w", err)
	}
	var indexes []searchIndexDoc
	if err := cur.All(ctx, &indexes); err != nil {
		return searchIndexDoc{}, false, fmt.Errorf("check vector index: %w", err)
	}
	if len(indexes) == 0 {
		return searchIndexDoc{}, false, nil
	}
	return indexes[0], true, nil
}

func definitionFields(def []byte) ([]indexField, error) {
	var d indexDefinition
	if err := bson.UnmarshalExtJSON(def, false, &d); err != nil {
		return nil, fmt.Errorf("vector index definition not readable: %w", err)
	}
	return d.Fields, nil
}

// needsUpdate: the index lacks a field of the wanted definition, e.g. the
// filter field "userId". Only missing fields count: an index that already
// has everything is left alone, so a restart never triggers a rebuild.
func needsUpdate(current, want []indexField) bool {
	for _, w := range want {
		if !slices.Contains(current, w) {
			return true
		}
	}
	return false
}

// hitDoc is a result of $vectorSearch.
type hitDoc struct {
	EntryID  bson.ObjectID `bson:"entryId"`
	Position int           `bson:"position"`
	Text     string        `bson:"text"`
	Score    float64       `bson:"score"`
}

// vectorSearchPipeline builds the aggregation for SimilarChunks. The owner
// and module filter is part of the $vectorSearch stage itself, not a later
// $match: chunks of other users are never candidates, and the k results are
// all the user's own.
func vectorSearchPipeline(owner bson.ObjectID, vector []float32, module string, k int) mongo.Pipeline {
	return mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: VectorIndexName},
			{Key: "path", Value: "embedding"},
			{Key: "queryVector", Value: vector},
			{Key: "numCandidates", Value: numCandidates},
			{Key: "limit", Value: k},
			// Only possible because "userId" and "module" are registered as
			// filter fields in the index (chunks_vector.json).
			{Key: "filter", Value: bson.D{{Key: "$and", Value: bson.A{
				bson.D{{Key: "userId", Value: bson.D{{Key: "$eq", Value: owner}}}},
				bson.D{{Key: "module", Value: bson.D{{Key: "$eq", Value: module}}}},
			}}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "entryId", Value: 1},
			{Key: "position", Value: 1},
			{Key: "text", Value: 1},
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "vectorSearchScore"}}},
		}}},
	}
}

// isFilterNotIndexed recognizes the Atlas error for a filter on a field the
// index does not (yet) know, e.g. "Path 'userId' needs to be indexed as
// filter". That happens while Atlas rebuilds the index after an update and
// still serves the old definition.
func isFilterNotIndexed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "needs to be indexed as filter") ||
		strings.Contains(msg, "needs to be indexed as a filter")
}

// SimilarChunks finds the k chunks of the user in the module whose
// embedding is most similar to the vector, by descending score.
//
// The score is Atlas' vectorSearchScore, with cosine (1 + cos) / 2. It is
// not filtered by threshold here but in the service (core.MinScore).
//
// While the index is rebuilt with the userId filter, the error wraps
// core.ErrSearchUpdating (503, never an unfiltered search).
func (s *Store) SimilarChunks(ctx context.Context, userID string, vector []float32, module string, k int) ([]core.Hit, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, err
	}
	cur, err := s.chunks.Aggregate(ctx, vectorSearchPipeline(uid, vector, module, k))
	if err != nil {
		if isFilterNotIndexed(err) {
			return nil, fmt.Errorf("vector search: %w", core.ErrSearchUpdating)
		}
		return nil, fmt.Errorf("vector search: %w", err)
	}
	var docs []hitDoc
	if err := cur.All(ctx, &docs); err != nil {
		if isFilterNotIndexed(err) {
			return nil, fmt.Errorf("vector search: %w", core.ErrSearchUpdating)
		}
		return nil, fmt.Errorf("vector search: %w", err)
	}
	hits := make([]core.Hit, 0, len(docs))
	for _, d := range docs {
		hits = append(hits, core.Hit{EntryID: d.EntryID.Hex(), Position: d.Position, Text: d.Text, Score: d.Score})
	}
	return hits, nil
}

// SummariesByIDs loads the short form of several entries of the user at
// once. Entries that do not exist (anymore) or belong to someone else are
// simply missing in the map; the service then drops the matching chunks.
// This is the second filter after the vector search.
func (s *Store) SummariesByIDs(ctx context.Context, userID string, ids []string) (map[string]core.EntrySummary, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, err
	}
	oids := make([]bson.ObjectID, 0, len(ids))
	for _, id := range ids {
		oid, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return nil, core.ErrInvalidID
		}
		oids = append(oids, oid)
	}
	result := map[string]core.EntrySummary{}
	if len(oids) == 0 {
		return result, nil
	}
	cur, err := s.entries.Find(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: oids}}}, {Key: "userId", Value: uid}},
		options.Find().SetProjection(summaryProjection))
	if err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}
	var docs []entrySummaryDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}
	for _, d := range docs {
		result[d.ID.Hex()] = d.toCore()
	}
	return result, nil
}
