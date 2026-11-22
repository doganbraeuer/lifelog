// Package llmengine implements speech.Engine with a language model that
// takes audio (llm.AudioTranscriber, today Gemini), for SPEECH_ENGINE=gemini.
//
// Why: on a server without a GPU, whisper large-v3 needs about 40 s per
// 30-second window (measured 2026-11-20 on the test server, 4 vCPUs AMD
// EPYC Rome, docs/speech/README.md), so recordings over about 1.5 minutes
// would hit the 120 s limit. The model answers in a few seconds.
//
// What stays the same: ffmpeg converts the recording first, the pipeline
// checks length and loudness before (no model call for silence), and
// core.CleanTranscript cleans the result. The vocabulary of the whisper
// engine is sent as a hint for spellings.
package llmengine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/speech"
	"lifelog/backend/internal/speech/whisper"
)

// Engine implements speech.Engine.
type Engine struct {
	// Vocabulary is sent as hint with every recording; New sets
	// whisper.Vocabulary, "" sends none.
	Vocabulary string

	model llm.AudioTranscriber
	name  string
}

var _ speech.Engine = (*Engine)(nil)

// New uses model; name is shown in the transcript and the log, e.g.
// "Gemini gemini-3.8-flash".
func New(model llm.AudioTranscriber, name string) *Engine {
	return &Engine{Vocabulary: whisper.Vocabulary, model: model, name: name}
}

// Name implements speech.Engine.
func (e *Engine) Name() string { return e.name }

// Recognize implements speech.Engine. The whole text is one segment (the
// model gives no times). No speech gives no segments, so the pipeline
// answers NO_SPEECH.
func (e *Engine) Recognize(ctx context.Context, wav []byte) ([]core.Segment, error) {
	text, err := e.model.TranscribeAudio(ctx, wav, e.Vocabulary)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err // the app dropped the connection
		}
		// Unreachable, limit, timeout or an answer in the wrong form: for
		// the app all of them mean "try again later" (503).
		return nil, fmt.Errorf("%w: %w", speech.ErrUnavailable, err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	return []core.Segment{{Text: text}}, nil
}

// Ping implements speech.Engine. There is no cheap check without a model
// call; a wrong key or an outage shows on the first transcription (503),
// like for reading and answering.
func (e *Engine) Ping(context.Context) error { return nil }
