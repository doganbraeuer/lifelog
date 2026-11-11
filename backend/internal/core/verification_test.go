package core

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

var checkTime = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

// ocrPage builds an OCR result from lines of text: word i of line l gets
// the box x = 0.1·i, y = 0.05·l, 0.08 × 0.02 and confidence 90. All words
// are on page 1.
func ocrPage(lines ...string) OCRResult {
	r := OCRResult{Engine: "test 1.0 deu", Pages: []OCRPage{{Number: 1, Width: 1000, Height: 1400}}}
	for l, line := range lines {
		for i, w := range strings.Fields(line) {
			r.Words = append(r.Words, OCRWord{
				Text: w, Confidence: 90, Page: 1, Line: l,
				Box: Box{X: 0.1 * float64(i), Y: 0.05 * float64(l), W: 0.08, H: 0.02},
			})
		}
	}
	return r
}

// labPage is the synthetic lab report from API.md as OCR could read it.
var labPage = []string{
	"Labor Muster GmbH",
	"Musterstraße 1, 5020 Salzburg",
	"Laborbefund vom 14.10.2026",
	"Patient: Max Mustermann, geb. 01.01.1990",
	"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)",
	"Leukozyten 6,8 G/l (Ref. 4,0-10,0)",
	"CRP <0,5 mg/dl (Ref. <0,5)",
	"Befundart: Blutbild klein",
}

