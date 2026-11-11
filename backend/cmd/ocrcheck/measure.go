package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lifelog/backend/internal/core"
)

// Variants of a demo document. The summary groups by kind: the question
// "is Tesseract good enough?" concerns photos and OCR on PDFs; the text
// layer and text import are the reference.
const (
	kindPDFText = "PDF, text layer"
	kindPDFOCR  = "PDF, OCR (forced)"
	kindPhoto   = "Photo"
	kindText    = "Text import"
)

var kindOrder = []string{kindPDFText, kindPDFOCR, kindPhoto, kindText}

// demoFile is one file in demo/.
type demoFile struct {
	Path string // relative to the demo folder, e.g. "photos/01_…_photo.png"
	Name string // file name
	Doc  string // document it shows: the name without extension, "_photo" and "_app"
}

// variantName: "pdf", "photo", "app" or "text".
func (f demoFile) variantName() string {
	switch {
	case strings.HasSuffix(f.Name, ".pdf"):
		return "pdf"
	case strings.HasSuffix(f.Name, ".txt"):
		return "text"
	case strings.Contains(f.Name, "_app."):
		return "app"
	}
	return "photo"
}

// documentKey: "04_prescription_2026-05-18_app.png" → "04_prescription_2026-05-18".
func documentKey(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	for _, suffix := range []string{"_photo", "_app"} {
		base = strings.TrimSuffix(base, suffix)
	}
	return base
}

// demoFiles lists the files in the demo folder (pdf/, photos/, live/,
// texts/), sorted by document and variant.
func demoFiles(dir string) ([]demoFile, error) {
	var files []demoFile
	for _, sub := range []string{"pdf", "photos", "live", "texts"} {
		entries, err := os.ReadDir(filepath.Join(dir, sub))
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || strings.HasPrefix(name, ".") {
				continue
			}
			switch strings.ToLower(filepath.Ext(name)) {
			case ".pdf", ".png", ".jpg", ".jpeg", ".txt":
				files = append(files, demoFile{Path: filepath.Join(sub, name), Name: name, Doc: documentKey(name)})
			}
		}
	}
	order := map[string]int{"pdf": 0, "photo": 1, "app": 2, "text": 3}
	slices.SortFunc(files, func(a, b demoFile) int {
		if a.Doc != b.Doc {
			return strings.Compare(a.Doc, b.Doc)
		}
		return order[a.variantName()] - order[b.variantName()]
	})
	return files, nil
}

// row is the measurement of one variant of one document.
type row struct {
	Doc, Variant, Kind string
	Source             string
	Reason             string // unchecked: why
	Date, Issuer       string // "✓", "✗" or "–" (no value)
	Facts, FactsTotal  int    // value and unit found
	Values             int    // confirmed, or the value found when the unit is ignored
	Match              *int
	Confidence         *float64 // mean confidence of the OCR words
	Duration           time.Duration
	Missing            []string // unconfirmed fields with expected value (and OCR row), for -values
}

// found: the current value is in the original (also after a correction).
func found(status string) bool {
	return status == core.FieldConfirmed || status == core.FieldCorrectedConfirmed
}

