package core

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func instant(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad test time %q", s)
	}
	return v
}

// routineFrom builds a routine through ParseNewRoutine from a JSON body.
func routineFrom(t *testing.T, body string) Routine {
	t.Helper()
	r, err := ParseNewRoutine(jsonBody(t, body))
	if err != nil {
		t.Fatalf("routine: %v", err)
	}
	r.ID = "6742c0a1b2c3d4e5f6a7b8c9"
	return r
}

var logCounter int

// occLog is a log for the occurrence at dueAt, created in the time zone
// loc, taken two minutes later.
func occLog(t *testing.T, loc *time.Location, dueAt string, status LogStatus, amount *float64) RoutineLog {
	t.Helper()
	due := instant(t, dueAt)
	logCounter++
	return RoutineLog{
		ID: fmt.Sprintf("log%03d", logCounter), RoutineID: "6742c0a1b2c3d4e5f6a7b8c9",
		DueAt: &due, DueLocal: due.In(loc).Format("2006-01-02T15:04"),
		TakenAt: due.Add(2 * time.Minute), Status: status, Amount: amount,
		Source: LogSourceManual, TimeZone: loc.String(),
	}
}

// freeLog is a done log without an occurrence (an extra glass, a portion).
func freeLog(t *testing.T, takenAt string, amount *float64) RoutineLog {
	t.Helper()
	logCounter++
	return RoutineLog{ID: fmt.Sprintf("log%03d", logCounter), RoutineID: "6742c0a1b2c3d4e5f6a7b8c9",
		TakenAt: instant(t, takenAt), Status: LogDone, Amount: amount, Source: LogSourceManual}
}

// rangeWant is a ProgressRange in short form; amounts < 0 mean nil.
func rangeWant(done, skipped, due int, amountDone, amountTarget float64) ProgressRange {
	p := ProgressRange{Done: done, Skipped: skipped, Due: due}
	if amountDone >= 0 {
		p.AmountDone = &amountDone
	}
	if amountTarget >= 0 {
		p.AmountTarget = &amountTarget
	}
	return p
}

const none = -1.0 // amount nil in rangeWant

func checkRange(t *testing.T, name string, got, want ProgressRange) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %s, want %s", name, formatRange(got), formatRange(want))
	}
}

func formatRange(p ProgressRange) string {
	f := func(v *float64) string {
		if v == nil {
			return "null"
		}
		return fmt.Sprint(*v)
	}
	return fmt.Sprintf("{done %d, skipped %d, due %d, amount %s of %s, day %d of %d}",
		p.Done, p.Skipped, p.Due, f(p.AmountDone), f(p.AmountTarget), p.Day, p.Days)
}

// Example of API.md 3.16: „Blutdruck messen“, daily 08:00 and 20:00 for 14
// days, on day 6: „Tag 6 von 14“.
func TestRoutineProgress_BloodPressure(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Blutdruck messen", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]},
		"startDate": "2026-11-18", "endDate": "2026-12-01"}`)
	var logs []RoutineLog
	for _, due := range []string{"2026-11-18T07:00:00Z", "2026-11-18T19:00:00Z", "2026-11-19T07:00:00Z",
		"2026-11-20T07:00:00Z", "2026-11-20T19:00:00Z", "2026-11-21T07:00:00Z", "2026-11-22T07:00:00Z",
		"2026-11-22T19:00:00Z", "2026-11-23T07:00:00Z"} {
		logs = append(logs, occLog(t, vienna, due, LogDone, nil))
	}
	logs = append(logs, occLog(t, vienna, "2026-11-19T19:00:00Z", LogSkipped, nil))

	p := RoutineProgress(r, logs, vienna, instant(t, "2026-11-23T20:30:00Z"))
	checkRange(t, "today", p.Today, rangeWant(1, 0, 2, none, none))
	checkRange(t, "week", p.Week, rangeWant(1, 0, 14, none, none))
	if p.Period == nil {
		t.Fatal("period is nil")
	}
	want := rangeWant(9, 1, 28, none, none)
	want.Day, want.Days = 6, 14
	checkRange(t, "period", *p.Period, want)
}

// Example of API.md 3.16: „Wasser“, 250 ml, hourly 08:00 to 20:00, on a
// Wednesday evening; two extra glasses on Tuesday without an occurrence.
func TestRoutineProgress_Water(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Wasser", "amount": 250, "unit": "ml",
		"schedule": {"frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00"}, "startDate": "2026-11-16"}`)
	var logs []RoutineLog
	for h := 7; h <= 18; h++ { // Monday 08:00 to 19:00
		logs = append(logs, occLog(t, vienna, fmt.Sprintf("2026-11-23T%02d:00:00Z", h), LogDone, nil))
	}
	for h := 7; h <= 17; h++ { // Tuesday 08:00 to 18:00
		logs = append(logs, occLog(t, vienna, fmt.Sprintf("2026-11-24T%02d:00:00Z", h), LogDone, nil))
	}
	logs = append(logs, occLog(t, vienna, "2026-11-24T18:00:00Z", LogSkipped, nil))
	logs = append(logs, freeLog(t, "2026-11-24T11:30:00Z", nil), freeLog(t, "2026-11-24T14:30:00Z", nil))
	for h := 7; h <= 11; h++ { // Wednesday 08:00 to 12:00
		logs = append(logs, occLog(t, vienna, fmt.Sprintf("2026-11-25T%02d:00:00Z", h), LogDone, nil))
	}

	p := RoutineProgress(r, logs, vienna, instant(t, "2026-11-25T19:30:00Z"))
	checkRange(t, "today", p.Today, rangeWant(5, 0, 13, 1250, 3250))
	checkRange(t, "week", p.Week, rangeWant(28, 1, 91, 7500, 22750))
	if p.Period != nil {
		t.Errorf("period = %+v, want nil without endDate", *p.Period)
	}
}

