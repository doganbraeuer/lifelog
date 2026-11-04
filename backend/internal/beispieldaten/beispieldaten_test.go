package beispieldaten

import (
	"testing"

	"lifelog/backend/internal/kern"
)

func TestEintraege_GueltigUndEindeutig(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range Eintraege() {
		for _, id := range []string{e.ID, e.Datei.ID} {
			if err := kern.PruefeID(id); err != nil {
				t.Errorf("%s: ungültige ID %q", e.Typ, id)
			}
			if ids[id] {
				t.Errorf("ID %q doppelt", id)
			}
			ids[id] = true
		}
		if !kern.TypErlaubt(e.Modul, e.Typ) {
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
		texte := kern.ZerlegeInAbschnitte(e.Volltext, kern.AbschnittMaxZeichen)
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
