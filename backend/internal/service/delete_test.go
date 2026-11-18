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
	entry  *core.Entry // nil: the entry does not exist (anymore); owned by testUser
	chunks bool
	ocr    bool
	file   bool

	failsAt   string // name of the method that returns an error
	err       error  // this error, otherwise errDB
	afterRead func() // runs after GetEntry, e.g. to cancel ctx
	ctxErrs   []error
	log       []string
	users     []string // user ID of every call
}

var errDB = errors.New("database gone")

func (s *fakeDeleteStore) step(ctx context.Context, name, userID, arg string) error {
	s.log = append(s.log, name+"("+arg+")")
	s.users = append(s.users, userID)
	s.ctxErrs = append(s.ctxErrs, ctx.Err())
	if s.failsAt == name {
		if s.err != nil {
			return s.err
		}
		return errDB
	}
	return nil
}

// owns: the data of the fake belongs to testUser; like the store, every
// method acts only for the owner.
func (s *fakeDeleteStore) owns(userID string) bool { return userID == testUser }

func (s *fakeDeleteStore) GetEntry(ctx context.Context, userID, id string) (core.Entry, bool, error) {
	if err := s.step(ctx, "GetEntry", userID, id); err != nil {
		return core.Entry{}, false, err
	}
	if s.entry == nil || s.entry.ID != id || !s.owns(userID) {
		return core.Entry{}, false, nil
	}
	e := *s.entry
	if s.afterRead != nil {
		s.afterRead()
	}
	return e, true, nil
}

func (s *fakeDeleteStore) DeleteChunks(ctx context.Context, userID, id string) error {
	if err := s.step(ctx, "DeleteChunks", userID, id); err != nil {
		return err
	}
	if s.owns(userID) {
		s.chunks = false
	}
	return nil
}

func (s *fakeDeleteStore) DeleteOCR(ctx context.Context, userID, id string) error {
	if err := s.step(ctx, "DeleteOCR", userID, id); err != nil {
		return err
	}
	if s.owns(userID) {
		s.ocr = false
	}
	return nil
}

// DeleteFile behaves like store: if the file is missing, no error.
func (s *fakeDeleteStore) DeleteFile(ctx context.Context, userID, id string) error {
	if err := s.step(ctx, "DeleteFile", userID, id); err != nil {
		return err
	}
	if s.owns(userID) {
		s.file = false
	}
	return nil
}

func (s *fakeDeleteStore) DeleteEntry(ctx context.Context, userID, id string) (bool, error) {
	if err := s.step(ctx, "DeleteEntry", userID, id); err != nil {
		return false, err
	}
	if s.entry == nil || s.entry.ID != id || !s.owns(userID) {
		return false, nil
	}
	s.entry = nil
	return true, nil
}

func deleteData() *fakeDeleteStore {
	return &fakeDeleteStore{
		entry: &core.Entry{
			ID: idLab, Module: "health", Type: "lab_report",
			Issuer: ptr("Labor Geheim GmbH"), FullText: "Hämoglobin 14,2 g/dl",
			Facts: []core.Fact{{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}},
			File:  core.File{ID: throwawayFile, FileName: "befund_geheim.pdf", MimeType: "application/pdf"},
		},
		chunks: true,
		ocr:    true,
		file:   true,
	}
}

var allSteps = []string{
	"GetEntry(" + idLab + ")",
	"DeleteChunks(" + idLab + ")",
	"DeleteOCR(" + idLab + ")",
	"DeleteFile(" + throwawayFile + ")",
	"DeleteEntry(" + idLab + ")",
}

func TestDelete_SuccessInThisOrder(t *testing.T) {
	s := deleteData()
	if err := NewDeleter(s).Delete(context.Background(), testUser, idLab); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.log, allSteps) {
		t.Errorf("calls\n%v\nwant\n%v", s.log, allSteps)
	}
	if s.entry != nil || s.chunks || s.ocr || s.file {
		t.Errorf("not everything deleted: entry %v, chunks %v, file %v", s.entry != nil, s.chunks, s.file)
	}
}