// Example of API.md 3.16: „Eiweiß“, 30 g, daily at 20:00, logged in
// portions.
func TestRoutineProgress_Protein(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Eiweiß", "amount": 30, "unit": "g",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["20:00"]}, "startDate": "2026-11-20"}`)
	logs := []RoutineLog{freeLog(t, "2026-11-23T11:00:00Z", float(10)), freeLog(t, "2026-11-23T14:00:00Z", float(15))}

	p := RoutineProgress(r, logs, vienna, instant(t, "2026-11-23T16:00:00Z"))
	checkRange(t, "today with two portions", p.Today, rangeWant(0, 0, 1, 25, 30))

	logs = append(logs, occLog(t, vienna, "2026-11-23T19:00:00Z", LogDone, float(5)))
	p = RoutineProgress(r, logs, vienna, instant(t, "2026-11-23T19:05:00Z"))
	checkRange(t, "today with the last portion", p.Today, rangeWant(1, 0, 1, 30, 30))
}

// Example of API.md 3.17: „Vitamin D“, 2 Stück, daily at 08:00.
func TestRoutineProgress_VitaminD(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Vitamin D", "amount": 2, "unit": "Stück",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-18"}`)
	logs := []RoutineLog{occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, nil)}
	p := RoutineProgress(r, logs, vienna, instant(t, "2026-11-23T08:00:00Z"))
	checkRange(t, "today", p.Today, rangeWant(1, 0, 1, 2, 2))
	checkRange(t, "week", p.Week, rangeWant(1, 0, 7, 2, 14))
	if p.Period != nil {
		t.Error("period without endDate")
	}
}

// Example of API.md 3.18: „Wasser“ created on Monday 2026-11-23 at 09:15,
// no logs yet; today counts the earlier occurrences of the day too.
func TestRoutineProgress_NewRoutine(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Wasser", "amount": 250, "unit": "ml",
		"schedule": {"frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00"}, "startDate": "2026-11-23"}`)
	p := RoutineProgress(r, nil, vienna, instant(t, "2026-11-23T08:15:03Z"))
	checkRange(t, "today", p.Today, rangeWant(0, 0, 13, 0, 3250))
	checkRange(t, "week", p.Week, rangeWant(0, 0, 91, 0, 22750))
}

