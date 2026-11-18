package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// request sends method/path with an optional Authorization header ("" =
// none) and body.
func request(h http.Handler, method, path, authorization, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if authorization != "" {
		r.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func checkUnauthorized(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	checkError(t, rec, http.StatusUnauthorized, CodeUnauthorized)
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
}

func TestNewRouter_RefusesWithoutAccounts(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRouter without Accounts must panic, never run without login")
		}
	}()
	NewRouter(Dependencies{DB: fakePinger{}})
}

func TestAuth_PublicRoutesWithoutToken(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})

	if rec := request(router, http.MethodGet, "/api/v1/status", "", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /status without token: %d, want 200", rec.Code)
	}
	// Reaches the login handler: 400 for the empty body, not 401.
	rec := request(router, http.MethodPost, "/api/v1/auth/login", "", "{}")
	checkError(t, rec, http.StatusBadRequest, CodeInvalidRequest)
}

func TestAuth_NoToken(t *testing.T) {
	router := newTestAccounts().router(Dependencies{Entries: &fakeStore{}})
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", "", ""))
}

func TestAuth_BadHeader(t *testing.T) {
	router := newTestAccounts().router(Dependencies{Entries: &fakeStore{}})
	for _, header := range []string{
		"Basic dGVzdGVyMDE6aDRQcThzWnQya1h3",
		"Bearer",
		"Bearer ",
		"Bearer    ",
		testToken,
		"Token " + testToken,
		"Bearer " + testToken + " extra",
	} {
		t.Run(header, func(t *testing.T) {
			checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", header, ""))
		})
	}
}

func TestAuth_SchemeIsCaseInsensitive(t *testing.T) {
	router := newTestAccounts().router(Dependencies{Entries: &fakeStore{}})
	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		if rec := request(router, http.MethodGet, "/api/v1/entries", scheme+" "+testToken, ""); rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", scheme, rec.Code)
		}
	}
}

func TestAuth_WrongToken(t *testing.T) {
	router := newTestAccounts().router(Dependencies{Entries: &fakeStore{}})
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", "Bearer "+testToken+"x", ""))
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", "Bearer "+strings.Repeat("A", 43), ""))
}

func TestAuth_ExpiredSession(t *testing.T) {
	ta := newTestAccounts()
	ta.addSession(ta.user.ID, "expired-token", time.Now().Add(-time.Second))
	router := ta.router(Dependencies{Entries: &fakeStore{}})
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", "Bearer expired-token", ""))
}

func TestAuth_DisabledUser(t *testing.T) {
	ta := newTestAccounts()
	u := ta.store.users[ta.user.ID]
	u.Disabled = true
	ta.store.users[u.ID] = u
	router := ta.router(Dependencies{Entries: &fakeStore{}})
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", "Bearer "+testToken, ""))
}

func TestAuth_DeletedUser(t *testing.T) {
	ta := newTestAccounts()
	delete(ta.store.users, ta.user.ID)
	router := ta.router(Dependencies{Entries: &fakeStore{}})
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/entries", "Bearer "+testToken, ""))
}

// Database down: 500, never let the request through.
func TestAuth_DatabaseError(t *testing.T) {
	ta := newTestAccounts()
	ta.store.err = errors.New("connection refused")
	store := &fakeStore{}
	router := ta.router(Dependencies{Entries: store})
	rec := request(router, http.MethodGet, "/api/v1/entries", "Bearer "+testToken, "")
	checkError(t, rec, http.StatusInternalServerError, CodeInternalError)
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Error("internal cause in the response")
	}
}

func TestAuth_MustChangePassword(t *testing.T) {
	ta := newTestAccounts()
	u := ta.addUser("tester02", "Startpasswort1", true)
	ta.addSession(u.ID, "start-token", time.Now().Add(time.Hour))
	router := ta.router(Dependencies{Entries: &fakeStore{}})
	auth := "Bearer start-token"

	// Everything else: 403.
	for _, route := range [][2]string{
		{http.MethodGet, "/api/v1/entries"},
		{http.MethodGet, "/api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0"},
		{http.MethodPatch, "/api/v1/me"},
		{http.MethodPost, "/api/v1/questions"},
		{http.MethodGet, "/api/v1/nothing"},
	} {
		rec := request(router, route[0], route[1], auth, "")
		checkError(t, rec, http.StatusForbidden, CodePasswordChangeRequired)
	}

	// The three allowed routes reach their handlers.
	rec := request(router, http.MethodGet, "/api/v1/me", auth, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"mustChangePassword":true`) {
		t.Errorf("GET /me: %d %s", rec.Code, rec.Body)
	}
	rec = request(router, http.MethodPost, "/api/v1/me/password", auth, "{}")
	checkError(t, rec, http.StatusBadRequest, CodeInvalidRequest) // the handler's own check
	if rec := request(router, http.MethodPost, "/api/v1/auth/logout", auth, ""); rec.Code != http.StatusNoContent {
		t.Errorf("logout: %d, want 204", rec.Code)
	}
}

// Without a token nothing about the API can be probed: unknown paths and
// wrong methods are 401 too. With a token they are 404 and 405 as before.
func TestAuth_UnknownPathAndWrongMethod(t *testing.T) {
	router := newTestAccounts().router(Dependencies{})
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/nothing", "", ""))
	checkUnauthorized(t, request(router, http.MethodGet, "/", "", ""))
	checkUnauthorized(t, request(router, http.MethodDelete, "/api/v1/status", "", ""))
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/auth/login", "", ""))

	if rec := request(router, http.MethodGet, "/api/v1/nothing", "Bearer "+testToken, ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown path with token: %d, want 404", rec.Code)
	}
	if rec := request(router, http.MethodDelete, "/api/v1/status", "Bearer "+testToken, ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("wrong method with token: %d, want 405", rec.Code)
	}
}

// Every route except the two public ones needs a token. A new route that
// is forgotten here is still protected (withAuth locks by default); this
// list makes sure none of the existing ones slipped through.
func TestAuth_AllProtectedRoutesNeedToken(t *testing.T) {
	router := newTestAccounts().router(Dependencies{})
	id := "6727a1f3c2b4e5d6f7a8b9c0"
	for _, route := range [][2]string{
		{http.MethodGet, "/api/v1/entries"},
		{http.MethodPost, "/api/v1/entries"},
		{http.MethodGet, "/api/v1/entries/" + id},
		{http.MethodPatch, "/api/v1/entries/" + id},
		{http.MethodDelete, "/api/v1/entries/" + id},
		{http.MethodGet, "/api/v1/entries/" + id + "/file"},
		{http.MethodPost, "/api/v1/questions"},
		{http.MethodPost, "/api/v1/transcriptions"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/me"},
		{http.MethodPatch, "/api/v1/me"},
		{http.MethodPost, "/api/v1/me/password"},
		{http.MethodPut, "/api/v1/me/consent"},
	} {
		t.Run(route[0]+" "+route[1], func(t *testing.T) {
			checkUnauthorized(t, request(router, route[0], route[1], "", ""))
		})
	}
}

// The user the middleware found is the one the handler sees.
func TestAuth_UserInContext(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})
	rec := request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+ta.user.ID+`"`) {
		t.Errorf("GET /me: %d %s", rec.Code, rec.Body)
	}
}
