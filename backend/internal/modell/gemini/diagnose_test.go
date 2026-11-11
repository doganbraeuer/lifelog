package gemini

// Diagnose: ein einzelner Leseaufruf, ohne Wiederholung und ohne Ersatz,
// mit Dauer, Abbruchgrund und Token-Verbrauch.
//
//	LIFELOG_DIAGNOSE=1 LIFELOG_DIAGNOSE_VARIANT=voll go test ./internal/modell/gemini/ -run Diagnose -v
//
// Varianten (bauen aufeinander auf):
//
//	voll         genau die Anfrage des Servers (leseAnfrage)
//	ohne-schema  ohne JSON-Schema und ohne JSON-Modus ("ohne Structured Output")
//	ohne-system  zusätzlich ohne Systemanweisung (wie der Playground-Test)
//
// Weitere Schalter, um andere Modelle zu vergleichen:
//
//	LIFELOG_DIAGNOSE_MODEL=gemini-3.6-flash   statt GEMINI_MODEL
//	LIFELOG_DIAGNOSE_FILE=laborbefund.png     statt laborbefund.pdf (oder ein
//	                                           absoluter Pfad, nur synthetisch!)
//	LIFELOG_DIAGNOSE_THINKING=aus              ohne ThinkingConfig (für Modelle
//	                                           ohne thinkingLevel)
//
// Schickt nur synthetische Dateien aus testdaten/.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
)

const diagnoseLimit = 120 * time.Second

func TestDiagnose_Lesen(t *testing.T) {
	if os.Getenv("LIFELOG_DIAGNOSE") != "1" {
		t.Skip("Diagnose übersprungen (LIFELOG_DIAGNOSE=1 setzen)")
	}
	variante := os.Getenv("LIFELOG_DIAGNOSE_VARIANT")
	if variante == "" {
		variante = "voll"
	}

	if err := config.FindDotenv(); err != nil {
		t.Fatal(err)
	}
	k, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Neu(context.Background(), Einstellungen{
		Schluessel: k.GeminiKey, Modell: k.GeminiModel, EmbeddingModell: k.GeminiEmbeddingModel,
	})
	if err != nil {
		t.Fatal(err)
	}

	modellName := k.GeminiModel
	if m := os.Getenv("LIFELOG_DIAGNOSE_MODEL"); m != "" {
		modellName = m
	}
	dateiname := os.Getenv("LIFELOG_DIAGNOSE_FILE")
	if dateiname == "" {
		dateiname = "laborbefund.pdf"
	}
	// Ein absoluter Pfad erlaubt umgewandelte Varianten außerhalb des Repos
	// (z. B. dieselbe Seite als JPEG); sonst eine Datei aus testdaten/.
	var daten []byte
	if filepath.IsAbs(dateiname) {
		if daten, err = os.ReadFile(dateiname); err != nil {
			t.Fatal(err)
		}
	} else {
		daten = testdatei(t, dateiname)
	}
	mimeTyp, err := core.ErkenneDateityp(daten)
	if err != nil {
		t.Fatal(err)
	}

	inhalte, cfg := leseAnfrage(daten, mimeTyp, core.ErlaubteTypen("gesundheit"))
	thinking := string(cfg.ThinkingConfig.ThinkingLevel)
	if os.Getenv("LIFELOG_DIAGNOSE_THINKING") == "aus" {
		cfg.ThinkingConfig, thinking = nil, "aus"
	}
	switch variante {
	case "voll":
	case "ohne-schema":
		cfg.ResponseJsonSchema, cfg.ResponseMIMEType = nil, ""
	case "ohne-system":
		cfg.ResponseJsonSchema, cfg.ResponseMIMEType = nil, ""
		cfg.SystemInstruction = nil
	default:
		t.Fatalf("unbekannte Variante %q", variante)
	}
	t.Logf("Modell %s, Variante %s, Thinking %s, Limit %v, %s (%s, %d Bytes)",
		modellName, variante, thinking, diagnoseLimit, dateiname, mimeTyp, len(daten))

	ctx, abbrechen := context.WithTimeout(context.Background(), diagnoseLimit)
	defer abbrechen()

	start := time.Now()
	resp, err := g.erzeugen(ctx, modellName, inhalte, cfg)
	dauer := time.Since(start).Round(time.Millisecond)

	if err != nil {
		t.Fatalf("nach %v: %v", dauer, uebersetzeFehler(err, k.GeminiKey))
	}

	t.Logf("Dauer: %v", dauer)
	t.Logf("Modellversion: %s", resp.ModelVersion)
	if len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
		t.Logf("Abbruchgrund: %q", resp.Candidates[0].FinishReason)
	}
	if fb := resp.PromptFeedback; fb != nil && fb.BlockReason != "" {
		t.Logf("Blockiert: %s", fb.BlockReason)
	}
	if u := resp.UsageMetadata; u != nil {
		t.Logf("Tokens: Eingabe %d, Nachdenken %d, Ausgabe %d, gesamt %d",
			u.PromptTokenCount, u.ThoughtsTokenCount, u.CandidatesTokenCount, u.TotalTokenCount)
	}

	text := resp.Text()
	t.Logf("Antwort: %d Zeichen", utf8.RuneCountInString(text))
	t.Logf("Anfang:\n%s", kuerzen(text, 400, false))
	if utf8.RuneCountInString(text) > 800 {
		t.Logf("Ende:\n%s", kuerzen(text, 300, true))
	}

	if variante == "voll" {
		e, err := erkennungAusAntwort(resp)
		if err != nil {
			t.Errorf("Antwort nicht auswertbar: %v", err)
			return
		}
		eintrag, ausgelassen, err := core.PruefeErkennung(e, "gesundheit")
		if err != nil {
			t.Errorf("Prüfung: %v", err)
			return
		}
		t.Logf("Geprüft: typ=%s datum=%s quelle=%s, %d Infos, ausgelassen %v",
			eintrag.Typ, deref(eintrag.Dokumentdatum), deref(eintrag.Quelle), len(eintrag.Infos), ausgelassen)
		// Die Werte sind synthetisch (testdaten/), dürfen also ins Testlog.
		for _, i := range eintrag.Infos {
			t.Logf("  %s = %q %s", i.Name, i.Wert, deref(i.Einheit))
		}
	}
}

// kuerzen liefert die ersten (oder letzten) n Zeichen.
func kuerzen(s string, n int, ende bool) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	if ende {
		return "…" + string(r[len(r)-n:])
	}
	return string(r[:n]) + "…"
}