func labEntry() Entry {
	return Entry{
		DocumentDate: ptr("2026-10-14"),
		Issuer:       ptr("Labor Muster GmbH, Salzburg"),
		Facts: []Fact{
			{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")},
			{ID: "f2", Name: "Leukozyten", Value: "6,8", Unit: ptr("G/l")},
			{ID: "f3", Name: "CRP", Value: "<0,5", Unit: ptr("mg/dl")},
			{ID: "f4", Name: "Befundart", Value: "Blutbild klein"},
		},
	}
}

// field returns the check of one field or fails.
func field(t *testing.T, v Verification, name string) FieldCheck {
	t.Helper()
	i := slices.IndexFunc(v.Fields, func(f FieldCheck) bool { return f.Field == name })
	if i < 0 {
		t.Fatalf("field %s missing in %+v", name, v.Fields)
	}
	return v.Fields[i]
}

func TestVerify_LabReport(t *testing.T) {
	v := Verify(labEntry(), ocrPage(labPage...), checkTime)

	if v.Status != VerificationChecked || v.Source != SourceOCR || v.Engine != "test 1.0 deu" || v.Reason != "" {
		t.Errorf("status/source/engine/reason = %q/%q/%q/%q", v.Status, v.Source, v.Engine, v.Reason)
	}
	if v.CheckedAt == nil || !v.CheckedAt.Equal(checkTime) {
		t.Errorf("checkedAt = %v", v.CheckedAt)
	}
	wantOrder := []string{"documentDate", "issuer", "facts.f1", "facts.f2", "facts.f3", "facts.f4"}
	var order []string
	for _, f := range v.Fields {
		order = append(order, f.Field)
		if f.Status != FieldConfirmed {
			t.Errorf("%s = %s, want confirmed", f.Field, f.Status)
		}
		if f.Confidence == nil || *f.Confidence != 90 {
			t.Errorf("%s confidence = %v, want 90", f.Field, f.Confidence)
		}
	}
	if !slices.Equal(order, wantOrder) {
		t.Errorf("order = %v, want %v", order, wantOrder)
	}
	if v.Confirmed != 6 || v.Total != 6 || *v.MatchPercent() != 100 {
		t.Errorf("confirmed/total/percent = %d/%d/%d", v.Confirmed, v.Total, *v.MatchPercent())
	}

	// The date is the third word of line 2.
	date := field(t, v, "documentDate")
	want := Location{Page: 1, Box: Box{X: 0.2, Y: 0.1, W: 0.08, H: 0.02}}
	if len(date.Locations) != 1 || !near(date.Locations[0], want) {
		t.Errorf("date locations = %+v, want %+v", date.Locations, want)
	}
	// Issuer by parts: the name in line 0, the city in line 1.
	issuer := field(t, v, "issuer")
	if len(issuer.Locations) != 2 || issuer.Locations[0].Box.Y != 0 || issuer.Locations[1].Box.Y != 0.05 {
		t.Errorf("issuer locations = %+v", issuer.Locations)
	}
	// Hämoglobin: "14,2" and "g/dl" are words 1 and 2 of line 4, one box.
	hb := field(t, v, "facts.f1")
	want = Location{Page: 1, Box: Box{X: 0.1, Y: 0.2, W: 0.18, H: 0.02}}
	if len(hb.Locations) != 1 || !near(hb.Locations[0], want) {
		t.Errorf("f1 locations = %+v, want %+v", hb.Locations, want)
	}
}

func near(a, b Location) bool {
	const eps = 1e-9
	return a.Page == b.Page && a.Start == b.Start && a.End == b.End &&
		math.Abs(a.Box.X-b.Box.X) < eps && math.Abs(a.Box.Y-b.Box.Y) < eps &&
		math.Abs(a.Box.W-b.Box.W) < eps && math.Abs(a.Box.H-b.Box.H) < eps
}

// TestVerify_Facts covers the rules for value and unit.
func TestVerify_Facts(t *testing.T) {
	tests := []struct {
		name  string
		fact  Fact
		lines []string
		want  string
	}{
		{"unit separate", Fact{Name: "Glukose", Value: "98", Unit: ptr("mg/dl")}, []string{"Glukose 98 mg/dl"}, FieldConfirmed},
		{"unit written together", Fact{Name: "Hb", Value: "14,2", Unit: ptr("g/dl")}, []string{"Hb 14,2g/dl"}, FieldConfirmed},
		{"unit split by OCR", Fact{Name: "Hb", Value: "14,2", Unit: ptr("g/dl")}, []string{"Hb 14,2 g / dl"}, FieldConfirmed},
		{"unit misread", Fact{Name: "Gesamtcholesterin", Value: "232", Unit: ptr("mg/dl")}, []string{"Gesamtcholesterin 232 mgjdl"}, FieldUnconfirmed},
		{"unit after a flag", Fact{Name: "LDL", Value: "154", Unit: ptr("mg/dl")}, []string{"LDL 154 H * mg/dl"}, FieldConfirmed},
		{"unit too far", Fact{Name: "LDL", Value: "154", Unit: ptr("mg/dl")}, []string{"LDL 154 a b c mg/dl"}, FieldUnconfirmed},
		{"unit in next line", Fact{Name: "LDL", Value: "154", Unit: ptr("mg/dl")}, []string{"LDL 154", "mg/dl"}, FieldUnconfirmed},
		{"decimal point vs comma", Fact{Name: "Hb", Value: "14.2", Unit: ptr("g/dl")}, []string{"Hb 14,2 g/dl"}, FieldConfirmed},
		{"less than", Fact{Name: "CRP", Value: "<0,5", Unit: ptr("mg/dl")}, []string{"CRP < 0,5 mg/dl"}, FieldConfirmed},
		{"no match inside a number", Fact{Name: "X", Value: "4", Unit: ptr("g/dl")}, []string{"X 14 g/dl"}, FieldUnconfirmed},
		{"value missing", Fact{Name: "Triglyceride", Value: "131", Unit: ptr("mg/dl")}, []string{"Triglyceride mg/dl"}, FieldUnconfirmed},
		{"case ignored", Fact{Name: "Leukozyten", Value: "6,8", Unit: ptr("G/l")}, []string{"Leukozyten 6,8 g/l"}, FieldConfirmed},
		{"text value", Fact{Name: "Befundart", Value: "Blutbild klein"}, []string{"Befundart: Blutbild", "klein"}, FieldConfirmed},
		{"text value paraphrased", Fact{Name: "Befundart", Value: "kleines Blutbild"}, []string{"Befundart: Blutbild klein"}, FieldUnconfirmed},
		{"short value in name line", Fact{Name: "Packungen", Value: "2"}, []string{"Packungen: 2"}, FieldConfirmed},
		{"short value elsewhere", Fact{Name: "Packungen", Value: "2"}, []string{"Seite 2"}, FieldUnconfirmed},
		{"short value, name prefix", Fact{Name: "Glukose", Value: "98"}, []string{"Glukose-nüchtern 98"}, FieldConfirmed},
		{"long value without name", Fact{Name: "Dosierung", Value: "1-0-1"}, []string{"Einnahme 1-0-1"}, FieldConfirmed},
		{"empty after normalizing", Fact{Name: "X", Value: "."}, []string{"X ."}, FieldUnconfirmed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fact.ID = "f1"
			v := Verify(Entry{Facts: []Fact{tt.fact}}, ocrPage(tt.lines...), checkTime)
			if got := field(t, v, "facts.f1"); got.Status != tt.want {
				t.Errorf("status = %s, want %s", got.Status, tt.want)
			}
		})
	}
}

