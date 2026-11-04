package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// statusMerker merkt sich den gesendeten HTTP-Status fürs Log.
type statusMerker struct {
	http.ResponseWriter
	status int
}

func (s *statusMerker) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusMerker) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Unwrap erlaubt http.ResponseController, an den echten Writer zu kommen.
func (s *statusMerker) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// mitLog schreibt pro Anfrage eine Zeile: Methode, Pfad, Status, Dauer.
// Bewusst NICHT geloggt: Query, Body, Dateiinhalte, Fragen. Darin können
// Gesundheitsdaten stehen.
func mitLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m := &statusMerker{ResponseWriter: w}
		next.ServeHTTP(m, r)
		slog.Info("anfrage",
			"methode", r.Method,
			"pfad", r.URL.Path,
			"status", m.status,
			"dauer_ms", time.Since(start).Milliseconds(),
		)
	})
}

// mitAbfangen fängt einen panic in einem Handler ab und antwortet mit
// 500 INTERNER_FEHLER, statt die Verbindung kommentarlos abzubrechen.
func mitAbfangen(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler { // gewollter Abbruch, weiterreichen
					panic(p)
				}
				slog.Error("panic im Handler", "stack", string(debug.Stack()))
				schreibeInternenFehler(w, r, fmt.Errorf("panic: %v", p))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