func TestRoutineProgress_PeriodBeforeAndAfter(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	now := instant(t, "2026-11-23T12:00:00Z")

	upcoming := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]},
		"startDate": "2026-11-25", "endDate": "2026-11-30"}`)
	p := RoutineProgress(upcoming, nil, vienna, now)
	checkRange(t, "today before the start", p.Today, rangeWant(0, 0, 0, none, none))
	checkRange(t, "week before the start", p.Week, rangeWant(0, 0, 5, none, none))
	want := rangeWant(0, 0, 6, none, none)
	want.Day, want.Days = 0, 6
	checkRange(t, "period before the start", *p.Period, want)

	ended := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]},
		"startDate": "2026-11-01", "endDate": "2026-11-10"}`)
	p = RoutineProgress(ended, nil, vienna, now)
	checkRange(t, "week after the end", p.Week, rangeWant(0, 0, 0, none, none))
	want = rangeWant(0, 0, 10, none, none)
	want.Day, want.Days = 10, 10
	checkRange(t, "period after the end", *p.Period, want)
}

// Paused days are not due, unless an occurrence has a log.
func TestRoutineProgress_Pause(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]},
		"startDate": "2026-11-16", "endDate": "2026-11-29"}`)
	r.Pauses = []Pause{{From: date(t, "2026-11-17"), Until: ptrDate(date(t, "2026-11-18"))}, {From: date(t, "2026-11-23")}}
	logs := []RoutineLog{
		occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, nil),    // on a paused day: counts
		occLog(t, vienna, "2026-11-17T07:00:00Z", LogSkipped, nil), // on a paused day: counts
		occLog(t, vienna, "2026-11-16T07:00:00Z", LogDone, nil),
	}
	p := RoutineProgress(r, logs, vienna, instant(t, "2026-11-23T20:00:00Z"))
	checkRange(t, "today", p.Today, rangeWant(1, 0, 1, none, none))
	checkRange(t, "week", p.Week, rangeWant(1, 0, 1, none, none))
	// 14 days × 2 = 28; paused: 17, 18 and 23–29 = 9 days = 18 occurrences;
	// plus the two logged on paused days.
	want := rangeWant(2, 1, 28-18+2, none, none)
	want.Day, want.Days = 8, 14
	checkRange(t, "period", *p.Period, want)

	occ := RoutineOccurrences(r, logs, date(t, "2026-11-23"), date(t, "2026-11-23"), vienna)
	if len(occ) != 2 || occ[0].Status != OccurrenceDone || occ[0].LogID != logs[0].ID ||
		occ[1].Status != OccurrencePaused || occ[1].LogID != "" {
		t.Errorf("occurrences on the paused day: %+v", occ)
	}
}

func ptrDate(d Date) *Date { return &d }

// After a change of the schedule, a log whose occurrence no longer exists
// counts only for amountDone, on the day of its takenAt (API.md 3.16
// "Logs", rule 3).
func TestRoutineProgress_OccurrenceGone(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Vitamin D", "amount": 2, "unit": "Stück",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["07:30"]}, "startDate": "2026-11-16"}`)
	gone := occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, float(2)) // was 08:00
	late := occLog(t, vienna, "2026-11-22T07:00:00Z", LogDone, float(1)) // 08:00 on Sunday, logged on Monday
	late.TakenAt = instant(t, "2026-11-23T09:00:00Z")
	skipped := occLog(t, vienna, "2026-11-23T08:00:00Z", LogSkipped, nil) // 09:00, gone too

	p := RoutineProgress(r, []RoutineLog{gone, late, skipped}, vienna, instant(t, "2026-11-23T12:00:00Z"))
	checkRange(t, "today", p.Today, rangeWant(0, 0, 1, 3, 2))

	occ := RoutineOccurrences(r, []RoutineLog{gone, late, skipped}, date(t, "2026-11-23"), date(t, "2026-11-23"), vienna)
	if len(occ) != 1 || occ[0].Status != OccurrenceOpen || occ[0].Time.String() != "07:30" {
		t.Errorf("occurrences: %+v", occ)
	}
}

// After a change of the time zone a log stays with its occurrence: same
// local date and time, a new dueAt (API.md 3.16 "Logs", rule 2).
func TestRoutineProgress_TimeZoneChange(t *testing.T) {
	vienna, lisbon := mustZone(t, "Europe/Vienna"), mustZone(t, "Europe/Lisbon")
	r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-16"}`)
	logged := occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, nil) // 08:00 in Vienna

	p := RoutineProgress(r, []RoutineLog{logged}, lisbon, instant(t, "2026-11-23T12:00:00Z"))
	checkRange(t, "today in Lisbon", p.Today, rangeWant(1, 0, 1, none, none))

	occ := RoutineOccurrences(r, []RoutineLog{logged}, date(t, "2026-11-23"), date(t, "2026-11-23"), lisbon)
	if len(occ) != 1 || occ[0].DueAt.Format(time.RFC3339) != "2026-11-23T08:00:00Z" ||
		occ[0].Status != OccurrenceDone || occ[0].LogID != logged.ID {
		t.Errorf("occurrence in Lisbon: %+v", occ)
	}
}

