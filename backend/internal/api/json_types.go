package api

import (
	"time"

	"lifelog/backend/internal/core"
)

// The JSON types of the API. They are only built through the toJSON
// functions. These always turn nil lists into empty lists, because Go would
// write a nil slice as null, and the API promises [] (API.md, section 1).

type entrySummaryJSON struct {
	ID           string  `json:"id"`
	Module       string  `json:"modul"`
	Type         string  `json:"typ"`
	DocumentDate *string `json:"dokumentdatum"` // without omitempty: null stays visible
	ImportedAt   string  `json:"importdatum"`
	Issuer       *string `json:"quelle"`
}

type entryListJSON struct {
	Entries []entrySummaryJSON `json:"eintraege"`
	Count   int                `json:"anzahl"`
}

// timeJSON formats a time as RFC 3339 in UTC, e.g. "2026-11-03T18:42:07Z"
// (API.md, section "Dates and times").
func timeJSON(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func summaryToJSON(e core.EntrySummary) entrySummaryJSON {
	return entrySummaryJSON{
		ID:           e.ID,
		Module:       e.Module,
		Type:         e.Type,
		DocumentDate: e.DocumentDate,
		ImportedAt:   timeJSON(e.ImportedAt),
		Issuer:       e.Issuer,
	}
}

// entryJSON is the full entry (API.md, section 2).
type entryJSON struct {
	ID           string           `json:"id"`
	Module       string           `json:"modul"`
	Type         string           `json:"typ"`
	DocumentDate *string          `json:"dokumentdatum"`
	ImportedAt   string           `json:"importdatum"`
	Issuer       *string          `json:"quelle"`
	FullText     string           `json:"volltext"`
	Facts        []factJSON       `json:"infos"`
	File         fileJSON         `json:"datei"`
	Corrections  []correctionJSON `json:"korrekturen"`
}

type factJSON struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value string  `json:"wert"`
	Unit  *string `json:"einheit,omitempty"` // missing if the document names none
}

type fileJSON struct {
	ID       string `json:"id"`
	FileName string `json:"dateiname"`
	MimeType string `json:"mimeTyp"`
	Size     int64  `json:"groesse"`
	Sha256   string `json:"sha256"`
}

type correctionJSON struct {
	Field       string  `json:"feld"`
	Recognized  *string `json:"erkannt"`
	Corrected   *string `json:"korrigiert"`
	CorrectedAt string  `json:"zeitpunkt"`
}

func entryToJSON(e core.Entry) entryJSON {
	facts := make([]factJSON, 0, len(e.Facts)) // never nil
	for _, f := range e.Facts {
		facts = append(facts, factJSON{ID: f.ID, Name: f.Name, Value: f.Value, Unit: f.Unit})
	}
	corrections := make([]correctionJSON, 0, len(e.Corrections)) // never nil
	for _, c := range e.Corrections {
		corrections = append(corrections, correctionJSON{
			Field: c.Field, Recognized: c.Recognized, Corrected: c.Corrected, CorrectedAt: timeJSON(c.CorrectedAt),
		})
	}
	return entryJSON{
		ID:           e.ID,
		Module:       e.Module,
		Type:         e.Type,
		DocumentDate: e.DocumentDate,
		ImportedAt:   timeJSON(e.ImportedAt),
		Issuer:       e.Issuer,
		FullText:     e.FullText,
		Facts:        facts,
		File: fileJSON{
			ID: e.File.ID, FileName: e.File.FileName, MimeType: e.File.MimeType,
			Size: e.File.Size, Sha256: e.File.Sha256,
		},
		Corrections: corrections,
	}
}

func entryListToJSON(list []core.EntrySummary) entryListJSON {
	out := make([]entrySummaryJSON, 0, len(list)) // never nil
	for _, e := range list {
		out = append(out, summaryToJSON(e))
	}
	return entryListJSON{Entries: out, Count: len(out)}
}
