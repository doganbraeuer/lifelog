package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/core"
)

// Error codes for accounts and login (API.md, section 1 and 3.10–3.14).
const (
	CodeUnauthorized           = "UNAUTHORIZED"
	CodePasswordChangeRequired = "PASSWORD_CHANGE_REQUIRED"
	CodeInvalidCredentials     = "INVALID_CREDENTIALS"
	CodeWrongPassword          = "WRONG_PASSWORD"
	CodeWeakPassword           = "WEAK_PASSWORD"
	CodeTooManyAttempts        = "TOO_MANY_ATTEMPTS"
)

// Accounts is login and the account endpoints. Implemented by
// service.Accounts.
type Accounts interface {
	Login(ctx context.Context, username, password, clientIP string) (core.LoginResult, error)
	Authenticate(ctx context.Context, token string) (user core.User, sessionID string, err error)
	Logout(ctx context.Context, sessionID string) error
	ChangePassword(ctx context.Context, user core.User, sessionID, current, newPassword, clientIP string) error
	UpdateDisplayName(ctx context.Context, user core.User, displayName string) (core.User, error)
	RecordConsent(ctx context.Context, user core.User, version string) error
}

// publicRoutes are the only patterns that work without a token (API.md,
// section 1). Everything else, including every route added later, needs
// one: the middleware locks by default.
var publicRoutes = map[string]bool{
	"GET /api/v1/status":      true,
	"POST /api/v1/auth/login": true,
}

// passwordChangeRoutes are the only patterns allowed while
// mustChangePassword is true.
var passwordChangeRoutes = map[string]bool{
	"GET /api/v1/me":           true,
	"POST /api/v1/me/password": true,
	"POST /api/v1/auth/logout": true,
}

// withAuth wraps the whole mux. For every request:
//
//  1. mux.Handler tells which registered pattern the mux would choose. That
//     is exactly the routing decision, so there is no second, possibly
//     different path matching here. Public patterns pass without a token.
//  2. Otherwise: read "Authorization: Bearer <token>" and let Accounts check
//     it (session, expiry, account exists and is not disabled). Unknown
//     paths and wrong methods also need a token, so without one nothing
//     about the API can be probed.
//  3. While mustChangePassword is true, only three patterns are allowed.
//  4. The user and the session go into the request context
//     (auth.UserFrom, auth.SessionIDFrom).
//
// There is deliberately no switch to turn this off, not even for
// development.
func withAuth(mux *http.ServeMux, accounts Accounts) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)
		if publicRoutes[pattern] {
			mux.ServeHTTP(w, r)
			return
		}

		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeUnauthorized(w)
			return
		}
		user, sessionID, err := accounts.Authenticate(r.Context(), token)
		switch {
		case errors.Is(err, core.ErrUnauthorized):
			writeUnauthorized(w)
			return
		case err != nil:
			// Database down: refuse, never let the request through.
			writeInternalError(w, r, err)
			return
		}

		if user.MustChangePassword && !passwordChangeRoutes[pattern] {
			writeError(w, http.StatusForbidden, CodePasswordChangeRequired,
				"Bitte ändere zuerst dein Startpasswort.", nil)
			return
		}
		mux.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), user, sessionID)))
	})
}

// bearerToken reads "Bearer <token>". The scheme is case-insensitive (RFC
// 7235); the token must not be empty or contain spaces.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}

// writeUnauthorized: 401 with WWW-Authenticate, as HTTP requires for 401.
// One message for every cause, so the response does not tell whether a
// token existed, expired or belongs to a disabled account.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, CodeUnauthorized, "Bitte melde dich an.", nil)
}

// writeTooManyAttempts: 429 with Retry-After and details.retryAfterSeconds,
// both the same number of seconds (API.md 3.10). The message names no fixed
// time, because the wait grows.
func writeTooManyAttempts(w http.ResponseWriter, e *core.TooManyAttemptsError) {
	seconds := e.RetryAfterSeconds()
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, CodeTooManyAttempts,
		"Zu viele Fehlversuche. Bitte versuche es in einigen Minuten erneut.",
		map[string]any{"retryAfterSeconds": seconds})
}

// clientIP of a request for counting failed attempts (auth.ClientIP).
func clientIP(r *http.Request, deps Dependencies) string {
	return auth.ClientIP(r.RemoteAddr, r.Header.Values("X-Forwarded-For"), deps.TrustedProxies)
}
