// testdaten fügt synthetische Einträge in die Datenbank ein, damit Zeitleiste
// und Suche ohne Upload und ohne Vision-LLM getestet werden können. Alle
// Daten sind erfunden (internal/beispieldaten).
//
//	go run ./cmd/testdaten                   einfügen, mit Abschnitten und Embeddings (Gemini)
//	go run ./cmd/testdaten -ohne-abschnitte  einfügen ohne Gemini (Suche findet sie dann nicht)
//	go run ./cmd/testdaten -entfernen        wieder löschen
//	go run ./cmd/testdaten -wegwerf          einen Wegwerf-Eintrag zum Testen von DELETE anlegen
//
// Die Testeinträge haben feste IDs. Darum kann man das Programm beliebig oft
// ausführen: Es löscht zuerst die eigenen Einträge und fügt sie neu ein.
// Andere Einträge in der Datenbank bleiben unberührt.
//
// -wegwerf legt bei jedem Aufruf einen weiteren Eintrag mit neuer ID an
// (ohne Gemini, siehe beispieldaten.Wegwerf) und gibt die ID aus. Nur diese
// ID zum Ausprobieren von DELETE verwenden.
//
// Die Einträge sehen aus wie Text-Importe: Die Originaldatei ist der Volltext
// als text.txt in GridFS. Die Abschnitte entstehen wie beim Upload
// (gleiches Zerlegen, gleiches Embedding-Modell), damit POST /fragen und
// cmd/kalibrieren sie finden.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"lifelog/backend/internal/beispieldaten"
	"lifelog/backend/internal/config"
	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/modell/gemini"
	"lifelog/backend/internal/speicher"
)

