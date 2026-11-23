package core

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"
)

// occurrenceFile is testdata/routine_occurrences.json.
type occurrenceFile struct {
	Cases []struct {
		Name        string           `json:"name"`
		TimeZone    string           `json:"timeZone"`
		Schedule    map[string]any   `json:"schedule"`
		StartDate   string           `json:"startDate"`
		EndDate     *string          `json:"endDate"`
		From        string           `json:"from"`
		To          string           `json:"to"`
		Occurrences []occurrenceJSON `json:"occurrences"`
	} `json:"cases"`
}

type occurrenceJSON struct {
	DueAt string `json:"dueAt"`
	Date  string `json:"date"`
	Time  string `json:"time"`
}

func toJSONForm(list []Occurrence) []occurrenceJSON {
	out := make([]occurrenceJSON, 0, len(list))
	for _, o := range list {
		out = append(out, occurrenceJSON{DueAt: o.DueAt.UTC().Format(time.RFC3339), Date: o.Date.String(), Time: o.Time.String()})
	}
	return out
}

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := CheckTimeZone(name)
	if err != nil {
		t.Fatalf("time zone %q: %v", name, err)
	}
	return loc
}

// testRoutine builds a routine through ParseNewRoutine, as the API would.
func testRoutine(t *testing.T, schedule map[string]any, start string, end *string) Routine {
	t.Helper()
	body := map[string]any{"name": "Testroutine", "schedule": schedule, "startDate": start}
	if end != nil {
		body["endDate"] = *end
	}
	r, err := ParseNewRoutine(body)
	if err != nil {
		t.Fatalf("routine: %v", err)
	}
	r.ID = "6742c0a1b2c3d4e5f6a7b8c9"
	return r
}

func TestOccurrences_Cases(t *testing.T) {
	raw, err := os.ReadFile("testdata/routine_occurrences.json")
	if err != nil {
		t.Fatal(err)
	}
	var file occurrenceFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) < 20 {
		t.Fatalf("only %d cases read", len(file.Cases))
	}
	for _, c := range file.Cases {
		t.Run(c.Name, func(t *testing.T) {
			r := testRoutine(t, c.Schedule, c.StartDate, c.EndDate)
			got := Occurrences(r, date(t, c.From), date(t, c.To), mustZone(t, c.TimeZone))
			if c.Occurrences == nil {
				t.Fatal("case without occurrences")
			}
			if g := toJSONForm(got); !slices.Equal(g, c.Occurrences) {
				t.Errorf("occurrences =\n%v\nwant\n%v", g, c.Occurrences)
			}
			for _, o := range got {
				if o.RoutineID != r.ID || o.DueAt.Location() != time.UTC {
					t.Errorf("occurrence %+v: routine ID or location wrong", o)
				}
			}
		})
	}
}

func TestOccurrence_DueLocal(t *testing.T) {
	o := Occurrence{Date: date(t, "2026-03-29"), Time: clocks(t, "03:30")[0]}
	if got := o.DueLocal(); got != "2026-03-29T03:30" {
		t.Errorf("DueLocal = %q", got)
	}
}

func TestOccurrences_EmptyRange(t *testing.T) {
	r := testRoutine(t, map[string]any{"frequency": "daily", "interval": 1.0, "times": []any{"08:00"}}, "2026-11-23", nil)
	got := Occurrences(r, date(t, "2026-11-25"), date(t, "2026-11-24"), mustZone(t, "Europe/Vienna"))
	if got == nil || len(got) != 0 {
		t.Errorf("to before from: %v, want an empty list", got)
	}
}