func TestRoutineProgress_Amounts(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	now := instant(t, "2026-11-23T20:00:00Z")

	// No amount on the routine: sums are null, also with amounts on logs.
	r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-23"}`)
	logs := []RoutineLog{occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, float(3))}
	checkRange(t, "without amount", RoutineProgress(r, logs, vienna, now).Today, rangeWant(1, 0, 1, none, none))

	// 3 × 0.1 is 0.3, not 0.30000000000000004.
	r = routineFrom(t, `{"name": "x", "amount": 0.1, "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "12:00", "16:00"]}, "startDate": "2026-11-23"}`)
	logs = []RoutineLog{
		occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, nil),
		occLog(t, vienna, "2026-11-23T11:00:00Z", LogDone, nil),
		occLog(t, vienna, "2026-11-23T15:00:00Z", LogDone, nil),
	}
	checkRange(t, "rounded", RoutineProgress(r, logs, vienna, now).Today, rangeWant(3, 0, 3, 0.3, 0.3))

	// An amount on the log counts instead of the routine's; skipped logs add
	// nothing.
	logs = []RoutineLog{
		occLog(t, vienna, "2026-11-23T07:00:00Z", LogDone, float(0.5)),
		occLog(t, vienna, "2026-11-23T11:00:00Z", LogSkipped, nil),
	}
	checkRange(t, "own amount, skipped", RoutineProgress(r, logs, vienna, now).Today, rangeWant(1, 1, 3, 0.5, 0.3))
}

// Days with a change of the clocks have 23 or 25 hours, but never more
// occurrences than wall-clock times.
func TestRoutineProgress_DaylightSavingTime(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "hourly", "interval": 1, "windowStart": "00:00", "windowEnd": "23:00"},
		"startDate": "2026-03-16", "endDate": "2026-11-01"}`)
	p := RoutineProgress(r, nil, vienna, instant(t, "2026-03-29T10:00:00Z"))
	checkRange(t, "today, clocks forward", p.Today, rangeWant(0, 0, 23, none, none))
	checkRange(t, "week, clocks forward", p.Week, rangeWant(0, 0, 6*24+23, none, none))

	p = RoutineProgress(r, nil, vienna, instant(t, "2026-10-25T10:00:00Z"))
	checkRange(t, "today, clocks back", p.Today, rangeWant(0, 0, 24, none, none))
	checkRange(t, "week, clocks back", p.Week, rangeWant(0, 0, 7*24, none, none))
	// The period from 16 March to 1 November: 231 days × 24, minus one hour
	// in March.
	if p.Period.Due != 231*24-1 || p.Period.Days != 231 {
		t.Errorf("period: %s", formatRange(*p.Period))
	}
}

// The progress over a long period counts the days by the shortcut; it must
// give the same as counting the occurrences one by one.
func TestRoutineProgress_CountsLikeOccurrences(t *testing.T) {
	for _, zone := range []string{"Europe/Vienna", "America/Santiago", "Australia/Lord_Howe"} {
		loc := mustZone(t, zone)
		r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "weekly", "interval": 2, "weekdays": ["sat", "sun"], "times": ["00:00", "00:30", "02:30", "23:30"]},
			"startDate": "2025-01-01", "endDate": "2028-12-31"}`)
		r.Pauses = []Pause{{From: date(t, "2026-06-01"), Until: ptrDate(date(t, "2026-08-31"))}}
		p := RoutineProgress(r, nil, loc, instant(t, "2026-11-23T12:00:00Z"))
		want := 0
		for _, o := range Occurrences(r, r.StartDate, *r.EndDate, loc) {
			if !pausedOn(r.Pauses, o.Date) {
				want++
			}
		}
		if p.Period.Due != want {
			t.Errorf("%s: due %d, counted one by one %d", zone, p.Period.Due, want)
		}
	}
}

