package store

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// unitVector has a 1 at position i and 0 elsewhere. Two different unit
// vectors have cos 0, so an Atlas score of 0.5.
func unitVector(i int) []float32 {
	v := make([]float32, llm.EmbeddingDimensions)
	v[i] = 1
	return v
}

// Runs in the test database (see testStore). Atlas builds the index in the
// background; the test waits for it, which can take a minute.
func TestIntegration_VectorSearch(t *testing.T) {
	s, _ := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	entryA, entryB, entrySport := NewID(), NewID(), NewID()
	chunks := []core.Chunk{
		{EntryID: entryA, Module: "gesundheit", Position: 1, Text: "Hämoglobin 14,2 g/dl", Embedding: unitVector(0)},
		{EntryID: entryB, Module: "gesundheit", Position: 1, Text: "Blutdruck 128/82", Embedding: unitVector(1)},
		// Same vector as A, but another module: the filter must exclude it.
		{EntryID: entrySport, Module: "sport", Position: 1, Text: "Lauf 10 km", Embedding: unitVector(0)},
	}
	if err := s.SaveChunks(ctx, chunks); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	status, err := s.CreateVectorIndex(ctx)
	if err != nil {
		t.Fatalf("creating via code failed (then use way B in the Atlas UI): %v", err)
	}
	t.Logf("vector index: %s", status)
	if status2, err := s.CreateVectorIndex(ctx); err != nil || status2 == "CREATED" {
		t.Errorf("creating a second time must detect the existing index: %q, %v", status2, err)
	}

	// Wait until the index is ready and the chunks are found.
	var hits []core.Hit
	for {
		status, _, err := s.VectorIndexStatus(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if status == "READY" {
			hits, err = s.SimilarChunks(ctx, unitVector(0), "gesundheit", core.MaxHits)
			if err != nil {
				t.Fatalf("vector search: %v", err)
			}
			if len(hits) >= 2 {
				break
			}
		}
		if ctx.Err() != nil {
			t.Fatalf("index not ready after %v (status %q, %d hits)", time.Since(start).Round(time.Second), status, len(hits))
		}
		time.Sleep(3 * time.Second)
	}
	t.Logf("index ready and searchable after %v", time.Since(start).Round(time.Second))

	for _, h := range hits {
		t.Logf("  %s score %.4f %q", h.EntryID[18:], h.Score, h.Text)
		if h.EntryID == entrySport {
			t.Error("chunk from module sport found: the filter does not work")
		}
	}
	if hits[0].EntryID != entryA || hits[0].Position != 1 || hits[0].Text != "Hämoglobin 14,2 g/dl" {
		t.Errorf("best hit = %+v, want the chunk of A", hits[0])
	}
	// Same vector: cos 1 → score 1. Perpendicular: cos 0 → score 0.5.
	// This confirms the conversion (1 + cos) / 2 that core.MinScore is based on.
	if math.Abs(hits[0].Score-1) > 1e-3 || math.Abs(hits[1].Score-0.5) > 1e-3 {
		t.Errorf("scores = %.4f / %.4f, want 1 / 0.5", hits[0].Score, hits[1].Score)
	}

	// In module sport there is only the one chunk.
	sport, err := s.SimilarChunks(ctx, unitVector(0), "sport", core.MaxHits)
	if err != nil || len(sport) != 1 || sport[0].EntryID != entrySport {
		t.Errorf("module sport: %+v, %v", sport, err)
	}
}

func TestIntegration_SummariesByIDs(t *testing.T) {
	s, ctx := testStore(t)
	a := testEntry(NewID(), NewID(), strings.Repeat("12", 32))
	if err := s.SaveEntry(ctx, a); err != nil {
		t.Fatal(err)
	}
	missing := NewID()

	m, err := s.SummariesByIDs(ctx, []string{a.ID, missing})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[a.ID].Type != "laborbefund" || m[a.ID].Issuer == nil {
		t.Errorf("map = %+v", m)
	}
	if _, ok := m[missing]; ok {
		t.Error("missing entry in the map")
	}
	if empty, err := s.SummariesByIDs(ctx, nil); err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty list: %#v, %v", empty, err)
	}
}
