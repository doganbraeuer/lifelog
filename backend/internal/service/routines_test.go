package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/routinetest"
)

// routinesFixture: the real service on the in-memory store, with a fixed
// clock (Monday 2026-11-23, 10:15 in Vienna). A and B are two users.
type routinesFixture struct {
	store    *routinetest.Store
	routines *Routines
	now      time.Time
	a, b     core.User
}

func newRoutinesFixture(t *testing.T) *routinesFixture {
	t.Helper()
	f := &routinesFixture{
		store: routinetest.New(),
		now:   time.Date(2026, 11, 23, 9, 15, 0, 0, time.UTC),
		a:     core.User{ID: bson.NewObjectID().Hex(), Username: "tester01"},
		b:     core.User{ID: bson.NewObjectID().Hex(), Username: "tester02"},
	}
	f.routines = NewRoutines(f.store, func() string { return bson.NewObjectID().Hex() }, func() time.Time { return f.now })
	return f
}

func body(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

const vitaminD = `{"name": "Vitamin D", "amount": 2, "unit": "Stück",
	"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-18"}`

const bloodPressure = `{"name": "Blutdruck messen",
	"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]}, "startDate": "2026-11-18", "endDate": "2026-12-01"}`

func (f *routinesFixture) create(t *testing.T, user core.User, b string) core.RoutineView {
	t.Helper()
	v, err := f.routines.Create(context.Background(), user, body(t, b))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return v
}

// usersOf checks that every store call since mark carried the user.
func (f *routinesFixture) checkUser(t *testing.T, mark int, user core.User) {
	t.Helper()
	calls := f.store.Calls(mark)
	if len(calls) == 0 {
		t.Fatal("no store call")
	}
	for _, c := range calls {
		if c.UserID != user.ID {
			t.Errorf("%s called with user %q, want %q", c.Method, c.UserID, user.ID)
		}
	}
}

func methods(calls []routinetest.Call) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.Method)
	}
	return out
}

func TestRoutines_Create(t *testing.T) {
	f := newRoutinesFixture(t)
	mark := f.store.Mark()
	// The example of API.md 3.18, created on Monday 2026-11-23 at 10:15.
	v := f.create(t, f.a, `{"name": "Wasser", "amount": 250, "unit": "ml",
		"schedule": {"frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00"}, "startDate": "2026-11-23"}`)
	f.checkUser(t, mark, f.a)

	if v.ID == "" || v.UserID != f.a.ID || !v.CreatedAt.Equal(f.now) || v.Status != core.RoutineActive || v.Pauses == nil {
		t.Errorf("routine = %+v", v)
	}
	if v.Progress.Today.Due != 13 || v.Progress.Week.Due != 91 || *v.Progress.Today.AmountTarget != 3250 || v.Progress.Period != nil {
		t.Errorf("progress = %+v", v.Progress)
	}
	stored, ok := f.store.Routine(v.ID)
	if !ok || stored.UserID != f.a.ID || stored.Name != "Wasser" {
		t.Errorf("stored = %+v, %v", stored, ok)
	}
}

