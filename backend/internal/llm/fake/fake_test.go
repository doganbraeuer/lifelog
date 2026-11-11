package fake

import (
	"context"
	"errors"
	"slices"
	"testing"

	"lifelog/backend/internal/llm"
)

func TestEmbedder(t *testing.T) {
	e := &Embedder{}
	v, err := e.EmbedChunks(context.Background(), []string{"a", "b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || len(v[0]) != llm.EmbeddingDimensions {
		t.Fatalf("shape = %d × %d", len(v), len(v[0]))
	}
	if !slices.Equal(v[0], v[2]) || slices.Equal(v[0], v[1]) {
		t.Error("same text must give the same vector, another text another vector")
	}
	q, _ := e.EmbedQuestion(context.Background(), "a")
	if !slices.Equal(q, v[0]) {
		t.Error("question and chunk with the same text should be equal in the fake")
	}
	if e.Calls() != 2 {
		t.Errorf("calls = %d", e.Calls())
	}

	broken := &Embedder{Err: llm.ErrUnavailable}
	if _, err := broken.EmbedChunks(context.Background(), []string{"x"}); !errors.Is(err, llm.ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestReader(t *testing.T) {
	r := &Reader{Err: llm.ErrInvalidResponse}
	if _, err := r.ReadDocument(context.Background(), nil, "", nil); !errors.Is(err, llm.ErrInvalidResponse) {
		t.Errorf("err = %v", err)
	}
	if r.Calls() != 1 {
		t.Errorf("calls = %d", r.Calls())
	}
}
