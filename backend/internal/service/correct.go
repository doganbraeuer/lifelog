package service

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/core"
)

// CorrectionStore is what correcting needs from the database. Implemented
// by store.Store.
type CorrectionStore interface {
	GetEntry(ctx context.Context, id string) (e core.Entry, found bool, err error)
	SaveCorrection(ctx context.Context, e core.Entry) (found bool, err error)
}

// Corrector corrects recognized values of an entry (docs/API.md 3.5).
type Corrector struct {
	store CorrectionStore
	now   func() time.Time
}

func NewCorrector(s CorrectionStore) *Corrector {
	return &Corrector{store: s, now: time.Now}
}

// Correct:
//
//  1. Check the ID, load the entry.
//  2. core.Correct: check everything, then apply rules 1–4.
//  3. If nothing changed: write nothing, return the entry unchanged
//     (API.md 3.5).
//  4. Otherwise store the correctable fields.
//
// Without a lock: two simultaneous corrections of the same entry can
// overwrite each other (API.md 4, concurrent corrections).
//
// Errors: core.ErrInvalidID, core.ErrEntryNotFound, the errors of
// core.Correct (*core.CorrectionError) or an internal error.
func (c *Corrector) Correct(ctx context.Context, id string, ch core.Change) (core.Entry, error) {
	if err := core.CheckID(id); err != nil {
		return core.Entry{}, err
	}
	e, found, err := c.store.GetEntry(ctx, id)
	if err != nil {
		return core.Entry{}, err
	}
	if !found {
		return core.Entry{}, core.ErrEntryNotFound
	}

	// Milliseconds, because MongoDB stores times with that precision: the
	// response and a later query then show the same (as on upload).
	now := c.now().UTC().Truncate(time.Millisecond)
	updated, changed, err := core.Correct(e, ch, now)
	if err != nil {
		return core.Entry{}, err
	}
	if len(changed) == 0 {
		slog.Info("correction without change", "entry", id)
		return updated, nil
	}

	found, err = c.store.SaveCorrection(ctx, updated)
	if err != nil {
		return core.Entry{}, err
	}
	if !found {
		// Deleted between reading and writing.
		return core.Entry{}, core.ErrEntryNotFound
	}
	// Only field names, never values.
	slog.Info("correction saved", "entry", id, "fields", changed)
	return updated, nil
}
