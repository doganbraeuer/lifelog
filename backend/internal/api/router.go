package api

import "net/http"

// Dependencies is everything the handlers need from outside. Through
// interfaces instead of concrete types, the handlers can be tested without a
// database.
type Dependencies struct {
	DB        Pinger
	Entries   EntryStore
	Uploader  Uploader
	Asker     Asker
	Corrector Corrector
	Deleter   Deleter
	// Transcriber: nil means speech recognition is switched off
	// (SPEECH_ENGINE=none); /status then says "off".
	Transcriber Transcriber
}

// NewRouter connects the paths from docs/API.md with the handlers.
//
// Since Go 1.22, http.ServeMux understands the method and placeholders in
// the pattern, e.g. "GET /api/v1/entries/{id}". We do not need a
// framework.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", statusHandler(deps.DB, deps.Transcriber))
	mux.HandleFunc("GET /api/v1/entries", listEntriesHandler(deps.Entries))
	mux.HandleFunc("POST /api/v1/entries", uploadHandler(deps.Uploader))
	mux.HandleFunc("GET /api/v1/entries/{id}", entryHandler(deps.Entries))
	mux.HandleFunc("PATCH /api/v1/entries/{id}", correctHandler(deps.Corrector))
	mux.HandleFunc("DELETE /api/v1/entries/{id}", deleteHandler(deps.Deleter))
	mux.HandleFunc("GET /api/v1/entries/{id}/file", fileHandler(deps.Entries))
	mux.HandleFunc("POST /api/v1/questions", questionsHandler(deps.Asker))
	mux.HandleFunc("POST /api/v1/transcriptions", transcribeHandler(deps.Transcriber))

	// Order: logging outermost, so recovered panics also appear in the log
	// with status 500.
	return withLogging(withRecover(mux))
}