func TestRoutineStatusOn(t *testing.T) {
	base := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]},
		"startDate": "2026-11-18", "endDate": "2026-12-01"}`)
	pausedToday := []Pause{{From: date(t, "2026-11-20")}}
	cases := []struct {
		name   string
		today  string
		pauses []Pause
		want   RoutineStatus
	}{
		{"active", "2026-11-23", nil, RoutineActive},
		{"first day", "2026-11-18", nil, RoutineActive},
		{"last day", "2026-12-01", nil, RoutineActive},
		{"upcoming", "2026-11-17", nil, RoutineUpcoming},
		{"ended", "2026-12-02", nil, RoutineEnded},
		{"paused", "2026-11-23", pausedToday, RoutinePaused},
		{"pause over", "2026-11-23", []Pause{{From: date(t, "2026-11-19"), Until: ptrDate(date(t, "2026-11-22"))}}, RoutineActive},
		{"ended wins over paused", "2026-12-02", pausedToday, RoutineEnded},
		{"upcoming wins over paused", "2026-11-17", []Pause{{From: date(t, "2026-11-10")}}, RoutineUpcoming},
	}
	for _, c := range cases {
		r := base
		r.Pauses = c.pauses
		if got := RoutineStatusOn(r, date(t, c.today)); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	noEnd := base
	noEnd.EndDate = nil
	if got := RoutineStatusOn(noEnd, date(t, "2099-12-31")); got != RoutineActive {
		t.Errorf("without endDate: %s", got)
	}
}

func TestParsePauseRequest(t *testing.T) {
	if until, err := ParsePauseRequest(map[string]any{}); err != nil || until != nil {
		t.Errorf("{}: %v, %v", until, err)
	}
	if until, err := ParsePauseRequest(jsonBody(t, `{"until": null}`)); err != nil || until != nil {
		t.Errorf("null: %v, %v", until, err)
	}
	if until, err := ParsePauseRequest(jsonBody(t, `{"until": "2026-11-30"}`)); err != nil || until == nil || until.String() != "2026-11-30" {
		t.Errorf("date: %v, %v", until, err)
	}
	cases := []struct {
		body, field, message string
		value                any
	}{
		{`{"until": "30.11.2026"}`, "until", msgDateFormat, "30.11.2026"},
		{`{"until": 5}`, "until", msgDateFormat, float64(5)},
		{`{"until": "2100-01-01"}`, "until", msgDateRange, "2100-01-01"},
		{`{"from": "2026-11-23"}`, "from", "Unbekanntes Feld: from.", nil},
	}
	for _, c := range cases {
		_, err := ParsePauseRequest(jsonBody(t, c.body))
		checkRoutineError(t, err, c.field, c.message, c.value)
	}
	_, err := ParsePauseRequest(nil)
	checkRoutineError(t, err, "", msgPauseBody, nil)
}

func TestApplyPause(t *testing.T) {
	today := date(t, "2026-11-23")
	until := ptrDate(date(t, "2026-11-30"))
	past := []Pause{{From: date(t, "2026-11-02"), Until: ptrDate(date(t, "2026-11-08"))}}

	got, changed, err := ApplyPause(past, today, until)
	want := []Pause{past[0], {From: today, Until: until}}
	if err != nil || !changed || !reflect.DeepEqual(got, want) {
		t.Errorf("append: %v %v %v", got, changed, err)
	}
	// Sending it again changes nothing.
	again, changed, err := ApplyPause(got, today, until)
	if err != nil || changed || !reflect.DeepEqual(again, want) {
		t.Errorf("again: %v %v %v", again, changed, err)
	}
	// A running pause only gets a new until; from stays.
	open, changed, err := ApplyPause(got, today, nil)
	if err != nil || !changed || open[1].From != today || open[1].Until != nil || len(open) != 2 {
		t.Errorf("until removed: %v %v %v", open, changed, err)
	}
	shorter, _, err := ApplyPause([]Pause{{From: date(t, "2026-11-20"), Until: until}}, today, ptrDate(today))
	if err != nil || len(shorter) != 1 || shorter[0].From != date(t, "2026-11-20") || *shorter[0].Until != today {
		t.Errorf("shorter: %v %v", shorter, err)
	}
	// Overlapping pauses that include today all get the new until.
	overlap := []Pause{{From: date(t, "2026-11-20")}, {From: date(t, "2026-11-22"), Until: ptrDate(date(t, "2026-11-25"))}}
	both, _, err := ApplyPause(overlap, today, until)
	if err != nil || *both[0].Until != *until || *both[1].Until != *until {
		t.Errorf("overlap: %v %v", both, err)
	}
	if overlap[0].Until != nil || *overlap[1].Until != date(t, "2026-11-25") {
		t.Error("the pauses passed in were changed")
	}

	// until before today.
	_, _, err = ApplyPause(nil, today, ptrDate(today-1))
	checkRoutineError(t, err, "until", msgPauseUntil, "2026-11-22")

	// 100 pauses: no new one, but a running one can still change.
	full := make([]Pause, MaxPausesPerRoutine)
	for i := range full {
		d := date(t, "2026-01-01") + Date(2*i)
		full[i] = Pause{From: d, Until: ptrDate(d)}
	}
	_, _, err = ApplyPause(full, today, nil)
	var limit *RoutineLimitError
	if !errors.As(err, &limit) || limit.Limit != RoutineLimitPauses || limit.Max != 100 {
		t.Errorf("101st pause: %v", err)
	}
	full[99] = Pause{From: date(t, "2026-11-20")}
	if _, _, err := ApplyPause(full, today, until); err != nil {
		t.Errorf("change with 100 pauses: %v", err)
	}
}

func TestApplyResume(t *testing.T) {
	today := date(t, "2026-11-23")
	past := Pause{From: date(t, "2026-11-02"), Until: ptrDate(date(t, "2026-11-08"))}

	got, changed := ApplyResume([]Pause{past, {From: today}}, today)
	if !changed || !reflect.DeepEqual(got, []Pause{past}) {
		t.Errorf("begun today: %v %v", got, changed)
	}
	running := []Pause{past, {From: date(t, "2026-11-20"), Until: ptrDate(date(t, "2026-11-30"))}}
	got, changed = ApplyResume(running, today)
	if !changed || len(got) != 2 || *got[1].Until != today-1 || got[1].From != date(t, "2026-11-20") {
		t.Errorf("begun earlier: %v %v", got, changed)
	}
	if *running[1].Until != date(t, "2026-11-30") {
		t.Error("the pauses passed in were changed")
	}
	got, changed = ApplyResume([]Pause{past}, today)
	if changed || !reflect.DeepEqual(got, []Pause{past}) {
		t.Errorf("no running pause: %v %v", got, changed)
	}
}

func TestCheckNewLog(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "Vitamin D", "amount": 2, "unit": "Stück",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]}, "startDate": "2026-11-18", "endDate": "2026-11-30"}`)
	r.Pauses = []Pause{{From: date(t, "2026-11-20"), Until: ptrDate(date(t, "2026-11-21"))}}
	now := instant(t, "2026-11-23T09:15:30.123456Z")

	check := func(t *testing.T, body string) (RoutineLog, error) {
		t.Helper()
		return CheckNewLog(r, jsonBody(t, body), "Europe/Vienna", vienna, now)
	}

	l, err := check(t, `{"dueAt": "2026-11-23T07:00:00Z", "status": "done", "source": "notification"}`)
	if err != nil {
		t.Fatal(err)
	}
	if l.RoutineID != r.ID || l.DueAt == nil || !l.DueAt.Equal(instant(t, "2026-11-23T07:00:00Z")) ||
		l.DueLocal != "2026-11-23T08:00" || l.Status != LogDone || l.Source != LogSourceNotification ||
		l.Amount == nil || *l.Amount != 2 || l.EntryID != nil || l.TimeZone != "Europe/Vienna" {
		t.Errorf("log = %+v", l)
	}
	// takenAt and createdAt: the server time, UTC, milliseconds.
	if want := instant(t, "2026-11-23T09:15:30.123Z"); !l.TakenAt.Equal(want) || !l.CreatedAt.Equal(want) ||
		l.TakenAt.Location() != time.UTC || l.DueAt.Location() != time.UTC {
		t.Errorf("takenAt %v, createdAt %v", l.TakenAt, l.CreatedAt)
	}

	valid := []struct {
		name, body string
		check      func(t *testing.T, l RoutineLog)
	}{
		{"later today", `{"dueAt": "2026-11-23T19:00:00Z", "status": "done", "source": "manual"}`, nil},
		{"with an offset", `{"dueAt": "2026-11-23T08:00:00+01:00", "status": "done", "source": "manual"}`, func(t *testing.T, l RoutineLog) {
			if l.DueAt.Format(time.RFC3339) != "2026-11-23T07:00:00Z" {
				t.Errorf("dueAt %v", l.DueAt)
			}
		}},
		{"on a paused day", `{"dueAt": "2026-11-20T07:00:00Z", "status": "skipped", "source": "manual"}`, func(t *testing.T, l RoutineLog) {
			if l.Amount != nil {
				t.Errorf("skipped with amount %v", *l.Amount)
			}
		}},
		{"without occurrence", `{"dueAt": null, "status": "done", "source": "manual", "amount": 0.5}`, func(t *testing.T, l RoutineLog) {
			if l.DueAt != nil || l.DueLocal != "" || *l.Amount != 0.5 {
				t.Errorf("%+v", l)
			}
		}},
		{"takenAt entered", `{"status": "done", "source": "manual", "takenAt": "2026-11-22T18:30:00+01:00"}`, func(t *testing.T, l RoutineLog) {
			if l.TakenAt.Format(time.RFC3339) != "2026-11-22T17:30:00Z" {
				t.Errorf("takenAt %v", l.TakenAt)
			}
		}},
		{"takenAt null", `{"status": "done", "source": "manual", "takenAt": null}`, nil},
		{"takenAt exactly 5 minutes ahead", `{"status": "done", "source": "manual", "takenAt": "2026-11-23T09:20:30.123456Z"}`, nil},
		{"amount null", `{"status": "skipped", "source": "manual", "amount": null}`, nil},
		{"amount 100000", `{"status": "done", "source": "manual", "amount": 100000}`, nil},
	}
	for _, c := range valid {
		t.Run(c.name, func(t *testing.T) {
			l, err := check(t, c.body)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if c.check != nil {
				c.check(t, l)
			}
		})
	}

	errorCases := []struct {
		name, body, field, message string
		value                      any
	}{
		{"unknown field", `{"status": "done", "source": "manual", "entryId": "x"}`, "entryId", "Unbekanntes Feld: entryId.", nil},
		{"status missing", `{"source": "manual"}`, "status", msgLogStatus, nil},
		{"status unknown", `{"status": "maybe", "source": "manual"}`, "status", msgLogStatus, "maybe"},
		{"status before source", `{"status": "x", "source": "y"}`, "status", msgLogStatus, "x"},
		{"source missing", `{"status": "done"}`, "source", msgLogSource, nil},
		{"source entry", `{"status": "done", "source": "entry"}`, "source", msgLogSource, "entry"},
		{"dueAt no point in time", `{"status": "done", "source": "manual", "dueAt": "2026-11-23"}`, "dueAt", msgInstant, "2026-11-23"},
		{"dueAt number", `{"status": "done", "source": "manual", "dueAt": 5}`, "dueAt", msgInstant, float64(5)},
		{"takenAt no point in time", `{"status": "done", "source": "manual", "takenAt": "heute"}`, "takenAt", msgInstant, "heute"},
		{"amount not a number", `{"status": "done", "source": "manual", "amount": "2"}`, "amount", msgAmount, "2"},
		{"types before the occurrence", `{"status": "done", "source": "manual", "dueAt": "2026-11-23T07:01:00Z", "amount": "2"}`, "amount", msgAmount, "2"},

		{"wrong minute", `{"status": "done", "source": "manual", "dueAt": "2026-11-23T07:01:00Z"}`, "dueAt", msgNoOccurrence, "2026-11-23T07:01:00Z"},
		{"wrong second", `{"status": "done", "source": "manual", "dueAt": "2026-11-23T07:00:01Z"}`, "dueAt", msgNoOccurrence, "2026-11-23T07:00:01Z"},
		{"before startDate", `{"status": "done", "source": "manual", "dueAt": "2026-11-17T07:00:00Z"}`, "dueAt", msgNoOccurrence, "2026-11-17T07:00:00Z"},
		{"after endDate", `{"status": "done", "source": "manual", "dueAt": "2026-12-01T07:00:00Z"}`, "dueAt", msgNoOccurrence, "2026-12-01T07:00:00Z"},
		{"tomorrow", `{"status": "done", "source": "manual", "dueAt": "2026-11-24T07:00:00Z"}`, "dueAt", msgFutureOccurrence, "2026-11-24T07:00:00Z"},
		{"occurrence before takenAt", `{"status": "done", "source": "manual", "dueAt": "2026-11-24T07:00:00Z", "takenAt": "2027-01-01T00:00:00Z"}`, "dueAt", msgFutureOccurrence, "2026-11-24T07:00:00Z"},
		{"takenAt 5 minutes and a second ahead", `{"status": "done", "source": "manual", "takenAt": "2026-11-23T09:20:31.123456Z"}`, "takenAt", msgFutureTakenAt, "2026-11-23T09:20:31.123456Z"},
		{"takenAt before amount", `{"status": "skipped", "source": "manual", "takenAt": "2027-01-01T00:00:00Z", "amount": 1}`, "takenAt", msgFutureTakenAt, "2027-01-01T00:00:00Z"},
		{"skipped with amount", `{"status": "skipped", "source": "manual", "amount": 1}`, "amount", msgSkippedAmount, float64(1)},
		{"amount 0", `{"status": "done", "source": "manual", "amount": 0}`, "amount", msgAmount, float64(0)},
		{"amount too large", `{"status": "done", "source": "manual", "amount": 100000.5}`, "amount", msgAmount, 100000.5},
	}
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			_, err := check(t, c.body)
			checkRoutineError(t, err, c.field, c.message, c.value)
		})
	}

	t.Run("not an object", func(t *testing.T) {
		_, err := CheckNewLog(r, nil, "Europe/Vienna", vienna, now)
		checkRoutineError(t, err, "", msgLogBody, nil)
	})
}

