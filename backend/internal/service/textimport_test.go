package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/fake"
)

// A synthetic text with whitespace at the edges, \r\n and umlauts: it must
// be stored byte for byte as it arrives.
const enteredText = "  Hausarztpraxis Beispiel\r\nÜberweisung vom 21.10.2026\n\nDiagnose: Rückenschmerzen \n"

func textExtraction() core.Extraction {
	// As the fake returns it. The model does not set FullText and Readable for
	// a text; the service must set them itself and overwrite deviations.
	return core.Extraction{
		Readable:     false,
		FullText:     "vom Modell erfunden",
		DocumentDate: ptr("2026-10-21"),
		Issuer:       ptr("Hausarztpraxis Beispiel"),
		Type:         "doctor_letter",
		Facts:        []core.ExtractedFact{{Name: "Diagnose", Value: "Rückenschmerzen"}},
	}
}

// deref shows an optional date for messages; "<null>" for nil.
func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

func textSetup() setup {
	s := newSetup()
	s.reader.Extraction = textExtraction()
	return s
}

func TestTextImport_Success(t *testing.T) {
	s := textSetup()
	e, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText})
	if err != nil {
		t.Fatal(err)
	}

	// Only ReadText, never ReadDocument, and with the text unchanged.
	n, sent := s.reader.TextCalls()
	if n != 1 || s.reader.Calls() != 0 || sent != enteredText {
		t.Errorf("ReadText %d×, ReadDocument %d×, text %q", n, s.reader.Calls(), sent)
	}

	// The full text is the entered text, not the model's.
	if e.FullText != enteredText {
		t.Errorf("full text = %q", e.FullText)
	}
	sum := sha256.Sum256([]byte(enteredText))
	want := core.File{
		ID: e.File.ID, FileName: "text.txt", MimeType: "text/plain; charset=utf-8",
		Size: int64(len(enteredText)), Sha256: hex.EncodeToString(sum[:]),
	}
	if e.File != want {
		t.Errorf("file = %+v\nwant %+v", e.File, want)
	}
	// Bytes, not characters: Ü and ü are two bytes each.
	if e.File.Size != int64(len([]rune(enteredText))+2) {
		t.Errorf("size %d is not in bytes", e.File.Size)
	}
	if got := string(s.store.files[e.File.ID]); got != enteredText {
		t.Errorf("stored file = %q", got)
	}

	if e.Module != "health" || e.Type != "doctor_letter" || len(e.Facts) != 1 || e.Facts[0].ID != "f1" {
		t.Errorf("entry %+v", e)
	}
	if len(s.store.entries) != 1 || s.store.entries[0].FullText != enteredText {
		t.Error("entry not stored with the entered text")
	}
	// Chunks from the entered text.
	if len(s.store.chunks) == 0 || !strings.Contains(s.store.chunks[0].Text, "Hausarztpraxis") {
		t.Errorf("chunks %+v", s.store.chunks)
	}
}

// The model gets the time of the import, so "gestern" can be resolved; a
// date the model found counts as from the document (API.md 3.1).
func TestTextImport_ModelGetsTodayAndDateFromText(t *testing.T) {
	s := textSetup()
	e, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.reader.TextNow(), s.uploader.now(); !got.Equal(want) {
		t.Errorf("ReadText got %v, want the import time %v", got, want)
	}
	if deref(e.DocumentDate) != "2026-10-21" || e.DocumentDateSource != core.DateSourceDocument {
		t.Errorf("date %v, source %q, want 2026-10-21 document", deref(e.DocumentDate), e.DocumentDateSource)
	}
	if !slices.ContainsFunc(e.Verification.Fields, func(f core.FieldCheck) bool { return f.Field == "documentDate" }) {
		t.Error("a date from the text must be checked")
	}
}

// Without a date in the text (e.g. a voice note) the entry gets the day of
// its import in Europe/Vienna, marked "import", and that date is not checked.
func TestTextImport_WithoutDateGetsTheImportDay(t *testing.T) {
	for _, c := range []struct {
		name string
		now  time.Time
		want string
	}{
		{"afternoon", time.Date(2026, 11, 23, 9, 46, 0, 0, time.UTC), "2026-11-23"},
		// 23:30 UTC is 00:30 on the next day in Vienna: that day counts.
		{"after midnight in Vienna", time.Date(2026, 11, 23, 23, 30, 0, 0, time.UTC), "2026-11-24"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := textSetup()
			s.reader.Extraction.DocumentDate = nil
			s.uploader.now = func() time.Time { return c.now }

			e, err := s.uploader.UploadText(context.Background(), testUser,
				TextUpload{Text: "Blutdruck 130 zu 85, Puls 64"})
			if err != nil {
				t.Fatal(err)
			}
			if deref(e.DocumentDate) != c.want || e.DocumentDateSource != core.DateSourceImport {
				t.Errorf("date %v, source %q, want %s import", deref(e.DocumentDate), e.DocumentDateSource, c.want)
			}
			stored := s.store.entries[0]
			if deref(stored.DocumentDate) != c.want || stored.DocumentDateSource != core.DateSourceImport {
				t.Errorf("stored: %v %q", deref(stored.DocumentDate), stored.DocumentDateSource)
			}
			// Not checked, so never "not found" in red.
			for _, f := range e.Verification.Fields {
				if f.Field == "documentDate" {
					t.Errorf("import day checked: %+v", f)
				}
			}
		})
	}
}

