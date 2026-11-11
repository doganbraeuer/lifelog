package main

import (
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/beispieldaten"
	"lifelog/backend/internal/core"
)

func TestKalibrierfragen_Aufbau(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range beispieldaten.Eintraege() {
		ids[e.ID] = true
	}
	anzahl := map[gruppe]int{}
	texte := map[string]bool{}
	mehrere := 0
	for _, f := range kalibrierfragen() {
		anzahl[f.Gruppe]++
		if texte[f.Text] {
			t.Errorf("Frage doppelt: %q", f.Text)
		}
		texte[f.Text] = true
		if _, err := core.PruefeFrage(f.Text); err != nil {
			t.Errorf("%q: %v", f.Text, err)
		}
		if f.Gruppe != passend {
			if len(f.Erwartet) != 0 {
				t.Errorf("%q: Gruppe %v darf keinen erwarteten Eintrag haben", f.Text, f.Gruppe)
			}
			continue
		}
		if len(f.Erwartet) == 0 {
			t.Errorf("%q: passende Frage ohne erwarteten Eintrag", f.Text)
		}
		if len(f.Erwartet) > 1 {
			mehrere++
		}
		gesehen := map[ziel]bool{}
		for _, z := range f.Erwartet {
			if gesehen[z] {
				t.Errorf("%q: Ziel %v doppelt", f.Text, z)
			}
			gesehen[z] = true
			switch {
			case z.ID != "" && (z.Typ != "" || z.Datum != "" || z.Quelle != ""):
				t.Errorf("%q: Ziel %+v hat ID und Typ/Datum/Quelle", f.Text, z)
			case z.ID != "" && !ids[z.ID]:
				t.Errorf("%q: erwarteter Eintrag %q ist kein Testeintrag", f.Text, z.ID)
			case z.ID == "":
				if z.Typ == "" && z.Quelle == "" {
					t.Errorf("%q: Ziel %+v ohne Typ und ohne Quelle", f.Text, z)
				}
				if _, err := time.Parse(time.DateOnly, z.Datum); err != nil {
					t.Errorf("%q: Ziel %+v: Datum ungültig: %v", f.Text, z, err)
				}
			}
		}
	}
	if anzahl[passend] < 10 || anzahl[nah] < 5 || anzahl[fachfremd] < 5 || mehrere < 3 {
		t.Errorf("Fragen je Gruppe: %v, mit mehreren Einträgen %d; erwartet ≥ 10 passend, ≥ 5 nah, ≥ 5 fachfremd, ≥ 3 mit mehreren",
			anzahl, mehrere)
	}
}

func datum(s string) *string { return &s }

