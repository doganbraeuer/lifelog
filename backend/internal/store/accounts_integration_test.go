package store

// Only runs with LIFELOG_MONGO_TEST=1 (see integration_test.go). Only the
// account tests:
//
//	LIFELOG_MONGO_TEST=1 go test ./internal/store/ -run IntegrationAccounts -v
//
// They create no search index. All users are synthetic.

import (
	"errors"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

func testUser(username string) core.User {
	return core.User{
		ID:                 NewID(),
		Username:           username,
		DisplayName:        username,
		PasswordHash:       "$2a$04$notarealhashnotarealhashnotarealhashnotarealhashnot",
		Role:               core.RoleUser,
		MustChangePassword: true,
		CreatedAt:          time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC),
	}
}

func TestIntegrationAccounts_UserRoundTripAndUniqueUsername(t *testing.T) {
	s, ctx := testStore(t)

	u := testUser("tester01")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.UserByUsername(ctx, "tester01")
	if err != nil || !found {
		t.Fatalf("UserByUsername = %v, %v", found, err)
	}
	if got.ID != u.ID || got.Role != core.RoleUser || !got.MustChangePassword || got.Disabled ||
		got.PasswordHash != u.PasswordHash || !got.CreatedAt.Equal(u.CreatedAt) {
		t.Errorf("user = %+v, want %+v", got, u)
	}
	if got.ConsentVersion != nil || got.ConsentAt != nil {
		t.Error("consent must be empty for a new user")
	}

	byID, found, err := s.UserByID(ctx, u.ID)
	if err != nil || !found || byID.Username != "tester01" {
		t.Errorf("UserByID = %+v, %v, %v", byID, found, err)
	}

	second := testUser("tester01")
	if err := s.CreateUser(ctx, second); !errors.Is(err, core.ErrUsernameTaken) {
		t.Errorf("second tester01: err = %v, want ErrUsernameTaken", err)
	}

	if _, found, _ := s.UserByUsername(ctx, "tester02"); found {
		t.Error("unknown username found")
	}
}

func TestIntegrationAccounts_UpdateAndList(t *testing.T) {
	s, ctx := testStore(t)
	b, a := testUser("bbb"), testUser("aaa")
	for _, u := range []core.User{b, a} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if found, err := s.SetPassword(ctx, a.ID, "$2a$04$other", false); err != nil || !found {
		t.Fatalf("SetPassword = %v, %v", found, err)
	}
	if found, err := s.SetDisplayName(ctx, a.ID, "Erika"); err != nil || !found {
		t.Fatalf("SetDisplayName = %v, %v", found, err)
	}
	if found, err := s.SetDisabled(ctx, a.ID, true); err != nil || !found {
		t.Fatalf("SetDisabled = %v, %v", found, err)
	}
	got, _, _ := s.UserByID(ctx, a.ID)
	if got.PasswordHash != "$2a$04$other" || got.MustChangePassword || got.DisplayName != "Erika" || !got.Disabled {
		t.Errorf("after updates: %+v", got)
	}

	list, err := s.ListUsers(ctx)
	if err != nil || len(list) != 2 || list[0].Username != "aaa" || list[1].Username != "bbb" {
		t.Errorf("ListUsers = %+v, %v; want aaa, bbb", list, err)
	}

	if found, err := s.DeleteUser(ctx, a.ID); err != nil || !found {
		t.Errorf("DeleteUser = %v, %v", found, err)
	}
	if found, _ := s.SetDisabled(ctx, a.ID, false); found {
		t.Error("deleted user still updatable")
	}
}

func TestIntegrationAccounts_Sessions(t *testing.T) {
	s, ctx := testStore(t)
	u, other := testUser("tester01"), testUser("tester02")
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	mk := func(userID, hash string) core.Session {
		return core.Session{ID: NewID(), TokenHash: hash, UserID: userID,
			CreatedAt: now, ExpiresAt: now.Add(core.SessionLifetime)}
	}
	phone, tablet, laptop := mk(u.ID, "hash-phone"), mk(u.ID, "hash-tablet"), mk(u.ID, "hash-laptop")
	foreign := mk(other.ID, "hash-other")
	for _, sess := range []core.Session{phone, tablet, laptop, foreign} {
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatal(err)
		}
	}

	got, found, err := s.SessionByTokenHash(ctx, "hash-phone")
	if err != nil || !found || got.ID != phone.ID || got.UserID != u.ID || !got.ExpiresAt.Equal(phone.ExpiresAt) {
		t.Errorf("SessionByTokenHash = %+v, %v, %v", got, found, err)
	}
	if _, found, _ := s.SessionByTokenHash(ctx, "unknown"); found {
		t.Error("unknown hash found")
	}

	// Password change on the phone: tablet and laptop end, phone and the
	// other user stay.
	n, err := s.DeleteOtherSessions(ctx, u.ID, phone.ID)
	if err != nil || n != 2 {
		t.Errorf("DeleteOtherSessions = %d, %v; want 2", n, err)
	}
	for hash, want := range map[string]bool{"hash-phone": true, "hash-tablet": false, "hash-laptop": false, "hash-other": true} {
		if _, found, _ := s.SessionByTokenHash(ctx, hash); found != want {
			t.Errorf("%s found = %v, want %v", hash, found, want)
		}
	}

	if err := s.DeleteSession(ctx, phone.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(ctx, phone.ID); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if n, err := s.DeleteSessionsOfUser(ctx, other.ID); err != nil || n != 1 {
		t.Errorf("DeleteSessionsOfUser = %d, %v; want 1", n, err)
	}

	if err := s.CreateSession(ctx, mk(u.ID, "hash-a")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.DeleteAllSessions(ctx); err != nil || n != 1 {
		t.Errorf("DeleteAllSessions = %d, %v; want 1", n, err)
	}
}

func TestIntegrationAccounts_Indexes(t *testing.T) {
	s, ctx := testStore(t)

	specs, err := s.sessions.Indexes().ListSpecifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]bool{}
	for _, spec := range specs {
		byName[spec.Name] = true
		switch spec.Name {
		case "expires_at_ttl":
			if spec.ExpireAfterSeconds == nil || *spec.ExpireAfterSeconds != 0 {
				t.Errorf("expires_at_ttl: expireAfterSeconds = %v, want 0", spec.ExpireAfterSeconds)
			}
		case "token_hash_unique":
			if spec.Unique == nil || !*spec.Unique {
				t.Error("token_hash_unique is not unique")
			}
		}
	}
	for _, name := range []string{"expires_at_ttl", "token_hash_unique", "userId"} {
		if !byName[name] {
			t.Errorf("index %s missing on sessions", name)
		}
	}

	// The same token hash twice must be refused.
	now := time.Now()
	sess := core.Session{ID: NewID(), TokenHash: "same", UserID: NewID(), CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	sess.ID = NewID()
	if err := s.CreateSession(ctx, sess); err == nil {
		t.Error("duplicate token hash accepted")
	}
}

func TestIntegrationAccounts_AuditLog(t *testing.T) {
	s, ctx := testStore(t)
	err := s.WriteAudit(ctx, core.AuditEntry{
		Time: time.Now(), Action: core.AuditAdd, TargetUsername: "tester01", Operator: "dylan@pc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteAudit(ctx, core.AuditEntry{Time: time.Now(), Action: core.AuditRevokeAllSessions, Operator: "dylan@pc"}); err != nil {
		t.Fatal(err)
	}
	n, err := s.auditLog.CountDocuments(ctx, map[string]any{})
	if err != nil || n != 2 {
		t.Errorf("audit entries = %d, %v; want 2", n, err)
	}
}
