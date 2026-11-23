package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"lifelog/backend/internal/core"
)

// RoutineStore is what the routine endpoints need from the database.
// Implemented by store.Store. Every method filters by the user;
// CreateRoutine and UpdateRoutine report a taken name as
// core.ErrRoutineNameTaken, CreateRoutineLog a second log for an occurrence
// as core.ErrRoutineLogExists.
type RoutineStore interface {
	CreateRoutine(ctx context.Context, userID string, r core.Routine) error
	ListRoutines(ctx context.Context, userID, module string) ([]core.Routine, error)
	GetRoutine(ctx context.Context, userID, id string) (r core.Routine, found bool, err error)
	CountRoutines(ctx context.Context, userID string) (int64, error)
	RoutineIDByName(ctx context.Context, userID, module, nameKey string) (id string, found bool, err error)
	UpdateRoutine(ctx context.Context, userID string, r core.Routine, fields []string) (found bool, err error)
	SetPauses(ctx context.Context, userID, id string, old, pauses []core.Pause) (updated bool, err error)
	DeleteRoutine(ctx context.Context, userID, id string) (found bool, err error)
	CreateRoutineLog(ctx context.Context, userID string, l core.RoutineLog) error
	RoutineLogIDByDue(ctx context.Context, userID, routineID, dueLocal string) (id string, found bool, err error)
	CountRoutineLogsTaken(ctx context.Context, userID, routineID string, from, to time.Time) (int64, error)
	RoutineLogs(ctx context.Context, userID, routineID string, from, to time.Time) ([]core.RoutineLog, error)
	RoutineLogsInRange(ctx context.Context, userID string, routineIDs []string, from, to core.Date, takenFrom, takenTo time.Time) ([]core.RoutineLog, error)
	DeleteRoutineLog(ctx context.Context, userID, routineID, logID string) (found bool, err error)
	DeleteRoutineLogs(ctx context.Context, userID, routineID string) (int64, error)
}

// routineDeleteTimeout: time for the three steps of deleting a routine.
// They keep running without the request's context (see Delete).
const routineDeleteTimeout = 30 * time.Second

// pauseAttempts: how often Pause and Resume read, change and write the
// pauses when another request changed them at the same moment.
const pauseAttempts = 3

// Routines implements the routine endpoints (docs/API.md 3.17–3.27). The
// rules are in core; here are the database steps, the limits that need
// counting, and the order of the steps.
//
// "Now" is always the server clock (now), "today" its date in the user's
// time zone (core.UserTimeZone).
//
// Logging: only IDs, counts and field names, never a name, amount, unit or
// point in time of a routine or log; they are health data.
//
// Order of the checks for a routine ID in the path: the ID (400
// INVALID_ID), the routine of the user (404), then the body. A routine of
// another user is 404 whatever the body contains.
type Routines struct {
	store RoutineStore
	newID func() string
	now   func() time.Time
}

// NewRoutines: newID creates the IDs (store.NewID), now is the server clock
// (time.Now; fixed in tests).
func NewRoutines(s RoutineStore, newID func() string, now func() time.Time) *Routines {
	return &Routines{store: s, newID: newID, now: now}
}

// List returns the user's routines of the module with status and progress
// (API.md 3.17). Errors: *core.ParameterError (module), core.ErrNoUser or
// an internal error.
func (s *Routines) List(ctx context.Context, user core.User, module string) ([]core.RoutineView, error) {
	if user.ID == "" {
		return nil, core.ErrNoUser
	}
	m, err := core.CheckRoutineModule(module)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListRoutines(ctx, user.ID, m)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, user, list)
}

// Create stores a new routine (API.md 3.18): check the body (core), the
// limit of 50 routines, then insert; the unique index decides whether the
// name is taken.
//
// Errors: *core.RoutineError, *core.RoutineLimitError,
// *core.RoutineNameTakenError, core.ErrNoUser or an internal error.
func (s *Routines) Create(ctx context.Context, user core.User, body map[string]any) (core.RoutineView, error) {
	if user.ID == "" {
		return core.RoutineView{}, core.ErrNoUser
	}
	r, err := core.ParseNewRoutine(body)
	if err != nil {
		return core.RoutineView{}, err
	}
	// Counting and inserting are two steps: two requests at the same moment
	// can make it 51. That is acceptable for a limit that protects the
	// database; names and logs are secured by unique indexes instead.
	n, err := s.store.CountRoutines(ctx, user.ID)
	if err != nil {
		return core.RoutineView{}, err
	}
	if n >= core.MaxRoutinesPerUser {
		return core.RoutineView{}, &core.RoutineLimitError{Limit: core.RoutineLimitRoutines, Max: core.MaxRoutinesPerUser}
	}
	r.ID, r.UserID = s.newID(), user.ID
	r.CreatedAt = s.now().UTC().Truncate(time.Millisecond)
	if err := s.store.CreateRoutine(ctx, user.ID, r); err != nil {
		if errors.Is(err, core.ErrRoutineNameTaken) {
			return core.RoutineView{}, s.nameTaken(ctx, user.ID, r.Module, r.Name)
		}
		return core.RoutineView{}, err
	}
	slog.Info("routine created", "routine", r.ID, "user", user.ID)
	return s.view(ctx, user, r)
}