func TestAufloesen(t *testing.T) {
	liste := []core.EintragKurz{
		{ID: "T1", Typ: "arztbrief", Dokumentdatum: datum("2026-09-02")},
		{ID: "D1", Typ: "laborbefund", Dokumentdatum: datum("2026-01-20")},
		{ID: "D2", Typ: "laborbefund", Dokumentdatum: datum("2026-07-22")},
		{ID: "D3", Typ: "laborbefund", Dokumentdatum: datum("2026-10-14")},
		{ID: "D4", Typ: "laborbefund", Dokumentdatum: datum("2026-10-14")},
		{ID: "N1", Typ: "sonstiges", Dokumentdatum: nil},
		{ID: "O1", Typ: "arztbrief", Dokumentdatum: datum("2026-03-05"), Quelle: datum("Orthopädie, Dr. Probe, Facharzt")},
		{ID: "O2", Typ: "sonstiges", Dokumentdatum: datum("2026-03-05"), Quelle: datum("Orthopädie, Dr. Probe")},
	}
	jan := ziel{Typ: "laborbefund", Datum: "2026-01-20"}
	juli := ziel{Typ: "laborbefund", Datum: "2026-07-22"}
	ueber := ziel{Datum: "2026-03-05", Quelle: "Orthopädie, Dr. Probe"}
	frage := func(ziele ...ziel) kalibrierfrage {
		return kalibrierfrage{Text: "x", Gruppe: passend, Erwartet: ziele}
	}

	ids, err := aufloesen([]kalibrierfrage{
		frage(ziel{ID: "T1"}, juli),
		frage(jan, juli), // juli zum zweiten Mal: keine doppelte Prüfung, kein Fehler
		frage(ueber),     // über die Quelle, unabhängig vom Typ; die längere Quelle am selben Tag passt nicht
		{Text: "y", Gruppe: fachfremd},
	}, liste)
	if err != nil {
		t.Fatal(err)
	}
	soll := map[ziel]string{{ID: "T1"}: "T1", jan: "D1", juli: "D2", ueber: "O2"}
	if len(ids) != len(soll) {
		t.Errorf("ids = %v", ids)
	}
	for z, id := range soll {
		if ids[z] != id {
			t.Errorf("%v → %q, erwartet %q", z, ids[z], id)
		}
	}

	for name, fall := range map[string]struct {
		z    ziel
		soll string
	}{
		"ID fehlt":       {ziel{ID: "T9"}, "kein Eintrag für ID T9"},
		"Datum fehlt":    {ziel{Typ: "laborbefund", Datum: "2026-08-12"}, "kein Eintrag für laborbefund 2026-08-12"},
		"falscher Typ":   {ziel{Typ: "rezept", Datum: "2026-01-20"}, "kein Eintrag für rezept 2026-01-20"},
		"doppelt":        {ziel{Typ: "laborbefund", Datum: "2026-10-14"}, "2 Einträge für laborbefund 2026-10-14"},
		"ohne Datum nie": {ziel{Typ: "sonstiges", Datum: ""}, "kein Eintrag für sonstiges "},
		"Quelle fehlt":   {ziel{Datum: "2026-03-05", Quelle: "Physiotherapie Beispiel"}, "kein Eintrag für 2026-03-05 (Physiotherapie Beispiel)"},
		"nur Datum":      {ziel{Datum: "2026-03-05"}, "2 Einträge für 2026-03-05"},
	} {
		_, err := aufloesen([]kalibrierfrage{frage(jan, fall.z)}, liste)
		if err == nil || !strings.Contains(err.Error(), fall.soll) {
			t.Errorf("%s: Fehler %v, erwartet %q", name, err, fall.soll)
		}
	}

	// Mehrere Probleme kommen gemeinsam, jedes nur einmal.
	fehlt := ziel{Typ: "arztbrief", Datum: "2026-04-01"}
	_, err = aufloesen([]kalibrierfrage{frage(fehlt), frage(fehlt, ziel{ID: "T9"})}, liste)
	if err == nil || strings.Count(err.Error(), "arztbrief 2026-04-01") != 1 || !strings.Contains(err.Error(), "ID T9") {
		t.Errorf("mehrere Fehler: %v", err)
	}
}

func TestMessen(t *testing.T) {
	f := kalibrierfrage{Text: "x", Gruppe: passend, Erwartet: []ziel{{ID: "B"}}}
	// Absichtlich unsortiert; Eintrag B kommt mit zwei Abschnitten vor.
	m := messen(f, []string{"B"}, []core.Treffer{
		{EintragID: "B", Score: 0.81}, {EintragID: "A", Score: 0.90}, {EintragID: "B", Score: 0.85},
	})
	if m.Bester != 0.90 || m.BesterEintrag != "A" || m.Erwartet[0] != 0.85 || m.RichtigOben() || m.massgeblich() != 0.85 {
		t.Errorf("= %+v, RichtigOben %v", m, m.RichtigOben())
	}

	m = messen(f, []string{"B"}, []core.Treffer{{EintragID: "B", Score: 0.88}})
	if !m.RichtigOben() || m.Erwartet[0] != 0.88 || !m.AlleGefunden() {
		t.Errorf("richtig oben: %+v", m)
	}

	m = messen(f, []string{"B"}, nil)
	if m.Bester != 0 || m.BesterEintrag != "" || m.Erwartet[0] != 0 || m.RichtigOben() || m.AlleGefunden() {
		t.Errorf("ohne Treffer: %+v", m)
	}

	// Zwei Einträge: maßgeblich ist der schwächere; oben genügt einer davon.
	zwei := kalibrierfrage{Text: "z", Gruppe: passend, Erwartet: []ziel{{ID: "B"}, {ID: "C"}}}
	m = messen(zwei, []string{"B", "C"}, []core.Treffer{
		{EintragID: "C", Score: 0.89}, {EintragID: "A", Score: 0.86}, {EintragID: "B", Score: 0.84},
	})
	if !m.RichtigOben() || !m.AlleGefunden() || m.massgeblich() != 0.84 {
		t.Errorf("zwei: %+v", m)
	}
	m = messen(zwei, []string{"B", "C"}, []core.Treffer{{EintragID: "C", Score: 0.89}})
	if !m.RichtigOben() || m.AlleGefunden() || m.massgeblich() != 0 {
		t.Errorf("zwei, B fehlt: %+v", m)
	}

	// Bei fachfremden Fragen gibt es keinen erwarteten Eintrag, auch wenn
	// zufällig ein Treffer eine leere ID hätte.
	m = messen(kalibrierfrage{Gruppe: fachfremd}, nil, []core.Treffer{{EintragID: "", Score: 0.7}})
	if len(m.Erwartet) != 0 || m.RichtigOben() || m.massgeblich() != 0.7 {
		t.Errorf("fachfremd: %+v", m)
	}
}

