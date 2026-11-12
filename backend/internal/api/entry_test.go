package api

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

const (
	testID     = "6727a1f3c2b4e5d6f7a8b9c0"
	testFileID = "6727a1f3c2b4e5d6f7a8b9c1"
)

func storeWithEntry() *fakeStore {
	e := fullEntry() // from upload_test.go, ID testID, file testFileID
	return &fakeStore{
		entries: map[string]core.Entry{testID: e},
		files:   map[string][]byte{testFileID: []byte("%PDF-1.7 synthetisch")},
	}
}

func get(t *testing.T, s *fakeStore, path string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Dependencies{DB: fakePinger{}, Entries: s})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestEntry_Found(t *testing.T) {
	rec := get(t, storeWithEntry(), "/api/v1/entries/"+testID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	checkJSONHeader(t, rec)
	var a map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	// Full entry: all fields from API.md section 2 are present.
	for _, field := range []string{"id", "module", "type", "documentDate", "importedAt", "issuer", "fullText", "facts", "file", "corrections", "verification"} {
		if _, ok := a[field]; !ok {
			t.Errorf("field %s missing", field)
		}
	}
	if c, ok := a["corrections"].([]any); !ok || len(c) != 0 {
		t.Errorf("corrections = %#v, want []", a["corrections"])
	}
}

func TestEntry_Errors(t *testing.T) {
	cases := []struct {
		name, path string
		store      *fakeStore
		status     int
		code       string
	}{
		{"invalid ID", "/api/v1/entries/abc", storeWithEntry(), 400, CodeInvalidID},
		{"ID with non-hex", "/api/v1/entries/6727a1f3c2b4e5d6f7a8b9zz", storeWithEntry(), 400, CodeInvalidID},
		{"unknown", "/api/v1/entries/000000000000000000000000", storeWithEntry(), 404, CodeEntryNotFound},
		{"database error", "/api/v1/entries/" + testID, &fakeStore{err: errors.New("gone")}, 500, CodeInternalError},
		{"file: invalid ID", "/api/v1/entries/abc/file", storeWithEntry(), 400, CodeInvalidID},
		{"file: unknown", "/api/v1/entries/000000000000000000000000/file", storeWithEntry(), 404, CodeEntryNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, c.store, c.path)
			checkError(t, rec, c.status, c.code)
		})
	}
}

func TestEntry_InvalidIDDoesNotAskDatabase(t *testing.T) {
	s := storeWithEntry()
	get(t, s, "/api/v1/entries/abc")
	if s.calls != 0 {
		t.Error("the database must not be asked for an invalid ID")
	}
}

func TestFile_Found(t *testing.T) {
	rec := get(t, storeWithEntry(), "/api/v1/entries/"+testID+"/file")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if rec.Body.String() != "%PDF-1.7 synthetisch" {
		t.Errorf("body = %q", rec.Body)
	}
	h := rec.Header()
	want := map[string]string{
		"Content-Type":           "application/pdf",
		"Content-Length":         "20",
		"Content-Disposition":    `inline; filename=befund.pdf`,
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "private, no-store",
	}
	for k, v := range want {
		if h.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, h.Get(k), v)
		}
	}
}

func TestFile_TextImport(t *testing.T) {
	s := storeWithEntry()
	e := s.entries[testID]
	e.File.FileName, e.File.MimeType = "text.txt", "text/plain; charset=utf-8"
	s.entries[testID] = e

	rec := get(t, s, "/api/v1/entries/"+testID+"/file")
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != "inline; filename=text.txt" {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestFile_MissingIs500(t *testing.T) {
	s := storeWithEntry()
	s.files = map[string][]byte{} // entry there, file gone
	rec := get(t, s, "/api/v1/entries/"+testID+"/file")
	checkError(t, rec, 500, CodeInternalError)
}

func TestInlineWithName(t *testing.T) {
	cases := []string{
		"befund.pdf",
		"Befund Ä Ö ü.pdf",
		`böser"name;.pdf`,
		"zeile\r\nX-Header: x.pdf",
		"",
	}
	for _, name := range cases {
		value := inlineWithName(name)
		if strings.ContainsAny(value, "\r\n") {
			t.Errorf("%q: header contains a line break: %q", name, value)
			continue
		}
		// The value must be readable again and contain the name unchanged.
		disposition, params, err := mime.ParseMediaType(value)
		if err != nil || disposition != "inline" {
			t.Errorf("%q → %q: not readable (%v)", name, value, err)
			continue
		}
		if params["filename"] != name && !(name == "" || strings.ContainsAny(name, "\r\n")) {
			t.Errorf("%q → %q: filename = %q", name, value, params["filename"])
		}
	}
}

// An entry without verification (seed data, old entries) shows
// unchecked/not_run with the full shape: nulls and empty lists.
func TestEntry_VerificationNotRun(t *testing.T) {
	rec := get(t, storeWithEntry(), "/api/v1/entries/"+testID)
	var a struct {
		Verification map[string]any `json:"verification"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	v := a.Verification
	if v["status"] != "unchecked" || v["reason"] != "not_run" {
		t.Errorf("verification %v", v)
	}
	for _, key := range []string{"source", "engine", "checkedAt", "matchPercent"} {
		if val, ok := v[key]; !ok || val != nil {
			t.Errorf("%s = %v (present %v), want null", key, val, ok)
		}
	}
	if f, ok := v["fields"].([]any); !ok || len(f) != 0 {
		t.Errorf("fields = %#v, want []", v["fields"])
	}
}

// A location in a text import is a range, without page and rectangle.
func TestLocationToJSON_TextRange(t *testing.T) {
	b, err := json.Marshal(locationToJSON(core.Location{Start: 0, End: 11}))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"start":0,"end":11}` {
		t.Errorf("got %s", b)
	}
}
