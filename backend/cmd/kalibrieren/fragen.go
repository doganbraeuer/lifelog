package main

import "lifelog/backend/internal/beispieldaten"

// gruppe sagt, was die Suche bei einer Frage tun soll.
type gruppe int

const (
	// passend: Die Antwort steht in einem Testeintrag. Dessen Abschnitt soll
	// über der Schwelle liegen.
	passend gruppe = iota
	// nah: Gesundheitsthema, steht aber in keinem Testeintrag. Die Schwelle
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

type kalibrierfrage struct {
	Text     string
	Gruppe   gruppe
	Erwartet string // ID des Eintrags mit der Antwort, nur bei passend
}

// kalibrierfragen ist fest, damit Messungen vor und nach einer Änderung
// (Modell, Abschnittsgröße, Präfixe) vergleichbar bleiben. Neue Fragen
// unten anhängen.
func kalibrierfragen() []kalibrierfrage {
	p := func(text, eintrag string) kalibrierfrage {
		return kalibrierfrage{Text: text, Gruppe: passend, Erwartet: eintrag}
	}
	n := func(text string) kalibrierfrage { return kalibrierfrage{Text: text, Gruppe: nah} }
	f := func(text string) kalibrierfrage { return kalibrierfrage{Text: text, Gruppe: fachfremd} }

	return []kalibrierfrage{
		p("Welchen Hämoglobinwert hatte ich im Oktober?", beispieldaten.Laborbefund),
		p("Wie hoch waren meine Leukozyten?", beispieldaten.Laborbefund),
		p("Was stand beim CRP?", beispieldaten.Laborbefund),
		p("Welches Labor hat mein Blutbild gemacht?", beispieldaten.Laborbefund),
		p("Wie hoch war mein Blutdruck bei der Routinekontrolle?", beispieldaten.Arztbrief),
		p("Wann soll ich wieder zur Kontrolle bei Dr. Beispiel?", beispieldaten.Arztbrief),
		p("Welches Medikament wurde mir verschrieben?", beispieldaten.Rezept),
		p("Wie oft am Tag soll ich Beispielin nehmen?", beispieldaten.Rezept),
		p("Wie viele Tabletten stehen auf dem Rezept?", beispieldaten.Rezept),
		p("Wo liegt mein Impfpass?", beispieldaten.Notiz),

		n("Wie hoch war mein Cholesterin?"),
		n("Wann hatte ich meine letzte Tetanus-Impfung?"),
		n("Welche Allergien habe ich?"),

		f("Wie wird das Wetter morgen in Salzburg?"),
		f("Wer hat die Fußball-WM 2014 gewonnen?"),
		f("Wie koche ich Spaghetti Carbonara?"),
		f("Was kostet eine Zugfahrt von Salzburg nach Wien?"),
		f("Wie viele Kilometer bin ich letzte Woche gelaufen?"),
	}
}
