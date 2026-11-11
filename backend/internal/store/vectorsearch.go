package store

import (
	"context"
	_ "embed"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lifelog/backend/internal/core"
)

// VectorIndexName is the name of the Atlas Vector Search index on the
// collection "abschnitte".
const VectorIndexName = "abschnitte_vektor"

// vectorIndexDefinition: "embedding" as vector (768 dimensions, cosine),
// "modul" as filter field, so the search only looks in the chosen module.
// The same file is pasted when creating the index in the Atlas UI
// (backend/README.md, "Vektorindex").
//
//go:embed chunks_vector.json
var vectorIndexDefinition []byte

// numCandidates: Atlas considers this many candidates before it picks the
// best MaxHits. More candidates = more accurate but slower; 100 is plenty
// for a few thousand chunks.
const numCandidates = 100

// CreateVectorIndex creates the vector index if it does not exist yet, and
// returns its status (e.g. "PENDING", "BUILDING", "READY", or "CREATED" right
// after creating it).
//
// Atlas builds the index in the background; until it is "READY", search
// finds nothing. If creating fails (e.g. missing permissions), the function
// returns the error; the server still starts, and the index can be created
// in the Atlas UI.
//
// Precondition: the collection "abschnitte" exists. CreateIndexes creates it
// with the index on eintragId, so call that first.
func (s *Store) CreateVectorIndex(ctx context.Context) (status string, err error) {
	status, found, err := s.VectorIndexStatus(ctx)
	if err != nil || found {
		return status, err
	}

	var definition bson.D
	if err := bson.UnmarshalExtJSON(vectorIndexDefinition, false, &definition); err != nil {
		return "", fmt.Errorf("vector index definition not readable: %w", err)
	}
	_, err = s.chunks.SearchIndexes().CreateOne(ctx, mongo.SearchIndexModel{
		Definition: definition,
		Options:    options.SearchIndexes().SetName(VectorIndexName).SetType("vectorSearch"),
	})
	if err != nil {
		return "", fmt.Errorf("create vector index: %w", err)
	}
	return "CREATED", nil
}

// VectorIndexStatus returns the status of the vector index, if it exists.
func (s *Store) VectorIndexStatus(ctx context.Context) (status string, found bool, err error) {
	cur, err := s.chunks.SearchIndexes().List(ctx, options.SearchIndexes().SetName(VectorIndexName))
	if err != nil {
		return "", false, fmt.Errorf("check vector index: %w", err)
	}
	var indexes []struct {
		Status string `bson:"status"`
	}
	if err := cur.All(ctx, &indexes); err != nil {
		return "", false, fmt.Errorf("check vector index: %w", err)
	}
	if len(indexes) == 0 {
		return "", false, nil
	}
	return indexes[0].Status, true, nil
}

// hitDoc is a result of $vectorSearch.
type hitDoc struct {
	EntryID  bson.ObjectID `bson:"eintragId"`
	Position int           `bson:"nr"`
	Text     string        `bson:"text"`
	Score    float64       `bson:"score"`
}

// SimilarChunks finds the k chunks of the module whose embedding is most
// similar to the vector, by descending score.
//
// The score is Atlas' vectorSearchScore, with cosine (1 + cos) / 2. It is
// not filtered by threshold here but in the service (core.MinScore).
func (s *Store) SimilarChunks(ctx context.Context, vector []float32, module string, k int) ([]core.Hit, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: VectorIndexName},
			{Key: "path", Value: "embedding"},
			{Key: "queryVector", Value: vector},
			{Key: "numCandidates", Value: numCandidates},
			{Key: "limit", Value: k},
			// Only possible because "modul" is registered as a filter field in the index.
			{Key: "filter", Value: bson.D{{Key: "modul", Value: bson.D{{Key: "$eq", Value: module}}}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "eintragId", Value: 1},
			{Key: "nr", Value: 1},
			{Key: "text", Value: 1},
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "vectorSearchScore"}}},
		}}},
	}
	cur, err := s.chunks.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	var docs []hitDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	hits := make([]core.Hit, 0, len(docs))
	for _, d := range docs {
		hits = append(hits, core.Hit{EntryID: d.EntryID.Hex(), Position: d.Position, Text: d.Text, Score: d.Score})
	}
	return hits, nil
}

// SummariesByIDs loads the short form of several entries at once. Entries
// that do not exist (anymore) are simply missing in the map; the service
// then drops the matching chunks.
func (s *Store) SummariesByIDs(ctx context.Context, ids []string) (map[string]core.EntrySummary, error) {
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
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: oids}}}},
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
