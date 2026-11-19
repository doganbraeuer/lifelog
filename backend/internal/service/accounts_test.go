package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/core"
)

// fakeAccountStore keeps users and sessions in maps. err, if set, is
// returned by every call (database down).
type fakeAccountStore struct {
	users    map[string]core.User    // by ID
	sessions map[string]core.Session // by ID
	err      error
	lookups  int // UserByUsername calls
}

func newFakeAccountStore() *fakeAccountStore {
	return &fakeAccountStore{users: map[string]core.User{}, sessions: map[string]core.Session{}}
}

func (f *fakeAccountStore) UserByUsername(_ context.Context, username string) (core.User, bool, error) {
	f.lookups++
	if f.err != nil {
		return core.User{}, false, f.err
	}
	for _, u := range f.users {
		if u.Username == username {
			return u, true, nil
		}
	}
	return core.User{}, false, nil
}

func (f *fakeAccountStore) UserByID(_ context.Context, id string) (core.User, bool, error) {
	if f.err != nil {
		return core.User{}, false, f.err
	}
	u, ok := f.users[id]
	return u, ok, nil
}

func (f *fakeAccountStore) CreateSession(_ context.Context, s core.Session) error {
	if f.err != nil {
		return f.err
	}
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeAccountStore) SessionByTokenHash(_ context.Context, hash string) (core.Session, bool, error) {
	if f.err != nil {
		return core.Session{}, false, f.err
	}
	for _, s := range f.sessions {
		if s.TokenHash == hash {
			return s, true, nil
		}
	}
	return core.Session{}, false, nil
}

func (f *fakeAccountStore) DeleteSession(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.sessions, id)
	return nil
}

func (f *fakeAccountStore) DeleteOtherSessions(_ context.Context, userID, keepID string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	var n int64
	for id, s := range f.sessions {
		if s.UserID == userID && id != keepID {
			delete(f.sessions, id)
			n++
		}
	}
	return n, nil
}

func (f *fakeAccountStore) SetPassword(_ context.Context, id, hash string, mustChange bool) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return false, nil
	}
	u.PasswordHash, u.MustChangePassword = hash, mustChange
	f.users[id] = u
	return true, nil
}

func (f *fakeAccountStore) SetDisplayName(_ context.Context, id, name string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return false, nil
	}
	u.DisplayName = name
	f.users[id] = u
	return true, nil
}

func (f *fakeAccountStore) SetConsent(_ context.Context, id, version string, at time.Time) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return false, nil
	}
	u.ConsentVersion, u.ConsentAt = &version, &at
	f.users[id] = u
	return true, nil
}

// fastHash: bcrypt with the lowest cost, so the tests stay fast. The cost
// of real hashes (12) is tested in internal/auth.
func fastHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

type accountsFixture struct {
	accounts     *Accounts
	store        *fakeAccountStore
	clock        *time.Time
	user         core.User
	dummyCalls   int
	compareCalls int
}

const (
	startPassword = "h4Pq8sZt2kXw"
	clientIP      = "203.0.113.7"
)

// newAccountsFixture: one user tester01 with a start password.
func newAccountsFixture(t *testing.T) *accountsFixture {
	t.Helper()
	f := &accountsFixture{store: newFakeAccountStore()}
	now := time.Date(2026, 11, 18, 9, 15, 42, 0, time.UTC)
	f.clock = &now
	f.user = core.User{
		ID: bson.NewObjectID().Hex(), Username: "tester01", DisplayName: "tester01",
		PasswordHash: fastHash(t, startPassword), Role: core.RoleUser, MustChangePassword: true,
	}
	f.store.users[f.user.ID] = f.user

	a := NewAccounts(f.store, auth.NewLimiter(func() time.Time { return *f.clock }),
		func() string { return bson.NewObjectID().Hex() })
	a.now = func() time.Time { return *f.clock }
	a.compareDummy = func(string) { f.dummyCalls++ }
	a.comparePassword = func(hash, password string) bool {
		f.compareCalls++
		return auth.ComparePassword(hash, password)
	}
	a.hashPassword = func(p string) (string, error) {
		if len(p) > core.MaxPasswordBytes {
			return "", auth.ErrPasswordTooLong
		}
		return fastHash(t, p), nil
	}
	f.accounts = a
	return f
}

func (f *accountsFixture) login(t *testing.T, username, password string) (core.LoginResult, error) {
	t.Helper()
	return f.accounts.Login(context.Background(), username, password, clientIP)
}

