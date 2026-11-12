package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Extraction is what a model has read from a document, not yet checked. The
// model interface (package modell) returns this type, whichever provider is
// behind it.
type Extraction struct {
	Readable     bool    // false: no document recognizable (e.g. blurry photo)
	DocumentDate *string // should be "YYYY-MM-DD"
	Issuer       *string
	Type         string
	FullText     string
	Facts        []ExtractedFact
}

// ExtractedFact is a fact as the model returns it (still without an ID).
type ExtractedFact struct {
	Name  string
	Value string
	Unit  *string
}

// ErrUnreadable: the model recognizes no document (→ 422 UNREADABLE).
var ErrUnreadable = errors.New("no document recognizable")

// ErrInvalidExtraction: the model's response does not follow the rules
// (→ 502 EXTRACTION_FAILED).
var ErrInvalidExtraction = errors.New("invalid extraction")

// ExtractionError says which field does not fit. It deliberately never
// contains the value itself, because it ends up in the log and values can be
// health data.
type ExtractionError struct {
	Field  string // e.g. "documentDate" or "facts[2].name"
	Reason string
}

func (e *ExtractionError) Error() string {
	return fmt.Sprintf("invalid extraction: %s: %s", e.Field, e.Reason)
}

// Is makes errors.Is(err, ErrInvalidExtraction) true.
func (e *ExtractionError) Is(target error) bool { return target == ErrInvalidExtraction }

// ErrInvalidDate: a date is not in the format YYYY-MM-DD or not a real
// calendar date.
var ErrInvalidDate = errors.New("date must be a valid calendar date in the format YYYY-MM-DD")

// ErrNoSuchDay: the format is right, but the day does not exist (e.g.
// 2026-02-31). A special case of ErrInvalidDate; PATCH reports it with its
// own message (API.md 3.5).
var ErrNoSuchDay = fmt.Errorf("%w: this day does not exist", ErrInvalidDate)

// CheckDate checks a document date: exactly "YYYY-MM-DD" with leading
// zeros, and the date must exist (no 31 February). A wrong format gives
// ErrInvalidDate, an impossible day ErrNoSuchDay.
func CheckDate(date string) error {
	if !isDateFormat(date) {
		return ErrInvalidDate
	}
	// time.Parse rejects months and days out of range, e.g. 2026-02-30 or
	// 2026-13-01.
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return ErrNoSuchDay
	}
	return nil
}

// isDateFormat: exactly ten characters, digits with hyphens at positions 5
// and 8.
func isDateFormat(s string) bool {
	if len(s) != len("2006-01-02") {
		return false
	}
	for i, c := range []byte(s) {
		if i == 4 || i == 7 {
			if c != '-' {
				return false
			}
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CheckExtraction checks the model's response and turns it into an entry for
// the module. It sets module, type, document date, issuer, full text, facts
// (with IDs i1, i2, …) and an empty list of corrections. The caller sets ID,
// import time and file afterwards.
//
// Rules:
//   - Readable=false gives ErrUnreadable.
//   - The type must be allowed in the module.
//   - The full text must not be empty. It is not changed.
//   - Document date: empty means "not recognized" (nil), otherwise CheckDate.
//   - Issuer and unit: empty means nil.
//   - A fact without name or value is dropped, not rejected (API.md 3.1,
//     step 4). The remaining ones get IDs without gaps. Whitespace at the
//     edges is removed; otherwise the value stays exactly as on the document
//     ("<0,5" stays "<0,5").
//
// dropped names the missing fields of the dropped facts, e.g.
// "facts[2].value". The index refers to the model's list. The list never
// contains values, so the caller may log it. It is never nil.
func CheckExtraction(x Extraction, module string) (entry Entry, dropped []string, err error) {
	dropped = []string{}
	if !x.Readable {
		return Entry{}, dropped, ErrUnreadable
	}
	if !TypeAllowed(module, x.Type) {
		return Entry{}, dropped, &ExtractionError{Field: "type", Reason: "not allowed in module " + module}
	}
	if strings.TrimSpace(x.FullText) == "" {
		return Entry{}, dropped, &ExtractionError{Field: "fullText", Reason: "empty"}
	}

	date := trimmedOrNil(x.DocumentDate)
	if date != nil {
		if err := CheckDate(*date); err != nil {
			return Entry{}, dropped, &ExtractionError{Field: "documentDate", Reason: "not a valid date in the format YYYY-MM-DD"}
		}
	}

	facts := make([]Fact, 0, len(x.Facts))
	for i, xf := range x.Facts {
		name := strings.TrimSpace(xf.Name)
		value := strings.TrimSpace(xf.Value)
		if name == "" || value == "" {
			if name == "" {
				dropped = append(dropped, fmt.Sprintf("facts[%d].name", i))
			}
			if value == "" {
				dropped = append(dropped, fmt.Sprintf("facts[%d].value", i))
			}
			continue
		}
		facts = append(facts, Fact{
			ID:    fmt.Sprintf("f%d", len(facts)+1), // without gaps, even after dropped facts
			Name:  name,
			Value: value,
			Unit:  trimmedOrNil(xf.Unit),
		})
	}

	return Entry{
		Module:       module,
		Type:         x.Type,
		DocumentDate: date,
		Issuer:       trimmedOrNil(x.Issuer),
		FullText:     x.FullText,
		Facts:        facts,
		Corrections:  []Correction{},
	}, dropped, nil
}

// trimmedOrNil removes whitespace at the edges. If nothing remains, the
// result is nil.
func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
