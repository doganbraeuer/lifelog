package core

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Values of Verification and FieldCheck (docs/API.md, section 2,
// verification).
const (
	VerificationChecked   = "checked"
	VerificationUnchecked = "unchecked"

	FieldConfirmed   = "confirmed"
	FieldUnconfirmed = "unconfirmed"

	SourceOCR  = "ocr"  // file: compared against OCR
	SourceText = "text" // text import: compared against the entered text

	ReasonOCRUnavailable      = "ocr_unavailable"       // OCR switched off or not installed
	ReasonOCRFailed           = "ocr_failed"            // error or time limit
	ReasonUnsupportedFileType = "unsupported_file_type" // e.g. HEIC
	ReasonTooManyPages        = "too_many_pages"        // PDF over the page limit
	ReasonNoText              = "no_text"               // OCR found no text at all
	ReasonNotRun              = "not_run"               // entry from before verification existed
)

const (
	// maxLocations: at most this many locations per field, best first.
	maxLocations = 3
	// unitWindow: the unit must start within this many words after the
	// value. Room for a flag like "H" or "*" in between.
	unitWindow = 3
	// shortValueChars: a value without a unit that is at most this long
	// (e.g. "98") only counts in the line of the fact's name.
	shortValueChars = 3
)

// Verification is the check of an entry's recognized values by a second
// reader (OCR, or the entered text for a text import).
type Verification struct {
	Status    string     // VerificationChecked or VerificationUnchecked
	Reason    string     // only for unchecked, e.g. ReasonOCRFailed
	Source    string     // SourceOCR or SourceText; empty for unchecked
	Engine    string     // e.g. "tesseract 5.5.4 deu"; empty for text and unchecked
	CheckedAt *time.Time // nil for unchecked
	Confirmed int
	Total     int
	Fields    []FieldCheck // never nil
}

// MatchPercent is Confirmed/Total in percent, rounded. nil if the entry is
// unchecked or no field was checked.
func (v Verification) MatchPercent() *int {
	if v.Status != VerificationChecked || v.Total == 0 {
		return nil
	}
	p := int(math.Round(float64(v.Confirmed) * 100 / float64(v.Total)))
	return &p
}

// FieldCheck is the result for one field.
type FieldCheck struct {
	Field      string // "documentDate", "issuer" or "facts.<id>"
	Status     string // FieldConfirmed or FieldUnconfirmed
	Confidence *int   // lowest OCR confidence of the matched words; nil if unconfirmed or text
	Locations  []Location
}

// Location is a place in the original: a rectangle on a page (file) or a
// range in the full text (text import, Page 0).
type Location struct {
	Page       int // starting at 1; 0 = range in the text
	Box        Box
	Start, End int // code points, End exclusive (only if Page is 0)
}

// Unchecked is the verification of an entry that could not be checked.
func Unchecked(reason string) Verification {
	return Verification{Status: VerificationUnchecked, Reason: reason, Fields: []FieldCheck{}}
}

// Verify checks the entry's current values against the OCR result. If OCR
// found no text at all, the entry counts as unchecked (ReasonNoText): a
// blank OCR says nothing about whether the values are right.
func Verify(e Entry, r OCRResult, now time.Time) Verification {
	d := newDocument(r.Words, false)
	if len(d.tokens) == 0 {
		return Unchecked(ReasonNoText)
	}
	return d.verify(e, SourceOCR, r.Engine, now)
}

// VerifyText checks the entry's values against its full text (text import).
func VerifyText(e Entry, now time.Time) Verification {
	return newDocument(WordsFromText(e.FullText), true).verify(e, SourceText, "", now)
}

// token is a word in normalized form. Words whose normalized form is empty
// (lone punctuation) get no token.
type token struct {
	key  string
	word int // index into document.words
}

// document is the text a value is searched in.
type document struct {
	words  []OCRWord
	tokens []token
	text   bool // text import: locations are ranges, no confidence
}

func newDocument(words []OCRWord, text bool) *document {
	d := &document{words: words, text: text}
	for i, w := range words {
		if k := normalizeWord(w.Text); k != "" {
			d.tokens = append(d.tokens, token{key: k, word: i})
		}
	}
	return d
}

// match is a run of tokens, first and last inclusive.
type match struct{ first, last int }

func (d *document) verify(e Entry, source, engine string, now time.Time) Verification {
	at := now.UTC()
	v := Verification{
		Status:    VerificationChecked,
		Source:    source,
		Engine:    engine,
		CheckedAt: &at,
		Fields:    []FieldCheck{},
	}
	add := func(field string, best, all []match) {
		v.Fields = append(v.Fields, d.fieldCheck(field, best, all))
	}
	if e.DocumentDate != nil {
		all := d.findDate(*e.DocumentDate)
		add("documentDate", head(all), all)
	}
	if e.Issuer != nil {
		best, all := d.findIssuer(*e.Issuer)
		add("issuer", best, all)
	}
	for _, f := range e.Facts {
		all := d.findFact(f)
		add("facts."+f.ID, head(all), all)
	}

	v.Total = len(v.Fields)
	for _, fc := range v.Fields {
		if fc.Status == FieldConfirmed {
			v.Confirmed++
		}
	}
	return v
}

// head returns the first match as a list, or nil.
func head(all []match) []match {
	if len(all) == 0 {
		return nil
	}
	return all[:1]
}

// fieldCheck builds the result. best are the matches that confirm the field
// (their confidence counts), all are shown as locations (best first).
func (d *document) fieldCheck(field string, best, all []match) FieldCheck {
	fc := FieldCheck{Field: field, Status: FieldUnconfirmed, Locations: []Location{}}
	if len(best) == 0 {
		return fc
	}
	fc.Status = FieldConfirmed
	for _, m := range all {
		for _, l := range d.locations(m) {
			if len(fc.Locations) < maxLocations {
				fc.Locations = append(fc.Locations, l)
			}
		}
	}
	if !d.text {
		lowest := math.Inf(1)
		for _, m := range best {
			for wi := d.tokens[m.first].word; wi <= d.tokens[m.last].word; wi++ {
				lowest = min(lowest, d.words[wi].Confidence)
			}
		}
		c := int(math.Round(lowest))
		fc.Confidence = &c
	}
	return fc
}

// locations: for a text one range from the first to the last word; for a
// file one rectangle per line, enclosing all words of the match in that
// line (also punctuation in between).
func (d *document) locations(m match) []Location {
	firstWord, lastWord := d.tokens[m.first].word, d.tokens[m.last].word
	if d.text {
		// Without punctuation at the edges, so "mmHg," marks only "mmHg".
		first, last := d.words[firstWord], d.words[lastWord]
		lead := utf8.RuneCountInString(first.Text) - utf8.RuneCountInString(strings.TrimLeft(first.Text, edgePunctuation))
		trail := utf8.RuneCountInString(last.Text) - utf8.RuneCountInString(strings.TrimRight(last.Text, edgePunctuation))
		return []Location{{Start: first.Start + lead, End: last.End - trail}}
	}
	var out []Location
	for wi := firstWord; wi <= lastWord; wi++ {
		w := d.words[wi]
		if wi > firstWord && sameLine(d.words[wi-1], w) {
			out[len(out)-1].Box = out[len(out)-1].Box.union(w.Box)
			continue
		}
		out = append(out, Location{Page: w.Page, Box: w.Box})
	}
	return out
}

func sameLine(a, b OCRWord) bool { return a.Page == b.Page && a.Line == b.Line }

// tokensOnSameLine: are tokens i and j in the same line?
func (d *document) tokensOnSameLine(i, j int) bool {
	return sameLine(d.words[d.tokens[i].word], d.words[d.tokens[j].word])
}

// find returns all runs of tokens whose keys, joined without separators,
// equal key, in reading order. A run starts and ends at word boundaries, so
// "4" does not match inside "14".
func (d *document) find(key string) []match {
	if key == "" {
		return nil
	}
	var out []match
	for i := range d.tokens {
		if m, ok := d.matchAt(i, key); ok {
			out = append(out, m)
		}
	}
	return out
}

// matchAt checks whether the tokens starting at i spell exactly key.
func (d *document) matchAt(i int, key string) (match, bool) {
	rest := key
	for j := i; j < len(d.tokens); j++ {
		k := d.tokens[j].key
		if !strings.HasPrefix(rest, k) {
			return match{}, false
		}
		rest = rest[len(k):]
		if rest == "" {
			return match{i, j}, true
		}
	}
	return match{}, false
}

// findDate searches all common German forms of the date (see DateForms).
func (d *document) findDate(date string) []match {
	var all []match
	for _, form := range DateForms(date) {
		all = append(all, d.find(normalizeKey(form))...)
	}
	return sortedUnique(all)
}

// findIssuer: the whole issuer as one run (also across lines), or else
// every comma-separated part somewhere in the document. best are the
// matches that confirm it; nil if it is not confirmed.
func (d *document) findIssuer(issuer string) (best, all []match) {
	if all := d.find(normalizeKey(issuer)); len(all) > 0 {
		return all[:1], all
	}
	parts := strings.Split(issuer, ",")
	var found []match
	for _, p := range parts {
		key := normalizeKey(p)
		if key == "" {
			continue
		}
		ms := d.find(key)
		if len(ms) == 0 {
			return nil, nil
		}
		found = append(found, ms[0])
	}
	if len(found) < 2 { // no comma, or only one non-empty part: already covered above
		return nil, nil
	}
	return found, found
}

// findFact searches value and unit. Rules (docs/API.md, verification):
//   - With a unit: the unit follows the value in the same line within
//     unitWindow words, or is written together with it ("14,2g/dl").
//   - Without a unit: the value alone; a short value (≤ shortValueChars)
//     only in the line of the fact's name.
//   - Matches in the line of the name come first.
func (d *document) findFact(f Fact) []match {
	value := normalizeKey(f.Value)
	if value == "" {
		return nil
	}
	unit := ""
	if f.Unit != nil {
		unit = normalizeKey(*f.Unit)
	}

	var all []match
	if unit != "" {
		for _, m := range d.find(value + unit) {
			if d.tokensOnSameLine(m.first, m.last) {
				all = append(all, m)
			}
		}
		for _, m := range d.find(value) {
			for k := m.last + 1; k <= m.last+unitWindow && k < len(d.tokens); k++ {
				if !d.tokensOnSameLine(m.last, k) {
					break
				}
				if u, ok := d.matchAt(k, unit); ok && d.tokensOnSameLine(k, u.last) {
					all = append(all, match{m.first, u.last})
					break
				}
			}
		}
	} else {
		all = d.find(value)
		if utf8.RuneCountInString(value) <= shortValueChars {
			all = slices.DeleteFunc(all, func(m match) bool { return !d.onNameLine(m, f.Name) })
		}
	}

	all = sortedUnique(all)
	// Matches in the line of the name first; otherwise reading order.
	slices.SortStableFunc(all, func(a, b match) int {
		return boolRank(d.onNameLine(b, f.Name)) - boolRank(d.onNameLine(a, f.Name))
	})
	return all
}

// onNameLine: does a word in the line of the match start with the first
// word of the name? Prefix, so that "Glukose" also finds "Glukose-nüchtern".
func (d *document) onNameLine(m match, name string) bool {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return false
	}
	first := normalizeWord(fields[0])
	if first == "" {
		return false
	}
	for i := m.first - 1; i >= 0 && d.tokensOnSameLine(i, m.first); i-- {
		if strings.HasPrefix(d.tokens[i].key, first) {
			return true
		}
	}
	for i := m.last + 1; i < len(d.tokens) && d.tokensOnSameLine(i, m.first); i++ {
		if strings.HasPrefix(d.tokens[i].key, first) {
			return true
		}
	}
	return false
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sortedUnique sorts by position and removes duplicates.
func sortedUnique(ms []match) []match {
	slices.SortFunc(ms, func(a, b match) int {
		if a.first != b.first {
			return a.first - b.first
		}
		return a.last - b.last
	})
	return slices.Compact(ms)
}

// monthNames per month: full name, Austrian name, abbreviations. Matching
// is case-insensitive and ignores a trailing point ("Okt.").
var monthNames = [12][]string{
	{"Januar", "Jänner", "Jan", "Jän"},
	{"Februar", "Feber", "Feb"},
	{"März", "Mär", "Mrz"},
	{"April", "Apr"},
	{"Mai"},
	{"Juni", "Jun"},
	{"Juli", "Jul"},
	{"August", "Aug"},
	{"September", "Sept", "Sep"},
	{"Oktober", "Okt"},
	{"November", "Nov"},
	{"Dezember", "Dez"},
}

// DateForms returns the forms in which a date "YYYY-MM-DD" can appear on a
// German document, e.g. for 2026-01-20: "20.01.2026", "20.1.2026",
// "20.01.26", "2026-01-20", "20/01/2026", "20. Jänner 2026" … An invalid
// date gives no forms.
func DateForms(date string) []string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil || !isDateFormat(date) {
		return nil
	}
	d, m, y := t.Day(), int(t.Month()), t.Year()
	forms := []string{
		fmt.Sprintf("%02d.%02d.%04d", d, m, y),
		fmt.Sprintf("%d.%d.%04d", d, m, y),
		fmt.Sprintf("%02d.%02d.%02d", d, m, y%100),
		fmt.Sprintf("%d.%d.%02d", d, m, y%100),
		fmt.Sprintf("%04d-%02d-%02d", y, m, d),
		fmt.Sprintf("%02d/%02d/%04d", d, m, y),
		fmt.Sprintf("%d/%d/%04d", d, m, y),
		fmt.Sprintf("%02d-%02d-%04d", d, m, y),
	}
	for _, name := range monthNames[m-1] {
		forms = append(forms,
			fmt.Sprintf("%d. %s %04d", d, name, y),
			fmt.Sprintf("%02d. %s %04d", d, name, y))
	}
	// For day and month ≥ 10, "20.10.2026" comes out twice; keep each once.
	seen := map[string]bool{}
	return slices.DeleteFunc(forms, func(f string) bool {
		dup := seen[f]
		seen[f] = true
		return dup
	})
}
