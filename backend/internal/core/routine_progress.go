package core

import (
	"slices"
	"strconv"
	"time"
)

// Status, pauses, logs and progress of a routine (docs/API.md 3.16
// "Pauses", "Routine status", "Routine progress", "Logs", 3.21, 3.22,
// 3.24 and 3.26). "Today" is always the server's date in the user's time
// zone; the service passes it in.

// RoutineStatus is computed for today, never stored.
type RoutineStatus string

const (
	RoutineUpcoming RoutineStatus = "upcoming"
	RoutineActive   RoutineStatus = "active"
	RoutinePaused   RoutineStatus = "paused"
	RoutineEnded    RoutineStatus = "ended"
)

// OccurrenceStatus is the status of an occurrence (API.md 3.16
// "Occurrences").
type OccurrenceStatus string

const (
	OccurrenceOpen    OccurrenceStatus = "open"
	OccurrenceDone    OccurrenceStatus = "done"
	OccurrenceSkipped OccurrenceStatus = "skipped"
	OccurrencePaused  OccurrenceStatus = "paused"
)

// LogStatus: what the user did.
type LogStatus string

const (
	LogDone    LogStatus = "done"
	LogSkipped LogStatus = "skipped"
)

// LogSource: where a log comes from. LogSourceEntry only arises from a
// confirmed routine suggestion (stage 2); POST /routines/{id}/logs refuses
// it.
type LogSource string

const (
	LogSourceManual       LogSource = "manual"
	LogSourceNotification LogSource = "notification"
	LogSourceEntry        LogSource = "entry"
)

// RoutineLog is a log („Verlaufseintrag“) as stored.
type RoutineLog struct {
	ID        string
	UserID    string // the owner; never part of the API response
	RoutineID string
	DueAt     *time.Time // the occurrence (UTC); nil: without an occurrence
	// DueLocal is the local date and time of DueAt in TimeZone,
	// "YYYY-MM-DDTHH:MM"; "" without an occurrence. Only stored, never sent:
	// the log belongs to the occurrence with the same local date and time
	// in the user's current time zone (API.md 3.16 "Logs", rule 2), and
	// the unique index on it allows one log per occurrence.
	DueLocal  string
	TakenAt   time.Time // UTC
	Status    LogStatus
	Amount    *float64
	Source    LogSource
	EntryID   *string // stage 2; always nil in stage 1
	TimeZone  string  // the user's time zone when the log was created
	CreatedAt time.Time
}

// More messages (API.md 3.21, 3.26).
const (
	msgPauseBody        = "Bitte die Pause als JSON-Objekt schicken."
	msgPauseUntil       = "Das Ende der Pause darf nicht vor heute liegen."
	msgLogBody          = "Bitte den Verlaufseintrag als JSON-Objekt schicken."
	msgLogStatus        = "Unbekannter Status."
	msgLogSource        = "Unbekannte Herkunft."
	msgInstant          = "Bitte einen gültigen Zeitpunkt angeben."
	msgNoOccurrence     = "Zu diesem Zeitpunkt ist kein Termin der Routine geplant."
	msgFutureOccurrence = "Termine in der Zukunft können noch nicht eingetragen werden."
	msgFutureTakenAt    = "Der Zeitpunkt darf nicht in der Zukunft liegen."
	msgSkippedAmount    = "Ausgelassene Termine haben keine Menge."
)

var (
	pauseFields  = []string{"until"}
	newLogFields = []string{"amount", "dueAt", "source", "status", "takenAt"}
)

// RoutineStatusOn returns the status of r on the day today. The first rule
// that applies wins: ended (endDate before today), upcoming (startDate
// after today), paused (a pause includes today), active.
func RoutineStatusOn(r Routine, today Date) RoutineStatus {
	switch {
	case r.EndDate != nil && *r.EndDate < today:
		return RoutineEnded
	case r.StartDate > today:
		return RoutineUpcoming
	case pausedOn(r.Pauses, today):
		return RoutinePaused
	}
	return RoutineActive
}

// pausedOn: does any pause include d? Pauses count as the union of their
// days, so overlapping pauses (possible after a change of the time zone)
// do no harm.
func pausedOn(pauses []Pause, d Date) bool {
	for _, p := range pauses {
		if p.Includes(d) {
			return true
		}
	}
	return false
}

