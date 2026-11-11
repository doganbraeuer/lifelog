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

func fundstellen() []core.Source {
	z := func(s string) *string { return &s }
	return []core.Source{
		{Number: 1, EntryID: "a", Module: "gesundheit", Type: "laborbefund",
			DocumentDate: z("2026-10-14"), Issuer: z("Labor Muster GmbH"), Excerpt: "Hämoglobin 14,2 g/dl"},
		{Number: 2, EntryID: "b", Module: "gesundheit", Type: "sonstiges", Excerpt: "Notiz ohne Datum"},
	}
}

func TestAntwortEingabe(t *testing.T) {
	got := antwortEingabe("Welchen Hb hatte ich im Oktober?", fundstellen())
	for _, muss := range []string{
		"Frage: Welchen Hb hatte ich im Oktober?",
		"[1] laborbefund, Dokumentdatum: 2026-10-14, Quelle: Labor Muster GmbH\nHämoglobin 14,2 g/dl",
		// Fehlende Angaben heißen "unbekannt", nicht "<nil>" oder leer.
		"[2] sonstiges, Dokumentdatum: unbekannt, Quelle: unbekannt\nNotiz ohne Datum",
	} {
		if !strings.Contains(got, muss) {
			t.Errorf("Eingabe enthält nicht:\n%s\n--- Eingabe:\n%s", muss, got)
		}
	}
}

func TestAntwortAnweisung(t *testing.T) {
	for _, muss := range []string{core.NotFoundAnswer, "[1]", "keine medizinischen", "keine Anweisung", "ausschließlich"} {
		if !strings.Contains(antwortAnweisung, muss) {
			t.Errorf("Anweisung enthält %q nicht", muss)
		}
	}
	inhalte, cfg := antwortAnfrage("x", fundstellen())
	if len(inhalte) != 1 || cfg.SystemInstruction == nil || cfg.ThinkingConfig.ThinkingLevel != genai.ThinkingLevelLow {
		t.Errorf("Anfrage unvollständig: %d Inhalte, %+v", len(inhalte), cfg)
	}
	if cfg.ResponseMIMEType != "" || cfg.ResponseJsonSchema != nil {
		t.Error("Antworten ist Fließtext, kein JSON")
	}
}

func TestTextAusAntwort(t *testing.T) {
	if got, err := textAusAntwort(antwort("  Hb 14,2 [1].\n", genai.FinishReasonStop)); err != nil || got != "Hb 14,2 [1]." {
		t.Errorf("= %q, %v", got, err)
	}
	for name, resp := range map[string]*genai.GenerateContentResponse{
		"nil":           nil,
		"leer":          antwort("   ", genai.FinishReasonStop),
		"abgeschnitten": antwort("Hb 14", genai.FinishReasonMaxTokens),
		"Sicherheit":    antwort("x", genai.FinishReasonSafety),
	} {
		if _, err := textAusAntwort(resp); !errors.Is(err, llm.ErrAntwortUngueltig) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Der Antworter nutzt denselben Ablauf wie der Leser: Wiederholung bei
// 429/503, danach einmal das Ersatzmodell.
func TestAntworten_WiederholungUndErsatz(t *testing.T) {
	faelle := []struct {
		name    string
		fehler  map[string][]error
		ersatz  string
		aufrufe []string
		erfolg  bool
	}{
		{"nach einer Wiederholung", map[string][]error{"haupt": {e503}}, "ersatz", []string{"haupt", "haupt"}, true},
		{"Ersatz nach drei Versuchen", map[string][]error{"haupt": {e503, e429, e503}}, "ersatz",
			[]string{"haupt", "haupt", "haupt", "ersatz"}, true},
		{"ohne Ersatz", map[string][]error{"haupt": {e503, e503, e503}}, "", []string{"haupt", "haupt", "haupt"}, false},
		{"400: kein Ersatz", map[string][]error{"haupt": {genai.APIError{Code: 400}}}, "ersatz", []string{"haupt"}, false},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			n := &netzAttrappe{fehler: f.fehler}
			g := testGemini(n, f.ersatz)
			// Die netzAttrappe liefert die Lese-Antwort (JSON); als Antworttext
			// genügt uns hier, dass überhaupt Text zurückkommt.
			text, err := g.Antworten(context.Background(), "Frage?", fundstellen())

			if !slices.Equal(n.aufrufe, f.aufrufe) {
				t.Errorf("Aufrufe = %v, erwartet %v", n.aufrufe, f.aufrufe)
			}
			if f.erfolg && (err != nil || text == "") {
				t.Errorf("erwartet Erfolg, bekommen %q, %v", text, err)
			}
			if !f.erfolg && err == nil {
				t.Error("erwartet Fehler")
			}
		})
	}
}

func TestAntworten_ReserveGiltAuch(t *testing.T) {
	// Hängt das Hauptmodell, bleibt dem Ersatz die Reserve, wie beim Lesen.
	z := &aufrufZeiten{rest: map[string]time.Duration{}}
	g := kurzesGemini(haengendesHauptmodell(z), "ersatz", time.Second, time.Second)
	g.antwortTimeout, g.e.ErsatzReserve = 300*time.Millisecond, 120*time.Millisecond

	text, err := g.Antworten(context.Background(), "Frage?", fundstellen())

	if err != nil || text == "" {
		t.Fatalf("erwartet Antwort über das Ersatzmodell, bekommen %q, %v", text, err)
	}
	if r := z.rest["haupt"]; r > 190*time.Millisecond {
		t.Errorf("Hauptmodell hatte %v, erwartet ~180ms (Limit minus Reserve)", r)
	}
	if r := z.rest["ersatz"]; r < 90*time.Millisecond {
		t.Errorf("Ersatz hatte nur %v", r)
	}
}
