package core

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckTimeZone(t *testing.T) {
	valid := []string{
		"Europe/Vienna",
		"America/New_York",
		"UTC",
		"Etc/UTC",       // some Android phones report it
		"Asia/Calcutta", // legacy name some Android versions still send
		"Asia/Kolkata",
		"America/Argentina/Buenos_Aires", // three parts
		"America/Port-au-Prince",         // hyphen
		"Australia/Lord_Howe",
		"Antarctica/DumontDUrville",
		"Europe/Lisbon",
	}
	for _, name := range valid {
		loc, err := CheckTimeZone(name)
		if err != nil || loc == nil {
			t.Errorf("CheckTimeZone(%q) = %v, %v; want valid", name, loc, err)
			continue
		}
		if loc.String() != name {
			t.Errorf("CheckTimeZone(%q) gives location %q", name, loc.String())
		}
	}

	invalid := []string{
		"",                    // LoadLocation would give UTC
		"Local",               // LoadLocation would give the server's zone
		"CET",                 // abbreviation, but a file in the zone database
		"EST5EDT",             // POSIX-style name
		"+01:00",              // offset
		"Japan",               // legacy name without area
		"US/Eastern",          // legacy tree
		"GMT",                 // only UTC is accepted
		"Etc/GMT+1",           // Etc is not an area of places
		"posix/Europe/Vienna", // would load from Linux's zone directory
		"right/Europe/Vienna",
		"europe/vienna", // would load on macOS
		"Europe/vienna",
		"Europe/Atlantis",
		"Europe/" + strings.Repeat("A", 58), // 65 characters
		"../x",
		"Europe/../Vienna",
		"Europe//Vienna",
		"Europe/",
		"/Europe/Vienna",
		"Europe\\Vienna",
		" Europe/Vienna",
		"Europe/Vienna ",
		"America/Argentina/Buenos_Aires/X", // four parts
		"Europe/Wien",
	}
	for _, name := range invalid {
		if loc, err := CheckTimeZone(name); !errors.Is(err, ErrInvalidTimeZone) || loc != nil {
			t.Errorf("CheckTimeZone(%q) = %v, %v; want ErrInvalidTimeZone", name, loc, err)
		}
	}
}

// A second call comes from the cache and gives the same location.
func TestCheckTimeZone_Cache(t *testing.T) {
	a, err := CheckTimeZone("Pacific/Auckland")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CheckTimeZone("Pacific/Auckland")
	if err != nil || a != b {
		t.Errorf("second call: %p, %v; want %p", b, err, a)
	}
}

func TestUserTimeZone(t *testing.T) {
	cases := []struct {
		stored, want string
	}{
		{"", DefaultTimeZone}, // never set
		{"America/New_York", "America/New_York"},
		{"Mars/Olympus_Mons", DefaultTimeZone}, // no longer loadable: Vienna, name and location together
	}
	for _, c := range cases {
		name, loc := UserTimeZone(User{TimeZone: c.stored})
		if name != c.want || loc.String() != c.want {
			t.Errorf("UserTimeZone(%q) = %q, %q; want %q", c.stored, name, loc, c.want)
		}
	}
}
