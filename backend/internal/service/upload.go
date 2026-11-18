// Package service contains the procedures of the backend (upload,
// questions, delete, correct). A service connects domain logic (core),
// models (llm) and storage, but knows neither HTTP nor MongoDB nor Gemini
// directly, only their interfaces. That is why it can be tested completely
// with fakes.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/ocr"
)

// UploadStore is what the upload needs from the database. Implemented by
// store.Store. Every method takes the user: everything an upload stores
// carries the owner, and the duplicate check only looks at the user's own
// entries.
type UploadStore interface {
	EntryByHash(ctx context.Context, userID, sha256 string) (id string, found bool, err error)
	SaveFile(ctx context.Context, userID, id, fileName, mimeType string, data []byte) error
	DeleteFile(ctx context.Context, userID, id string) error
	SaveChunks(ctx context.Context, userID string, chunks []core.Chunk) error
	DeleteChunks(ctx context.Context, userID, entryID string) error
	SaveOCR(ctx context.Context, userID, entryID string, r core.OCRResult) error
	DeleteOCR(ctx context.Context, userID, entryID string) error
	SaveEntry(ctx context.Context, userID string, e core.Entry) error
}

// Upload is an uploaded file.
type Upload struct {
	Data     []byte
	FileName string
	Module   string // empty = default module
}

// TextUpload is an entered text (text import, API.md 3.1).
type TextUpload struct {
	Text   string // exactly as entered, not trimmed
	Module string // empty = default module
}

// input is what file and text upload have in common after the input
// checks. read calls the matching model. text marks a text import: its
// values are checked against the entered text instead of OCR.
type input struct {
	userID   string
	module   string
	data     []byte // is stored and hashed
	fileName string
	mimeType string
	text     bool
	read     func(ctx context.Context) (core.Extraction, error)
}

// DuplicateError: exactly this file already exists. EntryID names the
// existing entry, so the app can jump straight to it.
type DuplicateError struct{ EntryID string }

func (d *DuplicateError) Error() string { return "duplicate of entry " + d.EntryID }

// Is: errors.Is(err, core.ErrDuplicate) is also true for DuplicateError.
func (d *DuplicateError) Is(target error) bool { return target == core.ErrDuplicate }

// Uploader runs the upload (docs/API.md 3.1).
type Uploader struct {
	store      UploadStore
	reader     llm.Reader
	embedder   llm.Embedder
	recognizer ocr.Recognizer // nil: OCR switched off, files stay unchecked
	usage      UsageCounter
	newID      func() string
	now        func() time.Time
	ocrTimeout time.Duration
}

// NewUploader creates the service. newID assigns ObjectIds (store.NewID).
// rec reads the original for the verification; nil switches it off. usage
// counts the daily limit and is required: there is no upload without a
// limit.
func NewUploader(s UploadStore, r llm.Reader, e llm.Embedder, rec ocr.Recognizer, usage UsageCounter, newID func() string) *Uploader {
	if usage == nil {
		panic("service: NewUploader needs a UsageCounter")
	}
	return &Uploader{store: s, reader: r, embedder: e, recognizer: rec, usage: usage, newID: newID, now: time.Now,
		ocrTimeout: OCRTimeout}
}

// OCRTimeout: reading the original (text layer or OCR) may take this long.
// It runs while the vision LLM reads, so usually it costs no extra time. If
// it takes longer, the entry is stored unchecked (ocr_failed); the upload
// never fails because of it.
const OCRTimeout = 30 * time.Second

// cleanupTimeout: time for removing half-stored parts. Runs with a context
// of its own, because the request's context has often already expired or
// been canceled by then.
const cleanupTimeout = 10 * time.Second