// nameTaken looks up the routine that has the name, for details.routineId.
func (s *Routines) nameTaken(ctx context.Context, userID, module, name string) error {
	id, _, err := s.store.RoutineIDByName(ctx, userID, module, core.RoutineNameKey(name))
	if err != nil {
		return err
	}
	// If it was deleted in between, the ID is empty; the API leaves it out.
	return &core.RoutineNameTakenError{RoutineID: id}
}

// load checks the ID and loads the user's routine.
func (s *Routines) load(ctx context.Context, user core.User, id string) (core.Routine, error) {
	if user.ID == "" {
		return core.Routine{}, core.ErrNoUser
	}
	if err := core.CheckID(id); err != nil {
		return core.Routine{}, err
	}
	r, found, err := s.store.GetRoutine(ctx, user.ID, id)
	if err != nil {
		return core.Routine{}, err
	}
	if !found {
		return core.Routine{}, core.ErrRoutineNotFound
	}
	return r, nil
}

// Get returns one routine (API.md 3.19). Errors: core.ErrInvalidID,
// core.ErrRoutineNotFound (also for a routine of another user),
// core.ErrNoUser or an internal error.
func (s *Routines) Get(ctx context.Context, user core.User, id string) (core.RoutineView, error) {
	r, err := s.load(ctx, user, id)
	if err != nil {
		return core.RoutineView{}, err
	}
	return s.view(ctx, user, r)
}

// Update changes a routine (API.md 3.20). Only the fields whose value
// changes are written ($set); a request that only repeats values writes
// nothing. Two simultaneous changes of the same field: the last one wins.
//
// Errors: as Get, plus *core.RoutineError and *core.RoutineNameTakenError.
func (s *Routines) Update(ctx context.Context, user core.User, id string, body map[string]any) (core.RoutineView, error) {
	r, err := s.load(ctx, user, id)
	if err != nil {
		return core.RoutineView{}, err
	}
	n, changed, err := core.ApplyRoutinePatch(r, body)
	if err != nil {
		return core.RoutineView{}, err
	}
	if len(changed) > 0 {
		found, err := s.store.UpdateRoutine(ctx, user.ID, n, changed)
		if errors.Is(err, core.ErrRoutineNameTaken) {
			return core.RoutineView{}, s.nameTaken(ctx, user.ID, n.Module, n.Name)
		}
		if err != nil {
			return core.RoutineView{}, err
		}
		if !found {
			return core.RoutineView{}, core.ErrRoutineNotFound // deleted meanwhile
		}
		slog.Info("routine changed", "routine", id, "user", user.ID, "fields", changed)
	}
	return s.view(ctx, user, n)
}

// Pause pauses a routine from today (API.md 3.21). Body: {"until": …} or
// empty ({}).
//
// Read, change, write with a condition on the old pauses (compare and
// set): if another request changed them at the same moment, read again,
// up to 3 times. Sending the same request twice writes nothing the second
// time.
//
// Errors: as Get, plus *core.RoutineError and *core.RoutineLimitError.
func (s *Routines) Pause(ctx context.Context, user core.User, id string, body map[string]any) (core.RoutineView, error) {
	r, err := s.load(ctx, user, id)
	if err != nil {
		return core.RoutineView{}, err
	}
	until, err := core.ParsePauseRequest(body)
	if err != nil {
		return core.RoutineView{}, err
	}
	return s.changePauses(ctx, user, r, "routine paused", func(pauses []core.Pause, today core.Date) ([]core.Pause, bool, error) {
		return core.ApplyPause(pauses, today, until)
	})
}

// Resume resumes a routine today (API.md 3.22). Errors: as Get.
func (s *Routines) Resume(ctx context.Context, user core.User, id string) (core.RoutineView, error) {
	r, err := s.load(ctx, user, id)
	if err != nil {
		return core.RoutineView{}, err
	}
	return s.changePauses(ctx, user, r, "routine resumed", func(pauses []core.Pause, today core.Date) ([]core.Pause, bool, error) {
		p, changed := core.ApplyResume(pauses, today)
		return p, changed, nil
	})
}

