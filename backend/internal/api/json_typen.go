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

func eintragZuJSON(e kern.Eintrag) eintragJSON {
	infos := make([]infoJSON, 0, len(e.Infos)) // nie nil
	for _, i := range e.Infos {
		infos = append(infos, infoJSON{ID: i.ID, Name: i.Name, Wert: i.Wert, Einheit: i.Einheit})
	}
	korrekturen := make([]korrekturJSON, 0, len(e.Korrekturen)) // nie nil
	for _, k := range e.Korrekturen {
		korrekturen = append(korrekturen, korrekturJSON{
			Feld: k.Feld, Erkannt: k.Erkannt, Korrigiert: k.Korrigiert, Zeitpunkt: zeitpunktJSON(k.Zeitpunkt),
		})
	}
	return eintragJSON{
		ID:            e.ID,
		Modul:         e.Modul,
		Typ:           e.Typ,
		Dokumentdatum: e.Dokumentdatum,
		Importdatum:   zeitpunktJSON(e.Importdatum),
		Quelle:        e.Quelle,
		Volltext:      e.Volltext,
		Infos:         infos,
		Datei: dateiJSON{
			ID: e.Datei.ID, Dateiname: e.Datei.Dateiname, MimeTyp: e.Datei.MimeTyp,
			Groesse: e.Datei.Groesse, Sha256: e.Datei.Sha256,
		},
		Korrekturen: korrekturen,
	}
}

func eintraegeListeZuJSON(liste []kern.EintragKurz) eintraegeListeJSON {
	out := make([]eintragKurzJSON, 0, len(liste)) // nie nil
	for _, e := range liste {
		out = append(out, eintragKurzZuJSON(e))
	}
	return eintraegeListeJSON{Eintraege: out, Anzahl: len(out)}
}
