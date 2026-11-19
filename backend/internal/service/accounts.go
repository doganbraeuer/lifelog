package service

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/core"
)

// AccountStore is what login and the account endpoints need from the
// database. Implemented by store.Store.
type AccountStore interface {
	UserByUsername(ctx context.Context, username string) (u core.User, found bool, err error)
	UserByID(ctx context.Context, id string) (u core.User, found bool, err error)
	CreateSession(ctx context.Context, s core.Session) error
	SessionByTokenHash(ctx context.Context, tokenHash string) (s core.Session, found bool, err error)
	DeleteSession(ctx context.Context, id string) error
	DeleteOtherSessions(ctx context.Context, userID, keepID string) (int64, error)
	SetPassword(ctx context.Context, id, passwordHash string, mustChange bool) (found bool, err error)
	SetDisplayName(ctx context.Context, id, displayName string) (found bool, err error)
}

// accountWriteTimeout: time for the writes of a password change. They keep
// running without the request's context (see ChangePassword).
const accountWriteTimeout = 15 * time.Second

// maxTokenLength: a real token has 43 characters. Anything much longer is
// refused without asking the database.
const maxTokenLength = 256

// Accounts implements login, token check, logout, password change and the
// display name (docs/API.md 3.10–3.14).
//
// Logging: at most the user ID, never a username of a failed attempt (people
// sometimes type their password into that field), never a password, token
// or hash.
type Accounts struct {
	store   AccountStore
	limiter *auth.Limiter
	newID   func() string
	now     func() time.Time

	// The bcrypt functions, replaceable in tests (to count calls or to hash
	// faster). In the program always the ones from internal/auth.
	comparePassword func(hash, password string) bool
	compareDummy    func(password string)
	hashPassword    func(password string) (string, error)
}

// NewAccounts: newID creates session IDs (store.NewID in the program).
func NewAccounts(s AccountStore, l *auth.Limiter, newID func() string) *Accounts {
	return &Accounts{
		store:           s,
		limiter:         l,
		newID:           newID,
		now:             time.Now,
		comparePassword: auth.ComparePassword,
		compareDummy:    auth.CompareDummy,
		hashPassword:    auth.HashPassword,
	}
}

// Login checks username and password and creates a session (API.md 3.10).
//
//  1. Normalize the username (trim, lowercase) and ask the limiter. A
//     username that cannot exist counts only for the IP address.
//  2. Load the user.
//  3. Unknown user or a password over 72 bytes: compare against the dummy
//     hash, so this takes as long as a real comparison, then fail.
//  4. Compare the password. A disabled account also gets the real
//     comparison first and then the same error, so neither the answer nor
//     its timing tells it apart from a wrong password.
//  5. Create a token and a session valid for 30 days. Only the token's hash
//     is stored.
//
// Errors: core.ErrInvalidCredentials, *core.TooManyAttemptsError or an
// internal error.
func (a *Accounts) Login(ctx context.Context, username, password, clientIP string) (core.LoginResult, error) {
	name := core.NormalizeUsername(username)
	limitName := name
	if core.CheckUsername(name) != nil {
		limitName = ""
	}
	attempt, err := a.limiter.Begin(clientIP, limitName)
	if err != nil {
		slog.Info("login refused: too many failed attempts")
		return core.LoginResult{}, err
	}

	var u core.User
	found := false
	if limitName != "" {
		u, found, err = a.store.UserByUsername(ctx, name)
		if err != nil {
			attempt.Abort()
			return core.LoginResult{}, err
		}
	}
	if !found || len(password) > core.MaxPasswordBytes {
		a.compareDummy(password)
		attempt.Fail()
		slog.Info("login failed")
		return core.LoginResult{}, core.ErrInvalidCredentials
	}
	if !a.comparePassword(u.PasswordHash, password) || u.Disabled {
		attempt.Fail()
		slog.Info("login failed")
		return core.LoginResult{}, core.ErrInvalidCredentials
	}

	token := auth.NewToken()
	now := a.now().UTC()
	s := core.Session{
		ID:        a.newID(),
		TokenHash: auth.HashToken(token),
		UserID:    u.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(core.SessionLifetime),
	}
	if err := a.store.CreateSession(ctx, s); err != nil {
		attempt.Abort()
		return core.LoginResult{}, err
	}
	attempt.Succeed()
	slog.Info("login", "user", u.ID)
	return core.LoginResult{Token: token, ExpiresAt: s.ExpiresAt, User: u}, nil
}

