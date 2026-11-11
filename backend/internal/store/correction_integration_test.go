package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// Only runs with LIFELOG_MONGO_TEST=1 (see integration_test.go).
func TestIntegration_SaveCorrection(t *testing.T) {
	s, ctx := testStore(t)

	e := testEntry(NewID(), NewID(), strings.Repeat("cd", 32))
	e.Facts = append(e.Facts, core.Fact{ID: "f2", Name: "Befundart", Value: "Blutbild klein"})
	if err := s.SaveEntry(ctx, e); err != nil {
		t.Fatal(err)
	}

	// All kinds of correction at once, including null values: date and unit
	// removed, unit added, type and value changed.
	now := time.Date(2026, 11, 4, 19, 5, 31, 123_000_000, time.UTC) // milliseconds as in the service
	issuer := "Labor B"
	updated, changed, err := core.Correct(e, core.Change{
		Type:         &[]string{"arztbrief"}[0],
		DocumentDate: core.Nullable{Set: true},
		Issuer:       core.Nullable{Set: true, Value: &issuer},
		Facts: []core.FactChange{
			{ID: "f1", Value: &[]string{"14,0"}[0], Unit: core.Nullable{Set: true}},
			{ID: "f2", Unit: core.Nullable{Set: true, Value: &[]string{"Stück"}[0]}},
		},
	}, now)
	if err != nil || len(changed) != 6 {
		t.Fatalf("Correct: %v, changed %v", err, changed)
	}

	found, err := s.SaveCorrection(ctx, updated)
	if err != nil || !found {
		t.Fatalf("SaveCorrection: found %v, %v", found, err)
	}

	read, _, err := s.GetEntry(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	// There and back through MongoDB: exactly the corrected entry, including
	// null values and the time in UTC.
	if !reflect.DeepEqual(read, updated) {
		t.Errorf("read\n%+v\nwant\n%+v", read, updated)
	}
	// Fields that are not correctable are unchanged.
	if read.FullText != e.FullText || read.File != e.File || !read.ImportedAt.Equal(e.ImportedAt) || read.Module != e.Module {
		t.Errorf("non-correctable fields changed: %+v", read)
	}

	// If the entry does not exist, SaveCorrection reports that instead of
	// creating a new one.
	other := updated
	other.ID = NewID()
	found, err = s.SaveCorrection(ctx, other)
	if err != nil || found {
		t.Errorf("unknown entry: found %v, %v", found, err)
	}
	if _, exists, _ := s.GetEntry(ctx, other.ID); exists {
		t.Error("SaveCorrection created a new entry")
	}
}
