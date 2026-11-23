package core

import (
	"slices"
	"time"
)

// Expanding a schedule into occurrences (docs/API.md 3.16 "Schedule",
// "Time zone and points in time" and "Occurrences"). Occurrences are not
// stored; they are computed on request in the user's current time zone.

// Occurrence is a point in time the schedule plans.
type Occurrence struct {
	RoutineID string
	DueAt     time.Time // UTC
	// Date and Time are the local date and wall-clock time of DueAt in the
	// user's time zone. After a forward shift Time is the shifted time
	// (03:30 instead of 02:30).
	Date Date
	Time ClockTime
	// Status and LogID are only set by RoutineOccurrences
	// (routine_progress.go); "" otherwise.
	Status OccurrenceStatus
	LogID  string
}

// DueLocal is the occurrence's local date and time, "YYYY-MM-DDTHH:MM". A
// log stores it (dueLocal) and belongs to the occurrence with the same
// value in the user's current time zone (API.md 3.16 "Logs", rule 2).
func (o Occurrence) DueLocal() string {
	return o.Date.String() + "T" + o.Time.String()
}

// Occurrences returns the occurrences of r whose local date lies in
// [from, to], sorted by DueAt.
//
// Rules (API.md 3.16):
//   - hourly: every day, from windowStart every interval hours up to
//     windowEnd; daily: every interval-th day counted from startDate;
//     weekly: the weekdays of every interval-th week, weeks from Monday to
//     Sunday, the week containing startDate is week 0.
//   - no occurrences before startDate or after endDate.
//   - a wall-clock time that does not exist (clocks go forward) moves
//     forward by the length of the gap; one that exists twice (clocks go
//     back) takes the earlier point in time; two occurrences on the same
//     point in time become one.
func Occurrences(r Routine, from, to Date, loc *time.Location) []Occurrence {
	if to < from {
		return []Occurrence{}
	}
	x := newExpander(r, loc, from, to)
	out := []Occurrence{}
	for d := from; d <= to; d++ {
		out = append(out, x.on(d)...)
	}
	return out
}

// expander expands one routine in one time zone. It knows the days of
// [from, to] on which the zone's offset changes; on every other day one
// offset applies from midnight to midnight, and the occurrences follow
// directly from the wall-clock times.
type expander struct {
	routineID   string
	schedule    Schedule
	start       Date
	end         *Date
	wallTimes   []ClockTime
	loc         *time.Location
	transitions map[Date]bool
}

func newExpander(r Routine, loc *time.Location, from, to Date) *expander {
	return &expander{
		routineID:   r.ID,
		schedule:    r.Schedule,
		start:       r.StartDate,
		end:         r.EndDate,
		wallTimes:   r.Schedule.WallTimes(),
		loc:         loc,
		transitions: transitionDays(loc, from, to),
	}
}

// planned says whether the schedule plans occurrences on day p.
func (x *expander) planned(p Date) bool {
	if p < x.start || (x.end != nil && p > *x.end) {
		return false
	}
	switch x.schedule.Frequency {
	case FrequencyHourly:
		return true
	case FrequencyDaily:
		return int(p-x.start)%x.schedule.Interval == 0
	case FrequencyWeekly:
		week := int(p.Monday()-x.start.Monday()) / 7
		return week%x.schedule.Interval == 0 && slices.Contains(x.schedule.Weekdays, p.Weekday())
	}
	return false
}

// on returns the occurrences whose local date is d, sorted by DueAt.
func (x *expander) on(d Date) []Occurrence {
	if !x.transitions[d] {
		if !x.planned(d) {
			return nil
		}
		// One offset all day: every wall-clock time exists exactly once.
		y, m, day := d.YMD()
		_, offset := time.Date(y, m, day, 12, 0, 0, 0, x.loc).Zone()
		out := make([]Occurrence, 0, len(x.wallTimes))
		for _, c := range x.wallTimes {
			wall := time.Date(y, m, day, int(c)/60, int(c)%60, 0, 0, time.UTC)
			due := wall.Add(-time.Duration(offset) * time.Second)
			out = append(out, Occurrence{RoutineID: x.routineID, DueAt: due, Date: d, Time: c})
		}
		return out
	}

	// A day on which the offset changes: compute each point in time by the
	// rules. A forward shift can move a time past midnight, so the day
	// before counts too, and a shifted time can meet another one.
	var out []Occurrence
	for _, p := range []Date{d - 1, d} {
		if !x.planned(p) {
			continue
		}
		for _, c := range x.wallTimes {
			due := wallClockInstant(p, c, x.loc)
			local := due.In(x.loc)
			if DateOf(due, x.loc) != d {
				continue
			}
			out = append(out, Occurrence{RoutineID: x.routineID, DueAt: due, Date: d,
				Time: ClockTime(local.Hour()*60 + local.Minute())})
		}
	}
	slices.SortFunc(out, func(a, b Occurrence) int { return a.DueAt.Compare(b.DueAt) })
	return slices.CompactFunc(out, func(a, b Occurrence) bool { return a.DueAt.Equal(b.DueAt) })
}

