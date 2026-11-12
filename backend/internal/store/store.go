// Package store encapsulates access to MongoDB. Other packages only see core
// types, no BSON documents.
package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"lifelog/backend/internal/core"
)

// Names of the collections. For the bucket "originals", GridFS creates the
// collections originals.files (metadata) and originals.chunks (data in
// pieces of 255 KB) itself. The bucket is not called "files", so that
// "files.chunks" does not stand next to the collection "chunks".
const (
	collEntries = "entries"
	collChunks  = "chunks"
	bucketFiles = "originals"
)

// Store is the connection to a Lifelog database.
type Store struct {
	client  *mongo.Client
	db      *mongo.Database
	entries *mongo.Collection
	chunks  *mongo.Collection
	ocr     *mongo.Collection
	files   *mongo.GridFSBucket
}

// NewID creates a new ObjectId as hex string. The upload assigns the IDs in
// advance, because the file and the chunks refer to the entry, which is
// stored last.
func NewID() string {
	return bson.NewObjectID().Hex()
}

// Connect opens the connection and checks it with a ping.
//
// The error messages deliberately do not contain the driver's cause,
// because it can contain parts of the URI. The URI contains the password.
func Connect(ctx context.Context, uri, database string) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, errors.New("could not create MongoDB client (check MONGODB_URI)")
	}
	db := client.Database(database)
	s := &Store{
		client:  client,
		db:      db,
		entries: db.Collection(collEntries),
		chunks:  db.Collection(collChunks),
		ocr:     db.Collection(collOCR),
		files:   db.GridFSBucket(options.GridFSBucket().SetName(bucketFiles)),
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.Ping(pingCtx); err != nil {
		_ = s.Disconnect(context.Background())
		return nil, errors.New("MongoDB unreachable: check credentials and Network Access in Atlas")
	}
	return s, nil
}

// Disconnect closes the connection.
func (s *Store) Disconnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}

// Ping checks whether the database responds (for GET /status).
func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, readpref.Primary())
}

// CreateIndexes creates the regular indexes. If an index with the same
// definition already exists, nothing happens. It can be called on every
// start.
func (s *Store) CreateIndexes(ctx context.Context) error {
	_, err := s.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Duplicate detection (API.md 3.1): the unique index also catches
			// two simultaneous uploads of the same file.
			Keys:    bson.D{{Key: "file.sha256", Value: 1}},
			Options: options.Index().SetName("file_sha256_unique").SetUnique(true),
		},
		{
			// The timeline always filters by module.
			Keys:    bson.D{{Key: "module", Value: 1}},
			Options: options.Index().SetName("module"),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	// Deleting an entry removes its chunks via entryId. Without an index,
	// MongoDB would have to scan all chunks for that.
	_, err = s.chunks.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "entryId", Value: 1}},
		Options: options.Index().SetName("entryId"),
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

// ListSummaries returns all entries of a module in short form, unsorted.
// Sorting happens in core.SortEntries, because the rules (null at the end
// etc.) are tested there.
func (s *Store) ListSummaries(ctx context.Context, module string) ([]core.EntrySummary, error) {
	cur, err := s.entries.Find(ctx,
		bson.D{{Key: "module", Value: module}},
		options.Find().SetProjection(summaryProjection))
	if err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}
	var docs []entrySummaryDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}

	list := make([]core.EntrySummary, 0, len(docs))
	for _, d := range docs {
		list = append(list, d.toCore())
	}
	return list, nil
}

// SaveEntry inserts a new entry. ID and File.ID must be valid ObjectIds.
//
// If an entry with the same file sha256 already exists, the unique index
// reports an error and SaveEntry returns core.ErrDuplicate. This catches the
// case where two equal uploads pass the duplicate check at the same time.
func (s *Store) SaveEntry(ctx context.Context, e core.Entry) error {
	doc, err := toEntryDoc(e)
	if err != nil {
		return err
	}
	if _, err := s.entries.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return core.ErrDuplicate
		}
		return fmt.Errorf("save entry: %w", err)
	}
	return nil
}

// GetEntry returns the full entry. found is false if there is no entry with
// this ID.
func (s *Store) GetEntry(ctx context.Context, id string) (e core.Entry, found bool, err error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return core.Entry{}, false, core.ErrInvalidID
	}
	var doc entryDoc
	err = s.entries.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return core.Entry{}, false, nil
	}
	if err != nil {
		return core.Entry{}, false, fmt.Errorf("read entry: %w", err)
	}
	return doc.toCore(), true, nil
}

