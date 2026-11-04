package gemini

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/modell"
)

// uhr ist eine virtuelle Uhr: schlafen stellt sie nur vor und merkt sich
// die Wartezeiten. So dauern Tests keine echten Sekunden.
type uhr struct {
	jetzt   time.Time
	wartete []time.Duration
	fehler  error // wenn gesetzt, scheitert schlafen damit
}

func (u *uhr) wiederholer() wiederholer {
	w := standardWiederholer("Test")
	w.jetzt = func() time.Time { return u.jetzt }
	w.schlafen = func(_ context.Context, d time.Duration) error {
		if u.fehler != nil {
			return u.fehler
		}
		u.wartete = append(u.wartete, d)
		u.jetzt = u.jetzt.Add(d)
		return nil
	}
	return w
}

// skript liefert der Reihe nach die Fehler (nil = Erfolg) und zählt Aufrufe.
type skript struct {
	fehler  []error
	aufrufe int
}

func (s *skript) aufruf(context.Context) error {
	s.aufrufe++
	if s.aufrufe <= len(s.fehler) {
		return s.fehler[s.aufrufe-1]
	}
	return nil
}

var (
	e429 = genai.APIError{Code: 429, Message: "Quota exceeded"}
	e503 = genai.APIError{Code: 503, Message: "high demand"}
)

// mitFrist liefert einen Kontext, dessen Frist rest nach u.jetzt liegt.
func mitFrist(t *testing.T, u *uhr, rest time.Duration) context.Context {
	ctx, abbrechen := context.WithDeadline(context.Background(), u.jetzt.Add(rest))
	t.Cleanup(abbrechen)
	return ctx
}

func TestWiederholer(t *testing.T) {
	faelle := []struct {
		name     string
		fehler   []error
		rest     time.Duration // 0 = keine Frist
		aufrufe  int
		wartete  []time.Duration
		ergebnis error // nil = Erfolg
	}{
		{"Erfolg sofort", nil, 50 * time.Second, 1, nil, nil},
		{"503, 503, dann Erfolg", []error{e503, e503}, 50 * time.Second, 3,
			[]time.Duration{time.Second, 2 * time.Second}, nil},
		{"429 dreimal: aufgeben nach zwei Wiederholungen", []error{e429, e429, e429}, 50 * time.Second, 3,
			[]time.Duration{time.Second, 2 * time.Second}, e429},
		{"gemischt 429 und 503", []error{e429, e503}, 50 * time.Second, 3,
			[]time.Duration{time.Second, 2 * time.Second}, nil},
		{"500 wird nicht wiederholt", []error{genai.APIError{Code: 500}}, 50 * time.Second, 1, nil, genai.APIError{Code: 500}},
		{"504 wird nicht wiederholt", []error{genai.APIError{Code: 504}}, 50 * time.Second, 1, nil, genai.APIError{Code: 504}},
		{"400 wird nicht wiederholt", []error{genai.APIError{Code: 400}}, 50 * time.Second, 1, nil, genai.APIError{Code: 400}},
		{"Netzwerkfehler wird nicht wiederholt", []error{&net.DNSError{Err: "x"}}, 50 * time.Second, 1, nil, &net.DNSError{Err: "x"}},
		{"ohne Frist wird wiederholt", []error{e503}, 0, 2, []time.Duration{time.Second}, nil},
		// Gesamtlimit: Nach dem Warten müssen noch 5 s bleiben.
		{"Frist 5,5 s: 1 s Warten + 5 s Rest passt nicht", []error{e503, e503}, 5500 * time.Millisecond, 1, nil, e503},
		{"Frist 7,5 s: erste Wiederholung ja, zweite (2 s + 5 s bei 6,5 s Rest) nein",
			[]error{e503, e503, e503}, 7500 * time.Millisecond, 2, []time.Duration{time.Second}, e503},
		{"Frist genau 6 s: 1 s + 5 s passt gerade", []error{e503}, 6 * time.Second, 2, []time.Duration{time.Second}, nil},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			u := &uhr{jetzt: time.Now()}
			ctx := context.Background()
			if f.rest > 0 {
				ctx = mitFrist(t, u, f.rest)
			}
			s := &skript{fehler: f.fehler}

			err := u.wiederholer().ausfuehren(ctx, s.aufruf)

			if s.aufrufe != f.aufrufe {
				t.Errorf("Aufrufe = %d, erwartet %d", s.aufrufe, f.aufrufe)
			}
			if !slices.Equal(u.wartete, f.wartete) {
				t.Errorf("Wartezeiten = %v, erwartet %v", u.wartete, f.wartete)
			}
			if f.ergebnis == nil && err != nil {
				t.Errorf("erwartet Erfolg, bekommen %v", err)
			}
			if f.ergebnis != nil && (err == nil || err.Error() != f.ergebnis.Error()) {
				t.Errorf("Fehler = %v, erwartet %v", err, f.ergebnis)
			}
		})
	}
}

