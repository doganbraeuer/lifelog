package api

import (
	"time"

	"lifelog/backend/internal/core"
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

func eintragKurzZuJSON(e core.EntrySummary) eintragKurzJSON {
	return eintragKurzJSON{
		ID:            e.ID,
		Modul:         e.Module,
		Typ:           e.Type,
		Dokumentdatum: e.DocumentDate,
		Importdatum:   zeitpunktJSON(e.ImportedAt),
		Quelle:        e.Issuer,
	}
}

// eintragJSON ist der vollständige Eintrag (API.md, Abschnitt 2).
type eintragJSON struct {
	ID            string          `json:"id"`
	Modul         string          `json:"modul"`
	Typ           string          `json:"typ"`
	Dokumentdatum *string         `json:"dokumentdatum"`
	Importdatum   string          `json:"importdatum"`
	Quelle        *string         `json:"quelle"`
	Volltext      string          `json:"volltext"`
	Infos         []infoJSON      `json:"infos"`
	Datei         dateiJSON       `json:"datei"`
	Korrekturen   []korrekturJSON `json:"korrekturen"`
}

type infoJSON struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Wert    string  `json:"wert"`
	Einheit *string `json:"einheit,omitempty"` // fehlt, wenn das Dokument keine nennt
}

type dateiJSON struct {
	ID        string `json:"id"`
	Dateiname string `json:"dateiname"`
	MimeTyp   string `json:"mimeTyp"`
	Groesse   int64  `json:"groesse"`
	Sha256    string `json:"sha256"`
}

type korrekturJSON struct {
	Feld       string  `json:"feld"`
	Erkannt    *string `json:"erkannt"`
	Korrigiert *string `json:"korrigiert"`
	Zeitpunkt  string  `json:"zeitpunkt"`
}

func eintragZuJSON(e core.Entry) eintragJSON {
	infos := make([]infoJSON, 0, len(e.Facts)) // nie nil
	for _, i := range e.Facts {
		infos = append(infos, infoJSON{ID: i.ID, Name: i.Name, Wert: i.Value, Einheit: i.Unit})
	}
	korrekturen := make([]korrekturJSON, 0, len(e.Corrections)) // nie nil
	for _, k := range e.Corrections {
		korrekturen = append(korrekturen, korrekturJSON{
			Feld: k.Field, Erkannt: k.Recognized, Korrigiert: k.Corrected, Zeitpunkt: zeitpunktJSON(k.CorrectedAt),
		})
	}
	return eintragJSON{
		ID:            e.ID,
		Modul:         e.Module,
		Typ:           e.Type,
		Dokumentdatum: e.DocumentDate,
		Importdatum:   zeitpunktJSON(e.ImportedAt),
		Quelle:        e.Issuer,
		Volltext:      e.FullText,
		Infos:         infos,
		Datei: dateiJSON{
			ID: e.File.ID, Dateiname: e.File.FileName, MimeTyp: e.File.MimeType,
			Groesse: e.File.Size, Sha256: e.File.Sha256,
		},
		Korrekturen: korrekturen,
	}
}

func eintraegeListeZuJSON(liste []core.EntrySummary) eintraegeListeJSON {
	out := make([]eintragKurzJSON, 0, len(liste)) // nie nil
	for _, e := range liste {
		out = append(out, eintragKurzZuJSON(e))
	}
	return eintraegeListeJSON{Eintraege: out, Anzahl: len(out)}
}
