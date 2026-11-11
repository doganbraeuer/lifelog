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

type loescherAttrappe struct {
	err error
	ids []string
}

func (l *loescherAttrappe) Ausfuehren(_ context.Context, id string) error {
	l.ids = append(l.ids, id)
	return l.err
}

func loeschen(t *testing.T, l *loescherAttrappe, id string) *httptest.ResponseRecorder {
	t.Helper()
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Eintraege: speicherMitEintrag(), Loeschen: l})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/eintraege/"+id, nil))
	return rec
}

func TestLoeschen_204OhneBody(t *testing.T) {
	l := &loescherAttrappe{}
	rec := loeschen(t, l, testID)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Status %d, erwartet 204 (%s)", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("Body bei 204: %q", rec.Body)
	}
	if len(l.ids) != 1 || l.ids[0] != testID {
		t.Errorf("Dienst bekam %v, erwartet [%s]", l.ids, testID)
	}
}

func TestLoeschen_Fehler(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"ungültige ID", core.ErrInvalidID, http.StatusBadRequest, CodeUngueltigeID},
		{"nicht gefunden", core.ErrEntryNotFound, http.StatusNotFound, CodeEintragNichtGefunden},
		{"Datenbank", errors.New("Abschnitte löschen: Verbindung weg"), http.StatusInternalServerError, CodeInternerFehler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := loeschen(t, &loescherAttrappe{err: tc.err}, testID)
			a := pruefeFehler(t, rec, tc.status, tc.code)
			pruefeJSONHeader(t, rec)
			if a.Fehler.Details != nil {
				t.Errorf("Details: %v", a.Fehler.Details)
			}
			// Interne Fehlermeldungen gehen nur ins Log, nicht an die App.
			if strings.Contains(rec.Body.String(), "Verbindung weg") {
				t.Errorf("interner Fehler in der Antwort: %s", rec.Body)
			}
		})
	}
}

// Die ID prüft der Dienst; der Handler reicht sie unverändert durch.
func TestLoeschen_UngueltigeIDAusDemPfad(t *testing.T) {
	l := &loescherAttrappe{err: core.ErrInvalidID}
	rec := loeschen(t, l, "abc")
	pruefeFehler(t, rec, http.StatusBadRequest, CodeUngueltigeID)
	if len(l.ids) != 1 || l.ids[0] != "abc" {
		t.Errorf("Dienst bekam %v", l.ids)
	}
}

func TestLoeschen_Abbruch(t *testing.T) {
	rec := loeschen(t, &loescherAttrappe{err: context.Canceled}, testID)
	if rec.Body.Len() != 0 {
		t.Errorf("Body nach Abbruch: %s", rec.Body)
	}
}

// Die neue Route darf GET und PATCH auf demselben Pfad nicht stören, und
// DELETE auf /datei gibt es nicht.
func TestLoeschen_AndereMethodenUnberuehrt(t *testing.T) {
	l := &loescherAttrappe{}
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Eintraege: speicherMitEintrag(),
		Korrigieren: &korrigiererAttrappe{err: core.ErrEntryNotFound}, Loeschen: l})
	for _, tc := range []struct {
		methode, pfad string
		status        int
	}{
		{http.MethodGet, "/api/v1/eintraege/" + testID, http.StatusOK},
		{http.MethodPatch, "/api/v1/eintraege/" + testID, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/eintraege/" + testID + "/datei", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/v1/eintraege", http.StatusMethodNotAllowed},
	} {
		body := strings.NewReader(`{"typ": "arztbrief"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.methode, tc.pfad, body))
		if rec.Code != tc.status {
			t.Errorf("%s %s: Status %d, erwartet %d", tc.methode, tc.pfad, rec.Code, tc.status)
		}
	}
	if len(l.ids) != 0 {
		t.Errorf("Löschen aufgerufen: %v", l.ids)
	}
}