func TestWiederholer_AbbruchWaehrendDesWartens(t *testing.T) {
	u := &uhr{jetzt: time.Now(), fehler: context.Canceled}
	s := &skript{fehler: []error{e503, e503}}

	err := u.wiederholer().ausfuehren(context.Background(), s.aufruf)

	if !errors.Is(err, context.Canceled) || s.aufrufe != 1 {
		t.Errorf("err = %v, Aufrufe = %d; erwartet Canceled nach 1 Aufruf", err, s.aufrufe)
	}
}

func TestSchlafenMitKontext(t *testing.T) {
	if err := schlafenMitKontext(context.Background(), time.Millisecond); err != nil {
		t.Errorf("normales Schlafen: %v", err)
	}
	ctx, abbrechen := context.WithCancel(context.Background())
	abbrechen()
	start := time.Now()
	if err := schlafenMitKontext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("abgebrochen: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Error("Schlafen hat trotz Abbruch gewartet")
	}
}

// --- Zusammenspiel in Gemini: Wiederholung und Ersatzmodell ---

// netzAttrappe ersetzt die beiden SDK-Aufrufe. Pro Modellname eine Liste von
// Fehlern (nil = Erfolg), die der Reihe nach geliefert werden.
type netzAttrappe struct {
	fehler  map[string][]error
	aufrufe []string // Modellnamen in Aufrufreihenfolge
}

func (n *netzAttrappe) naechster(modellName string) error {
	n.aufrufe = append(n.aufrufe, modellName)
	liste := n.fehler[modellName]
	if len(liste) == 0 {
		return nil
	}
	n.fehler[modellName] = liste[1:]
	return liste[0]
}

func (n *netzAttrappe) erzeugen(_ context.Context, m string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	if err := n.naechster(m); err != nil {
		return nil, err
	}
	return antwort(gueltigeAntwort, genai.FinishReasonStop), nil
}

func (n *netzAttrappe) einbetten(_ context.Context, m string, inhalte []*genai.Content, _ *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
	if err := n.naechster(m); err != nil {
		return nil, err
	}
	resp := &genai.EmbedContentResponse{}
	for range inhalte {
		resp.Embeddings = append(resp.Embeddings, &genai.ContentEmbedding{Values: make([]float32, modell.EmbeddingDimensionen)})
	}
	return resp, nil
}

func testGemini(n *netzAttrappe, ersatz string) *Gemini {
	return &Gemini{
		e: Einstellungen{
			Schluessel: "GEHEIM", Modell: "haupt", ErsatzModell: ersatz, EmbeddingModell: "embed",
			ErsatzReserve: 20 * time.Second,
		},
		erzeugen:     n.erzeugen,
		einbettenAPI: n.einbetten,
		leseTimeout:  leseTimeout,
		neuWiederholer: func(b string) wiederholer {
			w := standardWiederholer(b)
			w.schlafen = func(context.Context, time.Duration) error { return nil } // nicht wirklich warten
			return w
		},
	}
}

