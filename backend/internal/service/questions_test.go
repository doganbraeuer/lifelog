package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/fake"
)

type fakeSearch struct {
	hits        []core.Hit
	entries     map[string]core.EntrySummary
	err         error
	askedModule string
	askedK      int
}

func (s *fakeSearch) SimilarChunks(_ context.Context, _ []float32, module string, k int) ([]core.Hit, error) {
	s.askedModule, s.askedK = module, k
	return s.hits, s.err
}

func (s *fakeSearch) SummariesByIDs(_ context.Context, ids []string) (map[string]core.EntrySummary, error) {
	m := map[string]core.EntrySummary{}
	for _, id := range ids {
		if e, ok := s.entries[id]; ok {
			m[id] = e
		}
	}
	return m, nil
}

const (
	idLab    = "aaaaaaaaaaaaaaaaaaaaaaaa"
	idLetter = "bbbbbbbbbbbbbbbbbbbbbbbb"
	idGone   = "cccccccccccccccccccccccc" // entry deleted, chunk still there
)

func searchData() *fakeSearch {
	return &fakeSearch{
		hits: []core.Hit{
			{EntryID: idLab, Position: 3, Text: "Hämoglobin 14,2 g/dl", Score: 0.89},
			{EntryID: idGone, Position: 1, Text: "verwaist", Score: 0.86},
			{EntryID: idLetter, Position: 1, Text: "Blutdruck 128/82", Score: core.MinScore},              // exactly on the threshold
			{EntryID: idLetter, Position: 2, Text: "Kontrolle in 6 Monaten", Score: core.MinScore - 0.01}, // below
		},
		entries: map[string]core.EntrySummary{
			idLab:    {ID: idLab, Module: "health", Type: "lab_report", DocumentDate: ptr("2026-10-14"), Issuer: ptr("Labor Muster GmbH")},
			idLetter: {ID: idLetter, Module: "health", Type: "doctor_letter", DocumentDate: ptr("2026-09-02"), Issuer: ptr("Dr. Erika Beispiel")},
		},
	}
}

func TestAsk_Hit(t *testing.T) {
	s := searchData()
	a := &fake.Answerer{Text: "Im Laborbefund steht Hämoglobin 14,2 g/dl [1]."}
	asker := NewAsker(s, &fake.Embedder{}, a)

	answer, err := asker.Ask(context.Background(), "  Wie hoch war mein Hb?  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.askedModule != "health" || s.askedK != core.MaxHits {
		t.Errorf("search with module %q, k %d", s.askedModule, s.askedK)
	}

	// Only hits ≥ MinScore with an existing entry went to the model, renumbered.
	question, sources := a.LastRequest()
	if question != "Wie hoch war mein Hb?" {
		t.Errorf("question = %q, want without whitespace", question)
	}
	if len(sources) != 2 || sources[0].Number != 1 || sources[0].EntryID != idLab || sources[1].Number != 2 || sources[1].EntryID != idLetter {
		t.Fatalf("sources = %+v", sources)
	}
	if sources[0].Type != "lab_report" || *sources[0].DocumentDate != "2026-10-14" || *sources[0].Issuer != "Labor Muster GmbH" || sources[0].Module != "health" {
		t.Errorf("details from the entry missing: %+v", sources[0])
	}

	if !answer.Found || len(answer.Sources) != 1 || answer.Sources[0].Excerpt != "Hämoglobin 14,2 g/dl" {
		t.Errorf("answer = %+v", answer)
	}
}

func TestAsk_BelowThresholdNoModelCall(t *testing.T) {
	s := &fakeSearch{hits: []core.Hit{{EntryID: idLab, Score: 0.77, Text: "Wetter?"}},
		entries: searchData().entries}
	a := &fake.Answerer{Text: "should never come [1]"}

	answer, err := NewAsker(s, &fake.Embedder{}, a).Ask(context.Background(), "Wie wird das Wetter?", "")

	if err != nil || answer.Found || answer.Text != core.NotFoundAnswer || answer.Sources == nil || len(answer.Sources) != 0 {
		t.Errorf("answer = %+v, %v; want the fixed sentence", answer, err)
	}
	if a.Calls() != 0 {
		t.Error("without hits above the threshold the language model must not be asked")
	}
}

func TestAsk_OnlyOrphanedHits(t *testing.T) {
	s := &fakeSearch{hits: []core.Hit{{EntryID: idGone, Score: 0.95}}, entries: map[string]core.EntrySummary{}}
	a := &fake.Answerer{}
	answer, _ := NewAsker(s, &fake.Embedder{}, a).Ask(context.Background(), "x", "")
	if answer.Found || a.Calls() != 0 {
		t.Errorf("orphaned chunks must not go to the model: %+v, %d calls", answer, a.Calls())
	}
}

func TestAsk_ModelWithoutReference(t *testing.T) {
	// Second layer: the model answers with the fixed sentence itself or
	// without a source; both become "nothing found".
	for _, text := range []string{core.NotFoundAnswer, "Der Wert war 14,2."} {
		a := &fake.Answerer{Text: text}
		answer, err := NewAsker(searchData(), &fake.Embedder{}, a).Ask(context.Background(), "x", "")
		if err != nil || answer.Found || answer.Text != core.NotFoundAnswer || len(answer.Sources) != 0 {
			t.Errorf("%q → %+v, %v", text, answer, err)
		}
	}
}

func TestAsk_Errors(t *testing.T) {
	cases := []struct {
		name     string
		question string
		module   string
		search   *fakeSearch
		embedder *fake.Embedder
		answerer *fake.Answerer
		err      error
	}{
		{"question empty", "  ", "", searchData(), &fake.Embedder{}, &fake.Answerer{}, core.ErrQuestionMissing},
		{"question too long", strings.Repeat("a", 501), "", searchData(), &fake.Embedder{}, &fake.Answerer{}, core.ErrQuestionTooLong},
		{"module unknown", "x", "sport", searchData(), &fake.Embedder{}, &fake.Answerer{}, core.ErrUnknownModule},
		{"embedding unavailable", "x", "", searchData(), &fake.Embedder{Err: llm.ErrUnavailable}, &fake.Answerer{}, llm.ErrUnavailable},
		{"answerer unavailable", "x", "", searchData(), &fake.Embedder{}, &fake.Answerer{Err: llm.ErrUnavailable}, llm.ErrUnavailable},
		{"vector search fails", "x", "", &fakeSearch{err: errors.New("index missing")}, &fake.Embedder{}, &fake.Answerer{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewAsker(c.search, c.embedder, c.answerer).Ask(context.Background(), c.question, c.module)
			if err == nil {
				t.Fatal("want an error")
			}
			if c.err != nil && !errors.Is(err, c.err) {
				t.Errorf("err = %v, want %v", err, c.err)
			}
		})
	}
}

func TestAsk_QuestionAndTextsNotInLog(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	a := &fake.Answerer{Text: "GEHEIME-ANTWORT [1]"}
	_, _ = NewAsker(searchData(), &fake.Embedder{}, a).Ask(context.Background(), "GEHEIME-FRAGE", "")

	for _, secret := range []string{"GEHEIME-FRAGE", "GEHEIME-ANTWORT", "Hämoglobin"} {
		if strings.Contains(log.String(), secret) {
			t.Errorf("%q in the log: %s", secret, log.String())
		}
	}
	if !strings.Contains(log.String(), "bestScore=0.89") {
		t.Errorf("score missing in the log (important for calibration): %s", log.String())
	}
}