func main() {
	entfernen := flag.Bool("entfernen", false, "Testdaten nur löschen, nicht neu einfügen")
	ohneAbschnitte := flag.Bool("ohne-abschnitte", false, "keine Abschnitte und Embeddings anlegen (ohne Gemini-Aufruf)")
	wegwerf := flag.Bool("wegwerf", false, "nur einen Wegwerf-Eintrag mit neuer ID anlegen (zum Testen von DELETE)")
	flag.Parse()

	var err error
	if *wegwerf {
		err = wegwerfAnlegen()
	} else {
		err = ausfuehren(*entfernen, *ohneAbschnitte)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

func ausfuehren(nurEntfernen, ohneAbschnitte bool) error {
	if err := config.DotenvLaden(); err != nil {
		return err
	}
	k, err := config.Laden(os.Getenv)
	if err != nil {
		return err
	}

	// 60 s: Embeddings bis 20 s plus Datenbank über das Netz.
	ctx, abbrechen := context.WithTimeout(context.Background(), 60*time.Second)
	defer abbrechen()

	eintraege := beispieldaten.Eintraege()

	// Embeddings zuerst: Scheitert Gemini, bleibt die Datenbank, wie sie war.
	abschnitte := map[string][]kern.Abschnitt{}
	if !nurEntfernen && !ohneAbschnitte {
		abschnitte, err = einbetten(ctx, k, eintraege)
		if err != nil {
			return fmt.Errorf("%w\nOhne Gemini geht es mit -ohne-abschnitte (dann findet die Suche die Testdaten nicht)", err)
		}
	}

	db, err := speicher.Verbinden(ctx, k.MongoURI, k.MongoDB)
	if err != nil {
		return err
	}
	defer db.Trennen(context.Background())

	if err := db.IndexeAnlegen(ctx); err != nil {
		return err
	}

	// Entfernen in der Reihenfolge aus API.md 3.7: Abschnitte, Datei, Eintrag.
	for _, e := range eintraege {
		if err := db.AbschnitteLoeschen(ctx, e.ID); err != nil {
			return err
		}
		if err := db.DateiLoeschen(ctx, e.Datei.ID); err != nil {
			return err
		}
		if _, err := db.EintragLoeschen(ctx, e.ID); err != nil {
			return err
		}
	}
	if nurEntfernen {
		fmt.Printf("%d Testeinträge samt Abschnitten entfernt (Datenbank %q).\n", len(eintraege), k.MongoDB)
		return nil
	}

	// Einfügen wie beim Upload: Datei, Abschnitte, Eintrag zuletzt.
	anzahl := 0
	for _, e := range eintraege {
		if err := db.DateiSpeichern(ctx, e.Datei.ID, e.Datei.Dateiname, e.Datei.MimeTyp, []byte(e.Volltext)); err != nil {
			return err
		}
		if err := db.AbschnitteSpeichern(ctx, abschnitte[e.ID]); err != nil {
			return err
		}
		if err := db.EintragSpeichern(ctx, e); err != nil {
			return err
		}
		anzahl += len(abschnitte[e.ID])
	}
	fmt.Printf("%d Testeinträge mit %d Abschnitten eingefügt (Datenbank %q).\n", len(eintraege), anzahl, k.MongoDB)
	if anzahl > 0 {
		fmt.Println("Atlas braucht einige Sekunden, bis neue Abschnitte in der Vektorsuche auftauchen.")
	}
	return nil
}

// einbetten zerlegt alle Testeinträge und holt die Embeddings in einem
// Durchgang (AbschnitteEinbetten teilt selbst in Stapel). Ergebnis nach
// Eintrag-ID.
func einbetten(ctx context.Context, k config.Konfig, eintraege []kern.Eintrag) (map[string][]kern.Abschnitt, error) {
	g, err := gemini.Neu(ctx, gemini.Einstellungen{
		Schluessel:      k.GeminiKey,
		Modell:          k.GeminiModell,
		EmbeddingModell: k.GeminiEmbeddingModell,
	})
	if err != nil {
		return nil, err
	}

	var alle []kern.Abschnitt
	var texte []string
	for _, e := range eintraege {
		for _, a := range beispieldaten.Abschnitte(e) {
			alle = append(alle, a)
			texte = append(texte, a.Text)
		}
	}
	vektoren, err := g.AbschnitteEinbetten(ctx, texte)
	if err != nil {
		return nil, fmt.Errorf("Embeddings (%s): %w", k.GeminiEmbeddingModell, err)
	}
	if len(vektoren) != len(texte) {
		return nil, fmt.Errorf("%d Embeddings für %d Abschnitte", len(vektoren), len(texte))
	}

	ergebnis := map[string][]kern.Abschnitt{}
	for i, a := range alle {
		a.Embedding = vektoren[i]
		ergebnis[a.EintragID] = append(ergebnis[a.EintragID], a)
	}
	return ergebnis, nil
}

// wegwerfAnlegen speichert einen Wegwerf-Eintrag wie beim Upload (Datei,
// Abschnitte, Eintrag zuletzt) und gibt seine ID aus. Die festen
// Testeinträge und alle anderen Einträge bleiben unberührt.
func wegwerfAnlegen() error {
	if err := config.DotenvLaden(); err != nil {
		return err
	}
	k, err := config.Laden(os.Getenv)
	if err != nil {
		return err
	}
	ctx, abbrechen := context.WithTimeout(context.Background(), 30*time.Second)
	defer abbrechen()

	db, err := speicher.Verbinden(ctx, k.MongoURI, k.MongoDB)
	if err != nil {
		return err
	}
	defer db.Trennen(context.Background())

	e, abschnitte := beispieldaten.Wegwerf(speicher.NeueID(), speicher.NeueID(), time.Now())
	if err := db.DateiSpeichern(ctx, e.Datei.ID, e.Datei.Dateiname, e.Datei.MimeTyp, []byte(e.Volltext)); err != nil {
		return err
	}
	if err := db.AbschnitteSpeichern(ctx, abschnitte); err != nil {
		return err
	}
	if err := db.EintragSpeichern(ctx, e); err != nil {
		return err
	}
	fmt.Printf("Wegwerf-Eintrag mit %d Abschnitt(en) angelegt (Datenbank %q):\n%s\n", len(abschnitte), k.MongoDB, e.ID)
	return nil
}
