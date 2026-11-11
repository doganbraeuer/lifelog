package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/fake"
)

// fakeStore records all calls in order and can fail for single methods.
type fakeStore struct {
	hashes  map[string]string // sha256 → entry ID (existing entries)
	errs    map[string]error  // method name → error
	log     []string
	entries []core.Entry
	chunks  []core.Chunk
	files   map[string][]byte
}

func newFakeStore() *fakeStore {
	return &fakeStore{hashes: map[string]string{}, errs: map[string]error{}, files: map[string][]byte{}}
}

func (s *fakeStore) EntryByHash(_ context.Context, sha string) (string, bool, error) {
	s.log = append(s.log, "EntryByHash")
	if err := s.errs["EntryByHash"]; err != nil {
		return "", false, err
	}
	id, ok := s.hashes[sha]
	return id, ok, nil
}

func (s *fakeStore) SaveFile(_ context.Context, id, _, _ string, data []byte) error {
	s.log = append(s.log, "SaveFile")
	if err := s.errs["SaveFile"]; err != nil {
		return err
	}
	s.files[id] = data
	return nil
}

func (s *fakeStore) DeleteFile(_ context.Context, id string) error {
	s.log = append(s.log, "DeleteFile")
	delete(s.files, id)
	return nil
}

func (s *fakeStore) SaveChunks(_ context.Context, c []core.Chunk) error {
	s.log = append(s.log, "SaveChunks")
	if err := s.errs["SaveChunks"]; err != nil {
		return err
	}
	s.chunks = append(s.chunks, c...)
	return nil
}

func (s *fakeStore) DeleteChunks(_ context.Context, entryID string) error {
	s.log = append(s.log, "DeleteChunks")
	s.chunks = slices.DeleteFunc(s.chunks, func(c core.Chunk) bool { return c.EntryID == entryID })
	return nil
}

func (s *fakeStore) SaveEntry(_ context.Context, e core.Entry) error {
	s.log = append(s.log, "SaveEntry")
	if err := s.errs["SaveEntry"]; err != nil {
		if errors.Is(err, core.ErrDuplicate) {
			// As with a simultaneous upload: now the other one exists.
			s.hashes[e.File.Sha256] = "aaaaaaaaaaaaaaaaaaaaaaaa"
		}
		return err
	}
	s.entries = append(s.entries, e)
	return nil
}

// isEmpty: after a failed upload nothing may be left.
func (s *fakeStore) isEmpty() bool {
	return len(s.entries) == 0 && len(s.chunks) == 0 && len(s.files) == 0
}

func ptr(s string) *string { return &s }

const pdf = "%PDF-1.7\nsynthetischer Befund"

func extraction() core.Extraction {
	return core.Extraction{
		Readable:     true,
		DocumentDate: ptr("2026-10-14"),
		Issuer:       ptr("Labor Muster GmbH"),
		Type:         "lab_report",
		FullText:     "Labor Muster GmbH\n\nHämoglobin 14,2 g/dl\nCRP <0,5 mg/dl",
		Facts: []core.ExtractedFact{
			{Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")},
			{Name: "CRP", Value: "<0,5", Unit: ptr("mg/dl")},
		},
	}
}

type setup struct {
	store    *fakeStore
	reader   *fake.Reader
	embedder *fake.Embedder
	uploader *Uploader
}

func newSetup() setup {
	s := setup{
		store:    newFakeStore(),
		reader:   &fake.Reader{Extraction: extraction()},
		embedder: &fake.Embedder{},
	}
	n := 0
	s.uploader = NewUploader(s.store, s.reader, s.embedder, func() string {
		n++
		return fmt.Sprintf("%024d", n) // 000…001, 000…002: valid ObjectId form
	})
	s.uploader.now = func() time.Time { return time.Date(2026, 11, 4, 13, 0, 0, 123456789, time.UTC) }
	return s
}

func TestUpload_Success(t *testing.T) {
	s := newSetup()
	e, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf), FileName: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}

	// Order: check first, then file → chunks → entry last.
	want := []string{"EntryByHash", "SaveFile", "SaveChunks", "SaveEntry"}
	if !slices.Equal(s.store.log, want) {
		t.Errorf("order = %v, want %v", s.store.log, want)
	}

	if e.ID == "" || e.File.ID == "" || e.ID == e.File.ID {
		t.Errorf("IDs: entry %q, file %q", e.ID, e.File.ID)
	}
	if e.Module != "health" || e.Type != "lab_report" {
		t.Errorf("module/type = %q/%q", e.Module, e.Type)
	}
	if !e.ImportedAt.Equal(time.Date(2026, 11, 4, 13, 0, 0, 123000000, time.UTC)) {
		t.Errorf("ImportedAt = %v, want truncated to milliseconds", e.ImportedAt)
	}
	f := e.File
	if f.MimeType != core.MimePDF || f.Size != int64(len(pdf)) || f.FileName != "befund.pdf" || len(f.Sha256) != 64 {
		t.Errorf("file = %+v", f)
	}
	if e.Corrections == nil || len(e.Corrections) != 0 {
		t.Errorf("corrections = %#v", e.Corrections)
	}
	if len(e.Facts) != 2 || e.Facts[0].ID != "f1" {
		t.Errorf("facts = %+v", e.Facts)
	}

	// Chunks belong to the entry, carry the module and are numbered.
	chunks := s.store.chunks
	if len(chunks) == 0 {
		t.Fatal("no chunks stored")
	}
	for i, c := range chunks {
		if c.EntryID != e.ID || c.Module != "health" || c.Position != i+1 || len(c.Embedding) != llm.EmbeddingDimensions {
			t.Errorf("chunk %d = %+v", i, c)
		}
	}
	if !bytes.Equal(s.store.files[f.ID], []byte(pdf)) {
		t.Error("file not stored unchanged")
	}
	if len(s.store.entries) != 1 {
		t.Errorf("%d entries stored", len(s.store.entries))
	}
}

