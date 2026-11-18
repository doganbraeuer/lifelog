// Package store encapsulates access to MongoDB. Other packages only see core
// types, no BSON documents.
//
// Per-user data (docs/API.md, section 1): every function that reads,
// changes or deletes entries, chunks, OCR words or original files takes the
// user ID as a required parameter and always filters by it. An empty or
// invalid user ID is a programming error (core.ErrNoUser); no function ever
// falls back to "all users".
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

// legacyHashIndex was unique across all users. It is replaced by
// user_file_sha256_unique and dropped on start: otherwise a second user
// could not upload the same file, and the 409 would reveal that someone
// else has it.
const legacyHashIndex = "file_sha256_unique"

// Store is the connection to a Lifelog database.
type Store struct {
	client  *mongo.Client
	db      *mongo.Database
	entries *mongo.Collection
	chunks  *mongo.Collection
	ocr     *mongo.Collection
	files   *mongo.GridFSBucket
	// fileDocs is originals.files: GridFS keeps the owner in metadata.userId.
	fileDocs *mongo.Collection

	// Accounts and login (accounts.go).
	users    *mongo.Collection
	sessions *mongo.Collection
	auditLog *mongo.Collection
	// Daily limits (usage.go).
	usage *mongo.Collection
}

// NewID creates a new ObjectId as hex string. The upload assigns the IDs in
// advance, because the file and the chunks refer to the entry, which is
// stored last.
func NewID() string {
	return bson.NewObjectID().Hex()
}

// ownerID turns the user ID into the ObjectId that every filter on entries,
// chunks, OCR words and originals contains. Empty or invalid is a
// programming error: core.ErrNoUser, never "all".
func ownerID(userID string) (bson.ObjectID, error) {
	if userID == "" {
		return bson.ObjectID{}, core.ErrNoUser
	}
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil || oid.IsZero() {
		return bson.ObjectID{}, core.ErrNoUser
	}
	return oid, nil
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
	files := db.GridFSBucket(options.GridFSBucket().SetName(bucketFiles))
	s := &Store{
		client:   client,
		db:       db,
		entries:  db.Collection(collEntries),
		chunks:   db.Collection(collChunks),
		ocr:      db.Collection(collOCR),
		files:    files,
		fileDocs: files.GetFilesCollection(),

		users:    db.Collection(collUsers),
		sessions: db.Collection(collSessions),
		auditLog: db.Collection(collAuditLog),
		usage:    db.Collection(collUsage),
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
	// First drop the old unique index across all users; with it, the new
	// per-user index would not help.
	if err := dropIndexIfExists(ctx, s.entries, legacyHashIndex); err != nil {
		return err
	}
	_, err := s.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Duplicate detection per user (API.md 3.1): the unique index also
			// catches two simultaneous uploads of the same file by one user.
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "file.sha256", Value: 1}},
			Options: options.Index().SetName("user_file_sha256_unique").SetUnique(true),
		},
		{
			// The timeline filters by user and module.
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "module", Value: 1}},
			Options: options.Index().SetName("user_module"),
		},
		{
			Keys:    bson.D{{Key: "module", Value: 1}},
			Options: options.Index().SetName("module"),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	// Deleting an entry removes its chunks via userId and entryId. Without
	// an index, MongoDB would have to scan all chunks for that.
	_, err = s.chunks.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "entryId", Value: 1}},
			Options: options.Index().SetName("entryId"),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "entryId", Value: 1}},
			Options: options.Index().SetName("user_entry"),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	// OCR words are found by _id (= entry ID) plus userId; this index is for
	// deleting and counting all data of one user.
	if _, err := s.ocr.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetName("userId"),
	}); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	if _, err := s.fileDocs.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "metadata.userId", Value: 1}},
		Options: options.Index().SetName("metadata_userId"),
	}); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	// users, sessions (accounts.go) and usage (usage.go).
	if err := s.createAccountIndexes(ctx); err != nil {
		return err
	}
	return s.createUsageIndexes(ctx)
}