func TestRoutines_CreateErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid body: nothing counted or stored", func(t *testing.T) {
		f := newRoutinesFixture(t)
		_, err := f.routines.Create(ctx, f.a, body(t, `{"name": ""}`))
		if !errors.Is(err, core.ErrInvalidRoutine) {
			t.Errorf("err = %v", err)
		}
		if n := f.store.Mark(); n != 0 {
			t.Errorf("%d store calls", n)
		}
	})

	t.Run("limit of 50", func(t *testing.T) {
		f := newRoutinesFixture(t)
		for i := range core.MaxRoutinesPerUser {
			f.create(t, f.a, fmt.Sprintf(`{"name": "Routine %d", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-23"}`, i))
		}
		_, err := f.routines.Create(ctx, f.a, body(t, vitaminD))
		var limit *core.RoutineLimitError
		if !errors.As(err, &limit) || limit.Limit != core.RoutineLimitRoutines || limit.Max != 50 {
			t.Errorf("51st routine: %v", err)
		}
		// The limit is per user.
		f.create(t, f.b, vitaminD)
	})

	t.Run("name taken, ignoring case", func(t *testing.T) {
		f := newRoutinesFixture(t)
		first := f.create(t, f.a, vitaminD)
		_, err := f.routines.Create(ctx, f.a, body(t, `{"name": "VITAMIN d", "schedule": {"frequency": "daily", "interval": 1, "times": ["09:00"]}, "startDate": "2026-11-23"}`))
		var taken *core.RoutineNameTakenError
		if !errors.As(err, &taken) || taken.RoutineID != first.ID {
			t.Errorf("err = %v, want RoutineNameTakenError with %s", err, first.ID)
		}
		// Another user may use the name.
		f.create(t, f.b, vitaminD)
	})

	t.Run("no user", func(t *testing.T) {
		f := newRoutinesFixture(t)
		if _, err := f.routines.Create(ctx, core.User{}, body(t, vitaminD)); !errors.Is(err, core.ErrNoUser) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRoutines_Get(t *testing.T) {
	f := newRoutinesFixture(t)
	ctx := context.Background()
	v := f.create(t, f.a, bloodPressure)

	got, err := f.routines.Get(ctx, f.a, v.ID)
	if err != nil || got.ID != v.ID || got.Progress.Period == nil || got.Progress.Period.Day != 6 {
		t.Errorf("Get = %+v, %v", got, err)
	}
	mark := f.store.Mark()
	if _, err := f.routines.Get(ctx, f.a, "abc"); !errors.Is(err, core.ErrInvalidID) {
		t.Errorf("invalid ID: %v", err)
	}
	if f.store.Mark() != mark {
		t.Error("the store was asked for an invalid ID")
	}
	if _, err := f.routines.Get(ctx, f.a, bson.NewObjectID().Hex()); !errors.Is(err, core.ErrRoutineNotFound) {
		t.Errorf("unknown: %v", err)
	}
	mark = f.store.Mark()
	if _, err := f.routines.Get(ctx, f.b, v.ID); !errors.Is(err, core.ErrRoutineNotFound) {
		t.Errorf("B on A's routine: %v", err)
	}
	f.checkUser(t, mark, f.b)
}

func TestRoutines_List(t *testing.T) {
	f := newRoutinesFixture(t)
	ctx := context.Background()
	first := f.create(t, f.a, bloodPressure)
	f.now = f.now.Add(time.Minute)
	second := f.create(t, f.a, vitaminD)
	// A log today at 08:00 for the blood pressure.
	if _, err := f.routines.AddLog(ctx, f.a, first.ID, body(t, `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "manual"}`)); err != nil {
		t.Fatal(err)
	}

	list, err := f.routines.List(ctx, f.a, "")
	if err != nil || len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("List = %v, %v", list, err)
	}
	if list[0].Progress.Today.Done != 1 || list[0].Progress.Period.Done != 1 || list[1].Progress.Today.Done != 0 {
		t.Errorf("progress: %+v / %+v", list[0].Progress, list[1].Progress)
	}
	if list, err := f.routines.List(ctx, f.b, "health"); err != nil || len(list) != 0 || list == nil {
		t.Errorf("B's list = %v, %v", list, err)
	}
	var pe *core.ParameterError
	if _, err := f.routines.List(ctx, f.a, "sport"); !errors.As(err, &pe) || pe.Parameter != "module" {
		t.Errorf("unknown module: %v", err)
	}
}

func TestRoutines_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("only changed fields", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		got, err := f.routines.Update(ctx, f.a, v.ID, body(t, `{"endDate": "2026-12-07", "name": "Blutdruck messen"}`))
		if err != nil || got.EndDate.String() != "2026-12-07" || got.Progress.Period.Days != 20 {
			t.Fatalf("Update = %+v, %v", got, err)
		}
		if stored, _ := f.store.Routine(v.ID); stored.EndDate.String() != "2026-12-07" {
			t.Errorf("stored endDate %v", stored.EndDate)
		}
		// Only values that already apply: nothing is written.
		mark := f.store.Mark()
		if _, err := f.routines.Update(ctx, f.a, v.ID, body(t, `{"endDate": "2026-12-07"}`)); err != nil {
			t.Fatal(err)
		}
		if slices.Contains(methods(f.store.Calls(mark)), "UpdateRoutine") {
			t.Error("UpdateRoutine called without a change")
		}
	})

	t.Run("name taken", func(t *testing.T) {
		f := newRoutinesFixture(t)
		other := f.create(t, f.a, vitaminD)
		v := f.create(t, f.a, bloodPressure)
		_, err := f.routines.Update(ctx, f.a, v.ID, body(t, `{"name": "vitamin d"}`))
		var taken *core.RoutineNameTakenError
		if !errors.As(err, &taken) || taken.RoutineID != other.ID {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("B changes nothing of A", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		mark := f.store.Mark()
		if _, err := f.routines.Update(ctx, f.b, v.ID, body(t, `{"name": "Von B"}`)); !errors.Is(err, core.ErrRoutineNotFound) {
			t.Errorf("err = %v", err)
		}
		f.checkUser(t, mark, f.b)
		if stored, _ := f.store.Routine(v.ID); stored.Name != "Blutdruck messen" {
			t.Errorf("A's routine changed: %q", stored.Name)
		}
	})

	t.Run("invalid body after the routine is found", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		if _, err := f.routines.Update(ctx, f.a, v.ID, body(t, `{}`)); !errors.Is(err, core.ErrInvalidRoutine) {
			t.Errorf("err = %v", err)
		}
		if _, err := f.routines.Update(ctx, f.b, v.ID, body(t, `{}`)); !errors.Is(err, core.ErrRoutineNotFound) {
			t.Errorf("B with an invalid body: %v, want 404 first", err)
		}
	})
}

