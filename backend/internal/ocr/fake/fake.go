// Package fake provides an ocr.Recognizer for tests: it returns a fixed
// result or error and counts the calls. No program is started.
package fake

import (
	"context"
	"sync"

	"lifelog/backend/internal/core"
)

// Recognizer implements ocr.Recognizer.
type Recognizer struct {
	Result core.OCRResult
	Err    error
	// Block: wait until the context ends and return its error (to test the
	// time limit and canceling).
	Block bool

	mu    sync.Mutex
	calls int
}

// Recognize returns Result or Err. Safe to call from several goroutines.
func (r *Recognizer) Recognize(ctx context.Context, data []byte, mimeType string) (core.OCRResult, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	if r.Block {
		<-ctx.Done()
		return core.OCRResult{}, ctx.Err()
	}
	if r.Err != nil {
		return core.OCRResult{}, r.Err
	}
	return r.Result, nil
}

// Calls returns how often Recognize was called.
func (r *Recognizer) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}
