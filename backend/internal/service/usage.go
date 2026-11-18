package service

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/core"
)

// UsageCounter counts one accepted request against the user's daily limit
// (docs/API.md, daily limits). Implemented by Usage; the upload, the
// questions and the transcription handler call it after their input checks
// and before any model call.
type UsageCounter interface {
	Use(ctx context.Context, userID string, kind core.LimitKind) error
}

// UsageStore is what the daily limits need from the database. Implemented
// by store.Store.
type UsageStore interface {
	IncrementUsage(ctx context.Context, userID, day string, kind core.LimitKind, max int, expiresAt time.Time) (allowed bool, err error)
}

// Limits per user and day (config: LIMIT_UPLOADS_PER_DAY,
// LIMIT_QUESTIONS_PER_DAY, LIMIT_TRANSCRIPTIONS_PER_DAY).
type Limits struct {
	Uploads        int
	Questions      int
	Transcriptions int
}

func (l Limits) of(kind core.LimitKind) int {
	switch kind {
	case core.LimitUploads:
		return l.Uploads
	case core.LimitQuestions:
		return l.Questions
	case core.LimitTranscriptions:
		return l.Transcriptions
	}
	return 0 // unknown kind: never allowed
}

// usageKeep: counters are kept two days after their day ends, then MongoDB
// deletes them (TTL index). Only for looking into the database; the limit
// itself only reads today's counter.
const usageKeep = 48 * time.Hour

// Usage checks and counts the daily limits. Admins have the same limits as
// everyone else.
type Usage struct {
	store  UsageStore
	limits Limits
	now    func() time.Time
}

func NewUsage(s UsageStore, l Limits) *Usage {
	return &Usage{store: s, limits: l, now: time.Now}
}

// Use counts one request of the user. At the limit it returns
// *core.LimitReachedError and counts nothing. Checking and counting are one
// atomic step in the database, so parallel requests cannot pass the limit.
// A database error is returned as it is: no request gets through without
// being counted.
func (u *Usage) Use(ctx context.Context, userID string, kind core.LimitKind) error {
	if userID == "" {
		return core.ErrNoUser
	}
	now := u.now()
	allowed, err := u.store.IncrementUsage(ctx, userID, core.UsageDay(now), kind, u.limits.of(kind),
		core.UsageDayEnd(now).Add(usageKeep))
	if err != nil {
		return err
	}
	if !allowed {
		slog.Info("daily limit reached", "user", userID, "limit", string(kind))
		return &core.LimitReachedError{Limit: kind}
	}
	return nil
}