func TestRoutines_PauseAndResume(t *testing.T) {
	ctx := context.Background()
	f := newRoutinesFixture(t)
	v := f.create(t, f.a, bloodPressure)

	got, err := f.routines.Pause(ctx, f.a, v.ID, map[string]any{})
	if err != nil || got.Status != core.RoutinePaused || len(got.Pauses) != 1 || got.Pauses[0].From.String() != "2026-11-23" {
		t.Fatalf("Pause = %+v, %v", got, err)
	}
	// Today's occurrences are paused: due 0.
	if got.Progress.Today.Due != 0 {
		t.Errorf("today due %d", got.Progress.Today.Due)
	}
	// Sending it again writes nothing.
	mark := f.store.Mark()
	if _, err := f.routines.Pause(ctx, f.a, v.ID, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(methods(f.store.Calls(mark)), "SetPauses") {
		t.Error("SetPauses called for the same request")
	}
	// until before today.
	if _, err := f.routines.Pause(ctx, f.a, v.ID, body(t, `{"until": "2026-11-22"}`)); !errors.Is(err, core.ErrInvalidRoutine) {
		t.Errorf("until before today: %v", err)
	}

	got, err = f.routines.Resume(ctx, f.a, v.ID)
	if err != nil || got.Status != core.RoutineActive || len(got.Pauses) != 0 || got.Progress.Today.Due != 2 {
		t.Errorf("Resume = %+v, %v", got, err)
	}
	if _, err := f.routines.Resume(ctx, f.b, v.ID); !errors.Is(err, core.ErrRoutineNotFound) {
		t.Errorf("B resumes A's routine: %v", err)
	}
}

// Another request changes the pauses between reading and writing: the
// service reads again and retries, at most 3 times.
func TestRoutines_PauseRetries(t *testing.T) {
	ctx := context.Background()

	t.Run("succeeds on the second attempt", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		interfered := false
		f.store.BeforeSetPauses = func() {
			if interfered {
				return
			}
			interfered = true
			r, _ := f.store.Routine(v.ID)
			until := core.DateFromYMD(2026, 11, 8)
			r.Pauses = []core.Pause{{From: core.DateFromYMD(2026, 11, 2), Until: &until}}
			f.store.Put(r)
		}
		mark := f.store.Mark()
		got, err := f.routines.Pause(ctx, f.a, v.ID, map[string]any{})
		if err != nil || len(got.Pauses) != 2 || got.Pauses[0].From.String() != "2026-11-02" {
			t.Fatalf("Pause = %+v, %v", got.Pauses, err)
		}
		if n := countOf(methods(f.store.Calls(mark)), "SetPauses"); n != 2 {
			t.Errorf("SetPauses called %d times, want 2", n)
		}
	})

	t.Run("gives up after 3", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		day := 1
		f.store.BeforeSetPauses = func() {
			r, _ := f.store.Routine(v.ID)
			d := core.DateFromYMD(2026, 10, day)
			r.Pauses = append(r.Pauses, core.Pause{From: d, Until: &d})
			f.store.Put(r)
			day++
		}
		mark := f.store.Mark()
		if _, err := f.routines.Pause(ctx, f.a, v.ID, map[string]any{}); err == nil {
			t.Fatal("no error after 3 conflicts")
		}
		if n := countOf(methods(f.store.Calls(mark)), "SetPauses"); n != 3 {
			t.Errorf("SetPauses called %d times, want 3", n)
		}
	})
}

