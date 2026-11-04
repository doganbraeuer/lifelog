package speicher

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/kern"
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
	ID      string  `bson:"id"`
	Name    string  `bson:"name"`
	Wert    string  `bson:"wert"`
	Einheit *string `bson:"einheit,omitempty"`
}

type dateiDok struct {
	ID        bson.ObjectID `bson:"id"`
	Dateiname string        `bson:"dateiname"`
	MimeTyp   string        `bson:"mimeTyp"`
	Groesse   int64         `bson:"groesse"`
	Sha256    string        `bson:"sha256"`
}

type korrekturDok struct {
	Feld       string    `bson:"feld"`
	Erkannt    *string   `bson:"erkannt"`
	Korrigiert *string   `bson:"korrigiert"`
	Zeitpunkt  time.Time `bson:"zeitpunkt"`
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

func zuEintragDok(e kern.Eintrag) (eintragDok, error) {
	id, err := bson.ObjectIDFromHex(e.ID)
	if err != nil {
		return eintragDok{}, fmt.Errorf("ungültige Eintrag-ID: %w", err)
	}
	dateiID, err := bson.ObjectIDFromHex(e.Datei.ID)
	if err != nil {
		return eintragDok{}, fmt.Errorf("ungültige Datei-ID: %w", err)
	}

	infos := make([]infoDok, 0, len(e.Infos))
	for _, i := range e.Infos {
		infos = append(infos, infoDok(i))
	}
	korrekturen := make([]korrekturDok, 0, len(e.Korrekturen))
	for _, k := range e.Korrekturen {
		k.Zeitpunkt = k.Zeitpunkt.UTC()
		korrekturen = append(korrekturen, korrekturDok(k))
	}

	return eintragDok{
		ID:            id,
		Modul:         e.Modul,
		Typ:           e.Typ,
		Dokumentdatum: e.Dokumentdatum,
		Importdatum:   e.Importdatum.UTC(),
		Quelle:        e.Quelle,
		Volltext:      e.Volltext,
		Infos:         infos,
		Datei: dateiDok{
			ID:        dateiID,
			Dateiname: e.Datei.Dateiname,
			MimeTyp:   e.Datei.MimeTyp,
			Groesse:   e.Datei.Groesse,
			Sha256:    e.Datei.Sha256,
		},
		Korrekturen: korrekturen,
	}, nil
}

func (d eintragDok) zuKern() kern.Eintrag {
	infos := make([]kern.Info, 0, len(d.Infos))
	for _, i := range d.Infos {
		infos = append(infos, kern.Info(i))
	}
	korrekturen := make([]kern.Korrektur, 0, len(d.Korrekturen))
	for _, k := range d.Korrekturen {
		korrekturen = append(korrekturen, kern.Korrektur(k))
	}
	return kern.Eintrag{
		ID:            d.ID.Hex(),
		Modul:         d.Modul,
		Typ:           d.Typ,
		Dokumentdatum: d.Dokumentdatum,
		Importdatum:   d.Importdatum.UTC(),
		Quelle:        d.Quelle,
		Volltext:      d.Volltext,
		Infos:         infos,
		Datei: kern.Datei{
			ID:        d.Datei.ID.Hex(),
			Dateiname: d.Datei.Dateiname,
			MimeTyp:   d.Datei.MimeTyp,
			Groesse:   d.Datei.Groesse,
			Sha256:    d.Datei.Sha256,
		},
		Korrekturen: korrekturen,
	}
}

func (d eintragKurzDok) zuKern() kern.EintragKurz {
	return kern.EintragKurz{
		ID:            d.ID.Hex(),
		Modul:         d.Modul,
		Typ:           d.Typ,
		Dokumentdatum: d.Dokumentdatum,
		Importdatum:   d.Importdatum.UTC(),
		Quelle:        d.Quelle,
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

func zuAbschnittDok(a kern.Abschnitt) (abschnittDok, error) {
	eintragID, err := bson.ObjectIDFromHex(a.EintragID)
	if err != nil {
		return abschnittDok{}, fmt.Errorf("ungültige Eintrag-ID im Abschnitt: %w", err)
	}
	if len(a.Embedding) == 0 {
		return abschnittDok{}, fmt.Errorf("Abschnitt %d hat kein Embedding", a.Nr)
	}
	return abschnittDok{
		ID:        bson.NewObjectID(),
		EintragID: eintragID,
		Modul:     a.Modul,
		Nr:        a.Nr,
		Text:      a.Text,
		Embedding: a.Embedding,
	}, nil
}

func (d abschnittDok) zuKern() kern.Abschnitt {
	return kern.Abschnitt{
		EintragID: d.EintragID.Hex(),
		Modul:     d.Modul,
		Nr:        d.Nr,
		Text:      d.Text,
		Embedding: d.Embedding,
	}
}
