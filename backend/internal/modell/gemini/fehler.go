package gemini

import (
	"context"
	"errors"
	"fmt"
	"net"

	"google.golang.org/genai"

	"lifelog/backend/internal/konfig"
	"lifelog/backend/internal/modell"
)

// uebersetzeFehler macht aus einem Fehler des Gemini-SDK einen Fehler, den
// der Rest des Backends versteht (modell.ErrNichtErreichbar usw.).
//
// Der API-Schlüssel wird aus jeder Meldung entfernt. Das SDK schickt ihn
// zwar im Header und nicht in der URL, aber Fehlermeldungen fremder
// Bibliotheken kontrollieren wir nicht.
func uebersetzeFehler(err error, schluessel string) error {
	if err == nil {
		return nil
	}
	text := konfig.Maskiere(err.Error(), schluessel)

	var api genai.APIError
	if asFehler(err, &api) {
		switch {
		case api.Code == 408, api.Code == 429, api.Code >= 500:
			// Zeitüberschreitung, Limit erreicht oder Störung bei Google:
			// Später noch einmal versuchen kann helfen.
			return fmt.Errorf("Gemini HTTP %d: %s: %w", api.Code, text, modell.ErrNichtErreichbar)
		default:
			// 400, 401, 403, 404: Unsere Anfrage oder Konfiguration ist falsch
			// (z. B. ungültiger Schlüssel, unbekanntes Modell). Das ist ein
			// interner Fehler, kein Problem des Dokuments.
			return fmt.Errorf("Gemini HTTP %d: %s", api.Code, text)
		}
	}

	if errors.Is(err, context.Canceled) {
		// Die App hat die Verbindung abgebrochen. Kein Modellproblem.
		return fmt.Errorf("Gemini-Aufruf abgebrochen: %w", context.Canceled)
	}
	var netz net.Error
	if errors.Is(err, context.DeadlineExceeded) || asFehler(err, &netz) {
		return fmt.Errorf("Gemini: %s: %w", text, modell.ErrNichtErreichbar)
	}
	return fmt.Errorf("Gemini: %s", text)
}

// asFehler ist errors.As, aber als Funktion mit eigenem Namen, damit die
// Aufrufe oben lesbar bleiben.
func asFehler[T any](err error, ziel *T) bool {
	return errors.As(err, ziel)
}