// wallClockInstant returns the point in time (UTC) at which the clocks in
// loc show the wall-clock time c on day d.
//
// It does not use time.Date with loc: for a time in a gap or an overlap Go
// does not guarantee which side time.Date takes. The rules of API.md 3.16:
//   - gap (clocks go forward): no point in time shows c. The time moves
//     forward by the length of the gap, i.e. it is computed with the offset
//     before the change (02:30 in Vienna → 01:30 UTC = 03:30 summer time).
//   - overlap (clocks go back): two points in time show c; the earlier one
//     is taken (still summer time).
//
// The candidates are the offsets a day before and a day after; a change
// happens at most once within them.
func wallClockInstant(d Date, c ClockTime, loc *time.Location) time.Time {
	y, m, day := d.YMD()
	wall := time.Date(y, m, day, int(c)/60, int(c)%60, 0, 0, time.UTC) // the clock reading, as if it were UTC
	before := offsetAt(wall.Add(-24*time.Hour), loc)
	after := offsetAt(wall.Add(24*time.Hour), loc)

	var best time.Time
	for _, offset := range []int{before, after} {
		t := wall.Add(-time.Duration(offset) * time.Second)
		if showsWallClock(t, loc, wall) && (best.IsZero() || t.Before(best)) {
			best = t
		}
	}
	if !best.IsZero() {
		return best
	}
	return wall.Add(-time.Duration(before) * time.Second)
}

// DayStart returns the first point in time (UTC) of the local date d in
// loc. Usually that is midnight; where the clocks go forward at midnight
// (America/Santiago), the end of the gap. The local date d is the range
// [DayStart(d), DayStart(d+1)); for a day a zone skipped (Pacific/Apia on
// 2011-12-30) the range is empty.
func DayStart(d Date, loc *time.Location) time.Time {
	return wallClockInstant(d, 0, loc)
}

func offsetAt(t time.Time, loc *time.Location) int {
	_, offset := t.In(loc).Zone()
	return offset
}

// showsWallClock: do the clocks in loc show the reading wall at t?
func showsWallClock(t time.Time, loc *time.Location, wall time.Time) bool {
	local := t.In(loc)
	y, m, d := local.Date()
	wy, wm, wd := wall.Date()
	return y == wy && m == wm && d == wd && local.Hour() == wall.Hour() && local.Minute() == wall.Minute()
}

// transitionDays returns the local dates in [from, to] on which the offset
// of loc changes, including days a change skips entirely (Pacific/Apia
// skipped 30 December 2011). On all other days one offset applies from
// midnight to midnight.
//
// time.Time.ZoneBounds also reports boundaries where only the abbreviation
// changes, or where Go starts a new year of a recurring rule, without any
// change of the offset. Such days count as well; they are only computed
// the slow way, with the same result.
func transitionDays(loc *time.Location, from, to Date) map[Date]bool {
	days := map[Date]bool{}
	y, m, d := (from - 1).YMD()
	t := time.Date(y, m, d, 0, 0, 0, 0, loc)
	y, m, d = (to + 2).YMD()
	limit := time.Date(y, m, d, 0, 0, 0, 0, loc)
	for t.Before(limit) {
		_, next := t.ZoneBounds()
		if next.IsZero() || !next.After(t) || !next.Before(limit) {
			break
		}
		a, b := DateOf(next.Add(-time.Nanosecond), loc), DateOf(next, loc)
		if a > b {
			a, b = b, a
		}
		for day := max(a, from); day <= min(b, to); day++ {
			days[day] = true
		}
		t = next
	}
	return days
}
