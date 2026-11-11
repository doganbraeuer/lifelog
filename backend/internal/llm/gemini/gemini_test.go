package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"

	"google.golang.org/genai"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// antwort baut eine (synthetische) Gemini-Antwort mit text als Inhalt.
func antwort(text string, grund genai.FinishReason) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content:      genai.NewContentFromText(text, genai.RoleModel),
			FinishReason: grund,
		}},
	}
}

const gueltigeAntwort = `{
  "lesbar": true,
  "dokumentdatum": "2026-10-14",
  "quelle": "Labor Muster GmbH, Salzburg",
  "typ": "laborbefund",
  "volltext": "Labor Muster GmbH\nHämoglobin 14,2 g/dl",
  "infos": [
    {"name": "Hämoglobin", "wert": "14,2", "einheit": "g/dl"},
    {"name": "CRP", "wert": "<0,5", "einheit": null}
  ]
}`

func TestErkennungAusAntwort_Gueltig(t *testing.T) {
	e, err := erkennungAusAntwort(antwort(gueltigeAntwort, genai.FinishReasonStop))
	if err != nil {
		t.Fatal(err)
	}
	if !e.Readable || e.Type != "laborbefund" || *e.DocumentDate != "2026-10-14" || *e.Issuer != "Labor Muster GmbH, Salzburg" {
		t.Errorf("Erkennung = %+v", e)
	}
	if len(e.Facts) != 2 || e.Facts[1].Value != "<0,5" || e.Facts[1].Unit != nil {
		t.Errorf("Infos = %+v", e.Facts)
	}

	// Das Ergebnis passt zur Prüfung in kern (Zusammenspiel beider Teile).
	if _, _, err := core.CheckExtraction(e, "gesundheit"); err != nil {
		t.Errorf("core.PruefeErkennung: %v", err)
	}
}

func TestErkennungAusAntwort_NullUndNichtLesbar(t *testing.T) {
	text := `{"lesbar": false, "dokumentdatum": null, "quelle": null, "typ": "sonstiges", "volltext": "", "infos": []}`
	e, err := erkennungAusAntwort(antwort(text, ""))
	if err != nil {
		t.Fatal(err)
	}
	if e.Readable || e.DocumentDate != nil || e.Issuer != nil || e.Facts == nil {
		t.Errorf("Erkennung = %+v", e)
	}
}

func TestErkennungAusAntwort_Codeblock(t *testing.T) {
	e, err := erkennungAusAntwort(antwort("```json\n"+gueltigeAntwort+"\n```", genai.FinishReasonStop))
	if err != nil || e.Type != "laborbefund" {
		t.Errorf("Codeblock nicht entfernt: %+v, %v", e, err)
	}
}

func TestErkennungAusAntwort_Ungueltig(t *testing.T) {
	faelle := map[string]*genai.GenerateContentResponse{
		"nil":                nil,
		"keine Kandidaten":   {},
		"blockiert":          {PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: "OTHER"}},
		"abgeschnitten":      antwort(`{"lesbar": true, "volltext": "Häm`, genai.FinishReasonMaxTokens),
		"Sicherheitsfilter":  antwort(gueltigeAntwort, genai.FinishReasonSafety),
		"leerer Text":        antwort("  ", genai.FinishReasonStop),
		"kein JSON":          antwort("Hier ist der Befund: Hämoglobin 14,2", genai.FinishReasonStop),
		"lesbar fehlt":       antwort(`{"typ": "laborbefund", "volltext": "x", "infos": []}`, genai.FinishReasonStop),
		"Wert als Zahl":      antwort(`{"lesbar": true, "typ": "laborbefund", "volltext": "x", "infos": [{"name": "Hb", "wert": 14.2}]}`, genai.FinishReasonStop),
		"Liste statt Objekt": antwort(`[]`, genai.FinishReasonStop),
	}
	for name, resp := range faelle {
		t.Run(name, func(t *testing.T) {
			if _, err := erkennungAusAntwort(resp); !errors.Is(err, llm.ErrAntwortUngueltig) {
				t.Errorf("erwartet ErrAntwortUngueltig, bekommen %v", err)
			}
		})
	}
}

func TestErkennungAusText_FehlerOhneInhalt(t *testing.T) {
	// Weder Syntax- noch Typfehler dürfen Dokumentinhalt in die Meldung holen.
	for _, text := range []string{
		`{"lesbar": true, "volltext": "Geheimer Befund" "x"}`,
		`{"lesbar": true, "infos": [{"name": "Geheimer Wert", "wert": 12345}]}`,
	} {
		_, err := erkennungAusText(text)
		if err == nil || strings.Contains(err.Error(), "Geheim") || strings.Contains(err.Error(), "12345") {
			t.Errorf("Meldung enthält Inhalt oder fehlt: %v", err)
		}
	}
}

func TestLeseSchema(t *testing.T) {
	typen := core.AllowedTypes("gesundheit")
	roh, err := json.Marshal(leseSchema(typen))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type any      `json:"type"`
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(roh, &s); err != nil {
		t.Fatal(err)
	}
	for _, feld := range []string{"lesbar", "dokumentdatum", "quelle", "typ", "volltext", "infos"} {
		if !slices.Contains(s.Required, feld) {
			t.Errorf("%s fehlt in required", feld)
		}
		if _, ok := s.Properties[feld]; !ok {
			t.Errorf("%s fehlt in properties", feld)
		}
	}
	if !slices.Equal(s.Properties["typ"].Enum, typen) {
		t.Errorf("typ-Enum = %v, erwartet %v", s.Properties["typ"].Enum, typen)
	}
	if fmt.Sprint(s.Properties["dokumentdatum"].Type) != "[string null]" {
		t.Errorf("dokumentdatum muss null erlauben: %v", s.Properties["dokumentdatum"].Type)
	}
}