func TestUpload_DuplicateBeforeModelCall(t *testing.T) {
	s := newSetup()
	if _, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf)}); err != nil {
		t.Fatal(err)
	}
	existing := s.store.entries[0]
	s.store.hashes[existing.File.Sha256] = existing.ID
	readerBefore, embedderBefore := s.reader.Calls(), s.embedder.Calls()

	_, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf)})

	var de *DuplicateError
	if !errors.As(err, &de) || de.EntryID != existing.ID {
		t.Fatalf("want DuplicateError with %q, got %v", existing.ID, err)
	}
	if !errors.Is(err, core.ErrDuplicate) {
		t.Error("DuplicateError must also be core.ErrDuplicate")
	}
	if s.reader.Calls() != readerBefore || s.embedder.Calls() != embedderBefore {
		t.Error("no model may be called for a duplicate")
	}
	if len(s.store.entries) != 1 {
		t.Error("duplicate was stored")
	}
}

func TestUpload_ChecksBeforeAnyCall(t *testing.T) {
	cases := []struct {
		name   string
		upload Upload
		err    error
	}{
		{"empty file", Upload{Data: nil}, core.ErrFileEmpty},
		{"too large", Upload{Data: append([]byte("%PDF-"), make([]byte, core.MaxFileBytes)...)}, core.ErrFileTooLarge},
		{"wrong type", Upload{Data: []byte("nur Text")}, core.ErrUnsupportedFileType},
		{"unknown module", Upload{Data: []byte(pdf), Module: "sport"}, core.ErrUnknownModule},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSetup()
			_, err := s.uploader.Upload(context.Background(), c.upload)
			if !errors.Is(err, c.err) {
				t.Errorf("want %v, got %v", c.err, err)
			}
			if len(s.store.log) != 0 || s.reader.Calls() != 0 {
				t.Errorf("nothing may be called before the checks pass: %v", s.store.log)
			}
		})
	}
}

func TestUpload_ErrorsBeforeSaving(t *testing.T) {
	unreadable := extraction()
	unreadable.Readable = false
	wrongType := extraction()
	wrongType.Type = "kochrezept"

	cases := []struct {
		name     string
		reader   *fake.Reader
		embedder *fake.Embedder
		err      error
	}{
		{"vision LLM unavailable", &fake.Reader{Err: llm.ErrUnavailable}, &fake.Embedder{}, llm.ErrUnavailable},
		{"invalid response", &fake.Reader{Err: llm.ErrInvalidResponse}, &fake.Embedder{}, llm.ErrInvalidResponse},
		{"unreadable", &fake.Reader{Extraction: unreadable}, &fake.Embedder{}, core.ErrUnreadable},
		{"invalid extraction", &fake.Reader{Extraction: wrongType}, &fake.Embedder{}, core.ErrInvalidExtraction},
		{"embedding unavailable", &fake.Reader{Extraction: extraction()}, &fake.Embedder{Err: llm.ErrUnavailable}, llm.ErrUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSetup()
			s.uploader.reader, s.uploader.embedder = c.reader, c.embedder

			_, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf)})

			if !errors.Is(err, c.err) {
				t.Errorf("want %v, got %v", c.err, err)
			}
			if !slices.Equal(s.store.log, []string{"EntryByHash"}) {
				t.Errorf("nothing may be stored after a model error: %v", s.store.log)
			}
		})
	}
}

