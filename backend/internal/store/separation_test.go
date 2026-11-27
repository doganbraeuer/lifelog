package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

// Every function that reads, changes or deletes entries, chunks, OCR words,
// originals, routines or routine logs refuses a missing or invalid owner
// with core.ErrNoUser,
// before it touches the database: the Store here has no connection at all,
// so a function that forgot the check would panic instead of passing.
//
// A new store function for user data belongs in this list.
func TestOwnerRequiredEverywhere(t *testing.T) {
	s := &Store{} // no collections: any database access panics
	ctx := context.Background()
	id := "6727a1f3c2b4e5d6f7a8b9c0"
	entry := core.Entry{ID: id, File: core.File{ID: id}}
	routine := core.Routine{ID: id, Name: "x", Schedule: core.Schedule{Frequency: core.FrequencyDaily, Interval: 1}}
	routineLog := core.RoutineLog{ID: id, RoutineID: id}
	calls := map[string]func(userID string) error{
		"ListSummaries": func(u string) error { _, err := s.ListSummaries(ctx, u, "health"); return err },
		"CountEntries":  func(u string) error { _, err := s.CountEntries(ctx, u); return err },
		"SaveEntry":     func(u string) error { return s.SaveEntry(ctx, u, entry) },
		"GetEntry":      func(u string) error { _, _, err := s.GetEntry(ctx, u, id); return err },
		"OpenFile":      func(u string) error { _, _, err := s.OpenFile(ctx, u, id); return err },
		"EntryByHash":   func(u string) error { _, _, err := s.EntryByHash(ctx, u, "abc"); return err },
		"SaveFile":      func(u string) error { return s.SaveFile(ctx, u, id, "a.pdf", core.MimePDF, []byte("x")) },
		"DeleteFile":    func(u string) error { return s.DeleteFile(ctx, u, id) },
		"SaveChunks":    func(u string) error { return s.SaveChunks(ctx, u, nil) },
		"DeleteChunks":  func(u string) error { return s.DeleteChunks(ctx, u, id) },
		"SaveCorrection": func(u string) error {
			_, err := s.SaveCorrection(ctx, u, entry)
			return err
		},
		"DeleteEntry": func(u string) error { _, err := s.DeleteEntry(ctx, u, id); return err },
		"SaveOCR":     func(u string) error { return s.SaveOCR(ctx, u, id, core.OCRResult{}) },
		"GetOCR":      func(u string) error { _, _, err := s.GetOCR(ctx, u, id); return err },
		"DeleteOCR":   func(u string) error { return s.DeleteOCR(ctx, u, id) },
		"SaveVerification": func(u string) error {
			_, err := s.SaveVerification(ctx, u, id, core.Verification{})
			return err
		},
		"OCRIDs": func(u string) error { _, err := s.OCRIDs(ctx, u); return err },
		"SimilarChunks": func(u string) error {
			_, err := s.SimilarChunks(ctx, u, []float32{1}, "health", 5)
			return err
		},
		"SummariesByIDs": func(u string) error { _, err := s.SummariesByIDs(ctx, u, []string{id}); return err },
		"AssignOrphans":  func(u string) error { _, err := s.AssignOrphans(ctx, u); return err },
		"DeleteUserData": func(u string) error { _, err := s.DeleteUserData(ctx, u); return err },
		"CountUserData":  func(u string) error { _, err := s.CountUserData(ctx, u); return err },
		"IncrementUsage": func(u string) error {
			_, err := s.IncrementUsage(ctx, u, "2026-11-18", core.LimitQuestions, 5, time.Now())
			return err
		},

		// Routines and their logs (routines.go).
		"CreateRoutine":   func(u string) error { return s.CreateRoutine(ctx, u, routine) },
		"ListRoutines":    func(u string) error { _, err := s.ListRoutines(ctx, u, "health"); return err },
		"GetRoutine":      func(u string) error { _, _, err := s.GetRoutine(ctx, u, id); return err },
		"CountRoutines":   func(u string) error { _, err := s.CountRoutines(ctx, u); return err },
		"RoutineIDByName": func(u string) error { _, _, err := s.RoutineIDByName(ctx, u, "health", "x"); return err },
		"UpdateRoutine": func(u string) error {
			_, err := s.UpdateRoutine(ctx, u, routine, []string{"name"})
			return err
		},
		"SetPauses":        func(u string) error { _, err := s.SetPauses(ctx, u, id, nil, nil); return err },
		"DeleteRoutine":    func(u string) error { _, err := s.DeleteRoutine(ctx, u, id); return err },
		"CreateRoutineLog": func(u string) error { return s.CreateRoutineLog(ctx, u, routineLog) },
		"RoutineLogIDByDue": func(u string) error {
			_, _, err := s.RoutineLogIDByDue(ctx, u, id, "2026-11-23T08:00")
			return err
		},
		"CountRoutineLogsTaken": func(u string) error {
			_, err := s.CountRoutineLogsTaken(ctx, u, id, time.Now(), time.Now())
			return err
		},
		"RoutineLogs": func(u string) error { _, err := s.RoutineLogs(ctx, u, id, time.Now(), time.Now()); return err },
		"RoutineLogsInRange": func(u string) error {
			_, err := s.RoutineLogsInRange(ctx, u, []string{id}, 0, 1, time.Now(), time.Now())
			return err
		},
		"DeleteRoutineLog":  func(u string) error { _, err := s.DeleteRoutineLog(ctx, u, id, id); return err },
		"DeleteRoutineLogs": func(u string) error { _, err := s.DeleteRoutineLogs(ctx, u, id); return err },
	}
	for name, call := range calls {
		for _, userID := range []string{"", "nope", "000000000000000000000000"} {
			t.Run(fmt.Sprintf("%s/%q", name, userID), func(t *testing.T) {
				defer func() {
					if p := recover(); p != nil {
						t.Fatalf("touched the database without an owner: %v", p)
					}
				}()
				if err := call(userID); !errors.Is(err, core.ErrNoUser) {
					t.Errorf("err = %v, want core.ErrNoUser", err)
				}
			})
		}
	}
}

