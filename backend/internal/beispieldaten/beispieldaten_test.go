package beispieldaten

import (
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/modell"
)

func TestEintraege_GueltigUndEindeutig(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range Eintraege() {
		for _, id := range []string{e.ID, e.File.ID} {
			if err := core.CheckID(id); err != nil {
				t.Errorf("%s: ungültige ID %q", e.Type, id)
			}
			if ids[id] {
				t.Errorf("ID %q doppelt", id)
			}
			ids[id] = true
		}
		if !core.TypeAllowed(e.Module, e.Type) {
			t.Errorf("Typ %q im Modul %q nicht erlaubt", e.Type, e.Module)
		}
	}
	for _, id := range []string{Laborbefund, Arztbrief, Notiz, Rezept} {
		if !ids[id] {
			t.Errorf("Konstante %q gehört zu keinem Eintrag", id)
		}
	}
}

func TestAbschnitte_WieBeimUpload(t *testing.T) {
	for _, e := range Eintraege() {
		a := Abschnitte(e)
		texte := core.SplitIntoChunks(e.FullText, core.ChunkMaxChars)
		if len(a) == 0 || len(a) != len(texte) {
			t.Fatalf("%s: %d Abschnitte, erwartet %d (> 0)", e.Type, len(a), len(texte))
		}
		for i, ab := range a {
			if ab.EntryID != e.ID || ab.Module != e.Module || ab.Position != i+1 || ab.Text != texte[i] {
				t.Errorf("%s Abschnitt %d = %+v", e.Type, i, ab)
			}
		}
	}
}

func TestWegwerf(t *testing.T) {
	const id, dateiID = "aaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbb"
	jetzt := time.Date(2026, 11, 5, 10, 0, 0, 123_456_789, time.FixedZone("MEZ", 3600))
	e, abschnitte := Wegwerf(id, dateiID, jetzt)

	if e.ID != id || e.File.ID != dateiID || !core.TypeAllowed(e.Module, e.Type) {
		t.Errorf("Eintrag %+v", e)
	}
	if want := time.Date(2026, 11, 5, 9, 0, 0, 123_000_000, time.UTC); !e.ImportedAt.Equal(want) || e.ImportedAt.Location() != time.UTC {
		t.Errorf("Importdatum %v, erwartet %v", e.ImportedAt, want)
	}
	if e.File.Size != int64(len(e.FullText)) || len(e.File.Sha256) != 64 {
		t.Errorf("Datei %+v", e.File)
	}
	if e.Facts == nil || e.Corrections == nil {
		t.Error("Listen müssen [] sein, nicht nil")
	}
	if len(abschnitte) == 0 {
		t.Fatal("keine Abschnitte")
	}
	for _, a := range abschnitte {
		if a.EntryID != id || len(a.Embedding) != modell.EmbeddingDimensionen || a.Embedding[0] != 1 {
			t.Errorf("Abschnitt %d: eintrag %s, %d Dimensionen", a.Position, a.EntryID, len(a.Embedding))
		}
	}

	// Ein anderer Zeitpunkt ergibt einen anderen Hash: kein Duplikat.
	e2, _ := Wegwerf(id, dateiID, jetzt.Add(time.Millisecond))
	if e2.File.Sha256 == e.File.Sha256 {
		t.Error("gleicher Hash für verschiedene Zeitpunkte")
	}
}