// Upload uploads the user's file, has it read and stores everything.
//
//  1. Check module, size and type.
//  2. Compute the hash, check for a duplicate among the user's own entries,
//     BEFORE the model call.
//  3. Count the upload against the user's daily limit; from here on it
//     costs model calls.
//  4. Have the vision LLM read it. At the same time, the second reader
//     reads the original (text layer or OCR, see verify).
//  5. Check the extraction (core.CheckExtraction).
//  6. Split the full text into chunks and embed them, then check the
//     values against what the second reader read (core.Verify).
//  7. Store file → chunks → words → entry, each with the owner. The entry
//     comes last; if a step fails, the parts already stored are removed
//     again. This way no half entry is left behind.
//
// Errors: core.ErrNoUser, core.ErrUnknownModule, core.ErrFileEmpty,
// core.ErrFileTooLarge, core.ErrUnsupportedFileType, *DuplicateError,
// *core.LimitReachedError, core.ErrUnreadable, core.ErrInvalidExtraction,
// llm.ErrInvalidResponse, llm.ErrUnavailable or another (internal) error.
func (u *Uploader) Upload(ctx context.Context, userID string, up Upload) (core.Entry, error) {
	if userID == "" {
		return core.Entry{}, core.ErrNoUser
	}
	// 1. Check, without network and without database.
	module, err := core.CheckModule(up.Module)
	if err != nil {
		return core.Entry{}, err
	}
	mimeType, err := core.CheckFile(up.Data)
	if err != nil {
		return core.Entry{}, err
	}

	return u.process(ctx, input{
		userID:   userID,
		module:   module,
		data:     up.Data,
		fileName: safeFileName(up.FileName),
		mimeType: mimeType,
		read: func(ctx context.Context) (core.Extraction, error) {
			return u.reader.ReadDocument(ctx, up.Data, mimeType, core.AllowedTypes(module))
		},
	})
}

// UploadText imports an entered text. The procedure is as in Upload, with
// three differences (API.md 3.1):
//
//   - Instead of size and type, core.CheckText checks the text (empty,
//     UTF-8, 100,000 characters).
//   - The UTF-8 bytes of the text are stored and hashed, as text.txt with
//     text/plain; charset=utf-8.
//   - The language model (Reader.ReadText) only returns date, issuer, type
//     and facts. The full text is the entered text, unchanged, and a text
//     always counts as readable.
//
// Errors: core.ErrNoUser, core.ErrUnknownModule, core.ErrFileEmpty,
// core.ErrTextNotUTF8, core.ErrTextTooLong, *DuplicateError,
// *core.LimitReachedError, core.ErrInvalidExtraction,
// llm.ErrInvalidResponse, llm.ErrUnavailable or an internal error.
func (u *Uploader) UploadText(ctx context.Context, userID string, up TextUpload) (core.Entry, error) {
	if userID == "" {
		return core.Entry{}, core.ErrNoUser
	}
	module, err := core.CheckModule(up.Module)
	if err != nil {
		return core.Entry{}, err
	}
	if err := core.CheckText(up.Text); err != nil {
		return core.Entry{}, err
	}
	return u.process(ctx, input{
		userID:   userID,
		module:   module,
		data:     []byte(up.Text), // Go strings are UTF-8: exactly these bytes
		fileName: core.TextFileName,
		mimeType: core.MimeText,
		text:     true,
		read: func(ctx context.Context) (core.Extraction, error) {
			x, err := u.reader.ReadText(ctx, up.Text, core.AllowedTypes(module))
			if err != nil {
				return core.Extraction{}, err
			}
			x.Readable = true
			x.FullText = up.Text
			return x, nil
		},
	})
}

