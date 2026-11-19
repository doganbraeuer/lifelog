package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// tokenBytes: 32 random bytes = 256 bits; guessing a token is hopeless.
const tokenBytes = 32

// NewToken returns a new token: 32 bytes from crypto/rand as base64url
// without padding (43 characters, only A-Z, a-z, 0-9, - and _). It goes to
// the client exactly once, in the login response.
//
// crypto/rand.Read never fails since Go 1.24 (the program stops if the
// operating system cannot deliver random bytes), so there is no error.
func NewToken() string {
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken returns the SHA-256 hash of the token as 64 hex characters.
// Only this hash is stored (sessions.tokenHash). A plain hash without salt
// is enough here, unlike for passwords: the token is 256 random bits, so
// there is nothing to guess with a dictionary.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