func TestLogin_Success(t *testing.T) {
	f := newAccountsFixture(t)
	res, err := f.login(t, "tester01", startPassword)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Token) != 43 {
		t.Errorf("token length = %d, want 43", len(res.Token))
	}
	if res.User.ID != f.user.ID || !res.User.MustChangePassword {
		t.Errorf("user = %+v", res.User)
	}
	if want := f.clock.Add(30 * 24 * time.Hour); !res.ExpiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v", res.ExpiresAt, want)
	}
	if len(f.store.sessions) != 1 {
		t.Fatalf("%d sessions stored, want 1", len(f.store.sessions))
	}
	for _, s := range f.store.sessions {
		if s.TokenHash != auth.HashToken(res.Token) {
			t.Error("stored hash does not belong to the token")
		}
		if strings.Contains(s.TokenHash, res.Token) || s.TokenHash == res.Token {
			t.Error("the token itself must never be stored")
		}
		if s.UserID != f.user.ID {
			t.Errorf("session user = %s", s.UserID)
		}
	}
}

func TestLogin_NormalizesUsername(t *testing.T) {
	f := newAccountsFixture(t)
	if _, err := f.login(t, "  Tester01 ", startPassword); err != nil {
		t.Errorf("login with capital letter and spaces: %v", err)
	}
}

func TestLogin_PasswordNotTrimmed(t *testing.T) {
	f := newAccountsFixture(t)
	if _, err := f.login(t, "tester01", startPassword+" "); !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	f := newAccountsFixture(t)
	_, err := f.login(t, "tester01", "falschesPasswort")
	if !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
	if len(f.store.sessions) != 0 {
		t.Error("session created for a wrong password")
	}
}

func TestLogin_UnknownUserComparesAgainstDummy(t *testing.T) {
	f := newAccountsFixture(t)
	_, err := f.login(t, "tester99", startPassword)
	if !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
	if f.dummyCalls != 1 || f.compareCalls != 0 {
		t.Errorf("dummy = %d, real = %d comparisons; want 1, 0", f.dummyCalls, f.compareCalls)
	}
}

func TestLogin_ImpossibleUsernameNoLookup(t *testing.T) {
	f := newAccountsFixture(t)
	_, err := f.login(t, "tester-01; drop", startPassword)
	if !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
	if f.store.lookups != 0 || f.dummyCalls != 1 {
		t.Errorf("lookups = %d, dummy = %d; want 0, 1", f.store.lookups, f.dummyCalls)
	}
}

func TestLogin_DisabledAccount(t *testing.T) {
	f := newAccountsFixture(t)
	u := f.store.users[f.user.ID]
	u.Disabled = true
	f.store.users[u.ID] = u

	_, err := f.login(t, "tester01", startPassword)
	if !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials (same as a wrong password)", err)
	}
	// The real comparison runs, so the timing is that of a wrong password.
	if f.compareCalls != 1 {
		t.Errorf("real comparisons = %d, want 1", f.compareCalls)
	}
	if len(f.store.sessions) != 0 {
		t.Error("session created for a disabled account")
	}
}

func TestLogin_Over72BytesRefused(t *testing.T) {
	f := newAccountsFixture(t)
	first72 := strings.Repeat("k", 72)
	u := f.store.users[f.user.ID]
	u.PasswordHash = fastHash(t, first72)
	f.store.users[u.ID] = u

	if _, err := f.login(t, "tester01", first72+"egal"); !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("72 bytes + suffix: err = %v, want ErrInvalidCredentials", err)
	}
	if f.dummyCalls != 1 {
		t.Errorf("dummy comparisons = %d, want 1", f.dummyCalls)
	}
	if _, err := f.login(t, "tester01", first72); err != nil {
		t.Errorf("exactly 72 bytes: %v", err)
	}
}

