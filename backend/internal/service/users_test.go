package service

import (
	"context"
	"testing"

	"lifelog/backend/internal/core"
)

// Two users for the service tests. testUser owns the data of the fakes;
// otherUser must never see or change it.
const (
	testUser  = "6730bbbbbbbbbbbbbbbbbbbb"
	otherUser = "6730cccccccccccccccccccc"
)

// fakeUsage counts Use calls per limit. limit > 0 refuses from the
// (limit+1)-th call of a kind on, like the real counter. err, if set, is
// returned instead (database down).
type fakeUsage struct {
	calls map[core.LimitKind]int
	users []string
	limit int
	err   error
}

func newFakeUsage() *fakeUsage {
	return &fakeUsage{calls: map[core.LimitKind]int{}}
}

func (u *fakeUsage) Use(_ context.Context, userID string, kind core.LimitKind) error {
	u.users = append(u.users, userID)
	if u.err != nil {
		return u.err
	}
	if u.limit > 0 && u.calls[kind] >= u.limit {
		return &core.LimitReachedError{Limit: kind}
	}
	u.calls[kind]++
	return nil
}

// onlyUser fails the test if any of the recorded user IDs is not want.
func onlyUser(t *testing.T, what string, got []string, want string) {
	t.Helper()
	if len(got) == 0 {
		t.Errorf("%s: no user ID recorded", what)
	}
	for _, u := range got {
		if u != want {
			t.Errorf("%s called with user %q, want %q", what, u, want)
		}
	}
}