func TestSaveEntry_RefusesOtherOwner(t *testing.T) {
	s := &Store{}
	e := core.Entry{ID: NewID(), UserID: NewID(), File: core.File{ID: NewID()}}
	if err := s.SaveEntry(context.Background(), NewID(), e); err == nil {
		t.Error("an entry of one user must not be saved for another")
	}
}

// The owner and module filter is inside $vectorSearch, not a later $match.
func TestVectorSearchPipeline_FiltersByOwnerAndModule(t *testing.T) {
	owner := bson.NewObjectID()
	p := vectorSearchPipeline(owner, []float32{0.1, 0.2}, "health", 5)
	if len(p) == 0 || p[0][0].Key != "$vectorSearch" {
		t.Fatalf("first stage = %v, want $vectorSearch", p)
	}
	stage := p[0][0].Value.(bson.D)
	var filter bson.D
	for _, e := range stage {
		if e.Key == "filter" {
			filter = e.Value.(bson.D)
		}
	}
	if filter == nil {
		t.Fatal("$vectorSearch has no filter")
	}
	raw, err := bson.MarshalExtJSON(filter, true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"$and":[{"userId":{"$eq":{"$oid":"%s"}}},{"module":{"$eq":"health"}}]}`, owner.Hex())
	if string(raw) != want {
		t.Errorf("filter =\n%s\nwant\n%s", raw, want)
	}
	for _, stage := range p[1:] {
		if stage[0].Key == "$match" {
			t.Error("the owner filter must not be a later $match")
		}
	}
}

func TestNeedsUpdate(t *testing.T) {
	want, err := definitionFields(vectorIndexDefinition)
	if err != nil {
		t.Fatal(err)
	}
	old := []indexField{{Type: "vector", Path: "embedding"}, {Type: "filter", Path: "module"}}
	if !needsUpdate(old, want) {
		t.Error("an index without the userId filter must be updated")
	}
	if needsUpdate(want, want) {
		t.Error("an index with the current definition must be left alone")
	}
	extra := append([]indexField{{Type: "filter", Path: "other"}}, want...)
	if needsUpdate(extra, want) {
		t.Error("an additional field alone is no reason to rebuild")
	}
}

func TestIsFilterNotIndexed(t *testing.T) {
	for _, msg := range []string{
		// Exactly as Atlas reported it on 2026-11-18 while the index was
		// BUILDING after the update and still served the old definition.
		"(UnknownError) PlanExecutor error during aggregation :: caused by :: Path 'userId' needs to be indexed as filter",
		"Path 'userId' needs to be indexed as a filter",
	} {
		if !isFilterNotIndexed(errors.New(msg)) {
			t.Errorf("not recognized: %s", msg)
		}
	}
	for _, err := range []error{nil, errors.New("connection reset"), errors.New("index not found")} {
		if isFilterNotIndexed(err) {
			t.Errorf("wrongly recognized: %v", err)
		}
	}
}
