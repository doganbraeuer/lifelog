package core

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// Synthetic lab report as in docs/API.md. Document content stays German.
const labReport = "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\n" +
	"Laborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\n" +
	"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\n" +
	"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein"

func TestSplit_ShortTextIsOneChunk(t *testing.T) {
	got := SplitIntoChunks(labReport, ChunkMaxChars)
	if len(got) != 1 || got[0] != labReport {
		t.Errorf("want the whole text as one chunk, got %q", got)
	}
}

func TestSplit_ParagraphsAndLines(t *testing.T) {
	// max 60. Lengths in characters:
	//   paragraph 1: 17 + 1 + 29 = 47      → fits, stays whole
	//   paragraph 2: 26 + 1 + 40 = 67      → too long, split at the line
	//   paragraph 3: 37 | 34 | 26 | 25     → lines are merged as long as
	//                they fit: 37+1+34 = 72 no, 34+1+26 = 61 no,
	//                26+1+25 = 52 yes
	// Paragraphs 1 and 2 together (47 + 2 + 26) would also be too long.
	got := SplitIntoChunks(labReport, 60)
	want := []string{
		"Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg",
		"Laborbefund vom 14.10.2026",
		"Patient: Max Mustermann, geb. 01.01.1990",
		"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)",
		"Leukozyten 6,8 G/l (Ref. 4,0-10,0)",
		"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
	}
	if !slices.Equal(got, want) {
		t.Errorf("chunks:\n%q\nwant:\n%q", got, want)
	}
}

func TestSplit_SmallParagraphsAreMerged(t *testing.T) {
	got := SplitIntoChunks("A\n\nB\n\nC", 4)
	// "A\n\nB" has exactly 4 characters, C no longer fits.
	if !slices.Equal(got, []string{"A\n\nB", "C"}) {
		t.Errorf("= %q", got)
	}
}

func TestSplit_LongLineIsSplitAtWords(t *testing.T) {
	got := SplitIntoChunks("eins zwei drei vier", 9)
	if !slices.Equal(got, []string{"eins zwei", "drei vier"}) {
		t.Errorf("= %q", got)
	}
}

func TestSplit_OverlongWordIsCutHard(t *testing.T) {
	got := SplitIntoChunks(strings.Repeat("ä", 7), 3)
	if !slices.Equal(got, []string{"äää", "äää", "ä"}) {
		t.Errorf("= %q", got)
	}
	for _, c := range got {
		if !utf8.ValidString(c) {
			t.Errorf("umlaut cut in half: %q", c)
		}
	}
}

func TestSplit_UmlautsCountAsOneCharacter(t *testing.T) {
	// 5 characters but 10 bytes: fits into max 5.
	if got := SplitIntoChunks("äöüßé", 5); !slices.Equal(got, []string{"äöüßé"}) {
		t.Errorf("= %q", got)
	}
}

func TestSplit_Whitespace(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\n\n", " \t\n \n"} {
		got := SplitIntoChunks(text, 10)
		if got == nil || len(got) != 0 {
			t.Errorf("SplitIntoChunks(%q) = %#v, want empty list", text, got)
		}
	}
}

func TestSplit_LineEndingsAndBlankLinesWithSpaces(t *testing.T) {
	// Windows line endings become "\n", and a "blank line" containing spaces
	// separates paragraphs. max 30 is deliberately large enough for
	// everything to fit into ONE chunk (12 + 2 + 5 = 19). So not the length
	// but only the splitting decides what the result looks like:
	//   - line with spaces not recognized as separator → "Kopf\nZeile 2\n  \nRumpf"
	//   - "\r\n" not converted                         → result contains "\r"
	got := SplitIntoChunks("Kopf\r\nZeile 2\r\n  \r\nRumpf", 30)
	if !slices.Equal(got, []string{"Kopf\nZeile 2\n\nRumpf"}) {
		t.Errorf("= %q", got)
	}
}

func TestSplit_Invariants(t *testing.T) {
	// Longer synthetic text with all cases mixed.
	text := strings.Repeat(labReport+"\n\n", 6) +
		strings.Repeat("Wort ", 300) + "\n\n" + strings.Repeat("x", 1200)

	for _, max := range []int{1, 7, 50, 120, ChunkMaxChars} {
		got := SplitIntoChunks(text, max)
		if len(got) == 0 {
			t.Fatalf("max %d: no chunks", max)
		}
		for i, c := range got {
			if strings.TrimSpace(c) == "" {
				t.Errorf("max %d: chunk %d is empty", max, i)
			}
			if n := utf8.RuneCountInString(c); n > max {
				t.Errorf("max %d: chunk %d has %d characters", max, i, n)
			}
		}
		// Nothing is lost: all non-whitespace stays in order.
		if withoutWhitespace(strings.Join(got, "")) != withoutWhitespace(text) {
			t.Errorf("max %d: content changed", max)
		}
		// Deterministic.
		if !slices.Equal(got, SplitIntoChunks(text, max)) {
			t.Errorf("max %d: second call gives a different result", max)
		}
	}
}

func TestSplit_InvalidMaxUsesDefault(t *testing.T) {
	if got := SplitIntoChunks(labReport, 0); len(got) != 1 {
		t.Errorf("max 0 should use ChunkMaxChars, got %d chunks", len(got))
	}
}

func withoutWhitespace(s string) string {
	return strings.Join(strings.Fields(s), "")
}
