package store

// Per-user data and daily limits against a real MongoDB. Only runs with
// LIFELOG_MONGO_TEST=1 (see integration_test.go); only these tests:
//
//	LIFELOG_MONGO_TEST=1 go test ./internal/store/ -run 'IntegrationSeparation|IntegrationUsage' -v
//
// They create no search index. All data is synthetic.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lifelog/backend/internal/core"
)

const (
	userA = "6730a1a1a1a1a1a1a1a1a1a1"
	userB = "6730b2b2b2b2b2b2b2b2b2b2"
)

// storeEntryOf stores an entry of the user with file, one chunk and OCR
// words, as an upload does. hash determines file.sha256.
func storeEntryOf(t *testing.T, s *Store, ctx context.Context, userID, hash string) core.Entry {
	t.Helper()
	e := testEntry(NewID(), NewID(), hash)
	if err := s.SaveFile(ctx, userID, e.File.ID, e.File.FileName, e.File.MimeType, []byte("%PDF-1.7 synthetisch "+hash)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveChunks(ctx, userID, []core.Chunk{{EntryID: e.ID, Module: "health", Position: 1, Text: "Hämoglobin 14,2 g/dl", Embedding: unitVector(0)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveOCR(ctx, userID, e.ID, core.OCRResult{Engine: "tesseract", Words: []core.OCRWord{{Text: "14,2", Page: 1}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveEntry(ctx, userID, e); err != nil {
		t.Fatal(err)
	}
	e.UserID = userID
	return e
}

func TestIntegrationSeparation_ReadingOnlyOwnEntries(t *testing.T) {
	s, ctx := testStore(t)
	a := storeEntryOf(t, s, ctx, userA, strings.Repeat("a1", 32))

	// GetEntry
	if got, found, err := s.GetEntry(ctx, userA, a.ID); err != nil || !found || got.UserID != userA {
		t.Fatalf("A reads own entry: %+v, %v, %v", got.UserID, found, err)
	}
	if _, found, err := s.GetEntry(ctx, userB, a.ID); err != nil || found {
		t.Errorf("B reads A's entry: found = %v, %v", found, err)
	}
	// ListSummaries
	if list, err := s.ListSummaries(ctx, userB, "health"); err != nil || len(list) != 0 {
		t.Errorf("B's timeline = %v, %v; want empty", list, err)
	}
	if list, err := s.ListSummaries(ctx, userA, "health"); err != nil || len(list) != 1 {
		t.Errorf("A's timeline = %d entries, %v; want 1", len(list), err)
	}
	// EntryByHash
	if _, found, err := s.EntryByHash(ctx, userB, a.File.Sha256); err != nil || found {
		t.Errorf("B finds A's hash: %v, %v", found, err)
	}
	// OpenFile: neither via B nor via the file ID alone.
	if _, _, err := s.OpenFile(ctx, userB, a.File.ID); !errors.Is(err, core.ErrOriginalMissing) {
		t.Errorf("B opens A's file: err = %v, want ErrOriginalMissing", err)
	}
	content, _, err := s.OpenFile(ctx, userA, a.File.ID)
	if err != nil {
		t.Fatalf("A opens own file: %v", err)
	}
	data, _ := io.ReadAll(content)
	content.Close()
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Errorf("file content = %q", data)
	}
	// OCR and summaries
	if _, found, _ := s.GetOCR(ctx, userB, a.ID); found {
		t.Error("B reads A's OCR words")
	}
	if m, err := s.SummariesByIDs(ctx, userB, []string{a.ID}); err != nil || len(m) != 0 {
		t.Errorf("B's summaries = %v, %v; want none", m, err)
	}
	if ids, err := s.OCRIDs(ctx, userB); err != nil || len(ids) != 0 {
		t.Errorf("B's OCR IDs = %v, %v", ids, err)
	}
}

func TestIntegrationSeparation_ChangingOnlyOwnEntries(t *testing.T) {
	s, ctx := testStore(t)
	a := storeEntryOf(t, s, ctx, userA, strings.Repeat("a2", 32))

	// SaveCorrection as B changes nothing.
	changed := a
	changed.Issuer = ptr("Von B geändert")
	if found, err := s.SaveCorrection(ctx, userB, changed); err != nil || found {
		t.Errorf("B corrects A's entry: found = %v, %v", found, err)
	}
	if found, err := s.SaveVerification(ctx, userB, a.ID, core.Unchecked(core.ReasonOCRFailed)); err != nil || found {
		t.Errorf("B sets A's verification: found = %v, %v", found, err)
	}
	// SaveOCR as B must not replace A's words (same _id).
	if err := s.SaveOCR(ctx, userB, a.ID, core.OCRResult{Engine: "von B"}); err == nil {
		t.Error("B replaced A's OCR words")
	}

	// Deleting as B removes nothing.
	if err := s.DeleteChunks(ctx, userB, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteOCR(ctx, userB, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile(ctx, userB, a.File.ID); err != nil {
		t.Fatal(err)
	}
	if found, err := s.DeleteEntry(ctx, userB, a.ID); err != nil || found {
		t.Errorf("B deletes A's entry: found = %v, %v", found, err)
	}

	got, found, err := s.GetEntry(ctx, userA, a.ID)
	if err != nil || !found {
		t.Fatalf("A's entry gone: %v, %v", found, err)
	}
	if *got.Issuer != *a.Issuer {
		t.Errorf("issuer = %q, want unchanged %q", *got.Issuer, *a.Issuer)
	}
	if words, found, _ := s.GetOCR(ctx, userA, a.ID); !found || words.Engine != "tesseract" {
		t.Errorf("A's OCR words changed: %+v, %v", words.Engine, found)
	}
	if n, _ := s.chunks.CountDocuments(ctx, bson.D{{Key: "entryId", Value: mustOID(t, a.ID)}}); n != 1 {
		t.Errorf("A's chunks = %d, want 1", n)
	}
	if _, _, err := s.OpenFile(ctx, userA, a.File.ID); err != nil {
		t.Errorf("A's file gone: %v", err)
	}
}

// Same file, two users: two entries. Same file, same user: duplicate. The
// old unique index across all users is dropped.
func TestIntegrationSeparation_UniqueHashPerUser(t *testing.T) {
	s, ctx := testStore(t)
	// The index of before 2026-11-18, as an existing database has it.
	if _, err := s.entries.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "file.sha256", Value: 1}},
		Options: options.Index().SetName(legacyHashIndex).SetUnique(true),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	specs, err := s.entries.Indexes().ListSpecifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, spec := range specs {
		names[spec.Name] = true
	}
	if names[legacyHashIndex] || !names["user_file_sha256_unique"] || !names["user_module"] {
		t.Errorf("indexes = %v", names)
	}

	hash := strings.Repeat("a3", 32)
	storeEntryOf(t, s, ctx, userA, hash)
	b := testEntry(NewID(), NewID(), hash)
	if err := s.SaveEntry(ctx, userB, b); err != nil {
		t.Errorf("B with the same file: %v, want a second entry", err)
	}
	again := testEntry(NewID(), NewID(), hash)
	if err := s.SaveEntry(ctx, userA, again); !errors.Is(err, core.ErrDuplicate) {
		t.Errorf("A with the same file again: err = %v, want ErrDuplicate", err)
	}
}

func TestIntegrationSeparation_AssignOrphans(t *testing.T) {
	s, ctx := testStore(t)
	// Data from before 2026-11-18: no userId anywhere, a file without
	// metadata and one with metadata but no owner.
	id, fileID := bson.NewObjectID(), bson.NewObjectID()
	orphan := testEntry(id.Hex(), fileID.Hex(), strings.Repeat("a4", 32))
	doc, err := toEntryDoc(orphan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.entries.InsertOne(ctx, doc); err != nil {
		t.Fatal(err)
	}
	if _, err := s.chunks.InsertOne(ctx, bson.D{{Key: "entryId", Value: id}, {Key: "module", Value: "health"}, {Key: "text", Value: "alt"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ocr.InsertOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "engine", Value: "alt"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.files.UploadFromStreamWithID(ctx, fileID, "alt.pdf", bytes.NewReader([]byte("%PDF alt")),
		options.GridFSUpload().SetMetadata(bson.D{{Key: "mimeType", Value: core.MimePDF}})); err != nil {
		t.Fatal(err)
	}
	if err := s.files.UploadFromStreamWithID(ctx, bson.NewObjectID(), "ohne.pdf", bytes.NewReader([]byte("%PDF ohne"))); err != nil {
		t.Fatal(err)
	}
	// Something of B that must stay B's.
	b := storeEntryOf(t, s, ctx, userB, strings.Repeat("b4", 32))

	// Invisible to everybody before.
	if _, found, _ := s.GetEntry(ctx, userA, id.Hex()); found {
		t.Fatal("orphan visible before assign")
	}

	got, err := s.AssignOrphans(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	if got != (DataCounts{Entries: 1, Chunks: 1, OCR: 1, Files: 2}) {
		t.Errorf("first run = %+v, want 1 entry, 1 chunk, 1 OCR, 2 files", got)
	}
	again, err := s.AssignOrphans(ctx, userA)
	if err != nil || again.Total() != 0 {
		t.Errorf("second run = %+v, %v; want all 0", again, err)
	}

	if _, found, _ := s.GetEntry(ctx, userA, id.Hex()); !found {
		t.Error("orphan not A's after assign")
	}
	if _, _, err := s.OpenFile(ctx, userA, fileID.Hex()); err != nil {
		t.Errorf("orphan file not A's: %v", err)
	}
	if _, found, _ := s.GetEntry(ctx, userB, b.ID); !found {
		t.Error("B's entry was taken over")
	}
	if _, found, _ := s.GetEntry(ctx, userA, b.ID); found {
		t.Error("A got B's entry")
	}
}

func TestIntegrationSeparation_DeleteUserData(t *testing.T) {
	s, ctx := testStore(t)
	storeEntryOf(t, s, ctx, userA, strings.Repeat("a5", 32))
	storeEntryOf(t, s, ctx, userA, strings.Repeat("a6", 32))
	b := storeEntryOf(t, s, ctx, userB, strings.Repeat("b5", 32))
	if _, err := s.IncrementUsage(ctx, userA, "2026-11-18", core.LimitUploads, 30, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	deleted, err := s.DeleteUserData(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != (DataCounts{Entries: 2, Chunks: 2, OCR: 2, Files: 2, Usage: 1}) {
		t.Errorf("deleted = %+v", deleted)
	}
	left, err := s.CountUserData(ctx, userA)
	if err != nil || left.Total() != 0 {
		t.Errorf("left of A = %+v, %v; want nothing", left, err)
	}
	// GridFS pieces of A's files are gone too.
	if n, _ := s.db.Collection(bucketFiles+".chunks").CountDocuments(ctx, bson.D{}); n != 1 {
		t.Errorf("GridFS pieces = %d, want 1 (B's file)", n)
	}
	bLeft, _ := s.CountUserData(ctx, userB)
	if bLeft != (DataCounts{Entries: 1, Chunks: 1, OCR: 1, Files: 1}) {
		t.Errorf("B's data = %+v, want untouched", bLeft)
	}
	if _, found, _ := s.GetEntry(ctx, userB, b.ID); !found {
		t.Error("B's entry deleted")
	}
	if n, err := s.CountEntries(ctx, userB); err != nil || n != 1 {
		t.Errorf("CountEntries(B) = %d, %v", n, err)
	}
}

// 20 requests at the same time with a limit of 7: exactly 7 are allowed.
func TestIntegrationUsage_AtomicCounter(t *testing.T) {
	s, ctx := testStore(t)
	const max, parallel = 7, 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed, failed := 0, 0
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.IncrementUsage(ctx, userA, "2026-11-18", core.LimitQuestions, max, time.Now().Add(time.Hour))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				t.Errorf("IncrementUsage: %v", err)
			}
			if ok {
				allowed++
			}
		}()
	}
	wg.Wait()
	if allowed != max {
		t.Errorf("allowed = %d of %d, want exactly %d", allowed, parallel, max)
	}

	// Another day, another limit and another user start at 0.
	for _, c := range []struct {
		user, day string
		kind      core.LimitKind
	}{
		{userA, "2026-11-19", core.LimitQuestions},
		{userA, "2026-11-18", core.LimitUploads},
		{userB, "2026-11-18", core.LimitQuestions},
	} {
		if ok, err := s.IncrementUsage(ctx, c.user, c.day, c.kind, max, time.Now().Add(time.Hour)); err != nil || !ok {
			t.Errorf("%+v: allowed = %v, %v; want a fresh counter", c, ok, err)
		}
	}

	var doc struct {
		Count     int       `bson:"count"`
		ExpiresAt time.Time `bson:"expiresAt"`
	}
	if err := s.usage.FindOne(ctx, bson.D{{Key: "userId", Value: mustOID(t, userA)}, {Key: "day", Value: "2026-11-18"},
		{Key: "kind", Value: "questions"}}).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Count != max || doc.ExpiresAt.IsZero() {
		t.Errorf("counter = %+v, want count %d with expiresAt", doc, max)
	}
}

func mustOID(t *testing.T, hex string) bson.ObjectID {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(hex)
	if err != nil {
		t.Fatal(err)
	}
	return oid
}
