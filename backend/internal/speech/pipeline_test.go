package speech

import (
	"context"
	"errors"
	"math"
	"testing"

	"lifelog/backend/internal/core"
)

type fakeConverter struct {
	samples []int16
	err     error
	limit   float64
}

func (c *fakeConverter) ToPCM(_ context.Context, _ []byte, limitSeconds float64) ([]int16, error) {
	c.limit = limitSeconds
	if c.err != nil {
		return nil, c.err
	}
	// Like ffmpeg: never more than the limit.
	n := min(len(c.samples), int(limitSeconds*core.SpeechSampleRate))
	return c.samples[:n], nil
}

type fakeEngine struct {
	segments []core.Segment
	err      error
	pingErr  error
	wavs     [][]byte
}

func (e *fakeEngine) Recognize(_ context.Context, wav []byte) ([]core.Segment, error) {
	e.wavs = append(e.wavs, wav)
	return e.segments, e.err
}
func (e *fakeEngine) Ping(context.Context) error { return e.pingErr }
func (e *fakeEngine) Name() string               { return "engine 1.0" }

// sound: seconds of a loud sine.
func sound(seconds float64) []int16 {
	out := make([]int16, int(seconds*core.SpeechSampleRate))
	for i := range out {
		out[i] = int16(8000 * math.Sin(2*math.Pi*220*float64(i)/core.SpeechSampleRate))
	}
	return out
}

func silence(seconds float64) []int16 { return make([]int16, int(seconds*core.SpeechSampleRate)) }

func TestPipeline_Transcribe(t *testing.T) {
	c := &fakeConverter{samples: sound(12.34)}
	e := &fakeEngine{segments: []core.Segment{
		{Start: 0, End: 3, Text: " Arztbrief vom 12. November 2026."},
		{Start: 3, End: 6, Text: " Ibuprofen 400 mg dreimal täglich."},
		{Start: 6, End: 8, Text: " Untertitel im Auftrag des ZDF, 2017"},
	}}
	got, err := (&Pipeline{Converter: c, Engine: e}).Transcribe(context.Background(), []byte("m4a"), core.MimeAudioMP4)
	if err != nil {
		t.Fatal(err)
	}
	want := core.Transcript{
		Text:            "Arztbrief vom 12. November 2026. Ibuprofen 400 mg dreimal täglich.",
		DurationSeconds: 12.3,
		Language:        "de",
		Engine:          "engine 1.0",
	}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if c.limit != core.MaxAudioSeconds+1 {
		t.Errorf("decoded up to %v s, want %v", c.limit, core.MaxAudioSeconds+1)
	}
	if len(e.wavs) != 1 || string(e.wavs[0][:4]) != "RIFF" || len(e.wavs[0]) != 44+2*len(c.samples) {
		t.Errorf("engine got no WAV of the samples")
	}
}

func TestPipeline_Errors(t *testing.T) {
	broken := errors.New("boom")
	tests := []struct {
		name       string
		conv       *fakeConverter
		eng        *fakeEngine
		max        float64
		want       error
		engineUsed bool
	}{
		{"exactly at the limit is fine", &fakeConverter{samples: sound(10)}, &fakeEngine{segments: []core.Segment{{Text: "Puls 64."}}}, 10, nil, true},
		{"too long", &fakeConverter{samples: sound(12)}, &fakeEngine{}, 10, ErrTooLong, false},
		{"default limit 5 minutes", &fakeConverter{samples: silence(core.MaxAudioSeconds + 5)}, &fakeEngine{}, 0, ErrTooLong, false},
		{"too short", &fakeConverter{samples: sound(0.4)}, &fakeEngine{}, 0, ErrNoSpeech, false},
		{"silence never reaches Whisper", &fakeConverter{samples: silence(10)}, &fakeEngine{segments: []core.Segment{{Text: "Untertitel im Auftrag des ZDF"}}}, 0, ErrNoSpeech, false},
		{"only invented text", &fakeConverter{samples: sound(5)}, &fakeEngine{segments: []core.Segment{{Text: "Vielen Dank fürs Zuschauen!"}}}, 0, ErrNoSpeech, true},
		{"engine finds nothing", &fakeConverter{samples: sound(5)}, &fakeEngine{}, 0, ErrNoSpeech, true},
		{"cannot decode", &fakeConverter{err: ErrUnreadable}, &fakeEngine{}, 0, ErrUnreadable, false},
		{"engine unavailable", &fakeConverter{samples: sound(5)}, &fakeEngine{err: ErrUnavailable}, 0, ErrUnavailable, true},
		{"other engine error passes through", &fakeConverter{samples: sound(5)}, &fakeEngine{err: broken}, 0, broken, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{Converter: tt.conv, Engine: tt.eng, MaxSeconds: tt.max}
			_, err := p.Transcribe(context.Background(), []byte("x"), core.MimeAudioMP4)
			if !errors.Is(err, tt.want) || (tt.want == nil) != (err == nil) {
				t.Errorf("error %v, want %v", err, tt.want)
			}
			if used := len(tt.eng.wavs) > 0; used != tt.engineUsed {
				t.Errorf("engine called: %v, want %v", used, tt.engineUsed)
			}
		})
	}
}

func TestPipeline_Unconfigured(t *testing.T) {
	for name, p := range map[string]*Pipeline{
		"no converter": {Engine: &fakeEngine{}},
		"no engine":    {Converter: &fakeConverter{}},
	} {
		if _, err := p.Transcribe(context.Background(), nil, ""); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: transcribe %v", name, err)
		}
		if err := p.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: ping %v", name, err)
		}
	}
	p := &Pipeline{Converter: &fakeConverter{}, Engine: &fakeEngine{pingErr: ErrUnavailable}}
	if err := p.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("engine down: ping %v", err)
	}
	p.Engine = &fakeEngine{}
	if err := p.Ping(context.Background()); err != nil {
		t.Errorf("all up: ping %v", err)
	}
}
