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

type fakeAsker struct {
	answer           core.Answer
	err              error
	question, module string
	called           bool
}

func (a *fakeAsker) Ask(_ context.Context, question, module string) (core.Answer, error) {
	a.called, a.question, a.module = true, question, module
	return a.answer, a.err
}

func ask(t *testing.T, a *fakeAsker, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Dependencies{DB: fakePinger{}, Asker: a})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/fragen", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func TestQuestions_Hit(t *testing.T) {
	p := func(s string) *string { return &s }
	a := &fakeAsker{answer: core.Answer{
		Text: "Im Laborbefund steht Hämoglobin 14,2 g/dl [1].", Found: true,
		Sources: []core.Source{{Number: 1, EntryID: "6727a1f3c2b4e5d6f7a8b9c0", Module: "gesundheit",
			Type: "laborbefund", Issuer: p("Labor Muster GmbH, Salzburg"), DocumentDate: p("2026-10-14"),
			Excerpt: "Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)"}},
	}}
	rec := ask(t, a, `{"frage": "Welchen Hämoglobinwert hatte ich im Oktober?", "modul": "gesundheit"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if a.question != "Welchen Hämoglobinwert hatte ich im Oktober?" || a.module != "gesundheit" {
		t.Errorf("to the service: %q / %q", a.question, a.module)
	}
	// Exactly the form from API.md 3.6.
	want := `{"antwort":"Im Laborbefund steht Hämoglobin 14,2 g/dl [1].","gefunden":true,"quellen":[` +
		`{"nr":1,"eintragId":"6727a1f3c2b4e5d6f7a8b9c0","modul":"gesundheit","typ":"laborbefund",` +
		`"quelle":"Labor Muster GmbH, Salzburg","dokumentdatum":"2026-10-14","auszug":"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestQuestions_NoHitIsEmptyList(t *testing.T) {
	// The app sends no module; that is allowed (default gesundheit).
	a := &fakeAsker{answer: core.Answer{Text: core.NotFoundAnswer, Sources: nil}}
	rec := ask(t, a, `{"frage": "Wie wird das Wetter?"}`)

	want := `{"antwort":"Dazu finde ich nichts in deinen Dokumenten.","gefunden":false,"quellen":[]}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != 200 || got != want {
		t.Errorf("%d %s", rec.Code, got)
	}
	if a.module != "" {
		t.Errorf("module = %q, empty means default (the service decides)", a.module)
	}
}

func TestQuestions_InvalidJSON(t *testing.T) {
	for _, body := range []string{
		``, `kein json`, `{"frage": 42}`, `{"frge": "Tippfehler"}`,
		`{"frage": "a"} {"frage": "b"}`, `["frage"]`,
	} {
		t.Run(body, func(t *testing.T) {
			a := &fakeAsker{}
			checkError(t, ask(t, a, body), 400, CodeInvalidRequest)
			if a.called {
				t.Error("service called for invalid JSON")
			}
		})
	}
}

func TestQuestions_BodyTooLarge(t *testing.T) {
	a := &fakeAsker{}
	body := `{"frage": "` + strings.Repeat("a", maxQuestionBody) + `"}`
	checkError(t, ask(t, a, body), 400, CodeInvalidRequest)
}

func TestQuestions_ServiceErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{core.ErrQuestionMissing, 400, CodeQuestionMissing},
		{core.ErrQuestionTooLong, 400, CodeQuestionTooLong},
		{core.ErrUnknownModule, 400, CodeInvalidRequest},
		{fmt.Errorf("Gemini HTTP 503: %w", llm.ErrUnavailable), 503, CodeModelUnavailable},
		{fmt.Errorf("blocked: %w", llm.ErrInvalidResponse), 503, CodeModelUnavailable},
		{errors.New("vector index missing"), 500, CodeInternalError},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			rec := ask(t, &fakeAsker{err: c.err}, `{"frage": "x", "modul": "sport"}`)
			a := checkError(t, rec, c.status, c.code)
			if errors.Is(c.err, core.ErrUnknownModule) && fmt.Sprint(a.Error.Details) != "map[feld:modul wert:sport]" {
				t.Errorf("details = %v", a.Error.Details)
			}
			for _, internal := range []string{"Gemini", "blocked", "vector index"} {
				if strings.Contains(rec.Body.String(), internal) {
					t.Errorf("internal cause in the response: %s", rec.Body)
				}
			}
		})
	}
}
