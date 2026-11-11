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

// Error codes for single entries (API.md 3.3, 3.4).
const (
	CodeInvalidID     = "UNGUELTIGE_ID"
	CodeEntryNotFound = "EINTRAG_NICHT_GEFUNDEN"
)

// entryHandler answers GET /api/v1/eintraege/{id} (API.md 3.3).
func entryHandler(s EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e, ok := loadEntry(w, r, s)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, entryToJSON(e))
	}
}

// fileHandler answers GET /api/v1/eintraege/{id}/datei (API.md 3.4): the
// original file as binary data. Errors still come as JSON.
func fileHandler(s EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e, ok := loadEntry(w, r, s)
		if !ok {
			return
		}

		content, length, err := s.OpenFile(r.Context(), e.File.ID)
		if err != nil {
			if errors.Is(err, core.ErrOriginalMissing) {
				// Must not happen: an entry without a file is a data error.
				slog.Error("original file missing", "entry", e.ID, "file", e.File.ID)
			}
			writeInternalError(w, r, err)
			return
		}
		defer content.Close()

		h := w.Header()
		h.Set("Content-Type", e.File.MimeType)
		h.Set("Content-Length", strconv.FormatInt(length, 10))
		h.Set("Content-Disposition", inlineWithName(e.File.FileName))
		// The browser/WebView must not guess the type itself.
		h.Set("X-Content-Type-Options", "nosniff")
		// Do not keep health documents in caches.
		h.Set("Cache-Control", "private, no-store")
		w.WriteHeader(http.StatusOK)

		// From here on the status is sent. If the transfer breaks off (e.g.
		// app closed), only the log remains.
		if _, err := io.Copy(w, content); err != nil {
			slog.Warn("file not transferred completely", "entry", e.ID, "error", err)
		}
	}
}

// loadEntry checks the ID from the path and loads the entry. On an error
// the response is already written and ok is false.
func loadEntry(w http.ResponseWriter, r *http.Request, s EntryStore) (core.Entry, bool) {
	id := r.PathValue("id")
	if err := core.CheckID(id); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidID, "Diese Eintrag-ID ist ungültig.", nil)
		return core.Entry{}, false
	}
	e, found, err := s.GetEntry(r.Context(), id)
	if err != nil {
		writeInternalError(w, r, err)
		return core.Entry{}, false
	}
	if !found {
		writeError(w, http.StatusNotFound, CodeEntryNotFound, "Diesen Eintrag gibt es nicht.", nil)
		return core.Entry{}, false
	}
	return e, true
}

// inlineWithName builds the Content-Disposition header.
// mime.FormatMediaType adds quotes and encodes umlauts per RFC 2231
// (parameter filename* with charset utf-8 and percent encoding), so no file
// name can break the header.
func inlineWithName(fileName string) string {
	if value := mime.FormatMediaType("inline", map[string]string{"filename": fileName}); value != "" {
		return value
	}
	return "inline"
}
