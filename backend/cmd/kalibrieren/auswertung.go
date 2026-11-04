package main

import (
	"fmt"
	"math"
	"strings"

	"lifelog/backend/internal/kern"
)

// messung ist das Ergebnis der Vektorsuche für eine Frage. Ohne I/O, damit
// die Auswertung testbar ist.
type messung struct {
	Frage         kalibrierfrage
	Bester        float64 // höchster Score aller Treffer, 0 ohne Treffer
	BesterEintrag string  // Eintrag des besten Treffers, "" ohne Treffer
	Erwartet      float64 // nur passend: bester Score eines Abschnitts des erwarteten Eintrags, 0 wenn nicht unter den Treffern
}

// messen fasst die Treffer einer Frage zusammen. Die Treffer kommen von
// Atlas absteigend sortiert, darauf verlassen wir uns aber nicht.
func messen(f kalibrierfrage, treffer []kern.Treffer) messung {
	m := messung{Frage: f}
	for _, t := range treffer {
		if t.Score > m.Bester {
			m.Bester, m.BesterEintrag = t.Score, t.EintragID
		}
		if f.Gruppe == passend && t.EintragID == f.Erwartet && t.Score > m.Erwartet {
			m.Erwartet = t.Score
		}
	}
	return m
}

// RichtigOben: Der beste Treffer gehört zum erwarteten Eintrag.
func (m messung) RichtigOben() bool {
	return m.Frage.Gruppe == passend && m.BesterEintrag != "" && m.BesterEintrag == m.Frage.Erwartet
}

// massgeblich ist der Score, der über die Schwelle entscheidet: bei
// passenden Fragen der des richtigen Eintrags (kommt seine Fundstelle zum
// Sprachmodell?), sonst der beste überhaupt (wird das Sprachmodell gefragt?).
func (m messung) massgeblich() float64 {
	if m.Frage.Gruppe == passend {
		return m.Erwartet
	}
	return m.Bester
}

// spanne: kleinster und größter maßgeblicher Score einer Gruppe und wie
// viele Fragen die Schwelle erreichen (≥, wie kern.UeberSchwelle).
type spanne struct {
	Anzahl, Durch int
	Min, Max      float64
}

type auswertung struct {
	Schwelle                  float64
	Passend, Nah, Fachfremd   spanne
	FalschOben, NichtGefunden int // nur passend
}

func auswerten(messungen []messung, schwelle float64) auswertung {
	a := auswertung{Schwelle: schwelle}
	for _, m := range messungen {
		s := &a.Fachfremd
		switch m.Frage.Gruppe {
		case passend:
			s = &a.Passend
			if m.Erwartet == 0 {
				a.NichtGefunden++
			} else if !m.RichtigOben() {
				a.FalschOben++
			}
		case nah:
			s = &a.Nah
		}
		wert := m.massgeblich()
		if s.Anzahl == 0 {
			s.Min, s.Max = wert, wert
		}
		s.Min, s.Max = math.Min(s.Min, wert), math.Max(s.Max, wert)
		s.Anzahl++
		if wert >= schwelle {
			s.Durch++
		}
	}
	return a
}

// Trennbar: Gibt es eine Schwelle, die alle passenden durchlässt und alle
// fachfremden abfängt? Dann liegt sie in (Fachfremd.Max, Passend.Min].
func (a auswertung) Trennbar() bool {
	return a.Passend.Anzahl > 0 && a.Fachfremd.Anzahl > 0 && a.Passend.Min > a.Fachfremd.Max
}

// zahl schreibt einen Score mit Dezimalkomma und drei Stellen.
func zahl(f float64) string {
	return strings.Replace(fmt.Sprintf("%.3f", f), ".", ",", 1)
}

// Zusammenfassung als Text für die Konsole.
func (a auswertung) Zusammenfassung() string {
	var b strings.Builder
	zeile := func(name string, s spanne, was string) {
		if s.Anzahl == 0 {
			return
		}
		fmt.Fprintf(&b, "  %-10s %2d Fragen, Score %s … %s, %2d/%d %s\n",
			name, s.Anzahl, zahl(s.Min), zahl(s.Max), s.Durch, s.Anzahl, was)
	}
	fmt.Fprintf(&b, "Schwelle %s (cos %s):\n", zahl(a.Schwelle), zahl(kern.KosinusAusScore(a.Schwelle)))
	zeile("passend", a.Passend, "Fundstellen kommen durch (soll: alle)")
	zeile("nah", a.Nah, "gehen ans Sprachmodell (erlaubt, es antwortet mit dem festen Satz)")
	zeile("fachfremd", a.Fachfremd, "gehen ans Sprachmodell (soll: keine)")
	if a.FalschOben > 0 {
		fmt.Fprintf(&b, "  %d passende Fragen: bester Treffer aus einem anderen Eintrag\n", a.FalschOben)
	}
	if a.NichtGefunden > 0 {
		fmt.Fprintf(&b, "  %d passende Fragen: richtiger Eintrag nicht unter den Treffern\n", a.NichtGefunden)
	}
	if a.Trennbar() {
		fmt.Fprintf(&b, "Trennbar: jede Schwelle über %s bis einschließlich %s trennt passend von fachfremd.\n",
			zahl(a.Fachfremd.Max), zahl(a.Passend.Min))
	} else if a.Passend.Anzahl > 0 && a.Fachfremd.Anzahl > 0 {
		fmt.Fprintf(&b, "Nicht trennbar: fachfremd reicht bis %s, passend beginnt bei %s.\n",
			zahl(a.Fachfremd.Max), zahl(a.Passend.Min))
	}
	return b.String()
}
