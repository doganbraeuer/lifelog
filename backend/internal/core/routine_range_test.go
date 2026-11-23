package core

import (
	"errors"
	"testing"
	"time"
)

func TestOccurrenceRange(t *testing.T) {
	today := date(t, "2026-11-23")
	cases := []struct {
		from, to, wantFrom, wantTo string
	}{
		{"", "", "2026-11-23", "2026-11-29"},                     // today and 6 days
		{"2026-12-01", "", "2026-12-01", "2026-12-07"},           // from + 6
		{"", "2026-11-23", "2026-11-23", "2026-11-23"},           // one day
		{"2026-11-01", "2026-12-01", "2026-11-01", "2026-12-01"}, // 31 days, both ends
	}
	for _, c := range cases {
		f, to, err := OccurrenceRange(c.from, c.to, today)
		if err != nil || f.String() != c.wantFrom || to.String() != c.wantTo {
			t.Errorf("OccurrenceRange(%q, %q) = %s, %s, %v; want %s, %s", c.from, c.to, f, to, err, c.wantFrom, c.wantTo)
		}
	}
	errorCases := []struct {
		from, to, param, value, message string
	}{
		{"23.11.2026", "", "from", "23.11.2026", msgDateFormat},
		{"", "2026-02-30", "to", "2026-02-30", msgDateFormat},
		{"2026-11-25", "2026-11-24", "to", "2026-11-24", msgRangeReversed},
		{"", "2026-11-22", "to", "2026-11-22", msgRangeReversed}, // before today, the default from
		{"2026-11-01", "2026-12-02", "to", "2026-12-02", "Der Zeitraum darf höchstens 31 Tage umfassen."},
	}
	for _, c := range errorCases {
		_, _, err := OccurrenceRange(c.from, c.to, today)
		checkParameterError(t, err, c.param, c.value, c.message)
	}
}

func TestLogRange(t *testing.T) {
	today := date(t, "2026-11-23")
	cases := []struct {
		from, to, wantFrom, wantTo string
	}{
		{"", "", "2026-10-25", "2026-11-23"}, // 30 days up to today (API.md 3.25)
		{"", "2026-01-30", "2026-01-01", "2026-01-30"},
		{"2026-11-01", "", "2026-11-01", "2026-11-23"},
		{"2025-11-23", "2026-11-23", "2025-11-23", "2026-11-23"}, // 366 days
	}
	for _, c := range cases {
		f, to, err := LogRange(c.from, c.to, today)
		if err != nil || f.String() != c.wantFrom || to.String() != c.wantTo {
			t.Errorf("LogRange(%q, %q) = %s, %s, %v; want %s, %s", c.from, c.to, f, to, err, c.wantFrom, c.wantTo)
		}
	}
	errorCases := []struct {
		from, to, param, value, message string
	}{
		{"x", "", "from", "x", msgDateFormat},
		{"2026-11-24", "", "from", "2026-11-24", msgRangeReversed}, // after today, the default to
		{"2025-11-22", "2026-11-23", "to", "2026-11-23", "Der Zeitraum darf höchstens 366 Tage umfassen."},
	}
	for _, c := range errorCases {
		_, _, err := LogRange(c.from, c.to, today)
		checkParameterError(t, err, c.param, c.value, c.message)
	}
}

func TestCheckRoutineModule(t *testing.T) {
	if m, err := CheckRoutineModule(""); err != nil || m != "health" {
		t.Errorf("empty: %q, %v", m, err)
	}
	_, err := CheckRoutineModule("sport")
	checkParameterError(t, err, "module", "sport", msgModule)
}

func checkParameterError(t *testing.T, err error, param, value, message string) {
	t.Helper()
	var pe *ParameterError
	if !errors.As(err, &pe) || !errors.Is(err, ErrInvalidParameter) {
		t.Fatalf("err = %v, want a *ParameterError", err)
	}
	if pe.Parameter != param || pe.Value != value || pe.Message != message {
		t.Errorf("error = %+v, want %s / %s / %s", pe, param, value, message)
	}
}

func TestDayStart(t *testing.T) {
	cases := []struct {
		zone, day, want string
	}{
		{"Europe/Vienna", "2026-11-23", "2026-11-22T23:00:00Z"},
		{"Europe/Vienna", "2026-03-29", "2026-03-28T23:00:00Z"},    // the gap is at 02:00, midnight exists
		{"America/Santiago", "2026-09-06", "2026-09-06T04:00:00Z"}, // no midnight: the day starts at 01:00
		{"America/Santiago", "2026-04-05", "2026-04-05T04:00:00Z"}, // 23:00 of the 4th twice, then midnight
		{"Pacific/Apia", "2011-12-30", "2011-12-30T10:00:00Z"},     // skipped: same as the 31st
		{"Pacific/Apia", "2011-12-31", "2011-12-30T10:00:00Z"},
	}
	for _, c := range cases {
		if got := DayStart(date(t, c.day), mustZone(t, c.zone)).Format(time.RFC3339); got != c.want {
			t.Errorf("DayStart(%s, %s) = %s, want %s", c.day, c.zone, got, c.want)
		}
	}
}
