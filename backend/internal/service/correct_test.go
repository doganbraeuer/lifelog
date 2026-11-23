package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// fakeCorrectionStore holds one entry in memory and remembers what was
// written.
type fakeCorrectionStore struct {
	entry    *core.Entry // nil: the entry does not exist
	readErr  error
	saveErr  error
	gone     bool // the entry disappears between reading and writing
	reads    int
	saved    []core.Entry
	ocr      *core.OCRResult // stored words; nil: none
	ocrErr   error
	ocrReads int
	users    []string // user ID of every call; the data belongs to testUser
}

func (s *fakeCorrectionStore) GetOCR(_ context.Context, userID, id string) (core.OCRResult, bool, error) {
	s.ocrReads++
	s.users = append(s.users, userID)
	if s.ocrErr != nil {
		return core.OCRResult{}, false, s.ocrErr
	}
	if s.ocr == nil || userID != testUser {
		return core.OCRResult{}, false, nil
	}
	return *s.ocr, true, nil
}

func (s *fakeCorrectionStore) GetEntry(_ context.Context, userID, id string) (core.Entry, bool, error) {
	s.reads++
	s.users = append(s.users, userID)
	if s.readErr != nil {
		return core.Entry{}, false, s.readErr
	}
	if s.entry == nil || s.entry.ID != id || userID != testUser {
		return core.Entry{}, false, nil
	}
	return *s.entry, true, nil
}

func (s *fakeCorrectionStore) SaveCorrection(_ context.Context, userID string, e core.Entry) (bool, error) {
	s.users = append(s.users, userID)
	if s.saveErr != nil {
		return false, s.saveErr
	}
	if s.gone || userID != testUser {
		return false, nil
	}
	s.saved = append(s.saved, e)
	return true, nil
}

func correctionData() *fakeCorrectionStore {
	return &fakeCorrectionStore{entry: &core.Entry{
		ID: idLab, Module: "health", Type: "lab_report",
		DocumentDate: ptr("2026-10-14"), Issuer: ptr("Labor Muster GmbH"),
		Facts:       []core.Fact{{ID: "f2", Name: "Leukozyten", Value: "6,3", Unit: ptr("G/l")}},
		Corrections: []core.Correction{},
	}}
}

func newFixedCorrector(s CorrectionStore) *Corrector {
	c := NewCorrector(s)
	// With nanoseconds and in another time zone, to check the truncation to
	// milliseconds and the conversion to UTC.
	c.now = func() time.Time {
		return time.Date(2026, 11, 4, 20, 5, 31, 123_456_789, time.FixedZone("CET", 3600))
	}
	return c
}

