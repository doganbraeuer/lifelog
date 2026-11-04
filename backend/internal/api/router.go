package api

import "net/http"

// Abhaengigkeiten sind alles, was die Handler von außen brauchen. Mit jedem
// Schritt kommen Felder dazu (Speicher, Dienste). Über Schnittstellen statt
// konkreter Typen lassen sich die Handler ohne Datenbank testen.
type Abhaengigkeiten struct {
	DB        Pinger
	Eintraege EintragSpeicher
	Hochladen Hochlader
}

// NeuerRouter verbindet die Pfade aus docs/API.md mit den Handlern.
//
// Seit Go 1.22 versteht http.ServeMux Methode und Platzhalter im Muster,
// z. B. "GET /api/v1/eintraege/{id}". Ein Framework brauchen wir nicht.
func NeuerRouter(abh Abhaengigkeiten) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", statusHandler(abh.DB))
	mux.HandleFunc("GET /api/v1/eintraege", eintraegeListeHandler(abh.Eintraege))
	mux.HandleFunc("POST /api/v1/eintraege", hochladenHandler(abh.Hochladen))
	mux.HandleFunc("GET /api/v1/eintraege/{id}", eintragHandler(abh.Eintraege))
	mux.HandleFunc("GET /api/v1/eintraege/{id}/datei", dateiHandler(abh.Eintraege))

	// Reihenfolge: Log ganz außen, damit auch abgefangene panics mit
	// Status 500 im Log erscheinen.
	return mitLog(mitAbfangen(mux))
}