func TestVerify_FactPrefersNameLine(t *testing.T) {
	f := Fact{ID: "f1", Name: "Glukose", Value: "98", Unit: ptr("mg/dl")}
	v := Verify(Entry{Facts: []Fact{f}}, ocrPage("Referenz 98 mg/dl", "Glukose 98 mg/dl"), checkTime)
	fc := field(t, v, "facts.f1")
	if len(fc.Locations) != 2 || fc.Locations[0].Box.Y != 0.05 || fc.Locations[1].Box.Y != 0 {
		t.Errorf("locations = %+v, want the Glukose line (y 0.05) first", fc.Locations)
	}
}

func TestVerify_Dates(t *testing.T) {
	tests := []struct {
		date string
		line string
		want string
	}{
		{"2026-01-20", "Befunddatum: 20.01.2026", FieldConfirmed},
		{"2026-01-20", "am 20.1.2026", FieldConfirmed},
		{"2026-01-20", "vom 20.01.26", FieldConfirmed},
		{"2026-01-20", "Datum 2026-01-20", FieldConfirmed},
		{"2026-01-20", "Salzburg, 20. Jänner 2026", FieldConfirmed},
		{"2026-01-20", "20. januar 2026", FieldConfirmed},
		{"2026-10-14", "14. Okt. 2026", FieldConfirmed},
		{"2026-03-05", "05. März 2026", FieldConfirmed},
		{"2026-01-20", "20/01/2026", FieldConfirmed},
		{"2026-01-20", "21.01.2026", FieldUnconfirmed},
		{"2026-01-20", "120.01.2026", FieldUnconfirmed},
	}
	for _, tt := range tests {
		v := Verify(Entry{DocumentDate: ptr(tt.date)}, ocrPage(tt.line), checkTime)
		if got := field(t, v, "documentDate"); got.Status != tt.want {
			t.Errorf("%s in %q: %s, want %s", tt.date, tt.line, got.Status, tt.want)
		}
	}
}

func TestDateForms(t *testing.T) {
	forms := DateForms("2026-01-05")
	for _, want := range []string{"05.01.2026", "5.1.2026", "05.01.26", "2026-01-05", "5. Jänner 2026", "05. Jan 2026"} {
		if !slices.Contains(forms, want) {
			t.Errorf("%q missing in %v", want, forms)
		}
	}
	forms = DateForms("2026-10-14")
	seen := map[string]bool{}
	for _, f := range forms {
		if seen[f] {
			t.Errorf("%q twice", f)
		}
		seen[f] = true
	}
	for _, bad := range []string{"2026-02-30", "14.10.2026", ""} {
		if got := DateForms(bad); len(got) != 0 {
			t.Errorf("DateForms(%q) = %v, want none", bad, got)
		}
	}
}

func TestVerify_Issuer(t *testing.T) {
	tests := []struct {
		name      string
		issuer    string
		lines     []string
		want      string
		locations int
	}{
		{"whole, across lines", "Dr. Erika Beispiel Allgemeinmedizin", []string{"Dr. Erika Beispiel", "Allgemeinmedizin"}, FieldConfirmed, 2},
		{"by parts", "Labor Muster GmbH, Salzburg", labPage, FieldConfirmed, 2},
		{"one part missing", "Labor Muster GmbH, Wien", labPage, FieldUnconfirmed, 0},
		{"no comma, not found", "Labor Beispiel", labPage, FieldUnconfirmed, 0},
		{"whole in one line", "Dr. Erika Beispiel, Allgemeinmedizin", []string{"Dr. Erika Beispiel, Allgemeinmedizin"}, FieldConfirmed, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Verify(Entry{Issuer: ptr(tt.issuer)}, ocrPage(tt.lines...), checkTime)
			fc := field(t, v, "issuer")
			if fc.Status != tt.want || len(fc.Locations) != tt.locations {
				t.Errorf("status %s with %d locations, want %s with %d", fc.Status, len(fc.Locations), tt.want, tt.locations)
			}
		})
	}
}

func TestVerify_MissingFieldsNotCounted(t *testing.T) {
	e := Entry{Facts: []Fact{{ID: "f1", Name: "Hb", Value: "14,2", Unit: ptr("g/dl")}, {ID: "f2", Name: "CRP", Value: "9,9"}}}
	v := Verify(e, ocrPage("Hb 14,2 g/dl", "CRP <0,5"), checkTime)
	if v.Total != 2 || v.Confirmed != 1 || *v.MatchPercent() != 50 {
		t.Errorf("total/confirmed/percent = %d/%d/%v", v.Total, v.Confirmed, v.MatchPercent())
	}
	if fc := field(t, v, "facts.f2"); fc.Confidence != nil || fc.Locations == nil || len(fc.Locations) != 0 {
		t.Errorf("unconfirmed: confidence %v, locations %#v; want nil and []", fc.Confidence, fc.Locations)
	}
}

