package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

func ptr(s string) *string { return &s }

func TestDocumentKey(t *testing.T) {
	tests := map[string]string{
		"01_lab_report_2026-01-20.pdf":         "01_lab_report_2026-01-20",
		"01_lab_report_2026-01-20_photo.png":   "01_lab_report_2026-01-20",
		"04_prescription_2026-05-18_app.png":   "04_prescription_2026-05-18",
		"07_note_physiotherapy_2026-08-12.txt": "07_note_physiotherapy_2026-08-12",
		"L3_eye_test_2026-09-15_photo.png":     "L3_eye_test_2026-09-15",
	}
	for name, want := range tests {
		if got := documentKey(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

// The real demo folder: PDF and photo of the same document land in one
// group, with the PDF first.
func TestDemoFiles(t *testing.T) {
	files, err := demoFiles("../../../demo")
	if err != nil {
		t.Fatal(err)
	}
	variants := map[string][]string{}
	for _, f := range files {
		variants[f.Doc] = append(variants[f.Doc], f.variantName())
	}
	want := map[string][]string{
		"01_lab_report_2026-01-20":         {"pdf", "photo"},
		"04_prescription_2026-05-18":       {"pdf", "photo", "photo", "app"}, // photos/ and live/
		"07_note_physiotherapy_2026-08-12": {"text"},
		"L1_lab_report_2026-10-28":         {"pdf"},
		"L3_eye_test_2026-09-15":           {"photo", "app"},
	}
	for doc, w := range want {
		if got := variants[doc]; !slices.Equal(got, w) {
			t.Errorf("%s: %v, want %v", doc, got, w)
		}
	}
	if len(variants) != 11 {
		t.Errorf("%d documents, want 11 (01–08, L1–L3)", len(variants))
	}
}

func labEntry() core.Entry {
	return core.Entry{
		DocumentDate: ptr("2026-01-20"),
		Issuer:       ptr("Labor Muster GmbH"),
		Facts: []core.Fact{
			{ID: "f1", Name: "Gesamtcholesterin", Value: "232", Unit: ptr("mg/dl")},
			{ID: "f2", Name: "Triglyceride", Value: "165", Unit: ptr("mg/dl")},
		},
	}
}

// words: one line per string, source OCR, confidence 80.
func words(lines ...string) core.OCRResult {
	var r core.OCRResult
	for l, line := range lines {
		for i, w := range strings.Fields(line) {
			r.Words = append(r.Words, core.OCRWord{Text: w, Source: core.SourceOCR, Confidence: 80, Page: 1, Line: l,
				Box: core.Box{X: 0.1 * float64(i), Y: 0.05 * float64(l), W: 0.08, H: 0.02}})
		}
	}
	return r
}

func TestMeasureRow(t *testing.T) {
	e := labEntry()
	r := words("Labor Muster GmbH", "Befunddatum: 20.01.2026", "Gesamtcholesterin 232 mgjdl", "Triglyceride mg/dl")
	now := time.Now()
	got := measureRow("01", "photo", kindPhoto, e, core.Verify(e, r, now), core.Verify(withoutUnits(e), r, now), r, time.Second)

	if got.Date != "✓" || got.Issuer != "✓" || got.Facts != 0 || got.FactsTotal != 2 || got.Values != 1 {
		t.Errorf("row %+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 80 || *got.Match != 50 {
		t.Errorf("confidence %v, match %v", got.Confidence, got.Match)
	}
	want := []string{
		`f1 Gesamtcholesterin: 232 mg/dl — read: "Gesamtcholesterin 232 mgjdl"`,
		`f2 Triglyceride: 165 mg/dl — read: "Triglyceride mg/dl"`,
	}
	if !slices.Equal(got.Missing, want) {
		t.Errorf("missing\n%q\nwant\n%q", got.Missing, want)
	}
	if e.Facts[0].Unit == nil {
		t.Error("withoutUnits changed the entry")
	}
}

func TestSummarize(t *testing.T) {
	p := func(i int) *int { return &i }
	rows := []row{
		{Kind: kindPhoto, Match: p(29), Facts: 0, FactsTotal: 5, Values: 4},
		{Kind: kindPhoto, Match: p(100), Facts: 3, FactsTotal: 3, Values: 3},
		{Kind: kindPhoto, Reason: core.ReasonOCRFailed},
		{Kind: kindPDFText, Match: p(100), Facts: 5, FactsTotal: 5, Values: 5},
	}
	s := summarize(rows)
	if len(s) != 2 || s[0].Kind != kindPDFText || s[1].Kind != kindPhoto {
		t.Fatalf("kinds %+v", s)
	}
	ph := s[1]
	if ph.Documents != 3 || ph.Unchecked != 1 || *ph.MatchMean != 64.5 || *ph.MatchMin != 29 ||
		ph.Facts != 3 || ph.FactsTotal != 8 || ph.Values != 7 {
		t.Errorf("photo %+v", ph)
	}
}

func TestMarkdown(t *testing.T) {
	p := func(i int) *int { return &i }
	rep := report{Date: "2026-11-11", Programs: "pdftotext 26.11.0, tesseract 5.5.4 deu", Database: "lifelog",
		Rows: []row{
			{Doc: "01", Variant: "photo", Kind: kindPhoto, Source: "ocr", Date: "✓", Issuer: "✓", Facts: 0, FactsTotal: 5,
				Values: 4, Match: p(29), Duration: 1500 * time.Millisecond, Missing: []string{"f1 x"}},
			{Doc: "02", Variant: "photo", Kind: kindPhoto, Reason: "ocr_failed"},
		},
		Missing: []string{"L3_eye_test_2026-09-15"}, AllValues: true}
	md := rep.Markdown()
	for _, want := range []string{
		"| 01 | photo | ocr | ✓ | ✓ | 0/5 | 4/5 | 29 % | – | 1.5 s |",
		"| 02 | photo | unchecked: ocr_failed |",
		"| Photo | 2 | 29 % | 29 % | 0/5 (0 %) | 4/5 (80 %) | 1 |",
		"Not measured (no entry in the database; run demo/upload.sh): L3_eye_test_2026-09-15",
		"- **01, photo**\n  - f1 x",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
}

// A short value with a unit: confirmed strictly, but without its unit the
// rule for short values applies. "Value only" must still count it.
func TestMeasureRow_ValueOnlyNeverBelowStrict(t *testing.T) {
	e := core.Entry{Facts: []core.Fact{{ID: "f1", Name: "Packungsgröße", Value: "20", Unit: ptr("Stück")}}}
	r := words("Packungsgröße:", "20 Stück")
	now := time.Now()
	got := measureRow("04", "pdf", kindPDFText, e, core.Verify(e, r, now), core.Verify(withoutUnits(e), r, now), r, 0)
	if got.Facts != 1 || got.Values != 1 {
		t.Errorf("facts %d, values %d; want 1 and 1", got.Facts, got.Values)
	}
}

func TestRowWith_MostWords(t *testing.T) {
	rows := []string{"Sehschärfe ohne Brille 10 0,8", "Sehschärfe mit Brille rl 12"}
	if got := rowWith(rows, "Sehschärfe mit Brille Rechts"); got != rows[1] {
		t.Errorf("got %q", got)
	}
	if got := rowWith(rows, "Blutdruck"); got != "" {
		t.Errorf("got %q, want none", got)
	}
}

func TestExtraFiles(t *testing.T) {
	files, err := extraFiles([]string{"../../../demo/photos/01_lab_report_2026-01-20_photo.png", " "})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Doc != "01_lab_report_2026-01-20" || !files[0].Extra || !strings.HasPrefix(files[0].Path, "/") {
		t.Errorf("files %+v", files)
	}
	if got := documentKey("01_lab_report_2026-01-20_phone.jpg"); got != "01_lab_report_2026-01-20" {
		t.Errorf("phone key %q", got)
	}
	if v := (demoFile{Name: "01_lab_report_2026-01-20_phone.jpg"}).variantName(); v != "phone" {
		t.Errorf("variant %q", v)
	}
	if _, err := extraFiles([]string{"/does/not/exist_phone.jpg"}); err == nil {
		t.Error("no error for a missing file")
	}
}
