package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

func login(h http.Handler, username, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return request(h, http.MethodPost, "/api/v1/auth/login", "", string(body))
}

func TestLogin_Success(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})
	rec := login(router, "Tester01 ", testPassword) // capital letter, space: still tester01

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	checkJSONHeader(t, rec)
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	var got struct {
		Token              string `json:"token"`
		ExpiresAt          string `json:"expiresAt"`
		MustChangePassword *bool  `json:"mustChangePassword"`
		User               struct {
			ID, Username, DisplayName string
			ConsentVersion            *string `json:"consentVersion"`
		} `json:"user"`
	}
	dec := json.NewDecoder(rec.Body)
	dec.DisallowUnknownFields() // nothing beyond the contract, e.g. no hash
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Token) != 43 || got.MustChangePassword == nil || *got.MustChangePassword {
		t.Errorf("response = %+v", got)
	}
	if got.User.ID != ta.user.ID || got.User.Username != "tester01" || got.User.DisplayName != "tester01" {
		t.Errorf("user = %+v", got.User)
	}
	exp, err := time.Parse(time.RFC3339, got.ExpiresAt)
	if err != nil || !strings.HasSuffix(got.ExpiresAt, "Z") ||
		exp.Before(time.Now().Add(30*24*time.Hour-time.Minute)) || exp.After(time.Now().Add(30*24*time.Hour+time.Minute)) {
		t.Errorf("expiresAt = %q, want RFC 3339 UTC, 30 days from now", got.ExpiresAt)
	}

	// The new token works.
	if rec := request(router, http.MethodGet, "/api/v1/me", "Bearer "+got.Token, ""); rec.Code != http.StatusOK {
		t.Errorf("GET /me with the new token: %d", rec.Code)
	}
}

// Unknown user, wrong password, disabled account: byte-for-byte the same
// response.
func TestLogin_SameAnswerForEveryFailure(t *testing.T) {
	ta := newTestAccounts()
	disabled := ta.addUser("tester03", "Passwort-tester03", false)
	disabled.Disabled = true
	ta.store.users[disabled.ID] = disabled
	router := ta.router(Dependencies{})

	wrong := login(router, "tester01", "falschesPasswort")
	checkError(t, wrong, http.StatusUnauthorized, CodeInvalidCredentials)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"unknown user":     login(router, "tester99", testPassword),
		"disabled account": login(router, "tester03", "Passwort-tester03"),
		"over 72 bytes":    login(router, "tester01", testPassword+strings.Repeat("x", 70)),
	} {
		if rec.Code != wrong.Code || rec.Body.String() != wrong.Body.String() {
			t.Errorf("%s: %d %s; want the same as a wrong password: %d %s", name, rec.Code, rec.Body, wrong.Code, wrong.Body)
		}
	}
	if !strings.Contains(wrong.Body.String(), "Benutzername oder Passwort stimmt nicht.") {
		t.Errorf("message: %s", wrong.Body)
	}
}

func TestLogin_LockAfterFiveFailures(t *testing.T) {
	router := newTestAccounts().router(Dependencies{})
	for i := 0; i < 5; i++ {
		checkError(t, login(router, "tester01", "falschesPasswort"), http.StatusUnauthorized, CodeInvalidCredentials)
	}
	rec := login(router, "tester01", testPassword) // correct, but locked
	a := checkError(t, rec, http.StatusTooManyRequests, CodeTooManyAttempts)
	if a.Error.Message != "Zu viele Fehlversuche. Bitte versuche es in einigen Minuten erneut." {
		t.Errorf("message = %q", a.Error.Message)
	}
	checkRetryAfter(t, rec, a, 60)
}

// Retry-After and details.retryAfterSeconds carry the same number.
func checkRetryAfter(t *testing.T, rec *httptest.ResponseRecorder, a errorResponse, want int) {
	t.Helper()
	header, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	detail, ok := a.Error.Details["retryAfterSeconds"].(float64)
	if !ok || int(detail) != header {
		t.Errorf("details.retryAfterSeconds = %v, Retry-After = %d; want equal", a.Error.Details["retryAfterSeconds"], header)
	}
	if header < want-1 || header > want {
		t.Errorf("Retry-After = %d, want about %d", header, want)
	}
}