// process runs steps 2–7, the same for file and text.
func (u *Uploader) process(ctx context.Context, in input) (core.Entry, error) {
	module := in.module

	// 2. Check for a duplicate before a model call costs anything. Only the
	// user's own entries count: another user's file is never a duplicate.
	sum := sha256.Sum256(in.data)
	hash := hex.EncodeToString(sum[:])
	if id, found, err := u.store.EntryByHash(ctx, in.userID, hash); err != nil {
		return core.Entry{}, err
	} else if found {
		return core.Entry{}, &DuplicateError{EntryID: id}
	}

	// 3. Daily limit: the request is accepted, from here on it costs model
	// calls. Counted whether the upload succeeds or not.
	if err := u.usage.Use(ctx, in.userID, core.LimitUploads); err != nil {
		return core.Entry{}, err
	}

	// 4. Read. The second reader starts first and runs at the same time as
	// the vision LLM, so the upload does not take longer. If the upload
	// fails before its result is needed, the deferred cancel stops it.
	ocrCtx, cancelOCR := context.WithTimeout(ctx, u.ocrTimeout)
	defer cancelOCR()
	ocrDone := u.startOCR(ocrCtx, in)

	extraction, err := in.read(ctx)
	if err != nil {
		return core.Entry{}, err
	}

	// 5. Check. Only field names go into the log, never values (health data).
	entry, dropped, err := core.CheckExtraction(extraction, module)
	if len(dropped) > 0 {
		slog.Warn("incomplete facts dropped", "fields", dropped)
	}
	if err != nil {
		var xe *core.ExtractionError
		if errors.As(err, &xe) {
			slog.Warn("invalid extraction", "field", xe.Field, "reason", xe.Reason)
		}
		return core.Entry{}, err
	}

	// 6. Chunks and embeddings.
	texts := core.SplitIntoChunks(entry.FullText, core.ChunkMaxChars)
	vectors, err := u.embedder.EmbedChunks(ctx, texts)
	if err != nil {
		return core.Entry{}, err
	}
	if len(vectors) != len(texts) {
		return core.Entry{}, fmt.Errorf("%d embeddings for %d chunks: %w",
			len(vectors), len(texts), llm.ErrInvalidResponse)
	}

	// Assign IDs and metadata. Milliseconds, because MongoDB stores times
	// with that precision: the response and a later query then show the
	// same.
	entry.ID = u.newID()
	entry.UserID = in.userID
	entry.ImportedAt = u.now().UTC().Truncate(time.Millisecond)

	// Check the values. Only now wait for the second reader: it has had
	// all the time of the model calls.
	var words *core.OCRResult
	entry.Verification, words = u.verify(entry, in, ocrDone)
	// Only IDs and counts, never values or the file name.
	slog.Info("verification", "entry", entry.ID, "user", in.userID,
		"status", entry.Verification.Status, "reason", entry.Verification.Reason,
		"source", entry.Verification.Source, "confirmed", entry.Verification.Confirmed,
		"total", entry.Verification.Total)
	entry.File = core.File{
		ID:       u.newID(),
		FileName: in.fileName,
		MimeType: in.mimeType,
		Size:     int64(len(in.data)),
		Sha256:   hash,
	}
	chunks := make([]core.Chunk, len(texts))
	for i, t := range texts {
		chunks[i] = core.Chunk{
			EntryID: entry.ID, Module: module, Position: i + 1, Text: t, Embedding: vectors[i],
		}
	}

	// 7. Store, entry last.
	if err := u.save(ctx, entry, chunks, words, in.data); err != nil {
		return core.Entry{}, err
	}
	return entry, nil
}

// ocrOutcome is the result of the second reader.
type ocrOutcome struct {
	result core.OCRResult
	err    error
}

// startOCR starts reading the original in its own goroutine and returns
// the channel its result arrives on. nil for a text import or if OCR is
// switched off. The channel is buffered, so the goroutine never blocks,
// even if nobody waits for it anymore (failed upload).
func (u *Uploader) startOCR(ctx context.Context, in input) <-chan ocrOutcome {
	if in.text || u.recognizer == nil {
		return nil
	}
	done := make(chan ocrOutcome, 1)
	go func() {
		r, err := u.recognizer.Recognize(ctx, in.data, in.mimeType)
		done <- ocrOutcome{r, err}
	}()
	return done
}

