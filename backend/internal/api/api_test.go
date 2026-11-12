package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePinger replaces MongoDB in tests.
type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestStatus_DatabaseReachable(t *testing.T) {
	router := NewRouter(Dependencies{DB: fakePinger{}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	checkJSONHeader(t, rec)
	var a statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if a.Status != "ok" || a.Database != "ok" || a.Speech != "off" {
		t.Errorf("response = %+v", a)
	}
}

func TestStatus_DatabaseUnreachable(t *testing.T) {
	router := NewRouter(Dependencies{DB: fakePinger{err: errors.New("timeout")}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	want := `{"status":"error","database":"unreachable","speech":"off"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if strings.Contains(rec.Body.String(), "timeout") {
		t.Error("the internal cause must not be in the response")
	}
}

func TestWriteError_Format(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusNotFound, "ENTRY_NOT_FOUND",
		"Diesen Eintrag gibt es nicht.", map[string]any{"id": "abc"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	checkJSONHeader(t, rec)
	want := `{"error":{"code":"ENTRY_NOT_FOUND","message":"Diesen Eintrag gibt es nicht.","details":{"id":"abc"}}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestWriteError_WithoutDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "QUESTION_MISSING", "Bitte stelle eine Frage.", nil)

	if strings.Contains(rec.Body.String(), "details") {
		t.Errorf("empty details should be missing: %s", rec.Body.String())
	}
}

func TestWriteJSON_NoHTMLEscaping(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"value": "<0,5 & >1"})

	want := `{"value":"<0,5 & >1"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestWriteJSON_NotEncodable(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]any{"broken": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !json.Valid(rec.Body.Bytes()) || !strings.Contains(rec.Body.String(), CodeInternalError) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRecover_PanicBecomes500(t *testing.T) {
	h := withRecover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret state")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"INTERNAL_ERROR"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret state") {
		t.Error("the panic content must not be in the response")
	}
}

func TestRouter_WrongMethod(t *testing.T) {
	router := NewRouter(Dependencies{DB: fakePinger{}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/status", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func checkJSONHeader(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}