func (s *Routines) changePauses(ctx context.Context, user core.User, r core.Routine, logMessage string,
	apply func([]core.Pause, core.Date) ([]core.Pause, bool, error)) (core.RoutineView, error) {
	_, loc := core.UserTimeZone(user)
	for attempt := 1; ; attempt++ {
		pauses, changed, err := apply(r.Pauses, core.DateOf(s.now(), loc))
		if err != nil {
			return core.RoutineView{}, err
		}
		if !changed {
			return s.view(ctx, user, r)
		}
		updated, err := s.store.SetPauses(ctx, user.ID, r.ID, r.Pauses, pauses)
		if err != nil {
			return core.RoutineView{}, err
		}
		if updated {
			r.Pauses = pauses
			slog.Info(logMessage, "routine", r.ID, "user", user.ID, "pauses", len(pauses))
			return s.view(ctx, user, r)
		}
		if attempt == pauseAttempts {
			return core.RoutineView{}, fmt.Errorf("pauses of routine %s changed concurrently %d times", r.ID, pauseAttempts)
		}
		// Changed or deleted meanwhile: read again.
		if r, err = s.load(ctx, user, r.ID); err != nil {
			return core.RoutineView{}, err
		}
	}
}

// Delete deletes a routine with its logs (API.md 3.23):
//
//  1. Check the ID, load the user's routine (otherwise 404).
//  2. Delete its logs.
//  3. Delete the routine.
//  4. Delete its logs once more: a log added while step 2 and 3 ran would
//     otherwise stay without its routine.
//
// The routine goes last: if a step fails, the answer is 500 and the
// routine stays visible; deleting again cleans up the rest. From step 2
// on, deleting runs to completion even if the app drops the connection.
//
// Errors: as Get.
func (s *Routines) Delete(ctx context.Context, user core.User, id string) error {
	if _, err := s.load(ctx, user, id); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), routineDeleteTimeout)
	defer cancel()

	logs, err := s.store.DeleteRoutineLogs(ctx, user.ID, id)
	if err != nil {
		return err
	}
	found, err := s.store.DeleteRoutine(ctx, user.ID, id)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrRoutineNotFound // a simultaneous call was faster
	}
	late, err := s.store.DeleteRoutineLogs(ctx, user.ID, id)
	if err != nil {
		return err
	}
	slog.Info("routine deleted", "routine", id, "user", user.ID, "logs", logs+late)
	return nil
}

// Occurrences returns the occurrences of all the user's routines of the
// module on the days from..to (API.md 3.24), sorted by dueAt, then
// routine ID, with the range actually used (defaults filled in).
//
// Errors: *core.ParameterError (module, from, to), core.ErrNoUser or an
// internal error.
func (s *Routines) Occurrences(ctx context.Context, user core.User, module, from, to string) (core.Date, core.Date, []core.Occurrence, error) {
	if user.ID == "" {
		return 0, 0, nil, core.ErrNoUser
	}
	m, err := core.CheckRoutineModule(module)
	if err != nil {
		return 0, 0, nil, err
	}
	_, loc := core.UserTimeZone(user)
	f, t, err := core.OccurrenceRange(from, to, core.DateOf(s.now(), loc))
	if err != nil {
		return 0, 0, nil, err
	}
	routines, err := s.store.ListRoutines(ctx, user.ID, m)
	if err != nil {
		return 0, 0, nil, err
	}
	ids := make([]string, 0, len(routines))
	for _, r := range routines {
		ids = append(ids, r.ID)
	}
	// Only logs with an occurrence on these days matter: an empty range of
	// takenAt.
	start := core.DayStart(f, loc)
	logs, err := s.store.RoutineLogsInRange(ctx, user.ID, ids, f, t, start, start)
	if err != nil {
		return 0, 0, nil, err
	}
	byRoutine := groupLogs(logs)
	list := []core.Occurrence{}
	for _, r := range routines {
		list = append(list, core.RoutineOccurrences(r, byRoutine[r.ID], f, t, loc)...)
	}
	slices.SortStableFunc(list, func(a, b core.Occurrence) int {
		return cmp.Or(a.DueAt.Compare(b.DueAt), cmp.Compare(a.RoutineID, b.RoutineID))
	})
	return f, t, list, nil
}

// Logs returns the logs of a routine whose takenAt lies on the local days
// from..to, newest first (API.md 3.25).
//
// Errors: as Get, plus *core.ParameterError (from, to).
func (s *Routines) Logs(ctx context.Context, user core.User, id, from, to string) ([]core.RoutineLog, error) {
	if _, err := s.load(ctx, user, id); err != nil {
		return nil, err
	}
	_, loc := core.UserTimeZone(user)
	f, t, err := core.LogRange(from, to, core.DateOf(s.now(), loc))
	if err != nil {
		return nil, err
	}
	return s.store.RoutineLogs(ctx, user.ID, id, core.DayStart(f, loc), core.DayStart(t+1, loc))
}