// measureRow evaluates the verification of entry e against r. valueOnly is
// the verification of the same entry without units.
func measureRow(doc, variant, kind string, e core.Entry, v, valueOnly core.Verification, r core.OCRResult, d time.Duration) row {
	out := row{Doc: doc, Variant: variant, Kind: kind, Source: v.Source, Reason: v.Reason,
		Date: "–", Issuer: "–", Match: v.MatchPercent(), Duration: d}
	if v.Status != core.VerificationChecked {
		return out
	}
	rows := core.RowTexts(r.Words)
	for _, f := range v.Fields {
		mark := "✗"
		if found(f.Status) {
			mark = "✓"
		}
		switch {
		case f.Field == "documentDate":
			out.Date = mark
		case f.Field == "issuer":
			out.Issuer = mark
		default:
			out.FactsTotal++
			if mark == "✓" {
				out.Facts++
			}
		}
		if mark == "✗" {
			out.Missing = append(out.Missing, describe(e, f.Field, rows))
		}
	}
	// "Value only" asks: does the fact fail only because of its unit? So a
	// fact counts if it is confirmed, or if its value is found once the
	// unit is ignored. (Without its unit a short value like "20" must stand
	// in the line of its name, so the unitless check alone can be stricter.)
	strict := map[string]bool{}
	for _, f := range v.Fields {
		strict[f.Field] = found(f.Status)
	}
	for _, f := range valueOnly.Fields {
		if strings.HasPrefix(f.Field, "facts.") && (strict[f.Field] || found(f.Status)) {
			out.Values++
		}
	}
	out.Confidence = meanConfidence(r.Words)
	return out
}

// describe: field, expected value and the OCR row that should hold it.
// Demo data are synthetic, so printing values is fine here (never in the
// server log).
func describe(e core.Entry, field string, rows []string) string {
	switch field {
	case "documentDate":
		return fmt.Sprintf("documentDate: %s", deref(e.DocumentDate))
	case "issuer":
		s := fmt.Sprintf("issuer: %s", deref(e.Issuer))
		if e.Issuer != nil {
			for _, part := range strings.Split(*e.Issuer, ",") {
				part = strings.TrimSpace(part)
				if r := rowWith(rows, part); r != "" {
					s += fmt.Sprintf(" — %q read: %q", part, r)
				} else {
					s += fmt.Sprintf(" — %q: no row with its words", part)
				}
			}
		}
		return s
	}
	id := strings.TrimPrefix(field, "facts.")
	for _, f := range e.Facts {
		if f.ID != id {
			continue
		}
		want := f.Value
		if f.Unit != nil {
			want += " " + *f.Unit
		}
		s := fmt.Sprintf("%s %s: %s", id, f.Name, want)
		if r := rowWith(rows, f.Name); r != "" {
			s += fmt.Sprintf(" — read: %q", r)
		} else {
			s += " — name not found in any row"
		}
		return s
	}
	return field
}

// rowWith returns the row that contains the most words of name (at least
// one), the first of them on a tie. "Sehschärfe mit Brille Rechts" thus
// finds the row "Sehschärfe mit Brille …", not "Sehschärfe ohne Brille …".
func rowWith(rows []string, name string) string {
	words := strings.Fields(strings.ToLower(name))
	best, bestCount := "", 0
	for _, r := range rows {
		lower, n := strings.ToLower(r), 0
		for _, w := range words {
			if strings.Contains(lower, strings.Trim(w, ".,:;()")) {
				n++
			}
		}
		if n > bestCount {
			best, bestCount = r, n
		}
	}
	return best
}

func meanConfidence(words []core.OCRWord) *float64 {
	sum, n := 0.0, 0
	for _, w := range words {
		if w.Source == core.SourceOCR {
			sum += w.Confidence
			n++
		}
	}
	if n == 0 {
		return nil
	}
	m := sum / float64(n)
	return &m
}

