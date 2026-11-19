package service

import (
	"context"
	"log/slog"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// QuestionStore is what the search needs from the database. Implemented by
// store.Store. Both methods filter by the user.
type QuestionStore interface {
	SimilarChunks(ctx context.Context, userID string, vector []float32, module string, k int) ([]core.Hit, error)
	SummariesByIDs(ctx context.Context, userID string, ids []string) (map[string]core.EntrySummary, error)
}

// Asker answers questions about the user's documents (docs/API.md 3.6).
type Asker struct {
	store       QuestionStore
	embedder    llm.Embedder
	answerer    llm.Answerer
	usage       UsageCounter
	searchReady func() bool
}

// NewAsker creates the service. usage counts the daily limit and is
// required. searchReady tells whether the vector index is READY (the server
// watches it); nil means always ready.
func NewAsker(s QuestionStore, e llm.Embedder, a llm.Answerer, usage UsageCounter, searchReady func() bool) *Asker {
	if usage == nil {
		panic("service: NewAsker needs a UsageCounter")
	}
	if searchReady == nil {
		searchReady = func() bool { return true }
	}
	return &Asker{store: s, embedder: e, answerer: a, usage: usage, searchReady: searchReady}
}

// Ask:
//
//  1. Check the user, the question and the module.
//  2. If the vector index is being rebuilt: core.ErrSearchUpdating, before
//     any model call and without counting.
//  3. Count the question against the user's daily limit.
//  4. Embed the question, find the most similar chunks of the user's own
//     entries in the module (the vector search filters by both).
//  5. First layer: drop hits below core.MinScore, as well as chunks whose
//     entry no longer exists or is not the user's (SummariesByIDs filters
//     by the user again: a second, independent filter).
//  6. If nothing remains: fixed sentence, WITHOUT calling the language
//     model.
//  7. Otherwise: language model with the question and numbered sources.
//  8. Second layer: core.BuildAnswer. Without a valid reference the answer
//     counts as "nothing found".
//
// Neither the question nor chunk texts go into the log, only numbers.
//
// Errors: core.ErrNoUser, core.ErrUnknownModule, core.ErrQuestionMissing,
// core.ErrQuestionTooLong, core.ErrSearchUpdating, *core.LimitReachedError,
// llm.ErrUnavailable, llm.ErrInvalidResponse or internal.
func (a *Asker) Ask(ctx context.Context, userID, question, module string) (core.Answer, error) {
	if userID == "" {
		return core.Answer{}, core.ErrNoUser
	}
	module, err := core.CheckModule(module)
	if err != nil {
		return core.Answer{}, err
	}
	question, err = core.CheckQuestion(question)
	if err != nil {
		return core.Answer{}, err
	}
	if !a.searchReady() {
		slog.Info("question refused: vector index is being rebuilt", "user", userID)
		return core.Answer{}, core.ErrSearchUpdating
	}
	if err := a.usage.Use(ctx, userID, core.LimitQuestions); err != nil {
		return core.Answer{}, err
	}

	vector, err := a.embedder.EmbedQuestion(ctx, question)
	if err != nil {
		return core.Answer{}, err
	}
	// If the index is still rebuilt after all (the watcher did not know
	// yet), the error wraps core.ErrSearchUpdating; it is returned as it is.
	all, err := a.store.SimilarChunks(ctx, userID, vector, module, core.MaxHits)
	if err != nil {
		return core.Answer{}, err
	}
	hits := core.AboveThreshold(all, core.MinScore)

	sources, err := a.sources(ctx, userID, hits, module)
	if err != nil {
		return core.Answer{}, err
	}
	slog.Info("question", "user", userID, "module", module, "hits", len(all), "aboveThreshold", len(hits),
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
// Hits without an entry of the user are dropped; that can briefly happen
// during an upload or a delete, and it is the second filter against
// foreign chunks.
func (a *Asker) sources(ctx context.Context, userID string, hits []core.Hit, module string) ([]core.Source, error) {
	if len(hits) == 0 {
		return []core.Source{}, nil
	}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.EntryID)
	}
	entries, err := a.store.SummariesByIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}

	sources := make([]core.Source, 0, len(hits))
	for _, h := range hits {
		e, ok := entries[h.EntryID]
		if !ok {
			continue // orphaned chunk, or not the user's
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
