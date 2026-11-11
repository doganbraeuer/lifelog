package main

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"lifelog/backend/internal/core"
)

// aufloesen macht aus den Zielen aller Fragen Eintrags-IDs. Ein ID-Ziel muss
// in der Liste stehen, ein Ziel über Dokumentdatum (plus Typ/Quelle) muss
// genau einen Eintrag treffen. Sonst gibt es einen Fehler (alle Probleme auf einmal),
// damit ein fehlender oder doppelter Upload nicht still falsche Zahlen
// liefert. Ohne I/O, damit testbar.
func aufloesen(fragen []kalibrierfrage, liste []core.EntrySummary) (map[ziel]string, error) {
	ids := map[ziel]string{}
	var fehler []error
	for _, f := range fragen {
		for _, z := range f.Erwartet {
			if _, schon := ids[z]; schon {
				continue
			}
			var passende []string
			for _, e := range liste {
				if z.ID != "" {
					if e.ID == z.ID {
						passende = append(passende, e.ID)
					}
				} else if passt(z, e) {
					passende = append(passende, e.ID)
				}
			}
			switch len(passende) {
			case 1:
				ids[z] = passende[0]
			case 0:
				fehler = append(fehler, fmt.Errorf("kein Eintrag für %v (Frage %q)", z, f.Text))
				ids[z] = "" // nur einmal melden
			default:
				fehler = append(fehler, fmt.Errorf("%d Einträge für %v, erwartet genau einen (Frage %q)", len(passende), z, f.Text))
				ids[z] = ""
			}
		}
	}
	if len(fehler) > 0 {
		return nil, errors.Join(fehler...)
	}
	return ids, nil
}

// passt: gleiches Dokumentdatum, dazu Typ und Quelle, soweit im Ziel angegeben.
func passt(z ziel, e core.EntrySummary) bool {
	if e.DocumentDate == nil || *e.DocumentDate != z.Datum {
		return false
	}
	if z.Typ != "" && e.Type != z.Typ {
		return false
	}
	if z.Quelle != "" && (e.Issuer == nil || *e.Issuer != z.Quelle) {
		return false
	}
	return true
}

// messung ist das Ergebnis der Vektorsuche für eine Frage. Ohne I/O, damit
// die Auswertung testbar ist.
type messung struct {
	Frage         kalibrierfrage
	IDs           []string  // aufgelöste Ziele, gleiche Reihenfolge wie Frage.Erwartet
	Bester        float64   // höchster Score aller Treffer, 0 ohne Treffer
	BesterEintrag string    // Eintrag des besten Treffers, "" ohne Treffer
	Erwartet      []float64 // je erwartetem Eintrag der beste Score, 0 wenn nicht unter den Treffern
}

// messen fasst die Treffer einer Frage zusammen. Die Treffer kommen von
// Atlas absteigend sortiert, darauf verlassen wir uns aber nicht.
func messen(f kalibrierfrage, ids []string, treffer []core.Hit) messung {
	m := messung{Frage: f, IDs: ids, Erwartet: make([]float64, len(ids))}
	for _, t := range treffer {
		if t.Score > m.Bester {
			m.Bester, m.BesterEintrag = t.Score, t.EntryID
		}
		for i, id := range ids {
			if t.EntryID == id && t.Score > m.Erwartet[i] {
				m.Erwartet[i] = t.Score
			}
		}
	}
	return m
}

// RichtigOben: Der beste Treffer gehört zu einem der erwarteten Einträge.
func (m messung) RichtigOben() bool {
	if m.Frage.Gruppe != passend || m.BesterEintrag == "" {
		return false
	}
	for _, id := range m.IDs {
		if id == m.BesterEintrag {
			return true
		}
	}
	return false
}

// AlleGefunden: Jeder erwartete Eintrag ist unter den Treffern.
func (m messung) AlleGefunden() bool {
	for _, s := range m.Erwartet {
		if s == 0 {
			return false
		}
	}
	return len(m.Erwartet) > 0
}

// massgeblich ist der Score, der über die Schwelle entscheidet: bei
// passenden Fragen der schwächste erwartete Eintrag (kommen alle nötigen
// Fundstellen zum Sprachmodell?), sonst der beste überhaupt (wird das
// Sprachmodell gefragt?).
func (m messung) massgeblich() float64 {
	if m.Frage.Gruppe != passend {
		return m.Bester
	}
	if len(m.Erwartet) == 0 {
		return 0
	}
	min := m.Erwartet[0]
	for _, s := range m.Erwartet[1:] {
		min = math.Min(min, s)
	}
	return min
}

// spanne: kleinster und größter maßgeblicher Score einer Gruppe und wie
// viele Fragen die Schwelle erreichen (≥, wie core.UeberSchwelle).
type spanne struct {
	Anzahl, Durch int
	Min, Max      float64
}

func (s *spanne) hinzu(wert, schwelle float64) {
	if s.Anzahl == 0 {
		s.Min, s.Max = wert, wert
	}
	s.Min, s.Max = math.Min(s.Min, wert), math.Max(s.Max, wert)
	s.Anzahl++
	if wert >= schwelle {
		s.Durch++
	}
}

type auswertung struct {
	Schwelle                  float64
	Passend, Nah, Fachfremd   spanne
	Mehrere                   spanne // Teilmenge von Passend: Antwort braucht mehrere Einträge
	FalschOben, NichtGefunden int    // nur passend
}

func auswerten(messungen []messung, schwelle float64) auswertung {
	a := auswertung{Schwelle: schwelle}
	for _, m := range messungen {
		wert := m.massgeblich()
		switch m.Frage.Gruppe {
		case passend:
			a.Passend.hinzu(wert, schwelle)
			if len(m.Erwartet) > 1 {
				a.Mehrere.hinzu(wert, schwelle)
			}
			if !m.AlleGefunden() {
				a.NichtGefunden++
			} else if !m.RichtigOben() {
				a.FalschOben++
			}
		case nah:
			a.Nah.hinzu(wert, schwelle)
		default:
			a.Fachfremd.hinzu(wert, schwelle)
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
	fmt.Fprintf(&b, "Schwelle %s (cos %s):\n", zahl(a.Schwelle), zahl(core.CosineFromScore(a.Schwelle)))
	zeile("passend", a.Passend, "Fundstellen kommen durch (soll: alle)")
	zeile("  mehrere", a.Mehrere, "davon mit mehreren Einträgen, alle kommen durch")
	zeile("nah", a.Nah, "gehen ans Sprachmodell (erlaubt, es antwortet mit dem festen Satz)")
	zeile("fachfremd", a.Fachfremd, "gehen ans Sprachmodell (soll: keine)")
	if a.FalschOben > 0 {
		fmt.Fprintf(&b, "  %d passende Fragen: bester Treffer aus einem anderen Eintrag\n", a.FalschOben)
	}
	if a.NichtGefunden > 0 {
		fmt.Fprintf(&b, "  %d passende Fragen: mindestens ein erwarteter Eintrag nicht unter den Treffern\n", a.NichtGefunden)
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
