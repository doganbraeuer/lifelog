package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	"lifelog/backend/internal/core"
)

// Error codes for corrections (API.md 3.5).
const (
	CodeFieldNotCorrectable = "FIELD_NOT_CORRECTABLE"
	CodeFactNotFound        = "FACT_NOT_FOUND"
)

// maxCorrectionBody: limit for the whole body; there are no separate length
// limits for single values yet (API.md 4, length limits).
const maxCorrectionBody = 64 << 10

// Corrector corrects an entry. Implemented by service.Corrector.
type Corrector interface {
	Correct(ctx context.Context, id string, c core.Change) (core.Entry, error)
}

// Fields of the entry that exist but must not be corrected. They get
// FIELD_NOT_CORRECTABLE instead of "unknown field".
var notCorrectableFields = []string{"corrections", "file", "fullText", "id", "importedAt", "module"}

// correctHandler answers PATCH /api/v1/entries/{id} (API.md 3.5).
func correctHandler(c Corrector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// Check the ID before the body: "/entries/abc" is always INVALID_ID.
		if err := core.CheckID(id); err != nil {
			writeError(w, http.StatusBadRequest, CodeInvalidID, "Diese Eintrag-ID ist ungültig.", nil)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCorrectionBody))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Die Anfrage ist zu groß oder unvollständig.", nil)
			return
		}
		change, err := parseChange(body)
		if err != nil {
			writeCorrectionError(w, r, err)
			return
		}

		e, err := c.Correct(r.Context(), id, change)
		if err != nil {
			writeCorrectionError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, entryToJSON(e))
	}
}

func writeCorrectionError(w http.ResponseWriter, r *http.Request, err error) {
	var ce *core.CorrectionError
	switch {
	case errors.As(err, &ce):
		status, code := http.StatusBadRequest, CodeInvalidRequest
		switch {
		case errors.Is(err, core.ErrFieldNotCorrectable):
			code = CodeFieldNotCorrectable
		case errors.Is(err, core.ErrFactNotFound):
			status, code = http.StatusNotFound, CodeFactNotFound
		}
		writeError(w, status, code, ce.Message, correctionDetails(ce))
	case errors.Is(err, core.ErrInvalidID):
		writeError(w, http.StatusBadRequest, CodeInvalidID, "Diese Eintrag-ID ist ungültig.", nil)
	case errors.Is(err, core.ErrEntryNotFound):
		writeError(w, http.StatusNotFound, CodeEntryNotFound, "Diesen Eintrag gibt es nicht.", nil)
	case errors.Is(err, context.Canceled):
		// The app gave up; nobody reads the response.
	default:
		writeInternalError(w, r, err)
	}
}

// correctionDetails: field, value and fact ID, as far as present (API.md
// 3.5). The value only goes back to the app that sent it, never into the
// log.
func correctionDetails(ce *core.CorrectionError) map[string]any {
	d := map[string]any{}
	if ce.Field != "" {
		d["field"] = ce.Field
	}
	if ce.Value != nil {
		d["value"] = ce.Value
	}
	if ce.FactID != "" {
		d["factId"] = ce.FactID
	}
	if len(d) == 0 {
		return nil
	}
	return d
}

// parseChange translates the JSON body into a core.Change. This catches
// everything that is wrong with the JSON itself (wrong type, unknown field,
// null where none is allowed). What needs the entry (type in the module,
// valid date, fact exists) is checked by core.Correct.
//
// Why not simply decode into a struct? Then "field missing" and
// "field": null would both be nil. With json.RawMessage we see the
// difference.
//
// Fields are checked alphabetically, so the same request always gives the
// same error (a Go map has no fixed order).
func parseChange(body []byte) (core.Change, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return core.Change{}, core.Invalid(
			"Bitte die Änderung als JSON-Objekt schicken, z. B. {\"issuer\": \"…\"}.", "", nil)
	}

	names := sortedKeys(fields)
	for _, name := range names {
		switch {
		case slices.Contains(notCorrectableFields, name):
			return core.Change{}, core.NotCorrectable(name)
		case !slices.Contains([]string{"documentDate", "facts", "issuer", "type"}, name):
			return core.Change{}, core.Invalid("Unbekanntes Feld: "+name+".", name, nil)
		}
	}

	var c core.Change
	if raw, ok := fields["type"]; ok {
		typ, isString := asString(raw)
		if !isString {
			return core.Change{}, core.Invalid("Der Typ muss ein Text sein.", "type", asValue(raw))
		}
		c.Type = &typ
	}
	if raw, ok := fields["documentDate"]; ok {
		n, ok := asNullable(raw)
		if !ok {
			return core.Change{}, core.Invalid("Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.",
				"documentDate", asValue(raw))
		}
		c.DocumentDate = n
	}
	if raw, ok := fields["issuer"]; ok {
		n, ok := asNullable(raw)
		if !ok {
			return core.Change{}, core.Invalid("Die Quelle muss ein Text oder null sein.", "issuer", asValue(raw))
		}
		c.Issuer = n
	}
	if raw, ok := fields["facts"]; ok {
		facts, err := parseFacts(raw)
		if err != nil {
			return core.Change{}, err
		}
		c.Facts = facts
	}
	return c, nil
}

// parseFacts: "facts" must be a list of objects with id and value and/or
// unit. The result is never nil, also for "facts": [].
func parseFacts(raw json.RawMessage) ([]core.FactChange, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil || list == nil {
		return nil, core.Invalid("\"facts\" muss eine Liste sein.", "facts", nil)
	}
	facts := make([]core.FactChange, 0, len(list))
	for _, element := range list {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(element, &fields); err != nil || fields == nil {
			return nil, core.Invalid("Jede Info muss ein Objekt sein.", "facts", nil)
		}
		id, isString := asString(fields["id"])
		if !isString || id == "" {
			return nil, core.Invalid("Jede Info braucht eine id.", "facts", nil)
		}
		path := "facts." + id + "."

		for _, name := range sortedKeys(fields) {
			switch name {
			case "id", "value", "unit":
			case "name":
				return nil, core.NotCorrectable(path + "name")
			default:
				return nil, core.Invalid("Unbekanntes Feld: "+path+name+".", path+name, nil)
			}
		}

		fact := core.FactChange{ID: id}
		if raw, ok := fields["value"]; ok {
			value, isString := asString(raw)
			if !isString {
				return nil, core.Invalid("Der Wert muss ein Text sein.", path+"value", asValue(raw))
			}
			fact.Value = &value
		}
		if raw, ok := fields["unit"]; ok {
			n, ok := asNullable(raw)
			if !ok {
				return nil, core.Invalid("Die Einheit muss ein Text oder null sein.", path+"unit", asValue(raw))
			}
			fact.Unit = n
		}
		facts = append(facts, fact)
	}
	return facts, nil
}

// asString: is raw a JSON string? null, numbers etc. are not.
func asString(raw json.RawMessage) (string, bool) {
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// asNullable: null or a string, for fields that may be removed.
func asNullable(raw json.RawMessage) (core.Nullable, bool) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return core.Nullable{Set: true}, true
	}
	s, ok := asString(raw)
	if !ok {
		return core.Nullable{}, false
	}
	return core.Nullable{Set: true, Value: &s}, true
}

// asValue decodes a rejected value for details. null gives nil, and the
// value is then missing in details.
func asValue(raw json.RawMessage) any {
	var v any
	_ = json.Unmarshal(raw, &v)
	return v
}

func sortedKeys(m map[string]json.RawMessage) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
