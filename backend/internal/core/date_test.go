package core

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	valid := map[string]time.Weekday{
		"2026-11-23": time.Monday,
		"2028-02-29": time.Tuesday, // leap day
		"2000-01-01": time.Saturday,
		"2099-12-31": time.Thursday,
		"1970-01-01": time.Thursday,
		"1969-12-31": time.Wednesday, // before day 0
	}
	for s, wd := range valid {
		d, ok := ParseDate(s)
		if !ok || d.String() != s || d.Weekday() != wd {
			t.Errorf("ParseDate(%q) = %v (%v), %v; want the same date, %v", s, d, d.Weekday(), ok, wd)
		}
	}
	for _, s := range []string{"2026-02-29", "2026-02-30", "2026-13-01", "2026-2-3", "23.11.2026",
		"2026-11-23T00:00:00Z", "", " 2026-11-23", "2026-11-23 ", "26-11-23"} {
		if d, ok := ParseDate(s); ok {
			t.Errorf("ParseDate(%q) = %v; want invalid", s, d)
		}
	}
}

func TestDate_Arithmetic(t *testing.T) {
	if d := DateFromYMD(1970, time.January, 1); d != 0 {
		t.Errorf("1970-01-01 = %d, want 0", d)
	}
	start, _ := ParseDate("2026-12-30")
	if got := (start + 3).String(); got != "2027-01-02" {
		t.Errorf("2026-12-30 + 3 = %s", got)
	}
	a, _ := ParseDate("2028-02-28")
	b, _ := ParseDate("2028-03-01")
	if b-a != 2 {
		t.Errorf("days from 2028-02-28 to 2028-03-01 = %d, want 2", b-a)
	}
	// Weekday and String agree with the time package on every day from 2000
	// to 2099.
	for d := MinRoutineDate; d <= MaxRoutineDate; d++ {
		tm := d.midnightUTC()
		if d.Weekday() != tm.Weekday() || d.String() != tm.Format(time.DateOnly) {
			t.Fatalf("day %d: %v %s, time says %v %s", d, d.Weekday(), d, tm.Weekday(), tm.Format(time.DateOnly))
		}
	}
}

func TestDate_Monday(t *testing.T) {
	cases := map[string]string{
		"2026-11-23": "2026-11-23", // Monday
		"2026-11-25": "2026-11-23",
		"2026-11-29": "2026-11-23", // Sunday belongs to the week before
		"2026-11-30": "2026-11-30",
		"2027-01-01": "2026-12-28", // across the turn of the year
	}
	for in, want := range cases {
		d, _ := ParseDate(in)
		if got := d.Monday().String(); got != want {
			t.Errorf("Monday(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestDateOf(t *testing.T) {
	instant := time.Date(2026, 11, 23, 23, 30, 0, 0, time.UTC)
	vienna := mustLoadLocation("Europe/Vienna")
	newYork := mustLoadLocation("America/New_York")
	if got := DateOf(instant, vienna).String(); got != "2026-11-24" {
		t.Errorf("Vienna: %s, want 2026-11-24", got)
	}
	if got := DateOf(instant, newYork).String(); got != "2026-11-23" {
		t.Errorf("New York: %s, want 2026-11-23", got)
	}
}

func TestParseClockTime(t *testing.T) {
	valid := map[string]ClockTime{"00:00": 0, "08:00": 480, "20:30": 1230, "23:59": 1439}
	for s, want := range valid {
		c, ok := ParseClockTime(s)
		if !ok || c != want || c.String() != s {
			t.Errorf("ParseClockTime(%q) = %d (%s), %v; want %d", s, c, c, ok, want)
		}
	}
	for _, s := range []string{"8:00", "24:00", "23:60", "08:00:00", "0800", "ab:cd", "", " 08:00", "08.00", "-1:00", "08:0"} {
		if c, ok := ParseClockTime(s); ok {
			t.Errorf("ParseClockTime(%q) = %d; want invalid", s, c)
		}
	}
}
