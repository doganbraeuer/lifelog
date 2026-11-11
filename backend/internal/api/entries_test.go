package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// fakeStore remembers with which module it was asked.
type fakeStore struct {
	list        []core.EntrySummary
	err         error
	calls       int
	askedModule string

	entries map[string]core.Entry // ID → entry (for GET /eintraege/{id})
	files   map[string][]byte     // file ID → content (for …/datei)
	fileErr error                 // if set, OpenFile fails with it
}

func (s *fakeStore) ListSummaries(_ context.Context, module string) ([]core.EntrySummary, error) {
	s.calls++
	s.askedModule = module
	return slices.Clone(s.list), s.err
}

func (s *fakeStore) GetEntry(_ context.Context, id string) (core.Entry, bool, error) {
	s.calls++
	if s.err != nil {
		return core.Entry{}, false, s.err
	}
	e, ok := s.entries[id]
	return e, ok, nil
}

func (s *fakeStore) OpenFile(_ context.Context, fileID string) (io.ReadCloser, int64, error) {
	if s.fileErr != nil {
		return nil, 0, s.fileErr
	}
	d, ok := s.files[fileID]
	if !ok {
		return nil, 0, core.ErrOriginalMissing
	}
	return io.NopCloser(bytes.NewReader(d)), int64(len(d)), nil
}

func summary(id, date string, importMinute int) core.EntrySummary {
	s := core.EntrySummary{
		ID: id, Module: "gesundheit", Type: "laborbefund",
		ImportedAt: time.Date(2026, 11, 3, 18, importMinute, 7, 0, time.UTC),
	}
	if date != "" {
		s.DocumentDate = &date
	}
	return s
}

func list(t *testing.T, s *fakeStore, query string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Dependencies{DB: fakePinger{}, Entries: s})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/eintraege"+query, nil))
	return rec
}

func TestList_EmptyIsEmptyArray(t *testing.T) {
	// The fake returns nil, as a store without hits could.
	rec := list(t, &fakeStore{list: nil}, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	want := `{"eintraege":[],"anzahl":0}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestList_DefaultsAndFormat(t *testing.T) {
	s := &fakeStore{list: []core.EntrySummary{
		summary("aaaaaaaaaaaaaaaaaaaaaaaa", "", 30),
		summary("bbbbbbbbbbbbbbbbbbbbbbbb", "2026-09-02", 20),
		summary("cccccccccccccccccccccccc", "2026-10-14", 10),
	}}
	rec := list(t, s, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	checkJSONHeader(t, rec)
	if s.askedModule != "gesundheit" {
		t.Errorf("module = %q, want gesundheit as default", s.askedModule)
	}

	var a struct {
		Entries []map[string]any `json:"eintraege"`
		Count   int              `json:"anzahl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Count != 3 || len(a.Entries) != 3 {
		t.Fatalf("count = %d, entries = %d", a.Count, len(a.Entries))
	}

	// Default: document date descending, without date at the end.
	var order []string
	for _, e := range a.Entries {
		order = append(order, e["id"].(string)[:1])
	}
	if !slices.Equal(order, []string{"c", "b", "a"}) {
		t.Errorf("order = %v, want [c b a]", order)
	}

	first := a.Entries[0]
	if first["importdatum"] != "2026-11-03T18:10:07Z" {
		t.Errorf("importdatum = %v", first["importdatum"])
	}
	if first["dokumentdatum"] != "2026-10-14" {
		t.Errorf("dokumentdatum = %v", first["dokumentdatum"])
	}

	// null stays visible as null; the field is not missing.
	last := a.Entries[2]
	for _, field := range []string{"dokumentdatum", "quelle"} {
		if v, ok := last[field]; !ok || v != nil {
			t.Errorf("%s = %v (present: %v), want null", field, v, ok)
		}
	}
	// The short form contains no detail fields.
	for _, field := range []string{"volltext", "infos", "datei", "korrekturen"} {
		if _, ok := first[field]; ok {
			t.Errorf("short form contains %s", field)
		}
	}
}

func TestList_ParametersAreApplied(t *testing.T) {
	s := &fakeStore{list: []core.EntrySummary{
		summary("aaaaaaaaaaaaaaaaaaaaaaaa", "2026-10-14", 30),
		summary("bbbbbbbbbbbbbbbbbbbbbbbb", "2026-09-02", 20),
	}}
	rec := list(t, s, "?modul=gesundheit&sortierung=importdatum&richtung=aufsteigend")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"eintraege":[{"id":"bbbb`) {
		t.Errorf("b (import :20) must come first: %s", rec.Body)
	}
}

func TestList_InvalidParameters(t *testing.T) {
	cases := []struct {
		query, parameter, value string
	}{
		{"?sortierung=quelle", "sortierung", "quelle"},
		{"?richtung=desc", "richtung", "desc"},
		{"?modul=sport", "modul", "sport"},
		{"?modul=Gesundheit", "modul", "Gesundheit"},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			s := &fakeStore{}
			rec := list(t, s, c.query)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			var a errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			if a.Error.Code != CodeInvalidParameter {
				t.Errorf("code = %q", a.Error.Code)
			}
			if a.Error.Details["parameter"] != c.parameter || a.Error.Details["wert"] != c.value {
				t.Errorf("details = %v", a.Error.Details)
			}
			if a.Error.Message == "" {
				t.Error("message missing")
			}
			if s.calls != 0 {
				t.Error("the database must not be asked for invalid parameters")
			}
		})
	}
}

func TestList_DatabaseError(t *testing.T) {
	rec := list(t, &fakeStore{err: errors.New("connection to cluster0 lost")}, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), CodeInternalError) {
		t.Errorf("body = %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "cluster0") {
		t.Error("the internal cause must not be in the response")
	}
}
