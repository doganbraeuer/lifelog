package core

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// NotFoundAnswer is the answer when the documents contain nothing about the
// question (CLAUDE.md, API.md 3.6). Verbatim and German: the app may compare
// against it.
const NotFoundAnswer = "Dazu finde ich nichts in deinen Dokumenten."

// MaxQuestionChars: longest allowed question (API.md 3.6), in characters.
const MaxQuestionChars = 500

// MaxHits: the vector search returns at most this many chunks.
const MaxHits = 5

// MinScore is the threshold for vector search hits.
//
// With similarity "cosine", Atlas does not report the cosine similarity
// itself but score = (1 + cos) / 2, a value between 0 and 1. 0.82
// corresponds to cos = 0.64 (see ScoreFromCosine/CosineFromScore).
//
// The threshold is deliberately only a coarse filter. The second layer is
// the language model: if the chunks do not answer the question, it answers
// with NotFoundAnswer.
//
// Calibration of 2026-11-04 (go run ./cmd/kalibrieren; gemini-embedding-2,
// 768 dim.; 4 test entries with one chunk each; 18 fixed questions):
//
//	relevant   (10)  0.857 … 0.902  right entry always ranked first
//	near       (3)   0.823 … 0.856  health topic, but in no document
//	unrelated  (5)   0.763 … 0.798  highest: weather in Salzburg 0.798,
//	                                train Salzburg–Vienna 0.791 ("Salzburg"
//	                                is in the letterheads)
//
// Any value above 0.798 up to 0.857 would separate them. 0.82 leaves a margin
// of 0.022 below and 0.037 above. The margin above is deliberately larger: a
// rejected relevant question costs an answer, a let-through unrelated one
// only a call to the language model, which then returns the fixed sentence.
// "near" cannot be separated by the threshold (0.856 versus 0.857); that is
// what the second layer is for.
//
// Re-measured on 2026-11-08 with the demo data: see docs/API.md, section 4.
const MinScore = 0.82

var (
	ErrQuestionMissing = errors.New("question missing")
	ErrQuestionTooLong = errors.New("question too long")
)

// CheckQuestion removes whitespace at the edges and checks the length.
func CheckQuestion(question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", ErrQuestionMissing
	}
	if utf8.RuneCountInString(question) > MaxQuestionChars {
		return "", ErrQuestionTooLong
	}
	return question, nil
}

// ScoreFromCosine converts a cosine similarity (−1 … 1) into the score that
// Atlas Vector Search reports (0 … 1).
func ScoreFromCosine(cos float64) float64 { return (1 + cos) / 2 }

// CosineFromScore is the inverse of ScoreFromCosine.
func CosineFromScore(score float64) float64 { return 2*score - 1 }

// Hit is a chunk from the vector search.
type Hit struct {
	EntryID  string
	Position int // position of the chunk within the full text
	Text     string
	Score    float64 // Atlas score, 0 … 1
}

// AboveThreshold keeps the hits with score ≥ min, in the same order.
func AboveThreshold(hits []Hit, min float64) []Hit {
	out := make([]Hit, 0, len(hits))
	for _, h := range hits {
		if h.Score >= min {
			out = append(out, h)
		}
	}
	return out
}

// Source is a chunk as it is shown to the language model and named as a
// source in the answer. Number is the number that references like [1] in the
// answer point to.
type Source struct {
	Number       int
	EntryID      string
	Module       string
	Type         string
	Issuer       *string
	DocumentDate *string
	Excerpt      string
}

// Answer to a question (API.md 3.6).
type Answer struct {
	Text    string
	Found   bool
	Sources []Source
}

// NoHit is the answer when nothing matching was found.
func NoHit() Answer {
	return Answer{Text: NotFoundAnswer, Found: false, Sources: []Source{}}
}

// citation finds references like [1], [12] and also [1, 3] or [1,3].
var citation = regexp.MustCompile(`\[(\d+(?:\s*,\s*\d+)*)\]`)

// ParseCitations returns the numbers the answer refers to, in order of first
// appearance and without duplicates. Only numbers from 1 to count count; [7]
// with 5 sources is a mistake of the model and is ignored. The result is
// never nil.
func ParseCitations(text string, count int) []int {
	numbers := []int{}
	seen := map[int]bool{}
	for _, m := range citation.FindAllStringSubmatch(text, -1) {
		for _, part := range strings.Split(m[1], ",") {
			n, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || n < 1 || n > count || seen[n] {
				continue
			}
			seen[n] = true
			numbers = append(numbers, n)
		}
	}
	return numbers
}

// BuildAnswer turns the language model's text into the answer for the app.
//
// Rule (CLAUDE.md: "always with sources"): if the text refers to none of the
// sources, it counts as "nothing found", and the app gets NotFoundAnswer.
// This also covers the case where the model writes the fixed sentence
// itself. Otherwise Sources contains exactly the cited sources, in the order
// of their numbers.
func BuildAnswer(text string, sources []Source) Answer {
	cited := ParseCitations(text, len(sources))
	if len(cited) == 0 {
		return NoHit()
	}
	isCited := map[int]bool{}
	for _, n := range cited {
		isCited[n] = true
	}
	used := []Source{}
	for _, s := range sources {
		if isCited[s.Number] {
			used = append(used, s)
		}
	}
	return Answer{Text: strings.TrimSpace(text), Found: true, Sources: used}
}