// AddLog adds a log (API.md 3.26):
//
//  1. Check the ID, load the user's routine.
//  2. Check the body against the routine (core.CheckNewLog).
//  3. The occurrence already has a log: *core.RoutineLogExistsError with
//     its ID.
//  4. At most 100 logs per routine on the local day of takenAt. Counting
//     and inserting are two steps; two requests at the same moment can
//     make it 101.
//  5. Store it with the owner and the user's time zone. The unique index
//     on the occurrence also catches two requests at the same moment.
//
// Errors: as Get, plus *core.RoutineError, *core.RoutineLogExistsError and
// *core.RoutineLimitError.
func (s *Routines) AddLog(ctx context.Context, user core.User, id string, body map[string]any) (core.RoutineLog, error) {
	r, err := s.load(ctx, user, id)
	if err != nil {
		return core.RoutineLog{}, err
	}
	zone, loc := core.UserTimeZone(user)
	l, err := core.CheckNewLog(r, body, zone, loc, s.now())
	if err != nil {
		return core.RoutineLog{}, err
	}
	if l.DueLocal != "" {
		existing, found, err := s.store.RoutineLogIDByDue(ctx, user.ID, id, l.DueLocal)
		if err != nil {
			return core.RoutineLog{}, err
		}
		if found {
			return core.RoutineLog{}, &core.RoutineLogExistsError{LogID: existing}
		}
	}
	day := core.DateOf(l.TakenAt, loc)
	n, err := s.store.CountRoutineLogsTaken(ctx, user.ID, id, core.DayStart(day, loc), core.DayStart(day+1, loc))
	if err != nil {
		return core.RoutineLog{}, err
	}
	if n >= core.MaxLogsPerRoutineDay {
		return core.RoutineLog{}, &core.RoutineLimitError{Limit: core.RoutineLimitLogs, Max: core.MaxLogsPerRoutineDay}
	}
	l.ID, l.UserID = s.newID(), user.ID
	if err := s.store.CreateRoutineLog(ctx, user.ID, l); err != nil {
		if errors.Is(err, core.ErrRoutineLogExists) {
			existing, _, lookupErr := s.store.RoutineLogIDByDue(ctx, user.ID, id, l.DueLocal)
			if lookupErr != nil {
				return core.RoutineLog{}, lookupErr
			}
			return core.RoutineLog{}, &core.RoutineLogExistsError{LogID: existing}
		}
		return core.RoutineLog{}, err
	}
	slog.Info("routine log added", "routine", id, "log", l.ID, "user", user.ID)
	return l, nil
}

// DeleteLog deletes one log of a routine (API.md 3.27). Errors: as Get,
// plus core.ErrRoutineLogNotFound (also when deleting it a second time, or
// for a log of another routine).
func (s *Routines) DeleteLog(ctx context.Context, user core.User, id, logID string) error {
	if err := core.CheckID(logID); err != nil {
		return err
	}
	if _, err := s.load(ctx, user, id); err != nil {
		return err
	}
	found, err := s.store.DeleteRoutineLog(ctx, user.ID, id, logID)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrRoutineLogNotFound
	}
	slog.Info("routine log deleted", "routine", id, "log", logID, "user", user.ID)
	return nil
}

func (s *Routines) view(ctx context.Context, user core.User, r core.Routine) (core.RoutineView, error) {
	views, err := s.views(ctx, user, []core.Routine{r})
	if err != nil {
		return core.RoutineView{}, err
	}
	return views[0], nil
}

// views adds status and progress. The logs of all routines come in one
// query over the union of their ranges (core.ProgressSpan); logs outside a
// routine's ranges do not count.
func (s *Routines) views(ctx context.Context, user core.User, list []core.Routine) ([]core.RoutineView, error) {
	out := make([]core.RoutineView, 0, len(list))
	if len(list) == 0 {
		return out, nil
	}
	_, loc := core.UserTimeZone(user)
	now := s.now()
	today := core.DateOf(now, loc)
	ids := make([]string, 0, len(list))
	from, to := core.ProgressSpan(list[0], today)
	for _, r := range list {
		f, t := core.ProgressSpan(r, today)
		from, to = min(from, f), max(to, t)
		ids = append(ids, r.ID)
	}
	logs, err := s.store.RoutineLogsInRange(ctx, user.ID, ids, from, to, core.DayStart(from, loc), core.DayStart(to+1, loc))
	if err != nil {
		return nil, err
	}
	byRoutine := groupLogs(logs)
	for _, r := range list {
		out = append(out, core.RoutineView{
			Routine:  r,
			Status:   core.RoutineStatusOn(r, today),
			Progress: core.RoutineProgress(r, byRoutine[r.ID], loc, now),
		})
	}
	return out, nil
}

func groupLogs(logs []core.RoutineLog) map[string][]core.RoutineLog {
	by := map[string][]core.RoutineLog{}
	for _, l := range logs {
		by[l.RoutineID] = append(by[l.RoutineID], l)
	}
	return by
}