// If a step fails, the later ones do not run, and the entry stays visible
// (API.md 3.7). A second call cleans up, a third gives 404.
func TestDelete_FailsInOneStep(t *testing.T) {
	for i, step := range []string{"GetEntry", "DeleteChunks", "DeleteOCR", "DeleteFile", "DeleteEntry"} {
		t.Run(step, func(t *testing.T) {
			s := deleteData()
			s.failsAt = step
			d := NewDeleter(s)

			err := d.Delete(context.Background(), testUser, idLab)
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
			if err := d.Delete(context.Background(), testUser, idLab); err != nil {
				t.Fatalf("second call: %v", err)
			}
			if s.entry != nil || s.chunks || s.ocr || s.file {
				t.Error("second call did not delete everything")
			}
			if err := d.Delete(context.Background(), testUser, idLab); !errors.Is(err, core.ErrEntryNotFound) {
				t.Errorf("third call: %v, want ErrEntryNotFound", err)
			}
		})
	}
}

func TestDelete_TwiceGivesNotFound(t *testing.T) {
	s := deleteData()
	d := NewDeleter(s)
	if err := d.Delete(context.Background(), testUser, idLab); err != nil {
		t.Fatal(err)
	}
	s.log = nil
	if err := d.Delete(context.Background(), testUser, idLab); !errors.Is(err, core.ErrEntryNotFound) {
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
		if err := NewDeleter(s).Delete(context.Background(), testUser, id); !errors.Is(err, core.ErrInvalidID) {
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
	if err := NewDeleter(s).Delete(context.Background(), testUser, idLab); err != nil {
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
	if err := NewDeleter(s).Delete(context.Background(), testUser, idLab); !errors.Is(err, core.ErrEntryNotFound) {
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

	if err := NewDeleter(s).Delete(ctx, testUser, idLab); err != nil {
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

	err := NewDeleter(s).Delete(context.Background(), testUser, idLab)
	if !errors.Is(err, context.Canceled) || s.entry == nil {
		t.Errorf("error %v, entry still there: %v", err, s.entry != nil)
	}
}

func TestDelete_LogOnlyID(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	if err := NewDeleter(deleteData()).Delete(context.Background(), testUser, idLab); err != nil {
		t.Fatal(err)
	}
	text := log.String()
	if !strings.Contains(text, "entry deleted") || !strings.Contains(text, idLab) {
		t.Errorf("log without message or ID: %s", text)
	}
	for _, content := range []string{"Geheim", "geheim", "Hämoglobin", "14,2", "lab_report", throwawayFile} {
		if strings.Contains(text, content) {
			t.Errorf("log contains %q: %s", content, text)
		}
	}
}

// Another user deleting testUser's entry: "not found", nothing is deleted,
// and no delete step runs at all.
func TestDelete_OtherUserIsNotFound(t *testing.T) {
	s := deleteData()
	err := NewDeleter(s).Delete(context.Background(), otherUser, idLab)
	if !errors.Is(err, core.ErrEntryNotFound) {
		t.Fatalf("err = %v, want ErrEntryNotFound", err)
	}
	if s.entry == nil || !s.chunks || !s.ocr || !s.file {
		t.Error("something of testUser was deleted by another user")
	}
	if !reflect.DeepEqual(s.log, []string{"GetEntry(" + idLab + ")"}) {
		t.Errorf("calls = %v, want only GetEntry", s.log)
	}
	onlyUser(t, "store", s.users, otherUser)
}

// Every store call carries the user of the request.
func TestDelete_PassesTheUserToEveryStep(t *testing.T) {
	s := deleteData()
	if err := NewDeleter(s).Delete(context.Background(), testUser, idLab); err != nil {
		t.Fatal(err)
	}
	if len(s.users) != len(allSteps) {
		t.Fatalf("%d calls, want %d", len(s.users), len(allSteps))
	}
	onlyUser(t, "store", s.users, testUser)
}

func TestDelete_WithoutUser(t *testing.T) {
	s := deleteData()
	if err := NewDeleter(s).Delete(context.Background(), "", idLab); !errors.Is(err, core.ErrNoUser) {
		t.Errorf("err = %v, want ErrNoUser", err)
	}
	if len(s.log) != 0 {
		t.Errorf("store called without a user: %v", s.log)
	}
}
