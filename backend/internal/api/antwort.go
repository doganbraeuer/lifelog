// Package api enthält die HTTP-Schicht: Router, Handler, das Fehlerformat
// aus docs/API.md und die JSON-Antworttypen.
package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Fehlercodes aus docs/API.md, Abschnitt 1. Weitere kommen mit den Endpunkten.
const (
	CodeInternerFehler = "INTERNER_FEHLER"
)

type fehlerAntwort struct {
	Fehler fehlerInhalt `json:"fehler"`
}

type fehlerInhalt struct {
	Code      string         `json:"code"`
	Nachricht string         `json:"nachricht"`
	Details   map[string]any `json:"details,omitempty"`
}

// schreibeJSON schreibt wert als JSON mit dem Status.
//
// SetEscapeHTML(false): Sonst würde Go "<0,5" als "<0,5" ausgeben. Das
// ist zwar gültiges JSON, aber Laborwerte sollen lesbar bleiben.
//
// Das JSON wird erst in einen Puffer geschrieben. Scheitert das Umwandeln,
// ist noch kein Status gesendet, und wir können sauber mit 500 antworten.
func schreibeJSON(w http.ResponseWriter, status int, wert any) {
	var puffer bytes.Buffer
	enc := json.NewEncoder(&puffer)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(wert); err != nil {
		slog.Error("JSON-Antwort konnte nicht erzeugt werden", "fehler", err)
		status = http.StatusInternalServerError
		puffer.Reset()
		puffer.WriteString(`{"fehler":{"code":"INTERNER_FEHLER","nachricht":"Interner Fehler im Backend."}}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(puffer.Bytes())
}

// schreibeFehler antwortet im einheitlichen Fehlerformat. details darf nil
// sein, dann fehlt das Feld im JSON.
func schreibeFehler(w http.ResponseWriter, status int, code, nachricht string, details map[string]any) {
	schreibeJSON(w, status, fehlerAntwort{Fehler: fehlerInhalt{
		Code:      code,
		Nachricht: nachricht,
		Details:   details,
	}})
}

// schreibeInternenFehler ist die Antwort für unerwartete Fehler. Die
// Ursache landet nur im Log, nie in der Antwort.
func schreibeInternenFehler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("interner Fehler", "methode", r.Method, "pfad", r.URL.Path, "fehler", err)
	schreibeFehler(w, http.StatusInternalServerError, CodeInternerFehler,
		"Da ist im Backend etwas schiefgegangen. Bitte versuche es später noch einmal.", nil)
}
