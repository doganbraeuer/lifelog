package core

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// The user's time zone (docs/API.md 3.12, 3.13 and 3.16 "Time zone and
// points in time"). Routines compute every wall-clock time and every date
// in it. Daily limits and the import day of a text import stay on
// Europe/Vienna (usage.go).

// DefaultTimeZone applies to a user without the field timeZone.
const DefaultTimeZone = "Europe/Vienna"

// MaxTimeZoneChars: the longest IANA name has 32 characters
// ("America/Argentina/ComodRivadavia"); 64 leaves room.
const MaxTimeZoneChars = 64

// ErrInvalidTimeZone: the name is not an accepted IANA time zone
// (400 INVALID_REQUEST, details.field timeZone).
var ErrInvalidTimeZone = errors.New("unknown time zone")

// timeZoneAreas are the first parts of the IANA names of places. Names
// outside these areas are abbreviations or legacy names ("CET", "EST5EDT",
// "Japan", "US/Eastern") or other trees of the zone database ("posix/…",
// "right/…", "Etc/GMT+1").
var timeZoneAreas = map[string]bool{
	"Africa": true, "America": true, "Antarctica": true, "Arctic": true, "Asia": true,
	"Atlantic": true, "Australia": true, "Europe": true, "Indian": true, "Pacific": true,
}

var defaultLocation = mustLoadLocation(DefaultTimeZone)

// zoneCache keeps the locations of checked names. Every routine request
// needs the user's zone; without the cache each one would parse the zone
// data again. Only valid names get in, so it stays small (fewer than 600).
var zoneCache sync.Map // name → *time.Location

// CheckTimeZone checks a time zone name from a request and returns its
// location.
//
// time.LoadLocation alone is not enough:
//   - it accepts "" (UTC) and "Local" (the server's zone) without an error;
//   - it accepts abbreviations and legacy names that are files in the zone
//     database, e.g. "CET", "EST5EDT" and "Japan";
//   - it reads the operating system's zone files first and the embedded
//     copy (time/tzdata) only as a fallback. On Linux "posix/Europe/Vienna"
//     therefore loads too, and on macOS, whose file system ignores case,
//     "Europe/vienna".
//
// Accepted are therefore "UTC", "Etc/UTC" (some Android phones report it)
// and names Area/Place or Area/Place/Place with an area from timeZoneAreas,
// every part starting with an upper-case letter and made of ASCII letters,
// digits, "_" and "-", at most 64 characters, and time.LoadLocation must
// load it. The name is stored exactly as sent.
func CheckTimeZone(name string) (*time.Location, error) {
	if loc, ok := zoneCache.Load(name); ok {
		return loc.(*time.Location), nil
	}
	if !wellFormedTimeZone(name) {
		return nil, ErrInvalidTimeZone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, ErrInvalidTimeZone
	}
	zoneCache.Store(name, loc)
	return loc, nil
}

// wellFormedTimeZone checks the form of a name, without the zone database.
func wellFormedTimeZone(name string) bool {
	if name == "UTC" || name == "Etc/UTC" {
		return true
	}
	if len(name) > MaxTimeZoneChars {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) < 2 || len(parts) > 3 || !timeZoneAreas[parts[0]] {
		return false
	}
	for _, p := range parts {
		if p == "" || p[0] < 'A' || p[0] > 'Z' {
			return false
		}
		for i := 1; i < len(p); i++ {
			c := p[i]
			if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '_' || c == '-') {
				return false
			}
		}
	}
	return true
}

// UserTimeZone returns the name and the location of the user's time zone.
// Without the field it is Europe/Vienna. A stored name was checked by
// CheckTimeZone when it was set; should it no longer load (zone data of a
// different build), Europe/Vienna applies, so that name and location always
// belong together.
func UserTimeZone(u User) (string, *time.Location) {
	if u.TimeZone != "" {
		if loc, err := CheckTimeZone(u.TimeZone); err == nil {
			return u.TimeZone, loc
		}
	}
	return DefaultTimeZone, defaultLocation
}
