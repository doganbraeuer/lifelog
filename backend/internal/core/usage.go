package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Europe/Vienna also on servers without installed time zone data
	"unicode"
	"unicode/utf8"
)

// Per-user data, daily limits and consent (docs/API.md, section 1 "Per-user
// data" and "Daily limits", 3.15).

// ErrNoUser: a function that reads or changes entries got no user ID. That
// is a programming error, never a reason to return "all" entries; the API
// answers 500.
var ErrNoUser = errors.New("no user ID given: refusing to access entries without an owner filter")

// ErrSearchUpdating: the vector index is being rebuilt (e.g. after its
// definition was updated). Questions answer 503 until it is READY again
// (API.md 3.6).
var ErrSearchUpdating = errors.New("search index is being rebuilt")

// LimitKind names a daily limit, as in details.limit (API.md, daily limits).
type LimitKind string

const (
	LimitUploads        LimitKind = "uploads"
	LimitQuestions      LimitKind = "questions"
	LimitTranscriptions LimitKind = "transcriptions"
)

// LimitReachedError: the user has reached the daily limit Limit
// (429 LIMIT_REACHED).
type LimitReachedError struct {
	Limit LimitKind
}

func (e *LimitReachedError) Error() string {
	return fmt.Sprintf("daily limit %q reached", string(e.Limit))
}

// usageZone: the calendar day of the limits is the one in Austria, so the
// counters reset at midnight local time, also on a server running in UTC.
var usageZone = mustLoadLocation("Europe/Vienna")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// Cannot happen: time/tzdata is embedded above.
		panic(fmt.Sprintf("time zone %s: %v", name, err))
	}
	return loc
}

// UsageDay returns the calendar day of t in Europe/Vienna as "YYYY-MM-DD",
// the key of the daily counters.
func UsageDay(t time.Time) string {
	return t.In(usageZone).Format(time.DateOnly)
}

// UsageDayEnd returns the first moment after the day of t in Europe/Vienna
// (midnight local time, in UTC). Counters may be deleted some time after it.
func UsageDayEnd(t time.Time) time.Time {
	local := t.In(usageZone)
	y, m, d := local.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, usageZone).UTC()
}

// MaxConsentVersionChars: a version names a text, e.g. "2026-11-18".
const MaxConsentVersionChars = 100

// ErrInvalidConsentVersion: the version is empty, too long or contains
// control characters (400 INVALID_REQUEST, API.md 3.15).
var ErrInvalidConsentVersion = errors.New("consent version must have 1 to 100 characters without control characters")

// NormalizeConsentVersion removes whitespace at the edges and checks the
// rest: 1 to 100 characters, valid UTF-8, no control characters.
func NormalizeConsentVersion(s string) (string, error) {
	v := strings.TrimSpace(s)
	if !utf8.ValidString(v) {
		return "", ErrInvalidConsentVersion
	}
	n := utf8.RuneCountInString(v)
	if n < 1 || n > MaxConsentVersionChars {
		return "", ErrInvalidConsentVersion
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return "", ErrInvalidConsentVersion
		}
	}
	return v, nil
}
