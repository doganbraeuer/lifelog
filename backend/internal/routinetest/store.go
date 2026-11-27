// Package routinetest provides an in-memory store for routines and routine
// logs, for the tests of internal/service and internal/api. The program
// never imports it.
//
// Store behaves like store.Store where the tests depend on it: every method
// only sees the user's own documents, an empty user ID is core.ErrNoUser,
// names are unique per user and module ignoring case, one log per
// occurrence, SetPauses is compare and set. Every call is recorded with its
// user ID, so a test can prove that a route passes the user on.
package routinetest

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"lifelog/backend/internal/core"
)

// Call is one recorded store call.
type Call struct {
	Method string
	UserID string
}

// Store is an in-memory service.RoutineStore.
type Store struct {
	mu       sync.Mutex
	routines map[string]core.Routine    // ID → routine (UserID = owner)
	logs     map[string]core.RoutineLog // ID → log (UserID = owner)
	calls    []Call

	// Fail, if set for a method name, is returned by that method instead
	// of doing anything.
	Fail map[string]error
	// BeforeSetPauses, if set, runs inside SetPauses before the comparison
	// (with the lock released), to simulate another request changing the
	// pauses at the same moment.
	BeforeSetPauses func()
}

func New() *Store {
	return &Store{routines: map[string]core.Routine{}, logs: map[string]core.RoutineLog{}, Fail: map[string]error{}}
}

// begin records the call and reports a configured failure. The caller
// holds the lock afterwards.
func (s *Store) begin(method, userID string) error {
	s.mu.Lock()
	s.calls = append(s.calls, Call{method, userID})
	if userID == "" {
		return core.ErrNoUser
	}
	return s.Fail[method]
}

// Calls returns the calls since mark (see Mark).
func (s *Store) Calls(mark int) []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls[mark:])
}

// Mark returns the number of calls so far.
func (s *Store) Mark() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// Routine returns a stored routine directly, without a user check.
func (s *Store) Routine(id string) (core.Routine, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.routines[id]
	return r, ok
}

// Log returns a stored log directly, without a user check.
func (s *Store) Log(id string) (core.RoutineLog, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.logs[id]
	return l, ok
}

// Put stores a routine directly (test setup).
func (s *Store) Put(r core.Routine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routines[r.ID] = r
}

// PutLog stores a log directly (test setup).
func (s *Store) PutLog(l core.RoutineLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs[l.ID] = l
}

func (s *Store) CreateRoutine(_ context.Context, userID string, r core.Routine) error {
	defer s.mu.Unlock()
	if err := s.begin("CreateRoutine", userID); err != nil {
		return err
	}
	if s.nameTakenLocked(userID, r.Module, r.Name, "") {
		return core.ErrRoutineNameTaken
	}
	r.UserID = userID
	s.routines[r.ID] = r
	return nil
}

func (s *Store) nameTakenLocked(userID, module, name, exceptID string) bool {
	for _, o := range s.routines {
		if o.UserID == userID && o.Module == module && o.ID != exceptID &&
			core.RoutineNameKey(o.Name) == core.RoutineNameKey(name) {
			return true
		}
	}
	return false
}

