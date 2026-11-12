package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	ocrfake "lifelog/backend/internal/ocr/fake"
)

type fakeVerifyStore struct {
	files   map[string][]byte
	ocr     map[string]core.OCRResult
	saved   map[string]core.Verification
	missing bool // SaveVerification finds no entry
	ocrErr  error
	log     []string
}

func newFakeVerifyStore() *fakeVerifyStore {
	return &fakeVerifyStore{files: map[string][]byte{}, ocr: map[string]core.OCRResult{}, saved: map[string]core.Verification{}}
}

func (s *fakeVerifyStore) OpenFile(_ context.Context, id string) (io.ReadCloser, int64, error) {
	s.log = append(s.log, "OpenFile")
	data, ok := s.files[id]
	if !ok {
		return nil, 0, core.ErrOriginalMissing
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

func (s *fakeVerifyStore) SaveOCR(_ context.Context, id string, r core.OCRResult) error {
	s.log = append(s.log, "SaveOCR")
	if s.ocrErr != nil {
		return s.ocrErr
	}
	s.ocr[id] = r
	return nil
}

func (s *fakeVerifyStore) SaveVerification(_ context.Context, id string, v core.Verification) (bool, error) {
	s.log = append(s.log, "SaveVerification")
	if s.missing {
		return false, nil
	}
	s.saved[id] = v
	return true, nil
}

// oldEntry is an entry from before 2026-11-11: never checked.
func oldEntry(mime string) core.Entry {
	return core.Entry{
		ID: idLab, Module: "health", Type: "lab_report", DocumentDate: ptr("2026-10-14"),
		Issuer: ptr("Labor Muster GmbH"), FullText: "Labor Muster GmbH\nBefund vom 14.10.2026\nHämoglobin 14,2 g/dl",
		Facts:        []core.Fact{{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}},
		File:         core.File{ID: throwawayFile, MimeType: mime},
		Corrections:  []core.Correction{},
		Verification: core.Unchecked(core.ReasonNotRun),
	}
}

func newTestVerifier(s *fakeVerifyStore, rec *ocrfake.Recognizer) *Verifier {
	// A nil *ocrfake.Recognizer must become a nil interface, otherwise the
	// Verifier would think OCR is there.
	v := NewVerifier(s, nil)
	if rec != nil {
		v = NewVerifier(s, rec)
	}
	v.now = func() time.Time { return time.Date(2026, 11, 11, 17, 0, 0, 123_456_789, time.UTC) }
	return v
}

func TestVerifier_File(t *testing.T) {
	s := newFakeVerifyStore()
	s.files[throwawayFile] = []byte(pdf)
	rec := &ocrfake.Recognizer{Result: ocrResult("Labor Muster GmbH\nBefund vom 14.10.2026\nHämoglobin 14,2 g/dl")}
	v := newTestVerifier(s, rec)

	e := oldEntry(core.MimePDF)
	ver, words, err := v.Check(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	if ver.Status != core.VerificationChecked || ver.Confirmed != 3 || words == nil {
		t.Fatalf("verification %+v, words %v", ver, words != nil)
	}
	if !ver.CheckedAt.Equal(time.Date(2026, 11, 11, 17, 0, 0, 123_000_000, time.UTC)) {
		t.Errorf("checkedAt %v, want truncated to milliseconds", ver.CheckedAt)
	}
	if slices.Contains(s.log, "SaveOCR") || slices.Contains(s.log, "SaveVerification") {
		t.Errorf("Check wrote something: %v", s.log)
	}

	if err := v.Save(context.Background(), e.ID, ver, words); err != nil {
		t.Fatal(err)
	}
	if want := []string{"OpenFile", "SaveOCR", "SaveVerification"}; !slices.Equal(s.log, want) {
		t.Errorf("calls %v, want %v", s.log, want)
	}
	if s.saved[e.ID].Confirmed != 3 || len(s.ocr[e.ID].Words) == 0 {
		t.Error("not saved")
	}
}

func TestVerifier_TextImport(t *testing.T) {
	s := newFakeVerifyStore()
	e := oldEntry(core.MimeText)
	s.files[throwawayFile] = []byte(e.FullText)
	rec := &ocrfake.Recognizer{}
	ver, words, err := newTestVerifier(s, rec).Check(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	if ver.Source != core.SourceText || ver.Confirmed != 3 || words != nil || rec.Calls() != 0 {
		t.Errorf("verification %+v, words %v, OCR calls %d", ver, words != nil, rec.Calls())
	}
}

func TestVerifier_OCRFails(t *testing.T) {
	s := newFakeVerifyStore()
	s.files[throwawayFile] = []byte(pdf)
	ver, words, err := newTestVerifier(s, &ocrfake.Recognizer{Err: errors.New("pdftotext: exit status 1")}).
		Check(context.Background(), oldEntry(core.MimePDF))
	if err != nil {
		t.Fatal(err)
	}
	if ver.Reason != core.ReasonOCRFailed || words != nil {
		t.Errorf("verification %+v", ver)
	}

	ver, _, _ = newTestVerifier(s, nil).Check(context.Background(), oldEntry(core.MimePDF))
	if ver.Reason != core.ReasonOCRUnavailable {
		t.Errorf("without OCR: %+v", ver)
	}
}

func TestVerifier_Errors(t *testing.T) {
	s := newFakeVerifyStore() // no file
	if _, _, err := newTestVerifier(s, &ocrfake.Recognizer{}).Check(context.Background(), oldEntry(core.MimePDF)); !errors.Is(err, core.ErrOriginalMissing) {
		t.Errorf("missing original: %v", err)
	}

	s = newFakeVerifyStore()
	s.missing = true
	v := newTestVerifier(s, &ocrfake.Recognizer{})
	if err := v.Save(context.Background(), idLab, core.Unchecked(core.ReasonOCRFailed), nil); !errors.Is(err, core.ErrEntryNotFound) {
		t.Errorf("entry gone: %v", err)
	}

	s = newFakeVerifyStore()
	s.ocrErr = errDB
	v = newTestVerifier(s, &ocrfake.Recognizer{})
	words := ocrResult("x")
	if err := v.Save(context.Background(), idLab, core.Verification{Status: core.VerificationChecked}, &words); !errors.Is(err, errDB) {
		t.Errorf("words fail: %v", err)
	}
	if slices.Contains(s.log, "SaveVerification") {
		t.Error("verification saved although its words are missing")
	}
}

func TestNeedsCheck(t *testing.T) {
	tests := []struct {
		v    core.Verification
		want bool
	}{
		{core.Unchecked(core.ReasonNotRun), true},
		{core.Unchecked(core.ReasonOCRFailed), false},
		{core.Verification{Status: core.VerificationChecked}, false},
	}
	for _, tt := range tests {
		if got := NeedsCheck(core.Entry{Verification: tt.v}); got != tt.want {
			t.Errorf("%+v: %v, want %v", tt.v, got, tt.want)
		}
	}
}