func countOf(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

func TestRoutines_Delete(t *testing.T) {
	ctx := context.Background()
	logBody := `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "manual"}`

	t.Run("logs, routine, logs again", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		l, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, logBody))
		if err != nil {
			t.Fatal(err)
		}
		mark := f.store.Mark()
		if err := f.routines.Delete(ctx, f.a, v.ID); err != nil {
			t.Fatal(err)
		}
		want := []string{"GetRoutine", "DeleteRoutineLogs", "DeleteRoutine", "DeleteRoutineLogs"}
		if got := methods(f.store.Calls(mark)); !slices.Equal(got, want) {
			t.Errorf("calls = %v, want %v", got, want)
		}
		if _, ok := f.store.Routine(v.ID); ok {
			t.Error("routine left")
		}
		if _, ok := f.store.Log(l.ID); ok {
			t.Error("log left")
		}
		if err := f.routines.Delete(ctx, f.a, v.ID); !errors.Is(err, core.ErrRoutineNotFound) {
			t.Errorf("second delete: %v", err)
		}
	})

	t.Run("failing logs keep the routine", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		f.store.Fail["DeleteRoutineLogs"] = errors.New("database unreachable")
		if err := f.routines.Delete(ctx, f.a, v.ID); err == nil {
			t.Fatal("no error")
		}
		if _, ok := f.store.Routine(v.ID); !ok {
			t.Error("routine deleted although its logs were not")
		}
	})

	t.Run("B deletes nothing of A", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, bloodPressure)
		l, _ := f.routines.AddLog(ctx, f.a, v.ID, body(t, logBody))
		mark := f.store.Mark()
		if err := f.routines.Delete(ctx, f.b, v.ID); !errors.Is(err, core.ErrRoutineNotFound) {
			t.Errorf("err = %v", err)
		}
		f.checkUser(t, mark, f.b)
		if _, ok := f.store.Routine(v.ID); !ok {
			t.Error("A's routine deleted by B")
		}
		if _, ok := f.store.Log(l.ID); !ok {
			t.Error("A's log deleted by B")
		}
	})
}

