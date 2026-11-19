package api

import (
	"context"
	"errors"
	"net/http"

	"lifelog/backend/internal/core"
)

// Deleter deletes an entry of the user. Implemented by service.Deleter.
type Deleter interface {
	Delete(ctx context.Context, userID, id string) error
}

// deleteHandler answers DELETE /api/v1/entries/{id} (API.md 3.7): 204
// without a body. An entry of another user is 404, and nothing is deleted.
func deleteHandler(d Deleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		err := d.Delete(r.Context(), user.ID, r.PathValue("id"))
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, core.ErrInvalidID):
			writeError(w, http.StatusBadRequest, CodeInvalidID, "Diese Eintrag-ID ist ungültig.", nil)
		case errors.Is(err, core.ErrEntryNotFound):
			writeError(w, http.StatusNotFound, CodeEntryNotFound, "Diesen Eintrag gibt es nicht.", nil)
		case errors.Is(err, context.Canceled):
			// The app gave up before deleting; nobody reads the response.
		default:
			writeInternalError(w, r, err)
		}
	}
}