// Authenticate checks a token on every request (auth middleware): the
// session must exist and not be expired, the user must exist and not be
// disabled. Returns the user and the session ID.
//
// The expiry is checked here even though the TTL index deletes expired
// sessions: MongoDB's TTL job only runs about once a minute.
//
// Errors: core.ErrUnauthorized for every refusal, or an internal error
// (database). Either way the request does not get through.
func (a *Accounts) Authenticate(ctx context.Context, token string) (core.User, string, error) {
	if token == "" || len(token) > maxTokenLength {
		return core.User{}, "", core.ErrUnauthorized
	}
	s, found, err := a.store.SessionByTokenHash(ctx, auth.HashToken(token))
	if err != nil {
		return core.User{}, "", err
	}
	if !found || !a.now().Before(s.ExpiresAt) {
		return core.User{}, "", core.ErrUnauthorized
	}
	u, found, err := a.store.UserByID(ctx, s.UserID)
	if err != nil {
		return core.User{}, "", err
	}
	if !found || u.Disabled {
		return core.User{}, "", core.ErrUnauthorized
	}
	return u, s.ID, nil
}

// Logout ends the session of this device (API.md 3.11).
func (a *Accounts) Logout(ctx context.Context, sessionID string) error {
	return a.store.DeleteSession(ctx, sessionID)
}

// ChangePassword replaces the password (API.md 3.14). user and sessionID
// come from the auth middleware; user.PasswordHash was read at the start of
// this request.
//
//  1. Ask the limiter (same counters as the login).
//  2. Check the current password; a wrong one counts as a failed attempt.
//  3. Check the new password (core.CheckNewPassword).
//  4. Store the hash, mustChangePassword false.
//  5. End all other sessions of the account; this one stays.
//
// Steps 4 and 5 run to completion even if the app drops the connection
// (context.WithoutCancel). Otherwise the password could be changed while
// stolen sessions survive.
//
// Errors: *core.TooManyAttemptsError, core.ErrWrongPassword,
// *core.WeakPasswordError, core.ErrUnauthorized (account deleted meanwhile)
// or an internal error.
func (a *Accounts) ChangePassword(ctx context.Context, user core.User, sessionID, current, newPassword, clientIP string) error {
	attempt, err := a.limiter.Begin(clientIP, user.Username)
	if err != nil {
		slog.Info("password change refused: too many failed attempts", "user", user.ID)
		return err
	}
	if !a.comparePassword(user.PasswordHash, current) {
		attempt.Fail()
		slog.Info("password change: wrong current password", "user", user.ID)
		return core.ErrWrongPassword
	}
	attempt.Succeed()

	if err := core.CheckNewPassword(current, newPassword); err != nil {
		return err
	}
	hash, err := a.hashPassword(newPassword)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), accountWriteTimeout)
	defer cancel()
	found, err := a.store.SetPassword(ctx, user.ID, hash, false)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrUnauthorized
	}
	ended, err := a.store.DeleteOtherSessions(ctx, user.ID, sessionID)
	if err != nil {
		return err
	}
	slog.Info("password changed", "user", user.ID, "other_sessions_ended", ended)
	return nil
}

// UpdateDisplayName stores a new display name (API.md 3.13) and returns the
// user with it.
//
// Errors: core.ErrInvalidDisplayName, core.ErrUnauthorized (account deleted
// meanwhile) or an internal error.
func (a *Accounts) UpdateDisplayName(ctx context.Context, user core.User, displayName string) (core.User, error) {
	name, err := core.NormalizeDisplayName(displayName)
	if err != nil {
		return core.User{}, err
	}
	found, err := a.store.SetDisplayName(ctx, user.ID, name)
	if err != nil {
		return core.User{}, err
	}
	if !found {
		return core.User{}, core.ErrUnauthorized
	}
	user.DisplayName = name
	return user, nil
}
