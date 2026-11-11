package api

import (
	"context"
	"io"
	"net/http"

	"lifelog/backend/internal/core"
)

// Fehlercodes für die Einträge-Endpunkte (API.md, Abschnitt 1).
const (
	CodeUngueltigerParameter = "UNGUELTIGER_PARAMETER"
)

// EintragSpeicher ist das, was die Einträge-Handler von der Datenbank
// brauchen. Umgesetzt von speicher.Speicher, im Test von einer Attrappe.
type EintragSpeicher interface {
	ListeKurz(ctx context.Context, modul string) ([]core.EntrySummary, error)
	EintragLesen(ctx context.Context, id string) (e core.Entry, gefunden bool, err error)
	DateiOeffnen(ctx context.Context, dateiID string) (inhalt io.ReadCloser, laenge int64, err error)
}

// eintraegeListeHandler beantwortet GET /api/v1/eintraege (API.md, 3.2).
func eintraegeListeHandler(speicher EintragSpeicher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		modul, err := core.CheckModule(q.Get("modul"))
		if err != nil {
			schreibeUngueltigenParameter(w, "modul", q.Get("modul"),
				"Unbekanntes Modul. Im Prototyp gibt es nur gesundheit.")
			return
		}
		sortierung, err := core.ParseSort(q.Get("sortierung"))
		if err != nil {
			schreibeUngueltigenParameter(w, "sortierung", q.Get("sortierung"),
				"Unbekannte Sortierung. Erlaubt sind dokumentdatum und importdatum.")
			return
		}
		richtung, err := core.ParseSortOrder(q.Get("richtung"))
		if err != nil {
			schreibeUngueltigenParameter(w, "richtung", q.Get("richtung"),
				"Unbekannte Richtung. Erlaubt sind absteigend und aufsteigend.")
			return
		}

		liste, err := speicher.ListeKurz(r.Context(), modul)
		if err != nil {
			schreibeInternenFehler(w, r, err)
			return
		}
		core.SortEntries(liste, sortierung, richtung)

		schreibeJSON(w, http.StatusOK, eintraegeListeZuJSON(liste))
	}
}

func schreibeUngueltigenParameter(w http.ResponseWriter, parameter, wert, nachricht string) {
	schreibeFehler(w, http.StatusBadRequest, CodeUngueltigerParameter, nachricht,
		map[string]any{"parameter": parameter, "wert": wert})
}
