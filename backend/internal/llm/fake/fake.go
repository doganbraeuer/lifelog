// Package fake replaces the models in tests: no network, no cost,
// predictable answers. The fakes count their calls, so tests can check that,
// e.g., no model was asked for a duplicate.
package fake

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// Reader always returns Extraction or Err.
type Reader struct {
	Extraction core.Extraction
	Err        error

	mu        sync.Mutex
	calls     int
	textCalls int
	lastText  string
}

var _ llm.Reader = (*Reader)(nil)

func (r *Reader) ReadDocument(_ context.Context, _ []byte, _ string, _ []string) (core.Extraction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.Extraction, r.Err
}

// ReadText returns the same extraction or error as ReadDocument, but counts
// separately.
func (r *Reader) ReadText(_ context.Context, text string, _ []string) (core.Extraction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.textCalls++
	r.lastText = text
	return r.Extraction, r.Err
}

// TextCalls: how often ReadText was called, and with which text last.
func (r *Reader) TextCalls() (int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.textCalls, r.lastText
}

// Calls: how often ReadDocument was called.
func (r *Reader) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// Embedder returns a fixed vector for every text that only depends on the
// text: same text, same vector. With Err every call fails.
type Embedder struct {
	Err error

	mu    sync.Mutex
	calls int
}

var _ llm.Embedder = (*Embedder)(nil)

func (e *Embedder) EmbedChunks(_ context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	if e.Err != nil {
		return nil, e.Err
	}
	vectors := make([][]float32, len(texts))
	for i, t := range texts {
		vectors[i] = Vector(t)
	}
	return vectors, nil
}

func (e *Embedder) EmbedQuestion(_ context.Context, question string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	if e.Err != nil {
		return nil, e.Err
	}
	return Vector(question), nil
}

// Calls: how often one of the two methods was called.
func (e *Embedder) Calls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

// Answerer always returns Text or Err and remembers the last request.
type Answerer struct {
	Text string
	Err  error

	mu       sync.Mutex
	calls    int
	question string
	sources  []core.Source
}

var _ llm.Answerer = (*Answerer)(nil)

func (a *Answerer) Answer(_ context.Context, question string, s []core.Source) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.question, a.sources = question, s
	return a.Text, a.Err
}

// Calls: how often Answer was called.
func (a *Answerer) Calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

// LastRequest: question and sources of the last call.
func (a *Answerer) LastRequest() (string, []core.Source) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.question, a.sources
}

// Vector turns a text into a fixed pseudo vector with
// llm.EmbeddingDimensions entries. It has no meaning but is deterministic,
// which is enough for tests.
func Vector(text string) []float32 {
	v := make([]float32, llm.EmbeddingDimensions)
	block := sha256.Sum256([]byte(text))
	for i := range v {
		if i%8 == 0 && i > 0 {
			block = sha256.Sum256(block[:])
		}
		v[i] = float32(binary.BigEndian.Uint32(block[(i%8)*4:])) / float32(1<<32)
	}
	return v
}
