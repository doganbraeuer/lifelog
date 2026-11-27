package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/routinetest"
	"lifelog/backend/internal/service"
)

// routineAPI: the real routine service on the in-memory store, the clock
// on Monday 2026-11-23 at 10:15 in Vienna, and IDs counted up from
// 6742c0a1b2c3d4e5f6a7b801, so responses can be compared exactly.
type routineAPI struct {
	ta     *testAccounts
	store  *routinetest.Store
	router http.Handler
}

func newRoutineAPI(t *testing.T) *routineAPI {
	t.Helper()
	ta := newTestAccounts()
	store := routinetest.New()
	n := 0
	newID := func() string { n++; return fmt.Sprintf("6742c0a1b2c3d4e5f6a7b8%02x", n) }
	deps := Dependencies{Routines: service.NewRoutines(store, newID, func() time.Time { return separationNow })}
	return &routineAPI{ta: ta, store: store, router: ta.router(deps)}
}

func (a *routineAPI) call(method, path, body string) *httptest.ResponseRecorder {
	return request(a.router, method, path, "Bearer "+testToken, body)
}

func id(n int) string { return fmt.Sprintf("6742c0a1b2c3d4e5f6a7b8%02x", n) }

const (
	waterBody = `{"name": "Wasser", "amount": 250, "unit": "ml",
		"schedule": {"frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00"}, "startDate": "2026-11-23"}`
	bloodPressureBody = `{"name": "Blutdruck messen",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]}, "startDate": "2026-11-18", "endDate": "2026-12-01"}`
	vitaminDBody = `{"name": "Vitamin D", "amount": 2, "unit": "Stück",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-18"}`
)

