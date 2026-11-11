package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/ocr"
)

// VerifyStore is what filling in the verification needs from the database.
// Implemented by store.Store.
type VerifyStore interface {
	OpenFile(ctx context.Context, fileID string) (content io.ReadCloser, length int64, err error)
	SaveOCR(ctx context.Context, entryID string, r core.OCRResult) error
	SaveVerification(ctx context.Context, entryID string, v core.Verification) (found bool, err error)
}

// Verifier checks entries that are already stored, e.g. those from before
// 2026-11-11 (cmd/ocrcheck). It uses the same rules as the upload, but reads
// the original from the database. No model is called.
type Verifier struct {
	store      VerifyStore
	recognizer ocr.Recognizer // nil: files become unchecked/ocr_unavailable
	now        func() time.Time
	timeout    time.Duration
}

func NewVerifier(s VerifyStore, rec ocr.Recognizer) *Verifier {
	return &Verifier{store: s, recognizer: rec, now: time.Now, timeout: OCRTimeout}
}

// NeedsCheck: the entry was never checked (stored before verification
// existed, or a seed entry).
func NeedsCheck(e core.Entry) bool {
	return e.Verification.Status != core.VerificationChecked && e.Verification.Reason == core.ReasonNotRun
}

// Check reads the entry's original and checks its current values,
// including corrections. It writes nothing; Save does. The error is only
// for the database (e.g. the original file is missing); a failed OCR gives
// an unchecked verification, as on upload.
func (v *Verifier) Check(ctx context.Context, e core.Entry) (core.Verification, *core.OCRResult, error) {
	now := v.now().UTC().Truncate(time.Millisecond)
	data, err := v.readFile(ctx, e.File.ID)
	if err != nil {
		return core.Verification{}, nil, err
	}
	if e.File.MimeType == core.MimeText {
		// The stored text is the full text, unchanged (API.md 3.1).
		return core.VerifyText(e, now), nil, nil
	}
	if v.recognizer == nil {
		return core.Unchecked(core.ReasonOCRUnavailable), nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	r, err := v.recognizer.Recognize(ctx, data, e.File.MimeType)
	ver, words := verificationFrom(e, r, err, now)
	return ver, words, nil
}

// Save stores the words (if any) and then the verification, in this order:
// an entry that names a check always has its words.
func (v *Verifier) Save(ctx context.Context, entryID string, ver core.Verification, words *core.OCRResult) error {
	if words != nil {
		if err := v.store.SaveOCR(ctx, entryID, *words); err != nil {
			return err
		}
	}
	found, err := v.store.SaveVerification(ctx, entryID, ver)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrEntryNotFound
	}
	return nil
}

func (v *Verifier) readFile(ctx context.Context, fileID string) ([]byte, error) {
	r, _, err := v.store.OpenFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, core.MaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read original: %w", err)
	}
	if len(data) > core.MaxFileBytes {
		return nil, errors.New("original larger than 10 MB")
	}
	return data, nil
}
