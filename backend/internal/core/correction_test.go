package core

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	t1 = time.Date(2026, 11, 4, 19, 5, 31, 0, time.UTC)
	t2 = time.Date(2026, 11, 4, 19, 7, 0, 0, time.UTC)
)

// correctionEntry: synthetic lab report without corrections.
func correctionEntry() Entry {
	return Entry{
		ID: "6727a1f3c2b4e5d6f7a8b9c0", Module: "health", Type: "lab_report",
		DocumentDate: ptr("2026-10-14"), Issuer: ptr("Labor Muster GmbH"),
		FullText: "Hämoglobin 14,2 g/dl …",
		Facts: []Fact{
			{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")},
			{ID: "f2", Name: "Leukozyten", Value: "6,3", Unit: ptr("G/l")},
			{ID: "f3", Name: "Befundart", Value: "Blutbild klein"},
		},
		Corrections: []Correction{},
	}
}

func set(s string) Nullable { return Nullable{Set: true, Value: &s} }

var remove = Nullable{Set: true}

// mustCorrect calls Correct and stops on an error.
func mustCorrect(t *testing.T, e Entry, c Change, now time.Time) (Entry, []string) {
	t.Helper()
	updated, changed, err := Correct(e, c, now)
	if err != nil {
		t.Fatalf("Correct: %v", err)
	}
	return updated, changed
}

func text(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

func checkCorrections(t *testing.T, got []Correction, want ...Correction) {
	t.Helper()
	if got == nil {
		t.Fatal("corrections is nil, want a list")
	}
	if len(got) != len(want) {
		t.Fatalf("%d corrections, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Field != w.Field || text(g.Recognized) != text(w.Recognized) || text(g.Corrected) != text(w.Corrected) || !g.CorrectedAt.Equal(w.CorrectedAt) {
			t.Errorf("correction %d = {%s %s→%s %v}, want {%s %s→%s %v}", i,
				g.Field, text(g.Recognized), text(g.Corrected), g.CorrectedAt,
				w.Field, text(w.Recognized), text(w.Corrected), w.CorrectedAt)
		}
	}
}

func TestCorrect_FirstCorrectionAsInAPIExample(t *testing.T) {
	// API.md 3.5: date and leukocytes in one request.
	updated, changed := mustCorrect(t, correctionEntry(), Change{
		DocumentDate: set("2026-10-15"),
		Facts:        []FactChange{{ID: "f2", Value: ptr("6,8")}},
	}, t1)

	// Rule 1: the fields hold the current value.
	if text(updated.DocumentDate) != "2026-10-15" || updated.Facts[1].Value != "6,8" {
		t.Errorf("date %s, f2 %s", text(updated.DocumentDate), updated.Facts[1].Value)
	}
	checkCorrections(t, updated.Corrections,
		Correction{Field: "documentDate", Recognized: ptr("2026-10-14"), Corrected: ptr("2026-10-15"), CorrectedAt: t1},
		Correction{Field: "facts.f2.value", Recognized: ptr("6,3"), Corrected: ptr("6,8"), CorrectedAt: t1},
	)
	if !slices.Equal(changed, []string{"documentDate", "facts.f2.value"}) {
		t.Errorf("changed = %v", changed)
	}
	// Everything else stays as it was.
	old := correctionEntry()
	if updated.Type != old.Type || text(updated.Issuer) != text(old.Issuer) || updated.FullText != old.FullText ||
		!reflect.DeepEqual(updated.Facts[0], old.Facts[0]) || !reflect.DeepEqual(updated.Facts[2], old.Facts[2]) {
		t.Errorf("unrelated fields changed: %+v", updated)
	}
}

func TestCorrect_CorrectedTwiceKeepsRecognized(t *testing.T) {
	// Rule 2: one item per field, recognized stays the original value.
	e, _ := mustCorrect(t, correctionEntry(), Change{Facts: []FactChange{{ID: "f2", Value: ptr("6,8")}}}, t1)
	e, _ = mustCorrect(t, e, Change{Facts: []FactChange{{ID: "f2", Value: ptr("6,9")}}}, t2)

	checkCorrections(t, e.Corrections,
		Correction{Field: "facts.f2.value", Recognized: ptr("6,3"), Corrected: ptr("6,9"), CorrectedAt: t2})
}

func TestCorrect_TypeTwice(t *testing.T) {
	// Catches the pointer bug: read must not return a pointer into the field
	// for the type, otherwise "recognized" would be the new value after
	// writing and the correction would disappear as "reset".
	e, _ := mustCorrect(t, correctionEntry(), Change{Type: ptr("doctor_letter")}, t1)
	checkCorrections(t, e.Corrections,
		Correction{Field: "type", Recognized: ptr("lab_report"), Corrected: ptr("doctor_letter"), CorrectedAt: t1})

	e, _ = mustCorrect(t, e, Change{Type: ptr("prescription")}, t2)
	if e.Type != "prescription" {
		t.Errorf("type = %s", e.Type)
	}
	checkCorrections(t, e.Corrections,
		Correction{Field: "type", Recognized: ptr("lab_report"), Corrected: ptr("prescription"), CorrectedAt: t2})
}

func TestCorrect_ResetRemovesCorrection(t *testing.T) {
	// Rule 3, for every kind of field.
	cases := []struct {
		name        string
		there, back Change
	}{
		{"type", Change{Type: ptr("prescription")}, Change{Type: ptr("lab_report")}},
		{"date", Change{DocumentDate: set("2026-10-15")}, Change{DocumentDate: set("2026-10-14")}},
		{"date removed and back", Change{DocumentDate: remove}, Change{DocumentDate: set("2026-10-14")}},
		{"issuer", Change{Issuer: set("Labor B")}, Change{Issuer: set("Labor Muster GmbH")}},
		{"value", Change{Facts: []FactChange{{ID: "f2", Value: ptr("6,8")}}}, Change{Facts: []FactChange{{ID: "f2", Value: ptr("6,3")}}}},
		{"unit removed", Change{Facts: []FactChange{{ID: "f1", Unit: remove}}}, Change{Facts: []FactChange{{ID: "f1", Unit: set("g/dl")}}}},
		{"unit added", Change{Facts: []FactChange{{ID: "f3", Unit: set("x")}}}, Change{Facts: []FactChange{{ID: "f3", Unit: remove}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _ := mustCorrect(t, correctionEntry(), c.there, t1)
			if len(e.Corrections) != 1 {
				t.Fatalf("after correcting %d corrections", len(e.Corrections))
			}
			e, changed := mustCorrect(t, e, c.back, t2)
			checkCorrections(t, e.Corrections) // empty
			if !reflect.DeepEqual(e, correctionEntry()) {
				t.Errorf("after reset not as before:\n%+v", e)
			}
			if len(changed) != 1 {
				t.Errorf("changed = %v", changed)
			}
		})
	}
}

func TestCorrect_NullValues(t *testing.T) {
	// The date was not recognized (null) and is set: recognized is null.
	noDate := correctionEntry()
	noDate.DocumentDate = nil
	e, _ := mustCorrect(t, noDate, Change{DocumentDate: set("2026-10-14")}, t1)
	checkCorrections(t, e.Corrections,
		Correction{Field: "documentDate", Recognized: nil, Corrected: ptr("2026-10-14"), CorrectedAt: t1})

	// Remove the issuer: corrected is null.
	e, _ = mustCorrect(t, correctionEntry(), Change{Issuer: remove}, t1)
	if e.Issuer != nil {
		t.Errorf("issuer = %s", text(e.Issuer))
	}
	checkCorrections(t, e.Corrections,
		Correction{Field: "issuer", Recognized: ptr("Labor Muster GmbH"), Corrected: nil, CorrectedAt: t1})

	// Add a unit where there was none, and remove one.
	e, _ = mustCorrect(t, correctionEntry(), Change{Facts: []FactChange{
		{ID: "f3", Unit: set("Stück")}, {ID: "f1", Unit: remove},
	}}, t1)
	if text(e.Facts[2].Unit) != "Stück" || e.Facts[0].Unit != nil {
		t.Errorf("units: f3 %s, f1 %s", text(e.Facts[2].Unit), text(e.Facts[0].Unit))
	}
	checkCorrections(t, e.Corrections,
		Correction{Field: "facts.f3.unit", Recognized: nil, Corrected: ptr("Stück"), CorrectedAt: t1},
		Correction{Field: "facts.f1.unit", Recognized: ptr("g/dl"), Corrected: nil, CorrectedAt: t1},
	)
}

func TestCorrect_NoChangeNoCorrection(t *testing.T) {
	// Rule 4, also with whitespace and with null where null already is.
	noDate := correctionEntry()
	noDate.DocumentDate = nil
	for name, c := range map[string]Change{
		"same type":        {Type: ptr(" lab_report ")},
		"same date":        {DocumentDate: set("2026-10-14")},
		"same issuer":      {Issuer: set("  Labor Muster GmbH ")},
		"same value":       {Facts: []FactChange{{ID: "f1", Value: ptr("14,2 ")}}},
		"no unit":          {Facts: []FactChange{{ID: "f3", Unit: remove}}},
		"empty unit":       {Facts: []FactChange{{ID: "f3", Unit: set("  ")}}},
		"empty facts list": {Facts: []FactChange{}},
	} {
		t.Run(name, func(t *testing.T) {
			updated, changed := mustCorrect(t, correctionEntry(), c, t1)
			if len(changed) != 0 || !reflect.DeepEqual(updated, correctionEntry()) {
				t.Errorf("changed %v, entry %+v", changed, updated)
			}
		})
	}
	updated, changed := mustCorrect(t, noDate, Change{DocumentDate: remove}, t1)
	if len(changed) != 0 || len(updated.Corrections) != 0 {
		t.Errorf("null to null: changed %v, %+v", changed, updated.Corrections)
	}
}

func TestCorrect_Whitespace(t *testing.T) {
	updated, _ := mustCorrect(t, correctionEntry(), Change{
		Type:         ptr(" doctor_letter\n"),
		DocumentDate: set(" 2026-10-15 "),
		Issuer:       set("   "), // empty → null
		Facts:        []FactChange{{ID: "f2", Value: ptr(" 6,8 "), Unit: set("")}},
	}, t1)
	if updated.Type != "doctor_letter" || text(updated.DocumentDate) != "2026-10-15" || updated.Issuer != nil ||
		updated.Facts[1].Value != "6,8" || updated.Facts[1].Unit != nil {
		t.Errorf("= type %q date %s issuer %s value %q unit %s", updated.Type, text(updated.DocumentDate),
			text(updated.Issuer), updated.Facts[1].Value, text(updated.Facts[1].Unit))
	}
}

func TestCorrect_InputStaysUnchanged(t *testing.T) {
	// Entry with one existing correction that is changed (rule 2) and one
	// that is removed (rule 3).
	e, _ := mustCorrect(t, correctionEntry(), Change{
		Issuer: set("Labor B"),
		Facts:  []FactChange{{ID: "f2", Value: ptr("6,8")}, {ID: "f1", Unit: remove}},
	}, t1)
	before := deepCopy(e)

	_, _ = mustCorrect(t, e, Change{
		Issuer: set("Labor C"),
		Facts:  []FactChange{{ID: "f2", Value: ptr("6,3")}, {ID: "f1", Value: ptr("14,0")}},
	}, t2)

	if !reflect.DeepEqual(e, before) {
		t.Errorf("input changed:\nbefore %+v\nafter  %+v", before, e)
	}
}

// deepCopy: deep copy, so the comparison also catches shared pointers.
func deepCopy(e Entry) Entry {
	c := e
	c.Facts = nil
	for _, f := range e.Facts {
		if f.Unit != nil {
			f.Unit = ptr(*f.Unit)
		}
		c.Facts = append(c.Facts, f)
	}
	c.Corrections = nil
	for _, co := range e.Corrections {
		if co.Recognized != nil {
			co.Recognized = ptr(*co.Recognized)
		}
		if co.Corrected != nil {
			co.Corrected = ptr(*co.Corrected)
		}
		c.Corrections = append(c.Corrections, co)
	}
	if e.Issuer != nil {
		c.Issuer = ptr(*e.Issuer)
	}
	if e.DocumentDate != nil {
		c.DocumentDate = ptr(*e.DocumentDate)
	}
	return c
}

func TestCorrect_NilCorrectionsBecomeList(t *testing.T) {
	e := correctionEntry()
	e.Corrections = nil
	updated, _ := mustCorrect(t, e, Change{Type: ptr("lab_report")}, t1)
	if updated.Corrections == nil {
		t.Error("corrections nil")
	}
}

func TestCorrect_CorrectedAtInUTC(t *testing.T) {
	vienna := time.FixedZone("CET", 3600)
	updated, _ := mustCorrect(t, correctionEntry(), Change{Type: ptr("prescription")}, t1.In(vienna))
	if updated.Corrections[0].CorrectedAt.Location() != time.UTC || !updated.Corrections[0].CorrectedAt.Equal(t1) {
		t.Errorf("CorrectedAt = %v", updated.Corrections[0].CorrectedAt)
	}
}

func TestCorrect_Errors(t *testing.T) {
	cases := []struct {
		name    string
		c       Change
		kind    error
		field   string
		factID  string
		value   any
		message string
	}{
		{"empty", Change{}, ErrInvalidCorrection, "", "", nil, "keine Änderung"},
		{"unknown type", Change{Type: ptr("befund")}, ErrInvalidCorrection, "type", "", "befund",
			"Erlaubt sind doctor_letter, lab_report, prescription, other."},
		{"date format", Change{DocumentDate: set("15.10.2026")}, ErrInvalidCorrection, "documentDate", "", "15.10.2026",
			"Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden."},
		{"date empty", Change{DocumentDate: set(" ")}, ErrInvalidCorrection, "documentDate", "", " ", "Format JJJJ-MM-TT"},
		{"no such day", Change{DocumentDate: set("2026-02-31")}, ErrInvalidCorrection, "documentDate", "", "2026-02-31",
			"Das Dokumentdatum ist kein gültiges Kalenderdatum."},
		{"fact without id", Change{Facts: []FactChange{{Value: ptr("1")}}}, ErrInvalidCorrection, "facts", "", nil, "braucht eine id"},
		{"fact twice", Change{Facts: []FactChange{{ID: "f1", Value: ptr("1")}, {ID: "f1", Unit: remove}}},
			ErrInvalidCorrection, "", "f1", nil, "mehrfach"},
		{"fact without change", Change{Facts: []FactChange{{ID: "f1"}}}, ErrInvalidCorrection, "", "f1", nil, "keine Änderung"},
		{"unknown fact", Change{Facts: []FactChange{{ID: "f9", Value: ptr("1")}}}, ErrFactNotFound, "", "f9", nil, "gibt es in dem Eintrag nicht"},
		{"empty value", Change{Facts: []FactChange{{ID: "f2", Value: ptr("  ")}}}, ErrInvalidCorrection, "facts.f2.value", "", "  ", "nicht leer"},
		// Valid parts first, invalid fact afterwards: error, nothing changed.
		{"partly valid", Change{Issuer: set("Labor B"), Facts: []FactChange{{ID: "f2", Value: ptr("6,8")}, {ID: "f7", Value: ptr("1")}}},
			ErrFactNotFound, "", "f7", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := correctionEntry()
			updated, changed, err := Correct(e, c.c, t1)
			if !errors.Is(err, c.kind) {
				t.Fatalf("err = %v, want %v", err, c.kind)
			}
			var ce *CorrectionError
			if !errors.As(err, &ce) {
				t.Fatalf("not a CorrectionError: %T", err)
			}
			if ce.Field != c.field || ce.FactID != c.factID || !reflect.DeepEqual(ce.Value, c.value) {
				t.Errorf("field %q factID %q value %#v, want %q %q %#v", ce.Field, ce.FactID, ce.Value, c.field, c.factID, c.value)
			}
			if !strings.Contains(ce.Message, c.message) {
				t.Errorf("message %q does not contain %q", ce.Message, c.message)
			}
			if updated.ID != "" || changed != nil {
				t.Errorf("on error result %+v, changed %v", updated, changed)
			}
			if !reflect.DeepEqual(e, correctionEntry()) {
				t.Error("input changed on error")
			}
			// Error() is for the log: no value, but the target.
			if s, ok := c.value.(string); ok && strings.TrimSpace(s) != "" && strings.Contains(err.Error(), s) {
				t.Errorf("Error() contains the value: %q", err.Error())
			}
		})
	}
}

func TestCorrectionError_ErrorAndConstructors(t *testing.T) {
	cases := map[string]error{
		"invalid correction":                   Invalid("x", "", nil),
		"invalid correction: issuer":           Invalid("x", "issuer", 42),
		"invalid correction: facts.f1":         &CorrectionError{Kind: ErrInvalidCorrection, FactID: "f1"},
		"field not correctable: fullText":      NotCorrectable("fullText"),
		"field not correctable: facts.f1.name": NotCorrectable("facts.f1.name"),
	}
	for want, err := range cases {
		if err.Error() != want {
			t.Errorf("Error() = %q, want %q", err.Error(), want)
		}
	}
	nc := NotCorrectable("fullText")
	if !errors.Is(nc, ErrFieldNotCorrectable) || errors.Is(nc, ErrInvalidCorrection) {
		t.Error("NotCorrectable has the wrong kind")
	}
	if nc.Message != "Das Feld fullText kann nicht korrigiert werden." {
		t.Errorf("message = %q", nc.Message)
	}
}
