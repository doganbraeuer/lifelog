package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Nullable is a value in a correction that can be missing, null or contain a
// text. JSON knows these three cases, Go pointers only two:
//
//	Nullable{}                        missing in the request → unchanged
//	Nullable{Set: true}               "field": null          → remove the value
//	Nullable{Set: true, Value: &s}    "field": "…"           → set the value
type Nullable struct {
	Set   bool
	Value *string
}

// FactChange changes a fact. Value nil means unchanged; null is not allowed
// for the value and is rejected already when reading the JSON.
type FactChange struct {
	ID    string
	Value *string
	Unit  Nullable
}

// Change is the content of PATCH /eintraege/{id} (API.md 3.5).
type Change struct {
	Type         *string // nil: unchanged (null is not allowed)
	DocumentDate Nullable
	Issuer       Nullable
	// Facts: nil means "infos" is missing in the request. An empty list means
	// "infos": [] and changes nothing.
	Facts []FactChange
}

// isEmpty: the request names not a single field, like the body {}.
func (c Change) isEmpty() bool {
	return c.Type == nil && !c.DocumentDate.Set && !c.Issuer.Set && c.Facts == nil
}

// Kinds of correction errors. The API turns them into the codes in API.md 3.5.
var (
	ErrInvalidCorrection   = errors.New("invalid correction")    // 400 UNGUELTIGE_ANFRAGE
	ErrFieldNotCorrectable = errors.New("field not correctable") // 400 FELD_NICHT_KORRIGIERBAR
	ErrFactNotFound        = errors.New("fact not found")        // 404 INFO_NICHT_GEFUNDEN
)

// CorrectionError describes what is wrong with a correction.
//
// Message is a German sentence for the app. Field, Value and FactID go into
// details. Error() deliberately does not contain the value: errors can end
// up in the log, and values can be health data.
type CorrectionError struct {
	Kind    error  // ErrInvalidCorrection, ErrFieldNotCorrectable or ErrFactNotFound
	Message string // for the app, German
	Field   string // e.g. "dokumentdatum", "infos.i2.wert"; empty if it is about a fact as a whole
	FactID  string // for errors concerning a fact as a whole
	Value   any    // the rejected value, only for details; nil if it does not belong there
}

func (e *CorrectionError) Error() string {
	target := e.Field
	if target == "" && e.FactID != "" {
		target = "infos." + e.FactID
	}
	if target == "" {
		return e.Kind.Error()
	}
	return e.Kind.Error() + ": " + target
}

// Is makes errors.Is(err, ErrInvalidCorrection) etc. true.
func (e *CorrectionError) Is(target error) bool { return target == e.Kind }

// Invalid builds an error of kind ErrInvalidCorrection. The API also uses it
// for errors that show up while reading the JSON.
func Invalid(message, field string, value any) *CorrectionError {
	return &CorrectionError{Kind: ErrInvalidCorrection, Message: message, Field: field, Value: value}
}

// NotCorrectable builds an error of kind ErrFieldNotCorrectable.
func NotCorrectable(field string) *CorrectionError {
	return &CorrectionError{
		Kind:    ErrFieldNotCorrectable,
		Message: fmt.Sprintf("Das Feld %s kann nicht korrigiert werden.", field),
		Field:   field,
	}
}

// update is a checked new value for a field path like "dokumentdatum" or
// "infos.i2.einheit".
type update struct {
	field string
	value *string
}

// Correct checks the change against the entry and applies it (API.md 2,
// rules for corrections, and 3.5).
//
// Procedure: everything is checked first. Only if everything is valid are
// the values written. On an error the result is therefore never half
// changed.
//
// Rules:
//  1. Afterwards the field holds the current value.
//  2. At most one correction per field. Recognized stays the original
//     value, Corrected and CorrectedAt show the latest state.
//  3. Back to the recognized value: the correction disappears.
//  4. If a value does not change, no correction is created.
//
// Whitespace at the edges is removed; an empty issuer or unit counts as
// null, an empty value is an error.
//
// e is not changed. changed names the fields whose value really changed, in
// the order of the request; if the list is empty, the caller does not need
// to store anything. It only contains field names, never values, so it may
// be logged.
func Correct(e Entry, c Change, now time.Time) (updated Entry, changed []string, err error) {
	updates, err := checkChange(e, c)
	if err != nil {
		return Entry{}, nil, err
	}

	updated = e
	updated.Facts = slices.Clone(e.Facts)
	updated.Corrections = slices.Clone(e.Corrections)
	if updated.Corrections == nil {
		updated.Corrections = []Correction{}
	}
	changed = []string{}
	for _, u := range updates {
		if apply(&updated, u.field, u.value, now.UTC()) {
			changed = append(changed, u.field)
		}
	}
	return updated, changed, nil
}

