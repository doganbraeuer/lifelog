package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

// Nothing of a routine in the log (API.md 3.16 "Per-user data and
// logging"): no name, unit, amount or point in time of a log, also not
// from a conflict, a pause, a change, a rejected request or an internal
// error. Only method, path, status, durations, IDs, counts and field
// names.
func TestSeparation_NoRoutineContentInTheLog(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	// Without the time of the log line: it could contain "08:00" by chance.
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		}})))
	defer slog.SetDefault(old)

	s := newTwoUsers(t)
	const create = `{"name": "Geheimroutine Blutverdünner", "amount": 123.5, "unit": "Tropfen",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-18"}`
	rec := s.do(jsonRequest(http.MethodPost, "/api/v1/routines", create), testToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var r struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	path := "/api/v1/routines/" + r.ID

	steps := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/api/v1/routines", create, http.StatusConflict}, // name taken
		{http.MethodPost, "/api/v1/routines", `{"name": "Geheimer Name", "schedule": {"frequency": "daily", "interval": 1, "times": ["Geheimzeit"]}, "startDate": "2026-11-18"}`, http.StatusBadRequest},
		{http.MethodPost, path + "/logs", `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "manual", "takenAt": "2026-11-23T06:58:47Z", "amount": 99.75}`, http.StatusCreated},
		{http.MethodPost, path + "/logs", `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "manual"}`, http.StatusConflict},
		{http.MethodPost, path + "/logs", `{"status": "done", "source": "manual", "amount": 999999}`, http.StatusBadRequest},
		{http.MethodPost, path + "/pause", `{"until": "2026-12-24"}`, http.StatusOK},
		{http.MethodPost, path + "/resume", "", http.StatusOK},
		{http.MethodPatch, path, `{"name": "Geheim Zwei", "amount": 77.25, "unit": "Kapseln", "endDate": "2027-03-14"}`, http.StatusOK},
		{http.MethodGet, "/api/v1/routines/occurrences?from=2026-11-23&to=2026-11-23", "", http.StatusOK},
		{http.MethodGet, path + "/logs?from=2026-11-01", "", http.StatusOK},
	}
	for _, st := range steps {
		if rec := s.do(jsonRequest(st.method, st.path, st.body), testToken); rec.Code != st.status {
			t.Fatalf("%s %s: %d %s", st.method, st.path, rec.Code, rec.Body)
		}
	}

	// An internal error on a change: only its cause, never the request.
	s.routines.Fail["UpdateRoutine"] = errors.New("database unreachable")
	if rec := s.do(jsonRequest(http.MethodPatch, path, `{"name": "Geheim Drei"}`), testToken); rec.Code != http.StatusInternalServerError {
		t.Fatalf("internal error: %d", rec.Code)
	}
	delete(s.routines.Fail, "UpdateRoutine")

	var logs struct{ Logs []struct{ ID string } }
	_ = json.Unmarshal(s.do(jsonRequest(http.MethodGet, path+"/logs?from=2026-11-01", ""), testToken).Body.Bytes(), &logs)
	if len(logs.Logs) != 1 {
		t.Fatalf("logs: %+v", logs)
	}
	s.do(jsonRequest(http.MethodDelete, path+"/logs/"+logs.Logs[0].ID, ""), testToken)
	if rec := s.do(jsonRequest(http.MethodDelete, path, ""), testToken); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}

	log := buf.String()
	for _, want := range []string{"routine created", "routine log added", "routine paused", "routine resumed",
		"routine changed", "routine log deleted", "routine deleted", r.ID} {
		if !strings.Contains(log, want) {
			t.Fatalf("log misses %q, the test would prove nothing:\n%s", want, log)
		}
	}
	for _, secret := range []string{
		"Geheim", "Blutverdünner", "Tropfen", "Kapseln", "123.5", "123,5", "99.75", "77.25", "999999",
		"06:58:47", "58:47", "2026-12-24", "2027-03-14", "08:00",
	} {
		if strings.Contains(log, secret) {
			t.Errorf("log contains %q:\n%s", secret, log)
		}
	}
}

