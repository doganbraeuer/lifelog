package core

import (
	"slices"
	"testing"
	"time"
)

// Date for text imports and voice notes (API.md 3.1, section 2
// documentDateSource, verification and 3.5).

func TestLocalDay_Vienna(t *testing.T) {
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 11, 23, 9, 46, 0, 0, time.UTC), "2026-11-23"},
		// Winter time (UTC+1): 23:30 UTC is already the next day.
		{time.Date(2026, 11, 23, 22, 59, 59, 0, time.UTC), "2026-11-23"},
		{time.Date(2026, 11, 23, 23, 0, 0, 0, time.UTC), "2026-11-24"},
		// Summer time (UTC+2): from 22:00 UTC.
		{time.Date(2026, 6, 30, 21, 59, 0, 0, time.UTC), "2026-06-30"},
		{time.Date(2026, 6, 30, 22, 0, 0, 0, time.UTC), "2026-07-01"},
	} {
		if got := LocalDay(c.at); got != c.want {
			t.Errorf("LocalDay(%v) = %s, want %s", c.at, got, c.want)
		}
	}
}

func TestImportDate(t *testing.T) {
	// Without a date: the day of the import in Vienna, marked "import".
	e := Entry{ImportedAt: time.Date(2026, 11, 23, 23, 30, 0, 0, time.UTC), DocumentDateSource: DateSourceDocument}
	ImportDate(&e)
	if e.DocumentDate == nil || *e.DocumentDate != "2026-11-24" || e.DocumentDateSource != DateSourceImport {
		t.Errorf("without date: %v %q, want 2026-11-24 import", text(e.DocumentDate), e.DocumentDateSource)
	}

	// With a date from the text: unchanged.
	e = Entry{DocumentDate: ptr("2026-11-20"), ImportedAt: time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC),
		DocumentDateSource: DateSourceDocument}
	ImportDate(&e)
	if *e.DocumentDate != "2026-11-20" || e.DocumentDateSource != DateSourceDocument {
		t.Errorf("with date: %v %q", text(e.DocumentDate), e.DocumentDateSource)
	}
}

func TestRelativeDateWords(t *testing.T) {
	// Import on Monday 2026-11-23, 09:46 in Vienna.
	imported := time.Date(2026, 11, 23, 8, 46, 0, 0, time.UTC)
	for _, c := range []struct {
		date string
		want []string
	}{
		{"2026-11-23", []string{"heute"}},
		{"2026-11-22", []string{"gestern", "Sonntag"}},
		{"2026-11-21", []string{"vorgestern", "Samstag"}},
		{"2026-11-17", []string{"Dienstag"}},
		{"2026-11-16", []string{"Montag"}}, // "letzten Montag", 7 days before
		{"2026-11-15", nil},                // 8 days before: too far
		{"2026-11-24", nil},                // after the import day
		{"23.11.2026", nil},                // not a date in the format YYYY-MM-DD
	} {
		if got := RelativeDateWords(c.date, imported); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.date, got, c.want)
		}
	}
	// The import day is the day in Vienna: 23:30 UTC on the 23rd is the 24th.
	late := time.Date(2026, 11, 23, 23, 30, 0, 0, time.UTC)
	if got := RelativeDateWords("2026-11-23", late); !slices.Equal(got, []string{"gestern", "Montag"}) {
		t.Errorf("after midnight in Vienna: %v", got)
	}
}

// A date from the import day is not in the text: it is not checked at all,
// so it is never shown as "not found".
func TestVerifyText_ImportDateNotChecked(t *testing.T) {
	e := Entry{
		DocumentDate:       ptr("2026-11-23"),
		DocumentDateSource: DateSourceImport,
		ImportedAt:         time.Date(2026, 11, 23, 9, 46, 0, 0, time.UTC),
		FullText:           "Blutdruck 130/85 mmHg",
		Facts:              []Fact{{ID: "f1", Name: "Blutdruck", Value: "130/85", Unit: ptr("mmHg")}},
	}
	v := VerifyText(e, checkTime)
	if slices.ContainsFunc(v.Fields, func(f FieldCheck) bool { return f.Field == "documentDate" }) {
		t.Errorf("documentDate checked: %+v", v.Fields)
	}
	if v.Total != 1 || v.Confirmed != 1 || *v.MatchPercent() != 100 {
		t.Errorf("total %d, confirmed %d, want 1 and 1", v.Total, v.Confirmed)
	}

	// The same date from the document is checked as before.
	e.DocumentDateSource = DateSourceDocument
	if fc := field(t, VerifyText(e, checkTime), "documentDate"); fc.Status != FieldUnconfirmed {
		t.Errorf("date from the document: %s, want unconfirmed", fc.Status)
	}
}