// A file without a date keeps null: only entered texts are written today.
func TestUpload_FileWithoutDateStaysNull(t *testing.T) {
	s := newSetup()
	s.reader.Extraction.DocumentDate = nil
	e, err := s.uploader.Upload(context.Background(), testUser, Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if e.DocumentDate != nil || e.DocumentDateSource != core.DateSourceDocument {
		t.Errorf("date %v, source %q, want null document", deref(e.DocumentDate), e.DocumentDateSource)
	}
}

func TestTextImport_DuplicateBeforeModelCall(t *testing.T) {
	s := textSetup()
	sum := sha256.Sum256([]byte(enteredText))
	s.store.addExisting(testUser, hex.EncodeToString(sum[:]), "bbbbbbbbbbbbbbbbbbbbbbbb")

	_, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText})

	var dup *DuplicateError
	if !errors.As(err, &dup) || dup.EntryID != "bbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("err = %v", err)
	}
	if n, _ := s.reader.TextCalls(); n != 0 || s.embedder.Calls() != 0 {
		t.Errorf("model called despite duplicate: read %d, embed %d", n, s.embedder.Calls())
	}
}

// One extra space already gives a different hash (API.md 4).
func TestTextImport_SpaceIsNoDuplicate(t *testing.T) {
	s := textSetup()
	sum := sha256.Sum256([]byte(enteredText))
	s.store.addExisting(testUser, hex.EncodeToString(sum[:]), "bbbbbbbbbbbbbbbbbbbbbbbb")
	if _, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText + " "}); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestTextImport_ChecksBeforeAnyCall(t *testing.T) {
	for _, c := range []struct {
		name   string
		upload TextUpload
		err    error
	}{
		{"empty", TextUpload{Text: ""}, core.ErrFileEmpty},
		{"only whitespace", TextUpload{Text: " \r\n\t"}, core.ErrFileEmpty},
		{"invalid UTF-8", TextUpload{Text: "Befund \xff"}, core.ErrTextNotUTF8},
		{"too long", TextUpload{Text: strings.Repeat("ä", core.MaxTextChars+1)}, core.ErrTextTooLong},
		{"unknown module", TextUpload{Text: enteredText, Module: "sport"}, core.ErrUnknownModule},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := textSetup()
			_, err := s.uploader.UploadText(context.Background(), testUser, c.upload)
			if !errors.Is(err, c.err) {
				t.Errorf("err = %v, want %v", err, c.err)
			}
			if n, _ := s.reader.TextCalls(); n != 0 || len(s.store.log) != 0 {
				t.Errorf("calls despite error: read %d, store %v", n, s.store.log)
			}
		})
	}
}

// Exactly 100,000 characters with umlauts (200,000 bytes) are allowed.
func TestTextImport_LimitInCharacters(t *testing.T) {
	s := textSetup()
	text := strings.Repeat("ü", core.MaxTextChars)
	e, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if e.File.Size != 2*core.MaxTextChars {
		t.Errorf("size %d", e.File.Size)
	}
}

func TestTextImport_ModelErrorStoresNothing(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
	}{
		{"unavailable", llm.ErrUnavailable},
		{"invalid response", llm.ErrInvalidResponse},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := textSetup()
			s.uploader.reader = &fake.Reader{Err: c.err}
			_, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText})
			if !errors.Is(err, c.err) {
				t.Errorf("err = %v", err)
			}
			if !slices.Equal(s.store.log, []string{"EntryByHash"}) || !s.store.isEmpty() {
				t.Errorf("stored: %v", s.store.log)
			}
		})
	}
}

// A type from the model that is not allowed is invalid (502), not UNREADABLE.
func TestTextImport_TypeNotAllowed(t *testing.T) {
	s := textSetup()
	x := textExtraction()
	x.Type = "kochrezept"
	s.reader.Extraction = x
	_, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText})
	if !errors.Is(err, core.ErrInvalidExtraction) || errors.Is(err, core.ErrUnreadable) {
		t.Errorf("err = %v", err)
	}
}

func TestTextImport_CleanupOnStoreError(t *testing.T) {
	s := textSetup()
	dbErr := errors.New("connection lost")
	s.store.errs["SaveEntry"] = dbErr
	if _, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText}); !errors.Is(err, dbErr) {
		t.Errorf("err = %v", err)
	}
	if !s.store.isEmpty() {
		t.Error("not cleaned up")
	}
}

func TestTextImport_TextNotInLog(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	s := textSetup()
	x := textExtraction()
	x.Facts = append(x.Facts, core.ExtractedFact{Name: "Dosis", Value: ""}) // dropped and logged
	s.reader.Extraction = x
	if _, err := s.uploader.UploadText(context.Background(), testUser, TextUpload{Text: enteredText}); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"Hausarztpraxis", "Rückenschmerzen", "Überweisung", "Dosis"} {
		if strings.Contains(log.String(), content) {
			t.Errorf("log contains %q: %s", content, log.String())
		}
	}
}
