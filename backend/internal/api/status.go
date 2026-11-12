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
	Speech   string `json:"speech"` // "ok", "unavailable", "off" (since 2026-11-12)
}

// statusHandler answers GET /api/v1/status (docs/API.md, 3.8). The HTTP
// status depends only on the database; speech only tells the app whether
// to offer the microphone. Database and speech are asked at the same time.
func statusHandler(db Pinger, speech Transcriber) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), statusTimeout)
		defer cancel()

		speechState := make(chan string, 1)
		go func() { speechState <- speechStatus(ctx, speech) }()

		resp := statusResponse{Status: "ok", Database: "ok"}
		status := http.StatusOK
		if err := db.Ping(ctx); err != nil {
			slog.Warn("database unreachable", "error", err)
			resp.Status, resp.Database = "error", "unreachable"
			status = http.StatusServiceUnavailable
		}
		resp.Speech = <-speechState
		writeJSON(w, status, resp)
	}
}

func speechStatus(ctx context.Context, t Transcriber) string {
	if t == nil {
		return "off"
	}
	if err := t.Ping(ctx); err != nil {
		slog.Warn("speech recognition unavailable", "error", err)
		return "unavailable"
	}
	return "ok"
}
