// kalibrieren misst, wie gut die Schwelle kern.MinScore passende von
// fachfremden Fragen trennt. Es stellt eine feste Liste synthetischer Fragen
// (fragen.go), bettet jede ein wie POST /fragen und gibt pro Frage den
// besten Score der Vektorsuche aus. Das Sprachmodell (Antworter) wird NICHT
// aufgerufen; es kostet nur ein Embedding pro Frage.
//
// Vorher müssen die Testdaten (go run ./cmd/testdaten, mit Abschnitten) und
// die synthetischen Demo-Daten (Laborbefunde, Orthopädie, FSME, Notizen;
// hochgeladen) in der Datenbank liegen. Die Demo-Einträge findet das
// Programm über Typ und Dokumentdatum; fehlt einer oder gibt es ihn doppelt,
// bricht es vor dem ersten Embedding ab. In der Datenbank wird nur gelesen.
//
//	go run ./cmd/kalibrieren                  # mit der Schwelle kern.MinScore
//	go run ./cmd/kalibrieren -schwelle 0.82   # andere Schwelle durchrechnen
//
// Gesucht wird im Modul gesundheit über alle Abschnitte. Die Tabelle nennt
// beim besten Treffer Typ und Dokumentdatum (MM-TT).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/konfig"
	"lifelog/backend/internal/modell"
	"lifelog/backend/internal/modell/gemini"
	"lifelog/backend/internal/speicher"
	"strings"
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

	// Nur lesend: Ziele (Typ + Dokumentdatum oder Testdaten-ID) zu IDs auflösen,
	// bevor das erste Embedding Kontingent kostet.
	liste, err := db.ListeKurz(ctx, "gesundheit")
	if err != nil {
		return err
	}
	fragen := kalibrierfragen()
	ids, err := aufloesen(fragen, liste)
	if err != nil {
		return fmt.Errorf("Einträge zu den Fragen fehlen oder sind nicht eindeutig (Testdaten: go run ./cmd/testdaten; Demo-Daten: hochladen):\n%w", err)
	}
	namen := map[string]string{}
	for _, e := range liste {
		namen[e.ID] = kurzname(e)
	}

	fmt.Printf("%d Fragen, %d Einträge, Embedding-Modell %s, Datenbank %q, Top %d\n\n",
		len(fragen), len(liste), k.GeminiEmbeddingModell, k.MongoDB, kern.AnzahlTreffer)
	fmt.Println(" Nr  Gruppe     Bester  Treffer            Richtig        Frage")

	messungen := make([]messung, 0, len(fragen))
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

		erwartet := make([]string, len(f.Erwartet))
		for j, z := range f.Erwartet {
			erwartet[j] = ids[z]
		}
		m := messen(f, erwartet, treffer)
		messungen = append(messungen, m)
		fmt.Printf("%3d  %-9s  %s   %-18s %-14s %s\n",
			i+1, f.Gruppe, zahl(m.Bester), eintragName(namen, m.BesterEintrag), markierung(m), f.Text)
	}

	fmt.Println()
	fmt.Print(auswerten(messungen, schwelle).Zusammenfassung())
	return nil
}

// kurzname für die Tabelle, z. B. "laborbefund 07-22".
func kurzname(e kern.EintragKurz) string {
	if e.Dokumentdatum == nil || len(*e.Dokumentdatum) != 10 {
		return e.Typ + " o. D."
	}
	return e.Typ + " " + (*e.Dokumentdatum)[5:]
}

func eintragName(namen map[string]string, id string) string {
	if id == "" {
		return "–"
	}
	if n, ok := namen[id]; ok {
		return n
	}
	return "unbekannt"
}

// markierung (nur bei passend): ✓ wenn ein erwarteter Eintrag oben steht,
// sonst ✗; dahinter bei mehreren erwarteten Einträgen deren Scores, bei
// einem der Score, wenn er nicht oben steht (– = nicht unter den Treffern).
func markierung(m messung) string {
	if m.Frage.Gruppe != passend {
		return ""
	}
	zeichen := "✗"
	if m.RichtigOben() {
		zeichen = "✓"
	}
	if len(m.Erwartet) == 1 && m.RichtigOben() {
		return zeichen
	}
	werte := make([]string, len(m.Erwartet))
	for i, s := range m.Erwartet {
		if s == 0 {
			werte[i] = "–"
		} else {
			werte[i] = zahl(s)
		}
	}
	return zeichen + " " + strings.Join(werte, "/")
}