func TestDokumentLesen_WiederholungUndErsatz(t *testing.T) {
	faelle := []struct {
		name            string
		fehler          map[string][]error
		ersatz          string
		aufrufe         []string
		nichtErreichbar bool // erwartetes Ergebnis; sonst Erfolg oder anderer Fehler
		erfolg          bool
	}{
		{"Hauptmodell klappt nach einer Wiederholung",
			map[string][]error{"haupt": {e503}}, "ersatz",
			[]string{"haupt", "haupt"}, false, true},
		{"Hauptmodell bleibt überlastet, Ersatz klappt",
			map[string][]error{"haupt": {e503, e429, e503}}, "ersatz",
			[]string{"haupt", "haupt", "haupt", "ersatz"}, false, true},
		{"Ersatz wird nur einmal versucht, nicht wiederholt",
			map[string][]error{"haupt": {e503, e503, e503}, "ersatz": {e503, e503}}, "ersatz",
			[]string{"haupt", "haupt", "haupt", "ersatz"}, true, false},
		{"ohne Ersatzmodell: nach drei Versuchen nicht erreichbar",
			map[string][]error{"haupt": {e503, e503, e503}}, "",
			[]string{"haupt", "haupt", "haupt"}, true, false},
		{"504 wird nicht wiederholt, aber Ersatz greift",
			map[string][]error{"haupt": {genai.APIError{Code: 504}}}, "ersatz",
			[]string{"haupt", "ersatz"}, false, true},
		{"400 ist kein Erreichbarkeitsproblem: kein Ersatz",
			map[string][]error{"haupt": {genai.APIError{Code: 400}}}, "ersatz",
			[]string{"haupt"}, false, false},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			n := &netzAttrappe{fehler: f.fehler}
			g := testGemini(n, f.ersatz)

			e, err := g.DokumentLesen(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})

			if !slices.Equal(n.aufrufe, f.aufrufe) {
				t.Errorf("Aufrufe = %v, erwartet %v", n.aufrufe, f.aufrufe)
			}
			if got := errors.Is(err, modell.ErrNichtErreichbar); got != f.nichtErreichbar {
				t.Errorf("ErrNichtErreichbar = %v, erwartet %v (%v)", got, f.nichtErreichbar, err)
			}
			if f.erfolg && (err != nil || e.Typ != "laborbefund") {
				t.Errorf("erwartet Erfolg, bekommen %+v, %v", e, err)
			}
			if !f.erfolg && err == nil {
				t.Error("erwartet einen Fehler")
			}
		})
	}
}

func TestEinbetten_WiederholungOhneErsatz(t *testing.T) {
	n := &netzAttrappe{fehler: map[string][]error{"embed": {e429}}}
	g := testGemini(n, "ersatz")
	v, err := g.AbschnitteEinbetten(context.Background(), []string{"a", "b"})
	if err != nil || len(v) != 2 {
		t.Fatalf("v = %d, err = %v", len(v), err)
	}
	if !slices.Equal(n.aufrufe, []string{"embed", "embed"}) {
		t.Errorf("Aufrufe = %v", n.aufrufe)
	}

	// Bleibt das Embedding-Modell überlastet, gibt es keinen Ersatz.
	n = &netzAttrappe{fehler: map[string][]error{"embed": {e503, e503, e503}}}
	g = testGemini(n, "ersatz")
	if _, err := g.FrageEinbetten(context.Background(), "x"); !errors.Is(err, modell.ErrNichtErreichbar) {
		t.Errorf("err = %v", err)
	}
	if !slices.Equal(n.aufrufe, []string{"embed", "embed", "embed"}) {
		t.Errorf("Aufrufe = %v (kein anderes Modell erlaubt)", n.aufrufe)
	}
}

func TestEinbetten_LeereListeOhneAufruf(t *testing.T) {
	n := &netzAttrappe{fehler: map[string][]error{}}
	v, err := testGemini(n, "").AbschnitteEinbetten(context.Background(), nil)
	if err != nil || v == nil || len(v) != 0 || len(n.aufrufe) != 0 {
		t.Errorf("v = %#v, err = %v, Aufrufe = %v", v, err, n.aufrufe)
	}
}

func TestNeu_ErsatzGleichHauptWirdIgnoriert(t *testing.T) {
	g, err := Neu(context.Background(), Einstellungen{
		Schluessel: "x", Modell: "gemini-3.8-flash", ErsatzModell: "gemini-3.8-flash", EmbeddingModell: "e",
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.e.ErsatzModell != "" {
		t.Errorf("Ersatz = %q, erwartet leer", g.e.ErsatzModell)
	}
}
