package api

import (
	"math"
	"time"

	"lifelog/backend/internal/core"
)

// The JSON types of the API. They are only built through the toJSON
// functions. These always turn nil lists into empty lists, because Go would
// write a nil slice as null, and the API promises [] (API.md, section 1).

type entrySummaryJSON struct {
	ID           string  `json:"id"`
	Module       string  `json:"module"`
	Type         string  `json:"type"`
	DocumentDate *string `json:"documentDate"` // without omitempty: null stays visible
	ImportedAt   string  `json:"importedAt"`
	Issuer       *string `json:"issuer"`
}

type entryListJSON struct {
	Entries []entrySummaryJSON `json:"entries"`
	Count   int                `json:"count"`
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
	Module       string           `json:"module"`
	Type         string           `json:"type"`
	DocumentDate *string          `json:"documentDate"`
	ImportedAt   string           `json:"importedAt"`
	Issuer       *string          `json:"issuer"`
	FullText     string           `json:"fullText"`
	Facts        []factJSON       `json:"facts"`
	File         fileJSON         `json:"file"`
	Corrections  []correctionJSON `json:"corrections"`
	Verification verificationJSON `json:"verification"`
}

// verificationJSON is entry.verification (API.md, section 2). Empty texts
// become null, so the app sees one consistent shape for every status.
type verificationJSON struct {
	Status       string           `json:"status"`
	Reason       *string          `json:"reason"`
	Source       *string          `json:"source"`
	Engine       *string          `json:"engine"`
	CheckedAt    *string          `json:"checkedAt"`
	Confirmed    int              `json:"confirmed"`
	Corrected    int              `json:"corrected"`
	Total        int              `json:"total"`
	MatchPercent *int             `json:"matchPercent"`
	Fields       []fieldCheckJSON `json:"fields"`
}

type fieldCheckJSON struct {
	Field      string         `json:"field"`
	Status     string         `json:"status"`
	Source     *string        `json:"source"`
	Confidence *int           `json:"confidence"`
	Locations  []locationJSON `json:"locations"`
}

// locationJSON is either a rectangle on a page (page, x, y, width, height)
// or a range in the full text of a text import (start, end). The fields of
// the other kind are left out (pointers with omitempty).
type locationJSON struct {
	Page   *int     `json:"page,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	Width  *float64 `json:"width,omitempty"`
	Height *float64 `json:"height,omitempty"`
	Start  *int     `json:"start,omitempty"`
	End    *int     `json:"end,omitempty"`
}

type factJSON struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value string  `json:"value"`
	Unit  *string `json:"unit,omitempty"` // missing if the document names none
}

type fileJSON struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	Sha256   string `json:"sha256"`
}

type correctionJSON struct {
	Field       string  `json:"field"`
	Recognized  *string `json:"recognized"`
	Corrected   *string `json:"corrected"`
	CorrectedAt string  `json:"correctedAt"`
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
		Corrections:  corrections,
		Verification: verificationToJSON(e.Verification),
	}
}

func verificationToJSON(v core.Verification) verificationJSON {
	if v.Status == "" {
		// Never set (cannot come from the store, which reads it as
		// not_run): show the same.
		v = core.Unchecked(core.ReasonNotRun)
	}
	out := verificationJSON{
		Status:       v.Status,
		Reason:       nullIfEmpty(v.Reason),
		Source:       nullIfEmpty(v.Source),
		Engine:       nullIfEmpty(v.Engine),
		Confirmed:    v.Confirmed,
		Corrected:    v.Corrected,
		Total:        v.Total,
		MatchPercent: v.MatchPercent(),
		Fields:       make([]fieldCheckJSON, 0, len(v.Fields)), // never nil
	}
	if v.CheckedAt != nil {
		t := timeJSON(*v.CheckedAt)
		out.CheckedAt = &t
	}
	for _, f := range v.Fields {
		fj := fieldCheckJSON{
			Field: f.Field, Status: f.Status, Source: nullIfEmpty(f.Source), Confidence: f.Confidence,
			Locations: make([]locationJSON, 0, len(f.Locations)), // never nil
		}
		for _, l := range f.Locations {
			fj.Locations = append(fj.Locations, locationToJSON(l))
		}
		out.Fields = append(out.Fields, fj)
	}
	return out
}

func locationToJSON(l core.Location) locationJSON {
	if l.Page == 0 {
		start, end := l.Start, l.End
		return locationJSON{Start: &start, End: &end}
	}
	page := l.Page
	x, y, w, h := round4(l.Box.X), round4(l.Box.Y), round4(l.Box.W), round4(l.Box.H)
	return locationJSON{Page: &page, X: &x, Y: &y, Width: &w, Height: &h}
}

// round4: four decimals are a tenth of a millimetre on an A4 page, more
// than enough for drawing a rectangle.
func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func entryListToJSON(list []core.EntrySummary) entryListJSON {
	out := make([]entrySummaryJSON, 0, len(list)) // never nil
	for _, e := range list {
		out = append(out, summaryToJSON(e))
	}
	return entryListJSON{Entries: out, Count: len(out)}
}
