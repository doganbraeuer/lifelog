package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger prüft, ob die Datenbank erreichbar ist. Im Programm steckt dahinter
// der MongoDB-Client, in Tests eine Attrappe.
type Pinger interface {
	Ping(ctx context.Context) error
}

// statusTimeout begrenzt, wie lange der Health-Check auf MongoDB wartet.
const statusTimeout = 3 * time.Second

type statusAntwort struct {
	Status    string `json:"status"`
	Datenbank string `json:"datenbank"`
}

// statusHandler beantwortet GET /api/v1/status (docs/API.md, 3.8).
func statusHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, abbrechen := context.WithTimeout(r.Context(), statusTimeout)
		defer abbrechen()

		if err := db.Ping(ctx); err != nil {
			slog.Warn("Datenbank nicht erreichbar", "fehler", err)
			schreibeJSON(w, http.StatusServiceUnavailable,
				statusAntwort{Status: "fehler", Datenbank: "nicht erreichbar"})
			return
		}
		schreibeJSON(w, http.StatusOK, statusAntwort{Status: "ok", Datenbank: "ok"})
	}
}
