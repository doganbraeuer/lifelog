package core

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

// valid is a synthetic extraction as Gemini could return it.
func valid() Extraction {
	return Extraction{
		Readable:     true,
		DocumentDate: ptr("2026-10-14"),
		Issuer:       ptr("Labor Muster GmbH, Salzburg"),
		Type:         "lab_report",
		FullText:     labReport,
		Facts: []ExtractedFact{
			{Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")},
			{Name: "CRP", Value: "<0,5", Unit: ptr("mg/dl")},
			{Name: "Befundart", Value: "Blutbild klein"},
		},
	}
}

func TestCheckExtraction_Valid(t *testing.T) {
	e, dropped, err := CheckExtraction(valid(), "health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped == nil || len(dropped) != 0 {
		t.Errorf("dropped = %#v, want empty list", dropped)
	}
	if e.Module != "health" || e.Type != "lab_report" {
		t.Errorf("module/type = %q/%q", e.Module, e.Type)
	}
	if *e.DocumentDate != "2026-10-14" || *e.Issuer != "Labor Muster GmbH, Salzburg" {
		t.Errorf("date/issuer = %v/%v", *e.DocumentDate, *e.Issuer)
	}
	if e.FullText != labReport {
		t.Error("full text was changed")
	}
	if e.Corrections == nil || len(e.Corrections) != 0 {
		t.Errorf("corrections = %#v, want empty list", e.Corrections)
	}

	if len(e.Facts) != 3 {
		t.Fatalf("facts = %d", len(e.Facts))
	}
	for i, id := range []string{"f1", "f2", "f3"} {
		if e.Facts[i].ID != id {
			t.Errorf("Facts[%d].ID = %q, want %q", i, e.Facts[i].ID, id)
		}
	}
	if e.Facts[1].Value != "<0,5" {
		t.Errorf("value changed: %q", e.Facts[1].Value)
	}
	if e.Facts[2].Unit != nil {
		t.Error("fact without unit must have nil")
	}
}

func TestCheckExtraction_EmptyValuesBecomeNil(t *testing.T) {
	x := valid()
	x.DocumentDate = ptr("  ")
	x.Issuer = ptr("")
	x.Facts = []ExtractedFact{{Name: " Hämoglobin ", Value: " 14,2 ", Unit: ptr(" ")}}

	e, _, err := CheckExtraction(x, "health")
	if err != nil {
		t.Fatal(err)
	}
	if e.DocumentDate != nil || e.Issuer != nil {
		t.Errorf("empty date/issuer must be nil: %v %v", e.DocumentDate, e.Issuer)
	}
	f := e.Facts[0]
	if f.Name != "Hämoglobin" || f.Value != "14,2" || f.Unit != nil {
		t.Errorf("fact = %+v", f)
	}
}

func TestCheckExtraction_NoFactsGivesEmptyList(t *testing.T) {
	x := valid()
	x.Facts = nil
	e, _, err := CheckExtraction(x, "health")
	if err != nil {
		t.Fatal(err)
	}
	if e.Facts == nil {
		t.Error("facts must be an empty list, not nil")
	}
}

func TestCheckExtraction_Unreadable(t *testing.T) {
	x := valid()
	x.Readable = false
	if _, _, err := CheckExtraction(x, "health"); !errors.Is(err, ErrUnreadable) {
		t.Errorf("want ErrUnreadable, got %v", err)
	}
}

func TestCheckExtraction_Errors(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Extraction)
		module string
		field  string
	}{
		{"unknown type", func(x *Extraction) { x.Type = "kochrezept" }, "health", "type"},
		{"empty type", func(x *Extraction) { x.Type = "" }, "health", "type"},
		{"type in another module", func(x *Extraction) {}, "sport", "type"},
		{"empty full text", func(x *Extraction) { x.FullText = " \n " }, "health", "fullText"},
		{"German date format", func(x *Extraction) { x.DocumentDate = ptr("14.10.2026") }, "health", "documentDate"},
		{"date does not exist", func(x *Extraction) { x.DocumentDate = ptr("2026-02-31") }, "health", "documentDate"},
		{"date with time", func(x *Extraction) { x.DocumentDate = ptr("2026-10-14T00:00:00Z") }, "health", "documentDate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x := valid()
			c.change(&x)
			_, _, err := CheckExtraction(x, c.module)

			if !errors.Is(err, ErrInvalidExtraction) {
				t.Fatalf("want ErrInvalidExtraction, got %v", err)
			}
			var xe *ExtractionError
			if !errors.As(err, &xe) || xe.Field != c.field {
				t.Errorf("field = %v, want %q", err, c.field)
			}
		})
	}
}