func checkBody(t *testing.T, rec *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	if rec.Code != status {
		t.Errorf("status = %d, want %d", rec.Code, status)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

// The example of API.md 3.18, exactly: 201, Location, the full routine.
func TestRoutineAPI_Create(t *testing.T) {
	a := newRoutineAPI(t)
	rec := a.call(http.MethodPost, "/api/v1/routines", waterBody)
	checkBody(t, rec, http.StatusCreated, `{"id":"`+id(1)+`","module":"health","name":"Wasser","amount":250,"unit":"ml",`+
		`"schedule":{"frequency":"hourly","interval":1,"windowStart":"08:00","windowEnd":"20:00"},"startDate":"2026-11-23","endDate":null,`+
		`"status":"active","pauses":[],"createdAt":"2026-11-23T09:15:00Z",`+
		`"progress":{"today":{"done":0,"skipped":0,"due":13,"amountDone":0,"amountTarget":3250},`+
		`"week":{"done":0,"skipped":0,"due":91,"amountDone":0,"amountTarget":22750},"period":null}}`)
	if got := rec.Header().Get("Location"); got != "/api/v1/routines/"+id(1) {
		t.Errorf("Location = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestRoutineAPI_GetListUpdate(t *testing.T) {
	a := newRoutineAPI(t)
	a.call(http.MethodPost, "/api/v1/routines", bloodPressureBody)
	a.call(http.MethodPost, "/api/v1/routines", `{"name": "Eisbaden",
		"schedule": {"frequency": "weekly", "interval": 2, "weekdays": ["fri", "tue"], "times": ["07:00"]}, "startDate": "2026-11-23"}`)

	bloodPressure := `{"id":"` + id(1) + `","module":"health","name":"Blutdruck messen","amount":null,"unit":null,` +
		`"schedule":{"frequency":"daily","interval":1,"times":["08:00","20:00"]},"startDate":"2026-11-18","endDate":"2026-12-01",` +
		`"status":"active","pauses":[],"createdAt":"2026-11-23T09:15:00Z",` +
		`"progress":{"today":{"done":0,"skipped":0,"due":2,"amountDone":null,"amountTarget":null},` +
		`"week":{"done":0,"skipped":0,"due":14,"amountDone":null,"amountTarget":null},` +
		`"period":{"day":6,"days":14,"done":0,"skipped":0,"due":28,"amountDone":null,"amountTarget":null}}}`
	checkBody(t, a.call(http.MethodGet, "/api/v1/routines/"+id(1), ""), http.StatusOK, bloodPressure)

	rec := a.call(http.MethodGet, "/api/v1/routines?module=health", "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(body, `{"routines":[`+bloodPressure+`,{"id":"`+id(2)+`"`) ||
		!strings.Contains(body, `"schedule":{"frequency":"weekly","interval":2,"weekdays":["tue","fri"],"times":["07:00"]}`) ||
		!strings.HasSuffix(strings.TrimSpace(body), `],"count":2}`) {
		t.Errorf("list: %d %s", rec.Code, body)
	}

	// The example of API.md 3.20.
	rec = a.call(http.MethodPatch, "/api/v1/routines/"+id(1),
		`{"schedule": {"frequency": "daily", "interval": 1, "times": ["07:30", "20:00"]}, "endDate": "2026-12-07"}`)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"times":["07:30","20:00"]`) ||
		!strings.Contains(body, `"endDate":"2026-12-07"`) || !strings.Contains(body, `"day":6,"days":20`) {
		t.Errorf("PATCH: %d %s", rec.Code, body)
	}
}

func TestRoutineAPI_PauseResumeDelete(t *testing.T) {
	a := newRoutineAPI(t)
	a.call(http.MethodPost, "/api/v1/routines", vitaminDBody)
	path := "/api/v1/routines/" + id(1)

	rec := a.call(http.MethodPost, path+"/pause", `{"until": "2026-11-30"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"paused","pauses":[{"from":"2026-11-23","until":"2026-11-30"}]`) {
		t.Errorf("pause: %d %s", rec.Code, rec.Body)
	}
	// Without a body: paused until resumed (the running pause loses its end).
	rec = a.call(http.MethodPost, path+"/pause", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pauses":[{"from":"2026-11-23","until":null}]`) {
		t.Errorf("pause without body: %d %s", rec.Code, rec.Body)
	}
	// A body for resume is ignored.
	rec = a.call(http.MethodPost, path+"/resume", `{"egal": true}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"active","pauses":[]`) {
		t.Errorf("resume: %d %s", rec.Code, rec.Body)
	}

	rec = a.call(http.MethodDelete, path, "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("delete: %d %q", rec.Code, rec.Body)
	}
	checkError(t, a.call(http.MethodDelete, path, ""), http.StatusNotFound, CodeRoutineNotFound)
	checkError(t, a.call(http.MethodGet, path, ""), http.StatusNotFound, CodeRoutineNotFound)
}

func TestRoutineAPI_LogsAndOccurrences(t *testing.T) {
	a := newRoutineAPI(t)
	a.call(http.MethodPost, "/api/v1/routines", vitaminDBody)
	path := "/api/v1/routines/" + id(1)

	occurrence := func(status, logID string) string {
		return `{"from":"2026-11-23","to":"2026-11-23","occurrences":[{"routineId":"` + id(1) +
			`","dueAt":"2026-11-23T07:00:00Z","date":"2026-11-23","time":"08:00","status":"` + status + `","logId":` + logID + `}],"count":1}`
	}
	checkBody(t, a.call(http.MethodGet, "/api/v1/routines/occurrences?from=2026-11-23&to=2026-11-23", ""),
		http.StatusOK, occurrence("open", "null"))

	// The example of API.md 3.26.
	log := `{"id":"` + id(2) + `","routineId":"` + id(1) + `","dueAt":"2026-11-23T07:00:00Z","takenAt":"2026-11-23T09:15:00Z",` +
		`"status":"done","amount":2,"source":"notification","entryId":null,"timeZone":"Europe/Vienna","createdAt":"2026-11-23T09:15:00Z"}`
	rec := a.call(http.MethodPost, path+"/logs", `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "notification"}`)
	checkBody(t, rec, http.StatusCreated, log)
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("Location = %q, want none", loc)
	}
	checkBody(t, a.call(http.MethodGet, "/api/v1/routines/occurrences?from=2026-11-23&to=2026-11-23", ""),
		http.StatusOK, occurrence("done", `"`+id(2)+`"`))
	checkBody(t, a.call(http.MethodGet, path+"/logs", ""), http.StatusOK, `{"logs":[`+log+`],"count":1}`)

	// The reminder button sent twice.
	rec = a.call(http.MethodPost, path+"/logs", `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "notification"}`)
	checkBody(t, rec, http.StatusConflict, `{"error":{"code":"ROUTINE_LOG_EXISTS","message":"Dieser Termin ist schon eingetragen.","details":{"logId":"`+id(2)+`"}}}`)

	rec = a.call(http.MethodDelete, path+"/logs/"+id(2), "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("delete log: %d %q", rec.Code, rec.Body)
	}
	checkBody(t, a.call(http.MethodGet, path+"/logs", ""), http.StatusOK, `{"logs":[],"count":0}`)
	a2 := checkError(t, a.call(http.MethodDelete, path+"/logs/"+id(2), ""), http.StatusNotFound, CodeRoutineLogNotFound)
	if a2.Error.Message != "Diesen Verlaufseintrag gibt es nicht." {
		t.Errorf("message %q", a2.Error.Message)
	}

	// The fixed path wins over /routines/{id}; other methods on it are an
	// invalid ID.
	if rec := a.call(http.MethodGet, "/api/v1/routines/occurrences", ""); rec.Code != http.StatusOK {
		t.Errorf("GET occurrences: %d %s", rec.Code, rec.Body)
	}
	checkError(t, a.call(http.MethodPatch, "/api/v1/routines/occurrences", `{"name": "x"}`), http.StatusBadRequest, CodeInvalidID)
}

func TestRoutineAPI_Errors(t *testing.T) {
	a := newRoutineAPI(t)
	a.call(http.MethodPost, "/api/v1/routines", vitaminDBody)
	valid, unknown := id(1), "6742c0a1b2c3d4e5f6a7b8ff"

	// The example of API.md 3.18.
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines", `{"name": "Wasser", "schedule": {"frequency": "daily", "interval": 1, "times": ["8 Uhr"]}, "startDate": "2026-11-23"}`),
		http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00.","details":{"field":"schedule.times","value":"8 Uhr"}}}`)
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines", `name=Wasser`),
		http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Bitte die Routine als JSON-Objekt schicken."}}`)
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines", `{"name": "`+strings.Repeat("x", 17<<10)+`"}`),
		http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Die Anfrage ist zu groß oder unvollständig."}}`)
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines", `{"name": "vitamin d", "schedule": {"frequency": "daily", "interval": 1, "times": ["09:00"]}, "startDate": "2026-11-23"}`),
		http.StatusConflict, `{"error":{"code":"ROUTINE_NAME_TAKEN","message":"Eine Routine mit diesem Namen gibt es schon.","details":{"routineId":"`+valid+`"}}}`)
	checkBody(t, a.call(http.MethodPatch, "/api/v1/routines/"+valid, `{"status": "paused"}`),
		http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Das Feld status kann nicht geändert werden.","details":{"field":"status"}}}`)
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines/"+valid+"/pause", `{"until": "2026-11-22"}`),
		http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Das Ende der Pause darf nicht vor heute liegen.","details":{"field":"until","value":"2026-11-22"}}}`)
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines/"+valid+"/logs", `{"dueAt": "2026-11-24T07:00:00Z", "status": "done", "source": "manual"}`),
		http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Termine in der Zukunft können noch nicht eingetragen werden.","details":{"field":"dueAt","value":"2026-11-24T07:00:00Z"}}}`)

	for _, r := range []struct{ method, path, message string }{
		{http.MethodGet, "/api/v1/routines/abc", messageInvalidRoutineID},
		{http.MethodPatch, "/api/v1/routines/abc", messageInvalidRoutineID},
		{http.MethodDelete, "/api/v1/routines/abc", messageInvalidRoutineID},
		{http.MethodPost, "/api/v1/routines/abc/pause", messageInvalidRoutineID},
		{http.MethodPost, "/api/v1/routines/abc/resume", messageInvalidRoutineID},
		{http.MethodGet, "/api/v1/routines/abc/logs", messageInvalidRoutineID},
		{http.MethodPost, "/api/v1/routines/abc/logs", messageInvalidRoutineID},
		{http.MethodDelete, "/api/v1/routines/abc/logs/" + valid, messageInvalidRoutineID},
		{http.MethodDelete, "/api/v1/routines/" + valid + "/logs/abc", messageInvalidLogID},
	} {
		e := checkError(t, a.call(r.method, r.path, `{}`), http.StatusBadRequest, CodeInvalidID)
		if e.Error.Message != r.message {
			t.Errorf("%s %s: message %q", r.method, r.path, e.Error.Message)
		}
	}
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/routines/" + unknown},
		{http.MethodPost, "/api/v1/routines/" + unknown + "/resume"},
		{http.MethodDelete, "/api/v1/routines/" + unknown + "/logs/" + valid},
	} {
		checkError(t, a.call(r.method, r.path, ""), http.StatusNotFound, CodeRoutineNotFound)
	}

	for _, r := range []struct{ path, param, value string }{
		{"/api/v1/routines?module=sport", "module", "sport"},
		{"/api/v1/routines/occurrences?module=Health", "module", "Health"},
		{"/api/v1/routines/occurrences?from=23.11.2026", "from", "23.11.2026"},
		{"/api/v1/routines/occurrences?from=2026-11-01&to=2026-12-15", "to", "2026-12-15"},
		{"/api/v1/routines/occurrences?from=2026-11-25&to=2026-11-24", "to", "2026-11-24"},
		{"/api/v1/routines/" + valid + "/logs?to=2026-02-30", "to", "2026-02-30"},
		{"/api/v1/routines/" + valid + "/logs?from=2025-01-01&to=2026-11-23", "to", "2026-11-23"},
	} {
		e := checkError(t, a.call(http.MethodGet, r.path, ""), http.StatusBadRequest, CodeInvalidParameter)
		if e.Error.Details["parameter"] != r.param || e.Error.Details["value"] != r.value {
			t.Errorf("%s: details %v", r.path, e.Error.Details)
		}
	}
}

func TestRoutineAPI_Limit(t *testing.T) {
	a := newRoutineAPI(t)
	for i := range core.MaxRoutinesPerUser {
		a.store.Put(core.Routine{ID: fmt.Sprintf("6742c0a1b2c3d4e5f6a7c%03x", i), UserID: a.ta.user.ID, Module: "health",
			Name: fmt.Sprintf("Routine %d", i), Schedule: core.Schedule{Frequency: core.FrequencyDaily, Interval: 1}})
	}
	checkBody(t, a.call(http.MethodPost, "/api/v1/routines", vitaminDBody), http.StatusConflict,
		`{"error":{"code":"ROUTINE_LIMIT_REACHED","message":"Du hast schon 50 Routinen. Lösche eine, bevor du eine neue anlegst.","details":{"limit":"routines","max":50}}}`)
}

// Routines need a token, and a changed start password, like every
// endpoint except the three of section 1.
func TestRoutineAPI_LoginRequired(t *testing.T) {
	a := newRoutineAPI(t)
	checkError(t, request(a.router, http.MethodGet, "/api/v1/routines", "", ""), http.StatusUnauthorized, CodeUnauthorized)
	checkError(t, request(a.router, http.MethodGet, "/api/v1/routines/occurrences", "", ""), http.StatusUnauthorized, CodeUnauthorized)

	u := a.ta.addUser("tester02", "Startpasswort1", true)
	a.ta.addSession(u.ID, "start-token", time.Now().Add(time.Hour))
	checkError(t, request(a.router, http.MethodPost, "/api/v1/routines", "Bearer start-token", vitaminDBody),
		http.StatusForbidden, CodePasswordChangeRequired)
	if a.store.Mark() != 0 {
		t.Error("the store was asked before login and password change were checked")
	}
}
