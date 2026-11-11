package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/ocr"
	ocrfake "lifelog/backend/internal/ocr/fake"
)

// fieldStatus returns the status of one field of the verification.
func fieldStatus(t *testing.T, v core.Verification, field string) string {
	t.Helper()
	i := slices.IndexFunc(v.Fields, func(f core.FieldCheck) bool { return f.Field == field })
	if i < 0 {
		t.Fatalf("field %s missing in %+v", field, v.Fields)
	}
	return v.Fields[i].Status
}

func TestUpload_Verification(t *testing.T) {
	s := newSetup()
	e, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	v := e.Verification
	if v.Status != core.VerificationChecked || v.Source != core.SourceOCR || v.Engine != "tesseract 5.5.4 deu" {
		t.Fatalf("verification %+v", v)
	}
	if v.CheckedAt == nil || !v.CheckedAt.Equal(e.ImportedAt) {
		t.Errorf("checkedAt %v, want the import time %v", v.CheckedAt, e.ImportedAt)
	}
	want := map[string]string{
		"documentDate": core.FieldConfirmed,
		"issuer":       core.FieldConfirmed,
		"facts.f1":     core.FieldConfirmed,
		"facts.f2":     core.FieldUnconfirmed, // the original says mg/l, the model read mg/dl
	}
	for field, status := range want {
		if got := fieldStatus(t, v, field); got != status {
			t.Errorf("%s: %s, want %s", field, got, status)
		}
	}
	if v.Confirmed != 3 || v.Total != 4 {
		t.Errorf("confirmed %d of %d", v.Confirmed, v.Total)
	}
	// The words are stored for later checks, the entry with its check.
	if got, ok := s.store.ocr[e.ID]; !ok || len(got.Words) != len(s.ocr.Result.Words) {
		t.Errorf("stored words: %v, %d", ok, len(got.Words))
	}
	if len(s.store.entries) != 1 || s.store.entries[0].Verification.Total != 4 {
		t.Error("stored entry has no verification")
	}
	if s.ocr.Calls() != 1 {
		t.Errorf("OCR calls %d", s.ocr.Calls())
	}
}

// Reading the original fails: the upload still succeeds, unchecked with the
// matching reason, and no words are stored.
func TestUpload_VerificationFailsButUploadSucceeds(t *testing.T) {
	tests := []struct {
		name   string
		ocr    *ocrfake.Recognizer
		reason string
	}{
		{"error", &ocrfake.Recognizer{Err: errors.New("tesseract: exit status 1")}, core.ReasonOCRFailed},
		{"unsupported", &ocrfake.Recognizer{Err: fmt.Errorf("image/heic: %w", ocr.ErrUnsupported)}, core.ReasonUnsupportedFileType},
		{"too many pages", &ocrfake.Recognizer{Err: ocr.ErrTooManyPages}, core.ReasonTooManyPages},
		{"program missing", &ocrfake.Recognizer{Err: ocr.ErrUnavailable}, core.ReasonOCRUnavailable},
		{"no text", &ocrfake.Recognizer{Result: core.OCRResult{Engine: "tesseract"}}, core.ReasonNoText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSetup()
			s.uploader.recognizer = tt.ocr
			e, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
			if err != nil {
				t.Fatalf("upload failed: %v", err)
			}
			v := e.Verification
			if v.Status != core.VerificationUnchecked || v.Reason != tt.reason || v.Fields == nil || len(v.Fields) != 0 {
				t.Errorf("verification %+v, want unchecked/%s", v, tt.reason)
			}
			if slices.Contains(s.store.log, "SaveOCR") {
				t.Error("words stored although nothing was read")
			}
			if len(s.store.entries) != 1 {
				t.Error("entry not stored")
			}
		})
	}
}

func TestUpload_WithoutOCR(t *testing.T) {
	s := newSetup()
	s.uploader.recognizer = nil
	e, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Verification.Reason != core.ReasonOCRUnavailable {
		t.Errorf("verification %+v", e.Verification)
	}
}

func TestUpload_OCRTimeLimit(t *testing.T) {
	s := newSetup()
	s.ocr.Block = true // never finishes on its own
	s.uploader.ocrTimeout = 50 * time.Millisecond
	start := time.Now()
	e, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Verification.Reason != core.ReasonOCRFailed {
		t.Errorf("verification %+v, want ocr_failed", e.Verification)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("upload took %v, the time limit was 50 ms", d)
	}
}

// signalReader is a vision LLM that only answers once OCR has started.
// If OCR only started after reading (one after the other), it would wait
// until its limit and fail the test.
type signalReader struct {
	llm.Reader
	ocrStarted <-chan struct{}
}

