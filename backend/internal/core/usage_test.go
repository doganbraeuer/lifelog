package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUsageDay_ViennaMidnight(t *testing.T) {
	cases := []struct {
		utc  string
		want string
	}{
		// November: Vienna is UTC+1.
		{"2026-11-18T22:59:59Z", "2026-11-18"},
		{"2026-11-18T23:00:00Z", "2026-11-19"},
		{"2026-11-18T00:30:00Z", "2026-11-18"},
		{"2026-11-17T23:30:00Z", "2026-11-18"},
		// July: summer time, UTC+2.
		{"2026-07-10T21:59:59Z", "2026-07-10"},
		{"2026-07-10T22:00:00Z", "2026-07-11"},
	}
	for _, c := range cases {
		ts, err := time.Parse(time.RFC3339, c.utc)
		if err != nil {
			t.Fatal(err)
		}
		if got := UsageDay(ts); got != c.want {
			t.Errorf("UsageDay(%s) = %s, want %s", c.utc, got, c.want)
		}
	}
}

func TestUsageDayEnd(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-11-18T12:00:00Z")
	want, _ := time.Parse(time.RFC3339, "2026-11-18T23:00:00Z")
	if got := UsageDayEnd(ts); !got.Equal(want) {
		t.Errorf("UsageDayEnd = %v, want %v (midnight in Vienna)", got, want)
	}
	// The last second of the day still ends at the same midnight.
	late, _ := time.Parse(time.RFC3339, "2026-11-18T22:59:59Z")
	if got := UsageDayEnd(late); !got.Equal(want) {
		t.Errorf("UsageDayEnd(late) = %v, want %v", got, want)
	}
}

func TestLimitReachedError(t *testing.T) {
	var err error = &LimitReachedError{Limit: LimitQuestions}
	var le *LimitReachedError
	if !errors.As(err, &le) || le.Limit != "questions" {
		t.Errorf("err = %v", err)
	}
	if LimitUploads != "uploads" || LimitTranscriptions != "transcriptions" {
		t.Error("limit names are part of the contract (details.limit)")
	}
}

func TestNormalizeConsentVersion(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"2026-11-18", "2026-11-18", true},
		{"  v1  ", "v1", true},
		{strings.Repeat("x", 100), strings.Repeat("x", 100), true},
		{strings.Repeat("ä", 100), strings.Repeat("ä", 100), true},
		{strings.Repeat("x", 101), "", false},
		{"", "", false},
		{"   ", "", false},
		{"v1\nv2", "", false},
		{"v1\x00", "", false},
		{"v\xff", "", false},
	}
	for _, c := range cases {
		got, err := NormalizeConsentVersion(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("NormalizeConsentVersion(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
		if !c.ok && !errors.Is(err, ErrInvalidConsentVersion) {
			t.Errorf("NormalizeConsentVersion(%q) = %q, %v; want ErrInvalidConsentVersion", c.in, got, err)
		}
	}
}
