package llmengine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/speech"
	"lifelog/backend/internal/speech/whisper"
)

// fakeModel returns text or err and remembers what it got.
type fakeModel struct {
	text  string
	err   error
	wav   []byte
	vocab string
}

func (f *fakeModel) TranscribeAudio(_ context.Context, wav []byte, vocabulary string) (string, error) {
	f.wav, f.vocab = wav, vocabulary
	return f.text, f.err
}

func TestRecognize_TextIsOneSegment(t *testing.T) {
	m := &fakeModel{text: "Blutdruck 122 zu 79, Puls 64."}
	e := New(m, "Gemini test-model")

	segs, err := e.Recognize(context.Background(), []byte("RIFF"))

	if err != nil || len(segs) != 1 || segs[0].Text != m.text {
		t.Fatalf("got %v, %v", segs, err)
	}
	if string(m.wav) != "RIFF" {
		t.Error("the recording was not passed on")
	}
	if m.vocab != whisper.Vocabulary {
		t.Error("the vocabulary was not sent as hint")
	}
	if e.Name() != "Gemini test-model" {
		t.Errorf("name %q", e.Name())
	}
}

func TestRecognize_NoSpeech(t *testing.T) {
	for _, text := range []string{"", "  \n"} {
		segs, err := New(&fakeModel{text: text}, "x").Recognize(context.Background(), nil)
		if err != nil || len(segs) != 0 {
			t.Errorf("%q: got %v, %v; want no segments", text, segs, err)
		}
	}
}

func TestRecognize_Errors(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		unavailable bool
	}{
		{"model unavailable", fmt.Errorf("Gemini HTTP 503: %w", llm.ErrUnavailable), true},
		{"invalid answer", fmt.Errorf("no JSON: %w", llm.ErrInvalidResponse), true},
		{"wrong key", errors.New("Gemini HTTP 403"), true},
		{"app gone", fmt.Errorf("Gemini call canceled: %w", context.Canceled), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(&fakeModel{err: c.err}, "x").Recognize(context.Background(), nil)
			if err == nil {
				t.Fatal("want an error")
			}
			if got := errors.Is(err, speech.ErrUnavailable); got != c.unavailable {
				t.Errorf("ErrUnavailable = %v, want %v (%v)", got, c.unavailable, err)
			}
			if !errors.Is(err, c.err) {
				t.Errorf("original error lost: %v", err)
			}
		})
	}
}

func TestPing(t *testing.T) {
	if err := New(&fakeModel{}, "x").Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}
