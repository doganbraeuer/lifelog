package auth

import (
	"context"

	"lifelog/backend/internal/core"
)

// identityKey is the context key for the logged-in user. An own unexported
// type, so no other package can read or overwrite the value by accident.
type identityKey struct{}

type identity struct {
	user      core.User
	sessionID string
}

// WithIdentity stores the logged-in user and the ID of their session in the
// context. Only the auth middleware (internal/api) calls it, after it has
// checked the token.
func WithIdentity(ctx context.Context, user core.User, sessionID string) context.Context {
	return context.WithValue(ctx, identityKey{}, identity{user: user, sessionID: sessionID})
}

// UserFrom returns the logged-in user of the request. ok is false if the
// request did not pass the middleware (e.g. a public route); a handler that
// needs a user must then refuse, never continue.
func UserFrom(ctx context.Context) (user core.User, ok bool) {
	id, ok := ctx.Value(identityKey{}).(identity)
	return id.user, ok
}

// SessionIDFrom returns the ID of the session (device) of the request, e.g.
// to end exactly this session on logout.
func SessionIDFrom(ctx context.Context) (sessionID string, ok bool) {
	id, ok := ctx.Value(identityKey{}).(identity)
	return id.sessionID, ok
}
