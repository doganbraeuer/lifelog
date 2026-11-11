package store

// Integration tests against a real MongoDB (Atlas). They only run with
//
//	LIFELOG_MONGO_TEST=1 go test ./internal/store/ -run Integration -v
//
// They do NOT use the database from MONGODB_DB but a database of their own,
// "<MONGODB_DB>_integrationstest", and drop it again at the end. If there
// are collections that do not come from this test, the test stops instead of
// deleting anything.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
)

var ownCollections = []string{collEntries, collChunks, bucketFiles + ".files", bucketFiles + ".chunks"}

func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	if os.Getenv("LIFELOG_MONGO_TEST") != "1" {
		t.Skip("integration test skipped (set LIFELOG_MONGO_TEST=1)")
	}
	if err := config.FindDotenv(); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	name := c.MongoDB + "_integrationstest"
	s, err := Connect(ctx, c.MongoURI, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Disconnect(context.Background()) })

	existing, err := s.db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		t.Fatal(err)
	}
	for _, coll := range existing {
		if !slices.Contains(ownCollections, coll) {
			t.Fatalf("database %q contains foreign collection %q – test stops so it deletes nothing", name, coll)
		}
	}

	// Start clean and clean up at the end.
	if err := s.db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.db.Drop(ctx); err != nil {
			t.Errorf("test database %q not dropped: %v", name, err)
		}
	})
	t.Logf("test database: %s", name)

	if err := s.CreateIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return s, ctx
}

// testEntry is synthetic. hash determines the file's sha256.
func testEntry(id, fileID, hash string) core.Entry {
	return core.Entry{
		ID: id, Module: "health", Type: "lab_report",
		DocumentDate: ptr("2026-10-14"),
		ImportedAt:   time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC),
		Issuer:       ptr("Labor Muster GmbH"),
		FullText:     "Hämoglobin 14,2 g/dl",
		Facts:        []core.Fact{{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}},
		File: core.File{ID: fileID, FileName: "befund.pdf", MimeType: core.MimePDF,
			Size: 9, Sha256: hash},
		Corrections: []core.Correction{},
	}
}

func TestIntegration_SaveFindDelete(t *testing.T) {
	s, ctx := testStore(t)

	// Creating the indexes a second time must not break anything.
	if err := s.CreateIndexes(ctx); err != nil {
		t.Fatalf("CreateIndexes is not repeatable: %v", err)
	}

	entryID, fileID := NewID(), NewID()
	hash := strings.Repeat("ab", 32)
	data := []byte("%PDF-1.7 synthetic")

	// 1. File in GridFS, with metadata.
	if err := s.SaveFile(ctx, fileID, "befund.pdf", core.MimePDF, data); err != nil {
		t.Fatal(err)
	}
	oid, _ := bson.ObjectIDFromHex(fileID)
	stream, err := s.files.OpenDownloadStream(ctx, oid)
	if err != nil {
		t.Fatal(err)
	}
	read, _ := io.ReadAll(stream)
	_ = stream.Close()
	if !bytes.Equal(read, data) {
		t.Errorf("file changed: %q", read)
	}
	if f := stream.GetFile(); f.Name != "befund.pdf" || !strings.Contains(f.Metadata.String(), core.MimePDF) {
		t.Errorf("file metadata: %s / %s", f.Name, f.Metadata)
	}

	// 2. Chunks.
	chunks := []core.Chunk{
		{EntryID: entryID, Module: "health", Position: 1, Text: "Teil 1", Embedding: []float32{0.1, 0.2}},
		{EntryID: entryID, Module: "health", Position: 2, Text: "Teil 2", Embedding: []float32{0.3, 0.4}},
	}
	if err := s.SaveChunks(ctx, chunks); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveChunks(ctx, nil); err != nil {
		t.Errorf("empty list: %v", err)
	}
	checkChunkCount(t, s, ctx, entryID, 2)

	// 3. Entry last, then findable via the hash and in the list.
	if _, found, err := s.EntryByHash(ctx, hash); err != nil || found {
		t.Fatalf("before saving: found=%v, err=%v", found, err)
	}
	if err := s.SaveEntry(ctx, testEntry(entryID, fileID, hash)); err != nil {
		t.Fatal(err)
	}
	id, found, err := s.EntryByHash(ctx, hash)
	if err != nil || !found || id != entryID {
		t.Fatalf("EntryByHash = %q, %v, %v", id, found, err)
	}
	list, err := s.ListSummaries(ctx, "health")
	if err != nil || len(list) != 1 || list[0].ID != entryID {
		t.Fatalf("ListSummaries = %+v, %v", list, err)
	}

	// Read the full entry and open the file through the store.
	e, found, err := s.GetEntry(ctx, entryID)
	if err != nil || !found || e.FullText != "Hämoglobin 14,2 g/dl" || len(e.Facts) != 1 || e.File.ID != fileID {
		t.Fatalf("GetEntry = %+v, %v, %v", e, found, err)
	}
	if _, found, err := s.GetEntry(ctx, NewID()); err != nil || found {
		t.Errorf("unknown ID: found=%v, err=%v", found, err)
	}
	content, length, err := s.OpenFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	read, _ = io.ReadAll(content)
	_ = content.Close()
	if length != int64(len(data)) || !bytes.Equal(read, data) {
		t.Errorf("OpenFile: length %d, content %q", length, read)
	}

	// 4. Delete in the order of API.md 3.7, every step repeatable.
	for round := 1; round <= 2; round++ {
		if err := s.DeleteChunks(ctx, entryID); err != nil {
			t.Fatalf("round %d, chunks: %v", round, err)
		}
		if err := s.DeleteFile(ctx, fileID); err != nil {
			t.Fatalf("round %d, file: %v", round, err)
		}
		found, err := s.DeleteEntry(ctx, entryID)
		if err != nil || found != (round == 1) {
			t.Fatalf("round %d, entry: found=%v, err=%v", round, found, err)
		}
	}
	checkChunkCount(t, s, ctx, entryID, 0)
	if _, _, err := s.OpenFile(ctx, fileID); !errors.Is(err, core.ErrOriginalMissing) {
		t.Errorf("after deleting: want ErrOriginalMissing, got %v", err)
	}
	if _, found, _ := s.EntryByHash(ctx, hash); found {
		t.Error("hash still present after deleting: the same file could not be uploaded again")
	}
}

func TestIntegration_DuplicateViaIndex(t *testing.T) {
	s, ctx := testStore(t)
	hash := strings.Repeat("cd", 32)

	if err := s.SaveEntry(ctx, testEntry(NewID(), NewID(), hash)); err != nil {
		t.Fatal(err)
	}
	// Second entry with another ID but the same hash: the unique index must
	// reject it, even without a prior check.
	err := s.SaveEntry(ctx, testEntry(NewID(), NewID(), hash))
	if !errors.Is(err, core.ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}

	// Another hash works.
	if err := s.SaveEntry(ctx, testEntry(NewID(), NewID(), strings.Repeat("ef", 32))); err != nil {
		t.Errorf("another hash: %v", err)
	}
}

func checkChunkCount(t *testing.T, s *Store, ctx context.Context, entryID string, want int64) {
	t.Helper()
	oid, _ := bson.ObjectIDFromHex(entryID)
	n, err := s.chunks.CountDocuments(ctx, bson.D{{Key: "eintragId", Value: oid}})
	if err != nil {
		t.Fatal(err)
	}
	if n != want {
		t.Errorf("%d chunks, want %d", n, want)
	}
}
