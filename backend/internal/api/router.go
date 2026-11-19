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
	// Usage counts the daily limit of the transcriptions (uploads and
	// questions count in their services). Required, like Accounts.
	Usage Usage
	// TrustedProxies (TRUSTED_PROXIES): only from these addresses is
	// X-Forwarded-For believed for the client IP (auth.ClientIP).
	TrustedProxies []netip.Prefix
}

// route is one registered pattern with its handler.
type route struct {
	pattern string
	handler http.HandlerFunc
}

// routes lists every route of the API. NewRouter registers exactly these;
// the separation test (separation_test.go) reads the same list, so a new
// route that touches entries without passing the user on makes it fail.
func routes(deps Dependencies) []route {
	return []route{
		{"GET /api/v1/status", statusHandler(deps.DB, deps.Transcriber)},
		{"GET /api/v1/entries", listEntriesHandler(deps.Entries)},
		{"POST /api/v1/entries", uploadHandler(deps.Uploader)},
		{"GET /api/v1/entries/{id}", entryHandler(deps.Entries)},
		{"PATCH /api/v1/entries/{id}", correctHandler(deps.Corrector)},
		{"DELETE /api/v1/entries/{id}", deleteHandler(deps.Deleter)},
		{"GET /api/v1/entries/{id}/file", fileHandler(deps.Entries)},
		{"POST /api/v1/questions", questionsHandler(deps.Asker)},
		{"POST /api/v1/transcriptions", transcribeHandler(deps.Transcriber, deps.Usage)},
		{"POST /api/v1/auth/login", loginHandler(deps)},
		{"POST /api/v1/auth/logout", logoutHandler(deps.Accounts)},
		{"GET /api/v1/me", meHandler()},
		{"PATCH /api/v1/me", updateMeHandler(deps.Accounts)},
		{"POST /api/v1/me/password", changePasswordHandler(deps)},
		{"PUT /api/v1/me/consent", consentHandler(deps.Accounts)},
	}
}

// NewRouter connects the paths from docs/API.md with the handlers.
//
// Since Go 1.22, http.ServeMux understands the method and placeholders in
// the pattern, e.g. "GET /api/v1/entries/{id}". We do not need a
// framework.
//
// Every route needs a token except GET /status and POST /auth/login
// (withAuth, API.md section 1). Without Accounts and Usage there is no
// router at all: a server that forgot them must not start without login or
// without daily limits.
func NewRouter(deps Dependencies) http.Handler {
	if deps.Accounts == nil {
		panic("api: Dependencies.Accounts is required; the API never runs without login")
	}
	if deps.Usage == nil {
		panic("api: Dependencies.Usage is required; the API never runs without daily limits")
	}
	mux := http.NewServeMux()
	for _, rt := range routes(deps) {
		mux.HandleFunc(rt.pattern, rt.handler)
	}

	// Order: logging outermost, so recovered panics and refused requests
	// also appear in the log (method, path, status; never headers or body).
	// withAuth innermost around the mux, so it sees the routing decision.
	return withLogging(withRecover(withAuth(mux, deps.Accounts)))
}
