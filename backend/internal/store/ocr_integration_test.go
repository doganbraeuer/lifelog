package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// Only runs with LIFELOG_MONGO_TEST=1 (see integration_test.go).
func TestIntegration_OCRWords(t *testing.T) {
	s, ctx := testStore(t)
	id := NewID()

	if _, found, err := s.GetOCR(ctx, id); err != nil || found {
		t.Fatalf("before saving: found %v, %v", found, err)
	}

	r := core.OCRResult{
		Engine: "pdftotext 26.11.0",
		Pages:  []core.OCRPage{{Number: 1, Source: core.SourcePDFText, Width: 595.2756, Height: 841.8898}},
		Words: []core.OCRWord{
			{Text: "Hämoglobin", Source: core.SourcePDFText, Page: 1, Line: 4, Box: core.Box{X: 0.027, Y: 0.188, W: 0.118, H: 0.019}},
			{Text: "14,2", Source: core.SourcePDFText, Page: 1, Line: 5, Box: core.Box{X: 0.274, Y: 0.188, W: 0.03, H: 0.019}},
		},
	}
	if err := s.SaveOCR(ctx, id, r); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.GetOCR(ctx, id)
	if err != nil || !found || !reflect.DeepEqual(got, r) {
		t.Fatalf("read back: found %v, %v\n%+v\nwant\n%+v", found, err, got, r)
	}

	// Saving again replaces (a repeated upload step must not fail).
	r.Engine = "tesseract 5.5.4 deu"
	r.Words[0].Source, r.Words[0].Confidence = core.SourceOCR, 91.5
	if err := s.SaveOCR(ctx, id, r); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if got, _, _ := s.GetOCR(ctx, id); got.Engine != r.Engine || got.Words[0].Confidence != 91.5 {
		t.Errorf("not replaced: %+v", got)
	}

	if err := s.DeleteOCR(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.GetOCR(ctx, id); found {
		t.Error("still there after DeleteOCR")
	}
	if err := s.DeleteOCR(ctx, id); err != nil {
		t.Errorf("deleting again: %v", err)
	}
	if _, _, err := s.GetOCR(ctx, "nope"); err == nil {
		t.Error("no error for an invalid ID")
	}
}

// The verification is saved with a correction and read back unchanged.
func TestIntegration_SaveCorrectionWithVerification(t *testing.T) {
	s, ctx := testStore(t)
	e := testEntry(NewID(), NewID(), strings.Repeat("ef", 32))
	if err := s.SaveEntry(ctx, e); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 11, 11, 10, 0, 0, 123_000_000, time.UTC)
	e.Facts[0].Value = "14,0"
	e.Corrections = []core.Correction{{Field: "facts.f1.value", Recognized: ptr("14,2"), Corrected: ptr("14,0"), CorrectedAt: at}}
	e.Verification = core.Verification{
		Status: core.VerificationChecked, Source: core.SourcePDFText, Engine: "pdftotext 26.11.0",
		CheckedAt: &at, Confirmed: 0, Corrected: 1, Total: 1,
		Fields: []core.FieldCheck{{Field: "facts.f1", Status: core.FieldCorrectedUnconfirmed, Locations: []core.Location{}}},
	}
	if found, err := s.SaveCorrection(ctx, e); err != nil || !found {
		t.Fatalf("SaveCorrection: %v, %v", found, err)
	}
	read, _, err := s.GetEntry(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Verification, e.Verification) {
		t.Errorf("verification\n%+v\nwant\n%+v", read.Verification, e.Verification)
	}
}
