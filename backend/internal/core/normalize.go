package core

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// edgePunctuation is removed at the start and end of a word: "Beispiel,"
// and "(Ref." should match "Beispiel" and "Ref". Characters that carry
// meaning in values stay: "<0,5", "-", "+", "%", "/" (units like "g/dl").
const edgePunctuation = `.,:;!?()[]{}"'`

// normalizeWord turns a word into the form used for comparing:
//
//  1. Unicode NFKC: ligatures like "ﬁ" (frequent in OCR) become "fi",
//     composed and decomposed umlauts become the same.
//  2. Lowercase.
//  3. Typographic dashes and quotation marks become plain ones.
//  4. Punctuation at the edges is removed (edgePunctuation).
//  5. A decimal comma between two digits becomes a point: "14,2" = "14.2".
//
// The result can be empty, e.g. for a lone "." or ":". Empty words are
// skipped when comparing.
func normalizeWord(w string) string {
	w = norm.NFKC.String(w)
	w = strings.ToLower(w)
	w = strings.Map(func(r rune) rune {
		switch r {
		case '‐', '‑', '‒', '–', '—', '−':
			return '-'
		case '„', '“', '”', '«', '»':
			return '"'
		case '‚', '‘', '’', '‹', '›':
			return '\''
		}
		return r
	}, w)
	w = strings.Trim(w, edgePunctuation)

	r := []rune(w)
	for i := 1; i+1 < len(r); i++ {
		if r[i] == ',' && unicode.IsDigit(r[i-1]) && unicode.IsDigit(r[i+1]) {
			r[i] = '.'
		}
	}
	return string(r)
}

// normalizeKey turns a value (date form, issuer, fact value or unit) into a
// search key: split at whitespace, normalize each word, join without
// separators. Whitespace therefore does not count: "14,2 g/dl" and
// "14,2g/dl" give the same key "14.2g/dl".
func normalizeKey(s string) string {
	var b strings.Builder
	for _, w := range strings.Fields(s) {
		b.WriteString(normalizeWord(w))
	}
	return b.String()
}
