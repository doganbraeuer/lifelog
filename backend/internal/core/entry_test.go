package core

import (
	"errors"
	"testing"
)

func TestCheckID(t *testing.T) {
	for _, ok := range []string{"6727a1f3c2b4e5d6f7a8b9c0", "000000000000000000000000", "6727A1F3C2B4E5D6F7A8B9C0"} {
		if err := CheckID(ok); err != nil {
			t.Errorf("CheckID(%q) = %v", ok, err)
		}
	}
	for _, invalid := range []string{
		"", "abc", "6727a1f3c2b4e5d6f7a8b9c", "6727a1f3c2b4e5d6f7a8b9c00",
		"6727a1f3c2b4e5d6f7a8b9cg", "6727a1f3c2b4e5d6f7a8b9c ", "../../../../etc/passwd00",
		"6727a1f3c2b4e5d6f7a8b9ä", // 24 bytes, but not hex
	} {
		if err := CheckID(invalid); !errors.Is(err, ErrInvalidID) {
			t.Errorf("CheckID(%q) = %v, want ErrInvalidID", invalid, err)
		}
	}
}
