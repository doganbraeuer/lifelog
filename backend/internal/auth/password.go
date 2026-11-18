// Package auth holds the building blocks for accounts and login
// (docs/API.md, section 1 "Login and tokens"): password hashes, tokens,
// generated passwords, the counter for failed attempts, the client IP and
// the user in the request context.
//
// Nothing in this package logs a password, a token or a hash.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"lifelog/backend/internal/core"
)

// Cost is the bcrypt cost factor: 2^12 rounds, about a quarter of a second
// per hash on a normal computer. That makes guessing slow and a login still
// fast enough.
const Cost = 12

// ErrPasswordTooLong: bcrypt only uses the first 72 bytes. A longer password
// would be cut silently, so it is refused before hashing.
var ErrPasswordTooLong = errors.New("password longer than 72 bytes")

// dummyHash is a bcrypt hash (cost 12) of a random secret that was thrown
// away after hashing. A login with an unknown username is compared against
// it, so it takes as long as a login with a known one, and the response time
// does not reveal whether an account exists (API.md 3.10).
const dummyHash = "$2a$12$ZjFMHHYHNXn5EXzRqI0aieiroRG5lmb8i5EJUDA2S0Z5asxy/qR5."

// HashPassword returns the bcrypt hash (with its own random salt). The
// caller checks the password rules first (core.CheckPassword); the 72-byte
// limit is checked here again, because a cut password would be a silent
// security bug.
func HashPassword(password string) (string, error) {
	if len(password) > core.MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), Cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// ComparePassword reports whether password belongs to hash. A password over
// 72 bytes never matches, even if its first 72 bytes do: bcrypt would only
// compare those. The comparison still runs, so it takes the same time.
func ComparePassword(hash, password string) bool {
	ok := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	return ok && len(password) <= core.MaxPasswordBytes
}

// CompareDummy spends the time of one comparison without a real account
// (unknown username). The result is meaningless and ignored.
func CompareDummy(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
}
