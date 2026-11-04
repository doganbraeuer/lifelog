package gemini

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/modell"
)

// Die Tests hier arbeiten mit echten, aber kurzen Fristen (Millisekunden
// statt 50 s), weil es genau um das Zusammenspiel der Fristen geht.

// aufrufZeiten merkt sich pro Modell, wie viel Zeit bis zur Frist blieb, als
// es aufgerufen wurde.
type aufrufZeiten struct {
	mu   sync.Mutex
	rest map[string]time.Duration
}

func (a *aufrufZeiten) merken(ctx context.Context, modellName string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if frist, ok := ctx.Deadline(); ok {
		a.rest[modellName] = time.Until(frist)
	}
}

// haengendesHauptmodell: "haupt" antwortet nie (wartet bis zur Frist),
// "ersatz" antwortet sofort mit einer gültigen Erkennung.
func haengendesHauptmodell(z *aufrufZeiten) erzeugenFunk {
	return func(ctx context.Context, m string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		z.merken(ctx, m)
		if m == "haupt" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return antwort(gueltigeAntwort, genai.FinishReasonStop), nil
	}
}

func kurzesGemini(erzeugen erzeugenFunk, ersatz string, limit, reserve time.Duration) *Gemini {
	g := testGemini(&netzAttrappe{fehler: map[string][]error{}}, ersatz)
	g.erzeugen = erzeugen
	g.leseTimeout = limit
	g.e.ErsatzReserve = reserve
	return g
}

func TestReserve_HaengendesHauptmodellLaesstZeitFuerErsatz(t *testing.T) {
	z := &aufrufZeiten{rest: map[string]time.Duration{}}
	g := kurzesGemini(haengendesHauptmodell(z), "ersatz", 300*time.Millisecond, 120*time.Millisecond)

	start := time.Now()
	e, err := g.DokumentLesen(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})
	dauer := time.Since(start)

	if err != nil || e.Typ != "laborbefund" {
		t.Fatalf("erwartet Erfolg über das Ersatzmodell, bekommen %+v, %v", e, err)
	}
	// Das Hauptmodell sah eine Frist von ~180 ms (300 − 120), nicht 300 ms.
	if r := z.rest["haupt"]; r > 190*time.Millisecond || r < 150*time.Millisecond {
		t.Errorf("Hauptmodell hatte %v bis zur Frist, erwartet ~180ms", r)
	}
	// Dem Ersatz blieb ungefähr die Reserve.
	if r := z.rest["ersatz"]; r < 90*time.Millisecond {
		t.Errorf("Ersatzmodell hatte nur %v, erwartet ~120ms Reserve", r)
	}
	if dauer > 300*time.Millisecond {
		t.Errorf("Gesamtdauer %v überschreitet das Limit", dauer)
	}
}

func TestReserve_OhneErsatzBekommtHauptmodellDasGanzeLimit(t *testing.T) {
	z := &aufrufZeiten{rest: map[string]time.Duration{}}
	g := kurzesGemini(haengendesHauptmodell(z), "", 300*time.Millisecond, 120*time.Millisecond)

	_, err := g.DokumentLesen(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})

	if !errors.Is(err, modell.ErrNichtErreichbar) {
		t.Errorf("err = %v", err)
	}
	if r := z.rest["haupt"]; r < 280*time.Millisecond {
		t.Errorf("ohne Ersatzmodell darf es keine Reserve geben: Hauptmodell hatte nur %v", r)
	}
	if _, aufgerufen := z.rest["ersatz"]; aufgerufen {
		t.Error("Ersatzmodell aufgerufen, obwohl keins eingestellt ist")
	}
}

func TestReserve_NullHeisstKeinErsatzBeimHaengen(t *testing.T) {
	// Genau das Problem vom 04.11.: Ohne Reserve verbraucht das hängende
	// Hauptmodell alles, der Ersatz kommt nicht mehr zum Zug.
	z := &aufrufZeiten{rest: map[string]time.Duration{}}
	g := kurzesGemini(haengendesHauptmodell(z), "ersatz", 200*time.Millisecond, 0)

	_, err := g.DokumentLesen(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})

	if !errors.Is(err, modell.ErrNichtErreichbar) {
		t.Errorf("err = %v", err)
	}
	if _, aufgerufen := z.rest["ersatz"]; aufgerufen {
		t.Error("Ersatz ohne Restzeit aufgerufen")
	}
}

func TestReserve_FehlerDesHauptmodellsImLog(t *testing.T) {
	var log bytes.Buffer
	alt := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(alt) })

	n := &netzAttrappe{fehler: map[string][]error{"haupt": {e503, e503, e503}}}
	g := testGemini(n, "ersatz")
	if _, err := g.DokumentLesen(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"}); err != nil {
		t.Fatal(err)
	}

	var zeile string
	for _, z := range strings.Split(log.String(), "\n") {
		if strings.Contains(z, "Ersatzmodell wird einmal versucht") {
			zeile = z
		}
	}
	if zeile == "" {
		t.Fatalf("keine Logzeile zum Ersatz:\n%s", log.String())
	}
	for _, muss := range []string{"modell=haupt", "ersatz=ersatz", "HTTP 503", "restzeit="} {
		if !strings.Contains(zeile, muss) {
			t.Errorf("Logzeile enthält %q nicht: %s", muss, zeile)
		}
	}
	if strings.Contains(log.String(), "GEHEIM") {
		t.Error("Schlüssel im Log")
	}
}

func TestNeu_ReserveGrenzen(t *testing.T) {
	neu := func(reserve time.Duration) error {
		_, err := Neu(context.Background(), Einstellungen{
			Schluessel: "x", Modell: "a", ErsatzModell: "b", EmbeddingModell: "e", ErsatzReserve: reserve,
		})
		return err
	}
	for _, ok := range []time.Duration{0, 20 * time.Second, 40 * time.Second} {
		if err := neu(ok); err != nil {
			t.Errorf("Reserve %v: %v", ok, err)
		}
	}
	for _, falsch := range []time.Duration{-time.Second, 41 * time.Second, 50 * time.Second} {
		if err := neu(falsch); err == nil {
			t.Errorf("Reserve %v: erwartet Fehler", falsch)
		}
	}
}
