package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Accounts and login (docs/API.md, section 1 "Login and tokens" and
// 3.10–3.15; names from docs/GLOSSARY.md, "Accounts and login").

// Role of a user. In this version the role only decides how cmd/users
// creates and resets an account; no endpoint checks it yet.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User is an account (collection users). PasswordHash never leaves the
// backend: no JSON type contains it, and it is never logged.
type User struct {
	ID                 string
	Username           string
	DisplayName        string
	PasswordHash       string
	Role               Role
	MustChangePassword bool
	Disabled           bool
	CreatedAt          time.Time
	// ConsentVersion and ConsentAt are stored, but nothing sets them yet
	// (PUT /me/consent comes in a later PR). nil = no consent recorded.
	ConsentVersion *string
	ConsentAt      *time.Time
}

// Session is one login on one device (collection sessions). Only the hash
// of the token is stored, never the token itself.
type Session struct {
	ID        string
	TokenHash string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// LoginResult is what a successful login returns (API.md 3.10). Token is the
// only place where the token exists in plain text; it goes to the client
// once and is never stored or logged.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      User
}

// AuditEntry is one administrative action of cmd/users (collection
// auditLog). Never contains passwords, hashes or tokens.
type AuditEntry struct {
	Time           time.Time
	Action         string
	TargetUsername string // empty for actions without a single target
	Operator       string // "<os user>@<host>"
}

// Values of AuditEntry.Action (docs/GLOSSARY.md).
const (
	AuditAdd               = "add"
	AuditResetPassword     = "reset_password"
	AuditDisable           = "disable"
	AuditEnable            = "enable"
	AuditRevokeSessions    = "revoke_sessions"
	AuditRevokeAllSessions = "revoke_all_sessions"
	AuditList              = "list"
	AuditDelete            = "delete"
	AuditAssignOrphans     = "assign_orphans"
)

// SessionLifetime: a token is valid for 30 days from the login and is not
// extended by using it (API.md, section 1).
const SessionLifetime = 30 * 24 * time.Hour

// Limits (API.md 3.10, 3.12–3.14).
const (
	MinUsernameLength   = 3
	MaxUsernameLength   = 32
	MinPasswordChars    = 10
	MaxPasswordBytes    = 72 // bcrypt only uses the first 72 bytes
	MaxDisplayNameChars = 50
)

var (
	// ErrInvalidCredentials: login failed. Deliberately one error for unknown
	// username, wrong password and disabled account (401 INVALID_CREDENTIALS).
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrUnauthorized: token missing, unknown or expired, or the account is
	// disabled or gone (401 UNAUTHORIZED).
	ErrUnauthorized = errors.New("unauthorized")

	// ErrWrongPassword: the current password at a password change is wrong
	// (400 WRONG_PASSWORD).
	ErrWrongPassword = errors.New("current password is wrong")

	ErrUserNotFound       = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrInvalidUsername    = errors.New("username must have 3 to 32 characters, only a-z and 0-9")
	ErrInvalidDisplayName = errors.New("display name must have 1 to 50 characters without control characters")
)

// PasswordProblem says why a new password is rejected.
type PasswordProblem string

const (
	PasswordTooShort  PasswordProblem = "too_short" // fewer than 10 characters
	PasswordTooLong   PasswordProblem = "too_long"  // more than 72 bytes
	PasswordUnchanged PasswordProblem = "unchanged" // equal to the current one
)

// WeakPasswordError: a new password breaks a rule (400 WEAK_PASSWORD).
type WeakPasswordError struct {
	Problem PasswordProblem
}

func (e *WeakPasswordError) Error() string {
	switch e.Problem {
	case PasswordTooShort:
		return fmt.Sprintf("password must have at least %d characters", MinPasswordChars)
	case PasswordTooLong:
		return fmt.Sprintf("password must have at most %d bytes", MaxPasswordBytes)
	case PasswordUnchanged:
		return "new password must differ from the current one"
	}
	return "weak password"
}

// TooManyAttemptsError: too many failed attempts, try again after
// RetryAfter (429 TOO_MANY_ATTEMPTS).
type TooManyAttemptsError struct {
	RetryAfter time.Duration
}

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many failed attempts, retry after %d s", e.RetryAfterSeconds())
}

// RetryAfterSeconds is the wait in whole seconds, rounded up and at least 1,
// for the header Retry-After and details.retryAfterSeconds.
func (e *TooManyAttemptsError) RetryAfterSeconds() int {
	s := int((e.RetryAfter + time.Second - 1) / time.Second)
	return max(s, 1)
}

// NormalizeUsername prepares a typed username for the lookup: whitespace at
// the edges removed, lowercase. "Tester01 " (phone keyboards start with a
// capital letter) becomes "tester01" (API.md 3.10).
func NormalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// CheckUsername checks the stored form: 3 to 32 characters, only a-z and
// 0-9. Call NormalizeUsername first for typed input.
func CheckUsername(username string) error {
	if len(username) < MinUsernameLength || len(username) > MaxUsernameLength {
		return ErrInvalidUsername
	}
	for i := 0; i < len(username); i++ {
		c := username[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ErrInvalidUsername
		}
	}
	return nil
}

// CheckPassword checks the rules for a password that is about to be stored:
// at least 10 characters (Unicode characters, not bytes) and at most 72
// bytes in UTF-8, because bcrypt ignores everything after 72 bytes. The
// password is not trimmed; spaces count.
func CheckPassword(password string) error {
	if len(password) > MaxPasswordBytes {
		return &WeakPasswordError{Problem: PasswordTooLong}
	}
	if utf8.RuneCountInString(password) < MinPasswordChars {
		return &WeakPasswordError{Problem: PasswordTooShort}
	}
	return nil
}

// CheckNewPassword checks a password change: the rules of CheckPassword,
// and the new password must differ from the current one (API.md 3.14).
func CheckNewPassword(current, newPassword string) error {
	if err := CheckPassword(newPassword); err != nil {
		return err
	}
	if newPassword == current {
		return &WeakPasswordError{Problem: PasswordUnchanged}
	}
	return nil
}

// NormalizeDisplayName removes whitespace at the edges and checks the rest:
// 1 to 50 characters, valid UTF-8, no control characters such as line
// breaks or tabs (API.md 3.13). Control characters are refused because the
// name ends up in cmd/users list and later in the app.
func NormalizeDisplayName(s string) (string, error) {
	name := strings.TrimSpace(s)
	if !utf8.ValidString(name) {
		return "", ErrInvalidDisplayName
	}
	n := utf8.RuneCountInString(name)
	if n < 1 || n > MaxDisplayNameChars {
		return "", ErrInvalidDisplayName
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrInvalidDisplayName
		}
	}
	return name, nil
}
