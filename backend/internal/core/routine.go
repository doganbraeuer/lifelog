package core

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Routines, stage 1 (docs/API.md 3.16–3.27; names from docs/GLOSSARY.md,
// "Routines"). A routine is like an alarm clock for something the user
// wants to do regularly: a name, an optional target amount per occurrence,
// a schedule, a start date and an optional end date.

// Frequency is the kind of schedule.
type Frequency string

const (
	FrequencyHourly Frequency = "hourly"
	FrequencyDaily  Frequency = "daily"
	FrequencyWeekly Frequency = "weekly"
)

// MaxInterval returns the largest interval of a frequency, or 0 for an
// unknown one (API.md 3.16 "Schedule").
func MaxInterval(f Frequency) int {
	switch f {
	case FrequencyHourly:
		return 23
	case FrequencyDaily:
		return 365
	case FrequencyWeekly:
		return 52
	}
	return 0
}

// Schedule says when the occurrences are (API.md 3.16 "Schedule").
type Schedule struct {
	Frequency Frequency
	Interval  int // every N hours, days or weeks
	// Times: daily and weekly. 1 to 24 different values, sorted.
	Times []ClockTime
	// Weekdays: weekly. 1 to 7 different values, Monday first.
	Weekdays []time.Weekday
	// WindowStart and WindowEnd: hourly, WindowStart < WindowEnd.
	WindowStart, WindowEnd ClockTime
}

// WallTimes returns the planned wall-clock times of a day with
// occurrences, sorted: for hourly from WindowStart every Interval hours up
// to WindowEnd (which counts if it is hit), otherwise Times.
func (s Schedule) WallTimes() []ClockTime {
	if s.Frequency != FrequencyHourly {
		return slices.Clone(s.Times)
	}
	if s.Interval < 1 {
		return nil
	}
	var times []ClockTime
	for t := s.WindowStart; t <= s.WindowEnd; t += ClockTime(s.Interval * 60) {
		times = append(times, t)
	}
	return times
}

// Equal says whether both schedules plan the same.
func (s Schedule) Equal(o Schedule) bool {
	return s.Frequency == o.Frequency && s.Interval == o.Interval &&
		slices.Equal(s.Times, o.Times) && slices.Equal(s.Weekdays, o.Weekdays) &&
		s.WindowStart == o.WindowStart && s.WindowEnd == o.WindowEnd
}

// weekdayCodes are the values of schedule.weekdays, indexed by
// time.Weekday.
var weekdayCodes = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// WeekdayCode gives the code of a weekday, e.g. "tue".
func WeekdayCode(d time.Weekday) string {
	return weekdayCodes[d]
}

// ParseWeekdayCode reads a code such as "tue".
func ParseWeekdayCode(s string) (time.Weekday, bool) {
	i := slices.Index(weekdayCodes[:], s)
	return time.Weekday(i), i >= 0
}

// mondayFirst numbers the weekdays from Monday (0) to Sunday (6), the
// order in which weekdays are stored.
func mondayFirst(d time.Weekday) int {
	return (int(d) + 6) % 7
}

// Pause is a range of whole days in the user's time zone (API.md 3.16
// "Pauses").
type Pause struct {
	From  Date  // first paused day
	Until *Date // last paused day, inclusive; nil: paused until resumed
}

// Includes says whether the day d is paused by p.
func (p Pause) Includes(d Date) bool {
	return p.From <= d && (p.Until == nil || d <= *p.Until)
}

// Routine is a routine as stored. Status and progress are not stored; they
// are computed for the moment of the request (routine_progress.go).
type Routine struct {
	ID        string
	UserID    string // the owner; never part of the API response
	Module    string
	Name      string
	Amount    *float64 // target amount per occurrence; nil: none
	Unit      *string  // only with an Amount
	Schedule  Schedule
	StartDate Date
	EndDate   *Date // last day, inclusive; nil: no end
	Pauses    []Pause
	CreatedAt time.Time
}

// RoutineNameKey is the name in lower case. The unique index on user,
// module and this key makes names unique ignoring upper and lower case. It
// is strings.ToLower without further Unicode folding, so "ß" and "ss" stay
// different names.
func RoutineNameKey(name string) string {
	return strings.ToLower(name)
}

// Limits (API.md 3.16 "Limits and validation").
const (
	MaxRoutinesPerUser     = 50 // over all modules and statuses
	MaxRoutineNameChars    = 60
	MaxRoutineAmount       = 100000
	MaxRoutineUnitChars    = 20
	MaxTimesPerDay         = 24
	MaxPausesPerRoutine    = 100
	MaxLogsPerRoutineDay   = 100 // day of takenAt in the user's time zone
	MaxOccurrenceRangeDays = 31  // GET /routines/occurrences
	MaxLogRangeDays        = 366 // GET /routines/{id}/logs

	// LogFutureTolerance: takenAt may be up to 5 minutes after the server
	// time, for a time picker that starts from the phone's clock.
	LogFutureTolerance = 5 * time.Minute
)

// Dates of routines and pauses lie between 2000-01-01 and 2099-12-31. This
// keeps the progress of a period cheap to compute: without an upper limit,
// an endDate of 9999-12-31 would mean counting millions of days.
var (
	MinRoutineDate = DateFromYMD(2000, time.January, 1)
	MaxRoutineDate = DateFromYMD(2099, time.December, 31)
)

var (
	// ErrInvalidRoutine: a request about a routine or a log breaks a rule
	// (400 INVALID_REQUEST). The details are in a *RoutineError.
	ErrInvalidRoutine = errors.New("invalid routine request")

	ErrRoutineNotFound    = errors.New("routine not found")     // 404 ROUTINE_NOT_FOUND
	ErrRoutineLogNotFound = errors.New("routine log not found") // 404 ROUTINE_LOG_NOT_FOUND

	// ErrRoutineNameTaken: the user already has a routine with this name in
	// the module (409 ROUTINE_NAME_TAKEN). The service reports it as a
	// *RoutineNameTakenError with the ID of that routine.
	ErrRoutineNameTaken = errors.New("routine name already taken")

	// ErrRoutineLogExists: the occurrence already has a log
	// (409 ROUTINE_LOG_EXISTS). The service reports it as a
	// *RoutineLogExistsError with the ID of that log.
	ErrRoutineLogExists = errors.New("occurrence already has a log")
)

// RoutineError says what is wrong with a request about a routine or a log.
//
// Message is a German sentence for the app. Field and Value go into
// details. Error() deliberately does not contain the value: errors can end
// up in the log, and names, amounts and times are health data.
type RoutineError struct {
	Message string // for the app, German
	Field   string // e.g. "schedule.times"; empty if it is about the whole body
	Value   any    // the rejected value as sent; nil if there is none
}

func (e *RoutineError) Error() string {
	if e.Field == "" {
		return ErrInvalidRoutine.Error()
	}
	return ErrInvalidRoutine.Error() + ": " + e.Field
}

// Is makes errors.Is(err, ErrInvalidRoutine) true.
func (e *RoutineError) Is(target error) bool { return target == ErrInvalidRoutine }

func invalidRoutine(message, field string, value any) *RoutineError {
	return &RoutineError{Message: message, Field: field, Value: value}
}

// RoutineNameTakenError: 409 ROUTINE_NAME_TAKEN with details.routineId.
type RoutineNameTakenError struct {
	RoutineID string
}

func (e *RoutineNameTakenError) Error() string   { return ErrRoutineNameTaken.Error() }
func (e *RoutineNameTakenError) Is(t error) bool { return t == ErrRoutineNameTaken }

// RoutineLogExistsError: 409 ROUTINE_LOG_EXISTS with details.logId.
type RoutineLogExistsError struct {
	LogID string
}

func (e *RoutineLogExistsError) Error() string   { return ErrRoutineLogExists.Error() }
func (e *RoutineLogExistsError) Is(t error) bool { return t == ErrRoutineLogExists }

// RoutineLimit names a limit, as in details.limit of ROUTINE_LIMIT_REACHED.
type RoutineLimit string

const (
	RoutineLimitRoutines RoutineLimit = "routines"
	RoutineLimitPauses   RoutineLimit = "pauses"
	RoutineLimitLogs     RoutineLimit = "logs"
)

// RoutineLimitError: 409 ROUTINE_LIMIT_REACHED with details.limit and
// details.max.
type RoutineLimitError struct {
	Limit RoutineLimit
	Max   int
}

func (e *RoutineLimitError) Error() string {
	return fmt.Sprintf("routine limit %q reached (max %d)", string(e.Limit), e.Max)
}

// Messages of RoutineError (API.md 3.18, 3.20). They are user-facing and
// therefore German.
const (
	msgRoutineBody      = "Bitte die Routine als JSON-Objekt schicken."
	msgPatchBody        = "Bitte die Änderung als JSON-Objekt schicken."
	msgEmptyPatch       = "Bitte mindestens ein Feld zum Ändern schicken."
	msgUnknownField     = "Unbekanntes Feld: %s."
	msgReadOnlyField    = "Das Feld %s kann nicht geändert werden."
	msgModule           = "Unbekanntes Modul. Im Prototyp gibt es nur health."
	msgName             = "Der Name muss 1 bis 60 Zeichen lang sein."
	msgAmount           = "Die Menge muss größer als 0 und höchstens 100.000 sein."
	msgUnitNeedsAmount  = "Eine Einheit braucht eine Menge."
	msgUnitLength       = "Die Einheit darf höchstens 20 Zeichen lang sein."
	msgUnitType         = "Die Einheit muss ein Text sein."
	msgSchedule         = "Bitte einen Rhythmus angeben."
	msgFrequency        = "Unbekannter Rhythmus."
	msgForeignField     = "Dieses Feld passt nicht zum gewählten Rhythmus."
	msgInterval         = "Das Intervall muss zwischen 1 und %d liegen."
	msgTimeFormat       = "Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00."
	msgTimesEmpty       = "Bitte mindestens eine Uhrzeit angeben."
	msgTimesMax         = "Höchstens 24 Uhrzeiten pro Tag."
	msgTimesDuplicate   = "Jede Uhrzeit nur einmal."
	msgWindowOrder      = "Das Ende des Zeitfensters muss nach dem Beginn liegen."
	msgWeekdaysEmpty    = "Bitte mindestens einen Wochentag wählen."
	msgWeekdayUnknown   = "Unbekannter Wochentag."
	msgWeekdayDuplicate = "Jeden Wochentag nur einmal."
	msgDateFormat       = "Das Datum muss im Format JJJJ-MM-TT angegeben werden."
	msgDateRange        = "Das Datum muss zwischen 2000 und 2099 liegen."
	msgEndBeforeStart   = "Das Enddatum darf nicht vor dem Startdatum liegen."
	msgStartAfterEnd    = "Das Startdatum darf nicht nach dem Enddatum liegen."
)

// Fields of the requests.
var (
	routineCreateFields   = []string{"amount", "endDate", "module", "name", "schedule", "startDate", "unit"}
	routinePatchFields    = []string{"amount", "endDate", "name", "schedule", "startDate", "unit"}
	routineReadOnlyFields = []string{"createdAt", "id", "module", "pauses", "progress", "status"}
	scheduleFields        = []string{"frequency", "interval", "times", "weekdays", "windowEnd", "windowStart"}
)

// scheduleFieldsOf: the fields that belong to each frequency.
var scheduleFieldsOf = map[Frequency][]string{
	FrequencyHourly: {"frequency", "interval", "windowEnd", "windowStart"},
	FrequencyDaily:  {"frequency", "interval", "times"},
	FrequencyWeekly: {"frequency", "interval", "times", "weekdays"},
}

// ParseNewRoutine reads the body of POST /routines (API.md 3.18) and checks
// every rule of 3.16 "Limits and validation". It returns the routine
// without ID, owner and creation time, with no pauses; the service sets
// them.
//
// body is the request as encoding/json decodes it into a map[string]any
// (objects map[string]any, lists []any, numbers float64), nil if the body
// is not a JSON object. Reading it here instead of into a struct keeps
// "missing", null and a wrong type apart, and keeps every rule and every
// message of routines in one place.
//
// The first error is reported, in this order: unknown field
// (alphabetical), module, name, amount, unit, schedule, startDate,
// endDate, endDate before startDate. The same request therefore always
// gives the same error.
func ParseNewRoutine(body map[string]any) (Routine, error) {
	if body == nil {
		return Routine{}, invalidRoutine(msgRoutineBody, "", nil)
	}
	if err := checkFieldNames(body, "", routineCreateFields, nil); err != nil {
		return Routine{}, err
	}
	r := Routine{Pauses: []Pause{}}
	var err error
	if r.Module, err = parseRoutineModule(body["module"]); err != nil {
		return Routine{}, err
	}
	if r.Name, err = parseRoutineName(body["name"]); err != nil {
		return Routine{}, err
	}
	if v := body["amount"]; v != nil {
		a, err := parseRoutineAmount(v)
		if err != nil {
			return Routine{}, err
		}
		r.Amount = &a
	}
	if r.Unit, err = parseRoutineUnit(body["unit"]); err != nil {
		return Routine{}, err
	}
	if r.Unit != nil && r.Amount == nil {
		return Routine{}, invalidRoutine(msgUnitNeedsAmount, "unit", body["unit"])
	}
	if r.Schedule, err = parseSchedule(body["schedule"]); err != nil {
		return Routine{}, err
	}
	if r.StartDate, err = parseRoutineDate(body["startDate"], "startDate"); err != nil {
		return Routine{}, err
	}
	if v := body["endDate"]; v != nil {
		end, err := parseRoutineDate(v, "endDate")
		if err != nil {
			return Routine{}, err
		}
		r.EndDate = &end
	}
	if r.EndDate != nil && *r.EndDate < r.StartDate {
		return Routine{}, invalidRoutine(msgEndBeforeStart, "endDate", body["endDate"])
	}
	return r, nil
}

// ApplyRoutinePatch applies the body of PATCH /routines/{id} (API.md 3.20)
// to r and checks the whole routine afterwards. A missing field stays
// unchanged, null clears it; amount: null also clears the unit. The
// schedule is replaced as a whole.
//
// changed lists the fields whose stored value changes, by their API names
// in a fixed order (name, amount, unit, schedule, startDate, endDate). If
// it is empty, the request only repeated values that already apply, and
// nothing needs to be stored. r itself is not changed.
//
// body as in ParseNewRoutine. Order of the errors: body not an object,
// empty body, field names (alphabetical; read-only and unknown), name,
// amount, unit, schedule, startDate, endDate, then the rules over the
// merged routine (unit needs an amount, endDate not before startDate).
func ApplyRoutinePatch(r Routine, body map[string]any) (updated Routine, changed []string, err error) {
	if body == nil {
		return Routine{}, nil, invalidRoutine(msgPatchBody, "", nil)
	}
	if len(body) == 0 {
		return Routine{}, nil, invalidRoutine(msgEmptyPatch, "", nil)
	}
	if err := checkFieldNames(body, "", routinePatchFields, routineReadOnlyFields); err != nil {
		return Routine{}, nil, err
	}

	n := r
	if v, ok := body["name"]; ok {
		if n.Name, err = parseRoutineName(v); err != nil {
			return Routine{}, nil, err
		}
	}
	if v, ok := body["amount"]; ok {
		if v == nil {
			n.Amount, n.Unit = nil, nil
		} else {
			a, err := parseRoutineAmount(v)
			if err != nil {
				return Routine{}, nil, err
			}
			n.Amount = &a
		}
	}
	if v, ok := body["unit"]; ok {
		if n.Unit, err = parseRoutineUnit(v); err != nil {
			return Routine{}, nil, err
		}
	}
	if v, ok := body["schedule"]; ok {
		if n.Schedule, err = parseSchedule(v); err != nil {
			return Routine{}, nil, err
		}
	}
	if v, ok := body["startDate"]; ok {
		if n.StartDate, err = parseRoutineDate(v, "startDate"); err != nil {
			return Routine{}, nil, err
		}
	}
	if v, ok := body["endDate"]; ok {
		if v == nil {
			n.EndDate = nil
		} else {
			end, err := parseRoutineDate(v, "endDate")
			if err != nil {
				return Routine{}, nil, err
			}
			n.EndDate = &end
		}
	}

	// The old routine was valid, so a unit without an amount can only come
	// from this request: a unit sent while the amount is (or becomes) null.
	if n.Unit != nil && n.Amount == nil {
		return Routine{}, nil, invalidRoutine(msgUnitNeedsAmount, "unit", body["unit"])
	}
	if n.EndDate != nil && *n.EndDate < n.StartDate {
		if v := body["endDate"]; v != nil {
			return Routine{}, nil, invalidRoutine(msgEndBeforeStart, "endDate", v)
		}
		return Routine{}, nil, invalidRoutine(msgStartAfterEnd, "startDate", body["startDate"])
	}
	return n, changedRoutineFields(r, n), nil
}

func changedRoutineFields(old, n Routine) []string {
	changed := []string{}
	if old.Name != n.Name {
		changed = append(changed, "name")
	}
	if !equalPtr(old.Amount, n.Amount) {
		changed = append(changed, "amount")
	}
	if !equalPtr(old.Unit, n.Unit) {
		changed = append(changed, "unit")
	}
	if !old.Schedule.Equal(n.Schedule) {
		changed = append(changed, "schedule")
	}
	if old.StartDate != n.StartDate {
		changed = append(changed, "startDate")
	}
	if !equalPtr(old.EndDate, n.EndDate) {
		changed = append(changed, "endDate")
	}
	return changed
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// checkFieldNames reports the first field name (alphabetical) of obj that
// is not allowed. Read-only fields get a message of their own. prefix
// names the enclosing object in the error, e.g. "schedule.".
func checkFieldNames(obj map[string]any, prefix string, allowed, readOnly []string) error {
	for _, name := range sortedFieldNames(obj) {
		switch {
		case slices.Contains(allowed, name):
		case slices.Contains(readOnly, name):
			return invalidRoutine(fmt.Sprintf(msgReadOnlyField, prefix+name), prefix+name, nil)
		default:
			return invalidRoutine(fmt.Sprintf(msgUnknownField, prefix+name), prefix+name, nil)
		}
	}
	return nil
}

func sortedFieldNames(obj map[string]any) []string {
	names := make([]string, 0, len(obj))
	for name := range obj {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// parseRoutineModule: missing, null or "" is the default module.
func parseRoutineModule(v any) (string, error) {
	if v == nil {
		return DefaultModule, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", invalidRoutine(msgModule, "module", v)
	}
	m, err := CheckModule(s)
	if err != nil {
		return "", invalidRoutine(msgModule, "module", v)
	}
	return m, nil
}

// parseRoutineName: a string; whitespace at the edges removed, then 1 to 60
// characters without control characters.
func parseRoutineName(v any) (string, error) {
	s, ok := v.(string)
	name, valid := trimText(s, MaxRoutineNameChars)
	if !ok || !valid || name == "" {
		return "", invalidRoutine(msgName, "name", v)
	}
	return name, nil
}

// parseRoutineAmount: a number greater than 0 and at most 100,000.
func parseRoutineAmount(v any) (float64, error) {
	f, ok := v.(float64)
	if !ok || !(f > 0) || f > MaxRoutineAmount {
		return 0, invalidRoutine(msgAmount, "amount", v)
	}
	return f, nil
}

// parseRoutineUnit: null, or a string; whitespace at the edges removed,
// then at most 20 characters without control characters. Empty counts as
// null.
func parseRoutineUnit(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, invalidRoutine(msgUnitType, "unit", v)
	}
	unit, valid := trimText(s, MaxRoutineUnitChars)
	if !valid {
		return nil, invalidRoutine(msgUnitLength, "unit", v)
	}
	if unit == "" {
		return nil, nil
	}
	return &unit, nil
}

// trimText removes whitespace at the edges and checks the rest: valid
// UTF-8, at most maxChars characters (Unicode characters, not bytes), no
// control characters such as line breaks or tabs. Empty is allowed here.
func trimText(s string, maxChars int) (string, bool) {
	t := strings.TrimSpace(s)
	if !utf8.ValidString(t) || utf8.RuneCountInString(t) > maxChars {
		return "", false
	}
	for _, r := range t {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return t, true
}

// parseRoutineDate: "YYYY-MM-DD", a day that exists, from 2000 to 2099.
func parseRoutineDate(v any, field string) (Date, error) {
	s, isString := v.(string)
	d, ok := ParseDate(s)
	if !isString || !ok {
		return 0, invalidRoutine(msgDateFormat, field, v)
	}
	if d < MinRoutineDate || d > MaxRoutineDate {
		return 0, invalidRoutine(msgDateRange, field, v)
	}
	return d, nil
}

// parseSchedule reads "schedule" (API.md 3.16 "Schedule"). Order: object,
// unknown field, frequency, fields that do not belong to the frequency,
// interval, then the frequency's own fields in the order of the contract
// (windowStart, windowEnd; weekdays, times).
func parseSchedule(v any) (Schedule, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return Schedule{}, invalidRoutine(msgSchedule, "schedule", v)
	}
	if err := checkFieldNames(obj, "schedule.", scheduleFields, nil); err != nil {
		return Schedule{}, err
	}
	name, _ := obj["frequency"].(string)
	s := Schedule{Frequency: Frequency(name)}
	maxInterval := MaxInterval(s.Frequency)
	if maxInterval == 0 {
		return Schedule{}, invalidRoutine(msgFrequency, "schedule.frequency", obj["frequency"])
	}
	for _, field := range sortedFieldNames(obj) {
		if !slices.Contains(scheduleFieldsOf[s.Frequency], field) {
			return Schedule{}, invalidRoutine(msgForeignField, "schedule."+field, nil)
		}
	}

	// A whole number: 2.0 is fine, 1.5 is not.
	interval, ok := obj["interval"].(float64)
	if !ok || interval < 1 || interval > float64(maxInterval) || interval != math.Trunc(interval) {
		return Schedule{}, invalidRoutine(fmt.Sprintf(msgInterval, maxInterval), "schedule.interval", obj["interval"])
	}
	s.Interval = int(interval)

	var err error
	switch s.Frequency {
	case FrequencyHourly:
		if s.WindowStart, err = parseClockField(obj, "windowStart"); err != nil {
			return Schedule{}, err
		}
		if s.WindowEnd, err = parseClockField(obj, "windowEnd"); err != nil {
			return Schedule{}, err
		}
		if s.WindowEnd <= s.WindowStart {
			return Schedule{}, invalidRoutine(msgWindowOrder, "schedule.windowEnd", obj["windowEnd"])
		}
	case FrequencyWeekly:
		if s.Weekdays, err = parseWeekdays(obj["weekdays"]); err != nil {
			return Schedule{}, err
		}
		fallthrough
	case FrequencyDaily:
		if s.Times, err = parseTimes(obj["times"]); err != nil {
			return Schedule{}, err
		}
	}
	return s, nil
}

func parseClockField(obj map[string]any, name string) (ClockTime, error) {
	s, isString := obj[name].(string)
	c, ok := ParseClockTime(s)
	if !isString || !ok {
		return 0, invalidRoutine(msgTimeFormat, "schedule."+name, obj[name])
	}
	return c, nil
}

// parseTimes: a list of 1 to 24 different times "HH:MM"; stored sorted.
func parseTimes(v any) ([]ClockTime, error) {
	const field = "schedule.times"
	if v == nil {
		return nil, invalidRoutine(msgTimesEmpty, field, nil)
	}
	list, ok := v.([]any)
	if !ok {
		return nil, invalidRoutine(msgTimeFormat, field, v)
	}
	if len(list) == 0 {
		return nil, invalidRoutine(msgTimesEmpty, field, nil)
	}
	if len(list) > MaxTimesPerDay {
		return nil, invalidRoutine(msgTimesMax, field, nil)
	}
	times := make([]ClockTime, 0, len(list))
	for _, e := range list {
		s, isString := e.(string)
		c, ok := ParseClockTime(s)
		if !isString || !ok {
			return nil, invalidRoutine(msgTimeFormat, field, e)
		}
		if slices.Contains(times, c) {
			return nil, invalidRoutine(msgTimesDuplicate, field, e)
		}
		times = append(times, c)
	}
	slices.Sort(times)
	return times, nil
}

// parseWeekdays: a list of 1 to 7 different codes; stored Monday first.
func parseWeekdays(v any) ([]time.Weekday, error) {
	const field = "schedule.weekdays"
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, invalidRoutine(msgWeekdaysEmpty, field, v)
	}
	days := make([]time.Weekday, 0, len(list))
	for _, e := range list {
		s, _ := e.(string)
		d, ok := ParseWeekdayCode(s)
		if !ok {
			return nil, invalidRoutine(msgWeekdayUnknown, field, e)
		}
		if slices.Contains(days, d) {
			return nil, invalidRoutine(msgWeekdayDuplicate, field, e)
		}
		days = append(days, d)
	}
	slices.SortFunc(days, func(a, b time.Weekday) int { return mondayFirst(a) - mondayFirst(b) })
	return days, nil
}
