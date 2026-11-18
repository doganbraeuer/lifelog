package auth

import (
	"context"
	"testing"

	"lifelog/backend/internal/core"
)

func TestIdentityInContext(t *testing.T) {
	if _, ok := UserFrom(context.Background()); ok {
		t.Error("empty context must have no user")
	}
	if _, ok := SessionIDFrom(context.Background()); ok {
		t.Error("empty context must have no session")
	}

	u := core.User{ID: "673b0c1fa2d4e5f6a7b8c9d0", Username: "tester01"}
	ctx := WithIdentity(context.Background(), u, "673b0c1fa2d4e5f6a7b8c9d1")
	got, ok := UserFrom(ctx)
	if !ok || got.ID != u.ID || got.Username != "tester01" {
		t.Errorf("UserFrom = %+v, %v", got, ok)
	}
	sid, ok := SessionIDFrom(ctx)
	if !ok || sid != "673b0c1fa2d4e5f6a7b8c9d1" {
		t.Errorf("SessionIDFrom = %q, %v", sid, ok)
	}
}
