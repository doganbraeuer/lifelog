package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

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
