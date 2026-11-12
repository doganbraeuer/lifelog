package main

import "lifelog/backend/internal/core"

// term is one key term of a sentence: the forms in which it counts as
// recognized (compared word by word with core.TextContains). Key terms are
// what the app needs later: numbers with units, dates, names, medication.
// If Whisper writes "400 mg" instead of "400 Milligramm", that is fine; if
// it writes "40 mg", the term is missing. Every accepted spelling is listed
// here, so the list shows what counts as right.
type term []string

// date: every common German form of the date counts (core.DateForms), e.g.
// "20. Jänner 2026", "20. Januar 2026", "20.01.2026".
func date(d string) term { return term(core.DateForms(d)) }

// sample is one recorded sentence (docs/speech/README.md).
type sample struct {
	ID    string // "01" … "10": own recording <ID>.m4a, synthetic voice <ID>_anna.m4a
	Ref   string // reference text demo/speech/<Ref>.txt; "" = nothing is said, the result must be NO_SPEECH
	What  string // what the recording is, for the report
	Terms []term
}

var (
	termsBloodPressure = []term{
		{"Blutdruck"},
		{"122 zu 79", "122/79", "122 / 79"},
		{"Puls 64"},
		{"im Sitzen"},
	}
	termsDoctor = term{"Dr. Erika Beispiel", "Doktor Erika Beispiel"}
)

// samples are the sentences the measurement uses. 01–08 are read aloud;
// 09 is silence in a quiet room and 10 is sentence 03 with background
// noise, both only as own recordings.
var samples = []sample{
	{ID: "01", Ref: "01", What: "lab report, date and values", Terms: []term{
		date("2026-01-20"),
		{"Labor Muster", "Labormuster"},
		{"Gesamtcholesterin"},
		{"232"},
		{"Milligramm pro Deziliter", "mg/dl", "mg pro dl", "mg pro Deziliter", "Milligramm/Deziliter"},
		{"LDL 154"},
		{"HDL 48"},
	}},
	{ID: "02", Ref: "02", What: "prescription, dose scheme", Terms: []term{
		date("2026-05-18"),
		{"Musterol"},
		{"400 Milligramm", "400 mg"},
		// Not "101": a reader cannot tell it from 101 pieces.
		{"eins null eins", "1-0-1", "1 - 0 - 1", "eins-null-eins"},
		{"20 Stück", "zwanzig Stück"},
	}},
	{ID: "03", Ref: "03", What: "blood pressure note", Terms: termsBloodPressure},
	{ID: "04", Ref: "04", What: "referral, diagnosis and doctor", Terms: []term{
		{"Überweisung"},
		{"Physiotherapie"},
		{"sechs Einheiten", "6 Einheiten"},
		{"Gonarthrose rechts"},
		termsDoctor,
		{"Allgemeinmedizin"},
	}},
	{ID: "05", Ref: "05", What: "vaccination, two dates", Terms: []term{
		{"FSME-Auffrischung", "FSME Auffrischung", "FSME-Auffrischungsimpfung"},
		date("2026-06-10"),
		{"Juni 2029", "06/2029"},
	}},
	{ID: "06", Ref: "06", What: "lab values with decimal comma and units", Terms: []term{
		{"TSH 1,8"},
		{"Milliunits pro Liter", "mU/l", "Milli-Units pro Liter", "Milliunits/Liter", "mU pro Liter"},
		{"Vitamin D 32"},
		{"Nanogramm pro Milliliter", "ng/ml", "ng pro ml", "Nanogramm/Milliliter"},
		{"Ferritin 85"},
	}},
	{ID: "07", Ref: "07", What: "follow-up, date and medication", Terms: []term{
		{"Orthopäden"},
		date("2026-04-30"),
		{"ohne Beschwerden"},
		{"Ibuprofen"},
		{"bei Bedarf"},
	}},
	{ID: "08", Ref: "08", What: "spoken doctor's letter (about 40 s)", Terms: []term{
		termsDoctor,
		date("2026-11-12"),
		{"zwei Wochen", "2 Wochen"},
		{"rechten Knie"},
		{"kein Erguss"},
		{"Ultraschall"},
		{"Ibuprofen 400 Milligramm", "Ibuprofen 400 mg"},
		{"dreimal täglich", "drei Mal täglich", "3-mal täglich", "3 mal täglich", "3 Mal täglich", "3x täglich"},
		{"höchstens eine Woche", "höchstens 1 Woche"},
		{"drei Wochen", "3 Wochen"},
		{"Physiotherapie"},
		{"sechs Einheiten", "6 Einheiten"},
	}},
	{ID: "09", Ref: "", What: "silence in a quiet room"},
	{ID: "10", Ref: "03", What: "sentence 03 with background noise", Terms: termsBloodPressure},
}

// spoken: the samples that are read aloud, i.e. have a synthetic version.
func spoken() []sample {
	var out []sample
	for _, s := range samples {
		if s.Ref == s.ID {
			out = append(out, s)
		}
	}
	return out
}
