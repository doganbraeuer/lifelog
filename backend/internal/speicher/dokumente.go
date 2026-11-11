package speicher

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

// Die *Dok-Typen beschreiben, wie ein Eintrag in MongoDB aussieht. Sie sind
// von den kern-Typen getrennt, damit die Fachlogik nichts von BSON weiß.
//
// Gespeichert wird:
//   - _id als ObjectId
//   - dokumentdatum als String "YYYY-MM-DD" oder null. Ein Text statt
//     eines BSON-Datums, damit keine Zeitzone das Datum verschieben kann.
//   - importdatum als BSON-Datum (UTC)
//   - Listen immer als Array, nie als null

type eintragDok struct {
	ID            bson.ObjectID  `bson:"_id"`
	Modul         string         `bson:"modul"`
	Typ           string         `bson:"typ"`
	Dokumentdatum *string        `bson:"dokumentdatum"`
	Importdatum   time.Time      `bson:"importdatum"`
	Quelle        *string        `bson:"quelle"`
	Volltext      string         `bson:"volltext"`
	Infos         []infoDok      `bson:"infos"`
	Datei         dateiDok       `bson:"datei"`
	Korrekturen   []korrekturDok `bson:"korrekturen"`
}

type infoDok struct {
	ID    string  `bson:"id"`
	Name  string  `bson:"name"`
	Value string  `bson:"wert"`
	Unit  *string `bson:"einheit,omitempty"`
}

type dateiDok struct {
	ID        bson.ObjectID `bson:"id"`
	Dateiname string        `bson:"dateiname"`
	MimeTyp   string        `bson:"mimeTyp"`
	Groesse   int64         `bson:"groesse"`
	Sha256    string        `bson:"sha256"`
}

type korrekturDok struct {
	Field       string    `bson:"feld"`
	Recognized  *string   `bson:"erkannt"`
	Corrected   *string   `bson:"korrigiert"`
	CorrectedAt time.Time `bson:"zeitpunkt"`
}

// eintragKurzDok liest nur die Felder, die die Zeitleiste braucht.
type eintragKurzDok struct {
	ID            bson.ObjectID `bson:"_id"`
	Modul         string        `bson:"modul"`
	Typ           string        `bson:"typ"`
	Dokumentdatum *string       `bson:"dokumentdatum"`
	Importdatum   time.Time     `bson:"importdatum"`
	Quelle        *string       `bson:"quelle"`
}

// kurzProjektion sagt MongoDB, welche Felder es für die Liste schicken soll.
// Volltext, Infos usw. bleiben in der Datenbank, die Antwort bleibt klein.
var kurzProjektion = bson.D{
	{Key: "modul", Value: 1},
	{Key: "typ", Value: 1},
	{Key: "dokumentdatum", Value: 1},
	{Key: "importdatum", Value: 1},
	{Key: "quelle", Value: 1},
}

func zuEintragDok(e core.Entry) (eintragDok, error) {
	id, err := bson.ObjectIDFromHex(e.ID)
	if err != nil {
		return eintragDok{}, fmt.Errorf("ungültige Eintrag-ID: %w", err)
	}
	dateiID, err := bson.ObjectIDFromHex(e.File.ID)
	if err != nil {
		return eintragDok{}, fmt.Errorf("ungültige Datei-ID: %w", err)
	}

	infos := make([]infoDok, 0, len(e.Facts))
	for _, i := range e.Facts {
		infos = append(infos, infoDok(i))
	}
	korrekturen := make([]korrekturDok, 0, len(e.Corrections))
	for _, k := range e.Corrections {
		k.CorrectedAt = k.CorrectedAt.UTC()
		korrekturen = append(korrekturen, korrekturDok(k))
	}

	return eintragDok{
		ID:            id,
		Modul:         e.Module,
		Typ:           e.Type,
		Dokumentdatum: e.DocumentDate,
		Importdatum:   e.ImportedAt.UTC(),
		Quelle:        e.Issuer,
		Volltext:      e.FullText,
		Infos:         infos,
		Datei: dateiDok{
			ID:        dateiID,
			Dateiname: e.File.FileName,
			MimeTyp:   e.File.MimeType,
			Groesse:   e.File.Size,
			Sha256:    e.File.Sha256,
		},
		Korrekturen: korrekturen,
	}, nil
}

func (d eintragDok) zuKern() core.Entry {
	infos := make([]core.Fact, 0, len(d.Infos))
	for _, i := range d.Infos {
		infos = append(infos, core.Fact(i))
	}
	korrekturen := make([]core.Correction, 0, len(d.Korrekturen))
	for _, k := range d.Korrekturen {
		korrekturen = append(korrekturen, core.Correction(k))
	}
	return core.Entry{
		ID:           d.ID.Hex(),
		Module:       d.Modul,
		Type:         d.Typ,
		DocumentDate: d.Dokumentdatum,
		ImportedAt:   d.Importdatum.UTC(),
		Issuer:       d.Quelle,
		FullText:     d.Volltext,
		Facts:        infos,
		File: core.File{
			ID:       d.Datei.ID.Hex(),
			FileName: d.Datei.Dateiname,
			MimeType: d.Datei.MimeTyp,
			Size:     d.Datei.Groesse,
			Sha256:   d.Datei.Sha256,
		},
		Corrections: korrekturen,
	}
}

func (d eintragKurzDok) zuKern() core.EntrySummary {
	return core.EntrySummary{
		ID:           d.ID.Hex(),
		Module:       d.Modul,
		Type:         d.Typ,
		DocumentDate: d.Dokumentdatum,
		ImportedAt:   d.Importdatum.UTC(),
		Issuer:       d.Quelle,
	}
}

// abschnittDok ist ein Abschnitt in der Collection "abschnitte".
//
// Das Embedding wird als normales Zahlen-Array gespeichert (768 Zahlen).
// Atlas Vector Search liest genau dieses Feld (Pfad "embedding"), und
// "modul" ist dort als Filterfeld eingetragen (Schritt 5).
type abschnittDok struct {
	ID        bson.ObjectID `bson:"_id"`
	EintragID bson.ObjectID `bson:"eintragId"`
	Modul     string        `bson:"modul"`
	Nr        int           `bson:"nr"`
	Text      string        `bson:"text"`
	Embedding []float32     `bson:"embedding"`
}

func zuAbschnittDok(a core.Chunk) (abschnittDok, error) {
	eintragID, err := bson.ObjectIDFromHex(a.EntryID)
	if err != nil {
		return abschnittDok{}, fmt.Errorf("ungültige Eintrag-ID im Abschnitt: %w", err)
	}
	if len(a.Embedding) == 0 {
		return abschnittDok{}, fmt.Errorf("Abschnitt %d hat kein Embedding", a.Position)
	}
	return abschnittDok{
		ID:        bson.NewObjectID(),
		EintragID: eintragID,
		Modul:     a.Module,
		Nr:        a.Position,
		Text:      a.Text,
		Embedding: a.Embedding,
	}, nil
}

func (d abschnittDok) zuKern() core.Chunk {
	return core.Chunk{
		EntryID:   d.EintragID.Hex(),
		Module:    d.Modul,
		Position:  d.Nr,
		Text:      d.Text,
		Embedding: d.Embedding,
	}
}