// The model resolves "gestern" against the import day; the check finds the
// word, so such a date is not red either.
func TestVerifyText_RelativeWords(t *testing.T) {
	imported := time.Date(2026, 11, 23, 8, 46, 0, 0, time.UTC) // Monday
	for _, c := range []struct {
		name, text, date string
		want             string // status of documentDate
		marked           string // text of the location
	}{
		{"gestern", "Gestern Blutdruck gemessen: 130/85", "2026-11-22", FieldConfirmed, "Gestern"},
		{"heute", "Heute früh Kopfschmerzen.", "2026-11-23", FieldConfirmed, "Heute"},
		{"vorgestern", "Vorgestern beim Hausarzt.", "2026-11-21", FieldConfirmed, "Vorgestern"},
		{"letzten Montag", "Letzten Montag Zahnarzt, alles gut.", "2026-11-16", FieldConfirmed, "Montag"},
		{"written date wins", "Am 22.11.2026, also gestern, gemessen.", "2026-11-22", FieldConfirmed, "22.11.2026"},
		{"wrong word", "Gestern Blutdruck gemessen.", "2026-11-21", FieldUnconfirmed, ""},
		{"no word", "Blutdruck gemessen.", "2026-11-22", FieldUnconfirmed, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := Entry{DocumentDate: ptr(c.date), DocumentDateSource: DateSourceDocument,
				ImportedAt: imported, FullText: c.text}
			fc := field(t, VerifyText(e, checkTime), "documentDate")
			if fc.Status != c.want {
				t.Fatalf("status %s, want %s", fc.Status, c.want)
			}
			if c.marked == "" {
				return
			}
			l := fc.Locations[0]
			if got := string([]rune(c.text)[l.Start:l.End]); got != c.marked {
				t.Errorf("marked %q, want %q", got, c.marked)
			}
		})
	}
}

// A document on paper has its own date: "gestern" in it does not confirm.
func TestVerify_FileWithoutRelativeWords(t *testing.T) {
	e := Entry{DocumentDate: ptr("2026-11-22"), DocumentDateSource: DateSourceDocument,
		ImportedAt: time.Date(2026, 11, 23, 8, 46, 0, 0, time.UTC)}
	fc := field(t, Verify(e, ocrPage("Befund von gestern"), checkTime), "documentDate")
	if fc.Status != FieldUnconfirmed {
		t.Errorf("file: %s, want unconfirmed", fc.Status)
	}
}

func importDatedEntry() Entry {
	e := correctionEntry()
	e.DocumentDate = ptr("2026-11-23")
	e.DocumentDateSource = DateSourceImport
	return e
}

// A PATCH with documentDate removes the mark "import". The import day was
// never recognized, so a new date is a correction from null.
func TestCorrect_ImportDate(t *testing.T) {
	t.Run("new date", func(t *testing.T) {
		got, changed := mustCorrect(t, importDatedEntry(), Change{DocumentDate: set("2026-11-20")}, t1)
		if text(got.DocumentDate) != "2026-11-20" || got.DocumentDateSource != DateSourceDocument {
			t.Errorf("date %s, source %q", text(got.DocumentDate), got.DocumentDateSource)
		}
		checkCorrections(t, got.Corrections, Correction{Field: "documentDate", Recognized: nil, Corrected: ptr("2026-11-20"), CorrectedAt: t1})
		if !slices.Equal(changed, []string{"documentDate"}) {
			t.Errorf("changed = %v", changed)
		}
	})
	t.Run("the same day confirms it", func(t *testing.T) {
		got, changed := mustCorrect(t, importDatedEntry(), Change{DocumentDate: set("2026-11-23")}, t1)
		if text(got.DocumentDate) != "2026-11-23" || got.DocumentDateSource != DateSourceDocument {
			t.Errorf("date %s, source %q", text(got.DocumentDate), got.DocumentDateSource)
		}
		checkCorrections(t, got.Corrections, Correction{Field: "documentDate", Recognized: nil, Corrected: ptr("2026-11-23"), CorrectedAt: t1})
		if !slices.Equal(changed, []string{"documentDate"}) {
			t.Errorf("changed = %v", changed)
		}
	})
	t.Run("null removes the date without a correction", func(t *testing.T) {
		got, changed := mustCorrect(t, importDatedEntry(), Change{DocumentDate: remove}, t1)
		if got.DocumentDate != nil || got.DocumentDateSource != DateSourceDocument {
			t.Errorf("date %s, source %q", text(got.DocumentDate), got.DocumentDateSource)
		}
		checkCorrections(t, got.Corrections)
		// Stored: the entry in the database still has the import day.
		if !slices.Equal(changed, []string{"documentDate"}) {
			t.Errorf("changed = %v, want documentDate", changed)
		}
	})
	t.Run("another field keeps the mark", func(t *testing.T) {
		got, _ := mustCorrect(t, importDatedEntry(), Change{Issuer: set("Hausarzt")}, t1)
		if text(got.DocumentDate) != "2026-11-23" || got.DocumentDateSource != DateSourceImport {
			t.Errorf("date %s, source %q", text(got.DocumentDate), got.DocumentDateSource)
		}
	})
}