// dropIndexIfExists drops the index if the collection has it. A missing
// collection or index is not an error.
func dropIndexIfExists(ctx context.Context, coll *mongo.Collection, name string) error {
	specs, err := coll.Indexes().ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("list indexes: %w", err)
	}
	for _, spec := range specs {
		if spec.Name == name {
			if err := coll.Indexes().DropOne(ctx, name); err != nil {
				return fmt.Errorf("drop index %s: %w", name, err)
			}
			return nil
		}
	}
	return nil
}

// ListSummaries returns all entries of the user in a module in short form,
// unsorted. Sorting happens in core.SortEntries, because the rules (null at
// the end etc.) are tested there.
func (s *Store) ListSummaries(ctx context.Context, userID, module string) ([]core.EntrySummary, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, err
	}
	cur, err := s.entries.Find(ctx,
		bson.D{{Key: "userId", Value: uid}, {Key: "module", Value: module}},
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

// CountEntries returns how many entries the user has (cmd/users list).
func (s *Store) CountEntries(ctx context.Context, userID string) (int64, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return 0, err
	}
	n, err := s.entries.CountDocuments(ctx, bson.D{{Key: "userId", Value: uid}})
	if err != nil {
		return 0, fmt.Errorf("count entries: %w", err)
	}
	return n, nil
}

// SaveEntry inserts a new entry of the user. ID and File.ID must be valid
// ObjectIds. If e.UserID is set, it must be userID.
//
// If the user already has an entry with the same file sha256, the unique
// index reports an error and SaveEntry returns core.ErrDuplicate. This
// catches the case where two equal uploads pass the duplicate check at the
// same time.
func (s *Store) SaveEntry(ctx context.Context, userID string, e core.Entry) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	if e.UserID != "" && e.UserID != userID {
		return fmt.Errorf("entry %s: owner %s does not match %s", e.ID, e.UserID, userID)
	}
	doc, err := toEntryDoc(e)
	if err != nil {
		return err
	}
	doc.UserID = uid
	if _, err := s.entries.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return core.ErrDuplicate
		}
		return fmt.Errorf("save entry: %w", err)
	}
	return nil
}

// GetEntry returns the full entry if it belongs to the user. found is false
// if there is no such entry, also if it belongs to someone else: the caller
// cannot tell the two apart, and neither can the app (404).
func (s *Store) GetEntry(ctx context.Context, userID, id string) (e core.Entry, found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return core.Entry{}, false, err
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return core.Entry{}, false, core.ErrInvalidID
	}
	var doc entryDoc
	err = s.entries.FindOne(ctx, bson.D{{Key: "_id", Value: oid}, {Key: "userId", Value: uid}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return core.Entry{}, false, nil
	}
	if err != nil {
		return core.Entry{}, false, fmt.Errorf("read entry: %w", err)
	}
	return doc.toCore(), true, nil
}

