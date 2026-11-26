package store

// Routines and routine logs against a real MongoDB. Only runs with
// LIFELOG_MONGO_TEST=1 (see integration_test.go), in the test database of
// its own; only these tests:
//
//	LIFELOG_MONGO_TEST=1 go test ./internal/store/ -run IntegrationRoutines -v
//
// All data is synthetic.

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

const dailyBody = `"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]}, "startDate": "2026-11-18"`

// routineOf builds a routine of the user with the name, created at the
// given minute of 2026-11-23.
func routineOf(t *testing.T, userID, name string, minute int) core.Routine {
	t.Helper()
	r := parsedRoutine(t, `{"name": "`+name+`", "amount": 2, "unit": "Stück", `+dailyBody+`}`)
	r.UserID = userID
	r.CreatedAt = time.Date(2026, 11, 23, 8, minute, 0, 0, time.UTC)
	return r
}

func logOf(t *testing.T, r core.Routine, dueLocal string, takenAt time.Time) core.RoutineLog {
	t.Helper()
	l := core.RoutineLog{ID: NewID(), UserID: r.UserID, RoutineID: r.ID, TakenAt: takenAt, Status: core.LogDone,
		Source: core.LogSourceManual, TimeZone: "Europe/Vienna", CreatedAt: takenAt}
	if dueLocal != "" {
		due := takenAt.Add(-2 * time.Minute).Truncate(time.Minute)
		l.DueAt, l.DueLocal = &due, dueLocal
	}
	return l
}

