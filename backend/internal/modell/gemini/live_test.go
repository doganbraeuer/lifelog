package gemini

// Live-Tests gegen die echte Gemini-API. Sie laufen nur mit
//
//	LIFELOG_LIVE=1 go test ./internal/modell/gemini/ -run Live -v
//
// Sie kosten Anfragen aus dem Kontingent und schicken ausschließlich das
// synthetische Dokument aus backend/testdaten/ an Google. Der API-Schlüssel
// kommt aus der .env im Repo-Wurzelverzeichnis und wird nie ausgegeben.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/modell"
)

func liveGemini(t *testing.T) *Gemini {
	t.Helper()
	if os.Getenv("LIFELOG_LIVE") != "1" {
		t.Skip("Live-Test übersprungen (LIFELOG_LIVE=1 setzen)")
	}
	if err := config.FindDotenv(); err != nil {
		t.Fatal(err)
	}
	k, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Neu(context.Background(), Einstellungen{
		Schluessel:      k.GeminiKey,
		Modell:          k.GeminiModel,
		ErsatzModell:    k.GeminiFallbackModel,
		ErsatzReserve:   k.GeminiFallbackReserve,
		EmbeddingModell: k.GeminiEmbeddingModel,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Modelle: %s (Ersatz %q, Reserve %v) / %s", k.GeminiModel, g.e.ErsatzModell, k.GeminiFallbackReserve, k.GeminiEmbeddingModel)
	return g
}

func testdatei(t *testing.T, name string) []byte {
	t.Helper()
	daten, err := os.ReadFile(filepath.Join("..", "..", "..", "testdaten", name))
	if err != nil {
		t.Fatal(err)
	}
	return daten
}

func TestLive_DokumentLesen(t *testing.T) {
	g := liveGemini(t)
	typen := core.AllowedTypes("gesundheit")

	for _, name := range []string{"laborbefund.pdf", "laborbefund.png"} {
		t.Run(name, func(t *testing.T) {
			daten := testdatei(t, name)
			mime, err := core.CheckFile(daten)
			if err != nil {
				t.Fatal(err)
			}

			start := time.Now()
			e, err := g.DokumentLesen(context.Background(), daten, mime, typen)
			dauer := time.Since(start)
			if err != nil {
				t.Fatalf("nach %v: %v", dauer.Round(time.Millisecond), err)
			}
			t.Logf("Dauer %v", dauer.Round(time.Millisecond))

			eintrag, ausgelassen, err := core.CheckExtraction(e, "gesundheit")
			if err != nil {
				t.Fatalf("Prüfung: %v", err)
			}
			if len(ausgelassen) > 0 {
				t.Logf("ausgelassen: %v", ausgelassen)
			}

			if eintrag.Type != "laborbefund" {
				t.Errorf("typ = %q", eintrag.Type)
			}
			if eintrag.DocumentDate == nil || *eintrag.DocumentDate != "2026-10-14" {
				t.Errorf("dokumentdatum = %v (Geburtsdatum verwechselt?)", deref(eintrag.DocumentDate))
			}
			if eintrag.Issuer == nil || !strings.Contains(*eintrag.Issuer, "Labor Muster") {
				t.Errorf("quelle = %v", deref(eintrag.Issuer))
			}
			if !strings.Contains(eintrag.FullText, "Hämoglobin") {
				t.Error("volltext enthält Hämoglobin nicht (Umlaut verloren?)")
			}

			erwartet := map[string][2]string{ // Name → Wert, Einheit
				"Hämoglobin": {"14,2", "g/dl"},
				"CRP":        {"<0,5", "mg/dl"},
			}
			for _, info := range eintrag.Facts {
				t.Logf("  %s: %s = %q %s", info.ID, info.Name, info.Value, deref(info.Unit))
				if w, ok := erwartet[info.Name]; ok {
					if info.Value != w[0] || deref(info.Unit) != w[1] {
						t.Errorf("%s = %q %s, erwartet %q %s", info.Name, info.Value, deref(info.Unit), w[0], w[1])
					}
					delete(erwartet, info.Name)
				}
			}
			for name := range erwartet {
				t.Errorf("Info %s fehlt", name)
			}
		})
	}
}

func TestLive_TextLesen(t *testing.T) {
	g := liveGemini(t)
	text := string(testdatei(t, "laborbefund.txt"))

	start := time.Now()
	e, err := g.TextLesen(context.Background(), text, core.AllowedTypes("gesundheit"))
	dauer := time.Since(start).Round(time.Millisecond)
	if err != nil {
		t.Fatalf("nach %v: %v", dauer, err)
	}
	t.Logf("Dauer %v", dauer)

	// Wie im Dienst: Volltext ist der eingegebene Text.
	e.FullText = text
	eintrag, ausgelassen, err := core.CheckExtraction(e, "gesundheit")
	if err != nil {
		t.Fatalf("Prüfung: %v", err)
	}
	if len(ausgelassen) > 0 {
		t.Logf("ausgelassen: %v", ausgelassen)
	}
	if eintrag.Type != "laborbefund" {
		t.Errorf("typ = %q", eintrag.Type)
	}
	if deref(eintrag.DocumentDate) != "2026-10-14" {
		t.Errorf("dokumentdatum = %v", deref(eintrag.DocumentDate))
	}
	if !strings.Contains(deref(eintrag.Issuer), "Labor Muster") {
		t.Errorf("quelle = %v", deref(eintrag.Issuer))
	}
	hb := false
	for _, info := range eintrag.Facts {
		t.Logf("  %s: %s = %q %s", info.ID, info.Name, info.Value, deref(info.Unit))
		hb = hb || (info.Name == "Hämoglobin" && info.Value == "14,2" && deref(info.Unit) == "g/dl")
	}
	if !hb {
		t.Error("Hämoglobin 14,2 g/dl fehlt")
	}
}

func TestLive_LeereSeiteIstNichtLesbar(t *testing.T) {
	g := liveGemini(t)

	// Ein rein weißes Bild, im Speicher erzeugt.
	bild := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for x := range 400 {
		for y := range 400 {
			bild.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, bild); err != nil {
		t.Fatal(err)
	}

	e, err := g.DokumentLesen(context.Background(), buf.Bytes(), core.MimePNG, core.AllowedTypes("gesundheit"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.CheckExtraction(e, "gesundheit"); !errors.Is(err, core.ErrUnreadable) {
		t.Errorf("leere Seite: erwartet ErrNichtLesbar, bekommen %v (lesbar=%v)", err, e.Readable)
	}
}

func TestLive_Einbetten(t *testing.T) {
	g := liveGemini(t)
	ctx := context.Background()

	abschnitte := []string{
		"Laborbefund vom 14.10.2026\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l",
		"Arztbrief vom 02.09.2026\nVorstellung zur Routinekontrolle. Blutdruck 128/82 mmHg.",
		"Rezept vom 14.10.2026\nBeispielin 100 mg, 1-0-0, 30 Stück",
	}
	vektoren, err := g.AbschnitteEinbetten(ctx, abschnitte)
	if err != nil {
		t.Fatal(err)
	}
	if len(vektoren) != len(abschnitte) {
		t.Fatalf("%d Vektoren für %d Abschnitte", len(vektoren), len(abschnitte))
	}
	for i, v := range vektoren {
		if len(v) != modell.EmbeddingDimensionen {
			t.Errorf("Vektor %d hat %d Dimensionen", i, len(v))
		}
		t.Logf("Vektor %d: Länge (Norm) %.4f", i, norm(v))
	}
	if slices.Equal(vektoren[0], vektoren[1]) {
		t.Fatal("verschiedene Abschnitte ergeben denselben Vektor (zusammengefasst statt einzeln?)")
	}

	fragen := []struct {
		frage    string
		erwartet int // Index des passendsten Abschnitts, -1 = keiner passt
	}{
		{"Welchen Hämoglobinwert hatte ich im Oktober?", 0},
		{"Wie hoch war mein Blutdruck?", 1},
		{"Welches Medikament wurde mir verschrieben?", 2},
		{"Wie wird morgen das Wetter in Salzburg?", -1},
	}
	for _, f := range fragen {
		fv, err := g.FrageEinbetten(ctx, f.frage)
		if err != nil {
			t.Fatal(err)
		}
		bester, besterWert := -1, -2.0
		var werte []string
		for i, v := range vektoren {
			c := kosinus(fv, v)
			werte = append(werte, formatWerte(c))
			if c > besterWert {
				bester, besterWert = i, c
			}
		}
		t.Logf("%-46q → %s", f.frage, strings.Join(werte, " | "))
		if f.erwartet >= 0 && bester != f.erwartet {
			t.Errorf("%q: bester Abschnitt %d, erwartet %d", f.frage, bester, f.erwartet)
		}
	}
}

// formatWerte zeigt Kosinus-Ähnlichkeit und den Score, den Atlas Vector
// Search bei similarity "cosine" meldet: (1 + cos) / 2.
func formatWerte(c float64) string {
	return fmt.Sprintf("cos %.3f / Atlas %.3f", c, (1+c)/2)
}

func kosinus(a, b []float32) float64 {
	var s, na, nb float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return s / (math.Sqrt(na) * math.Sqrt(nb))
}

func norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}
