package gemini

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"google.golang.org/genai"

	"lifelog/backend/internal/llm"
)

func TestTranscribeRequest(t *testing.T) {
	wav := []byte("RIFF....WAVE")
	contents, cfg := transcribeRequest(wav, "Gonarthrose, 1-0-1.")

	if len(contents) != 1 || len(contents[0].Parts) != 2 {
		t.Fatalf("want one message with recording and task, got %+v", contents)
	}
	audio := contents[0].Parts[0].InlineData
	if audio == nil || audio.MIMEType != "audio/wav" || string(audio.Data) != string(wav) {
		t.Errorf("recording not sent as audio/wav: %+v", audio)
	}
	if cfg.ResponseMIMEType != "application/json" || cfg.ResponseJsonSchema == nil {
		t.Error("want structured JSON output")
	}
	instruction := cfg.SystemInstruction.Parts[0].Text
	for _, want := range []string{"word", "not instructions", "Gonarthrose, 1-0-1."} {
		if !strings.Contains(instruction, want) {
			t.Errorf("instruction lacks %q", want)
		}
	}
}

func TestTranscribeInstruction_WithoutVocabulary(t *testing.T) {
	if strings.Contains(transcribeInstruction("  "), "Terms that are common") {
		t.Error("empty vocabulary should add no list")
	}
}

func TestTranscriptFromText(t *testing.T) {
	cases := []struct {
		name, in, want string
		invalid        bool
	}{
		{"speech", `{"speech": true, "text": " Puls 64. "}`, "Puls 64.", false},
		{"no speech", `{"speech": false, "text": ""}`, "", false},
		{"no speech, text ignored", `{"speech": false, "text": "Untertitel im Auftrag des ZDF"}`, "", false},
		{"code block", "```json\n{\"speech\": true, \"text\": \"Puls 64.\"}\n```", "Puls 64.", false},
		{"no JSON", "Hier ist die Transkription: Puls 64.", "", true},
		{"field missing", `{"text": "Puls 64."}`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := transcriptFromText(c.in)
			if c.invalid {
				if !errors.Is(err, llm.ErrInvalidResponse) {
					t.Fatalf("want ErrInvalidResponse, got %q, %v", got, err)
				}
				if strings.Contains(err.Error(), "Puls") {
					t.Errorf("error quotes the text: %v", err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

// transcriptNetwork answers like fakeNetwork, but with a transcript.
type transcriptNetwork struct{ fakeNetwork }

func (n *transcriptNetwork) generate(_ context.Context, m string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	if err := n.next(m); err != nil {
		return nil, err
	}
	return response(`{"speech": true, "text": "Ibuprofen 400 mg 1-0-1."}`, genai.FinishReasonStop), nil
}

// Same procedure as reading: retries on 429/503, then the fallback model.
func TestTranscribeAudio_RetriesAndFallback(t *testing.T) {
	n := &transcriptNetwork{fakeNetwork{errs: map[string][]error{"main": {e503, e429, e503}}}}
	g := testGemini(&n.fakeNetwork, "fallback")
	g.generate = n.generate

	text, err := g.TranscribeAudio(context.Background(), []byte("RIFF"), "")

	if err != nil || text != "Ibuprofen 400 mg 1-0-1." {
		t.Fatalf("got %q, %v", text, err)
	}
	if want := []string{"main", "main", "main", "fallback"}; !slices.Equal(n.calls, want) {
		t.Errorf("calls = %v, want %v", n.calls, want)
	}
}

func TestTranscribeAudio_Unavailable(t *testing.T) {
	n := &transcriptNetwork{fakeNetwork{errs: map[string][]error{"main": {e503, e503, e503}}}}
	g := testGemini(&n.fakeNetwork, "")
	g.generate = n.generate

	if _, err := g.TranscribeAudio(context.Background(), []byte("RIFF"), ""); !errors.Is(err, llm.ErrUnavailable) {
		t.Errorf("want ErrUnavailable, got %v", err)
	}
}