// ParsePauseRequest reads the body of POST /routines/{id}/pause (API.md
// 3.21). The API passes an empty body as {}. until nil: paused until
// resumed. body as in ParseNewRoutine.
func ParsePauseRequest(body map[string]any) (until *Date, err error) {
	if body == nil {
		return nil, invalidRoutine(msgPauseBody, "", nil)
	}
	if err := checkFieldNames(body, "", pauseFields, nil); err != nil {
		return nil, err
	}
	v := body["until"]
	if v == nil {
		return nil, nil
	}
	d, err := parseRoutineDate(v, "until")
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ApplyPause pauses a routine from today (API.md 3.21) and returns its new
// pauses. changed is false if they stay the same (the request was sent
// twice). The pauses passed in are not changed.
//
//   - until before today is an error (details.field until).
//   - A pause that includes today only gets the new until; its from stays.
//     If several do (overlapping after a change of the time zone), all of
//     them, so the routine is paused exactly until then.
//   - Otherwise {from: today, until} is appended, unless the routine
//     already has 100 pauses (*RoutineLimitError).
func ApplyPause(pauses []Pause, today Date, until *Date) (updated []Pause, changed bool, err error) {
	if until != nil && *until < today {
		return nil, false, invalidRoutine(msgPauseUntil, "until", until.String())
	}
	out := clonePauses(pauses)
	found := false
	for i := range out {
		if out[i].Includes(today) {
			found = true
			if !equalPtr(out[i].Until, until) {
				out[i].Until = copyDate(until)
				changed = true
			}
		}
	}
	if found {
		return out, changed, nil
	}
	if len(out) >= MaxPausesPerRoutine {
		return nil, false, &RoutineLimitError{Limit: RoutineLimitPauses, Max: MaxPausesPerRoutine}
	}
	return append(out, Pause{From: today, Until: copyDate(until)}), true, nil
}

// ApplyResume resumes a routine today (API.md 3.22) and returns its new
// pauses: a pause that includes today is removed if it began today,
// otherwise it ends yesterday. Today's occurrences are then due again,
// also the earlier ones. Without such a pause nothing changes (changed
// false). The pauses passed in are not changed.
func ApplyResume(pauses []Pause, today Date) (updated []Pause, changed bool) {
	out := make([]Pause, 0, len(pauses))
	for _, p := range pauses {
		if !p.Includes(today) {
			out = append(out, Pause{From: p.From, Until: copyDate(p.Until)})
			continue
		}
		changed = true
		if p.From == today {
			continue
		}
		yesterday := today - 1
		out = append(out, Pause{From: p.From, Until: &yesterday})
	}
	return out, changed
}

func clonePauses(pauses []Pause) []Pause {
	out := make([]Pause, 0, len(pauses)+1)
	for _, p := range pauses {
		out = append(out, Pause{From: p.From, Until: copyDate(p.Until)})
	}
	return out
}

func copyDate(d *Date) *Date {
	if d == nil {
		return nil
	}
	c := *d
	return &c
}

// CheckNewLog reads and checks the body of POST /routines/{id}/logs
// (API.md 3.26) for the routine r and returns the log to store, without
// ID and owner. zone and loc are the user's time zone, now is the server
// time. body as in ParseNewRoutine.
//
// Order of the errors: body not an object, unknown field (alphabetical),
// the values of status, source, dueAt, takenAt and the type of amount,
// then: dueAt is an occurrence of r, its day is not after today, takenAt
// is not later than now + 5 minutes, no amount for skipped, the amount's
// range. Whether the occurrence already has a log (409) and the 100 logs
// per day need the database; the service checks them afterwards.
//
// Defaults: takenAt missing or null is now; a done log without amount gets
// the routine's amount (which may be null); a skipped log never has one.
// An amount for a routine without amount is kept, but counts nowhere.
// Points in time are stored in UTC, to the millisecond.
func CheckNewLog(r Routine, body map[string]any, zone string, loc *time.Location, now time.Time) (RoutineLog, error) {
	if body == nil {
		return RoutineLog{}, invalidRoutine(msgLogBody, "", nil)
	}
	if err := checkFieldNames(body, "", newLogFields, nil); err != nil {
		return RoutineLog{}, err
	}
	status, _ := body["status"].(string)
	if status != string(LogDone) && status != string(LogSkipped) {
		return RoutineLog{}, invalidRoutine(msgLogStatus, "status", body["status"])
	}
	source, _ := body["source"].(string)
	if source != string(LogSourceManual) && source != string(LogSourceNotification) {
		return RoutineLog{}, invalidRoutine(msgLogSource, "source", body["source"])
	}
	var dueAt *time.Time
	if v := body["dueAt"]; v != nil {
		t, ok := parseInstant(v)
		if !ok {
			return RoutineLog{}, invalidRoutine(msgInstant, "dueAt", v)
		}
		dueAt = &t
	}
	takenAt := now
	if v := body["takenAt"]; v != nil {
		t, ok := parseInstant(v)
		if !ok {
			return RoutineLog{}, invalidRoutine(msgInstant, "takenAt", v)
		}
		takenAt = t
	}
	var amount *float64
	if v := body["amount"]; v != nil {
		f, ok := v.(float64)
		if !ok {
			return RoutineLog{}, invalidRoutine(msgAmount, "amount", v)
		}
		amount = &f
	}

	log := RoutineLog{
		RoutineID: r.ID,
		TakenAt:   takenAt.UTC().Truncate(time.Millisecond),
		Status:    LogStatus(status),
		Source:    LogSource(source),
		TimeZone:  zone,
		CreatedAt: now.UTC().Truncate(time.Millisecond),
	}
	if dueAt != nil {
		occ, ok := findOccurrence(r, *dueAt, loc)
		if !ok {
			return RoutineLog{}, invalidRoutine(msgNoOccurrence, "dueAt", body["dueAt"])
		}
		if occ.Date > DateOf(now, loc) {
			return RoutineLog{}, invalidRoutine(msgFutureOccurrence, "dueAt", body["dueAt"])
		}
		due := occ.DueAt
		log.DueAt, log.DueLocal = &due, occ.DueLocal()
	}
	if takenAt.After(now.Add(LogFutureTolerance)) {
		return RoutineLog{}, invalidRoutine(msgFutureTakenAt, "takenAt", body["takenAt"])
	}
	switch {
	case log.Status == LogSkipped && amount != nil:
		return RoutineLog{}, invalidRoutine(msgSkippedAmount, "amount", body["amount"])
	case amount != nil && (!(*amount > 0) || *amount > MaxRoutineAmount):
		return RoutineLog{}, invalidRoutine(msgAmount, "amount", body["amount"])
	case amount != nil:
		log.Amount = amount
	case log.Status == LogDone && r.Amount != nil:
		a := *r.Amount
		log.Amount = &a
	}
	return log, nil
}

// parseInstant reads an RFC 3339 point in time, e.g.
// "2026-11-23T07:00:00Z" or "2026-11-23T08:00:00+01:00".
func parseInstant(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// findOccurrence returns the occurrence of r at exactly dueAt, if there is
// one. Occurrences on paused days count; logging them is allowed.
func findOccurrence(r Routine, dueAt time.Time, loc *time.Location) (Occurrence, bool) {
	d := DateOf(dueAt, loc)
	for _, o := range newExpander(r, loc, d, d).on(d) {
		if o.DueAt.Equal(dueAt) {
			return o, true
		}
	}
	return Occurrence{}, false
}

// RoutineOccurrences returns the occurrences of r on the local dates
// from..to with their status (API.md 3.24), sorted by DueAt: done or
// skipped with the log that belongs to the occurrence (same local date and
// time, rule 2), paused if the day is paused and there is no log,
// otherwise open. logs may contain more logs than needed.
func RoutineOccurrences(r Routine, logs []RoutineLog, from, to Date, loc *time.Location) []Occurrence {
	byDue := map[string]RoutineLog{}
	for _, l := range logs {
		if l.DueLocal != "" {
			byDue[l.DueLocal] = l
		}
	}
	list := Occurrences(r, from, to, loc)
	for i := range list {
		o := &list[i]
		if l, ok := byDue[o.DueLocal()]; ok {
			o.LogID = l.ID
			o.Status = OccurrenceDone
			if l.Status == LogSkipped {
				o.Status = OccurrenceSkipped
			}
			continue
		}
		o.Status = OccurrenceOpen
		if pausedOn(r.Pauses, o.Date) {
			o.Status = OccurrencePaused
		}
	}
	return list
}

// ProgressRange is one range of the progress (API.md 3.16 "Routine
// progress").
type ProgressRange struct {
	Done    int // occurrences of the range with a done log
	Skipped int // occurrences of the range with a skipped log
	// Due: occurrences of the range, also those later today, without those
	// on paused days unless they have a log.
	Due int
	// AmountDone and AmountTarget: nil if the routine has no amount.
	AmountDone   *float64
	AmountTarget *float64
	// Day and Days: only in the period. Day 1 is startDate, 0 before the
	// start, Days after the end.
	Day, Days int
}

// Progress: today, the week (Monday to Sunday) and the period from
// startDate to endDate (nil without endDate).
type Progress struct {
	Today  ProgressRange
	Week   ProgressRange
	Period *ProgressRange
}

// RoutineView is a routine as the API returns it: with status and progress
// for the moment of the request (API.md 3.16 "The routine").
type RoutineView struct {
	Routine
	Status   RoutineStatus
	Progress Progress
}

// ProgressSpan returns the local dates the progress of r covers on the day
// today: the week, and the period if the routine has an endDate. The
// service loads the logs whose occurrence (the date in dueLocal) or whose
// takenAt (as a local date) lies in it.
func ProgressSpan(r Routine, today Date) (from, to Date) {
	from, to = today.Monday(), today.Monday()+6
	if r.EndDate != nil {
		from, to = min(from, r.StartDate), max(to, *r.EndDate)
	}
	return from, to
}

// RoutineProgress computes the progress of r at the moment now (API.md
// 3.16 "Routine progress").
//
//   - done and skipped count the occurrences of a range that have a log.
//   - due counts every occurrence of a range, also those later today,
//     without those on paused days, unless they have a log.
//   - amountDone adds the amounts of all done logs, also of logs without
//     an occurrence. A log counts on the day of its occurrence; a log
//     without one, or whose occurrence no longer exists after a change of
//     the schedule, on the local day of its takenAt. A done log without
//     amount counts with the routine's current amount.
//   - amountTarget is amount × due. Both are nil if the routine has no
//     amount, and are rounded to 6 decimal places (0.1 + 0.2 gives 0.3).
//
// logs may contain more logs than needed; at most one log per occurrence
// (the unique index on dueLocal ensures it).
func RoutineProgress(r Routine, logs []RoutineLog, loc *time.Location, now time.Time) Progress {
	today := DateOf(now, loc)
	from, to := ProgressSpan(r, today)
	for _, l := range logs {
		if d, _, ok := parseDueLocal(l.DueLocal); ok {
			from, to = min(from, d), max(to, d)
		}
	}
	c := newProgressCounter(r, loc, from, to)
	placed := make([]placedLog, 0, len(logs))
	for _, l := range logs {
		placed = append(placed, c.place(l))
	}

	monday := today.Monday()
	p := Progress{
		Today: c.rangeProgress(today, today, placed),
		Week:  c.rangeProgress(monday, monday+6, placed),
	}
	if r.EndDate != nil {
		period := c.rangeProgress(r.StartDate, *r.EndDate, placed)
		period.Days = int(*r.EndDate-r.StartDate) + 1
		switch {
		case today < r.StartDate:
			period.Day = 0
		case today > *r.EndDate:
			period.Day = period.Days
		default:
			period.Day = int(today-r.StartDate) + 1
		}
		p.Period = &period
	}
	return p
}

// progressCounter counts the occurrences of one routine over many days. On
// a day without a change of offset a planned day has exactly as many
// occurrences as wall-clock times; only the days with a change are
// expanded.
type progressCounter struct {
	r        Routine
	loc      *time.Location
	x        *expander
	wallSet  map[ClockTime]bool
	perDay   int
	pauses   []dayRange // merged, sorted
	lastDate Date       // occurrences end here (endDate + 1 for a time moved past midnight)
}

type dayRange struct{ from, to Date }

// farFuture stands for "no end" (a pause until resumed, a routine without
// endDate): far beyond 2099, and far enough from the largest int on 32-bit
// systems that adding a day cannot overflow.
const farFuture Date = 1 << 30

// placedLog is a log with the day it counts on.
type placedLog struct {
	day        Date
	occurrence bool // its occurrence exists in the current schedule and time zone
	log        RoutineLog
}

func newProgressCounter(r Routine, loc *time.Location, from, to Date) *progressCounter {
	x := newExpander(r, loc, from, to)
	c := &progressCounter{r: r, loc: loc, x: x, wallSet: map[ClockTime]bool{}, perDay: len(x.wallTimes),
		pauses: mergePauses(r.Pauses), lastDate: farFuture}
	for _, t := range x.wallTimes {
		c.wallSet[t] = true
	}
	if r.EndDate != nil {
		c.lastDate = *r.EndDate + 1
	}
	return c
}

// mergePauses turns the pauses into sorted ranges without overlaps.
func mergePauses(pauses []Pause) []dayRange {
	ranges := make([]dayRange, 0, len(pauses))
	for _, p := range pauses {
		to := farFuture
		if p.Until != nil {
			to = *p.Until
		}
		ranges = append(ranges, dayRange{p.From, to})
	}
	slices.SortFunc(ranges, func(a, b dayRange) int { return int(a.from - b.from) })
	merged := ranges[:0]
	for _, r := range ranges {
		if n := len(merged); n > 0 && r.from <= merged[n-1].to+1 {
			merged[n-1].to = max(merged[n-1].to, r.to)
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// place finds the day a log counts on.
func (c *progressCounter) place(l RoutineLog) placedLog {
	if d, clock, ok := parseDueLocal(l.DueLocal); ok && c.occursAt(d, clock) {
		return placedLog{day: d, occurrence: true, log: l}
	}
	return placedLog{day: DateOf(l.TakenAt, c.loc), log: l}
}

// occursAt: does the routine have an occurrence at the local date d and
// time clock, in its current schedule and time zone? d must lie in the
// counter's range.
func (c *progressCounter) occursAt(d Date, clock ClockTime) bool {
	if c.x.transitions[d] {
		for _, o := range c.x.on(d) {
			if o.Time == clock {
				return true
			}
		}
		return false
	}
	return c.x.planned(d) && c.wallSet[clock]
}

// count returns the number of occurrences on the local date d.
func (c *progressCounter) count(d Date) int {
	if c.x.transitions[d] {
		return len(c.x.on(d))
	}
	if c.x.planned(d) {
		return c.perDay
	}
	return 0
}

func (c *progressCounter) rangeProgress(a, b Date, placed []placedLog) ProgressRange {
	var p ProgressRange
	pause := 0
	for d := max(a, c.r.StartDate); d <= min(b, c.lastDate); d++ {
		for pause < len(c.pauses) && c.pauses[pause].to < d {
			pause++
		}
		if pause < len(c.pauses) && c.pauses[pause].from <= d {
			continue
		}
		p.Due += c.count(d)
	}

	var amount float64
	for _, pl := range placed {
		if pl.day < a || pl.day > b {
			continue
		}
		if pl.occurrence {
			if pl.log.Status == LogDone {
				p.Done++
			} else {
				p.Skipped++
			}
			if pausedOn(c.r.Pauses, pl.day) {
				p.Due++ // an occurrence with a log counts, also on a paused day
			}
		}
		if pl.log.Status == LogDone {
			switch {
			case pl.log.Amount != nil:
				amount += *pl.log.Amount
			case c.r.Amount != nil:
				amount += *c.r.Amount
			}
		}
	}
	if c.r.Amount != nil {
		p.AmountDone = roundAmount(amount)
		p.AmountTarget = roundAmount(*c.r.Amount * float64(p.Due))
	}
	return p
}

// roundAmount rounds to 6 decimal places, so that sums such as 0.1 + 0.2
// show as 0.3 and not 0.30000000000000004. Through the decimal text, so it
// stays exact for large numbers too.
func roundAmount(x float64) *float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 6, 64), 64)
	if err != nil {
		r = x
	}
	return &r
}

// parseDueLocal reads "YYYY-MM-DDTHH:MM".
func parseDueLocal(s string) (Date, ClockTime, bool) {
	if len(s) != 16 || s[10] != 'T' {
		return 0, 0, false
	}
	d, ok := ParseDate(s[:10])
	if !ok {
		return 0, 0, false
	}
	c, ok := ParseClockTime(s[11:])
	return d, c, ok
}
