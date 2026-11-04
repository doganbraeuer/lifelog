package main

import (
	"strings"
	"testing"

	"lifelog/backend/internal/beispieldaten"
	"lifelog/backend/internal/kern"
)

func TestKalibrierfragen_Aufbau(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range beispieldaten.Eintraege() {
		ids[e.ID] = true
	}
	anzahl := map[gruppe]int{}
	texte := map[string]bool{}
	for _, f := range kalibrierfragen() {
		anzahl[f.Gruppe]++
		if texte[f.Text] {
			t.Errorf("Frage doppelt: %q", f.Text)
		}
		texte[f.Text] = true
		if _, err := kern.PruefeFrage(f.Text); err != nil {
			t.Errorf("%q: %v", f.Text, err)
		}
		switch {
		case f.Gruppe == passend && !ids[f.Erwartet]:
			t.Errorf("%q: erwarteter Eintrag %q ist kein Testeintrag", f.Text, f.Erwartet)
		case f.Gruppe != passend && f.Erwartet != "":
			t.Errorf("%q: Gruppe %v darf keinen erwarteten Eintrag haben", f.Text, f.Gruppe)
		}
	}
	if anzahl[passend] < 10 || anzahl[fachfremd] < 5 {
		t.Errorf("Fragen je Gruppe: %v, erwartet ≥ 10 passend und ≥ 5 fachfremd", anzahl)
	}
}

func TestMessen(t *testing.T) {
	f := kalibrierfrage{Text: "x", Gruppe: passend, Erwartet: "B"}
	// Absichtlich unsortiert; Eintrag B kommt mit zwei Abschnitten vor.
	m := messen(f, []kern.Treffer{
		{EintragID: "B", Score: 0.81}, {EintragID: "A", Score: 0.90}, {EintragID: "B", Score: 0.85},
	})
	if m.Bester != 0.90 || m.BesterEintrag != "A" || m.Erwartet != 0.85 || m.RichtigOben() {
		t.Errorf("= %+v, RichtigOben %v", m, m.RichtigOben())
	}

	m = messen(f, []kern.Treffer{{EintragID: "B", Score: 0.88}})
	if !m.RichtigOben() || m.Erwartet != 0.88 {
		t.Errorf("richtig oben: %+v", m)
	}

	m = messen(f, nil)
	if m.Bester != 0 || m.BesterEintrag != "" || m.Erwartet != 0 || m.RichtigOben() {
		t.Errorf("ohne Treffer: %+v", m)
	}

	// Bei fachfremden Fragen gibt es keinen erwarteten Eintrag, auch wenn
	// zufällig ein Treffer eine leere ID hätte.
	m = messen(kalibrierfrage{Gruppe: fachfremd}, []kern.Treffer{{EintragID: "", Score: 0.7}})
	if m.Erwartet != 0 || m.RichtigOben() {
		t.Errorf("fachfremd: %+v", m)
	}
}

func mess(g gruppe, bester, erwartet float64, richtigOben bool) messung {
	m := messung{Frage: kalibrierfrage{Gruppe: g}, Bester: bester, BesterEintrag: "X", Erwartet: erwartet}
	if g == passend {
		m.Frage.Erwartet = "Y"
		if richtigOben {
			m.BesterEintrag = "Y"
		}
	}
	return m
}

func TestAuswerten(t *testing.T) {
	messungen := []messung{
		mess(passend, 0.90, 0.90, true),
		mess(passend, 0.88, 0.80, false), // falscher Eintrag oben; zählt der richtige: genau auf der Schwelle
		mess(passend, 0.86, 0, false),    // richtiger Eintrag fehlt → maßgeblich 0
		mess(nah, 0.83, 0, false),
		mess(fachfremd, 0.77, 0, false),
		mess(fachfremd, 0.81, 0, false),
	}
	a := auswerten(messungen, 0.80)

	if a.Passend != (spanne{Anzahl: 3, Durch: 2, Min: 0, Max: 0.90}) {
		t.Errorf("passend = %+v", a.Passend)
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
		mess(passend, 0.90, 0.90, true),
		mess(passend, 0.87, 0.87, true),
		mess(fachfremd, 0.78, 0, false),
		mess(fachfremd, 0.74, 0, false),
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
	if strings.Contains(text, "nah") {
		t.Errorf("leere Gruppe nah erscheint:\n%s", text)
	}

	// Gleichstand ist nicht trennbar: Eine Schwelle ≥ 0,85 lässt beide durch
	// oder keine.
	gleich := auswerten([]messung{mess(passend, 0.85, 0.85, true), mess(fachfremd, 0.85, 0, false)}, 0.80)
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
