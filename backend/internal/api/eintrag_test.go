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
	testID      = "6727a1f3c2b4e5d6f7a8b9c0"
	testDateiID = "6727a1f3c2b4e5d6f7a8b9c1"
)

func speicherMitEintrag() *speicherAttrappe {
	e := vollerEintrag() // aus hochladen_test.go, ID testID, Datei testDateiID
	return &speicherAttrappe{
		eintraege: map[string]core.Entry{testID: e},
		dateien:   map[string][]byte{testDateiID: []byte("%PDF-1.7 synthetisch")},
	}
}

func abrufen(t *testing.T, s *speicherAttrappe, pfad string) *httptest.ResponseRecorder {
	t.Helper()
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Eintraege: s})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, pfad, nil))
	return rec
}

func TestEintrag_Gefunden(t *testing.T) {
	rec := abrufen(t, speicherMitEintrag(), "/api/v1/eintraege/"+testID)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, Body %s", rec.Code, rec.Body)
	}
	pruefeJSONHeader(t, rec)
	var a map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	// Vollständiger Eintrag: alle Felder aus API.md Abschnitt 2 vorhanden.
	for _, feld := range []string{"id", "modul", "typ", "dokumentdatum", "importdatum", "quelle", "volltext", "infos", "datei", "korrekturen"} {
		if _, ok := a[feld]; !ok {
			t.Errorf("Feld %s fehlt", feld)
		}
	}
	if k, ok := a["korrekturen"].([]any); !ok || len(k) != 0 {
		t.Errorf("korrekturen = %#v, erwartet []", a["korrekturen"])
	}
}

func TestEintrag_Fehler(t *testing.T) {
	faelle := []struct {
		name, pfad string
		speicher   *speicherAttrappe
		status     int
		code       string
	}{
		{"ungültige ID", "/api/v1/eintraege/abc", speicherMitEintrag(), 400, CodeUngueltigeID},
		{"ID mit Nicht-Hex", "/api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9zz", speicherMitEintrag(), 400, CodeUngueltigeID},
		{"unbekannt", "/api/v1/eintraege/000000000000000000000000", speicherMitEintrag(), 404, CodeEintragNichtGefunden},
		{"Datenbankfehler", "/api/v1/eintraege/" + testID, &speicherAttrappe{err: errors.New("weg")}, 500, CodeInternerFehler},
		{"Datei: ungültige ID", "/api/v1/eintraege/abc/datei", speicherMitEintrag(), 400, CodeUngueltigeID},
		{"Datei: unbekannt", "/api/v1/eintraege/000000000000000000000000/datei", speicherMitEintrag(), 404, CodeEintragNichtGefunden},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			rec := abrufen(t, f.speicher, f.pfad)
			pruefeFehler(t, rec, f.status, f.code)
		})
	}
}

func TestEintrag_UngueltigeIDFragtDatenbankNicht(t *testing.T) {
	s := speicherMitEintrag()
	abrufen(t, s, "/api/v1/eintraege/abc")
	if s.aufrufe != 0 {
		t.Error("bei ungültiger ID darf die Datenbank nicht gefragt werden")
	}
}

func TestDatei_Gefunden(t *testing.T) {
	rec := abrufen(t, speicherMitEintrag(), "/api/v1/eintraege/"+testID+"/datei")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, Body %s", rec.Code, rec.Body)
	}
	if rec.Body.String() != "%PDF-1.7 synthetisch" {
		t.Errorf("Body = %q", rec.Body)
	}
	h := rec.Header()
	erwartet := map[string]string{
		"Content-Type":           "application/pdf",
		"Content-Length":         "20",
		"Content-Disposition":    `inline; filename=befund.pdf`,
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "private, no-store",
	}
	for k, v := range erwartet {
		if h.Get(k) != v {
			t.Errorf("%s = %q, erwartet %q", k, h.Get(k), v)
		}
	}
}

func TestDatei_TextImport(t *testing.T) {
	s := speicherMitEintrag()
	e := s.eintraege[testID]
	e.File.FileName, e.File.MimeType = "text.txt", "text/plain; charset=utf-8"
	s.eintraege[testID] = e

	rec := abrufen(t, s, "/api/v1/eintraege/"+testID+"/datei")
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != "inline; filename=text.txt" {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestDatei_FehltIst500(t *testing.T) {
	s := speicherMitEintrag()
	s.dateien = map[string][]byte{} // Eintrag da, Datei weg
	rec := abrufen(t, s, "/api/v1/eintraege/"+testID+"/datei")
	pruefeFehler(t, rec, 500, CodeInternerFehler)
}

func TestInlineMitName(t *testing.T) {
	faelle := []string{
		"befund.pdf",
		"Befund Ä Ö ü.pdf",
		`böser"name;.pdf`,
		"zeile\r\nX-Header: x.pdf",
		"",
	}
	for _, name := range faelle {
		wert := inlineMitName(name)
		if strings.ContainsAny(wert, "\r\n") {
			t.Errorf("%q: Header enthält Zeilenumbruch: %q", name, wert)
			continue
		}
		// Der Wert muss sich zurücklesen lassen und den Namen unverändert enthalten.
		art, params, err := mime.ParseMediaType(wert)
		if err != nil || art != "inline" {
			t.Errorf("%q → %q: nicht lesbar (%v)", name, wert, err)
			continue
		}
		if params["filename"] != name && !(name == "" || strings.ContainsAny(name, "\r\n")) {
			t.Errorf("%q → %q: filename = %q", name, wert, params["filename"])
		}
	}
}
