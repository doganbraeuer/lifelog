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

// pingerAttrappe ersetzt MongoDB im Test.
type pingerAttrappe struct{ err error }

func (p pingerAttrappe) Ping(context.Context) error { return p.err }

func TestStatus_DatenbankErreichbar(t *testing.T) {
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	pruefeJSONHeader(t, rec)
	var a statusAntwort
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v", err)
	}
	if a.Status != "ok" || a.Datenbank != "ok" {
		t.Errorf("Antwort = %+v", a)
	}
}

func TestStatus_DatenbankNichtErreichbar(t *testing.T) {
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{err: errors.New("Zeitüberschreitung")}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Status = %d, erwartet 503", rec.Code)
	}
	want := `{"status":"fehler","datenbank":"nicht erreichbar"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("Body = %s, erwartet %s", got, want)
	}
	if strings.Contains(rec.Body.String(), "Zeitüberschreitung") {
		t.Error("interne Fehlerursache darf nicht in der Antwort stehen")
	}
}

func TestSchreibeFehler_Format(t *testing.T) {
	rec := httptest.NewRecorder()
	schreibeFehler(rec, http.StatusNotFound, "EINTRAG_NICHT_GEFUNDEN",
		"Diesen Eintrag gibt es nicht.", map[string]any{"id": "abc"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Status = %d", rec.Code)
	}
	pruefeJSONHeader(t, rec)
	want := `{"fehler":{"code":"EINTRAG_NICHT_GEFUNDEN","nachricht":"Diesen Eintrag gibt es nicht.","details":{"id":"abc"}}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("Body =\n%s\nerwartet\n%s", got, want)
	}
}

func TestSchreibeFehler_OhneDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	schreibeFehler(rec, http.StatusBadRequest, "FRAGE_FEHLT", "Bitte stelle eine Frage.", nil)

	if strings.Contains(rec.Body.String(), "details") {
		t.Errorf("details ohne Inhalt soll fehlen: %s", rec.Body.String())
	}
}

func TestSchreibeJSON_KeinHTMLEscaping(t *testing.T) {
	rec := httptest.NewRecorder()
	schreibeJSON(rec, http.StatusOK, map[string]string{"wert": "<0,5 & >1"})

	want := `{"wert":"<0,5 & >1"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("Body = %s, erwartet %s", got, want)
	}
}

func TestSchreibeJSON_NichtUmwandelbar(t *testing.T) {
	rec := httptest.NewRecorder()
	schreibeJSON(rec, http.StatusOK, map[string]any{"kaputt": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("Status = %d, erwartet 500", rec.Code)
	}
	if !json.Valid(rec.Body.Bytes()) || !strings.Contains(rec.Body.String(), CodeInternerFehler) {
		t.Errorf("Body = %s", rec.Body.String())
	}
}

func TestAbfangen_PanicWird500(t *testing.T) {
	h := mitAbfangen(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("geheimer Zustand")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("Status = %d, erwartet 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"INTERNER_FEHLER"`) {
		t.Errorf("Body = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "geheimer Zustand") {
		t.Error("panic-Inhalt darf nicht in der Antwort stehen")
	}
}

func TestRouter_FalscheMethode(t *testing.T) {
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/status", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status = %d, erwartet 405", rec.Code)
	}
}

func pruefeJSONHeader(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}