func TestCorrect_Success(t *testing.T) {
	s := correctionData()
	e, err := newFixedCorrector(s).Correct(context.Background(), testUser, idLab,
		core.Change{Facts: []core.FactChange{{ID: "f2", Value: ptr("6,8")}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 1 {
		t.Fatalf("saved %d times", len(s.saved))
	}
	if e.Facts[0].Value != "6,8" || s.saved[0].Facts[0].Value != "6,8" {
		t.Errorf("response %q, saved %q", e.Facts[0].Value, s.saved[0].Facts[0].Value)
	}
	want := time.Date(2026, 11, 4, 19, 5, 31, 123_000_000, time.UTC)
	if c := e.Corrections; len(c) != 1 || !c[0].CorrectedAt.Equal(want) || c[0].CorrectedAt.Location() != time.UTC {
		t.Errorf("corrections = %+v, want CorrectedAt %v", c, want)
	}
}

// A text import with the import day: a PATCH of documentDate removes the
// mark, stores it and checks again (API.md 3.5).
func TestCorrect_ImportDateMarkRemoved(t *testing.T) {
	text := "Blutdruck gestern 130/85 mmHg"
	importDated := func() *fakeCorrectionStore {
		return &fakeCorrectionStore{entry: &core.Entry{
			ID: idLab, Module: "health", Type: "other",
			DocumentDate: ptr("2026-11-23"), DocumentDateSource: core.DateSourceImport,
			ImportedAt: time.Date(2026, 11, 23, 8, 46, 0, 0, time.UTC),
			FullText:   text, Corrections: []core.Correction{},
			Verification: core.Verification{Status: core.VerificationChecked, Source: core.SourceText,
				Fields: []core.FieldCheck{}},
		}}
	}

	// A new date: correction from null, mark gone, checked against the text
	// ("gestern" confirms 2026-11-22).
	s := importDated()
	e, err := newFixedCorrector(s).Correct(context.Background(), testUser, idLab,
		core.Change{DocumentDate: core.Nullable{Set: true, Value: ptr("2026-11-22")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 1 || s.saved[0].DocumentDateSource != core.DateSourceDocument {
		t.Fatalf("saved %+v, want one save with source document", s.saved)
	}
	if len(e.Corrections) != 1 || e.Corrections[0].Recognized != nil || deref(e.Corrections[0].Corrected) != "2026-11-22" {
		t.Errorf("corrections %+v", e.Corrections)
	}
	i := slices.IndexFunc(e.Verification.Fields, func(f core.FieldCheck) bool { return f.Field == "documentDate" })
	if i < 0 || e.Verification.Fields[i].Status != core.FieldCorrectedConfirmed {
		t.Errorf("verification %+v, want documentDate corrected_confirmed", e.Verification.Fields)
	}

	// null: no date, no correction, but stored.
	s = importDated()
	e, err = newFixedCorrector(s).Correct(context.Background(), testUser, idLab,
		core.Change{DocumentDate: core.Nullable{Set: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 1 || e.DocumentDate != nil || e.DocumentDateSource != core.DateSourceDocument || len(e.Corrections) != 0 {
		t.Errorf("saved %d, date %v, source %q, corrections %+v", len(s.saved), deref(e.DocumentDate), e.DocumentDateSource, e.Corrections)
	}
}

func TestCorrect_NoChangeNoWrite(t *testing.T) {
	s := correctionData()
	e, err := newFixedCorrector(s).Correct(context.Background(), testUser, idLab,
		core.Change{Issuer: core.Nullable{Set: true, Value: ptr("Labor Muster GmbH")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 0 {
		t.Error("saved without a change")
	}
	if e.ID != idLab || len(e.Corrections) != 0 {
		t.Errorf("response = %+v, want the unchanged entry", e)
	}
}

func TestCorrect_Errors(t *testing.T) {
	internal := errors.New("database gone")
	valid := core.Change{Type: ptr("doctor_letter")}
	cases := []struct {
		name  string
		s     *fakeCorrectionStore
		id    string
		c     core.Change
		err   error
		reads int
	}{
		{"invalid ID", correctionData(), "abc", valid, core.ErrInvalidID, 0},
		{"not found", correctionData(), idLetter, valid, core.ErrEntryNotFound, 1},
		{"reading fails", &fakeCorrectionStore{readErr: internal}, idLab, valid, internal, 1},
		{"invalid correction", correctionData(), idLab, core.Change{Type: ptr("befund")}, core.ErrInvalidCorrection, 1},
		{"unknown fact", correctionData(), idLab,
			core.Change{Facts: []core.FactChange{{ID: "f9", Value: ptr("1")}}}, core.ErrFactNotFound, 1},
		{"saving fails", func() *fakeCorrectionStore { s := correctionData(); s.saveErr = internal; return s }(),
			idLab, valid, internal, 1},
		{"deleted in between", func() *fakeCorrectionStore { s := correctionData(); s.gone = true; return s }(),
			idLab, valid, core.ErrEntryNotFound, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := newFixedCorrector(c.s).Correct(context.Background(), testUser, c.id, c.c)
			if !errors.Is(err, c.err) {
				t.Errorf("err = %v, want %v", err, c.err)
			}
			if c.s.reads != c.reads {
				t.Errorf("read %d times, want %d", c.s.reads, c.reads)
			}
			if len(c.s.saved) != 0 {
				t.Error("saved despite an error")
			}
		})
	}
}

func TestCorrect_LogWithoutValues(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	_, err := newFixedCorrector(correctionData()).Correct(context.Background(), testUser, idLab, core.Change{
		Issuer: core.Nullable{Set: true, Value: ptr("Dr. Geheim")},
		Facts:  []core.FactChange{{ID: "f2", Value: ptr("7,7")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "issuer") || !strings.Contains(log.String(), "facts.f2.value") {
		t.Errorf("field names missing in the log: %s", log.String())
	}
	for _, value := range []string{"Dr. Geheim", "7,7", "6,3", "Labor Muster"} {
		if strings.Contains(log.String(), value) {
			t.Errorf("value %q in the log: %s", value, log.String())
		}
	}
}

// Another user correcting testUser's entry: "not found", nothing is saved.
func TestCorrect_OtherUserIsNotFound(t *testing.T) {
	s := correctionData()
	_, err := newFixedCorrector(s).Correct(context.Background(), otherUser, idLab,
		core.Change{Issuer: core.Nullable{Set: true, Value: ptr("Fremd")}})
	if !errors.Is(err, core.ErrEntryNotFound) {
		t.Fatalf("err = %v, want ErrEntryNotFound", err)
	}
	if len(s.saved) != 0 || *s.entry.Issuer != "Labor Muster GmbH" {
		t.Error("testUser's entry was changed by another user")
	}
	onlyUser(t, "store", s.users, otherUser)
}

func TestCorrect_PassesTheUserToEveryCall(t *testing.T) {
	s := correctionData()
	if _, err := newFixedCorrector(s).Correct(context.Background(), testUser, idLab,
		core.Change{Issuer: core.Nullable{Set: true, Value: ptr("Labor Neu")}}); err != nil {
		t.Fatal(err)
	}
	onlyUser(t, "store", s.users, testUser)
}

func TestCorrect_WithoutUser(t *testing.T) {
	s := correctionData()
	if _, err := newFixedCorrector(s).Correct(context.Background(), "", idLab, core.Change{}); !errors.Is(err, core.ErrNoUser) {
		t.Errorf("err = %v, want ErrNoUser", err)
	}
	if s.reads != 0 {
		t.Error("store called without a user")
	}
}
