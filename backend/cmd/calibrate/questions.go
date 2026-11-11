package main

import (
	"strings"

	"lifelog/backend/internal/sampledata"
)

// gruppe sagt, was die Suche bei einer Frage tun soll.
type gruppe int

const (
	// passend: Die Antwort steht in einem oder mehreren Einträgen. Deren
	// Abschnitte sollen über der Schwelle liegen.
	passend gruppe = iota
	// nah: Gesundheitsthema, steht aber in keinem Eintrag. Die Schwelle
	// muss sie nicht abfangen (zweite Schicht: das Sprachmodell antwortet mit
	// dem festen Satz); die Zahlen zeigen, wie nah solche Fragen liegen.
	nah
	// fachfremd: Hat mit den Dokumenten nichts zu tun. Soll unter der
	// Schwelle bleiben, damit das Sprachmodell gar nicht erst gefragt wird.
	fachfremd
)

func (g gruppe) String() string {
	switch g {
	case passend:
		return "passend"
	case nah:
		return "nah"
	default:
		return "fachfremd"
	}
}

// ziel bezeichnet den Eintrag, in dem die Antwort steht: entweder über die
// feste ID eines Testeintrags (cmd/testdaten) oder über das Dokumentdatum
// plus Typ und/oder Quelle (hochgeladene Demo-Daten, deren IDs bei jedem
// Upload neu sind). Leere Felder werden nicht verglichen. aufloesen macht
// daraus eine ID.
type ziel struct {
	ID     string
	Typ    string
	Datum  string // "YYYY-MM-DD"
	Quelle string // genau wie im Eintrag
}

func (z ziel) String() string {
	if z.ID != "" {
		return "ID " + z.ID
	}
	s := z.Typ + " " + z.Datum
	if z.Quelle != "" {
		s += " (" + z.Quelle + ")"
	}
	return strings.TrimSpace(s)
}

type kalibrierfrage struct {
	Text     string
	Gruppe   gruppe
	Erwartet []ziel // Einträge mit der Antwort, nur bei passend; bei mehreren müssen alle ankommen
}

// Demo-Daten (synthetisch, hochgeladen am 07./08.11.2026). Die
// Impfbestätigung und die Text-Notizen hat das Lesemodell als sonstiges
// eingeordnet. Die Überweisung kam als rezept an; ihr Typ wird vermutlich
// in der App korrigiert, deshalb wird sie über die Quelle gefunden (der
// Arztbrief am selben Tag hat eine längere Quelle).
var (
	laborJanuar  = ziel{Typ: "laborbefund", Datum: "2026-01-20"}
	laborJuli    = ziel{Typ: "laborbefund", Datum: "2026-07-22"}
	orthopaedie  = ziel{Typ: "arztbrief", Datum: "2026-03-05"}
	ueberweisung = ziel{Datum: "2026-03-05", Quelle: "Orthopädie am Fluss, Dr. Thomas Probe"}
	fsme         = ziel{Typ: "sonstiges", Datum: "2026-06-10"}
	physioEnde   = ziel{Typ: "sonstiges", Datum: "2026-08-12"}
	blutdruck    = ziel{Typ: "sonstiges", Datum: "2026-09-23"}
)

