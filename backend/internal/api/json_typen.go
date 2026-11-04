package api

import (
	"time"

	"lifelog/backend/internal/kern"
)

// Die JSON-Typen der API. Sie werden nur über die zuJSON-Funktionen gebaut.
// Diese machen aus nil-Listen immer leere Listen, denn Go würde eine
// nil-Slice als null ausgeben, und die API verspricht [] (API.md, Abschnitt 1).

type eintragKurzJSON struct {
	ID            string  `json:"id"`
	Modul         string  `json:"modul"`
	Typ           string  `json:"typ"`
	Dokumentdatum *string `json:"dokumentdatum"` // ohne omitempty: null bleibt sichtbar
	Importdatum   string  `json:"importdatum"`
	Quelle        *string `json:"quelle"`
}

type eintraegeListeJSON struct {
	Eintraege []eintragKurzJSON `json:"eintraege"`
	Anzahl    int               `json:"anzahl"`
}

// zeitpunktJSON formatiert einen Zeitpunkt als RFC 3339 in UTC, z. B.
// "2026-11-03T18:42:07Z" (API.md, Abschnitt „Datum und Zeit“).
func zeitpunktJSON(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func eintragKurzZuJSON(e kern.EintragKurz) eintragKurzJSON {
	return eintragKurzJSON{
		ID:            e.ID,
		Modul:         e.Modul,
		Typ:           e.Typ,
		Dokumentdatum: e.Dokumentdatum,
		Importdatum:   zeitpunktJSON(e.Importdatum),
		Quelle:        e.Quelle,
	}
}

func eintraegeListeZuJSON(liste []kern.EintragKurz) eintraegeListeJSON {
	out := make([]eintragKurzJSON, 0, len(liste)) // nie nil
	for _, e := range liste {
		out = append(out, eintragKurzZuJSON(e))
	}
	return eintraegeListeJSON{Eintraege: out, Anzahl: len(out)}
}
