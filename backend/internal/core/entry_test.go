package core

import (
	"errors"
	"testing"
)

func TestPruefeID(t *testing.T) {
	for _, ok := range []string{"6727a1f3c2b4e5d6f7a8b9c0", "000000000000000000000000", "6727A1F3C2B4E5D6F7A8B9C0"} {
		if err := PruefeID(ok); err != nil {
			t.Errorf("PruefeID(%q) = %v", ok, err)
		}
	}
	for _, falsch := range []string{
		"", "abc", "6727a1f3c2b4e5d6f7a8b9c", "6727a1f3c2b4e5d6f7a8b9c00",
		"6727a1f3c2b4e5d6f7a8b9cg", "6727a1f3c2b4e5d6f7a8b9c ", "../../../../etc/passwd00",
		"6727a1f3c2b4e5d6f7a8b9ä", // 24 Bytes, aber kein Hex
	} {
		if err := PruefeID(falsch); !errors.Is(err, ErrUngueltigeID) {
			t.Errorf("PruefeID(%q) = %v, erwartet ErrUngueltigeID", falsch, err)
		}
	}
}
