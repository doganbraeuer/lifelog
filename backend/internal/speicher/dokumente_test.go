package speicher

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

func zeiger(s string) *string { return &s }

// beispielEintrag ist synthetisch (siehe docs/API.md).
func beispielEintrag() core.Entry {
	return core.Entry{
		ID:           "6727a1f3c2b4e5d6f7a8b9c0",
		Module:       "gesundheit",
		Type:         "laborbefund",
		DocumentDate: zeiger("2026-10-14"),
		ImportedAt:   time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer:       zeiger("Labor Muster GmbH, Salzburg"),
		FullText:     "Hämoglobin 14,2 g/dl",
		Facts: []core.Fact{
			{ID: "i1", Name: "Hämoglobin", Value: "14,2", Unit: zeiger("g/dl")},
			{ID: "i2", Name: "Befundart", Value: "Blutbild klein"},
		},
		File: core.File{
			ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund.pdf",
			MimeType: "application/pdf", Size: 184320, Sha256: strings.Repeat("ab", 32),
		},
		Corrections: []core.Correction{{
			Field: "infos.i1.wert", Recognized: zeiger("14,1"), Corrected: zeiger("14,2"),
			CorrectedAt: time.Date(2026, 11, 3, 19, 5, 31, 0, time.UTC),
		}},
	}
}

// Hin und zurück über echtes BSON, ohne Datenbank: Es darf nichts verloren gehen.
func TestEintragDok_HinUndZurueck(t *testing.T) {
	original := beispielEintrag()

	dok, err := zuEintragDok(original)
	if err != nil {
		t.Fatal(err)
	}
	roh, err := bson.Marshal(dok)
	if err != nil {
		t.Fatal(err)
	}
	var gelesen eintragDok
	if err := bson.Unmarshal(roh, &gelesen); err != nil {
		t.Fatal(err)
	}

	if got := gelesen.zuKern(); !reflect.DeepEqual(got, original) {
		t.Errorf("nach BSON:\n%+v\nerwartet:\n%+v", got, original)
	}
}

func TestEintragDok_NullUndLeereListen(t *testing.T) {
	e := beispielEintrag()
	e.DocumentDate = nil
	e.Issuer = nil
	e.Facts = nil
	e.Corrections = nil

	dok, err := zuEintragDok(e)
	if err != nil {
		t.Fatal(err)
	}
	roh, _ := bson.Marshal(dok)
	var m bson.M
	if err := bson.Unmarshal(roh, &m); err != nil {
		t.Fatal(err)
	}

	// Datum und Quelle stehen als null in der Datenbank (Feld vorhanden).
	for _, feld := range []string{"dokumentdatum", "quelle"} {
		if v, ok := m[feld]; !ok || v != nil {
			t.Errorf("%s = %v (vorhanden: %v), erwartet null", feld, v, ok)
		}
	}
	// Listen sind leere Arrays, nicht null.
	for _, feld := range []string{"infos", "korrekturen"} {
		if _, ok := m[feld].(bson.A); !ok {
			t.Errorf("%s ist %T, erwartet ein Array", feld, m[feld])
		}
	}

	// Ein altes Dokument mit null-Listen wird beim Lesen zu leeren Listen.
	dok.Infos, dok.Korrekturen = nil, nil
	k := dok.zuKern()
	if k.Facts == nil || k.Corrections == nil {
		t.Error("zuKern muss leere statt nil-Listen liefern")
	}
}

func TestEintragDok_EinheitFehltWennNil(t *testing.T) {
	dok, _ := zuEintragDok(beispielEintrag())
	roh, _ := bson.Marshal(dok.Infos[1])
	var m bson.M
	_ = bson.Unmarshal(roh, &m)
	if _, ok := m["einheit"]; ok {
		t.Errorf("einheit soll fehlen, wenn nil: %v", m)
	}
}

func TestEintragDok_UngueltigeIDs(t *testing.T) {
	e := beispielEintrag()
	e.ID = "keine-objectid"
	if _, err := zuEintragDok(e); err == nil {
		t.Error("ungültige Eintrag-ID muss Fehler liefern")
	}
	e = beispielEintrag()
	e.File.ID = ""
	if _, err := zuEintragDok(e); err == nil {
		t.Error("ungültige Datei-ID muss Fehler liefern")
	}
}

func TestAbschnittDok_HinUndZurueck(t *testing.T) {
	original := core.Chunk{
		EntryID:  "6727a1f3c2b4e5d6f7a8b9c0",
		Module:   "gesundheit",
		Position: 2,
		Text:     "Hämoglobin 14,2 g/dl",
		// Werte, die als float32 nicht exakt dezimal darstellbar sind.
		Embedding: []float32{0.1, -0.33333334, 1e-7, 0.99999994, -1},
	}
	dok, err := zuAbschnittDok(original)
	if err != nil {
		t.Fatal(err)
	}
	if dok.ID.IsZero() {
		t.Error("Abschnitt braucht eine eigene _id")
	}

	roh, err := bson.Marshal(dok)
	if err != nil {
		t.Fatal(err)
	}
	var m bson.M
	_ = bson.Unmarshal(roh, &m)
	if _, ok := m["eintragId"].(bson.ObjectID); !ok {
		t.Errorf("eintragId ist %T, erwartet ObjectID (sonst findet AbschnitteLoeschen nichts)", m["eintragId"])
	}
	if _, ok := m["embedding"].(bson.A); !ok {
		t.Errorf("embedding ist %T, erwartet Array (Atlas Vector Search)", m["embedding"])
	}

	var gelesen abschnittDok
	if err := bson.Unmarshal(roh, &gelesen); err != nil {
		t.Fatal(err)
	}
	if got := gelesen.zuKern(); !reflect.DeepEqual(got, original) {
		t.Errorf("nach BSON:\n%+v\nerwartet:\n%+v", got, original)
	}
}

func TestAbschnittDok_Ungueltig(t *testing.T) {
	if _, err := zuAbschnittDok(core.Chunk{EntryID: "x", Embedding: []float32{1}}); err == nil {
		t.Error("ungültige Eintrag-ID muss Fehler liefern")
	}
	if _, err := zuAbschnittDok(core.Chunk{EntryID: "6727a1f3c2b4e5d6f7a8b9c0"}); err == nil {
		t.Error("Abschnitt ohne Embedding muss Fehler liefern")
	}
}

func TestNeueID(t *testing.T) {
	a, b := NeueID(), NeueID()
	if a == b {
		t.Error("zwei IDs sind gleich")
	}
	if _, err := bson.ObjectIDFromHex(a); err != nil || len(a) != 24 {
		t.Errorf("%q ist keine ObjectId: %v", a, err)
	}
}

func TestImportdatumWirdUTC(t *testing.T) {
	e := beispielEintrag()
	wien := time.FixedZone("MEZ", 3600)
	e.ImportedAt = time.Date(2026, 11, 3, 19, 42, 7, 0, wien) // = 18:42:07 UTC
	dok, _ := zuEintragDok(e)
	if dok.Importdatum.Location() != time.UTC || dok.Importdatum.Hour() != 18 {
		t.Errorf("Importdatum = %v, erwartet 18:42:07 UTC", dok.Importdatum)
	}
}