// A routine without amount keeps an amount on a log; it counts nowhere.
func TestCheckNewLog_AmountWithoutRoutineAmount(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-18"}`)
	now := instant(t, "2026-11-23T09:00:00Z")
	l, err := CheckNewLog(r, jsonBody(t, `{"status": "done", "source": "manual", "amount": 3}`), "Europe/Vienna", vienna, now)
	if err != nil || l.Amount == nil || *l.Amount != 3 {
		t.Errorf("with amount: %+v, %v", l, err)
	}
	l, err = CheckNewLog(r, jsonBody(t, `{"status": "done", "source": "manual"}`), "Europe/Vienna", vienna, now)
	if err != nil || l.Amount != nil {
		t.Errorf("without amount: %+v, %v", l, err)
	}
}

// The occurrence moved by the clocks going forward (02:30 → 03:30) is
// accepted at its real point in time, and stored with the shifted time.
func TestCheckNewLog_ShiftedOccurrence(t *testing.T) {
	vienna := mustZone(t, "Europe/Vienna")
	r := routineFrom(t, `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["02:30"]}, "startDate": "2026-03-01"}`)
	now := instant(t, "2026-03-29T10:00:00Z")
	l, err := CheckNewLog(r, jsonBody(t, `{"dueAt": "2026-03-29T01:30:00Z", "status": "done", "source": "notification"}`), "Europe/Vienna", vienna, now)
	if err != nil || l.DueLocal != "2026-03-29T03:30" {
		t.Errorf("shifted occurrence: %+v, %v", l, err)
	}
	// 00:30 UTC would be 01:30 local: no occurrence.
	_, err = CheckNewLog(r, jsonBody(t, `{"dueAt": "2026-03-29T00:30:00Z", "status": "done", "source": "notification"}`), "Europe/Vienna", vienna, now)
	checkRoutineError(t, err, "dueAt", msgNoOccurrence, "2026-03-29T00:30:00Z")
}

func TestParseDueLocal(t *testing.T) {
	d, c, ok := parseDueLocal("2026-11-23T08:00")
	if !ok || d.String() != "2026-11-23" || c.String() != "08:00" {
		t.Errorf("parse: %v %v %v", d, c, ok)
	}
	for _, s := range []string{"", "2026-11-23 08:00", "2026-11-23T8:00", "2026-11-23T08:00:00", "2026-02-30T08:00"} {
		if _, _, ok := parseDueLocal(s); ok {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestRoundAmount(t *testing.T) {
	cases := map[float64]float64{0.1 + 0.2: 0.3, 1250: 1250, 0: 0, 87600000000: 87600000000, 1.0000004: 1, 7500.25: 7500.25}
	for in, want := range cases {
		if got := *roundAmount(in); got != want {
			t.Errorf("roundAmount(%v) = %v, want %v", in, got, want)
		}
	}
}
