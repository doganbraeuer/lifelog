package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

const throwawayFile = "bbbbbbbbbbbbbbbbbbbbbbbb"

// fakeDeleteStore holds an entry with chunks and file, records every call
// with its argument and can fail in one step.
type fakeDeleteStore struct {
	entry  *core.Entry // nil: the entry does not exist (anymore)
	chunks bool
	file   bool

	failsAt   string // name of the method that returns an error
	err       error  // this error, otherwise errDB
	afterRead func() // runs after GetEntry, e.g. to cancel ctx
	ctxErrs   []error
	log       []string
}

var errDB = errors.New("database gone")

func (s *fakeDeleteStore) step(ctx context.Context, name, arg string) error {
	s.log = append(s.log, name+"("+arg+")")
	s.ctxErrs = append(s.ctxErrs, ctx.Err())
	if s.failsAt == name {
		if s.err != nil {
			return s.err
		}
		return errDB
	}
	return nil
}

func (s *fakeDeleteStore) GetEntry(ctx context.Context, id string) (core.Entry, bool, error) {
	if err := s.step(ctx, "GetEntry", id); err != nil {
		return core.Entry{}, false, err
	}
	if s.entry == nil || s.entry.ID != id {
		return core.Entry{}, false, nil
	}
	e := *s.entry
	if s.afterRead != nil {
		s.afterRead()
	}
	return e, true, nil
}

func (s *fakeDeleteStore) DeleteChunks(ctx context.Context, id string) error {
	if err := s.step(ctx, "DeleteChunks", id); err != nil {
		return err
	}
	s.chunks = false
	return nil
}

// DeleteFile behaves like store: if the file is missing, no error.
func (s *fakeDeleteStore) DeleteFile(ctx context.Context, id string) error {
	if err := s.step(ctx, "DeleteFile", id); err != nil {
		return err
	}
	s.file = false
	return nil
}

func (s *fakeDeleteStore) DeleteEntry(ctx context.Context, id string) (bool, error) {
	if err := s.step(ctx, "DeleteEntry", id); err != nil {
		return false, err
	}
	if s.entry == nil || s.entry.ID != id {
		return false, nil
	}
	s.entry = nil
	return true, nil
}

