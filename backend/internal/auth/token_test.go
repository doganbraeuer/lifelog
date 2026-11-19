package auth

import (
	"encoding/base64"
	"regexp"
	"testing"
)

func TestNewToken_FormatAndUnique(t *testing.T) {
	format := regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		tok := NewToken()
		if !format.MatchString(tok) {
			t.Fatalf("token %q is not 43 characters of base64url", tok)
		}
		raw, err := base64.RawURLEncoding.DecodeString(tok)
		if err != nil || len(raw) != 32 {
			t.Fatalf("token decodes to %d bytes, %v; want 32", len(raw), err)
		}
		if seen[tok] {
			t.Fatal("token repeated")
		}
		seen[tok] = true
	}
}

func TestHashToken(t *testing.T) {
	// SHA-256 of "abc", a well-known test vector.
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := HashToken("abc"); got != want {
		t.Errorf("HashToken(abc) = %s, want %s", got, want)
	}
	tok := NewToken()
	if HashToken(tok) == tok || len(HashToken(tok)) != 64 {
		t.Error("the hash must be 64 hex characters and differ from the token")
	}
	if HashToken(tok) != HashToken(tok) {
		t.Error("the hash must be deterministic")
	}
}
