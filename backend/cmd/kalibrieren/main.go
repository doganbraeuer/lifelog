// kalibrieren misst, wie gut die Schwelle kern.MinScore passende von
// fachfremden Fragen trennt. Es stellt eine feste Liste synthetischer Fragen
// (fragen.go), bettet jede ein wie POST /fragen und gibt pro Frage den
// besten Score der Vektorsuche aus. Das Sprachmodell (Antworter) wird NICHT
// aufgerufen; es kostet nur ein Embedding pro Frage.
//
// Vorher einmal die Testdaten mit Abschnitten einfügen:
//
//	go run ./cmd/testdaten
//	go run ./cmd/kalibrieren                  # mit der Schwelle kern.MinScore
//	go run ./cmd/kalibrieren -schwelle 0.82   # andere Schwelle durchrechnen
//
// Gesucht wird im Modul gesundheit über alle Abschnitte, also auch über
// hochgeladene Einträge (in der Tabelle als "anderer" markiert).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"lifelog/backend/internal/beispieldaten"
	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/konfig"
	"lifelog/backend/internal/modell"
	"lifelog/backend/internal/modell/gemini"
	"lifelog/backend/internal/speicher"
)

func main() {
	schwelle := flag.Float64("schwelle", kern.MinScore, "Schwelle, mit der ausgewertet wird (Atlas-Score 0 … 1)")
	flag.Parse()

	if err := ausfuehren(*schwelle); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

func ausfuehren(schwelle float64) error {
	if schwelle <= 0 || schwelle > 1 {
		return fmt.Errorf("-schwelle muss zwischen 0 und 1 liegen")
	}
	if err := konfig.DotenvLaden(); err != nil {
		return err
	}
	k, err := konfig.Laden(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stopp := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopp()
	ctx, abbrechen := context.WithTimeout(ctx, 3*time.Minute)
	defer abbrechen()

	db, err := speicher.Verbinden(ctx, k.MongoURI, k.MongoDB)
	if err != nil {
		return err
	}
	defer db.Trennen(context.Background())

	status, gefunden, err := db.VektorindexStatus(ctx)
	if err != nil {
		return err
	}
	if !gefunden || status != "READY" {
		return fmt.Errorf("Vektorindex %q ist nicht bereit (Status %q); einmal den Server starten oder backend/README.md, Abschnitt Vektorindex",
			speicher.VektorindexName, status)
	}

	// Nur der Einbetter wird benutzt; Lese- und Antwortmodell bleiben unberührt.
	g, err := gemini.Neu(ctx, gemini.Einstellungen{
		Schluessel:      k.GeminiKey,
		Modell:          k.GeminiModell,
		EmbeddingModell: k.GeminiEmbeddingModell,
	})
	if err != nil {
		return err
	}

	fragen := kalibrierfragen()
	fmt.Printf("%d Fragen, Embedding-Modell %s, Datenbank %q, Top %d\n\n",
		len(fragen), k.GeminiEmbeddingModell, k.MongoDB, kern.AnzahlTreffer)
	fmt.Println(" Nr  Gruppe     Bester  Treffer      Richtig  Frage")

	typen := map[string]string{}
	for _, e := range beispieldaten.Eintraege() {
		typen[e.ID] = e.Typ
	}

	messungen := make([]messung, 0, len(fragen))
	testdatenGetroffen := false
	for i, f := range fragen {
		vektor, err := g.FrageEinbetten(ctx, f.Text)
		if err != nil {
			if errors.Is(err, modell.ErrNichtErreichbar) {
				return fmt.Errorf("Frage %d: Embedding-Modell nicht erreichbar, später noch einmal: %w", i+1, err)
			}
			return fmt.Errorf("Frage %d: %w", i+1, err)
		}
		treffer, err := db.AehnlicheAbschnitte(ctx, vektor, "gesundheit", kern.AnzahlTreffer)
		if err != nil {
			return fmt.Errorf("Frage %d: %w", i+1, err)
		}
		for _, t := range treffer {
			if _, ok := typen[t.EintragID]; ok {
				testdatenGetroffen = true
			}
		}

		m := messen(f, treffer)
		messungen = append(messungen, m)
		fmt.Printf("%3d  %-9s  %s   %-12s %-8s %s\n",
			i+1, f.Gruppe, zahl(m.Bester), eintragName(typen, m.BesterEintrag), markierung(m), f.Text)
	}

	fmt.Println()
	fmt.Print(auswerten(messungen, schwelle).Zusammenfassung())
	if !testdatenGetroffen {
		fmt.Println("\nHinweis: Kein Treffer stammt aus den Testdaten. Erst `go run ./cmd/testdaten` (mit Abschnitten),",
			"dann einige Sekunden warten, bis Atlas sie in den Index übernommen hat.")
	}
	return nil
}

func eintragName(typen map[string]string, id string) string {
	if id == "" {
		return "–"
	}
	if typ, ok := typen[id]; ok {
		return typ
	}
	return "anderer"
}

// markierung: ✓ richtiger Eintrag oben, sonst ✗ und der Score des richtigen
// Eintrags (– wenn er nicht unter den Treffern ist). Nur bei passend.
func markierung(m messung) string {
	switch {
	case m.Frage.Gruppe != passend:
		return ""
	case m.RichtigOben():
		return "✓"
	case m.Erwartet == 0:
		return "✗ –"
	default:
		return "✗ " + zahl(m.Erwartet)
	}
}
