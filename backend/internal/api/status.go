package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger checks whether the database is reachable. In the program the
// MongoDB client is behind it, in tests a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// statusTimeout limits how long the health check waits for MongoDB.
const statusTimeout = 3 * time.Second

type statusResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// statusHandler answers GET /api/v1/status (docs/API.md, 3.8).
func statusHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), statusTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			slog.Warn("database unreachable", "error", err)
			writeJSON(w, http.StatusServiceUnavailable,
				statusResponse{Status: "error", Database: "unreachable"})
			return
		}
		writeJSON(w, http.StatusOK, statusResponse{Status: "ok", Database: "ok"})
	}
}