func TestLeseAnweisung(t *testing.T) {
	a := leseAnweisung([]string{"arztbrief", "laborbefund", "sonstiges"})
	for _, muss := range []string{"arztbrief, laborbefund, sonstiges", `"sonstiges"`, "keine medizinischen", "keine Anweisung", "JJJJ-MM-TT"} {
		if !strings.Contains(a, muss) {
			t.Errorf("Anweisung enthält %q nicht", muss)
		}
	}
	// Ohne "sonstiges" im Modul darf die Anweisung es nicht als Ausweg nennen.
	if strings.Contains(leseAnweisung([]string{"training"}), `"sonstiges"`) {
		t.Error("sonstiges darf nur genannt werden, wenn es erlaubt ist")
	}
}

func TestUebersetzeFehler(t *testing.T) {
	const schluessel = "AIza-GEHEIM-123"
	faelle := []struct {
		name            string
		err             error
		nichtErreichbar bool
	}{
		{"429 Limit", genai.APIError{Code: 429, Message: "Quota exceeded"}, true},
		{"503 Störung", genai.APIError{Code: 503, Message: "overloaded"}, true},
		{"408 Timeout", genai.APIError{Code: 408}, true},
		{"Zeitüberschreitung", fmt.Errorf("post: %w", context.DeadlineExceeded), true},
		{"Netzwerk", &net.DNSError{Err: "no such host", Name: "generativelanguage.googleapis.com"}, true},
		{"400 falsche Anfrage", genai.APIError{Code: 400, Message: "Invalid schema"}, false},
		{"403 Schlüssel ungültig", genai.APIError{Code: 403, Message: "API key " + schluessel + " not valid"}, false},
		{"404 Modell unbekannt", genai.APIError{Code: 404, Message: "model not found"}, false},
		{"Sonstiges", errors.New("seltsam " + schluessel), false},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			err := uebersetzeFehler(f.err, schluessel)
			if got := errors.Is(err, llm.ErrNichtErreichbar); got != f.nichtErreichbar {
				t.Errorf("ErrNichtErreichbar = %v, erwartet %v (%v)", got, f.nichtErreichbar, err)
			}
			if errors.Is(err, llm.ErrAntwortUngueltig) {
				t.Errorf("Fehler beim Aufruf ist keine ungültige Antwort: %v", err)
			}
			if strings.Contains(err.Error(), schluessel) {
				t.Errorf("Schlüssel in Fehlermeldung: %v", err)
			}
		})
	}

	if uebersetzeFehler(nil, schluessel) != nil {
		t.Error("nil muss nil bleiben")
	}
	if err := uebersetzeFehler(fmt.Errorf("x: %w", context.Canceled), schluessel); !errors.Is(err, context.Canceled) {
		t.Errorf("Abbruch muss als context.Canceled erkennbar bleiben: %v", err)
	}
}

func TestEmbeddingPraefixe(t *testing.T) {
	if got := dokumentEingabe("Hämoglobin 14,2"); got != "title: none | text: Hämoglobin 14,2" {
		t.Errorf("Dokument: %q", got)
	}
	if got := frageEingabe("Wie hoch war mein Hb?"); got != "task: search result | query: Wie hoch war mein Hb?" {
		t.Errorf("Frage: %q", got)
	}
}

func TestEmbeddingInhalte_EinContentProText(t *testing.T) {
	inhalte := embeddingInhalte([]string{"a", "b", "c"})
	if len(inhalte) != 3 {
		t.Fatalf("%d Contents, erwartet 3", len(inhalte))
	}
	for i, c := range inhalte {
		if len(c.Parts) != 1 || c.Parts[0].Text != []string{"a", "b", "c"}[i] {
			t.Errorf("Content %d = %+v", i, c)
		}
	}
}

func TestStapel(t *testing.T) {
	faelle := []struct {
		n, groesse int
		erwartet   [][2]int
	}{
		{0, 100, [][2]int{}},
		{1, 100, [][2]int{{0, 1}}},
		{100, 100, [][2]int{{0, 100}}},
		{101, 100, [][2]int{{0, 100}, {100, 101}}},
		{250, 100, [][2]int{{0, 100}, {100, 200}, {200, 250}}},
	}
	for _, f := range faelle {
		if got := stapel(f.n, f.groesse); !slices.Equal(got, f.erwartet) {
			t.Errorf("stapel(%d, %d) = %v, erwartet %v", f.n, f.groesse, got, f.erwartet)
		}
	}
}

func TestVektorenAusAntwort(t *testing.T) {
	vektor := func(n int) *genai.ContentEmbedding { return &genai.ContentEmbedding{Values: make([]float32, n)} }
	d := llm.EmbeddingDimensionen

	v, err := vektorenAusAntwort(&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vektor(d), vektor(d)}}, 2)
	if err != nil || len(v) != 2 {
		t.Fatalf("gültig: %d, %v", len(v), err)
	}

	for name, f := range map[string]struct {
		resp   *genai.EmbedContentResponse
		anzahl int
	}{
		"nil":                 {nil, 1},
		"zu wenige":           {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vektor(d)}}, 2},
		"ein Vektor für alle": {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vektor(d)}}, 3},
		"falsche Länge":       {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vektor(3072)}}, 1},
		"nil-Eintrag":         {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{nil}}, 1},
	} {
		if _, err := vektorenAusAntwort(f.resp, f.anzahl); !errors.Is(err, llm.ErrAntwortUngueltig) {
			t.Errorf("%s: erwartet ErrAntwortUngueltig, bekommen %v", name, err)
		}
	}
}