// X-Forwarded-For is only believed from a trusted proxy. Without one, a
// forged header does not give a locked client a fresh counter.
func TestLogin_ForwardedForOnlyFromTrustedProxy(t *testing.T) {
	loginFrom := func(h http.Handler, remote, xff, password string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"username": "tester01", "password": password})
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(body)))
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	// No trusted proxy: five failures from one address lock it, whatever
	// the header claims.
	router := newTestAccounts().router(Dependencies{})
	for i := 0; i < 5; i++ {
		loginFrom(router, "203.0.113.7:5000", "198.51.100."+strconv.Itoa(i), "falsch-falsch")
	}
	if rec := loginFrom(router, "203.0.113.7:5000", "198.51.100.200", testPassword); rec.Code != http.StatusTooManyRequests {
		t.Errorf("forged header escaped the lock: %d", rec.Code)
	}

	// Behind Caddy on 127.0.0.1: each client is counted by its own address.
	router = newTestAccounts().router(Dependencies{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}})
	for i := 0; i < 5; i++ {
		loginFrom(router, "127.0.0.1:40000", "198.51.100.66", "falsch-falsch")
	}
	if rec := loginFrom(router, "127.0.0.1:40000", "198.51.100.66", testPassword); rec.Code != http.StatusTooManyRequests {
		t.Errorf("attacker behind the proxy not locked: %d", rec.Code)
	}
	if rec := loginFrom(router, "127.0.0.1:40000", "203.0.113.7", testPassword); rec.Code != http.StatusOK {
		t.Errorf("real tester behind the same proxy locked out: %d %s", rec.Code, rec.Body)
	}
}

func TestLogin_BadRequests(t *testing.T) {
	router := newTestAccounts().router(Dependencies{})
	for name, body := range map[string]string{
		"not JSON":         "username=tester01",
		"array":            `["tester01"]`,
		"empty object":     `{}`,
		"password missing": `{"username": "tester01"}`,
		"username null":    `{"username": null, "password": "x"}`,
		"username empty":   `{"username": "  ", "password": "x"}`,
		"password empty":   `{"username": "tester01", "password": ""}`,
		"number":           `{"username": 1, "password": "x"}`,
		"unknown field":    `{"username": "tester01", "password": "x", "remember": true}`,
		"two objects":      `{"username": "tester01", "password": "x"} {}`,
		"too large":        `{"username": "tester01", "password": "` + strings.Repeat("x", 5000) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := request(router, http.MethodPost, "/api/v1/auth/login", "", body)
			checkError(t, rec, http.StatusBadRequest, CodeInvalidRequest)
		})
	}
}

func TestLogout(t *testing.T) {
	ta := newTestAccounts()
	ta.addSession(ta.user.ID, "tablet-token", time.Now().Add(time.Hour))
	router := ta.router(Dependencies{})

	rec := request(router, http.MethodPost, "/api/v1/auth/logout", "Bearer "+testToken, "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("logout: %d %q", rec.Code, rec.Body)
	}
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, ""))
	checkUnauthorized(t, request(router, http.MethodPost, "/api/v1/auth/logout", "Bearer "+testToken, ""))
	if rec := request(router, http.MethodGet, "/api/v1/me", "Bearer tablet-token", ""); rec.Code != http.StatusOK {
		t.Errorf("other device logged out too: %d", rec.Code)
	}
}