// kalibrierfragen ist fest, damit Messungen vor und nach einer Änderung
// (Modell, Abschnittsgröße, Präfixe) vergleichbar bleiben. Neue Fragen
// unten anhängen.
func kalibrierfragen() []kalibrierfrage {
	p := func(text string, ziele ...ziel) kalibrierfrage {
		return kalibrierfrage{Text: text, Gruppe: passend, Erwartet: ziele}
	}
	n := func(text string) kalibrierfrage { return kalibrierfrage{Text: text, Gruppe: nah} }
	f := func(text string) kalibrierfrage { return kalibrierfrage{Text: text, Gruppe: fachfremd} }
	id := func(id string) ziel { return ziel{ID: id} }

	return []kalibrierfrage{
		// Testdaten (04.11.2026)
		p("Welchen Hämoglobinwert hatte ich im Oktober?", id(sampledata.Laborbefund)),
		p("Wie hoch waren meine Leukozyten?", id(sampledata.Laborbefund)),
		p("Was stand beim CRP?", id(sampledata.Laborbefund)),
		p("Welches Labor hat mein Blutbild gemacht?", id(sampledata.Laborbefund)),
		p("Wie hoch war mein Blutdruck bei der Routinekontrolle?", id(sampledata.Arztbrief)),
		p("Wann soll ich wieder zur Kontrolle bei Dr. Beispiel?", id(sampledata.Arztbrief)),
		p("Welches Medikament wurde mir verschrieben?", id(sampledata.Rezept)),
		p("Wie oft am Tag soll ich Beispielin nehmen?", id(sampledata.Rezept)),
		p("Wie viele Tabletten stehen auf dem Rezept?", id(sampledata.Rezept)),
		p("Wo liegt mein Impfpass?", id(sampledata.Notiz)),

		// Bis 07.11. nah; seit den Lipidprofilen steht die Antwort in beiden Laborbefunden.
		p("Wie hoch war mein Cholesterin?", laborJanuar, laborJuli),
		n("Wann hatte ich meine letzte Tetanus-Impfung?"),
		n("Welche Allergien habe ich?"),

		f("Wie wird das Wetter morgen in Salzburg?"),
		f("Wer hat die Fußball-WM 2014 gewonnen?"),
		f("Wie koche ich Spaghetti Carbonara?"),
		f("Was kostet eine Zugfahrt von Salzburg nach Wien?"),
		// Bis 07.11. fachfremd; seit der Physio-Notiz („wieder 5 km gelaufen“)
		// gibt es einen Bezug, die Antwort für „letzte Woche“ steht aber nicht drin.
		n("Wie viele Kilometer bin ich letzte Woche gelaufen?"),

		// Demo-Daten (08.11.2026): ein Dokument
		p("Wie hoch war mein LDL-Cholesterin im Januar?", laborJanuar),
		p("Wie hoch war mein Nüchternblutzucker im Januar?", laborJanuar),
		p("Wie hoch war mein HDL-Cholesterin bei der Kontrolle im Juli?", laborJuli),
		p("Wie hoch waren meine Triglyceride im Juli?", laborJuli),
		p("Warum war ich beim Orthopäden?", orthopaedie),
		p("Was hat das Röntgen meines Knies ergeben?", orthopaedie),
		p("Wie lange sollte ich mit dem Laufen pausieren?", orthopaedie),
		p("An welche Physiotherapie-Praxis wurde ich überwiesen?", ueberweisung),
		p("Wie oft pro Woche soll ich zur Physiotherapie?", ueberweisung),
		p("Wann muss ich die FSME-Impfung auffrischen?", fsme),
		p("Welcher Impfstoff wurde bei der FSME-Impfung verwendet?", fsme),
		p("Wie hoch war mein Puls bei der Messung zu Hause?", blutdruck),
		p("Wann habe ich die Physiotherapie abgeschlossen?", physioEnde),

		// Demo-Daten: Antwort braucht zwei Dokumente
		p("Wie hat sich mein LDL-Cholesterin verändert?", laborJanuar, laborJuli),
		p("Wie hat sich mein Gesamtcholesterin zwischen Januar und Juli entwickelt?", laborJanuar, laborJuli),
		p("Warum wurde mir Physiotherapie verordnet und wie oft pro Woche?", orthopaedie, ueberweisung),
		p("Wie lange hat es von der Überweisung bis zum Abschluss der Physiotherapie gedauert?", ueberweisung, physioEnde),
		p("Wie hat sich mein Blutdruck seit der Routinekontrolle verändert?", id(sampledata.Arztbrief), blutdruck),

		n("Wurde bei mir schon ein MRT gemacht?"),
		n("Wie hoch war mein HbA1c-Wert?"),
		n("Wie hoch war mein Vitamin-D-Wert?"),
		n("Wann hatte ich meine letzte Covid-Impfung?"),
		n("Welche Blutgruppe habe ich?"),

		f("Hast du ein Rezept für Apfelkuchen?"),
		f("Bis wann muss ich meine Steuererklärung abgeben?"),
		f("Was kostet ein Flug nach Rom?"),
		f("Wie wechsle ich einen Fahrradschlauch?"),
	}
}