func mess(g gruppe, bester float64, erwartet []float64, richtigOben bool) messung {
	m := messung{Frage: kalibrierfrage{Gruppe: g}, Bester: bester, BesterEintrag: "X"}
	if g == passend {
		m.Erwartet = erwartet
		for i := range erwartet {
			id := string(rune('Y' + i))
			m.IDs = append(m.IDs, id)
			m.Frage.Erwartet = append(m.Frage.Erwartet, ziel{ID: id})
		}
		if richtigOben {
			m.BesterEintrag = "Y"
		}
	}
	return m
}

func TestAuswerten(t *testing.T) {
	messungen := []messung{
		mess(passend, 0.90, []float64{0.90}, true),
		mess(passend, 0.88, []float64{0.80}, false),      // falscher Eintrag oben; zählt der richtige: genau auf der Schwelle
		mess(passend, 0.86, []float64{0}, false),         // richtiger Eintrag fehlt → maßgeblich 0
		mess(passend, 0.89, []float64{0.89, 0.83}, true), // zwei Einträge: maßgeblich 0,83
		mess(nah, 0.83, nil, false),
		mess(fachfremd, 0.77, nil, false),
		mess(fachfremd, 0.81, nil, false),
	}
	a := auswerten(messungen, 0.80)

	if a.Passend != (spanne{Anzahl: 4, Durch: 3, Min: 0, Max: 0.90}) {
		t.Errorf("passend = %+v", a.Passend)
	}
	if a.Mehrere != (spanne{Anzahl: 1, Durch: 1, Min: 0.83, Max: 0.83}) {
		t.Errorf("mehrere = %+v", a.Mehrere)
	}
	if a.Nah != (spanne{Anzahl: 1, Durch: 1, Min: 0.83, Max: 0.83}) {
		t.Errorf("nah = %+v", a.Nah)
	}
	if a.Fachfremd != (spanne{Anzahl: 2, Durch: 1, Min: 0.77, Max: 0.81}) {
		t.Errorf("fachfremd = %+v", a.Fachfremd)
	}
	if a.FalschOben != 1 || a.NichtGefunden != 1 {
		t.Errorf("FalschOben %d, NichtGefunden %d", a.FalschOben, a.NichtGefunden)
	}
	if a.Trennbar() {
		t.Error("trennbar, obwohl passend bei 0 beginnt")
	}
}

func TestTrennbarUndZusammenfassung(t *testing.T) {
	a := auswerten([]messung{
		mess(passend, 0.90, []float64{0.90}, true),
		mess(passend, 0.87, []float64{0.87}, true),
		mess(fachfremd, 0.78, nil, false),
		mess(fachfremd, 0.74, nil, false),
	}, 0.80)
	if !a.Trennbar() {
		t.Fatalf("nicht trennbar: %+v", a)
	}
	text := a.Zusammenfassung()
	for _, soll := range []string{
		"Schwelle 0,800 (cos 0,600)",
		"2 Fragen, Score 0,870 … 0,900,  2/2 Fundstellen kommen durch",
		"2 Fragen, Score 0,740 … 0,780,  0/2 gehen ans Sprachmodell (soll: keine)",
		"jede Schwelle über 0,780 bis einschließlich 0,870",
	} {
		if !strings.Contains(text, soll) {
			t.Errorf("fehlt %q in\n%s", soll, text)
		}
	}
	if strings.Contains(text, "nah") || strings.Contains(text, "mehrere") {
		t.Errorf("leere Gruppe erscheint:\n%s", text)
	}

	// Gleichstand ist nicht trennbar: Eine Schwelle ≥ 0,85 lässt beide durch
	// oder keine.
	gleich := auswerten([]messung{mess(passend, 0.85, []float64{0.85}, true), mess(fachfremd, 0.85, nil, false)}, 0.80)
	if gleich.Trennbar() || !strings.Contains(gleich.Zusammenfassung(), "Nicht trennbar") {
		t.Errorf("Gleichstand: %s", gleich.Zusammenfassung())
	}
}

func TestZahl(t *testing.T) {
	for f, soll := range map[float64]string{0.8: "0,800", 0.8916: "0,892", 0.8914: "0,891", 0: "0,000", 1: "1,000"} {
		if got := zahl(f); got != soll {
			t.Errorf("zahl(%v) = %q, erwartet %q", f, got, soll)
		}
	}
}
