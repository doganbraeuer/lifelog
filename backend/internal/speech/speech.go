// Package speech turns a spoken recording into text (docs/API.md, 3.9).
// It runs locally and costs no model call. Nothing is stored: the recording
// only exists in memory and, for a few seconds, in one temporary file for
// ffmpeg.
//
// Two interfaces keep the programs replaceable, as in packages llm and ocr:
//
//   - Converter turns any allowed recording into 16 kHz mono PCM, the format
//     Whisper needs. Today ffmpeg (subpackage ffmpeg).
//   - Engine recognizes the speech in such a recording. Today whisper.cpp,
//     running as its own local server (subpackage whisper); another engine
//     would be a new subpackage and one line in cmd/server.
//
// Pipeline combines them with the checks against invented text: length,
// loudness, and core.CleanTranscript. The rest of the backend only knows
// Transcriber.
package speech

import (
	"context"
	"errors"

	"lifelog/backend/internal/core"
)

// Transcriber turns a recording into text.
type Transcriber interface {
	// Transcribe returns the recognized text of the recording. mimeType is
	// the type from core.CheckAudio.
	Transcribe(ctx context.Context, audio []byte, mimeType string) (core.Transcript, error)

	// Ping checks whether transcribing works right now (GET /status).
	Ping(ctx context.Context) error
}

// Converter turns a recording into 16 kHz mono 16-bit samples.
type Converter interface {
	// ToPCM decodes at most limitSeconds of the recording; longer
	// recordings are cut there, so the caller can see that they are too
	// long without decoding hours.
	ToPCM(ctx context.Context, audio []byte, limitSeconds float64) ([]int16, error)
}

// Engine recognizes the speech in a 16 kHz mono WAV file.
type Engine interface {
	Recognize(ctx context.Context, wav []byte) ([]core.Segment, error)
	// Ping checks whether the engine answers.
	Ping(ctx context.Context) error
	// Name is program and model, e.g. "whisper.cpp large-v3-turbo".
	Name() string
}

// Kinds of errors. The HTTP layer chooses the code from them (docs/API.md,
// 3.9); every implementation translates its own errors into these.
var (
	// ErrUnavailable: a program is missing or does not answer (→ 503).
	ErrUnavailable = errors.New("speech recognition unavailable")
	// ErrUnreadable: the recording cannot be decoded, e.g. damaged or
	// without an audio track (→ 422 UNREADABLE).
	ErrUnreadable = errors.New("recording cannot be decoded")
	// ErrTooLong: longer than core.MaxAudioSeconds (→ 413 AUDIO_TOO_LONG).
	ErrTooLong = errors.New("recording too long")
	// ErrNoSpeech: silence, noise or too short (→ 422 NO_SPEECH).
	ErrNoSpeech = errors.New("no speech in the recording")
)