func TestRoutines_AddLog(t *testing.T) {
	ctx := context.Background()
	logBody := `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "notification"}`

	t.Run("stored with owner and time zone", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, vitaminD)
		mark := f.store.Mark()
		l, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, logBody))
		if err != nil {
			t.Fatal(err)
		}
		f.checkUser(t, mark, f.a)
		stored, ok := f.store.Log(l.ID)
		if !ok || stored.UserID != f.a.ID || stored.RoutineID != v.ID || stored.TimeZone != "Europe/Vienna" ||
			stored.DueLocal != "2026-11-23T08:00" || *stored.Amount != 2 || !stored.TakenAt.Equal(f.now) {
			t.Errorf("stored log = %+v", stored)
		}
	})

	t.Run("in the user's time zone", func(t *testing.T) {
		f := newRoutinesFixture(t)
		f.a.TimeZone = "Europe/Lisbon"
		v := f.create(t, f.a, vitaminD)
		// 08:00 in Lisbon is 08:00 UTC; 07:00 UTC is no occurrence there.
		if _, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, logBody)); !errors.Is(err, core.ErrInvalidRoutine) {
			t.Errorf("Vienna's 08:00 in Lisbon: %v", err)
		}
		l, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"dueAt": "2026-11-23T08:00:00Z", "status": "done", "source": "manual"}`))
		if err != nil || l.TimeZone != "Europe/Lisbon" || l.DueLocal != "2026-11-23T08:00" {
			t.Errorf("log = %+v, %v", l, err)
		}
	})

	t.Run("second log for the occurrence", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, vitaminD)
		first, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, logBody))
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"dueAt": "2026-11-23T07:00:00Z", "status": "skipped", "source": "manual"}`))
		var exists *core.RoutineLogExistsError
		if !errors.As(err, &exists) || exists.LogID != first.ID {
			t.Errorf("err = %v, want RoutineLogExistsError with %s", err, first.ID)
		}
	})

	t.Run("100 logs per day", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, vitaminD)
		for i := range core.MaxLogsPerRoutineDay {
			f.store.PutLog(core.RoutineLog{ID: bson.NewObjectID().Hex(), UserID: f.a.ID, RoutineID: v.ID,
				TakenAt: f.now.Add(-time.Duration(i) * time.Minute), Status: core.LogDone, Source: core.LogSourceManual})
		}
		_, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"status": "done", "source": "manual"}`))
		var limit *core.RoutineLimitError
		if !errors.As(err, &limit) || limit.Limit != core.RoutineLimitLogs || limit.Max != 100 {
			t.Errorf("101st log: %v", err)
		}
		// Yesterday (in Vienna) is another day.
		if _, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"status": "done", "source": "manual", "takenAt": "2026-11-22T20:00:00Z"}`)); err != nil {
			t.Errorf("log for yesterday: %v", err)
		}
	})

	t.Run("B logs nothing on A's routine", func(t *testing.T) {
		f := newRoutinesFixture(t)
		v := f.create(t, f.a, vitaminD)
		if _, err := f.routines.AddLog(ctx, f.b, v.ID, body(t, logBody)); !errors.Is(err, core.ErrRoutineNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRoutines_DeleteLog(t *testing.T) {
	ctx := context.Background()
	f := newRoutinesFixture(t)
	v := f.create(t, f.a, bloodPressure)
	other := f.create(t, f.a, vitaminD)
	l, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "manual"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.routines.DeleteLog(ctx, f.a, v.ID, "abc"); !errors.Is(err, core.ErrInvalidID) {
		t.Errorf("invalid log ID: %v", err)
	}
	if err := f.routines.DeleteLog(ctx, f.a, other.ID, l.ID); !errors.Is(err, core.ErrRoutineLogNotFound) {
		t.Errorf("through another routine: %v", err)
	}
	if err := f.routines.DeleteLog(ctx, f.b, v.ID, l.ID); !errors.Is(err, core.ErrRoutineNotFound) {
		t.Errorf("B: %v", err)
	}
	if err := f.routines.DeleteLog(ctx, f.a, v.ID, l.ID); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := f.routines.DeleteLog(ctx, f.a, v.ID, l.ID); !errors.Is(err, core.ErrRoutineLogNotFound) {
		t.Errorf("second delete: %v", err)
	}
}

func TestRoutines_Occurrences(t *testing.T) {
	ctx := context.Background()
	f := newRoutinesFixture(t)
	bp := f.create(t, f.a, bloodPressure)
	vd := f.create(t, f.a, vitaminD)
	l, err := f.routines.AddLog(ctx, f.a, vd.ID, body(t, `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "manual"}`))
	if err != nil {
		t.Fatal(err)
	}

	mark := f.store.Mark()
	from, to, list, err := f.routines.Occurrences(ctx, f.a, "", "2026-11-23", "2026-11-23")
	if err != nil {
		t.Fatal(err)
	}
	f.checkUser(t, mark, f.a)
	if from.String() != "2026-11-23" || to.String() != "2026-11-23" || len(list) != 3 {
		t.Fatalf("%s..%s: %+v", from, to, list)
	}
	// Both 08:00 first, by routine ID; then 20:00.
	first, second := bp.ID, vd.ID
	if second < first {
		first, second = second, first
	}
	if list[0].RoutineID != first || list[1].RoutineID != second || list[2].RoutineID != bp.ID || list[2].Time.String() != "20:00" {
		t.Errorf("order: %+v", list)
	}
	for _, o := range list {
		if o.RoutineID == vd.ID && (o.Status != core.OccurrenceDone || o.LogID != l.ID) {
			t.Errorf("Vitamin D: %+v", o)
		}
		if o.RoutineID == bp.ID && o.Status != core.OccurrenceOpen {
			t.Errorf("Blutdruck: %+v", o)
		}
	}

	// Defaults: today and the next 6 days.
	from, to, list, err = f.routines.Occurrences(ctx, f.a, "health", "", "")
	if err != nil || from.String() != "2026-11-23" || to.String() != "2026-11-29" || len(list) != 7*2+7 {
		t.Errorf("defaults: %s..%s, %d, %v", from, to, len(list), err)
	}
	// B sees nothing.
	if _, _, list, err := f.routines.Occurrences(ctx, f.b, "", "", ""); err != nil || len(list) != 0 || list == nil {
		t.Errorf("B: %v, %v", list, err)
	}
	var pe *core.ParameterError
	if _, _, _, err := f.routines.Occurrences(ctx, f.a, "", "2026-11-01", "2026-12-15"); !errors.As(err, &pe) || pe.Parameter != "to" {
		t.Errorf("too long: %v", err)
	}
	if _, _, _, err := f.routines.Occurrences(ctx, f.a, "x", "y", ""); !errors.As(err, &pe) || pe.Parameter != "module" {
		t.Errorf("module first: %v", err)
	}
}

func TestRoutines_Logs(t *testing.T) {
	ctx := context.Background()
	f := newRoutinesFixture(t)
	v := f.create(t, f.a, bloodPressure)
	early, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"dueAt": "2026-11-22T07:00:00Z", "status": "done", "source": "manual", "takenAt": "2026-11-22T07:03:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	late, err := f.routines.AddLog(ctx, f.a, v.ID, body(t, `{"dueAt": "2026-11-23T07:00:00Z", "status": "skipped", "source": "manual"}`))
	if err != nil {
		t.Fatal(err)
	}
	list, err := f.routines.Logs(ctx, f.a, v.ID, "", "")
	if err != nil || len(list) != 2 || list[0].ID != late.ID || list[1].ID != early.ID {
		t.Errorf("Logs = %v, %v; want newest first", list, err)
	}
	if list, err := f.routines.Logs(ctx, f.a, v.ID, "2026-11-23", "2026-11-23"); err != nil || len(list) != 1 || list[0].ID != late.ID {
		t.Errorf("one day: %v, %v", list, err)
	}
	var pe *core.ParameterError
	if _, err := f.routines.Logs(ctx, f.a, v.ID, "2025-01-01", ""); !errors.As(err, &pe) {
		t.Errorf("more than 366 days: %v", err)
	}
	if _, err := f.routines.Logs(ctx, f.b, v.ID, "", ""); !errors.Is(err, core.ErrRoutineNotFound) {
		t.Errorf("B: %v", err)
	}
}