func (s *Store) ListRoutines(_ context.Context, userID, module string) ([]core.Routine, error) {
	defer s.mu.Unlock()
	if err := s.begin("ListRoutines", userID); err != nil {
		return nil, err
	}
	list := []core.Routine{}
	for _, r := range s.routines {
		if r.UserID == userID && r.Module == module {
			list = append(list, r)
		}
	}
	slices.SortFunc(list, func(a, b core.Routine) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return list, nil
}

func (s *Store) GetRoutine(_ context.Context, userID, id string) (core.Routine, bool, error) {
	defer s.mu.Unlock()
	if err := s.begin("GetRoutine", userID); err != nil {
		return core.Routine{}, false, err
	}
	r, ok := s.routines[id]
	if !ok || r.UserID != userID {
		return core.Routine{}, false, nil
	}
	return r, true, nil
}

func (s *Store) CountRoutines(_ context.Context, userID string) (int64, error) {
	defer s.mu.Unlock()
	if err := s.begin("CountRoutines", userID); err != nil {
		return 0, err
	}
	var n int64
	for _, r := range s.routines {
		if r.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (s *Store) RoutineIDByName(_ context.Context, userID, module, nameKey string) (string, bool, error) {
	defer s.mu.Unlock()
	if err := s.begin("RoutineIDByName", userID); err != nil {
		return "", false, err
	}
	for _, r := range s.routines {
		if r.UserID == userID && r.Module == module && core.RoutineNameKey(r.Name) == nameKey {
			return r.ID, true, nil
		}
	}
	return "", false, nil
}

func (s *Store) UpdateRoutine(_ context.Context, userID string, r core.Routine, fields []string) (bool, error) {
	defer s.mu.Unlock()
	if err := s.begin("UpdateRoutine", userID); err != nil {
		return false, err
	}
	old, ok := s.routines[r.ID]
	if !ok || old.UserID != userID {
		return false, nil
	}
	if slices.Contains(fields, "name") && s.nameTakenLocked(userID, old.Module, r.Name, r.ID) {
		return false, core.ErrRoutineNameTaken
	}
	// Only the named fields, like $set.
	for _, f := range fields {
		switch f {
		case "name":
			old.Name = r.Name
		case "amount":
			old.Amount = r.Amount
		case "unit":
			old.Unit = r.Unit
		case "schedule":
			old.Schedule = r.Schedule
		case "startDate":
			old.StartDate = r.StartDate
		case "endDate":
			old.EndDate = r.EndDate
		}
	}
	s.routines[r.ID] = old
	return true, nil
}

func (s *Store) SetPauses(_ context.Context, userID, id string, old, pauses []core.Pause) (bool, error) {
	if hook := s.BeforeSetPauses; hook != nil {
		hook()
	}
	defer s.mu.Unlock()
	if err := s.begin("SetPauses", userID); err != nil {
		return false, err
	}
	r, ok := s.routines[id]
	if !ok || r.UserID != userID || !reflect.DeepEqual(normalize(r.Pauses), normalize(old)) {
		return false, nil
	}
	r.Pauses = slices.Clone(pauses)
	s.routines[id] = r
	return true, nil
}

func normalize(p []core.Pause) []core.Pause {
	if p == nil {
		return []core.Pause{}
	}
	return p
}

func (s *Store) DeleteRoutine(_ context.Context, userID, id string) (bool, error) {
	defer s.mu.Unlock()
	if err := s.begin("DeleteRoutine", userID); err != nil {
		return false, err
	}
	r, ok := s.routines[id]
	if !ok || r.UserID != userID {
		return false, nil
	}
	delete(s.routines, id)
	return true, nil
}

func (s *Store) CreateRoutineLog(_ context.Context, userID string, l core.RoutineLog) error {
	defer s.mu.Unlock()
	if err := s.begin("CreateRoutineLog", userID); err != nil {
		return err
	}
	if l.DueLocal != "" {
		for _, o := range s.logs {
			if o.UserID == userID && o.RoutineID == l.RoutineID && o.DueLocal == l.DueLocal {
				return core.ErrRoutineLogExists
			}
		}
	}
	l.UserID = userID
	s.logs[l.ID] = l
	return nil
}

func (s *Store) RoutineLogIDByDue(_ context.Context, userID, routineID, dueLocal string) (string, bool, error) {
	defer s.mu.Unlock()
	if err := s.begin("RoutineLogIDByDue", userID); err != nil {
		return "", false, err
	}
	for _, l := range s.logs {
		if l.UserID == userID && l.RoutineID == routineID && l.DueLocal == dueLocal {
			return l.ID, true, nil
		}
	}
	return "", false, nil
}

func (s *Store) CountRoutineLogsTaken(_ context.Context, userID, routineID string, from, to time.Time) (int64, error) {
	defer s.mu.Unlock()
	if err := s.begin("CountRoutineLogsTaken", userID); err != nil {
		return 0, err
	}
	var n int64
	for _, l := range s.logs {
		if l.UserID == userID && l.RoutineID == routineID && !l.TakenAt.Before(from) && l.TakenAt.Before(to) {
			n++
		}
	}
	return n, nil
}

func (s *Store) RoutineLogs(_ context.Context, userID, routineID string, from, to time.Time) ([]core.RoutineLog, error) {
	defer s.mu.Unlock()
	if err := s.begin("RoutineLogs", userID); err != nil {
		return nil, err
	}
	list := []core.RoutineLog{}
	for _, l := range s.logs {
		if l.UserID == userID && l.RoutineID == routineID && !l.TakenAt.Before(from) && l.TakenAt.Before(to) {
			list = append(list, l)
		}
	}
	slices.SortFunc(list, func(a, b core.RoutineLog) int {
		if c := b.TakenAt.Compare(a.TakenAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	return list, nil
}

func (s *Store) RoutineLogsInRange(_ context.Context, userID string, routineIDs []string, from, to core.Date, takenFrom, takenTo time.Time) ([]core.RoutineLog, error) {
	defer s.mu.Unlock()
	if err := s.begin("RoutineLogsInRange", userID); err != nil {
		return nil, err
	}
	list := []core.RoutineLog{}
	lo, hi := from.String(), (to + 1).String()
	for _, l := range s.logs {
		if l.UserID != userID || !slices.Contains(routineIDs, l.RoutineID) {
			continue
		}
		byDue := l.DueLocal != "" && l.DueLocal >= lo && l.DueLocal < hi
		byTaken := !l.TakenAt.Before(takenFrom) && l.TakenAt.Before(takenTo)
		if byDue || byTaken {
			list = append(list, l)
		}
	}
	return list, nil
}

func (s *Store) DeleteRoutineLog(_ context.Context, userID, routineID, logID string) (bool, error) {
	defer s.mu.Unlock()
	if err := s.begin("DeleteRoutineLog", userID); err != nil {
		return false, err
	}
	l, ok := s.logs[logID]
	if !ok || l.UserID != userID || l.RoutineID != routineID {
		return false, nil
	}
	delete(s.logs, logID)
	return true, nil
}

func (s *Store) DeleteRoutineLogs(_ context.Context, userID, routineID string) (int64, error) {
	defer s.mu.Unlock()
	if err := s.begin("DeleteRoutineLogs", userID); err != nil {
		return 0, err
	}
	var n int64
	for id, l := range s.logs {
		if l.UserID == userID && l.RoutineID == routineID {
			delete(s.logs, id)
			n++
		}
	}
	return n, nil
}
