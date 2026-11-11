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
}

// NewRouter connects the paths from docs/API.md with the handlers.
//
// Since Go 1.22, http.ServeMux understands the method and placeholders in
// the pattern, e.g. "GET /api/v1/eintraege/{id}". We do not need a
// framework.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", statusHandler(deps.DB))
	mux.HandleFunc("GET /api/v1/eintraege", listEntriesHandler(deps.Entries))
	mux.HandleFunc("POST /api/v1/eintraege", uploadHandler(deps.Uploader))
	mux.HandleFunc("GET /api/v1/eintraege/{id}", entryHandler(deps.Entries))
	mux.HandleFunc("PATCH /api/v1/eintraege/{id}", correctHandler(deps.Corrector))
	mux.HandleFunc("DELETE /api/v1/eintraege/{id}", deleteHandler(deps.Deleter))
	mux.HandleFunc("GET /api/v1/eintraege/{id}/datei", fileHandler(deps.Entries))
	mux.HandleFunc("POST /api/v1/fragen", questionsHandler(deps.Asker))

	// Order: logging outermost, so recovered panics also appear in the log
	// with status 500.
	return withLogging(withRecover(mux))
}
