// Package fake provides a speech.Transcriber for tests: it returns a fixed
// transcript or error and records the calls. No program is started.
package fake

import (
	"context"
	"sync"

	"lifelog/backend/internal/core"
)

// Transcriber implements speech.Transcriber.
type Transcriber struct {
	Result  core.Transcript
	Err     error
	PingErr error
	// Block: wait until the context ends and return its error (to test the
	// time limit).
	Block bool

	mu        sync.Mutex
	calls     int
	lastMime  string
	lastBytes int
}

// Transcribe returns Result or Err.
func (t *Transcriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (core.Transcript, error) {
	t.mu.Lock()
	t.calls++
	t.lastMime, t.lastBytes = mimeType, len(audio)
	t.mu.Unlock()
	if t.Block {
		<-ctx.Done()
		return core.Transcript{}, ctx.Err()
	}
	if t.Err != nil {
		return core.Transcript{}, t.Err
	}
	return t.Result, nil
}

// Ping returns PingErr.
func (t *Transcriber) Ping(context.Context) error { return t.PingErr }

// Calls returns how often Transcribe was called.
func (t *Transcriber) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

// Last returns MIME type and size of the last recording.
func (t *Transcriber) Last() (mimeType string, size int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastMime, t.lastBytes
}
