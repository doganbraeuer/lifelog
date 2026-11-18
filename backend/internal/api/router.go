package api

import (
	"net/http"
	"net/netip"
)

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

	// Accounts checks every token and serves login and /me. Required:
	// NewRouter refuses to build a router without it.
	Accounts Accounts
	// TrustedProxies (TRUSTED_PROXIES): only from these addresses is
	// X-Forwarded-For believed for the client IP (auth.ClientIP).
	TrustedProxies []netip.Prefix
}

// NewRouter connects the paths from docs/API.md with the handlers.
//
// Since Go 1.22, http.ServeMux understands the method and placeholders in
// the pattern, e.g. "GET /api/v1/entries/{id}". We do not need a
// framework.
//
// Every route needs a token except GET /status and POST /auth/login
// (withAuth, API.md section 1). Without Accounts there is no router at all:
// a server that forgot it must not start without login.
func NewRouter(deps Dependencies) http.Handler {
	if deps.Accounts == nil {
		panic("api: Dependencies.Accounts is required; the API never runs without login")
	}
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
	mux.HandleFunc("POST /api/v1/auth/login", loginHandler(deps))
	mux.HandleFunc("POST /api/v1/auth/logout", logoutHandler(deps.Accounts))
	mux.HandleFunc("GET /api/v1/me", meHandler())
	mux.HandleFunc("PATCH /api/v1/me", updateMeHandler(deps.Accounts))
	mux.HandleFunc("POST /api/v1/me/password", changePasswordHandler(deps))

	// Order: logging outermost, so recovered panics and refused requests
	// also appear in the log (method, path, status; never headers or body).
	// withAuth innermost around the mux, so it sees the routing decision.
	return withLogging(withRecover(withAuth(mux, deps.Accounts)))
}
