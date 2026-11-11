package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/dienst"
)

// create stores a full entry like the upload does: file, two chunks, entry
// last.
func create(t *testing.T, s *Store, ctx context.Context, hash string) core.Entry {
	t.Helper()
	e := testEntry(NewID(), NewID(), hash)
	if err := s.SaveFile(ctx, e.File.ID, e.File.FileName, e.File.MimeType, []byte("%PDF-1.7 synthetic")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveChunks(ctx, []core.Chunk{
		{EntryID: e.ID, Module: "gesundheit", Position: 1, Text: "Teil 1", Embedding: []float32{0.1, 0.2}},
		{EntryID: e.ID, Module: "gesundheit", Position: 2, Text: "Teil 2", Embedding: []float32{0.3, 0.4}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveEntry(ctx, e); err != nil {
		t.Fatal(err)
	}
	return e
}

// checkFile: exists is true if the file can be opened.
func checkFile(t *testing.T, s *Store, ctx context.Context, fileID string, exists bool) {
	t.Helper()
	content, _, err := s.OpenFile(ctx, fileID)
	switch {
	case err == nil:
		content.Close()
		if !exists {
			t.Errorf("file %s still there", fileID)
		}
	case errors.Is(err, core.ErrOriginalMissing):
		if exists {
			t.Errorf("file %s missing", fileID)
		}
	default:
		t.Fatal(err)
	}
}

// Only runs with LIFELOG_MONGO_TEST=1 (see integration_test.go). Real
// service against a real MongoDB (API.md 3.7).
func TestIntegration_DeleteEntry(t *testing.T) {
	s, ctx := testStore(t)
	d := dienst.NeuLoeschen(s)

	a := create(t, s, ctx, strings.Repeat("a1", 32))
	b := create(t, s, ctx, strings.Repeat("b2", 32))

	if err := d.Ausfuehren(ctx, a.ID); err != nil {
		t.Fatal(err)
	}

	// A is completely gone.
	if _, exists, err := s.GetEntry(ctx, a.ID); err != nil || exists {
		t.Errorf("entry A: exists %v, %v", exists, err)
	}
	checkChunkCount(t, s, ctx, a.ID, 0)
	checkFile(t, s, ctx, a.File.ID, false)
	// The hash is free: the same file could be uploaded again.
	if _, exists, err := s.EntryByHash(ctx, a.File.Sha256); err != nil || exists {
		t.Errorf("hash of A still found: %v, %v", exists, err)
	}

	// B is untouched.
	if _, exists, err := s.GetEntry(ctx, b.ID); err != nil || !exists {
		t.Errorf("entry B: exists %v, %v", exists, err)
	}
	checkChunkCount(t, s, ctx, b.ID, 2)
	checkFile(t, s, ctx, b.File.ID, true)

	// Second delete: 404.
	if err := d.Ausfuehren(ctx, a.ID); !errors.Is(err, core.ErrEntryNotFound) {
		t.Errorf("second delete: %v, want ErrEntryNotFound", err)
	}
}

// As after an interrupted delete: file and chunks are already gone, the
// entry is still there. Calling again cleans up without an error.
func TestIntegration_DeleteAfterInterruption(t *testing.T) {
	s, ctx := testStore(t)
	e := create(t, s, ctx, strings.Repeat("c3", 32))
	if err := s.DeleteChunks(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile(ctx, e.File.ID); err != nil {
		t.Fatal(err)
	}

	if err := dienst.NeuLoeschen(s).Ausfuehren(ctx, e.ID); err != nil {
		t.Fatalf("delete with missing file: %v", err)
	}
	if _, exists, err := s.GetEntry(ctx, e.ID); err != nil || exists {
		t.Errorf("entry: exists %v, %v", exists, err)
	}
}