func deleteData() *fakeDeleteStore {
	return &fakeDeleteStore{
		entry: &core.Entry{
			ID: idLab, Module: "gesundheit", Type: "laborbefund",
			Issuer: ptr("Labor Geheim GmbH"), FullText: "Hämoglobin 14,2 g/dl",
			Facts: []core.Fact{{ID: "i1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}},
			File:  core.File{ID: throwawayFile, FileName: "befund_geheim.pdf", MimeType: "application/pdf"},
		},
		chunks: true,
		file:   true,
	}
}

var allSteps = []string{
	"GetEntry(" + idLab + ")",
	"DeleteChunks(" + idLab + ")",
	"DeleteFile(" + throwawayFile + ")",
	"DeleteEntry(" + idLab + ")",
}

func TestDelete_SuccessInThisOrder(t *testing.T) {
	s := deleteData()
	if err := NewDeleter(s).Delete(context.Background(), idLab); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.log, allSteps) {
		t.Errorf("calls\n%v\nwant\n%v", s.log, allSteps)
	}
	if s.entry != nil || s.chunks || s.file {
		t.Errorf("not everything deleted: entry %v, chunks %v, file %v", s.entry != nil, s.chunks, s.file)
	}
}

// If a step fails, the later ones do not run, and the entry stays visible
// (API.md 3.7). A second call cleans up, a third gives 404.
func TestDelete_FailsInOneStep(t *testing.T) {
	for i, step := range []string{"GetEntry", "DeleteChunks", "DeleteFile", "DeleteEntry"} {
		t.Run(step, func(t *testing.T) {
			s := deleteData()
			s.failsAt = step
			d := NewDeleter(s)

			err := d.Delete(context.Background(), idLab)
			if !errors.Is(err, errDB) {
				t.Fatalf("error %v, want %v", err, errDB)
			}
			if want := allSteps[:i+1]; !reflect.DeepEqual(s.log, want) {
				t.Errorf("calls %v, want %v", s.log, want)
			}
			if s.entry == nil {
				t.Error("entry deleted although an earlier step failed")
			}

			// Repeating cleans up the rest.
			s.failsAt, s.log = "", nil
			if err := d.Delete(context.Background(), idLab); err != nil {
				t.Fatalf("second call: %v", err)
			}
			if s.entry != nil || s.chunks || s.file {
				t.Error("second call did not delete everything")
			}
			if err := d.Delete(context.Background(), idLab); !errors.Is(err, core.ErrEntryNotFound) {
				t.Errorf("third call: %v, want ErrEntryNotFound", err)
			}
		})
	}
}

func TestDelete_TwiceGivesNotFound(t *testing.T) {
	s := deleteData()
	d := NewDeleter(s)
	if err := d.Delete(context.Background(), idLab); err != nil {
		t.Fatal(err)
	}
	s.log = nil
	if err := d.Delete(context.Background(), idLab); !errors.Is(err, core.ErrEntryNotFound) {
		t.Fatalf("second call: %v", err)
	}
	// Only read, nothing deleted.
	if want := allSteps[:1]; !reflect.DeepEqual(s.log, want) {
		t.Errorf("calls %v, want %v", s.log, want)
	}
}

func TestDelete_InvalidIDWithoutStore(t *testing.T) {
	s := deleteData()
	for _, id := range []string{"", "abc", strings.Repeat("z", 24)} {
		if err := NewDeleter(s).Delete(context.Background(), id); !errors.Is(err, core.ErrInvalidID) {
			t.Errorf("%q: %v, want ErrInvalidID", id, err)
		}
	}
	if len(s.log) != 0 {
		t.Errorf("store called: %v", s.log)
	}
}

// File already gone (e.g. after an interrupted delete): no error.
func TestDelete_FileAlreadyMissing(t *testing.T) {
	s := deleteData()
	s.file, s.chunks = false, false
	if err := NewDeleter(s).Delete(context.Background(), idLab); err != nil {
		t.Fatal(err)
	}
	if s.entry != nil {
		t.Error("entry not deleted")
	}
}

// A simultaneous call deletes the entry between reading and deleting.
func TestDelete_DeletedSimultaneously(t *testing.T) {
	s := deleteData()
	s.afterRead = func() { s.entry = nil }
	if err := NewDeleter(s).Delete(context.Background(), idLab); !errors.Is(err, core.ErrEntryNotFound) {
		t.Errorf("%v, want ErrEntryNotFound", err)
	}
	if !reflect.DeepEqual(s.log, allSteps) {
		t.Errorf("calls %v, want all four", s.log)
	}
}

// If the app cancels after reading, the delete steps still run to
// completion with a valid context.
func TestDelete_CancelAfterStartRunsToCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := deleteData()
	s.afterRead = cancel

	if err := NewDeleter(s).Delete(ctx, idLab); err != nil {
		t.Fatal(err)
	}
	if s.entry != nil {
		t.Fatal("entry not deleted")
	}
	for i, err := range s.ctxErrs[1:] {
		if err != nil {
			t.Errorf("%s with a canceled context: %v", s.log[i+1], err)
		}
	}
}

// Canceled before the start: reading fails, nothing is deleted.
func TestDelete_CancelBeforeStart(t *testing.T) {
	s := deleteData()
	s.failsAt, s.err = "GetEntry", context.Canceled

	err := NewDeleter(s).Delete(context.Background(), idLab)
	if !errors.Is(err, context.Canceled) || s.entry == nil {
		t.Errorf("error %v, entry still there: %v", err, s.entry != nil)
	}
}

func TestDelete_LogOnlyID(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	if err := NewDeleter(deleteData()).Delete(context.Background(), idLab); err != nil {
		t.Fatal(err)
	}
	text := log.String()
	if !strings.Contains(text, "entry deleted") || !strings.Contains(text, idLab) {
		t.Errorf("log without message or ID: %s", text)
	}
	for _, content := range []string{"Geheim", "geheim", "Hämoglobin", "14,2", "laborbefund", throwawayFile} {
		if strings.Contains(text, content) {
			t.Errorf("log contains %q: %s", content, text)
		}
	}
}
