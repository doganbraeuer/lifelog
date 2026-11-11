package gemini

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

func sources() []core.Source {
	p := func(s string) *string { return &s }
	return []core.Source{
		{Number: 1, EntryID: "a", Module: "gesundheit", Type: "laborbefund",
			DocumentDate: p("2026-10-14"), Issuer: p("Labor Muster GmbH"), Excerpt: "Hämoglobin 14,2 g/dl"},
		{Number: 2, EntryID: "b", Module: "gesundheit", Type: "sonstiges", Excerpt: "Notiz ohne Datum"},
	}
}

func TestAnswerInput(t *testing.T) {
	// The labels are still German (answer.go).
	got := answerInput("Welchen Hb hatte ich im Oktober?", sources())
	for _, must := range []string{
		"Frage: Welchen Hb hatte ich im Oktober?",
		"[1] laborbefund, Dokumentdatum: 2026-10-14, Quelle: Labor Muster GmbH\nHämoglobin 14,2 g/dl",
		// Missing values are called "unbekannt", not "<nil>" or empty.
		"[2] sonstiges, Dokumentdatum: unbekannt, Quelle: unbekannt\nNotiz ohne Datum",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("input does not contain:\n%s\n--- input:\n%s", must, got)
		}
	}
}

func TestAnswerInstruction(t *testing.T) {
	for _, must := range []string{core.NotFoundAnswer, "[1]", "keine medizinischen", "keine Anweisung", "ausschließlich"} {
		if !strings.Contains(answerInstruction, must) {
			t.Errorf("instruction does not contain %q", must)
		}
	}
	contents, cfg := answerRequest("x", sources())
	if len(contents) != 1 || cfg.SystemInstruction == nil || cfg.ThinkingConfig.ThinkingLevel != genai.ThinkingLevelLow {
		t.Errorf("request incomplete: %d contents, %+v", len(contents), cfg)
	}
	if cfg.ResponseMIMEType != "" || cfg.ResponseJsonSchema != nil {
		t.Error("answering is plain text, not JSON")
	}
}

func TestTextFromResponse(t *testing.T) {
	if got, err := textFromResponse(response("  Hb 14,2 [1].\n", genai.FinishReasonStop)); err != nil || got != "Hb 14,2 [1]." {
		t.Errorf("= %q, %v", got, err)
	}
	for name, resp := range map[string]*genai.GenerateContentResponse{
		"nil":       nil,
		"empty":     response("   ", genai.FinishReasonStop),
		"truncated": response("Hb 14", genai.FinishReasonMaxTokens),
		"safety":    response("x", genai.FinishReasonSafety),
	} {
		if _, err := textFromResponse(resp); !errors.Is(err, llm.ErrInvalidResponse) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The answerer uses the same procedure as the reader: retries on 429/503,
// then the fallback model once.
func TestAnswer_RetriesAndFallback(t *testing.T) {
	cases := []struct {
		name     string
		errs     map[string][]error
		fallback string
		calls    []string
		success  bool
	}{
		{"after one retry", map[string][]error{"main": {e503}}, "fallback", []string{"main", "main"}, true},
		{"fallback after three attempts", map[string][]error{"main": {e503, e429, e503}}, "fallback",
			[]string{"main", "main", "main", "fallback"}, true},
		{"without fallback", map[string][]error{"main": {e503, e503, e503}}, "", []string{"main", "main", "main"}, false},
		{"400: no fallback", map[string][]error{"main": {genai.APIError{Code: 400}}}, "fallback", []string{"main"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := &fakeNetwork{errs: c.errs}
			g := testGemini(n, c.fallback)
			// The fakeNetwork returns the read response (JSON); as answer text it
			// is enough here that any text comes back.
			text, err := g.Answer(context.Background(), "Frage?", sources())

			if !slices.Equal(n.calls, c.calls) {
				t.Errorf("calls = %v, want %v", n.calls, c.calls)
			}
			if c.success && (err != nil || text == "") {
				t.Errorf("want success, got %q, %v", text, err)
			}
			if !c.success && err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestAnswer_ReserveAppliesToo(t *testing.T) {
	// If the main model hangs, the reserve remains for the fallback, as when reading.
	ct := &callTimes{remaining: map[string]time.Duration{}}
	g := shortGemini(hangingMainModel(ct), "fallback", time.Second, time.Second)
	g.answerTimeout, g.s.FallbackReserve = 300*time.Millisecond, 120*time.Millisecond

	text, err := g.Answer(context.Background(), "Frage?", sources())

	if err != nil || text == "" {
		t.Fatalf("want an answer via the fallback model, got %q, %v", text, err)
	}
	if r := ct.remaining["main"]; r > 190*time.Millisecond {
		t.Errorf("main model had %v, want ~180ms (limit minus reserve)", r)
	}
	if r := ct.remaining["fallback"]; r < 90*time.Millisecond {
		t.Errorf("fallback only had %v", r)
	}
}