func (r signalReader) ReadDocument(ctx context.Context, data []byte, mime string, types []string) (core.Extraction, error) {
	select {
	case <-r.ocrStarted:
		return r.Reader.ReadDocument(ctx, data, mime, types)
	case <-time.After(2 * time.Second):
		return core.Extraction{}, errors.New("OCR did not run while the model was reading")
	}
}

// signalRecognizer reports its start and then waits until the model has
// answered, so both are really busy at the same time.
type signalRecognizer struct {
	started     chan struct{}
	modelDone   <-chan struct{}
	result      core.OCRResult
	sawModelEnd bool
}

func (r *signalRecognizer) Recognize(ctx context.Context, _ []byte, _ string) (core.OCRResult, error) {
	close(r.started)
	select {
	case <-r.modelDone:
		r.sawModelEnd = true
		return r.result, nil
	case <-ctx.Done():
		return core.OCRResult{}, ctx.Err()
	}
}

type doneReader struct {
	llm.Reader
	done chan struct{}
}

func (r doneReader) ReadDocument(ctx context.Context, data []byte, mime string, types []string) (core.Extraction, error) {
	x, err := r.Reader.ReadDocument(ctx, data, mime, types)
	close(r.done)
	return x, err
}

func TestUpload_OCRRunsWhileModelReads(t *testing.T) {
	s := newSetup()
	modelDone := make(chan struct{})
	rec := &signalRecognizer{started: make(chan struct{}), modelDone: modelDone, result: s.ocr.Result}
	s.uploader.recognizer = rec
	s.uploader.reader = doneReader{Reader: signalReader{Reader: s.reader, ocrStarted: rec.started}, done: modelDone}

	e, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if !rec.sawModelEnd {
		t.Error("OCR did not wait for the model: they did not overlap")
	}
	if e.Verification.Status != core.VerificationChecked {
		t.Errorf("verification %+v", e.Verification)
	}
}

// If the model fails, the upload fails at once; OCR is canceled and not
// waited for.
func TestUpload_ModelErrorCancelsOCR(t *testing.T) {
	s := newSetup()
	s.ocr.Block = true
	s.uploader.ocrTimeout = 10 * time.Second
	s.reader.Err = llm.ErrUnavailable
	start := time.Now()
	_, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("error %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("upload waited %v for OCR", d)
	}
	if !s.store.isEmpty() {
		t.Error("something was stored")
	}
}

func TestTextImport_VerifiedAgainstText(t *testing.T) {
	s := textSetup()
	e, err := s.uploader.UploadText(context.Background(), TextUpload{Text: enteredText})
	if err != nil {
		t.Fatal(err)
	}
	// Date, issuer and diagnosis are all in the entered text.
	v := e.Verification
	if v.Status != core.VerificationChecked || v.Source != core.SourceText || v.Engine != "" || v.Confirmed != 3 || v.Total != 3 {
		t.Errorf("verification %+v", v)
	}
	if s.ocr.Calls() != 0 || slices.Contains(s.store.log, "SaveOCR") {
		t.Errorf("OCR calls %d, log %v: a text needs no OCR", s.ocr.Calls(), s.store.log)
	}
}

// --- Correcting ---

// checkedEntry is an entry as the upload stores it: verified against words.
func checkedEntry(words core.OCRResult) core.Entry {
	e := core.Entry{
		ID: idLab, Module: "health", Type: "lab_report",
		DocumentDate: ptr("2026-10-14"), Issuer: ptr("Labor Muster GmbH"),
		Facts:       []core.Fact{{ID: "f2", Name: "Leukozyten", Value: "6,3", Unit: ptr("G/l")}},
		Corrections: []core.Correction{},
	}
	e.Verification = core.Verify(e, words, time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC))
	return e
}

func TestCorrect_Reverifies(t *testing.T) {
	words := ocrResult("Labor Muster GmbH\nvom 14.10.2026\nLeukozyten 6,8 G/l")
	tests := []struct {
		value  string
		status string
	}{
		{"6,8", core.FieldCorrectedConfirmed},   // what the original says
		{"6,9", core.FieldCorrectedUnconfirmed}, // not in the original
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			e := checkedEntry(words)
			if got := fieldStatus(t, e.Verification, "facts.f2"); got != core.FieldUnconfirmed {
				t.Fatalf("before: %s", got)
			}
			s := &fakeCorrectionStore{entry: &e, ocr: &words}
			got, err := newFixedCorrector(s).Correct(context.Background(), idLab,
				core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr(tt.value)}}})
			if err != nil {
				t.Fatal(err)
			}
			v := got.Verification
			if st := fieldStatus(t, v, "facts.f2"); st != tt.status {
				t.Errorf("after: %s, want %s", st, tt.status)
			}
			if v.Corrected != 1 || !v.CheckedAt.Equal(time.Date(2026, 11, 4, 19, 5, 31, 123_000_000, time.UTC)) {
				t.Errorf("corrected %d, checkedAt %v", v.Corrected, v.CheckedAt)
			}
			if len(s.saved) != 1 || fieldStatus(t, s.saved[0].Verification, "facts.f2") != tt.status {
				t.Error("verification not saved")
			}
		})
	}
}