func TestCheckExtraction_ErrorContainsNoValues(t *testing.T) {
	x := valid()
	x.DocumentDate = ptr("Hämoglobin 14,2 geheim")
	_, _, err := CheckExtraction(x, "health")
	if err == nil || strings.Contains(err.Error(), "geheim") {
		t.Errorf("error message must not contain the value: %v", err)
	}
}

func TestCheckExtraction_IncompleteFactsAreDropped(t *testing.T) {
	x := valid()
	x.Facts = []ExtractedFact{
		{Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}, // 0: kept → i1
		{Name: " ", Value: "geheim-ohne-name"},                 // 1: without name
		{Name: "Leukozyten", Value: "", Unit: ptr("G/l")},      // 2: without value
		{Name: "", Value: "  "},                                // 3: without both
		{Name: "CRP", Value: "<0,5", Unit: ptr("mg/dl")},       // 4: kept → i2
	}

	e, dropped, err := CheckExtraction(x, "health")
	if err != nil {
		t.Fatalf("an incomplete fact must not reject the upload: %v", err)
	}

	// The complete facts remain, numbered without gaps.
	if len(e.Facts) != 2 {
		t.Fatalf("facts = %+v, want 2", e.Facts)
	}
	if e.Facts[0].ID != "f1" || e.Facts[0].Name != "Hämoglobin" ||
		e.Facts[1].ID != "f2" || e.Facts[1].Name != "CRP" {
		t.Errorf("facts = %+v", e.Facts)
	}

	// Only field paths with the index from the model's list are reported.
	want := []string{"facts[1].name", "facts[2].value", "facts[3].name", "facts[3].value"}
	if !slices.Equal(dropped, want) {
		t.Errorf("dropped = %q, want %q", dropped, want)
	}
	for _, d := range dropped {
		if strings.Contains(d, "geheim") || strings.Contains(d, "Leukozyten") {
			t.Errorf("dropped contains a value or name: %q", d)
		}
	}
}

func TestCheckExtraction_AllFactsIncomplete(t *testing.T) {
	x := valid()
	x.Facts = []ExtractedFact{{Name: "", Value: "x"}}

	e, dropped, err := CheckExtraction(x, "health")
	if err != nil {
		t.Fatal(err)
	}
	if e.Facts == nil || len(e.Facts) != 0 {
		t.Errorf("facts = %#v, want empty list", e.Facts)
	}
	if !slices.Equal(dropped, []string{"facts[0].name"}) {
		t.Errorf("dropped = %q", dropped)
	}
}

func TestCheckExtraction_DroppedNeverNilEvenOnError(t *testing.T) {
	x := valid()
	x.Readable = false
	if _, dropped, _ := CheckExtraction(x, "health"); dropped == nil {
		t.Error("dropped must not be nil, even on errors")
	}
}

func TestCheckDate(t *testing.T) {
	for _, ok := range []string{"2026-10-14", "2024-02-29", "1999-12-31", "2026-01-01"} {
		if err := CheckDate(ok); err != nil {
			t.Errorf("CheckDate(%q) = %v", ok, err)
		}
	}
	for _, invalid := range []string{
		"", "2026-1-5", "2026-01-5", "14.10.2026", "2026/10/14",
		"2025-02-29", // not a leap year
		"2026-02-30", "2026-04-31", "2026-13-01", "2026-00-10", "2026-10-00",
		"2026-10-14 ", " 2026-10-14", "2026-10-14T10:00:00Z", "２０２６-10-14",
	} {
		if err := CheckDate(invalid); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("CheckDate(%q) = %v, want ErrInvalidDate", invalid, err)
		}
	}

	// Wrong format and impossible day can be told apart (API.md 3.5).
	for _, format := range []string{"", "14.10.2026", "2026-1-05", "2026/10/14", "2026-10-1a", " 2026-10-14", "２０２６-10-14"} {
		if err := CheckDate(format); errors.Is(err, ErrNoSuchDay) {
			t.Errorf("CheckDate(%q): format error reported as ErrNoSuchDay", format)
		}
	}
	for _, day := range []string{"2025-02-29", "2026-02-30", "2026-04-31", "2026-13-01", "2026-00-10", "2026-10-00"} {
		if err := CheckDate(day); !errors.Is(err, ErrNoSuchDay) {
			t.Errorf("CheckDate(%q) = %v, want ErrNoSuchDay", day, err)
		}
	}
}
