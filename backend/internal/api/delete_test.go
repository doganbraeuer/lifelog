package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

type fakeDeleter struct {
	err error
	ids []string
}

func (d *fakeDeleter) Delete(_ context.Context, id string) error {
	d.ids = append(d.ids, id)
	return d.err
}

func deleteEntry(t *testing.T, d *fakeDeleter, id string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Dependencies{DB: fakePinger{}, Entries: storeWithEntry(), Deleter: d})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/entries/"+id, nil))
	return rec
}

func TestDelete_204WithoutBody(t *testing.T) {
	d := &fakeDeleter{}
	rec := deleteEntry(t, d, testID)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204 (%s)", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body with 204: %q", rec.Body)
	}
	if len(d.ids) != 1 || d.ids[0] != testID {
		t.Errorf("service got %v, want [%s]", d.ids, testID)
	}
}

func TestDelete_Errors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid ID", core.ErrInvalidID, http.StatusBadRequest, CodeInvalidID},
		{"not found", core.ErrEntryNotFound, http.StatusNotFound, CodeEntryNotFound},
		{"database", errors.New("delete chunks: connection gone"), http.StatusInternalServerError, CodeInternalError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := deleteEntry(t, &fakeDeleter{err: tc.err}, testID)
			a := checkError(t, rec, tc.status, tc.code)
			checkJSONHeader(t, rec)
			if a.Error.Details != nil {
				t.Errorf("details: %v", a.Error.Details)
			}
			// Internal error messages only go into the log, not to the app.
			if strings.Contains(rec.Body.String(), "connection gone") {
				t.Errorf("internal error in the response: %s", rec.Body)
			}
		})
	}
}

// The service checks the ID; the handler passes it through unchanged.
func TestDelete_InvalidIDFromPath(t *testing.T) {
	d := &fakeDeleter{err: core.ErrInvalidID}
	rec := deleteEntry(t, d, "abc")
	checkError(t, rec, http.StatusBadRequest, CodeInvalidID)
	if len(d.ids) != 1 || d.ids[0] != "abc" {
		t.Errorf("service got %v", d.ids)
	}
}

func TestDelete_Canceled(t *testing.T) {
	rec := deleteEntry(t, &fakeDeleter{err: context.Canceled}, testID)
	if rec.Body.Len() != 0 {
		t.Errorf("body after cancel: %s", rec.Body)
	}
}

// The new route must not disturb GET and PATCH on the same path, and there
// is no DELETE on /file.
func TestDelete_OtherMethodsUnaffected(t *testing.T) {
	d := &fakeDeleter{}
	router := NewRouter(Dependencies{DB: fakePinger{}, Entries: storeWithEntry(),
		Corrector: &fakeCorrector{err: core.ErrEntryNotFound}, Deleter: d})
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/api/v1/entries/" + testID, http.StatusOK},
		{http.MethodPatch, "/api/v1/entries/" + testID, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/entries/" + testID + "/file", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/v1/entries", http.StatusMethodNotAllowed},
	} {
		body := strings.NewReader(`{"type": "arztbrief"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, body))
		if rec.Code != tc.status {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, rec.Code, tc.status)
		}
	}
	if len(d.ids) != 0 {
		t.Errorf("delete called: %v", d.ids)
	}
}
