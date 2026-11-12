package service

import (
	"context"
	"log/slog"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// QuestionStore is what the search needs from the database. Implemented by
// store.Store.
type QuestionStore interface {
	SimilarChunks(ctx context.Context, vector []float32, module string, k int) ([]core.Hit, error)
	SummariesByIDs(ctx context.Context, ids []string) (map[string]core.EntrySummary, error)
}

// Asker answers questions about the documents (docs/API.md 3.6).
type Asker struct {
	store    QuestionStore
	embedder llm.Embedder
	answerer llm.Answerer
}

func NewAsker(s QuestionStore, e llm.Embedder, a llm.Answerer) *Asker {
	return &Asker{store: s, embedder: e, answerer: a}
}

// Ask:
//
//  1. Check the question and the module.
//  2. Embed the question, find the most similar chunks of the module.
//  3. First layer: drop hits below core.MinScore, as well as chunks whose
//     entry no longer exists.
//  4. If nothing remains: fixed sentence, WITHOUT calling the language
//     model.
//  5. Otherwise: language model with the question and numbered sources.
//  6. Second layer: core.BuildAnswer. Without a valid reference the answer
//     counts as "nothing found".
//
// Neither the question nor chunk texts go into the log, only numbers.
//
// Errors: core.ErrUnknownModule, core.ErrQuestionMissing,
// core.ErrQuestionTooLong, llm.ErrUnavailable, llm.ErrInvalidResponse or
// internal.
func (a *Asker) Ask(ctx context.Context, question, module string) (core.Answer, error) {
	module, err := core.CheckModule(module)
	if err != nil {
		return core.Answer{}, err
	}
	question, err = core.CheckQuestion(question)
	if err != nil {
		return core.Answer{}, err
	}

	vector, err := a.embedder.EmbedQuestion(ctx, question)
	if err != nil {
		return core.Answer{}, err
	}
	all, err := a.store.SimilarChunks(ctx, vector, module, core.MaxHits)
	if err != nil {
		return core.Answer{}, err
	}
	hits := core.AboveThreshold(all, core.MinScore)

	sources, err := a.sources(ctx, hits, module)
	if err != nil {
		return core.Answer{}, err
	}
	slog.Info("question", "module", module, "hits", len(all), "aboveThreshold", len(hits),
		"sources", len(sources), "bestScore", bestScore(all))
	if len(sources) == 0 {
		return core.NoHit(), nil
	}

	text, err := a.answerer.Answer(ctx, question, sources)
	if err != nil {
		return core.Answer{}, err
	}
	answer := core.BuildAnswer(text, sources)
	if !answer.Found {
		slog.Info("answer without a valid reference, fixed sentence", "sources", len(sources))
	}
	return answer, nil
}

// sources numbers the hits (1, 2, …) and adds the current details of the
// entry: type, date and issuer, even if they were corrected (API.md 3.6).
// Hits without an entry are dropped; that can briefly happen during an
// upload or a delete.
func (a *Asker) sources(ctx context.Context, hits []core.Hit, module string) ([]core.Source, error) {
	if len(hits) == 0 {
		return []core.Source{}, nil
	}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.EntryID)
	}
	entries, err := a.store.SummariesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	sources := make([]core.Source, 0, len(hits))
	for _, h := range hits {
		e, ok := entries[h.EntryID]
		if !ok {
			continue // orphaned chunk
		}
		sources = append(sources, core.Source{
			Number:       len(sources) + 1,
			EntryID:      h.EntryID,
			Module:       module,
			Type:         e.Type,
			Issuer:       e.Issuer,
			DocumentDate: e.DocumentDate,
			Excerpt:      h.Text,
		})
	}
	return sources, nil
}

func bestScore(hits []core.Hit) float64 {
	if len(hits) == 0 {
		return 0
	}
	return hits[0].Score
}
