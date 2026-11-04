// testdaten fügt synthetische Einträge in die Datenbank ein, damit die
// Zeitleiste in der App ohne Upload und ohne Gemini getestet werden kann.
// Alle Daten sind erfunden.
//
//	go run ./cmd/testdaten            Testdaten einfügen (vorhandene werden ersetzt)
//	go run ./cmd/testdaten -entfernen Testdaten wieder löschen
//
// Die Testeinträge haben feste IDs. Darum kann man das Programm beliebig oft
// ausführen: Es löscht zuerst die eigenen Einträge und fügt sie neu ein.
// Andere Einträge in der Datenbank bleiben unberührt.
//
// Die Einträge sehen aus wie Text-Importe: Die Originaldatei ist der Volltext
// als text.txt in GridFS, damit GET /eintraege/{id}/datei auch für sie
// funktioniert. Abschnitte und Embeddings gibt es für Testdaten nicht; sie
// tauchen also in der Suche (POST /fragen) nicht auf.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/konfig"
	"lifelog/backend/internal/speicher"
)

func main() {
	entfernen := flag.Bool("entfernen", false, "Testdaten nur löschen, nicht neu einfügen")
	flag.Parse()

	if err := ausfuehren(*entfernen); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

func ausfuehren(nurEntfernen bool) error {
	if err := konfig.DotenvLaden(); err != nil {
		return err
	}
	k, err := konfig.Laden(os.Getenv)
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

	if err := db.IndexeAnlegen(ctx); err != nil {
		return err
	}

	// Entfernen in der Reihenfolge aus API.md 3.7: Datei vor Eintrag.
	eintraege := testeintraege()
	for _, e := range eintraege {
		if err := db.DateiLoeschen(ctx, e.Datei.ID); err != nil {
			return err
		}
		if _, err := db.EintragLoeschen(ctx, e.ID); err != nil {
			return err
		}
	}
	if nurEntfernen {
		fmt.Printf("%d Testeinträge entfernt (Datenbank %q).\n", len(eintraege), k.MongoDB)
		return nil
	}

	// Einfügen wie beim Upload: Datei zuerst, Eintrag zuletzt.
	for _, e := range eintraege {
		if err := db.DateiSpeichern(ctx, e.Datei.ID, e.Datei.Dateiname, e.Datei.MimeTyp, []byte(e.Volltext)); err != nil {
			return err
		}
		if err := db.EintragSpeichern(ctx, e); err != nil {
			return err
		}
	}
	fmt.Printf("%d Testeinträge eingefügt (Datenbank %q).\n", len(eintraege), k.MongoDB)
	return nil
}

func zeiger(s string) *string { return &s }

func utc(text string) time.Time {
	t, err := time.Parse(time.RFC3339, text)
	if err != nil {
		panic(err)
	}
	return t
}

// textEintrag baut einen Eintrag, als wäre volltext per Text-Import gekommen.
// Hash und Größe stimmen mit dem Text überein.
func textEintrag(id, dateiID, typ string, datum, quelle *string, importdatum, volltext string, infos []kern.Info) kern.Eintrag {
	hash := sha256.Sum256([]byte(volltext))
	return kern.Eintrag{
		ID:            id,
		Modul:         "gesundheit",
		Typ:           typ,
		Dokumentdatum: datum,
		Importdatum:   utc(importdatum),
		Quelle:        quelle,
		Volltext:      volltext,
		Infos:         infos,
		Datei: kern.Datei{
			ID:        dateiID,
			Dateiname: "text.txt",
			MimeTyp:   "text/plain; charset=utf-8",
			Groesse:   int64(len(volltext)),
			Sha256:    hex.EncodeToString(hash[:]),
		},
		Korrekturen: []kern.Korrektur{},
	}
}

// testeintraege deckt die Fälle der Zeitleiste ab: verschiedene Typen, ein
// Eintrag ohne Datum und ohne Quelle, zwei Einträge mit gleichem Datum.
func testeintraege() []kern.Eintrag {
	return []kern.Eintrag{
		textEintrag(
			"6727a1f3c2b4e5d6f7a8b9c0", "6727a1f3c2b4e5d6f7a8b9c1", "laborbefund",
			zeiger("2026-10-14"), zeiger("Labor Muster GmbH, Salzburg"), "2026-11-03T18:42:07Z",
			"Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\n"+
				"Patient: Max Mustermann, geb. 01.01.1990\n\n"+
				"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\n"+
				"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
			[]kern.Info{
				{ID: "i1", Name: "Hämoglobin", Wert: "14,2", Einheit: zeiger("g/dl")},
				{ID: "i2", Name: "Leukozyten", Wert: "6,8", Einheit: zeiger("G/l")},
				{ID: "i3", Name: "CRP", Wert: "<0,5", Einheit: zeiger("mg/dl")},
				{ID: "i4", Name: "Befundart", Wert: "Blutbild klein"},
			},
		),
		textEintrag(
			"6727a20bc2b4e5d6f7a8b9d4", "6727a20bc2b4e5d6f7a8b9d5", "arztbrief",
			zeiger("2026-09-02"), zeiger("Dr. Erika Beispiel, Allgemeinmedizin"), "2026-11-03T18:50:12Z",
			"Dr. Erika Beispiel, Allgemeinmedizin\nBeispielgasse 7, 5020 Salzburg\n\n"+
				"Arztbrief vom 02.09.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\n"+
				"Vorstellung zur Routinekontrolle. Blutdruck 128/82 mmHg.\n"+
				"Kontrolle in 6 Monaten vereinbart.",
			[]kern.Info{
				{ID: "i1", Name: "Blutdruck", Wert: "128/82", Einheit: zeiger("mmHg")},
				{ID: "i2", Name: "Nächste Kontrolle", Wert: "in 6 Monaten"},
			},
		),
		textEintrag(
			"6727a2a9c2b4e5d6f7a8b9e7", "6727a2a9c2b4e5d6f7a8b9e8", "sonstiges",
			nil, nil, "2026-11-03T19:01:44Z",
			"Notiz ohne Datum und ohne Absender.\nImpfpass liegt in der Schublade.",
			[]kern.Info{},
		),
		textEintrag(
			"6727a2f0c2b4e5d6f7a8b9f2", "6727a2f0c2b4e5d6f7a8b9f3", "rezept",
			zeiger("2026-10-14"), zeiger("Dr. Erika Beispiel, Allgemeinmedizin"), "2026-11-03T19:10:00Z",
			"Dr. Erika Beispiel, Allgemeinmedizin\n\nRezept vom 14.10.2026\n"+
				"Patient: Max Mustermann\n\nBeispielin 100 mg, 1-0-0, 30 Stück",
			[]kern.Info{
				{ID: "i1", Name: "Präparat", Wert: "Beispielin 100 mg"},
				{ID: "i2", Name: "Dosierung", Wert: "1-0-0"},
				{ID: "i3", Name: "Menge", Wert: "30", Einheit: zeiger("Stück")},
			},
		),
	}
}