func TestVerify_NoText(t *testing.T) {
	// "–" becomes "-", which stays (it can be part of a value): that is text.
	v := Verify(labEntry(), ocrPage(". :", "–"), checkTime)
	if v.Status != VerificationChecked {
		t.Fatalf("a lone dash is text: status %s", v.Status)
	}
	v = Verify(labEntry(), ocrPage(". :", ","), checkTime)
	if v.Status != VerificationUnchecked || v.Reason != ReasonNoText || v.Fields == nil || len(v.Fields) != 0 {
		t.Errorf("got %+v, want unchecked/no_text with empty fields", v)
	}
	v = Verify(labEntry(), OCRResult{}, checkTime)
	if v.Reason != ReasonNoText {
		t.Errorf("empty result: reason %q", v.Reason)
	}
}

func TestVerify_LowestConfidence(t *testing.T) {
	r := ocrPage("Hb 14,2 g/dl")
	r.Words[2].Confidence = 61.6
	v := Verify(Entry{Facts: []Fact{{ID: "f1", Name: "Hb", Value: "14,2", Unit: ptr("g/dl")}}}, r, checkTime)
	if c := field(t, v, "facts.f1").Confidence; c == nil || *c != 62 {
		t.Errorf("confidence = %v, want 62", c)
	}
}

func TestVerify_AtMostThreeLocations(t *testing.T) {
	v := Verify(Entry{DocumentDate: ptr("2026-01-20")},
		ocrPage("20.01.2026", "20.01.2026", "20.01.2026", "20.01.2026"), checkTime)
	if fc := field(t, v, "documentDate"); len(fc.Locations) != 3 || fc.Locations[0].Box.Y != 0 {
		t.Errorf("locations = %+v, want the first 3", fc.Locations)
	}
}

func TestVerify_OtherPage(t *testing.T) {
	r := ocrPage("Seite eins", "Hb 14,2 g/dl")
	for i := range r.Words {
		if r.Words[i].Line == 1 {
			r.Words[i].Page = 2
		}
	}
	v := Verify(Entry{Facts: []Fact{{ID: "f1", Name: "Hb", Value: "14,2", Unit: ptr("g/dl")}}}, r, checkTime)
	if fc := field(t, v, "facts.f1"); len(fc.Locations) != 1 || fc.Locations[0].Page != 2 {
		t.Errorf("locations = %+v, want page 2", fc.Locations)
	}
}

func TestVerifyText(t *testing.T) {
	text := "Notiz vom 23.09.2026\nBlutdruck zu Hause: 122/79 mmHg, Übelkeit keine"
	e := Entry{
		DocumentDate: ptr("2026-09-23"),
		FullText:     text,
		Facts:        []Fact{{ID: "f1", Name: "Blutdruck", Value: "122/79", Unit: ptr("mmHg")}},
	}
	v := VerifyText(e, checkTime)
	if v.Status != VerificationChecked || v.Source != SourceText || v.Engine != "" || *v.MatchPercent() != 100 {
		t.Fatalf("got %+v", v)
	}
	fc := field(t, v, "facts.f1")
	if fc.Confidence != nil {
		t.Errorf("confidence = %v, want nil for text", *fc.Confidence)
	}
	if len(fc.Locations) != 1 {
		t.Fatalf("locations = %+v", fc.Locations)
	}
	l := fc.Locations[0]
	runes := []rune(text)
	if l.Page != 0 || string(runes[l.Start:l.End]) != "122/79 mmHg" {
		t.Errorf("range %d–%d = %q", l.Start, l.End, string(runes[l.Start:l.End]))
	}
}

func TestMatchPercent(t *testing.T) {
	tests := []struct {
		v    Verification
		want *int
	}{
		{Verification{Status: VerificationChecked, Confirmed: 5, Total: 6}, intPtr(83)},
		{Verification{Status: VerificationChecked, Confirmed: 2, Total: 3}, intPtr(67)},
		{Verification{Status: VerificationChecked, Confirmed: 1, Total: 8}, intPtr(13)}, // 12.5 rounds up
		{Verification{Status: VerificationChecked, Confirmed: 0, Total: 0}, nil},
		{Unchecked(ReasonOCRFailed), nil},
	}
	for _, tt := range tests {
		got := tt.v.MatchPercent()
		if (got == nil) != (tt.want == nil) || got != nil && *got != *tt.want {
			t.Errorf("%d/%d %s: got %v, want %v", tt.v.Confirmed, tt.v.Total, tt.v.Status, deref(got), deref(tt.want))
		}
	}
}

func intPtr(i int) *int { return &i }

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