func TestUpload_CleanupOnStoreErrors(t *testing.T) {
	dbErr := errors.New("connection lost")
	cases := []struct {
		name  string
		fails string
		log   []string
	}{
		{"file fails", "SaveFile",
			[]string{"EntryByHash", "SaveFile", "DeleteFile"}},
		{"chunks fail", "SaveChunks",
			[]string{"EntryByHash", "SaveFile", "SaveChunks", "DeleteChunks", "DeleteFile"}},
		{"entry fails", "SaveEntry",
			[]string{"EntryByHash", "SaveFile", "SaveChunks", "SaveEntry", "DeleteChunks", "DeleteFile"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSetup()
			s.store.errs[c.fails] = dbErr

			_, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf)})

			if !errors.Is(err, dbErr) {
				t.Errorf("err = %v", err)
			}
			if !slices.Equal(s.store.log, c.log) {
				t.Errorf("log = %v\nwant  %v", s.store.log, c.log)
			}
			if !s.store.isEmpty() {
				t.Error("something is still stored after cleanup")
			}
		})
	}
}

func TestUpload_SimultaneousDuplicate(t *testing.T) {
	// Both uploads pass the check in step 2; only the unique index notices
	// when saving.
	s := newSetup()
	s.store.errs["SaveEntry"] = core.ErrDuplicate

	_, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf)})

	var de *DuplicateError
	if !errors.As(err, &de) || de.EntryID != "aaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("want DuplicateError with the existing ID, got %v", err)
	}
	if !s.store.isEmpty() {
		t.Error("own file and chunks must be removed again")
	}
}

func TestUpload_CleanupAlsoAfterCancel(t *testing.T) {
	// The app cancels while saving. Cleaning up needs a context of its own,
	// otherwise it fails itself because of the cancellation.
	s := newSetup()
	ctx, cancel := context.WithCancel(context.Background())
	s.store.errs["SaveEntry"] = context.Canceled
	cancel()

	// The fakes ignore the context; what matters is that it cleans up and
	// the cancellation stays recognizable.
	_, err := s.uploader.Upload(ctx, Upload{Data: []byte(pdf)})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	if !s.store.isEmpty() {
		t.Error("not cleaned up after cancel")
	}
}

func TestUpload_DroppedFactsOnlyAsFieldInLog(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	x := extraction()
	x.Facts = append(x.Facts, core.ExtractedFact{Name: "", Value: "GEHEIMER-WERT-42"})
	s := newSetup()
	s.uploader.reader = &fake.Reader{Extraction: x}

	entry, err := s.uploader.Upload(context.Background(), Upload{Data: []byte(pdf)})
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Facts) != 2 {
		t.Errorf("facts = %+v, the incomplete one must be missing", entry.Facts)
	}
	if !strings.Contains(log.String(), "infos[2].name") {
		t.Errorf("field missing in the log: %s", log.String())
	}
	if strings.Contains(log.String(), "GEHEIMER-WERT-42") {
		t.Errorf("value in the log: %s", log.String())
	}
}

func TestSafeFileName(t *testing.T) {
	cases := map[string]string{
		"befund.pdf":              "befund.pdf",
		"  befund.pdf  ":          "befund.pdf",
		"../../etc/passwd":        "passwd",
		`C:\Users\x\Befund Ä.pdf`: "Befund Ä.pdf",
		"zeile\nzwei\x00.pdf":     "zeilezwei.pdf",
		"":                        "dokument",
		"   ":                     "dokument",
		strings.Repeat("ä", 250):  strings.Repeat("ä", 200),
	}
	for in, want := range cases {
		if got := safeFileName(in); got != want {
			t.Errorf("safeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}
