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

// speicherAttrappe merkt sich, mit welchem Modul gefragt wurde.
type speicherAttrappe struct {
	liste        []core.EntrySummary
	err          error
	aufrufe      int
	gefragtModul string

	eintraege map[string]core.Entry // ID → Eintrag (für GET /eintraege/{id})
	dateien   map[string][]byte     // Datei-ID → Inhalt (für …/datei)
	dateiErr  error                 // wenn gesetzt, scheitert DateiOeffnen damit
}

func (s *speicherAttrappe) ListSummaries(_ context.Context, modul string) ([]core.EntrySummary, error) {
	s.aufrufe++
	s.gefragtModul = modul
	return slices.Clone(s.liste), s.err
}

func (s *speicherAttrappe) GetEntry(_ context.Context, id string) (core.Entry, bool, error) {
	s.aufrufe++
	if s.err != nil {
		return core.Entry{}, false, s.err
	}
	e, ok := s.eintraege[id]
	return e, ok, nil
}

func (s *speicherAttrappe) OpenFile(_ context.Context, dateiID string) (io.ReadCloser, int64, error) {
	if s.dateiErr != nil {
		return nil, 0, s.dateiErr
	}
	d, ok := s.dateien[dateiID]
	if !ok {
		return nil, 0, core.ErrOriginalMissing
	}
	return io.NopCloser(bytes.NewReader(d)), int64(len(d)), nil
}

func kurz(id, datum string, importMinute int) core.EntrySummary {
	k := core.EntrySummary{
		ID: id, Module: "gesundheit", Type: "laborbefund",
		ImportedAt: time.Date(2026, 11, 3, 18, importMinute, 7, 0, time.UTC),
	}
	if datum != "" {
		k.DocumentDate = &datum
	}
	return k
}

func liste(t *testing.T, s *speicherAttrappe, query string) *httptest.ResponseRecorder {
	t.Helper()
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Eintraege: s})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/eintraege"+query, nil))
	return rec
}

func TestListe_LeerIstLeeresArray(t *testing.T) {
	// Die Attrappe liefert nil, so wie ein Speicher ohne Treffer es könnte.
	rec := liste(t, &speicherAttrappe{liste: nil}, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d", rec.Code)
	}
	want := `{"eintraege":[],"anzahl":0}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("Body = %s, erwartet %s", got, want)
	}
}

func TestListe_StandardwerteUndFormat(t *testing.T) {
	s := &speicherAttrappe{liste: []core.EntrySummary{
		kurz("aaaaaaaaaaaaaaaaaaaaaaaa", "", 30),
		kurz("bbbbbbbbbbbbbbbbbbbbbbbb", "2026-09-02", 20),
		kurz("cccccccccccccccccccccccc", "2026-10-14", 10),
	}}
	rec := liste(t, s, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, Body %s", rec.Code, rec.Body)
	}
	pruefeJSONHeader(t, rec)
	if s.gefragtModul != "gesundheit" {
		t.Errorf("Modul = %q, erwartet gesundheit als Standard", s.gefragtModul)
	}

	var a struct {
		Eintraege []map[string]any `json:"eintraege"`
		Anzahl    int              `json:"anzahl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Anzahl != 3 || len(a.Eintraege) != 3 {
		t.Fatalf("anzahl = %d, Einträge = %d", a.Anzahl, len(a.Eintraege))
	}

	// Standard: Dokumentdatum absteigend, ohne Datum am Ende.
	var reihenfolge []string
	for _, e := range a.Eintraege {
		reihenfolge = append(reihenfolge, e["id"].(string)[:1])
	}
	if !slices.Equal(reihenfolge, []string{"c", "b", "a"}) {
		t.Errorf("Reihenfolge = %v, erwartet [c b a]", reihenfolge)
	}

	erster := a.Eintraege[0]
	if erster["importdatum"] != "2026-11-03T18:10:07Z" {
		t.Errorf("importdatum = %v", erster["importdatum"])
	}
	if erster["dokumentdatum"] != "2026-10-14" {
		t.Errorf("dokumentdatum = %v", erster["dokumentdatum"])
	}

	// null bleibt als null sichtbar, das Feld fehlt nicht.
	letzter := a.Eintraege[2]
	for _, feld := range []string{"dokumentdatum", "quelle"} {
		if v, ok := letzter[feld]; !ok || v != nil {
			t.Errorf("%s = %v (vorhanden: %v), erwartet null", feld, v, ok)
		}
	}
	// Die Kurzform enthält keine Detailfelder.
	for _, feld := range []string{"volltext", "infos", "datei", "korrekturen"} {
		if _, ok := erster[feld]; ok {
			t.Errorf("Kurzform enthält %s", feld)
		}
	}
}

func TestListe_ParameterWerdenAngewendet(t *testing.T) {
	s := &speicherAttrappe{liste: []core.EntrySummary{
		kurz("aaaaaaaaaaaaaaaaaaaaaaaa", "2026-10-14", 30),
		kurz("bbbbbbbbbbbbbbbbbbbbbbbb", "2026-09-02", 20),
	}}
	rec := liste(t, s, "?modul=gesundheit&sortierung=importdatum&richtung=aufsteigend")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"eintraege":[{"id":"bbbb`) {
		t.Errorf("b (Import :20) muss zuerst kommen: %s", rec.Body)
	}
}

func TestListe_UngueltigeParameter(t *testing.T) {
	faelle := []struct {
		query, parameter, wert string
	}{
		{"?sortierung=quelle", "sortierung", "quelle"},
		{"?richtung=desc", "richtung", "desc"},
		{"?modul=sport", "modul", "sport"},
		{"?modul=Gesundheit", "modul", "Gesundheit"},
	}
	for _, f := range faelle {
		t.Run(f.query, func(t *testing.T) {
			s := &speicherAttrappe{}
			rec := liste(t, s, f.query)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("Status = %d, erwartet 400", rec.Code)
			}
			var a fehlerAntwort
			if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			if a.Fehler.Code != CodeUngueltigerParameter {
				t.Errorf("code = %q", a.Fehler.Code)
			}
			if a.Fehler.Details["parameter"] != f.parameter || a.Fehler.Details["wert"] != f.wert {
				t.Errorf("details = %v", a.Fehler.Details)
			}
			if a.Fehler.Nachricht == "" {
				t.Error("nachricht fehlt")
			}
			if s.aufrufe != 0 {
				t.Error("bei ungültigen Parametern darf die Datenbank nicht gefragt werden")
			}
		})
	}
}

func TestListe_Datenbankfehler(t *testing.T) {
	rec := liste(t, &speicherAttrappe{err: errors.New("verbindung zu cluster0 verloren")}, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("Status = %d, erwartet 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), CodeInternerFehler) {
		t.Errorf("Body = %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "cluster0") {
		t.Error("interne Ursache darf nicht in der Antwort stehen")
	}
}
