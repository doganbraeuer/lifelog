// Package api contains the HTTP layer: router, handlers, the error format
// from docs/API.md and the JSON response types.
//
// The JSON field names, paths, error code values and details keys are still
// German; they change together with docs/API.md (plan step B7). The message
// texts stay German, because the app shows them.
package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error codes from docs/API.md, section 1. More come with the endpoints.
const (
	CodeInternalError = "INTERNER_FEHLER"
)

type errorResponse struct {
	Error errorBody `json:"fehler"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"nachricht"`
	Details map[string]any `json:"details,omitempty"`
}

// writeJSON writes value as JSON with the status.
//
// SetEscapeHTML(false): otherwise Go would write "<0,5" as "<0,5". That
// is valid JSON, but lab values should stay readable.
//
// The JSON is written into a buffer first. If encoding fails, no status has
// been sent yet, and we can cleanly answer with 500.
func writeJSON(w http.ResponseWriter, status int, value any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		slog.Error("could not encode JSON response", "error", err)
		status = http.StatusInternalServerError
		buf.Reset()
		buf.WriteString(`{"fehler":{"code":"INTERNER_FEHLER","nachricht":"Interner Fehler im Backend."}}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// writeError answers in the common error format. details may be nil; then
// the field is missing in the JSON.
func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	writeJSON(w, status, errorResponse{Error: errorBody{
		Code:    code,
		Message: message,
		Details: details,
	}})
}

// writeInternalError is the response for unexpected errors. The cause only
// goes into the log, never into the response.
func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, CodeInternalError,
		"Da ist im Backend etwas schiefgegangen. Bitte versuche es später noch einmal.", nil)
}
