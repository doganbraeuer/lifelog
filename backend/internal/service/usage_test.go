package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// memUsageStore counts like store.IncrementUsage: per user, day and kind,
// only below max.
type memUsageStore struct {
	counts    map[string]int
	expiresAt time.Time
	err       error
}

func (m *memUsageStore) IncrementUsage(_ context.Context, userID, day string, kind core.LimitKind, max int, expiresAt time.Time) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	key := userID + "/" + day + "/" + string(kind)
	if m.counts[key] >= max {
		return false, nil
	}
	m.counts[key]++
	m.expiresAt = expiresAt
	return true, nil
}

func newTestUsage(limits Limits) (*Usage, *memUsageStore, *time.Time) {
	store := &memUsageStore{counts: map[string]int{}}
	now := time.Date(2026, 11, 18, 22, 0, 0, 0, time.UTC) // 23:00 in Vienna
	u := NewUsage(store, limits)
	u.now = func() time.Time { return now }
	return u, store, &now
}

func TestUsage_LimitReached(t *testing.T) {
	u, _, _ := newTestUsage(Limits{Uploads: 2, Questions: 60, Transcriptions: 30})
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := u.Use(ctx, testUser, core.LimitUploads); err != nil {
			t.Fatalf("upload %d: %v", i+1, err)
		}
	}
	err := u.Use(ctx, testUser, core.LimitUploads)
	var le *core.LimitReachedError
	if !errors.As(err, &le) || le.Limit != core.LimitUploads {
		t.Fatalf("err = %v, want LimitReachedError uploads", err)
	}
	// Other limits and other users are counted separately.
	if err := u.Use(ctx, testUser, core.LimitQuestions); err != nil {
		t.Errorf("questions: %v", err)
	}
	if err := u.Use(ctx, otherUser, core.LimitUploads); err != nil {
		t.Errorf("other user: %v", err)
	}
}

// A new day in Vienna starts at 0, also if it is still the same day in UTC.
func TestUsage_NewDayStartsAtZero(t *testing.T) {
	u, store, now := newTestUsage(Limits{Uploads: 1, Questions: 1, Transcriptions: 1})
	ctx := context.Background()
	*now = time.Date(2026, 11, 18, 22, 59, 59, 0, time.UTC) // 23:59:59 in Vienna
	if err := u.Use(ctx, testUser, core.LimitQuestions); err != nil {
		t.Fatal(err)
	}
	if err := u.Use(ctx, testUser, core.LimitQuestions); err == nil {
		t.Fatal("second question of the day allowed")
	}
	*now = time.Date(2026, 11, 18, 23, 0, 0, 0, time.UTC) // 00:00 on 19 November in Vienna
	if err := u.Use(ctx, testUser, core.LimitQuestions); err != nil {
		t.Errorf("first question of the new day: %v", err)
	}
	if store.counts[testUser+"/2026-11-19/questions"] != 1 {
		t.Errorf("counts = %v", store.counts)
	}
	// Kept two days after the day ends (midnight 19/20 November in Vienna).
	if want := time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC); !store.expiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v", store.expiresAt, want)
	}
}

func TestUsage_Errors(t *testing.T) {
	u, store, _ := newTestUsage(Limits{Uploads: 30, Questions: 60, Transcriptions: 30})
	if err := u.Use(context.Background(), "", core.LimitUploads); !errors.Is(err, core.ErrNoUser) {
		t.Errorf("no user: err = %v, want ErrNoUser", err)
	}
	store.err = errDB
	if err := u.Use(context.Background(), testUser, core.LimitUploads); !errors.Is(err, errDB) {
		t.Errorf("database down: err = %v, want the error (fail closed)", err)
	}
	store.err = nil
	var le *core.LimitReachedError
	if err := u.Use(context.Background(), testUser, core.LimitKind("unknown")); !errors.As(err, &le) {
		t.Errorf("unknown limit: err = %v, want refused", err)
	}
}
