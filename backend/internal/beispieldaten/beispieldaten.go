// Package beispieldaten enthält die erfundenen Testeinträge. Sie werden von
// cmd/testdaten (einfügen, entfernen) und cmd/kalibrieren (Fragen, deren
// Antwort in einem bestimmten Eintrag steht) gemeinsam benutzt. Alle Daten
// sind erfunden.
package beispieldaten

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
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
func Eintraege() []core.Entry {
	return []core.Entry{
		textEintrag(
			Laborbefund, "6727a1f3c2b4e5d6f7a8b9c1", "laborbefund",
			zeiger("2026-10-14"), zeiger("Labor Muster GmbH, Salzburg"), "2026-11-03T18:42:07Z",
			"Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\n"+
				"Patient: Max Mustermann, geb. 01.01.1990\n\n"+
				"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\n"+
				"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
			[]core.Fact{
				{ID: "i1", Name: "Hämoglobin", Value: "14,2", Unit: zeiger("g/dl")},
				{ID: "i2", Name: "Leukozyten", Value: "6,8", Unit: zeiger("G/l")},
				{ID: "i3", Name: "CRP", Value: "<0,5", Unit: zeiger("mg/dl")},
				{ID: "i4", Name: "Befundart", Value: "Blutbild klein"},
			},
		),
		textEintrag(
			Arztbrief, "6727a20bc2b4e5d6f7a8b9d5", "arztbrief",
			zeiger("2026-09-02"), zeiger("Dr. Erika Beispiel, Allgemeinmedizin"), "2026-11-03T18:50:12Z",
			"Dr. Erika Beispiel, Allgemeinmedizin\nBeispielgasse 7, 5020 Salzburg\n\n"+
				"Arztbrief vom 02.09.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\n"+
				"Vorstellung zur Routinekontrolle. Blutdruck 128/82 mmHg.\n"+
				"Kontrolle in 6 Monaten vereinbart.",
			[]core.Fact{
				{ID: "i1", Name: "Blutdruck", Value: "128/82", Unit: zeiger("mmHg")},
				{ID: "i2", Name: "Nächste Kontrolle", Value: "in 6 Monaten"},
			},
		),
		textEintrag(
			Notiz, "6727a2a9c2b4e5d6f7a8b9e8", "sonstiges",
			nil, nil, "2026-11-03T19:01:44Z",
			"Notiz ohne Datum und ohne Absender.\nImpfpass liegt in der Schublade.",
			[]core.Fact{},
		),
		textEintrag(
			Rezept, "6727a2f0c2b4e5d6f7a8b9f3", "rezept",
			zeiger("2026-10-14"), zeiger("Dr. Erika Beispiel, Allgemeinmedizin"), "2026-11-03T19:10:00Z",
			"Dr. Erika Beispiel, Allgemeinmedizin\n\nRezept vom 14.10.2026\n"+
				"Patient: Max Mustermann\n\nBeispielin 100 mg, 1-0-0, 30 Stück",
			[]core.Fact{
				{ID: "i1", Name: "Präparat", Value: "Beispielin 100 mg"},
				{ID: "i2", Name: "Dosierung", Value: "1-0-0"},
				{ID: "i3", Name: "Menge", Value: "30", Unit: zeiger("Stück")},
			},
		),
	}
}

// Abschnitte zerlegt den Volltext eines Eintrags wie beim Upload
// (dienst.Hochladen): gleiche Obergrenze, Nr ab 1, Modul des Eintrags. Die
// Embeddings fehlen noch; die setzt der Aufrufer.
func Abschnitte(e core.Entry) []core.Chunk {
	texte := core.SplitIntoChunks(e.FullText, core.ChunkMaxChars)
	abschnitte := make([]core.Chunk, len(texte))
	for i, t := range texte {
		abschnitte[i] = core.Chunk{EntryID: e.ID, Module: e.Module, Position: i + 1, Text: t}
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
func textEintrag(id, dateiID, typ string, datum, quelle *string, importdatum, volltext string, infos []core.Fact) core.Entry {
	hash := sha256.Sum256([]byte(volltext))
	return core.Entry{
		ID:           id,
		Module:       "gesundheit",
		Type:         typ,
		DocumentDate: datum,
		ImportedAt:   utc(importdatum),
		Issuer:       quelle,
		FullText:     volltext,
		Facts:        infos,
		File: core.File{
			ID:       dateiID,
			FileName: "text.txt",
			MimeType: "text/plain; charset=utf-8",
			Size:     int64(len(volltext)),
			Sha256:   hex.EncodeToString(hash[:]),
		},
		Corrections: []core.Correction{},
	}
}

// Wegwerf baut einen erfundenen Eintrag zum Ausprobieren von DELETE
// (cmd/testdaten -wegwerf). Die IDs vergibt der Aufrufer neu, damit nie
// ein fester Testeintrag oder ein hochgeladenes Dokument getroffen wird.
// Der Zeitpunkt steht im Text, so hat jeder Wegwerf-Eintrag einen anderen
// Hash und ist kein Duplikat.
//
// Die Abschnitte haben statt eines echten Embeddings einen festen
// Einheitsvektor (1, 0, 0, …): kein Gemini-Aufruf. Sein Score zu echten
// Fragen liegt um 0,5, weit unter core.MinScore; die Suche zeigt ihn also
// nicht an.
func Wegwerf(id, dateiID string, jetzt time.Time) (core.Entry, []core.Chunk) {
	jetzt = jetzt.UTC().Truncate(time.Millisecond)
	volltext := "Wegwerf-Eintrag zum Testen des Löschens.\n\nAngelegt am " +
		jetzt.Format(time.RFC3339Nano) + ". Enthält keine Gesundheitsdaten."
	hash := sha256.Sum256([]byte(volltext))
	e := core.Entry{
		ID:         id,
		Module:     "gesundheit",
		Type:       "sonstiges",
		ImportedAt: jetzt,
		Issuer:     zeiger("Wegwerf-Test"),
		FullText:   volltext,
		Facts:      []core.Fact{},
		File: core.File{
			ID:       dateiID,
			FileName: "wegwerf.txt",
			MimeType: "text/plain; charset=utf-8",
			Size:     int64(len(volltext)),
			Sha256:   hex.EncodeToString(hash[:]),
		},
		Corrections: []core.Correction{},
	}
	abschnitte := Abschnitte(e)
	for i := range abschnitte {
		v := make([]float32, llm.EmbeddingDimensions)
		v[0] = 1
		abschnitte[i].Embedding = v
	}
	return e, abschnitte
}