func TestWallClockInstant(t *testing.T) {
	cases := []struct {
		zone, day, clock, want string
	}{
		{"Europe/Vienna", "2026-11-23", "08:00", "2026-11-23T07:00:00Z"},
		{"Europe/Vienna", "2026-07-01", "08:00", "2026-07-01T06:00:00Z"},
		{"Europe/Vienna", "2026-03-29", "01:59", "2026-03-29T00:59:00Z"}, // just before the gap
		{"Europe/Vienna", "2026-03-29", "02:00", "2026-03-29T01:00:00Z"}, // gap: 03:00
		{"Europe/Vienna", "2026-03-29", "02:59", "2026-03-29T01:59:00Z"}, // gap: 03:59
		{"Europe/Vienna", "2026-03-29", "03:00", "2026-03-29T01:00:00Z"},
		{"Europe/Vienna", "2026-10-25", "01:59", "2026-10-24T23:59:00Z"},
		{"Europe/Vienna", "2026-10-25", "02:00", "2026-10-25T00:00:00Z"}, // overlap: the earlier
		{"Europe/Vienna", "2026-10-25", "02:59", "2026-10-25T00:59:00Z"}, // overlap: the earlier
		{"Europe/Vienna", "2026-10-25", "03:00", "2026-10-25T02:00:00Z"},
		{"America/New_York", "2026-03-08", "02:30", "2026-03-08T07:30:00Z"},
		{"America/New_York", "2026-11-01", "01:30", "2026-11-01T05:30:00Z"},
		{"Australia/Lord_Howe", "2026-10-04", "02:15", "2026-10-03T15:45:00Z"},
		{"Asia/Kolkata", "2026-11-23", "08:00", "2026-11-23T02:30:00Z"}, // +05:30, no daylight saving time
		{"UTC", "2026-03-29", "02:30", "2026-03-29T02:30:00Z"},
	}
	for _, c := range cases {
		got := wallClockInstant(date(t, c.day), clocks(t, c.clock)[0], mustZone(t, c.zone))
		if got.Format(time.RFC3339) != c.want {
			t.Errorf("%s %s %s = %s, want %s", c.zone, c.day, c.clock, got.Format(time.RFC3339), c.want)
		}
	}
}

func TestTransitionDays(t *testing.T) {
	vienna := transitionDays(mustZone(t, "Europe/Vienna"), date(t, "2026-02-01"), date(t, "2026-11-30"))
	for _, d := range []string{"2026-03-29", "2026-10-25"} {
		if !vienna[date(t, d)] {
			t.Errorf("Vienna: %s missing in %v", d, vienna)
		}
	}
	for _, d := range []string{"2026-03-28", "2026-03-30", "2026-07-01", "2026-10-24", "2026-10-26"} {
		if vienna[date(t, d)] {
			t.Errorf("Vienna: %s marked", d)
		}
	}
	if days := transitionDays(time.UTC, date(t, "2026-01-01"), date(t, "2026-12-31")); len(days) != 0 {
		t.Errorf("UTC: %v", days)
	}
	// Samoa skipped 30 December 2011: the day before, the skipped day and
	// the day after are marked.
	apia := transitionDays(mustZone(t, "Pacific/Apia"), date(t, "2011-12-20"), date(t, "2011-12-31"))
	for _, d := range []string{"2011-12-29", "2011-12-30", "2011-12-31"} {
		if !apia[date(t, d)] {
			t.Errorf("Apia: %s missing in %v", d, apia)
		}
	}
	// Only days inside the range are returned.
	if days := transitionDays(mustZone(t, "Europe/Vienna"), date(t, "2026-03-30"), date(t, "2026-10-24")); len(days) != 0 {
		t.Errorf("range between the changes: %v", days)
	}
}

// referenceOn computes the occurrences of a local date always by the full
// rules (wallClockInstant for the day and the day before), without the
// shortcut for days with one offset.
func referenceOn(x *expander, d Date) []Occurrence {
	var out []Occurrence
	for _, p := range []Date{d - 1, d} {
		if !x.planned(p) {
			continue
		}
		for _, c := range x.wallTimes {
			due := wallClockInstant(p, c, x.loc)
			if DateOf(due, x.loc) != d {
				continue
			}
			local := due.In(x.loc)
			out = append(out, Occurrence{RoutineID: x.routineID, DueAt: due, Date: d, Time: ClockTime(local.Hour()*60 + local.Minute())})
		}
	}
	slices.SortFunc(out, func(a, b Occurrence) int { return a.DueAt.Compare(b.DueAt) })
	return slices.CompactFunc(out, func(a, b Occurrence) bool { return a.DueAt.Equal(b.DueAt) })
}

