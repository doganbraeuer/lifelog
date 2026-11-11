package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// fakeCorrectionStore holds one entry in memory and remembers what was
// written.
type fakeCorrectionStore struct {
	entry   *core.Entry // nil: the entry does not exist
	readErr error
	saveErr error
	gone    bool // the entry disappears between reading and writing
	reads   int
	saved   []core.Entry
}

func (s *fakeCorrectionStore) GetEntry(_ context.Context, id string) (core.Entry, bool, error) {
	s.reads++
	if s.readErr != nil {
		return core.Entry{}, false, s.readErr
	}
	if s.entry == nil || s.entry.ID != id {
		return core.Entry{}, false, nil
	}
	return *s.entry, true, nil
}

func (s *fakeCorrectionStore) SaveCorrection(_ context.Context, e core.Entry) (bool, error) {
	if s.saveErr != nil {
		return false, s.saveErr
	}
	if s.gone {
		return false, nil
	}
	s.saved = append(s.saved, e)
	return true, nil
}

func correctionData() *fakeCorrectionStore {
	return &fakeCorrectionStore{entry: &core.Entry{
		ID: idLab, Module: "gesundheit", Type: "laborbefund",
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
	e, err := newFixedCorrector(s).Correct(context.Background(), idLab,
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

func TestCorrect_NoChangeNoWrite(t *testing.T) {
	s := correctionData()
	e, err := newFixedCorrector(s).Correct(context.Background(), idLab,
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
	valid := core.Change{Type: ptr("arztbrief")}
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
			_, err := newFixedCorrector(c.s).Correct(context.Background(), c.id, c.c)
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

	_, err := newFixedCorrector(correctionData()).Correct(context.Background(), idLab, core.Change{
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
