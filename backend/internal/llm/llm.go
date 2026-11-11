// Package llm is the only place through which the backend calls language,
// vision and embedding models (CLAUDE.md: "provider stays replaceable").
//
// The rest of the backend only knows these interfaces. Another provider
// means: a new subpackage next to gemini/ that implements them, and one line
// in cmd/server/main.go.
package llm

import (
	"context"
	"errors"

	"lifelog/backend/internal/core"
)

// EmbeddingDimensions is the length of every embedding vector. The vector
// index in Atlas is fixed to exactly this number. If it changes, all chunks
// must be re-embedded.
const EmbeddingDimensions = 768

// Reader reads a document (image or PDF) and returns what is in it. The
// result is unchecked; core.CheckExtraction must check it.
type Reader interface {
	ReadDocument(ctx context.Context, data []byte, mimeType string, allowedTypes []string) (core.Extraction, error)

	// ReadText classifies an entered text (text import): date, issuer, type
	// and facts. Readable is always true and FullText stays empty; the full
	// text is the entered text, which the caller sets.
	ReadText(ctx context.Context, text string, allowedTypes []string) (core.Extraction, error)
}

// Embedder computes embeddings: vectors whose distance expresses how similar
// two texts are in content.
//
// Chunks and questions are embedded separately, because some models treat
// them differently ("document" vs. "search query"). Both methods return
// vectors with EmbeddingDimensions entries.
type Embedder interface {
	// EmbedChunks returns exactly one vector per text, in the same order.
	EmbedChunks(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuestion(ctx context.Context, question string) ([]float32, error)
}

// Answerer answers a question only from the given sources. It sets
// references like [1] that refer to Source.Number, and answers with
// core.NotFoundAnswer if the sources do not answer the question. Whether the
// answer really contains references is checked afterwards by
// core.BuildAnswer.
type Answerer interface {
	Answer(ctx context.Context, question string, sources []core.Source) (string, error)
}

// Kinds of errors. Every implementation translates its own errors into
// these, so the HTTP layer can choose the right code, whatever the provider.
var (
	// ErrUnavailable: network, timeout, limit (429) or server error of the
	// provider (→ 503 MODEL_UNAVAILABLE).
	ErrUnavailable = errors.New("model unavailable")

	// ErrInvalidResponse: the model answered, but not in the expected form
	// (→ 502 EXTRACTION_FAILED).
	ErrInvalidResponse = errors.New("invalid model response")
)