func TestMe(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})
	rec := request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	// Without a stored time zone GET /me names the default (API.md 3.12).
	want := `{"id":"` + ta.user.ID + `","username":"tester01","displayName":"tester01","mustChangePassword":false,"consentVersion":null,"timeZone":"Europe/Vienna"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestConsent(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})
	rec := request(router, http.MethodPut, "/api/v1/me/consent", "Bearer "+testToken, `{"version": " 2026-11-18 "}`)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("PUT /me/consent: %d %s", rec.Code, rec.Body)
	}
	u := ta.store.users[ta.user.ID]
	if u.ConsentVersion == nil || *u.ConsentVersion != "2026-11-18" || u.ConsentAt == nil {
		t.Errorf("stored consent = %v / %v", u.ConsentVersion, u.ConsentAt)
	}
	// GET /me and the login response show it.
	if me := request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, ""); !strings.Contains(me.Body.String(), `"consentVersion":"2026-11-18"`) {
		t.Errorf("GET /me: %s", me.Body)
	}
	if l := login(router, "tester01", testPassword); !strings.Contains(l.Body.String(), `"consentVersion":"2026-11-18"`) {
		t.Errorf("login: %s", l.Body)
	}
}

func TestConsent_Errors(t *testing.T) {
	cases := map[string]struct {
		body, field string
	}{
		"empty":         {`{"version": "  "}`, "version"},
		"null":          {`{"version": null}`, "version"},
		"number":        {`{"version": 1}`, "version"},
		"missing":       {`{}`, "version"},
		"too long":      {`{"version": "` + strings.Repeat("x", 101) + `"}`, "version"},
		"control":       {`{"version": "v1\nv2"}`, "version"},
		"unknown field": {`{"version": "v1", "consentAt": "2020-01-01"}`, "consentAt"},
		"not an object": {`"v1"`, "version"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestAccounts()
			router := ta.router(Dependencies{})
			rec := request(router, http.MethodPut, "/api/v1/me/consent", "Bearer "+testToken, c.body)
			a := checkError(t, rec, http.StatusBadRequest, CodeInvalidRequest)
			if got, _ := a.Error.Details["field"].(string); got != c.field {
				t.Errorf("details.field = %q, want %q", got, c.field)
			}
			if ta.store.users[ta.user.ID].ConsentVersion != nil {
				t.Error("consent stored")
			}
		})
	}
}

// While the start password is unchanged, consent is not one of the allowed
// routes (API.md 3.15).
func TestConsent_RequiresPasswordChange(t *testing.T) {
	ta := newTestAccounts()
	u := ta.addUser("tester02", "Startpasswort1", true)
	ta.addSession(u.ID, "start-token", time.Now().Add(time.Hour))
	router := ta.router(Dependencies{})
	rec := request(router, http.MethodPut, "/api/v1/me/consent", "Bearer start-token", `{"version": "v1"}`)
	checkError(t, rec, http.StatusForbidden, CodePasswordChangeRequired)
}

func TestUpdateMe(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})
	rec := request(router, http.MethodPatch, "/api/v1/me", "Bearer "+testToken, `{"displayName": "  Erika  "}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"displayName":"Erika"`) {
		t.Fatalf("PATCH /me: %d %s", rec.Code, rec.Body)
	}
	if ta.store.users[ta.user.ID].DisplayName != "Erika" {
		t.Error("not stored")
	}
	// GET /me shows the new name.
	if rec := request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, ""); !strings.Contains(rec.Body.String(), `"displayName":"Erika"`) {
		t.Errorf("GET /me after PATCH: %s", rec.Body)
	}
}

func TestUpdateMe_TimeZone(t *testing.T) {
	ta := newTestAccounts()
	router := ta.router(Dependencies{})
	rec := request(router, http.MethodPatch, "/api/v1/me", "Bearer "+testToken, `{"timeZone": "Europe/Lisbon"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"timeZone":"Europe/Lisbon"`) ||
		!strings.Contains(rec.Body.String(), `"displayName":"tester01"`) {
		t.Fatalf("PATCH /me: %d %s", rec.Code, rec.Body)
	}
	if u := ta.store.users[ta.user.ID]; u.TimeZone != "Europe/Lisbon" || u.DisplayName != "tester01" {
		t.Errorf("stored: %q / %q", u.TimeZone, u.DisplayName)
	}
	if rec := request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, ""); !strings.Contains(rec.Body.String(), `"timeZone":"Europe/Lisbon"`) {
		t.Errorf("GET /me after PATCH: %s", rec.Body)
	}

	// Both fields in one request.
	rec = request(router, http.MethodPatch, "/api/v1/me", "Bearer "+testToken, `{"displayName": "Erika", "timeZone": "UTC"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"displayName":"Erika"`) ||
		!strings.Contains(rec.Body.String(), `"timeZone":"UTC"`) {
		t.Errorf("both fields: %d %s", rec.Code, rec.Body)
	}

	// The login response stays as in 3.10, without timeZone.
	if l := login(router, "tester01", testPassword); strings.Contains(l.Body.String(), "timeZone") {
		t.Errorf("login response has timeZone: %s", l.Body)
	}
}

func TestUpdateMe_Errors(t *testing.T) {
	cases := map[string]struct {
		body    string
		field   string // details.field; "" = no details
		value   any    // details.value; nil = none
		message string // checked if not empty
	}{
		"empty":         {body: `{"displayName": "   "}`, field: "displayName"},
		"51 characters": {body: `{"displayName": "` + strings.Repeat("ä", 51) + `"}`, field: "displayName"},
		"line break":    {body: `{"displayName": "Erika\nMuster"}`, field: "displayName"},
		"null":          {body: `{"displayName": null}`, field: "displayName"},
		"number":        {body: `{"displayName": 5}`, field: "displayName"},
		"missing": {body: `{}`, field: "",
			message: "Bitte einen Anzeigenamen oder eine Zeitzone schicken."},
		"unknown field": {body: `{"displayName": "Erika", "username": "admin"}`, field: "username"},
		"role":          {body: `{"role": "admin"}`, field: "role"},
		"not JSON":      {body: `displayName=Erika`, field: ""},
		"not an object": {body: `"Erika"`, field: ""},

		"time zone abbreviation": {body: `{"timeZone": "CET"}`, field: "timeZone", value: "CET",
			message: "Unbekannte Zeitzone."},
		"time zone offset":  {body: `{"timeZone": "+01:00"}`, field: "timeZone", value: "+01:00"},
		"time zone empty":   {body: `{"timeZone": ""}`, field: "timeZone", value: ""},
		"time zone Local":   {body: `{"timeZone": "Local"}`, field: "timeZone", value: "Local"},
		"time zone null":    {body: `{"timeZone": null}`, field: "timeZone"},
		"time zone number":  {body: `{"timeZone": 1}`, field: "timeZone", value: float64(1)},
		"time zone unknown": {body: `{"timeZone": "Europe/Atlantis"}`, field: "timeZone", value: "Europe/Atlantis"},
		// Order: unknown field, display name, time zone. The valid display
		// name is not stored either.
		"unknown before both":    {body: `{"displayName": "", "timeZone": "CET", "zone": 1}`, field: "zone"},
		"display name first":     {body: `{"displayName": "", "timeZone": 5}`, field: "displayName"},
		"valid name, wrong zone": {body: `{"displayName": "Erika", "timeZone": "CET"}`, field: "timeZone", value: "CET"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestAccounts()
			router := ta.router(Dependencies{})
			rec := request(router, http.MethodPatch, "/api/v1/me", "Bearer "+testToken, c.body)
			a := checkError(t, rec, http.StatusBadRequest, CodeInvalidRequest)
			if got, _ := a.Error.Details["field"].(string); got != c.field {
				t.Errorf("details.field = %q, want %q", got, c.field)
			}
			if got := a.Error.Details["value"]; got != c.value {
				t.Errorf("details.value = %#v, want %#v", got, c.value)
			}
			if c.message != "" && a.Error.Message != c.message {
				t.Errorf("message = %q, want %q", a.Error.Message, c.message)
			}
			if u := ta.store.users[ta.user.ID]; u.DisplayName != "tester01" || u.TimeZone != "" {
				t.Errorf("changed: %q / %q", u.DisplayName, u.TimeZone)
			}
		})
	}
}

func TestChangePassword_Success(t *testing.T) {
	ta := newTestAccounts()
	u := ta.addUser("tester02", "Startpasswort1", true)
	ta.addSession(u.ID, "phone-token", time.Now().Add(time.Hour))
	ta.addSession(u.ID, "tablet-token", time.Now().Add(time.Hour))
	router := ta.router(Dependencies{Entries: &fakeStore{}})

	rec := request(router, http.MethodPost, "/api/v1/me/password", "Bearer phone-token",
		`{"currentPassword": "Startpasswort1", "newPassword": "Sonnenblume am Mönchsberg"}`)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	// This device stays logged in and may now use everything.
	me := request(router, http.MethodGet, "/api/v1/me", "Bearer phone-token", "")
	if !strings.Contains(me.Body.String(), `"mustChangePassword":false`) {
		t.Errorf("GET /me: %s", me.Body)
	}
	if rec := request(router, http.MethodGet, "/api/v1/entries", "Bearer phone-token", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /entries after the change: %d", rec.Code)
	}
	// The other device is logged out.
	checkUnauthorized(t, request(router, http.MethodGet, "/api/v1/me", "Bearer tablet-token", ""))
	// Old password gone, new one works.
	checkError(t, login(router, "tester02", "Startpasswort1"), http.StatusUnauthorized, CodeInvalidCredentials)
	if rec := login(router, "tester02", "Sonnenblume am Mönchsberg"); rec.Code != http.StatusOK {
		t.Errorf("login with the new password: %d", rec.Code)
	}
}

func TestChangePassword_Errors(t *testing.T) {
	cases := map[string]struct {
		body    string
		status  int
		code    string
		message string
	}{
		"wrong current": {`{"currentPassword": "falsch-falsch", "newPassword": "Sonnenblume am Mönchsberg"}`,
			400, CodeWrongPassword, "Das bisherige Passwort stimmt nicht."},
		"too short": {`{"currentPassword": "` + testPassword + `", "newPassword": "kurz"}`,
			400, CodeWeakPassword, "Das neue Passwort muss mindestens 10 Zeichen lang sein."},
		"too long": {`{"currentPassword": "` + testPassword + `", "newPassword": "` + strings.Repeat("ä", 37) + `"}`,
			400, CodeWeakPassword, "Das neue Passwort ist zu lang. Erlaubt sind höchstens 72 Byte, Umlaute zählen doppelt."},
		"unchanged": {`{"currentPassword": "` + testPassword + `", "newPassword": "` + testPassword + `"}`,
			400, CodeWeakPassword, "Das neue Passwort muss sich vom bisherigen unterscheiden."},
		"field missing": {`{"currentPassword": "` + testPassword + `"}`,
			400, CodeInvalidRequest, "Bitte das bisherige und das neue Passwort schicken."},
		"unknown field": {`{"currentPassword": "a", "newPassword": "b", "username": "x"}`,
			400, CodeInvalidRequest, "Bitte das bisherige und das neue Passwort schicken."},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestAccounts()
			router := ta.router(Dependencies{})
			before := ta.store.users[ta.user.ID].PasswordHash
			rec := request(router, http.MethodPost, "/api/v1/me/password", "Bearer "+testToken, c.body)
			a := checkError(t, rec, c.status, c.code)
			if a.Error.Message != c.message {
				t.Errorf("message = %q, want %q", a.Error.Message, c.message)
			}
			if ta.store.users[ta.user.ID].PasswordHash != before {
				t.Error("password changed")
			}
			// A wrong password is not a logout: the token still works.
			if rec := request(router, http.MethodGet, "/api/v1/me", "Bearer "+testToken, ""); rec.Code != http.StatusOK {
				t.Errorf("token no longer valid: %d", rec.Code)
			}
		})
	}
}

// Dylan's decision: a wrong current password counts as a failed attempt,
// with the same counters as the login.
func TestChangePassword_LockAfterFiveWrongPasswords(t *testing.T) {
	router := newTestAccounts().router(Dependencies{})
	wrong := `{"currentPassword": "falsch-falsch", "newPassword": "Sonnenblume am Mönchsberg"}`
	for i := 0; i < 5; i++ {
		rec := request(router, http.MethodPost, "/api/v1/me/password", "Bearer "+testToken, wrong)
		checkError(t, rec, http.StatusBadRequest, CodeWrongPassword)
	}
	right := `{"currentPassword": "` + testPassword + `", "newPassword": "Sonnenblume am Mönchsberg"}`
	rec := request(router, http.MethodPost, "/api/v1/me/password", "Bearer "+testToken, right)
	a := checkError(t, rec, http.StatusTooManyRequests, CodeTooManyAttempts)
	checkRetryAfter(t, rec, a, 60)
	// The login of the same user from the same IP is locked too.
	checkError(t, login(router, "tester01", testPassword), http.StatusTooManyRequests, CodeTooManyAttempts)
}

// No password, token or Authorization header in the log, whatever happens.
func TestAccounts_NothingSecretInTheLog(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)

	ta := newTestAccounts()
	router := ta.router(Dependencies{Entries: &fakeStore{}})
	ok := login(router, "tester01", testPassword)
	var res struct{ Token string }
	_ = json.Unmarshal(ok.Body.Bytes(), &res)
	login(router, "tester01", "geheim-falsch-123")
	login(router, "geheimer-name", "geheim-falsch-456")
	request(router, http.MethodGet, "/api/v1/entries", "Bearer "+res.Token, "")
	request(router, http.MethodGet, "/api/v1/entries", "Bearer falsches-token-789", "")
	request(router, http.MethodPost, "/api/v1/me/password", "Bearer "+res.Token,
		`{"currentPassword": "geheim-falsch-000", "newPassword": "Sonnenblume am Mönchsberg"}`)
	request(router, http.MethodPost, "/api/v1/me/password", "Bearer "+res.Token,
		`{"currentPassword": "`+testPassword+`", "newPassword": "Sonnenblume am Mönchsberg"}`)
	request(router, http.MethodPost, "/api/v1/auth/logout", "Bearer "+res.Token, "")

	log := buf.String()
	if !strings.Contains(log, "login") {
		t.Fatalf("log looks empty, the test would prove nothing:\n%s", log)
	}
	for _, secret := range []string{
		testPassword, "geheim-falsch", "geheimer-name", "Sonnenblume", res.Token, testToken,
		"falsches-token", "Bearer", "Authorization", "$2a$",
	} {
		if secret != "" && strings.Contains(log, secret) {
			t.Errorf("log contains %q:\n%s", secret, log)
		}
	}
}