// OpenFile opens the user's original file for reading. The data comes from
// GridFS in pieces while the caller reads; the file is never loaded into
// memory as a whole. The caller must call Close.
//
// The file must carry the user in metadata.userId; the caller has already
// checked that the entry belongs to the user, this is the second check.
//
// ctx applies to the whole reading, not only to opening.
// If the file is missing or belongs to someone else, the error is
// core.ErrOriginalMissing.
func (s *Store) OpenFile(ctx context.Context, userID, fileID string) (content io.ReadCloser, length int64, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, 0, err
	}
	oid, err := bson.ObjectIDFromHex(fileID)
	if err != nil {
		return nil, 0, core.ErrInvalidID
	}
	owned, err := s.ownsFile(ctx, uid, oid)
	if err != nil {
		return nil, 0, err
	}
	if !owned {
		return nil, 0, core.ErrOriginalMissing
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

// ownsFile: does originals.files have this file with this owner?
func (s *Store) ownsFile(ctx context.Context, uid, fileID bson.ObjectID) (bool, error) {
	n, err := s.fileDocs.CountDocuments(ctx,
		bson.D{{Key: "_id", Value: fileID}, {Key: "metadata.userId", Value: uid}},
		options.Count().SetLimit(1))
	if err != nil {
		return false, fmt.Errorf("check file owner: %w", err)
	}
	return n > 0, nil
}

// EntryByHash looks for the user's entry whose file has this SHA-256 hash.
// The search applies across all modules of the user, never across users
// (API.md 3.1).
func (s *Store) EntryByHash(ctx context.Context, userID, sha256 string) (id string, found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return "", false, err
	}
	var hit struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err = s.entries.FindOne(ctx,
		bson.D{{Key: "userId", Value: uid}, {Key: "file.sha256", Value: sha256}},
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

// SaveFile stores the user's original file in GridFS, with the owner in
// metadata.userId. GridFS splits files into pieces, because a single
// MongoDB document may be at most 16 MB. Our files are ≤ 10 MB, but this
// way the entry document stays small and the file is only loaded when
// someone really looks at it.
func (s *Store) SaveFile(ctx context.Context, userID, id, fileName, mimeType string, data []byte) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid file ID: %w", err)
	}
	err = s.files.UploadFromStreamWithID(ctx, oid, fileName, bytes.NewReader(data),
		options.GridFSUpload().SetMetadata(bson.D{
			{Key: "mimeType", Value: mimeType},
			{Key: "userId", Value: uid},
		}))
	if err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

// DeleteFile removes the user's file from GridFS. If it does not exist
// (anymore) or belongs to someone else, nothing happens and that is not an
// error: an interrupted delete can simply be repeated (API.md 3.7), and a
// foreign file is never touched.
func (s *Store) DeleteFile(ctx context.Context, userID, id string) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid file ID: %w", err)
	}
	owned, err := s.ownsFile(ctx, uid, oid)
	if err != nil || !owned {
		return err
	}
	if err := s.files.Delete(ctx, oid); err != nil && !errors.Is(err, mongo.ErrFileNotFound) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// SaveChunks inserts all chunks of an entry of the user, each with the
// owner, so the vector search can filter by it. An empty list is allowed
// and does nothing.
func (s *Store) SaveChunks(ctx context.Context, userID string, chunks []core.Chunk) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return nil
	}
	docs := make([]any, 0, len(chunks))
	for _, c := range chunks {
		doc, err := toChunkDoc(c)
		if err != nil {
			return err
		}
		doc.UserID = uid
		docs = append(docs, doc)
	}
	if _, err := s.chunks.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("save chunks: %w", err)
	}
	return nil
}

// DeleteChunks removes all chunks of the user's entry. If there are none,
// that is not an error.
func (s *Store) DeleteChunks(ctx context.Context, userID, entryID string) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	oid, err := bson.ObjectIDFromHex(entryID)
	if err != nil {
		return fmt.Errorf("invalid entry ID: %w", err)
	}
	if _, err := s.chunks.DeleteMany(ctx, bson.D{{Key: "userId", Value: uid}, {Key: "entryId", Value: oid}}); err != nil {
		return fmt.Errorf("delete chunks: %w", err)
	}
	return nil
}

// SaveCorrection writes the correctable fields of the user's entry back
// (API.md 3.5): type, document date, issuer, facts and corrections, plus the
// verification recomputed for them. Everything else (full text, file,
// import time, module, owner) is not touched, not even by accident, because
// only these six fields are in the $set.
//
// found is false if the entry does not exist (anymore) or is not the
// user's.
func (s *Store) SaveCorrection(ctx context.Context, userID string, e core.Entry) (found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	// The same conversion as when saving, so times (UTC), missing units
	// (omitempty) and empty lists look the same.
	doc, err := toEntryDoc(e)
	if err != nil {
		return false, err
	}
	res, err := s.entries.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: doc.ID}, {Key: "userId", Value: uid}},
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

// DeleteEntry deletes only the user's entry document. found is false if
// there was no such entry. The caller deletes chunks and file before, with
// DeleteChunks and DeleteFile (API.md 3.7).
func (s *Store) DeleteEntry(ctx context.Context, userID, id string) (found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, fmt.Errorf("invalid entry ID: %w", err)
	}
	res, err := s.entries.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}, {Key: "userId", Value: uid}})
	if err != nil {
		return false, fmt.Errorf("delete entry: %w", err)
	}
	return res.DeletedCount > 0, nil
}
