package core

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestCheckQuestion(t *testing.T) {
	if q, err := CheckQuestion("  Wie hoch war mein Hb?\n"); err != nil || q != "Wie hoch war mein Hb?" {
		t.Errorf("= %q, %v", q, err)
	}
	for _, empty := range []string{"", "   ", "\n\t"} {
		if _, err := CheckQuestion(empty); !errors.Is(err, ErrQuestionMissing) {
			t.Errorf("%q: %v", empty, err)
		}
	}
	// Limit in characters, not bytes: 500 × "ä" (1000 bytes) is allowed.
	if _, err := CheckQuestion(strings.Repeat("ä", 500)); err != nil {
		t.Errorf("500 characters: %v", err)
	}
	if _, err := CheckQuestion(strings.Repeat("ä", 501)); !errors.Is(err, ErrQuestionTooLong) {
		t.Errorf("501 characters: %v", err)
	}
	// Whitespace at the edges does not count.
	if _, err := CheckQuestion("  " + strings.Repeat("a", 500) + "  "); err != nil {
		t.Errorf("500 characters with edges: %v", err)
	}
}

func TestScoreConversion(t *testing.T) {
	cases := map[float64]float64{-1: 0, 0: 0.5, 0.6: 0.8, 1: 1}
	for cos, score := range cases {
		if got := ScoreFromCosine(cos); math.Abs(got-score) > 1e-12 {
			t.Errorf("ScoreFromCosine(%v) = %v, want %v", cos, got, score)
		}
		if got := CosineFromScore(score); math.Abs(got-cos) > 1e-12 {
			t.Errorf("CosineFromScore(%v) = %v, want %v", score, got, cos)
		}
	}
	// The threshold 0.82 corresponds to cos 0.64 (comment on MinScore).
	if got := CosineFromScore(MinScore); math.Abs(got-0.64) > 1e-12 {
		t.Errorf("MinScore corresponds to cos %v", got)
	}
}

func TestAboveThreshold(t *testing.T) {
	// Relative to MinScore, so a new calibration does not break the test.
	hits := []Hit{
		{Position: 1, Score: 0.95}, {Position: 2, Score: MinScore}, {Position: 3, Score: MinScore - 0.0001}, {Position: 4, Score: MinScore + 0.03},
	}
	got := AboveThreshold(hits, MinScore)
	var positions []int
	for _, h := range got {
		positions = append(positions, h.Position)
	}
	// Exactly on the threshold counts as a hit; the order stays.
	if !slices.Equal(positions, []int{1, 2, 4}) {
		t.Errorf("= %v", positions)
	}
	if empty := AboveThreshold(nil, MinScore); empty == nil || len(empty) != 0 {
		t.Errorf("nil → %#v, want empty list", empty)
	}
}

func TestParseCitations(t *testing.T) {
	cases := []struct {
		text  string
		count int
		want  []int
	}{
		{"Hämoglobin 14,2 g/dl [1].", 3, []int{1}},
		{"A [2]. B [1][3].", 3, []int{2, 1, 3}},
		{"A [1, 3] und [2,1].", 3, []int{1, 3, 2}},
		{"Doppelt [1] [1] [1].", 3, []int{1}},
		{"Unbekannte Nummer [7] und [0].", 3, []int{}},
		{"Teils gültig [2] [9].", 3, []int{2}},
		{"Kein Verweis.", 3, []int{}},
		{NotFoundAnswer, 3, []int{}},
		{"Wert [1a] oder [ 1 ] oder (1).", 3, []int{}},
		{"[12] bei zwölf Fundstellen", 12, []int{12}},
	}
	for _, c := range cases {
		got := ParseCitations(c.text, c.count)
		if got == nil || !slices.Equal(got, c.want) {
			t.Errorf("ParseCitations(%q, %d) = %#v, want %v", c.text, c.count, got, c.want)
		}
	}
}

func TestBuildAnswer(t *testing.T) {
	sources := []Source{
		{Number: 1, EntryID: "a", Excerpt: "Hämoglobin 14,2 g/dl"},
		{Number: 2, EntryID: "b", Excerpt: "Blutdruck 128/82"},
		{Number: 3, EntryID: "a", Excerpt: "CRP <0,5"},
	}

	a := BuildAnswer("  Laut Befund: Hämoglobin 14,2 g/dl [1], CRP <0,5 [3].  ", sources)
	if !a.Found || a.Text != "Laut Befund: Hämoglobin 14,2 g/dl [1], CRP <0,5 [3]." {
		t.Errorf("answer = %+v", a)
	}
	var numbers []int
	for _, s := range a.Sources {
		numbers = append(numbers, s.Number)
	}
	if !slices.Equal(numbers, []int{1, 3}) {
		t.Errorf("sources = %v, want only the cited [1 3]", numbers)
	}

	// Without a valid reference: fixed sentence, even if the model wrote something else.
	for _, text := range []string{NotFoundAnswer, "Der Wert ist 14,2.", "Siehe [9].", ""} {
		a := BuildAnswer(text, sources)
		if a.Found || a.Text != NotFoundAnswer || a.Sources == nil || len(a.Sources) != 0 {
			t.Errorf("%q → %+v, want NoHit", text, a)
		}
	}
}