func TestLogin_LockAfterFiveFailures(t *testing.T) {
	f := newAccountsFixture(t)
	for i := 0; i < 5; i++ {
		if _, err := f.login(t, "tester01", "falschesPasswort"); !errors.Is(err, core.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	// The sixth is refused even with the right password.
	_, err := f.login(t, "tester01", startPassword)
	var tm *core.TooManyAttemptsError
	if !errors.As(err, &tm) || tm.RetryAfter != time.Minute {
		t.Fatalf("err = %v, want TooManyAttemptsError with 1 min", err)
	}
	// From another IP the tester can still log in.
	if _, err := f.accounts.Login(context.Background(), "tester01", startPassword, "198.51.100.1"); err != nil {
		t.Errorf("other IP: %v", err)
	}
	*f.clock = f.clock.Add(time.Minute)
	if _, err := f.login(t, "tester01", startPassword); err != nil {
		t.Errorf("after the wait: %v", err)
	}
}

func TestLogin_UnknownUsernameAlsoLocks(t *testing.T) {
	f := newAccountsFixture(t)
	for i := 0; i < 5; i++ {
		_, _ = f.login(t, "tester99", "egal")
	}
	var tm *core.TooManyAttemptsError
	if _, err := f.login(t, "tester99", "egal"); !errors.As(err, &tm) {
		t.Errorf("err = %v, want TooManyAttemptsError (no hint that tester99 does not exist)", err)
	}
}

func TestLogin_DatabaseErrorIsNotAFailedAttempt(t *testing.T) {
	f := newAccountsFixture(t)
	f.store.err = errors.New("timeout")
	for i := 0; i < 6; i++ {
		_, err := f.login(t, "tester01", startPassword)
		if err == nil || errors.Is(err, core.ErrInvalidCredentials) {
			t.Fatalf("err = %v, want the database error", err)
		}
	}
	f.store.err = nil
	if _, err := f.login(t, "tester01", startPassword); err != nil {
		t.Errorf("after the database is back: %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	f := newAccountsFixture(t)
	res, err := f.login(t, "tester01", startPassword)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	u, sid, err := f.accounts.Authenticate(ctx, res.Token)
	if err != nil || u.ID != f.user.ID || f.store.sessions[sid].UserID != f.user.ID {
		t.Fatalf("Authenticate = %+v, %q, %v", u, sid, err)
	}

	for name, token := range map[string]string{
		"empty":     "",
		"unknown":   auth.NewToken(),
		"too long":  strings.Repeat("a", 300),
		"truncated": res.Token[:42],
	} {
		if _, _, err := f.accounts.Authenticate(ctx, token); !errors.Is(err, core.ErrUnauthorized) {
			t.Errorf("%s token: err = %v, want ErrUnauthorized", name, err)
		}
	}
}

func TestAuthenticate_Expired(t *testing.T) {
	f := newAccountsFixture(t)
	res, _ := f.login(t, "tester01", startPassword)

	*f.clock = f.clock.Add(30*24*time.Hour - time.Second)
	if _, _, err := f.accounts.Authenticate(context.Background(), res.Token); err != nil {
		t.Errorf("one second before expiry: %v", err)
	}
	*f.clock = f.clock.Add(time.Second)
	if _, _, err := f.accounts.Authenticate(context.Background(), res.Token); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("at expiry: err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthenticate_DisabledOrDeletedUser(t *testing.T) {
	f := newAccountsFixture(t)
	res, _ := f.login(t, "tester01", startPassword)

	u := f.store.users[f.user.ID]
	u.Disabled = true
	f.store.users[u.ID] = u
	if _, _, err := f.accounts.Authenticate(context.Background(), res.Token); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("disabled: err = %v, want ErrUnauthorized", err)
	}

	delete(f.store.users, u.ID)
	if _, _, err := f.accounts.Authenticate(context.Background(), res.Token); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("deleted: err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthenticate_DatabaseErrorIsNotUnauthorized(t *testing.T) {
	f := newAccountsFixture(t)
	res, _ := f.login(t, "tester01", startPassword)
	f.store.err = errors.New("timeout")
	_, _, err := f.accounts.Authenticate(context.Background(), res.Token)
	if err == nil || errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("err = %v, want the database error (500, not 401)", err)
	}
}

func TestLogout(t *testing.T) {
	f := newAccountsFixture(t)
	phone, _ := f.login(t, "tester01", startPassword)
	tablet, _ := f.login(t, "tester01", startPassword)
	ctx := context.Background()

	_, sid, _ := f.accounts.Authenticate(ctx, phone.Token)
	if err := f.accounts.Logout(ctx, sid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.accounts.Authenticate(ctx, phone.Token); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("after logout: err = %v, want ErrUnauthorized", err)
	}
	if _, _, err := f.accounts.Authenticate(ctx, tablet.Token); err != nil {
		t.Errorf("other device logged out too: %v", err)
	}
}

func TestChangePassword_Success(t *testing.T) {
	f := newAccountsFixture(t)
	phone, _ := f.login(t, "tester01", startPassword)
	tablet, _ := f.login(t, "tester01", startPassword)
	ctx := context.Background()
	u, sid, _ := f.accounts.Authenticate(ctx, phone.Token)

	if err := f.accounts.ChangePassword(ctx, u, sid, startPassword, "Sonnenblume am Mönchsberg", clientIP); err != nil {
		t.Fatal(err)
	}
	stored := f.store.users[u.ID]
	if stored.MustChangePassword {
		t.Error("mustChangePassword still true")
	}
	if _, _, err := f.accounts.Authenticate(ctx, phone.Token); err != nil {
		t.Errorf("the changing device was logged out: %v", err)
	}
	if _, _, err := f.accounts.Authenticate(ctx, tablet.Token); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("other device still logged in: err = %v", err)
	}
	if _, err := f.login(t, "tester01", startPassword); !errors.Is(err, core.ErrInvalidCredentials) {
		t.Errorf("old password still works: %v", err)
	}
	if _, err := f.login(t, "tester01", "Sonnenblume am Mönchsberg"); err != nil {
		t.Errorf("new password does not work: %v", err)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	f := newAccountsFixture(t)
	err := f.accounts.ChangePassword(context.Background(), f.user, "s1", "falschesPasswort", "Sonnenblume am Mönchsberg", clientIP)
	if !errors.Is(err, core.ErrWrongPassword) {
		t.Errorf("err = %v, want ErrWrongPassword", err)
	}
	if f.store.users[f.user.ID].PasswordHash != f.user.PasswordHash {
		t.Error("password changed despite a wrong current password")
	}
}

func TestChangePassword_Weak(t *testing.T) {
	cases := map[string]struct {
		newPassword string
		problem     core.PasswordProblem
	}{
		"too short": {"kurz", core.PasswordTooShort},
		"too long":  {strings.Repeat("ä", 37), core.PasswordTooLong},
		"unchanged": {startPassword, core.PasswordUnchanged},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newAccountsFixture(t)
			err := f.accounts.ChangePassword(context.Background(), f.user, "s1", startPassword, c.newPassword, clientIP)
			var we *core.WeakPasswordError
			if !errors.As(err, &we) || we.Problem != c.problem {
				t.Errorf("err = %v, want WeakPasswordError %s", err, c.problem)
			}
			if f.store.users[f.user.ID].PasswordHash != f.user.PasswordHash {
				t.Error("password changed")
			}
		})
	}
}

// Dylan's decision: a wrong current password counts like a failed login,
// with the same counters.
func TestChangePassword_WrongCurrentCountsAsFailedAttempt(t *testing.T) {
	f := newAccountsFixture(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		err := f.accounts.ChangePassword(ctx, f.user, "s1", "falschesPasswort", "Sonnenblume am Mönchsberg", clientIP)
		if !errors.Is(err, core.ErrWrongPassword) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	var tm *core.TooManyAttemptsError
	err := f.accounts.ChangePassword(ctx, f.user, "s1", startPassword, "Sonnenblume am Mönchsberg", clientIP)
	if !errors.As(err, &tm) {
		t.Fatalf("sixth change: err = %v, want TooManyAttemptsError", err)
	}
	if _, err := f.login(t, "tester01", startPassword); !errors.As(err, &tm) {
		t.Errorf("login after 5 wrong current passwords: err = %v, want TooManyAttemptsError", err)
	}
}

func TestUpdateDisplayName(t *testing.T) {
	f := newAccountsFixture(t)
	ctx := context.Background()

	u, err := f.accounts.UpdateDisplayName(ctx, f.user, "  Erika  ")
	if err != nil || u.DisplayName != "Erika" || f.store.users[f.user.ID].DisplayName != "Erika" {
		t.Errorf("UpdateDisplayName = %+v, %v", u, err)
	}
	if _, err := f.accounts.UpdateDisplayName(ctx, f.user, "   "); !errors.Is(err, core.ErrInvalidDisplayName) {
		t.Errorf("empty: err = %v, want ErrInvalidDisplayName", err)
	}
	delete(f.store.users, f.user.ID)
	if _, err := f.accounts.UpdateDisplayName(ctx, f.user, "Erika"); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("deleted user: err = %v, want ErrUnauthorized", err)
	}
}

func TestRecordConsent(t *testing.T) {
	f := newAccountsFixture(t)
	ctx := context.Background()
	if err := f.accounts.RecordConsent(ctx, f.user, "  2026-11-18 "); err != nil {
		t.Fatal(err)
	}
	u := f.store.users[f.user.ID]
	if u.ConsentVersion == nil || *u.ConsentVersion != "2026-11-18" {
		t.Errorf("consentVersion = %v", u.ConsentVersion)
	}
	// Server time, truncated to milliseconds, in UTC.
	if u.ConsentAt == nil || !u.ConsentAt.Equal(f.clock.UTC()) || u.ConsentAt.Location() != time.UTC {
		t.Errorf("consentAt = %v, want %v", u.ConsentAt, *f.clock)
	}
	if err := f.accounts.RecordConsent(ctx, f.user, "   "); !errors.Is(err, core.ErrInvalidConsentVersion) {
		t.Errorf("empty: err = %v, want ErrInvalidConsentVersion", err)
	}
	delete(f.store.users, f.user.ID)
	if err := f.accounts.RecordConsent(ctx, f.user, "v1"); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("deleted user: err = %v, want ErrUnauthorized", err)
	}
}