func deref(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

// withoutUnits is the entry with every unit removed: "is the value there at
// all?" (the plan's question whether units need a tolerance).
func withoutUnits(e core.Entry) core.Entry {
	facts := slices.Clone(e.Facts)
	for i := range facts {
		facts[i].Unit = nil
	}
	e.Facts = facts
	return e
}

// summary per kind.
type summary struct {
	Kind              string
	Documents         int
	Unchecked         int
	MatchMean         *float64
	MatchMin          *int
	Facts, FactsTotal int
	Values            int
}

func summarize(rows []row) []summary {
	var out []summary
	for _, kind := range kindOrder {
		s := summary{Kind: kind}
		sum, n := 0, 0
		for _, r := range rows {
			if r.Kind != kind {
				continue
			}
			s.Documents++
			if r.Match == nil {
				if r.Reason != "" {
					s.Unchecked++
				}
				continue
			}
			sum += *r.Match
			n++
			if s.MatchMin == nil || *r.Match < *s.MatchMin {
				m := *r.Match
				s.MatchMin = &m
			}
			s.Facts += r.Facts
			s.FactsTotal += r.FactsTotal
			s.Values += r.Values
		}
		if s.Documents == 0 {
			continue
		}
		if n > 0 {
			m := float64(sum) / float64(n)
			s.MatchMean = &m
		}
		out = append(out, s)
	}
	return out
}

// report is the measurement as Markdown (docs/ocr/).
type report struct {
	Date      string
	Programs  string
	Database  string
	Rows      []row
	Missing   []string // documents in demo/ without an entry in the database
	AllValues bool     // -values: list the unconfirmed fields
}

func (rep report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# OCR measurement %s\n\n", rep.Date)
	fmt.Fprintf(&b, "Generated with `cd backend && go run ./cmd/ocrcheck`. Programs: %s. "+
		"Reference values: the current values (including corrections) of the entries in database %q "+
		"that were uploaded from `demo/`; every variant of a document is checked against the same "+
		"entry. No model was called. Rules: docs/API.md, section verification.\n\n", rep.Programs, rep.Database)
	b.WriteString("\"Facts (value + unit)\" is the strict rule of the verification; \"value only\" " +
		"ignores the unit and shows how many facts fail only because of the unit.\n\n")

	b.WriteString("## Per document\n\n")
	b.WriteString("| Document | Variant | Source | Date | Issuer | Facts (value + unit) | Facts (value only) | Match | Mean OCR confidence | Time |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rep.Rows {
		if r.Reason != "" {
			fmt.Fprintf(&b, "| %s | %s | unchecked: %s | | | | | | | %s |\n", r.Doc, r.Variant, r.Reason, seconds(r.Duration))
			continue
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d/%d | %d/%d | %s | %s | %s |\n",
			r.Doc, r.Variant, r.Source, r.Date, r.Issuer, r.Facts, r.FactsTotal, r.Values, r.FactsTotal,
			percent(r.Match), conf(r.Confidence), seconds(r.Duration))
	}

	b.WriteString("\n## Summary\n\n")
	b.WriteString("| Variant | Documents | Match mean | Match min | Facts (value + unit) | Facts (value only) | Unchecked |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, s := range summarize(rep.Rows) {
		mean := "–"
		if s.MatchMean != nil {
			mean = fmt.Sprintf("%.0f %%", *s.MatchMean)
		}
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s | %d |\n", s.Kind, s.Documents, mean, percent(s.MatchMin),
			ratio(s.Facts, s.FactsTotal), ratio(s.Values, s.FactsTotal), s.Unchecked)
	}

	if len(rep.Missing) > 0 {
		b.WriteString("\nNot measured (no entry in the database; run demo/upload.sh): " +
			strings.Join(rep.Missing, ", ") + "\n")
	}

	if rep.AllValues {
		b.WriteString("\n## Not confirmed\n\n")
		for _, r := range rep.Rows {
			if len(r.Missing) == 0 {
				continue
			}
			fmt.Fprintf(&b, "- **%s, %s**\n", r.Doc, r.Variant)
			for _, m := range r.Missing {
				fmt.Fprintf(&b, "  - %s\n", m)
			}
		}
	}
	return b.String()
}

func percent(p *int) string {
	if p == nil {
		return "–"
	}
	return fmt.Sprintf("%d %%", *p)
}

func conf(c *float64) string {
	if c == nil {
		return "–"
	}
	return fmt.Sprintf("%.0f", *c)
}

func ratio(a, b int) string {
	if b == 0 {
		return "–"
	}
	return fmt.Sprintf("%d/%d (%.0f %%)", a, b, math.Round(float64(a)*100/float64(b)))
}

func seconds(d time.Duration) string { return fmt.Sprintf("%.1f s", d.Seconds()) }