// verify checks the values (docs/API.md, verification). For a file it
// waits for the second reader; the time limit of its context ends the
// wait. If reading failed, the entry is unchecked with the matching reason;
// the upload goes on. words is what to store for later checks (PATCH), nil
// if there is nothing to store.
func (u *Uploader) verify(e core.Entry, in input, ocrDone <-chan ocrOutcome) (core.Verification, *core.OCRResult) {
	now := e.ImportedAt
	if in.text {
		return core.VerifyText(e, now), nil
	}
	if ocrDone == nil {
		return core.Unchecked(core.ReasonOCRUnavailable), nil
	}
	out := <-ocrDone
	return verificationFrom(e, out.result, out.err, now)
}

// verificationFrom turns what the second reader returned into the
// verification, for the upload and for filling in old entries (Verifier).
// If reading failed, the entry is unchecked with the matching reason. words
// is what to store for later checks; nil if nothing was read.
func verificationFrom(e core.Entry, r core.OCRResult, err error, now time.Time) (v core.Verification, words *core.OCRResult) {
	if err != nil {
		reason := ocr.Reason(err)
		// The errors of Poppler and Tesseract name the program and its
		// message, never document content.
		slog.Warn("reading the original failed, entry stays unchecked", "reason", reason, "error", err)
		return core.Unchecked(reason), nil
	}
	v = core.Verify(e, r, now)
	if v.Status != core.VerificationChecked {
		return v, nil // no text: nothing worth storing
	}
	return v, &r
}

func (u *Uploader) save(ctx context.Context, e core.Entry, chunks []core.Chunk, words *core.OCRResult, data []byte) error {
	if err := u.store.SaveFile(ctx, e.UserID, e.File.ID, e.File.FileName, e.File.MimeType, data); err != nil {
		u.cleanup(ctx, e, false)
		return err
	}
	if err := u.store.SaveChunks(ctx, e.UserID, chunks); err != nil {
		u.cleanup(ctx, e, true)
		return err
	}
	if words != nil {
		if err := u.store.SaveOCR(ctx, e.UserID, e.ID, *words); err != nil {
			u.cleanup(ctx, e, true)
			return err
		}
	}
	err := u.store.SaveEntry(ctx, e.UserID, e)
	if err == nil {
		return nil
	}
	u.cleanup(ctx, e, true)

	if errors.Is(err, core.ErrDuplicate) {
		// The same user uploaded the same file at the same time and was
		// faster. Name the existing entry as in step 2.
		if id, found, ferr := u.store.EntryByHash(ctx, e.UserID, e.File.Sha256); ferr == nil && found {
			return &DuplicateError{EntryID: id}
		}
	}
	return err
}

// cleanup removes the file and (if already stored) the chunks and words of
// a failed upload. The file is also deleted after an error in SaveFile: GridFS may
// already have written some pieces. If cleaning up fails, it is logged; the
// entry itself does not exist, so the app sees nothing half-done.
func (u *Uploader) cleanup(ctx context.Context, e core.Entry, withChunks bool) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()

	if withChunks {
		if err := u.store.DeleteChunks(ctx, e.UserID, e.ID); err != nil {
			slog.Error("cleanup: chunks not deleted", "entry", e.ID, "error", err)
		}
		// Also if they were never stored: deleting nothing is no error.
		if err := u.store.DeleteOCR(ctx, e.UserID, e.ID); err != nil {
			slog.Error("cleanup: ocr words not deleted", "entry", e.ID, "error", err)
		}
	}
	if err := u.store.DeleteFile(ctx, e.UserID, e.File.ID); err != nil {
		slog.Error("cleanup: file not deleted", "file", e.File.ID, "error", err)
	}
}

// safeFileName turns the name at upload into a safe display name: without
// path and control characters, at most 200 characters, never empty. The
// fallback "dokument" is shown in the app, so it stays German.
func safeFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	if name == "" {
		return "dokument"
	}
	return name
}