// checkChange checks everything and returns the new values as a list of
// field paths. Order: type, document date, issuer, then the facts in the
// order of the request. This way the same request always reports the same
// error.
func checkChange(e Entry, c Change) ([]update, error) {
	if c.isEmpty() {
		return nil, Invalid("Die Anfrage enthält keine Änderung.", "", nil)
	}
	var updates []update

	if c.Type != nil {
		typ := strings.TrimSpace(*c.Type)
		if !TypeAllowed(e.Module, typ) {
			return nil, Invalid(
				fmt.Sprintf("Unbekannter Typ. Erlaubt sind %s.", strings.Join(AllowedTypes(e.Module), ", ")),
				"typ", *c.Type)
		}
		updates = append(updates, update{"typ", &typ})
	}

	if c.DocumentDate.Set {
		var date *string
		if c.DocumentDate.Value != nil {
			d := strings.TrimSpace(*c.DocumentDate.Value)
			if err := CheckDate(d); err != nil {
				message := "Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden."
				if errors.Is(err, ErrNoSuchDay) {
					message = "Das Dokumentdatum ist kein gültiges Kalenderdatum."
				}
				return nil, Invalid(message, "dokumentdatum", *c.DocumentDate.Value)
			}
			date = &d
		}
		updates = append(updates, update{"dokumentdatum", date})
	}

	if c.Issuer.Set {
		updates = append(updates, update{"quelle", trimmedOrNil(c.Issuer.Value)})
	}

	seen := map[string]bool{}
	for _, fc := range c.Facts {
		if fc.ID == "" {
			return nil, Invalid("Jede Info braucht eine id.", "infos", nil)
		}
		if seen[fc.ID] {
			return nil, &CorrectionError{Kind: ErrInvalidCorrection, FactID: fc.ID,
				Message: fmt.Sprintf("Die Info %s kommt mehrfach vor.", fc.ID)}
		}
		seen[fc.ID] = true
		if fc.Value == nil && !fc.Unit.Set {
			return nil, &CorrectionError{Kind: ErrInvalidCorrection, FactID: fc.ID,
				Message: fmt.Sprintf("Die Info %s enthält keine Änderung.", fc.ID)}
		}
		if factIndex(e.Facts, fc.ID) < 0 {
			return nil, &CorrectionError{Kind: ErrFactNotFound, FactID: fc.ID,
				Message: "Diese Info gibt es in dem Eintrag nicht."}
		}
		if fc.Value != nil {
			value := strings.TrimSpace(*fc.Value)
			if value == "" {
				return nil, Invalid("Der Wert darf nicht leer sein.", "infos."+fc.ID+".wert", *fc.Value)
			}
			updates = append(updates, update{"infos." + fc.ID + ".wert", &value})
		}
		if fc.Unit.Set {
			updates = append(updates, update{"infos." + fc.ID + ".einheit", trimmedOrNil(fc.Unit.Value)})
		}
	}
	return updates, nil
}

// apply writes a checked value and maintains the corrections following
// rules 2 to 4. Result: whether the value changed.
func apply(e *Entry, field string, value *string, now time.Time) bool {
	old := read(e, field)
	if equal(old, value) {
		return false // rule 4
	}

	i := slices.IndexFunc(e.Corrections, func(c Correction) bool { return c.Field == field })
	// If there is no correction yet, the current value is the recognized one.
	recognized := old
	if i >= 0 {
		recognized = e.Corrections[i].Recognized
	}

	write(e, field, value)

	switch {
	case equal(value, recognized): // rule 3: reset
		if i >= 0 {
			e.Corrections = slices.Delete(e.Corrections, i, i+1)
		}
	case i >= 0: // rule 2: recognized stays, latest state
		e.Corrections[i].Corrected = value
		e.Corrections[i].CorrectedAt = now
	default:
		e.Corrections = append(e.Corrections, Correction{Field: field, Recognized: recognized, Corrected: value, CorrectedAt: now})
	}
	return true
}

// read returns the current value of a field path. The paths are checked, so
// the facts exist.
//
// Important: return a copy for type and value, never &e.Type. apply keeps
// the old value as "recognized" and then overwrites the field; a pointer into
// the field would then point to the new value. The *string fields (date,
// issuer, unit) are not critical: write replaces the pointer there but never
// changes the text it points to.
func read(e *Entry, field string) *string {
	switch field {
	case "typ":
		typ := e.Type
		return &typ
	case "dokumentdatum":
		return e.DocumentDate
	case "quelle":
		return e.Issuer
	}
	id, part := factPath(field)
	fact := e.Facts[factIndex(e.Facts, id)]
	if part == "wert" {
		value := fact.Value
		return &value
	}
	return fact.Unit
}

// write sets a field path. Facts is a copy (Correct), so the original stays
// untouched. Type and value are never nil after the check.
func write(e *Entry, field string, value *string) {
	switch field {
	case "typ":
		e.Type = *value
		return
	case "dokumentdatum":
		e.DocumentDate = value
		return
	case "quelle":
		e.Issuer = value
		return
	}
	id, part := factPath(field)
	fact := &e.Facts[factIndex(e.Facts, id)]
	if part == "wert" {
		fact.Value = *value
	} else {
		fact.Unit = value
	}
}

// factPath splits "infos.i2.wert" into "i2" and "wert".
func factPath(field string) (id, part string) {
	rest := strings.TrimPrefix(field, "infos.")
	i := strings.LastIndexByte(rest, '.')
	return rest[:i], rest[i+1:]
}

func factIndex(facts []Fact, id string) int {
	return slices.IndexFunc(facts, func(f Fact) bool { return f.ID == id })
}

// equal compares two optional values: both nil or both the same text.
func equal(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
