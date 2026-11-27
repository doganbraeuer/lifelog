package core

import (
	"errors"
	"fmt"
)

// Query parameters of the routine endpoints (docs/API.md 3.17, 3.24,
// 3.25): module, from and to.

// ErrInvalidParameter: a query parameter has an invalid value
// (400 INVALID_PARAMETER). The details are in a *ParameterError.
var ErrInvalidParameter = errors.New("invalid query parameter")

// ParameterError says which query parameter is wrong. Message is German
// for the app; Parameter and Value go into details. The value is the text
// the app sent; Error() does not contain it.
type ParameterError struct {
	Parameter string
	Value     string
	Message   string
}

func (e *ParameterError) Error() string   { return ErrInvalidParameter.Error() + ": " + e.Parameter }
func (e *ParameterError) Is(t error) bool { return t == ErrInvalidParameter }

const (
	msgRangeReversed = "Das Ende des Zeitraums darf nicht vor dem Anfang liegen."
	msgRangeTooLong  = "Der Zeitraum darf höchstens %d Tage umfassen."
)

// CheckRoutineModule checks the parameter module: empty is the default
// module.
func CheckRoutineModule(value string) (string, error) {
	m, err := CheckModule(value)
	if err != nil {
		return "", &ParameterError{Parameter: "module", Value: value, Message: msgModule}
	}
	return m, nil
}

// OccurrenceRange reads from and to of GET /routines/occurrences (API.md
// 3.24): from defaults to today (in the user's time zone), to to from + 6
// days; to must not be before from, and the range has at most 31 days,
// both ends included.
func OccurrenceRange(from, to string, today Date) (Date, Date, error) {
	f, err := parseParamDate("from", from, today)
	if err != nil {
		return 0, 0, err
	}
	t, err := parseParamDate("to", to, f+6)
	if err != nil {
		return 0, 0, err
	}
	return f, t, checkDayRange(from, to, f, t, MaxOccurrenceRangeDays)
}

// LogRange reads from and to of GET /routines/{id}/logs (API.md 3.25): to
// defaults to today, from to to − 29 days; to must not be before from, and
// the range has at most 366 days, both ends included.
func LogRange(from, to string, today Date) (Date, Date, error) {
	t, err := parseParamDate("to", to, today)
	if err != nil {
		return 0, 0, err
	}
	f, err := parseParamDate("from", from, t-29)
	if err != nil {
		return 0, 0, err
	}
	return f, t, checkDayRange(from, to, f, t, MaxLogRangeDays)
}

// parseParamDate: an empty value gives the default.
func parseParamDate(name, value string, def Date) (Date, error) {
	if value == "" {
		return def, nil
	}
	d, ok := ParseDate(value)
	if !ok {
		return 0, &ParameterError{Parameter: name, Value: value, Message: msgDateFormat}
	}
	return d, nil
}

// checkDayRange names the parameter the app sent: to if it was given,
// otherwise from (to was its default).
func checkDayRange(fromValue, toValue string, f, t Date, maxDays int) error {
	param, value := "to", toValue
	if toValue == "" {
		param, value = "from", fromValue
	}
	if t < f {
		return &ParameterError{Parameter: param, Value: value, Message: msgRangeReversed}
	}
	if int(t-f)+1 > maxDays {
		return &ParameterError{Parameter: param, Value: value, Message: fmt.Sprintf(msgRangeTooLong, maxDays)}
	}
	return nil
}
