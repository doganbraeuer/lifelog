package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/dienst"
	"lifelog/backend/internal/modell"
)

// hochladerAttrappe merkt sich den Upload und liefert eintrag bzw. err.
type hochladerAttrappe struct {
	eintrag    core.Entry
	err        error
	upload     *dienst.Upload
	textUpload *dienst.TextUpload
}

func (h *hochladerAttrappe) Ausfuehren(_ context.Context, u dienst.Upload) (core.Entry, error) {
	h.upload = &u
	return h.eintrag, h.err
}

func (h *hochladerAttrappe) TextAusfuehren(_ context.Context, u dienst.TextUpload) (core.Entry, error) {
	h.textUpload = &u
	return h.eintrag, h.err
}

// formular baut eine multipart-Anfrage. dateien: Dateiname → Inhalt.
func formular(t *testing.T, dateien map[string][]byte, felder map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, inhalt := range dateien {
		fw, err := mw.CreateFormFile("datei", name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(inhalt)
	}
	for k, v := range felder {
		mw.WriteField(k, v)
	}
	mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/eintraege", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func hochladen(t *testing.T, h *hochladerAttrappe, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Hochladen: h})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func vollerEintrag() core.Entry {
	z := func(s string) *string { return &s }
	return core.Entry{
		ID: "6727a1f3c2b4e5d6f7a8b9c0", Module: "gesundheit", Type: "laborbefund",
		DocumentDate: z("2026-10-14"),
		ImportedAt:   time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer:       z("Labor Muster GmbH, Salzburg"),
		FullText:     "Hämoglobin 14,2 g/dl",
		Facts: []core.Fact{
			{ID: "i1", Name: "Hämoglobin", Value: "14,2", Unit: z("g/dl")},
			{ID: "i2", Name: "Befundart", Value: "Blutbild klein"},
		},
		File: core.File{ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund.pdf",
			MimeType: "application/pdf", Size: 184320, Sha256: strings.Repeat("ab", 32)},
		Corrections: nil, // muss trotzdem als [] herauskommen
	}
}

func TestHochladen_Erfolg(t *testing.T) {
	h := &hochladerAttrappe{eintrag: vollerEintrag()}
	rec := hochladen(t, h, formular(t,
		map[string][]byte{"befund.pdf": []byte("%PDF-1.7")},
		map[string]string{"modul": "gesundheit"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status = %d, Body %s", rec.Code, rec.Body)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0" {
		t.Errorf("Location = %q", loc)
	}
	if h.upload == nil || string(h.upload.Daten) != "%PDF-1.7" || h.upload.Dateiname != "befund.pdf" || h.upload.Modul != "gesundheit" {
		t.Errorf("Upload an den Dienst = %+v", h.upload)
	}

	body := rec.Body.String()
	for _, muss := range []string{
		`"korrekturen":[]`,
		`"importdatum":"2026-11-03T18:42:07Z"`,
		`"mimeTyp":"application/pdf"`,
		`{"id":"i2","name":"Befundart","wert":"Blutbild klein"}`, // ohne einheit
		`"volltext":"Hämoglobin 14,2 g/dl"`,
	} {
		if !strings.Contains(body, muss) {
			t.Errorf("Antwort enthält %s nicht:\n%s", muss, body)
		}
	}
}

func TestHochladen_OhneModulIstLeer(t *testing.T) {
	h := &hochladerAttrappe{eintrag: vollerEintrag()}
	hochladen(t, h, formular(t, map[string][]byte{"a.pdf": []byte("%PDF-")}, nil))
	if h.upload == nil || h.upload.Modul != "" {
		t.Errorf("Modul = %+v (leer heißt Standard, das entscheidet der Dienst)", h.upload)
	}
}

func TestHochladen_Formularfehler(t *testing.T) {
	pdf := []byte("%PDF-")
	faelle := []struct {
		name    string
		anfrage func(t *testing.T) *http.Request
		status  int
		code    string
	}{
		{"kein multipart", func(t *testing.T) *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/eintraege", strings.NewReader(`{"datei":"x"}`))
			r.Header.Set("Content-Type", "application/json")
			return r
		}, 400, CodeUngueltigeAnfrage},
		{"ohne datei", func(t *testing.T) *http.Request {
			return formular(t, nil, map[string]string{"modul": "gesundheit"})
		}, 400, CodeDateiFehlt},
		{"datei und text", func(t *testing.T) *http.Request {
			return formular(t, map[string][]byte{"a.pdf": pdf}, map[string]string{"text": "Befund"})
		}, 400, CodeUngueltigeAnfrage},
		{"zwei Dateien", func(t *testing.T) *http.Request {
			return formular(t, map[string][]byte{"a.pdf": pdf, "b.pdf": pdf}, nil)
		}, 400, CodeUngueltigeAnfrage},
		{"Anfrage viel zu groß", func(t *testing.T) *http.Request {
			return formular(t, map[string][]byte{"gross.pdf": make([]byte, core.MaxFileBytes+2*formularPuffer)}, nil)
		}, 413, CodeDateiZuGross},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			h := &hochladerAttrappe{}
			rec := hochladen(t, h, f.anfrage(t))
			pruefeFehler(t, rec, f.status, f.code)
			if h.upload != nil || h.textUpload != nil {
				t.Error("bei einem Formularfehler darf der Dienst nicht aufgerufen werden")
			}
		})
	}
}

func TestHochladen_LeererTextZaehltNicht(t *testing.T) {
	// "Leer oder nur Leerzeichen gilt als fehlend" (API.md 3.1): datei + leerer text ist ok.
	h := &hochladerAttrappe{eintrag: vollerEintrag()}
	rec := hochladen(t, h, formular(t, map[string][]byte{"a.pdf": []byte("%PDF-")}, map[string]string{"text": "  "}))
	if rec.Code != http.StatusCreated {
		t.Errorf("Status = %d, Body %s", rec.Code, rec.Body)
	}
	if h.upload == nil || h.textUpload != nil {
		t.Error("muss als Datei-Upload laufen")
	}
}

func TestHochladen_FehlerDesDienstes(t *testing.T) {
	faelle := []struct {
		err     error
		status  int
		code    string
		details map[string]any
	}{
		{core.ErrUnknownModule, 400, CodeUngueltigeAnfrage, map[string]any{"feld": "modul", "wert": "sport"}},
		{core.ErrFileEmpty, 400, CodeDateiFehlt, nil},
		{core.ErrFileTooLarge, 413, CodeDateiZuGross, nil},
		{core.ErrUnsupportedFileType, 415, CodeDateitypNichtUnterstuetzt, nil},
		{&dienst.DuplikatFehler{EintragID: "6727a1f3c2b4e5d6f7a8b9c0"}, 409, CodeDuplikat,
			map[string]any{"eintragId": "6727a1f3c2b4e5d6f7a8b9c0"}},
		{core.ErrUnreadable, 422, CodeNichtLesbar, nil},
		{&core.ExtractionError{Field: "typ", Reason: "x"}, 502, CodeAuslesenFehlgeschlagen, nil},
		{fmt.Errorf("JSON kaputt: %w", modell.ErrAntwortUngueltig), 502, CodeAuslesenFehlgeschlagen, nil},
		{fmt.Errorf("Gemini HTTP 503: %w", modell.ErrNichtErreichbar), 503, CodeModellNichtErreichbar, nil},
		{errors.New("Datenbank weg"), 500, CodeInternerFehler, nil},
	}
	for _, f := range faelle {
		t.Run(f.code, func(t *testing.T) {
			h := &hochladerAttrappe{err: f.err}
			rec := hochladen(t, h, formular(t,
				map[string][]byte{"a.pdf": []byte("%PDF-")}, map[string]string{"modul": "sport"}))

			a := pruefeFehler(t, rec, f.status, f.code)
			if f.details != nil && fmt.Sprint(a.Fehler.Details) != fmt.Sprint(f.details) {
				t.Errorf("details = %v, erwartet %v", a.Fehler.Details, f.details)
			}
			// Interne Ursachen (z. B. Gemini-Meldungen) gehören nicht in die Antwort.
			for _, intern := range []string{"Datenbank weg", "Gemini", "JSON kaputt"} {
				if strings.Contains(rec.Body.String(), intern) {
					t.Errorf("interne Ursache %q in der Antwort: %s", intern, rec.Body)
				}
			}
		})
	}
}

func TestHochladen_Abbruch(t *testing.T) {
	h := &hochladerAttrappe{err: context.Canceled}
	rec := hochladen(t, h, formular(t, map[string][]byte{"a.pdf": []byte("%PDF-")}, nil))
	// Die App ist weg; es wird nichts Sinnvolles mehr geschrieben.
	if rec.Body.Len() != 0 {
		t.Errorf("Body nach Abbruch: %s", rec.Body)
	}
}

func pruefeFehler(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) fehlerAntwort {
	t.Helper()
	if rec.Code != status {
		t.Errorf("Status = %d, erwartet %d (Body %s)", rec.Code, status, rec.Body)
	}
	var a fehlerAntwort
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("kein Fehler-JSON: %v (%s)", err, rec.Body)
	}
	if a.Fehler.Code != code || a.Fehler.Nachricht == "" {
		t.Errorf("Fehler = %+v, erwartet Code %s", a.Fehler, code)
	}
	return a
}