// The shortcut for days with one offset must give exactly what the full
// rules give, on every day of whole years in zones with different kinds of
// changes: at 02:00 (Vienna, New York), by 30 minutes (Lord Howe), at
// midnight (Santiago), a skipped day (Apia 2011), none (Kolkata), the end
// of daylight saving time (Tehran 2022) and a year far ahead (Vienna 2090).
func TestOccurrences_ShortcutMatchesFullRules(t *testing.T) {
	zones := []struct {
		zone string
		year int
	}{
		{"Europe/Vienna", 2026}, {"America/New_York", 2026}, {"Australia/Lord_Howe", 2026},
		{"America/Santiago", 2026}, {"Pacific/Apia", 2011}, {"Asia/Kolkata", 2026},
		{"Asia/Tehran", 2022}, {"Europe/Vienna", 2090}, {"Europe/Lisbon", 2026},
	}
	schedules := []map[string]any{
		{"frequency": "daily", "interval": 1.0, "times": []any{"00:00", "00:30", "01:00", "01:30", "01:45", "02:00", "02:15", "02:30", "03:00", "03:30", "12:00", "23:00", "23:30", "23:59"}},
		{"frequency": "hourly", "interval": 1.0, "windowStart": "00:00", "windowEnd": "23:00"},
	}
	for _, z := range zones {
		loc := mustZone(t, z.zone)
		from := DateFromYMD(z.year, time.January, 1)
		to := DateFromYMD(z.year, time.December, 31)
		for _, s := range schedules {
			r := testRoutine(t, s, (from - 3).String(), nil)
			x := newExpander(r, loc, from, to)
			for d := from; d <= to; d++ {
				got, want := x.on(d), referenceOn(x, d)
				if !slices.Equal(toJSONForm(got), toJSONForm(want)) {
					t.Fatalf("%s %s %s:\nshortcut %v\nfull     %v", z.zone, s["frequency"], d, toJSONForm(got), toJSONForm(want))
				}
			}
		}
	}
}

// An independent check of wallClockInstant for times that exist: scanning
// the points in time around it in steps of 15 minutes (every offset since
// 1980 is a multiple of 15 minutes), the earliest one that shows the time
// must be the result. For times in a gap, the result must show a later
// time on the same day.
func TestWallClockInstant_Scan(t *testing.T) {
	for _, z := range []string{"Europe/Vienna", "America/New_York", "Australia/Lord_Howe", "America/Santiago"} {
		loc := mustZone(t, z)
		days := transitionDays(loc, DateFromYMD(2026, 1, 1), DateFromYMD(2026, 12, 31))
		for d := range days {
			for c := ClockTime(0); c < 24*60; c += 15 {
				got := wallClockInstant(d, c, loc)
				y, m, day := d.YMD()
				wall := time.Date(y, m, day, int(c)/60, int(c)%60, 0, 0, time.UTC)
				var earliest time.Time
				for step := -16 * 4; step <= 16*4; step++ {
					cand := wall.Add(time.Duration(step) * 15 * time.Minute)
					if showsWallClock(cand, loc, wall) {
						earliest = cand
						break
					}
				}
				if !earliest.IsZero() {
					if !got.Equal(earliest) {
						t.Errorf("%s %s %s: %s, earliest is %s", z, d, c, got.Format(time.RFC3339), earliest.Format(time.RFC3339))
					}
					continue
				}
				local := got.In(loc)
				if DateOf(got, loc) != d || ClockTime(local.Hour()*60+local.Minute()) <= c {
					t.Errorf("%s %s %s is in a gap, but gives %s", z, d, c, local)
				}
			}
		}
	}
}
