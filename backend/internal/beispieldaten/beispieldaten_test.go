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
		for _, id := range []string{e.ID, e.Datei.ID} {
			if err := core.PruefeID(id); err != nil {
				t.Errorf("%s: ungültige ID %q", e.Typ, id)
			}
			if ids[id] {
				t.Errorf("ID %q doppelt", id)
			}
			ids[id] = true
		}
		if !core.TypErlaubt(e.Modul, e.Typ) {
			t.Errorf("Typ %q im Modul %q nicht erlaubt", e.Typ, e.Modul)
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
		texte := core.ZerlegeInAbschnitte(e.Volltext, core.AbschnittMaxZeichen)
		if len(a) == 0 || len(a) != len(texte) {
			t.Fatalf("%s: %d Abschnitte, erwartet %d (> 0)", e.Typ, len(a), len(texte))
		}
		for i, ab := range a {
			if ab.EintragID != e.ID || ab.Modul != e.Modul || ab.Nr != i+1 || ab.Text != texte[i] {
				t.Errorf("%s Abschnitt %d = %+v", e.Typ, i, ab)
			}
		}
	}
}

func TestWegwerf(t *testing.T) {
	const id, dateiID = "aaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbb"
	jetzt := time.Date(2026, 11, 5, 10, 0, 0, 123_456_789, time.FixedZone("MEZ", 3600))
	e, abschnitte := Wegwerf(id, dateiID, jetzt)

	if e.ID != id || e.Datei.ID != dateiID || !core.TypErlaubt(e.Modul, e.Typ) {
		t.Errorf("Eintrag %+v", e)
	}
	if want := time.Date(2026, 11, 5, 9, 0, 0, 123_000_000, time.UTC); !e.Importdatum.Equal(want) || e.Importdatum.Location() != time.UTC {
		t.Errorf("Importdatum %v, erwartet %v", e.Importdatum, want)
	}
	if e.Datei.Groesse != int64(len(e.Volltext)) || len(e.Datei.Sha256) != 64 {
		t.Errorf("Datei %+v", e.Datei)
	}
	if e.Infos == nil || e.Korrekturen == nil {
		t.Error("Listen müssen [] sein, nicht nil")
	}
	if len(abschnitte) == 0 {
		t.Fatal("keine Abschnitte")
	}
	for _, a := range abschnitte {
		if a.EintragID != id || len(a.Embedding) != modell.EmbeddingDimensionen || a.Embedding[0] != 1 {
			t.Errorf("Abschnitt %d: eintrag %s, %d Dimensionen", a.Nr, a.EintragID, len(a.Embedding))
		}
	}

	// Ein anderer Zeitpunkt ergibt einen anderen Hash: kein Duplikat.
	e2, _ := Wegwerf(id, dateiID, jetzt.Add(time.Millisecond))
	if e2.Datei.Sha256 == e.Datei.Sha256 {
		t.Error("gleicher Hash für verschiedene Zeitpunkte")
	}
}
