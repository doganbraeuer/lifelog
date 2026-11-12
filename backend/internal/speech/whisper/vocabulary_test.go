package whisper

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestVocabularyTerms(t *testing.T) {
	terms := VocabularyTerms()
	if len(terms) < 40 {
		t.Fatalf("only %d terms: %q", len(terms), terms)
	}
	for _, want := range []string{"Blutdruck", "Vitamin D", "mU/l", "1-0-1", "bei Bedarf", "Diabetes mellitus", "FSME"} {
		if !slices.Contains(terms, want) {
			t.Errorf("term %q missing in %q", want, terms)
		}
	}
	seen := map[string]bool{}
	for _, term := range terms {
		if seen[term] {
			t.Errorf("term %q twice", term)
		}
		seen[term] = true
		if strings.ContainsAny(term, ",.") || term != strings.TrimSpace(term) {
			t.Errorf("badly split term %q", term)
		}
	}
}

// doseScheme is "1-0-1" and the like.
var doseScheme = regexp.MustCompile(`^[0-9]-[0-9]-[0-9]$`)

// A number in the prompt could show up in a transcript although nobody
// said it. Only dose schemes and names with digits are allowed.
func TestVocabulary_NoExampleValues(t *testing.T) {
	names := []string{"HbA1c"}
	for _, term := range VocabularyTerms() {
		if !strings.ContainsFunc(term, unicode.IsDigit) || doseScheme.MatchString(term) || slices.Contains(names, term) {
			continue
		}
		t.Errorf("term %q contains a number", term)
	}
}

// Whisper drops the beginning of a prompt over 224 tokens. The 433
// characters of 2026-11-12 were about 205 tokens (vocabulary.go), about
// 2.1 characters per token; 450 characters stay below 224.
func TestVocabulary_Length(t *testing.T) {
	if n := utf8.RuneCountInString(Vocabulary); n > 450 {
		t.Errorf("Vocabulary has %d characters; keep it at most 450 (about 215 tokens)", n)
	}
}
