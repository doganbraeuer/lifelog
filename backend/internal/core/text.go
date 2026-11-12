package core

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Text import (API.md 3.1): the text is stored as text.txt.
const (
	MaxTextChars = 100_000 // Unicode characters (runes), not bytes
	TextFileName = "text.txt"
	MimeText     = "text/plain; charset=utf-8"
)

var (
	ErrTextNotUTF8 = errors.New("text is not valid UTF-8")
	ErrTextTooLong = errors.New("text is longer than 100,000 characters")
)

// CheckText checks an entered text. It is not changed; exactly what arrives
// is stored and hashed.
//
// Errors, in this order: ErrFileEmpty (empty or only whitespace, →
// 400 FILE_MISSING), ErrTextNotUTF8 (→ 400 INVALID_REQUEST),
// ErrTextTooLong (→ 413 FILE_TOO_LARGE).
func CheckText(text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrFileEmpty
	}
	if !utf8.ValidString(text) {
		return ErrTextNotUTF8
	}
	// Characters, not bytes: "ü" is one character but two bytes.
	if utf8.RuneCountInString(text) > MaxTextChars {
		return ErrTextTooLong
	}
	return nil
}
