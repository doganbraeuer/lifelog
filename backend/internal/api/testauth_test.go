package api

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/service"
)

// Test helper for the API tests: every protected route now needs a valid
// token. Instead of switching the middleware off for tests, the router gets
// the real service.Accounts with an in-memory store that knows one user and
// one session; requests carry that session's token. The middleware runs
// exactly as in the program.
//
//	router := NewRouter(withTestUser(Dependencies{...}))
//	router.ServeHTTP(rec, authorized(httptest.NewRequest(...)))

const (
	// testToken belongs to the session of testUser. The store keeps only
	// its hash, as the real one does.
	testToken    = "dGVzdC10b2tlbi1mb3ItdGhlLWFwaS10ZXN0cy0wMDA"
	testPassword = "h4Pq8sZt2kXw"
)

// withTestUser adds Accounts with the test user (tester01, password
// already changed) to deps.
func withTestUser(deps Dependencies) Dependencies {
	deps.Accounts = newTestAccounts().accounts
	return deps
}

// authorized sets the test user's token on the request.
func authorized(r *http.Request) *http.Request {
	r.Header.Set("Authorization", "Bearer "+testToken)
	return r
}

// testAccounts gives tests access to the store behind the router, e.g. to
// disable the user or add a second session.
type testAccounts struct {
	store    *memAccountStore
	accounts *service.Accounts
	user     core.User
}

func newTestAccounts() *testAccounts {
	ta := &testAccounts{store: &memAccountStore{users: map[string]core.User{}, sessions: map[string]core.Session{}}}
	ta.accounts = service.NewAccounts(ta.store, auth.NewLimiter(time.Now), func() string { return bson.NewObjectID().Hex() })
	ta.user = ta.addUser("tester01", testPassword, false)
	ta.addSession(ta.user.ID, testToken, time.Now().Add(time.Hour))
	return ta
}

// addUser stores a user. bcrypt with the lowest cost keeps the tests fast;
// comparing works the same way for every cost.
func (ta *testAccounts) addUser(username, password string, mustChange bool) core.User {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	u := core.User{
		ID: bson.NewObjectID().Hex(), Username: username, DisplayName: username,
		PasswordHash: string(h), Role: core.RoleUser, MustChangePassword: mustChange,
	}
	ta.store.users[u.ID] = u
	return u
}

// addSession stores a session for token, as a login would.
func (ta *testAccounts) addSession(userID, token string, expiresAt time.Time) core.Session {
	s := core.Session{
		ID: bson.NewObjectID().Hex(), TokenHash: auth.HashToken(token), UserID: userID,
		CreatedAt: time.Now(), ExpiresAt: expiresAt,
	}
	ta.store.sessions[s.ID] = s
	return s
}

func (ta *testAccounts) router(deps Dependencies) http.Handler {
	deps.Accounts = ta.accounts
	if deps.DB == nil {
		deps.DB = fakePinger{}
	}
	return NewRouter(deps)
}

// memAccountStore implements service.AccountStore in memory. err, if set,
// is returned by every call (database down).
type memAccountStore struct {
	users    map[string]core.User
	sessions map[string]core.Session
	err      error
}

func (m *memAccountStore) UserByUsername(_ context.Context, username string) (core.User, bool, error) {
	if m.err != nil {
		return core.User{}, false, m.err
	}
	for _, u := range m.users {
		if u.Username == username {
			return u, true, nil
		}
	}
	return core.User{}, false, nil
}

func (m *memAccountStore) UserByID(_ context.Context, id string) (core.User, bool, error) {
	if m.err != nil {
		return core.User{}, false, m.err
	}
	u, ok := m.users[id]
	return u, ok, nil
}

func (m *memAccountStore) CreateSession(_ context.Context, s core.Session) error {
	if m.err != nil {
		return m.err
	}
	m.sessions[s.ID] = s
	return nil
}

func (m *memAccountStore) SessionByTokenHash(_ context.Context, hash string) (core.Session, bool, error) {
	if m.err != nil {
		return core.Session{}, false, m.err
	}
	for _, s := range m.sessions {
		if s.TokenHash == hash {
			return s, true, nil
		}
	}
	return core.Session{}, false, nil
}

func (m *memAccountStore) DeleteSession(_ context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.sessions, id)
	return nil
}

func (m *memAccountStore) DeleteOtherSessions(_ context.Context, userID, keepID string) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	var n int64
	for id, s := range m.sessions {
		if s.UserID == userID && id != keepID {
			delete(m.sessions, id)
			n++
		}
	}
	return n, nil
}

func (m *memAccountStore) SetPassword(_ context.Context, id, hash string, mustChange bool) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	u, ok := m.users[id]
	if !ok {
		return false, nil
	}
	u.PasswordHash, u.MustChangePassword = hash, mustChange
	m.users[id] = u
	return true, nil
}

func (m *memAccountStore) SetDisplayName(_ context.Context, id, name string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	u, ok := m.users[id]
	if !ok {
		return false, nil
	}
	u.DisplayName = name
	m.users[id] = u
	return true, nil
}
