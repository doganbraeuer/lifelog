package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	speechfake "lifelog/backend/internal/speech/fake"
)

// While the vector index is rebuilt: 503 MODEL_UNAVAILABLE with its own
// message, never 500 and never "nothing found" (API.md 3.6).
func TestQuestions_SearchUpdatingIs503(t *testing.T) {
	for _, err := range []error{core.ErrSearchUpdating, fmt.Errorf("vector search: %w", core.ErrSearchUpdating)} {
		rec := ask(t, &fakeAsker{err: err}, `{"question": "Hb?"}`)
		a := checkError(t, rec, http.StatusServiceUnavailable, CodeModelUnavailable)
		if a.Error.Message != "Die Suche wird gerade aktualisiert. Bitte versuche es in ein paar Minuten erneut." {
			t.Errorf("message = %q", a.Error.Message)
		}
	}
}

func checkLimitReached(t *testing.T, rec *httptest.ResponseRecorder, limit string) {
	t.Helper()
	a := checkError(t, rec, http.StatusTooManyRequests, CodeLimitReached)
	if a.Error.Message != "Du hast dein Tageslimit erreicht. Morgen geht es weiter." {
		t.Errorf("message = %q", a.Error.Message)
	}
	if got, _ := a.Error.Details["limit"].(string); got != limit {
		t.Errorf("details.limit = %q, want %q", got, limit)
	}
}

func TestQuestions_LimitReachedIs429(t *testing.T) {
	rec := ask(t, &fakeAsker{err: &core.LimitReachedError{Limit: core.LimitQuestions}}, `{"question": "Hb?"}`)
	checkLimitReached(t, rec, "questions")
}

func TestUpload_LimitReachedIs429(t *testing.T) {
	u := &fakeUploader{err: &core.LimitReachedError{Limit: core.LimitUploads}}
	checkLimitReached(t, upload(t, u, textForm(t, [2]string{"text", "Notiz"})), "uploads")
	checkLimitReached(t, upload(t, u, form(t, map[string][]byte{"a.pdf": []byte("%PDF-1.7")}, nil)), "uploads")
}

// limitAfter refuses from the (n+1)-th request on.
type limitAfter struct {
	n, calls int
}

func (l *limitAfter) Use(_ context.Context, _ string, kind core.LimitKind) error {
	if l.calls >= l.n {
		return &core.LimitReachedError{Limit: kind}
	}
	l.calls++
	return nil
}

// The transcription counts after the format check and before Whisper; at
// the limit Whisper is not called.
func TestTranscribe_DailyLimit(t *testing.T) {
	tr := &speechfake.Transcriber{Result: core.Transcript{Text: "Notiz", Language: "de"}}
	limit := &limitAfter{n: 1}
	router := NewRouter(withTestUser(Dependencies{DB: fakePinger{}, Transcriber: tr, Usage: limit}))

	serve := func(r *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authorized(r))
		return rec
	}
	if rec := serve(audioForm(t, "audio", m4a)); rec.Code != http.StatusOK {
		t.Fatalf("first transcription: %d %s", rec.Code, rec.Body)
	}
	checkLimitReached(t, serve(audioForm(t, "audio", m4a)), "transcriptions")
	if tr.Calls() != 1 {
		t.Errorf("Whisper called %d times, want 1", tr.Calls())
	}
	// A refused format does not count: still 415, not 429.
	if rec := serve(audioForm(t, "audio", []byte("not audio at all"))); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("wrong format at the limit: %d, want 415", rec.Code)
	}
}

func TestNewRouter_RefusesWithoutUsage(t *testing.T) {
	defer func() {
		if p := recover(); p == nil || !strings.Contains(fmt.Sprint(p), "Usage") {
			t.Errorf("NewRouter without Usage: panic = %v, want one naming Usage", p)
		}
	}()
	NewRouter(Dependencies{DB: fakePinger{}, Accounts: newTestAccounts().accounts})
}