// B on A's routine: 404 ROUTINE_NOT_FOUND on every route, exactly like a
// routine that does not exist, nothing of A's routine in the answer, and
// A's routine and log stay as they were.
func TestSeparation_OtherUsersRoutineIsNotFound(t *testing.T) {
	s := newTwoUsers(t)
	routineID, logID := s.routineOfA(t, "Vitamin D")
	before, _ := s.routines.Routine(routineID)
	path := "/api/v1/routines/" + routineID

	for _, r := range []*http.Request{
		jsonRequest(http.MethodGet, path, ""),
		jsonRequest(http.MethodPatch, path, `{"name": "Von B", "endDate": "2026-11-30"}`),
		jsonRequest(http.MethodPost, path+"/pause", `{"until": "2026-12-31"}`),
		jsonRequest(http.MethodPost, path+"/resume", ""),
		jsonRequest(http.MethodGet, path+"/logs", ""),
		jsonRequest(http.MethodPost, path+"/logs", `{"dueAt": "2026-11-23T19:00:00Z", "status": "done", "source": "manual"}`),
		jsonRequest(http.MethodDelete, path+"/logs/"+logID, ""),
		jsonRequest(http.MethodDelete, path, ""),
		// Also with an invalid body: 404 comes first.
		jsonRequest(http.MethodPatch, path, `{}`),
	} {
		t.Run(r.Method+" "+r.URL.Path, func(t *testing.T) {
			rec := s.do(r, tokenB)
			a := checkError(t, rec, http.StatusNotFound, CodeRoutineNotFound)
			if a.Error.Message != "Diese Routine gibt es nicht." || a.Error.Details != nil {
				t.Errorf("error = %+v", a.Error)
			}
			if strings.Contains(rec.Body.String(), "Vitamin") {
				t.Error("A's routine in the response")
			}
		})
	}

	after, ok := s.routines.Routine(routineID)
	if !ok || after.Name != before.Name || len(after.Pauses) != 0 || after.EndDate != nil {
		t.Errorf("A's routine changed: %+v", after)
	}
	if _, ok := s.routines.Log(logID); !ok {
		t.Error("A's log deleted by B")
	}
	// A still sees both.
	rec := s.do(jsonRequest(http.MethodGet, path+"/logs", ""), testToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), logID) {
		t.Errorf("A's logs: %d %s", rec.Code, rec.Body)
	}
}

// B's lists are empty; B's own routine with A's log ID: 404
// ROUTINE_LOG_NOT_FOUND.
func TestSeparation_RoutineListsOnlyOwnData(t *testing.T) {
	s := newTwoUsers(t)
	routineID, logID := s.routineOfA(t, "Vitamin D")

	rec := s.do(jsonRequest(http.MethodGet, "/api/v1/routines", ""), tokenB)
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `{"routines":[],"count":0}` {
		t.Errorf("B's routines: %d %s", rec.Code, got)
	}
	rec = s.do(jsonRequest(http.MethodGet, "/api/v1/routines/occurrences?from=2026-11-23&to=2026-11-23", ""), tokenB)
	if got := strings.TrimSpace(rec.Body.String()); got != `{"from":"2026-11-23","to":"2026-11-23","occurrences":[],"count":0}` {
		t.Errorf("B's occurrences: %s", got)
	}
	recA := s.do(jsonRequest(http.MethodGet, "/api/v1/routines/occurrences?from=2026-11-23&to=2026-11-23", ""), testToken)
	if !strings.Contains(recA.Body.String(), routineID) || !strings.Contains(recA.Body.String(), logID) {
		t.Errorf("A's occurrences: %s", recA.Body)
	}

	rec = s.do(jsonRequest(http.MethodPost, "/api/v1/routines",
		`{"name": "Vitamin D", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-23"}`), tokenB)
	if rec.Code != http.StatusCreated {
		t.Fatalf("B's routine with A's name: %d %s", rec.Code, rec.Body)
	}
	var own struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &own); err != nil {
		t.Fatal(err)
	}
	rec = s.do(jsonRequest(http.MethodDelete, "/api/v1/routines/"+own.ID+"/logs/"+logID, ""), tokenB)
	checkError(t, rec, http.StatusNotFound, CodeRoutineLogNotFound)
	if _, ok := s.routines.Log(logID); !ok {
		t.Error("A's log deleted through B's routine")
	}
}
