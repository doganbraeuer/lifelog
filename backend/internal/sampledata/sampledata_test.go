package sampledata

import (
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

func TestEntries_ValidAndUnique(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range Entries() {
		for _, id := range []string{e.ID, e.File.ID} {
			if err := core.CheckID(id); err != nil {
				t.Errorf("%s: invalid ID %q", e.Type, id)
			}
			if ids[id] {
				t.Errorf("ID %q twice", id)
			}
			ids[id] = true
		}
		if !core.TypeAllowed(e.Module, e.Type) {
			t.Errorf("type %q not allowed in module %q", e.Type, e.Module)
		}
	}
	for _, id := range []string{LabReport, DoctorLetter, Note, Prescription} {
		if !ids[id] {
			t.Errorf("constant %q belongs to no entry", id)
		}
	}
}

func TestChunks_AsOnUpload(t *testing.T) {
	for _, e := range Entries() {
		c := Chunks(e)
		texts := core.SplitIntoChunks(e.FullText, core.ChunkMaxChars)
		if len(c) == 0 || len(c) != len(texts) {
			t.Fatalf("%s: %d chunks, want %d (> 0)", e.Type, len(c), len(texts))
		}
		for i, ch := range c {
			if ch.EntryID != e.ID || ch.Module != e.Module || ch.Position != i+1 || ch.Text != texts[i] {
				t.Errorf("%s chunk %d = %+v", e.Type, i, ch)
			}
		}
	}
}

func TestThrowaway(t *testing.T) {
	const id, fileID = "aaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbb"
	now := time.Date(2026, 11, 5, 10, 0, 0, 123_456_789, time.FixedZone("CET", 3600))
	e, chunks := Throwaway(id, fileID, now)

	if e.ID != id || e.File.ID != fileID || !core.TypeAllowed(e.Module, e.Type) {
		t.Errorf("entry %+v", e)
	}
	if want := time.Date(2026, 11, 5, 9, 0, 0, 123_000_000, time.UTC); !e.ImportedAt.Equal(want) || e.ImportedAt.Location() != time.UTC {
		t.Errorf("ImportedAt %v, want %v", e.ImportedAt, want)
	}
	if e.File.Size != int64(len(e.FullText)) || len(e.File.Sha256) != 64 {
		t.Errorf("file %+v", e.File)
	}
	if e.Facts == nil || e.Corrections == nil {
		t.Error("lists must be [], not nil")
	}
	if len(chunks) == 0 {
		t.Fatal("no chunks")
	}
	for _, c := range chunks {
		if c.EntryID != id || len(c.Embedding) != llm.EmbeddingDimensions || c.Embedding[0] != 1 {
			t.Errorf("chunk %d: entry %s, %d dimensions", c.Position, c.EntryID, len(c.Embedding))
		}
	}

	// Another time gives another hash: no duplicate.
	e2, _ := Throwaway(id, fileID, now.Add(time.Millisecond))
	if e2.File.Sha256 == e.File.Sha256 {
		t.Error("same hash for different times")
	}
}
