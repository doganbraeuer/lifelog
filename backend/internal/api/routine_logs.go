package api

import (
	"net/http"

	"lifelog/backend/internal/core"
)

// routineLogsHandler answers GET /api/v1/routines/{id}/logs (API.md 3.25).
func routineLogsHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !checkRoutineID(w, id) {
			return
		}
		q := r.URL.Query()
		list, err := s.Logs(r.Context(), user, id, q.Get("from"), q.Get("to"))
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		out := routineLogListJSON{Logs: make([]routineLogJSON, 0, len(list)), Count: len(list)}
		for _, l := range list {
			out.Logs = append(out.Logs, routineLogToJSON(l))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// addRoutineLogHandler answers POST /api/v1/routines/{id}/logs (API.md
// 3.26): 201 with the log; there is no GET for one log, so no Location.
func addRoutineLogHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !checkRoutineID(w, id) {
			return
		}
		body, ok := readRoutineBody(w, r, false)
		if !ok {
			return
		}
		l, err := s.AddLog(r.Context(), user, id, body)
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, routineLogToJSON(l))
	}
}

// deleteRoutineLogHandler answers DELETE
// /api/v1/routines/{id}/logs/{logId} (API.md 3.27).
func deleteRoutineLogHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, logID := r.PathValue("id"), r.PathValue("logId")
		if !checkRoutineID(w, id) {
			return
		}
		if core.CheckID(logID) != nil {
			writeError(w, http.StatusBadRequest, CodeInvalidID, messageInvalidLogID, nil)
			return
		}
		if err := s.DeleteLog(r.Context(), user, id, logID); err != nil {
			writeRoutineError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
