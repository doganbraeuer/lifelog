package speech

import (
	"context"
	"fmt"
	"math"

	"lifelog/backend/internal/core"
)

// Pipeline implements Transcriber: convert, check length and loudness,
// recognize, clean.
type Pipeline struct {
	Converter Converter // nil: ErrUnavailable
	Engine    Engine    // nil: ErrUnavailable

	// MaxSeconds: longer recordings give ErrTooLong. 0: core.MaxAudioSeconds.
	MaxSeconds float64
}

// Transcribe implements Transcriber. Whisper is only asked if the
// recording has enough sound: in silence it invents sentences.
func (p *Pipeline) Transcribe(ctx context.Context, audio []byte, mimeType string) (core.Transcript, error) {
	if p.Converter == nil || p.Engine == nil {
		return core.Transcript{}, ErrUnavailable
	}
	maxSeconds := p.MaxSeconds
	if maxSeconds <= 0 {
		maxSeconds = core.MaxAudioSeconds
	}

	// One second more than allowed, so we can tell "exactly 5 minutes"
	// from "longer".
	samples, err := p.Converter.ToPCM(ctx, audio, maxSeconds+1)
	if err != nil {
		return core.Transcript{}, fmt.Errorf("convert %s: %w", mimeType, err)
	}
	seconds := core.PCMSeconds(samples)
	if seconds > maxSeconds {
		return core.Transcript{}, fmt.Errorf("more than %.0f s: %w", maxSeconds, ErrTooLong)
	}
	if seconds < core.MinAudioSeconds {
		return core.Transcript{}, fmt.Errorf("%.2f s: %w", seconds, ErrNoSpeech)
	}
	if sound := core.SpeechSeconds(samples); sound < core.MinSpeechSeconds {
		return core.Transcript{}, fmt.Errorf("%.2f s above the silence threshold: %w", sound, ErrNoSpeech)
	}

	segments, err := p.Engine.Recognize(ctx, core.WAV(samples))
	if err != nil {
		return core.Transcript{}, err
	}
	text := core.CleanTranscript(segments)
	if text == "" {
		return core.Transcript{}, fmt.Errorf("nothing left after cleaning %d segments: %w", len(segments), ErrNoSpeech)
	}
	return core.Transcript{
		Text:            text,
		DurationSeconds: math.Round(seconds*10) / 10,
		Language:        core.TranscriptLanguage,
		Engine:          p.Engine.Name(),
	}, nil
}

// Ping implements Transcriber.
func (p *Pipeline) Ping(ctx context.Context) error {
	if p.Converter == nil || p.Engine == nil {
		return ErrUnavailable
	}
	return p.Engine.Ping(ctx)
}
