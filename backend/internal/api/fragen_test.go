package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

type fragerAttrappe struct {
	antwort      core.Answer
	err          error
	frage, modul string
	aufgerufen   bool
}

func (f *fragerAttrappe) Ask(_ context.Context, frage, modul string) (core.Answer, error) {
	f.aufgerufen, f.frage, f.modul = true, frage, modul
	return f.antwort, f.err
}

func fragen(t *testing.T, f *fragerAttrappe, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Fragen: f})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/fragen", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func TestFragen_Treffer(t *testing.T) {
	z := func(s string) *string { return &s }
	f := &fragerAttrappe{antwort: core.Answer{
		Text: "Im Laborbefund steht Hämoglobin 14,2 g/dl [1].", Found: true,
		Sources: []core.Source{{Number: 1, EntryID: "6727a1f3c2b4e5d6f7a8b9c0", Module: "gesundheit",
			Type: "laborbefund", Issuer: z("Labor Muster GmbH, Salzburg"), DocumentDate: z("2026-10-14"),
			Excerpt: "Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)"}},
	}}
	rec := fragen(t, f, `{"frage": "Welchen Hämoglobinwert hatte ich im Oktober?", "modul": "gesundheit"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, Body %s", rec.Code, rec.Body)
	}
	if f.frage != "Welchen Hämoglobinwert hatte ich im Oktober?" || f.modul != "gesundheit" {
		t.Errorf("an den Dienst: %q / %q", f.frage, f.modul)
	}
	// Genau die Form aus API.md 3.6.
	want := `{"antwort":"Im Laborbefund steht Hämoglobin 14,2 g/dl [1].","gefunden":true,"quellen":[` +
		`{"nr":1,"eintragId":"6727a1f3c2b4e5d6f7a8b9c0","modul":"gesundheit","typ":"laborbefund",` +
		`"quelle":"Labor Muster GmbH, Salzburg","dokumentdatum":"2026-10-14","auszug":"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("Body =\n%s\nerwartet\n%s", got, want)
	}
}

func TestFragen_KeinTrefferIstLeereListe(t *testing.T) {
	// Die App schickt kein modul mit; das ist erlaubt (Standard gesundheit).
	f := &fragerAttrappe{antwort: core.Answer{Text: core.NotFoundAnswer, Sources: nil}}
	rec := fragen(t, f, `{"frage": "Wie wird das Wetter?"}`)

	want := `{"antwort":"Dazu finde ich nichts in deinen Dokumenten.","gefunden":false,"quellen":[]}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != 200 || got != want {
		t.Errorf("%d %s", rec.Code, got)
	}
	if f.modul != "" {
		t.Errorf("Modul = %q, leer heißt Standard (entscheidet der Dienst)", f.modul)
	}
}

func TestFragen_UngueltigesJSON(t *testing.T) {
	for _, body := range []string{
		``, `kein json`, `{"frage": 42}`, `{"frge": "Tippfehler"}`,
		`{"frage": "a"} {"frage": "b"}`, `["frage"]`,
	} {
		t.Run(body, func(t *testing.T) {
			f := &fragerAttrappe{}
			pruefeFehler(t, fragen(t, f, body), 400, CodeUngueltigeAnfrage)
			if f.aufgerufen {
				t.Error("Dienst bei ungültigem JSON aufgerufen")
			}
		})
	}
}

func TestFragen_ZuGrosserBody(t *testing.T) {
	f := &fragerAttrappe{}
	body := `{"frage": "` + strings.Repeat("a", maxFrageBody) + `"}`
	pruefeFehler(t, fragen(t, f, body), 400, CodeUngueltigeAnfrage)
}

func TestFragen_FehlerDesDienstes(t *testing.T) {
	faelle := []struct {
		err    error
		status int
		code   string
	}{
		{core.ErrQuestionMissing, 400, CodeFrageFehlt},
		{core.ErrQuestionTooLong, 400, CodeFrageZuLang},
		{core.ErrUnknownModule, 400, CodeUngueltigeAnfrage},
		{fmt.Errorf("Gemini HTTP 503: %w", llm.ErrUnavailable), 503, CodeModellNichtErreichbar},
		{fmt.Errorf("blockiert: %w", llm.ErrInvalidResponse), 503, CodeModellNichtErreichbar},
		{errors.New("Vektorindex fehlt"), 500, CodeInternerFehler},
	}
	for _, f := range faelle {
		t.Run(f.code, func(t *testing.T) {
			rec := fragen(t, &fragerAttrappe{err: f.err}, `{"frage": "x", "modul": "sport"}`)
			a := pruefeFehler(t, rec, f.status, f.code)
			if errors.Is(f.err, core.ErrUnknownModule) && fmt.Sprint(a.Fehler.Details) != "map[feld:modul wert:sport]" {
				t.Errorf("details = %v", a.Fehler.Details)
			}
			for _, intern := range []string{"Gemini", "blockiert", "Vektorindex"} {
				if strings.Contains(rec.Body.String(), intern) {
					t.Errorf("interne Ursache in der Antwort: %s", rec.Body)
				}
			}
		})
	}
}
