package main

import (
	"strings"

	"lifelog/backend/internal/sampledata"
)

// group says what search should do for a question.
type group int

const (
	// relevant: the answer is in one or more entries. Their chunks should
	// be above the threshold.
	relevant group = iota
	// near: a health topic, but in no entry. The threshold does not have to
	// catch these (second layer: the language model answers with the fixed
	// sentence); the numbers show how close such questions are.
	near
	// unrelated: has nothing to do with the documents. Should stay below the
	// threshold, so the language model is not asked at all.
	unrelated
)

func (g group) String() string {
	switch g {
	case relevant:
		return "relevant"
	case near:
		return "near"
	default:
		return "unrelated"
	}
}

// target names the entry that contains the answer: either by the fixed ID
// of a test entry (cmd/seed) or by document date plus type and/or issuer
// (uploaded demo data, whose IDs are new on every upload). Empty fields are
// not compared. resolve turns it into an ID.
type target struct {
	ID     string
	Type   string
	Date   string // "YYYY-MM-DD"
	Issuer string // exactly as in the entry
}

func (t target) String() string {
	if t.ID != "" {
		return "ID " + t.ID
	}
	s := t.Type + " " + t.Date
	if t.Issuer != "" {
		s += " (" + t.Issuer + ")"
	}
	return strings.TrimSpace(s)
}

type question struct {
	Text     string
	Group    group
	Expected []target // entries with the answer, only for relevant; with several, all must arrive
}

// Demo data (synthetic, uploaded on 2026-11-07/08). The read model
// classified the vaccination certificate and the text notes as sonstiges.
// The referral came in as rezept; its type will probably be corrected in the
// app, so it is found by its issuer (the doctor's letter on the same day has
// a longer issuer).
var (
	labJanuary    = target{Type: "laborbefund", Date: "2026-01-20"}
	labJuly       = target{Type: "laborbefund", Date: "2026-07-22"}
	orthopedics   = target{Type: "arztbrief", Date: "2026-03-05"}
	referral      = target{Date: "2026-03-05", Issuer: "Orthopädie am Fluss, Dr. Thomas Probe"}
	tbe           = target{Type: "sonstiges", Date: "2026-06-10"}
	physioEnd     = target{Type: "sonstiges", Date: "2026-08-12"}
	bloodPressure = target{Type: "sonstiges", Date: "2026-09-23"}
)

// questions is fixed, so measurements before and after a change (model,
// chunk size, prefixes, prompts) stay comparable. Append new questions at
// the end. The questions are German because users ask in German.
func questions() []question {
	r := func(text string, targets ...target) question {
		return question{Text: text, Group: relevant, Expected: targets}
	}
	n := func(text string) question { return question{Text: text, Group: near} }
	u := func(text string) question { return question{Text: text, Group: unrelated} }
	id := func(id string) target { return target{ID: id} }

	return []question{
		// Seed data (2026-11-04)
		r("Welchen Hämoglobinwert hatte ich im Oktober?", id(sampledata.LabReport)),
		r("Wie hoch waren meine Leukozyten?", id(sampledata.LabReport)),
		r("Was stand beim CRP?", id(sampledata.LabReport)),
		r("Welches Labor hat mein Blutbild gemacht?", id(sampledata.LabReport)),
		r("Wie hoch war mein Blutdruck bei der Routinekontrolle?", id(sampledata.DoctorLetter)),
		r("Wann soll ich wieder zur Kontrolle bei Dr. Beispiel?", id(sampledata.DoctorLetter)),
		r("Welches Medikament wurde mir verschrieben?", id(sampledata.Prescription)),
		r("Wie oft am Tag soll ich Beispielin nehmen?", id(sampledata.Prescription)),
		r("Wie viele Tabletten stehen auf dem Rezept?", id(sampledata.Prescription)),
		r("Wo liegt mein Impfpass?", id(sampledata.Note)),

		// Near until 2026-11-07; since the lipid profiles the answer is in both lab reports.
		r("Wie hoch war mein Cholesterin?", labJanuary, labJuly),
		n("Wann hatte ich meine letzte Tetanus-Impfung?"),
		n("Welche Allergien habe ich?"),

		u("Wie wird das Wetter morgen in Salzburg?"),
		u("Wer hat die Fußball-WM 2014 gewonnen?"),
		u("Wie koche ich Spaghetti Carbonara?"),
		u("Was kostet eine Zugfahrt von Salzburg nach Wien?"),
		// Unrelated until 2026-11-07; since the physio note ("wieder 5 km
		// gelaufen") there is a connection, but the answer for "last week" is
		// not in it.
		n("Wie viele Kilometer bin ich letzte Woche gelaufen?"),

		// Demo data (2026-11-08): one document
		r("Wie hoch war mein LDL-Cholesterin im Januar?", labJanuary),
		r("Wie hoch war mein Nüchternblutzucker im Januar?", labJanuary),
		r("Wie hoch war mein HDL-Cholesterin bei der Kontrolle im Juli?", labJuly),
		r("Wie hoch waren meine Triglyceride im Juli?", labJuly),
		r("Warum war ich beim Orthopäden?", orthopedics),
		r("Was hat das Röntgen meines Knies ergeben?", orthopedics),
		r("Wie lange sollte ich mit dem Laufen pausieren?", orthopedics),
		r("An welche Physiotherapie-Praxis wurde ich überwiesen?", referral),
		r("Wie oft pro Woche soll ich zur Physiotherapie?", referral),
		r("Wann muss ich die FSME-Impfung auffrischen?", tbe),
		r("Welcher Impfstoff wurde bei der FSME-Impfung verwendet?", tbe),
		r("Wie hoch war mein Puls bei der Messung zu Hause?", bloodPressure),
		r("Wann habe ich die Physiotherapie abgeschlossen?", physioEnd),

		// Demo data: the answer needs two documents
		r("Wie hat sich mein LDL-Cholesterin verändert?", labJanuary, labJuly),
		r("Wie hat sich mein Gesamtcholesterin zwischen Januar und Juli entwickelt?", labJanuary, labJuly),
		r("Warum wurde mir Physiotherapie verordnet und wie oft pro Woche?", orthopedics, referral),
		r("Wie lange hat es von der Überweisung bis zum Abschluss der Physiotherapie gedauert?", referral, physioEnd),
		r("Wie hat sich mein Blutdruck seit der Routinekontrolle verändert?", id(sampledata.DoctorLetter), bloodPressure),

		n("Wurde bei mir schon ein MRT gemacht?"),
		n("Wie hoch war mein HbA1c-Wert?"),
		n("Wie hoch war mein Vitamin-D-Wert?"),
		n("Wann hatte ich meine letzte Covid-Impfung?"),
		n("Welche Blutgruppe habe ich?"),

		u("Hast du ein Rezept für Apfelkuchen?"),
		u("Bis wann muss ich meine Steuererklärung abgeben?"),
		u("Was kostet ein Flug nach Rom?"),
		u("Wie wechsle ich einen Fahrradschlauch?"),
	}
}
