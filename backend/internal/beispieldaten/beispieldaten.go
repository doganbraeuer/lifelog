// Package beispieldaten enthält die erfundenen Testeinträge. Sie werden von
// cmd/testdaten (einfügen, entfernen) und cmd/kalibrieren (Fragen, deren
// Antwort in einem bestimmten Eintrag steht) gemeinsam benutzt. Alle Daten
// sind erfunden.
package beispieldaten

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"lifelog/backend/internal/kern"
)

// Feste IDs der Testeinträge. Fest, damit cmd/testdaten genau diese wieder
// löschen kann und cmd/kalibrieren weiß, welcher Eintrag die Antwort enthält.
const (
	Laborbefund = "6727a1f3c2b4e5d6f7a8b9c0"
	Arztbrief   = "6727a20bc2b4e5d6f7a8b9d4"
	Notiz       = "6727a2a9c2b4e5d6f7a8b9e7"
	Rezept      = "6727a2f0c2b4e5d6f7a8b9f2"
)

// Eintraege deckt die Fälle der Zeitleiste ab: verschiedene Typen, ein
// Eintrag ohne Datum und ohne Quelle, zwei Einträge mit gleichem Datum.
//
// Die Einträge sehen aus wie Text-Importe: Die Originaldatei ist der
// Volltext als text.txt.
func Eintraege() []kern.Eintrag {
	return []kern.Eintrag{
		textEintrag(
			Laborbefund, "6727a1f3c2b4e5d6f7a8b9c1", "laborbefund",
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
			Arztbrief, "6727a20bc2b4e5d6f7a8b9d5", "arztbrief",
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
			Notiz, "6727a2a9c2b4e5d6f7a8b9e8", "sonstiges",
			nil, nil, "2026-11-03T19:01:44Z",
			"Notiz ohne Datum und ohne Absender.\nImpfpass liegt in der Schublade.",
			[]kern.Info{},
		),
		textEintrag(
			Rezept, "6727a2f0c2b4e5d6f7a8b9f3", "rezept",
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

// Abschnitte zerlegt den Volltext eines Eintrags wie beim Upload
// (dienst.Hochladen): gleiche Obergrenze, Nr ab 1, Modul des Eintrags. Die
// Embeddings fehlen noch; die setzt der Aufrufer.
func Abschnitte(e kern.Eintrag) []kern.Abschnitt {
	texte := kern.ZerlegeInAbschnitte(e.Volltext, kern.AbschnittMaxZeichen)
	abschnitte := make([]kern.Abschnitt, len(texte))
	for i, t := range texte {
		abschnitte[i] = kern.Abschnitt{EintragID: e.ID, Modul: e.Modul, Nr: i + 1, Text: t}
	}
	return abschnitte
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