// OpenFile opens the original file for reading. The data comes from GridFS
// in pieces while the caller reads; the file is never loaded into memory as
// a whole. The caller must call Close.
//
// ctx applies to the whole reading, not only to opening.
// If the file is missing, the error is core.ErrOriginalMissing.
func (s *Store) OpenFile(ctx context.Context, fileID string) (content io.ReadCloser, length int64, err error) {
	oid, err := bson.ObjectIDFromHex(fileID)
	if err != nil {
		return nil, 0, core.ErrInvalidID
	}
	stream, err := s.files.OpenDownloadStream(ctx, oid)
	if errors.Is(err, mongo.ErrFileNotFound) {
		return nil, 0, core.ErrOriginalMissing
	}
	if err != nil {
		return nil, 0, fmt.Errorf("open file: %w", err)
	}
	return stream, stream.GetFile().Length, nil
}

// EntryByHash looks for the entry whose file has this SHA-256 hash. The
// search applies across all modules (API.md 3.1).
func (s *Store) EntryByHash(ctx context.Context, sha256 string) (id string, found bool, err error) {
	var hit struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err = s.entries.FindOne(ctx,
		bson.D{{Key: "file.sha256", Value: sha256}},
		options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}}),
	).Decode(&hit)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("check duplicate: %w", err)
	}
	return hit.ID.Hex(), true, nil
}

// SaveFile stores the original file in GridFS. GridFS splits files into
// pieces, because a single MongoDB document may be at most 16 MB. Our files
// are ≤ 10 MB, but this way the entry document stays small and the file is
// only loaded when someone really looks at it.
func (s *Store) SaveFile(ctx context.Context, id, fileName, mimeType string, data []byte) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid file ID: %w", err)
	}
	err = s.files.UploadFromStreamWithID(ctx, oid, fileName, bytes.NewReader(data),
		options.GridFSUpload().SetMetadata(bson.D{{Key: "mimeType", Value: mimeType}}))
	if err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

// DeleteFile removes the file from GridFS. If it does not exist (anymore),
// that is not an error: an interrupted delete can simply be repeated
// (API.md 3.7).
func (s *Store) DeleteFile(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid file ID: %w", err)
	}
	if err := s.files.Delete(ctx, oid); err != nil && !errors.Is(err, mongo.ErrFileNotFound) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// SaveChunks inserts all chunks of an entry. An empty list is allowed and
// does nothing.
func (s *Store) SaveChunks(ctx context.Context, chunks []core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	docs := make([]any, 0, len(chunks))
	for _, c := range chunks {
		doc, err := toChunkDoc(c)
		if err != nil {
			return err
		}
		docs = append(docs, doc)
	}
	if _, err := s.chunks.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("save chunks: %w", err)
	}
	return nil
}

// DeleteChunks removes all chunks of an entry. If there are none, that is
// not an error.
func (s *Store) DeleteChunks(ctx context.Context, entryID string) error {
	oid, err := bson.ObjectIDFromHex(entryID)
	if err != nil {
		return fmt.Errorf("invalid entry ID: %w", err)
	}
	if _, err := s.chunks.DeleteMany(ctx, bson.D{{Key: "entryId", Value: oid}}); err != nil {
		return fmt.Errorf("delete chunks: %w", err)
	}
	return nil
}

// SaveCorrection writes the correctable fields of an entry back (API.md
// 3.5): type, document date, issuer, facts and corrections, plus the
// verification recomputed for them. Everything else (full text, file,
// import time, module) is not touched, not even by accident, because only
// these six fields are in the $set.
//
// found is false if the entry does not exist (anymore).
func (s *Store) SaveCorrection(ctx context.Context, e core.Entry) (found bool, err error) {
	// The same conversion as when saving, so times (UTC), missing units
	// (omitempty) and empty lists look the same.
	doc, err := toEntryDoc(e)
	if err != nil {
		return false, err
	}
	res, err := s.entries.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: doc.ID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "type", Value: doc.Type},
			{Key: "documentDate", Value: doc.DocumentDate},
			{Key: "issuer", Value: doc.Issuer},
			{Key: "facts", Value: doc.Facts},
			{Key: "corrections", Value: doc.Corrections},
			{Key: "verification", Value: doc.Verification},
		}}},
	)
	if err != nil {
		return false, fmt.Errorf("save correction: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// DeleteEntry deletes only the entry document. found is false if there was
// no entry with this ID. The caller deletes chunks and file before, with
// DeleteChunks and DeleteFile (API.md 3.7).
func (s *Store) DeleteEntry(ctx context.Context, id string) (found bool, err error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, fmt.Errorf("invalid entry ID: %w", err)
	}
	res, err := s.entries.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return false, fmt.Errorf("delete entry: %w", err)
	}
	return res.DeletedCount > 0, nil
}
