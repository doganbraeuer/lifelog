package api

import (
	"context"
	"errors"
	"net/http"

	"lifelog/backend/internal/core"
)

// Loescher löscht einen Eintrag. Umgesetzt von dienst.Loeschen.
type Loescher interface {
	Ausfuehren(ctx context.Context, id string) error
}

// loeschenHandler beantwortet DELETE /api/v1/eintraege/{id} (API.md 3.7):
// 204 ohne Body.
func loeschenHandler(l Loescher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := l.Ausfuehren(r.Context(), r.PathValue("id"))
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, core.ErrInvalidID):
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeID, "Diese Eintrag-ID ist ungültig.", nil)
		case errors.Is(err, core.ErrEntryNotFound):
			schreibeFehler(w, http.StatusNotFound, CodeEintragNichtGefunden, "Diesen Eintrag gibt es nicht.", nil)
		case errors.Is(err, context.Canceled):
			// Die App hat vor dem Löschen aufgegeben; niemand liest die Antwort.
		default:
			schreibeInternenFehler(w, r, err)
		}
	}
}
