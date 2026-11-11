package api

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"lifelog/backend/internal/core"
)

// Fehlercodes für einzelne Einträge (API.md 3.3, 3.4).
const (
	CodeUngueltigeID         = "UNGUELTIGE_ID"
	CodeEintragNichtGefunden = "EINTRAG_NICHT_GEFUNDEN"
)

// eintragHandler beantwortet GET /api/v1/eintraege/{id} (API.md 3.3).
func eintragHandler(s EintragSpeicher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e, ok := ladeEintrag(w, r, s)
		if !ok {
			return
		}
		schreibeJSON(w, http.StatusOK, eintragZuJSON(e))
	}
}

// dateiHandler beantwortet GET /api/v1/eintraege/{id}/datei (API.md 3.4):
// die Originaldatei als Binärdaten. Fehler kommen trotzdem als JSON.
func dateiHandler(s EintragSpeicher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e, ok := ladeEintrag(w, r, s)
		if !ok {
			return
		}

		inhalt, laenge, err := s.DateiOeffnen(r.Context(), e.Datei.ID)
		if err != nil {
			if errors.Is(err, core.ErrOriginalFehlt) {
				// Darf nicht vorkommen: Eintrag ohne Datei ist ein Datenfehler.
				slog.Error("Originaldatei fehlt", "eintrag", e.ID, "datei", e.Datei.ID)
			}
			schreibeInternenFehler(w, r, err)
			return
		}
		defer inhalt.Close()

		h := w.Header()
		h.Set("Content-Type", e.Datei.MimeTyp)
		h.Set("Content-Length", strconv.FormatInt(laenge, 10))
		h.Set("Content-Disposition", inlineMitName(e.Datei.Dateiname))
		// Der Browser/die WebView soll den Typ nicht selbst erraten.
		h.Set("X-Content-Type-Options", "nosniff")
		// Gesundheitsdokumente nicht in Zwischenspeichern ablegen.
		h.Set("Cache-Control", "private, no-store")
		w.WriteHeader(http.StatusOK)

		// Ab hier ist der Status gesendet. Bricht die Übertragung ab (z. B.
		// App geschlossen), bleibt nur das Log.
		if _, err := io.Copy(w, inhalt); err != nil {
			slog.Warn("Datei nicht vollständig übertragen", "eintrag", e.ID, "fehler", err)
		}
	}
}

// ladeEintrag prüft die ID aus dem Pfad und lädt den Eintrag. Bei einem
// Fehler ist die Antwort schon geschrieben und ok ist false.
func ladeEintrag(w http.ResponseWriter, r *http.Request, s EintragSpeicher) (core.Eintrag, bool) {
	id := r.PathValue("id")
	if err := core.PruefeID(id); err != nil {
		schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeID, "Diese Eintrag-ID ist ungültig.", nil)
		return core.Eintrag{}, false
	}
	e, gefunden, err := s.EintragLesen(r.Context(), id)
	if err != nil {
		schreibeInternenFehler(w, r, err)
		return core.Eintrag{}, false
	}
	if !gefunden {
		schreibeFehler(w, http.StatusNotFound, CodeEintragNichtGefunden, "Diesen Eintrag gibt es nicht.", nil)
		return core.Eintrag{}, false
	}
	return e, true
}

// inlineMitName baut den Content-Disposition-Header. mime.FormatMediaType
// setzt Anführungszeichen und kodiert Umlaute nach RFC 2231 (Parameter
// filename* mit Zeichensatz utf-8 und Prozent-Kodierung), damit kein
// Dateiname den Header zerbrechen kann.
func inlineMitName(dateiname string) string {
	if wert := mime.FormatMediaType("inline", map[string]string{"filename": dateiname}); wert != "" {
		return wert
	}
	return "inline"
}
