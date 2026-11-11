package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// Fehlercodes für Fragen (API.md 3.6).
const (
	CodeFrageFehlt  = "FRAGE_FEHLT"
	CodeFrageZuLang = "FRAGE_ZU_LANG"
)

// maxFrageBody: Eine Frage hat höchstens 500 Zeichen (≤ 2000 Bytes); 64 KiB
// lassen viel Luft und begrenzen trotzdem, was der Server liest.
const maxFrageBody = 64 << 10

// Frager beantwortet Fragen. Umgesetzt von dienst.Fragen.
type Frager interface {
	Ausfuehren(ctx context.Context, frage, modul string) (core.Answer, error)
}

type frageAnfrage struct {
	Frage string `json:"frage"`
	Modul string `json:"modul"`
}

type antwortJSON struct {
	Antwort  string       `json:"antwort"`
	Gefunden bool         `json:"gefunden"`
	Quellen  []quelleJSON `json:"quellen"`
}

type quelleJSON struct {
	Nr            int     `json:"nr"`
	EintragID     string  `json:"eintragId"`
	Modul         string  `json:"modul"`
	Typ           string  `json:"typ"`
	Quelle        *string `json:"quelle"`
	Dokumentdatum *string `json:"dokumentdatum"`
	Auszug        string  `json:"auszug"`
}

func antwortZuJSON(a core.Answer) antwortJSON {
	quellen := make([]quelleJSON, 0, len(a.Sources)) // nie nil
	for _, q := range a.Sources {
		quellen = append(quellen, quelleJSON{
			Nr: q.Number, EintragID: q.EntryID, Modul: q.Module, Typ: q.Type,
			Quelle: q.Issuer, Dokumentdatum: q.DocumentDate, Auszug: q.Excerpt,
		})
	}
	return antwortJSON{Antwort: a.Text, Gefunden: a.Found, Quellen: quellen}
}

// fragenHandler beantwortet POST /api/v1/fragen (API.md 3.6).
func fragenHandler(f Frager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var anfrage frageAnfrage
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFrageBody))
		// Unbekannte Felder sind ein Fehler (API.md 1): Ein Tippfehler wie
		// "frge" soll auffallen, statt still als "Frage fehlt" zu enden.
		dec.DisallowUnknownFields()
		if err := dec.Decode(&anfrage); err != nil || dec.More() {
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
				"Bitte die Frage als JSON schicken, z. B. {\"frage\": \"…\"}.", nil)
			return
		}

		antwort, err := f.Ausfuehren(r.Context(), anfrage.Frage, anfrage.Modul)
		if err != nil {
			schreibeFragenFehler(w, r, err, anfrage.Modul)
			return
		}
		schreibeJSON(w, http.StatusOK, antwortZuJSON(antwort))
	}
}

func schreibeFragenFehler(w http.ResponseWriter, r *http.Request, err error, modul string) {
	switch {
	case errors.Is(err, core.ErrQuestionMissing):
		schreibeFehler(w, http.StatusBadRequest, CodeFrageFehlt, "Bitte stelle eine Frage.", nil)
	case errors.Is(err, core.ErrQuestionTooLong):
		schreibeFehler(w, http.StatusBadRequest, CodeFrageZuLang,
			"Die Frage ist zu lang. Erlaubt sind höchstens 500 Zeichen.", nil)
	case errors.Is(err, core.ErrUnknownModule):
		schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
			"Unbekanntes Modul. Im Prototyp gibt es nur gesundheit.",
			map[string]any{"feld": "modul", "wert": modul})
	case errors.Is(err, llm.ErrUnavailable), errors.Is(err, llm.ErrInvalidResponse):
		slog.Warn("Frage: Modell nicht erreichbar oder Antwort unbrauchbar", "fehler", err)
		schreibeFehler(w, http.StatusServiceUnavailable, CodeModellNichtErreichbar,
			"Die Suche ist gerade nicht erreichbar. Bitte versuche es später noch einmal.", nil)
	case errors.Is(err, context.Canceled):
		slog.Info("Frage abgebrochen")
	default:
		schreibeInternenFehler(w, r, err)
	}
}
