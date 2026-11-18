package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword_CostAndCompare(t *testing.T) {
	hash, err := HashPassword("Sonnenblume am Mönchsberg")
	if err != nil {
		t.Fatal(err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil || cost != 12 {
		t.Errorf("cost = %d, %v; want 12", cost, err)
	}
	if strings.Contains(hash, "Sonnenblume") {
		t.Error("hash must not contain the password")
	}
	if !ComparePassword(hash, "Sonnenblume am Mönchsberg") {
		t.Error("correct password not accepted")
	}
	if ComparePassword(hash, "Sonnenblume am Mönchsberg ") {
		t.Error("password with an extra space accepted")
	}
	if ComparePassword(hash, "") {
		t.Error("empty password accepted")
	}
}

func TestHashPassword_SaltDiffers(t *testing.T) {
	// Fast cost: only the salt matters here.
	a, _ := bcrypt.GenerateFromPassword([]byte("gleichesPasswort"), bcrypt.MinCost)
	b, _ := bcrypt.GenerateFromPassword([]byte("gleichesPasswort"), bcrypt.MinCost)
	if string(a) == string(b) {
		t.Error("two hashes of the same password must differ (salt)")
	}
}

func TestHashPassword_TooLong(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("err = %v, want ErrPasswordTooLong", err)
	}
	if _, err := HashPassword(strings.Repeat("ä", 37)); !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("37 umlauts (74 bytes): err = %v, want ErrPasswordTooLong", err)
	}
}

// bcrypt itself only compares the first 72 bytes. ComparePassword must not
// accept "the right 72 bytes plus anything".
func TestComparePassword_Over72BytesNeverMatches(t *testing.T) {
	first72 := strings.Repeat("k", 72)
	h, err := bcrypt.GenerateFromPassword([]byte(first72), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if !ComparePassword(string(h), first72) {
		t.Fatal("exactly 72 bytes must match")
	}
	if ComparePassword(string(h), first72+"egal") {
		t.Error("72 bytes plus a suffix must not match")
	}
}

func TestDummyHash_IsValidWithCost12(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyHash))
	if err != nil {
		t.Fatalf("dummy hash is not a bcrypt hash: %v", err)
	}
	// The same cost as real hashes, otherwise unknown usernames would be
	// faster or slower than known ones.
	if cost != Cost {
		t.Errorf("dummy cost = %d, want %d", cost, Cost)
	}
	CompareDummy("irgendwas") // must not panic
}
