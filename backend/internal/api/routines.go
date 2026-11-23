package api

import (
	"context"
	"net/http"

	"lifelog/backend/internal/core"
)

// Routines is the routine endpoints (API.md 3.17–3.27). Implemented by
// service.Routines. Every method gets the logged-in user and only sees the
// user's own routines and logs. The bodies are passed on as encoding/json
// decodes them (nil: not a JSON object); core checks them.
type Routines interface {
	List(ctx context.Context, user core.User, module string) ([]core.RoutineView, error)
	Create(ctx context.Context, user core.User, body map[string]any) (core.RoutineView, error)
	Get(ctx context.Context, user core.User, id string) (core.RoutineView, error)
	Update(ctx context.Context, user core.User, id string, body map[string]any) (core.RoutineView, error)
	Pause(ctx context.Context, user core.User, id string, body map[string]any) (core.RoutineView, error)
	Resume(ctx context.Context, user core.User, id string) (core.RoutineView, error)
	Delete(ctx context.Context, user core.User, id string) error
	Occurrences(ctx context.Context, user core.User, module, from, to string) (core.Date, core.Date, []core.Occurrence, error)
	Logs(ctx context.Context, user core.User, id, from, to string) ([]core.RoutineLog, error)
	AddLog(ctx context.Context, user core.User, id string, body map[string]any) (core.RoutineLog, error)
	DeleteLog(ctx context.Context, user core.User, id, logID string) error
}

// listRoutinesHandler answers GET /api/v1/routines (API.md 3.17).
func listRoutinesHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		list, err := s.List(r.Context(), user, r.URL.Query().Get("module"))
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		out := routineListJSON{Routines: make([]routineJSON, 0, len(list)), Count: len(list)}
		for _, v := range list {
			out.Routines = append(out.Routines, routineToJSON(v))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// createRoutineHandler answers POST /api/v1/routines (API.md 3.18): 201
// with the routine and its Location.
func createRoutineHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		body, ok := readRoutineBody(w, r, false)
		if !ok {
			return
		}
		v, err := s.Create(r.Context(), user, body)
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		w.Header().Set("Location", "/api/v1/routines/"+v.ID)
		writeJSON(w, http.StatusCreated, routineToJSON(v))
	}
}

// routineHandler answers GET /api/v1/routines/{id} (API.md 3.19).
func routineHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !checkRoutineID(w, id) {
			return
		}
		v, err := s.Get(r.Context(), user, id)
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, routineToJSON(v))
	}
}

// updateRoutineHandler answers PATCH /api/v1/routines/{id} (API.md 3.20).
func updateRoutineHandler(s Routines) http.HandlerFunc {
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
		v, err := s.Update(r.Context(), user, id, body)
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, routineToJSON(v))
	}
}

// pauseRoutineHandler answers POST /api/v1/routines/{id}/pause (API.md
// 3.21). An empty body counts as {}: paused until resumed.
func pauseRoutineHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !checkRoutineID(w, id) {
			return
		}
		body, ok := readRoutineBody(w, r, true)
		if !ok {
			return
		}
		v, err := s.Pause(r.Context(), user, id, body)
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, routineToJSON(v))
	}
}

// resumeRoutineHandler answers POST /api/v1/routines/{id}/resume (API.md
// 3.22). A body is ignored.
func resumeRoutineHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !checkRoutineID(w, id) {
			return
		}
		v, err := s.Resume(r.Context(), user, id)
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, routineToJSON(v))
	}
}

// deleteRoutineHandler answers DELETE /api/v1/routines/{id} (API.md 3.23).
func deleteRoutineHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !checkRoutineID(w, id) {
			return
		}
		if err := s.Delete(r.Context(), user, id); err != nil {
			writeRoutineError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// occurrencesHandler answers GET /api/v1/routines/occurrences (API.md
// 3.24). ServeMux prefers this fixed path to /routines/{id}.
func occurrencesHandler(s Routines) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		from, to, list, err := s.Occurrences(r.Context(), user, q.Get("module"), q.Get("from"), q.Get("to"))
		if err != nil {
			writeRoutineError(w, r, err)
			return
		}
		out := occurrenceListJSON{From: from.String(), To: to.String(),
			Occurrences: make([]occurrenceJSON, 0, len(list)), Count: len(list)}
		for _, o := range list {
			out.Occurrences = append(out.Occurrences, occurrenceToJSON(o))
		}
		writeJSON(w, http.StatusOK, out)
	}
}
