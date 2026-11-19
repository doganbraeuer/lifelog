package api

import (
	"context"
	"errors"
	"net/http"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/core"
)

// Error codes for per-user data and daily limits (API.md, section 1).
const (
	CodeLimitReached = "LIMIT_REACHED"
)

// Usage counts a request against the user's daily limit. Implemented by
// service.Usage; the transcription handler uses it directly, upload and
// questions inside their services.
type Usage interface {
	Use(ctx context.Context, userID string, kind core.LimitKind) error
}

// requireUser returns the logged-in user of the request. Every handler that
// touches entries calls it and passes user.ID on; without a user (must not
// happen behind withAuth) the answer is 500 and no data at all.
func requireUser(w http.ResponseWriter, r *http.Request) (core.User, bool) {
	user, ok := auth.UserFrom(r.Context())
	if !ok || user.ID == "" {
		writeInternalError(w, r, errors.New("request without a user in the context"))
		return core.User{}, false
	}
	return user, true
}

// writeLimitReached: 429 LIMIT_REACHED with details.limit (API.md, daily
// limits).
func writeLimitReached(w http.ResponseWriter, e *core.LimitReachedError) {
	writeError(w, http.StatusTooManyRequests, CodeLimitReached,
		"Du hast dein Tageslimit erreicht. Morgen geht es weiter.",
		map[string]any{"limit": string(e.Limit)})
}

// messageSearchUpdating: questions while the vector index is rebuilt
// (API.md 3.6).
const messageSearchUpdating = "Die Suche wird gerade aktualisiert. Bitte versuche es in ein paar Minuten erneut."
