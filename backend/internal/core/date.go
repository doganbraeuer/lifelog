package core

import (
	"fmt"
	"time"
)

// Plain dates and wall-clock times for routines (docs/API.md 3.16).

// Date is a plain calendar date without time and without time zone, like
// startDate, endDate, the days of a pause or "today" in the user's time
// zone. It counts the days since 1970-01-01, so days are added with +,
// counted with - and compared with < and ==, with no time zone or daylight
// saving time in between. Only a wall-clock time on a Date becomes a point
// in time, and only in the user's time zone (schedule.go).
type Date int

const secondsPerDay = 24 * 60 * 60

// DateFromYMD returns the date. Values out of range are normalized like
// time.Date does (31 November is 1 December).
func DateFromYMD(year int, month time.Month, day int) Date {
	return Date(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / secondsPerDay)
}

// DateOf returns the calendar date of the point in time t in loc.
func DateOf(t time.Time, loc *time.Location) Date {
	y, m, d := t.In(loc).Date()
	return DateFromYMD(y, m, d)
}

// ParseDate reads "YYYY-MM-DD" (time.DateOnly). Anything else, also a day
// that does not exist such as 2026-02-30, gives ok false.
func ParseDate(s string) (d Date, ok bool) {
	if len(s) != len(time.DateOnly) {
		return 0, false
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return 0, false
	}
	return DateFromYMD(t.Date()), true
}

// midnightUTC is 00:00 UTC of the date, only for calendar calculations.
func (d Date) midnightUTC() time.Time {
	return time.Unix(int64(d)*secondsPerDay, 0).UTC()
}

// YMD returns year, month and day.
func (d Date) YMD() (year int, month time.Month, day int) {
	return d.midnightUTC().Date()
}

// String gives "YYYY-MM-DD".
func (d Date) String() string {
	return d.midnightUTC().Format(time.DateOnly)
}

// Weekday: 1970-01-01 (day 0) was a Thursday.
func (d Date) Weekday() time.Weekday {
	return time.Weekday((int(d)%7 + 11) % 7)
}

// Monday returns the Monday of the week (Monday to Sunday) that contains d.
func (d Date) Monday() Date {
	return d - Date((d.Weekday()+6)%7)
}

// ClockTime is a wall-clock time in minutes after midnight, from 0 (00:00)
// to 1439 (23:59).
type ClockTime int

// ParseClockTime reads "HH:MM" with 24 hours, from 00:00 to 23:59. "8:00",
// "24:00" and "08:00:00" give ok false.
func ParseClockTime(s string) (c ClockTime, ok bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h > 23 || m > 59 {
		return 0, false
	}
	return ClockTime(h*60 + m), true
}

// String gives "HH:MM".
func (c ClockTime) String() string {
	return fmt.Sprintf("%02d:%02d", int(c)/60, int(c)%60)
}