func TestIntegrationRoutines_Indexes(t *testing.T) {
	s, ctx := testStore(t)
	// Creating the indexes a second time must not break anything.
	if err := s.CreateIndexes(ctx); err != nil {
		t.Fatalf("CreateIndexes is not repeatable: %v", err)
	}
	specs, err := s.routines.Indexes().ListSpecifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, spec := range specs {
		found[spec.Name] = true
		if spec.Name == "user_module_name_unique" && (spec.Unique == nil || !*spec.Unique) {
			t.Error("user_module_name_unique is not unique")
		}
	}
	if !found["user_module"] || !found["user_module_name_unique"] {
		t.Errorf("routines indexes: %v", found)
	}

	cur, err := s.routineLogs.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var raw []bson.M
	if err := cur.All(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	names := map[string]bson.M{}
	for _, idx := range raw {
		names[idx["name"].(string)] = idx
	}
	due, ok := names["routine_due_unique"]
	if !ok || due["unique"] != true {
		t.Fatalf("routine_due_unique missing or not unique: %v", due)
	}
	// Driver v2 decodes nested documents in a bson.M as bson.D, so read the
	// filter as raw BSON instead of asserting a map type.
	pfe, err := bson.Marshal(due["partialFilterExpression"])
	if err != nil {
		t.Fatalf("partial filter not marshalable: %v", err)
	}
	if typ, ok := bson.Raw(pfe).Lookup("dueLocal", "$type").StringValueOK(); !ok || typ != "string" {
		t.Errorf("partial filter = %v, want {dueLocal: {$type: string}}", due["partialFilterExpression"])
	}
	if _, ok := names["user_routine_taken"]; !ok {
		t.Errorf("user_routine_taken missing: %v", names)
	}
}

func TestIntegrationRoutines_RoundTripAndList(t *testing.T) {
	s, ctx := testStore(t)
	later := routineOf(t, userA, "Vitamin D", 30)
	earlier := routineOf(t, userA, "Blutdruck messen", 10)
	until := core.DateFromYMD(2026, 11, 30)
	earlier.Pauses = []core.Pause{{From: core.DateFromYMD(2026, 11, 20), Until: &until}}
	for _, r := range []core.Routine{later, earlier} {
		if err := s.CreateRoutine(ctx, userA, r); err != nil {
			t.Fatal(err)
		}
	}

	got, found, err := s.GetRoutine(ctx, userA, earlier.ID)
	if err != nil || !found {
		t.Fatalf("GetRoutine = %v, %v", found, err)
	}
	if !equalRoutines(got, earlier) {
		t.Errorf("round trip:\n%+v\n%+v", got, earlier)
	}

	list, err := s.ListRoutines(ctx, userA, "health")
	if err != nil || len(list) != 2 || list[0].ID != earlier.ID || list[1].ID != later.ID {
		t.Errorf("ListRoutines = %v, %v; want sorted by createdAt", list, err)
	}
	if n, err := s.CountRoutines(ctx, userA); err != nil || n != 2 {
		t.Errorf("CountRoutines = %d, %v", n, err)
	}
	if list, _ := s.ListRoutines(ctx, userB, "health"); len(list) != 0 {
		t.Errorf("B lists A's routines: %v", list)
	}
}

// equalRoutines compares routines as they come back from MongoDB (times in
// milliseconds).
func equalRoutines(a, b core.Routine) bool {
	return a.ID == b.ID && a.UserID == b.UserID && a.Name == b.Name && a.Module == b.Module &&
		equalFloat(a.Amount, b.Amount) && equalString(a.Unit, b.Unit) && a.Schedule.Equal(b.Schedule) &&
		a.StartDate == b.StartDate && equalDate(a.EndDate, b.EndDate) && slices.EqualFunc(a.Pauses, b.Pauses,
		func(x, y core.Pause) bool { return x.From == y.From && equalDate(x.Until, y.Until) }) &&
		a.CreatedAt.Equal(b.CreatedAt)
}

func equalFloat(a, b *float64) bool  { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func equalString(a, b *string) bool  { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func equalDate(a, b *core.Date) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// A name exists once per user and module, ignoring case. The duplicate key
// is reported as core.ErrRoutineNameTaken itself, never with MongoDB's
// message, which would contain the name.
func TestIntegrationRoutines_NameUnique(t *testing.T) {
	s, ctx := testStore(t)
	first := routineOf(t, userA, "Vitamin D", 1)
	if err := s.CreateRoutine(ctx, userA, first); err != nil {
		t.Fatal(err)
	}
	err := s.CreateRoutine(ctx, userA, routineOf(t, userA, "VITAMIN d", 2))
	if !errors.Is(err, core.ErrRoutineNameTaken) || err.Error() != core.ErrRoutineNameTaken.Error() ||
		strings.Contains(strings.ToLower(err.Error()), "vitamin") {
		t.Errorf("same name: err = %v, want exactly core.ErrRoutineNameTaken", err)
	}
	if id, found, err := s.RoutineIDByName(ctx, userA, "health", "vitamin d"); err != nil || !found || id != first.ID {
		t.Errorf("RoutineIDByName = %s, %v, %v; want %s", id, found, err, first.ID)
	}
	// Another user may have the same name.
	if err := s.CreateRoutine(ctx, userB, routineOf(t, userB, "Vitamin D", 3)); err != nil {
		t.Errorf("B with the same name: %v", err)
	}
	if _, found, _ := s.RoutineIDByName(ctx, userB, "health", "blutdruck"); found {
		t.Error("RoutineIDByName found a name that does not exist")
	}

	// Renaming onto a taken name, and only changing the case of its own.
	other := routineOf(t, userA, "Blutdruck", 4)
	if err := s.CreateRoutine(ctx, userA, other); err != nil {
		t.Fatal(err)
	}
	other.Name = "vitamin D"
	if _, err := s.UpdateRoutine(ctx, userA, other, []string{"name"}); !errors.Is(err, core.ErrRoutineNameTaken) ||
		strings.Contains(strings.ToLower(err.Error()), "vitamin") {
		t.Errorf("rename onto a taken name: %v", err)
	}
	first.Name = "VITAMIN D"
	if found, err := s.UpdateRoutine(ctx, userA, first, []string{"name"}); err != nil || !found {
		t.Errorf("own name in other case: %v, %v", found, err)
	}
	if got, _, _ := s.GetRoutine(ctx, userA, first.ID); got.Name != "VITAMIN D" {
		t.Errorf("name after rename = %q", got.Name)
	}
}

func TestIntegrationRoutines_UpdateOnlyNamedFields(t *testing.T) {
	s, ctx := testStore(t)
	r := routineOf(t, userA, "Vitamin D", 1)
	if err := s.CreateRoutine(ctx, userA, r); err != nil {
		t.Fatal(err)
	}
	changed := r
	changed.Name = "Anderer Name"
	end := core.DateFromYMD(2026, 12, 24)
	changed.EndDate = &end
	changed.Amount, changed.Unit = nil, nil
	// Only endDate is named: name and amount stay as stored.
	if found, err := s.UpdateRoutine(ctx, userA, changed, []string{"endDate"}); err != nil || !found {
		t.Fatalf("UpdateRoutine = %v, %v", found, err)
	}
	got, _, _ := s.GetRoutine(ctx, userA, r.ID)
	if got.Name != "Vitamin D" || got.Amount == nil || got.EndDate == nil || *got.EndDate != end {
		t.Errorf("after update: %+v", got)
	}
	if _, err := s.UpdateRoutine(ctx, userA, changed, []string{"pauses"}); err == nil {
		t.Error("a field that cannot be stored was accepted")
	}
}

// Compare and set: pauses are only replaced if they are still the old ones.
func TestIntegrationRoutines_SetPauses(t *testing.T) {
	s, ctx := testStore(t)
	r := routineOf(t, userA, "Vitamin D", 1)
	r.Pauses = []core.Pause{}
	if err := s.CreateRoutine(ctx, userA, r); err != nil {
		t.Fatal(err)
	}
	first := []core.Pause{{From: core.DateFromYMD(2026, 11, 23)}}
	if ok, err := s.SetPauses(ctx, userA, r.ID, []core.Pause{}, first); err != nil || !ok {
		t.Fatalf("first pause: %v, %v", ok, err)
	}
	// A second writer that still has the old (empty) list loses.
	if ok, err := s.SetPauses(ctx, userA, r.ID, []core.Pause{}, []core.Pause{{From: core.DateFromYMD(2026, 11, 24)}}); err != nil || ok {
		t.Errorf("stale pauses: %v, %v; want not updated", ok, err)
	}
	until := core.DateFromYMD(2026, 11, 30)
	second := []core.Pause{{From: core.DateFromYMD(2026, 11, 23), Until: &until}}
	if ok, err := s.SetPauses(ctx, userA, r.ID, first, second); err != nil || !ok {
		t.Errorf("with the current pauses: %v, %v", ok, err)
	}
	got, _, _ := s.GetRoutine(ctx, userA, r.ID)
	if len(got.Pauses) != 1 || got.Pauses[0].Until == nil || *got.Pauses[0].Until != until {
		t.Errorf("pauses = %+v", got.Pauses)
	}
}

// One log per occurrence; logs without an occurrence as many as needed.
func TestIntegrationRoutines_LogPerOccurrence(t *testing.T) {
	s, ctx := testStore(t)
	r := routineOf(t, userA, "Vitamin D", 1)
	other := routineOf(t, userA, "Blutdruck", 2)
	for _, x := range []core.Routine{r, other} {
		if err := s.CreateRoutine(ctx, userA, x); err != nil {
			t.Fatal(err)
		}
	}
	taken := time.Date(2026, 11, 23, 7, 2, 0, 0, time.UTC)
	first := logOf(t, r, "2026-11-23T08:00", taken)
	if err := s.CreateRoutineLog(ctx, userA, first); err != nil {
		t.Fatal(err)
	}
	err := s.CreateRoutineLog(ctx, userA, logOf(t, r, "2026-11-23T08:00", taken.Add(time.Minute)))
	if !errors.Is(err, core.ErrRoutineLogExists) || err.Error() != core.ErrRoutineLogExists.Error() {
		t.Errorf("second log for the occurrence: %v", err)
	}
	if id, found, err := s.RoutineLogIDByDue(ctx, userA, r.ID, "2026-11-23T08:00"); err != nil || !found || id != first.ID {
		t.Errorf("RoutineLogIDByDue = %s, %v, %v", id, found, err)
	}
	// The same occurrence of another routine, and logs without one.
	if err := s.CreateRoutineLog(ctx, userA, logOf(t, other, "2026-11-23T08:00", taken)); err != nil {
		t.Errorf("other routine: %v", err)
	}
	for i := range 3 {
		if err := s.CreateRoutineLog(ctx, userA, logOf(t, r, "", taken.Add(time.Duration(i)*time.Hour))); err != nil {
			t.Errorf("log without occurrence %d: %v", i, err)
		}
	}
}

func TestIntegrationRoutines_LogQueries(t *testing.T) {
	s, ctx := testStore(t)
	r := routineOf(t, userA, "Wasser", 1)
	if err := s.CreateRoutine(ctx, userA, r); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	logs := []core.RoutineLog{
		logOf(t, r, "2026-11-22T20:00", day.Add(-4*time.Hour+2*time.Minute)), // Sunday evening
		logOf(t, r, "2026-11-23T08:00", day.Add(7*time.Hour+2*time.Minute)),
		logOf(t, r, "", day.Add(10*time.Hour)),                               // extra glass on Monday
		logOf(t, r, "2026-11-20T08:00", day.Add(9*time.Hour)),                // Friday's, logged on Monday
		logOf(t, r, "2026-11-25T08:00", day.Add(2*24*time.Hour+7*time.Hour)), // Wednesday
	}
	for _, l := range logs {
		if err := s.CreateRoutineLog(ctx, userA, l); err != nil {
			t.Fatal(err)
		}
	}

	// takenAt on Monday (UTC here): newest first.
	list, err := s.RoutineLogs(ctx, userA, r.ID, day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if ids := logIDs(list); !slices.Equal(ids, []string{logs[2].ID, logs[3].ID, logs[1].ID}) {
		t.Errorf("RoutineLogs = %v, want newest first", ids)
	}
	if n, err := s.CountRoutineLogsTaken(ctx, userA, r.ID, day, day.Add(24*time.Hour)); err != nil || n != 3 {
		t.Errorf("CountRoutineLogsTaken = %d, %v", n, err)
	}

	// Occurrences on Monday and Tuesday, or taken then.
	monday, tuesday := core.DateFromYMD(2026, 11, 23), core.DateFromYMD(2026, 11, 24)
	got, err := s.RoutineLogsInRange(ctx, userA, []string{r.ID}, monday, tuesday, day, day.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	ids := logIDs(got)
	slices.Sort(ids)
	want := []string{logs[1].ID, logs[2].ID, logs[3].ID}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("RoutineLogsInRange = %v, want %v", ids, want)
	}
	// Without a takenAt range only the occurrences count.
	got, _ = s.RoutineLogsInRange(ctx, userA, []string{r.ID}, monday, tuesday, day, day)
	if ids := logIDs(got); len(ids) != 1 || ids[0] != logs[1].ID {
		t.Errorf("occurrences only: %v", ids)
	}
	if got, err := s.RoutineLogsInRange(ctx, userA, nil, monday, tuesday, day, day); err != nil || got == nil || len(got) != 0 {
		t.Errorf("no routines: %v, %v", got, err)
	}
}

func logIDs(list []core.RoutineLog) []string {
	ids := make([]string, 0, len(list))
	for _, l := range list {
		ids = append(ids, l.ID)
	}
	return ids
}

// B can neither read nor change nor delete A's routine and log, also not
// with the right IDs.
func TestIntegrationRoutines_Separation(t *testing.T) {
	s, ctx := testStore(t)
	r := routineOf(t, userA, "Vitamin D", 1)
	if err := s.CreateRoutine(ctx, userA, r); err != nil {
		t.Fatal(err)
	}
	taken := time.Date(2026, 11, 23, 7, 2, 0, 0, time.UTC)
	l := logOf(t, r, "2026-11-23T08:00", taken)
	if err := s.CreateRoutineLog(ctx, userA, l); err != nil {
		t.Fatal(err)
	}

	if _, found, err := s.GetRoutine(ctx, userB, r.ID); err != nil || found {
		t.Errorf("B reads A's routine: %v, %v", found, err)
	}
	changed := r
	changed.Name = "Von B"
	if found, err := s.UpdateRoutine(ctx, userB, changed, []string{"name"}); err != nil || found {
		t.Errorf("B changes A's routine: %v, %v", found, err)
	}
	if ok, err := s.SetPauses(ctx, userB, r.ID, []core.Pause{}, []core.Pause{{From: 1}}); err != nil || ok {
		t.Errorf("B pauses A's routine: %v, %v", ok, err)
	}
	if found, err := s.DeleteRoutineLog(ctx, userB, r.ID, l.ID); err != nil || found {
		t.Errorf("B deletes A's log: %v, %v", found, err)
	}
	if n, err := s.DeleteRoutineLogs(ctx, userB, r.ID); err != nil || n != 0 {
		t.Errorf("B deletes A's logs: %d, %v", n, err)
	}
	if found, err := s.DeleteRoutine(ctx, userB, r.ID); err != nil || found {
		t.Errorf("B deletes A's routine: %v, %v", found, err)
	}
	if list, _ := s.RoutineLogs(ctx, userB, r.ID, taken.Add(-time.Hour), taken.Add(time.Hour)); len(list) != 0 {
		t.Errorf("B lists A's logs: %v", list)
	}
	if list, _ := s.RoutineLogsInRange(ctx, userB, []string{r.ID}, 0, core.MaxRoutineDate, taken.Add(-time.Hour), taken.Add(time.Hour)); len(list) != 0 {
		t.Errorf("B gets A's logs in a range: %v", list)
	}
	if _, found, _ := s.RoutineLogIDByDue(ctx, userB, r.ID, "2026-11-23T08:00"); found {
		t.Error("B finds A's log by its occurrence")
	}
	// A's data is unchanged.
	got, found, _ := s.GetRoutine(ctx, userA, r.ID)
	if !found || got.Name != "Vitamin D" || len(got.Pauses) != 0 {
		t.Errorf("A's routine after B: %+v, %v", got, found)
	}
	if list, _ := s.RoutineLogs(ctx, userA, r.ID, taken.Add(-time.Hour), taken.Add(time.Hour)); len(list) != 1 {
		t.Errorf("A's logs after B: %v", list)
	}
	// A's log under another routine of A is not found either.
	other := routineOf(t, userA, "Blutdruck", 2)
	if err := s.CreateRoutine(ctx, userA, other); err != nil {
		t.Fatal(err)
	}
	if found, err := s.DeleteRoutineLog(ctx, userA, other.ID, l.ID); err != nil || found {
		t.Errorf("log deleted through another routine: %v, %v", found, err)
	}
}

func TestIntegrationRoutines_DeleteUserData(t *testing.T) {
	s, ctx := testStore(t)
	storeEntryOf(t, s, ctx, userA, strings.Repeat("c1", 32))
	taken := time.Date(2026, 11, 23, 7, 2, 0, 0, time.UTC)
	for i, name := range []string{"Vitamin D", "Wasser"} {
		r := routineOf(t, userA, name, i)
		if err := s.CreateRoutine(ctx, userA, r); err != nil {
			t.Fatal(err)
		}
		for _, l := range []core.RoutineLog{logOf(t, r, "2026-11-23T08:00", taken), logOf(t, r, "", taken)} {
			if err := s.CreateRoutineLog(ctx, userA, l); err != nil {
				t.Fatal(err)
			}
		}
	}
	rb := routineOf(t, userB, "Vitamin D", 5)
	if err := s.CreateRoutine(ctx, userB, rb); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRoutineLog(ctx, userB, logOf(t, rb, "2026-11-23T08:00", taken)); err != nil {
		t.Fatal(err)
	}

	before, err := s.CountUserData(ctx, userA)
	if err != nil || before.Routines != 2 || before.RoutineLogs != 4 {
		t.Errorf("CountUserData(A) before = %+v, %v", before, err)
	}
	deleted, err := s.DeleteUserData(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != (DataCounts{Entries: 1, Chunks: 1, OCR: 1, Files: 1, Routines: 2, RoutineLogs: 4}) {
		t.Errorf("deleted = %+v", deleted)
	}
	if left, err := s.CountUserData(ctx, userA); err != nil || left.Total() != 0 {
		t.Errorf("left of A = %+v, %v", left, err)
	}
	if bLeft, _ := s.CountUserData(ctx, userB); bLeft != (DataCounts{Routines: 1, RoutineLogs: 1}) {
		t.Errorf("B's data = %+v, want untouched", bLeft)
	}
}