// Back to the recognized value: the correction disappears, and so does the
// corrected status.
func TestCorrect_ResetRemovesCorrectedStatus(t *testing.T) {
	words := ocrResult("Labor Muster GmbH\nLeukozyten 6,8 G/l")
	e := checkedEntry(words)
	s := &fakeCorrectionStore{entry: &e, ocr: &words}
	c := newFixedCorrector(s)
	first, err := c.Correct(context.Background(), idLab, core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,8")}}})
	if err != nil {
		t.Fatal(err)
	}
	s.entry = &first
	back, err := c.Correct(context.Background(), idLab, core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,3")}}})
	if err != nil {
		t.Fatal(err)
	}
	if st := fieldStatus(t, back.Verification, "facts.f2"); st != core.FieldUnconfirmed || back.Verification.Corrected != 0 {
		t.Errorf("after reset: %s, corrected %d", st, back.Verification.Corrected)
	}
}

func TestCorrect_UncheckedStaysUnchecked(t *testing.T) {
	s := correctionData()
	s.entry.Verification = core.Unchecked(core.ReasonOCRFailed)
	got, err := newFixedCorrector(s).Correct(context.Background(), idLab,
		core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,8")}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Verification.Reason != core.ReasonOCRFailed || s.ocrReads != 0 {
		t.Errorf("verification %+v, words read %d times", got.Verification, s.ocrReads)
	}
}

func TestCorrect_TextImportUsesFullText(t *testing.T) {
	e := core.Entry{
		ID: idLab, Module: "health", Type: "other", FullText: "Blutdruck 122/79 mmHg",
		Facts: []core.Fact{{ID: "f1", Name: "Blutdruck", Value: "122/80", Unit: ptr("mmHg")}}, Corrections: []core.Correction{},
	}
	e.Verification = core.VerifyText(e, time.Now())
	s := &fakeCorrectionStore{entry: &e}
	got, err := newFixedCorrector(s).Correct(context.Background(), idLab,
		core.Change{Facts: []core.FactChange{{ID: "f1", Value: ptr("122/79")}}})
	if err != nil {
		t.Fatal(err)
	}
	if st := fieldStatus(t, got.Verification, "facts.f1"); st != core.FieldCorrectedConfirmed || s.ocrReads != 0 {
		t.Errorf("status %s, words read %d times", st, s.ocrReads)
	}
}

func TestCorrect_WordsMissingKeepsVerification(t *testing.T) {
	words := ocrResult("Leukozyten 6,8 G/l")
	e := checkedEntry(words)
	old := e.Verification
	s := &fakeCorrectionStore{entry: &e} // no words stored
	got, err := newFixedCorrector(s).Correct(context.Background(), idLab,
		core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,8")}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Facts[0].Value != "6,8" || len(s.saved) != 1 {
		t.Error("correction not saved")
	}
	if !got.Verification.CheckedAt.Equal(*old.CheckedAt) {
		t.Errorf("verification changed although the words are missing")
	}
}

func TestCorrect_WordsReadErrorSavesNothing(t *testing.T) {
	words := ocrResult("Leukozyten 6,8 G/l")
	e := checkedEntry(words)
	s := &fakeCorrectionStore{entry: &e, ocrErr: errDB}
	_, err := newFixedCorrector(s).Correct(context.Background(), idLab,
		core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,8")}}})
	if !errors.Is(err, errDB) || len(s.saved) != 0 {
		t.Errorf("error %v, saved %d", err, len(s.saved))
	}
}

func TestCorrect_NoChangeReadsNoWords(t *testing.T) {
	words := ocrResult("Leukozyten 6,3 G/l")
	e := checkedEntry(words)
	s := &fakeCorrectionStore{entry: &e, ocr: &words}
	if _, err := newFixedCorrector(s).Correct(context.Background(), idLab,
		core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,3")}}}); err != nil {
		t.Fatal(err)
	}
	if s.ocrReads != 0 || len(s.saved) != 0 {
		t.Errorf("words read %d times, saved %d times", s.ocrReads, len(s.saved))
	}
}
