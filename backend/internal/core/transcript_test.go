package core

import (
	"math"
	"strings"
	"testing"
)

// segs builds segments as Whisper returns them: each text starts with a
// space (the start of a word).
func segs(texts ...string) []Segment {
	out := make([]Segment, len(texts))
	for i, t := range texts {
		if !strings.HasPrefix(t, " ") && !strings.HasPrefix(t, "\n") {
			t = " " + t
		}
		out[i] = Segment{Start: float64(i), End: float64(i + 1), Text: t}
	}
	return out
}

func TestCleanTranscript(t *testing.T) {
	tests := []struct {
		name string
		in   []Segment
		want string
	}{
		{"joined with one space, whitespace cleaned",
			segs(" Arztbrief vom 12. November 2026.", "\n Kniegelenk  rechts. "),
			"Arztbrief vom 12. November 2026. Kniegelenk rechts."},
		{"nothing", nil, ""},
		{"only empty segments", segs(" ", ""), ""},
		{"ZDF credit in silence", segs("Untertitel im Auftrag des ZDF, 2017"), ""},
		{"credit after real text",
			segs("Ibuprofen 400 mg dreimal täglich.", "Untertitelung des ZDF für funk, 2017"),
			"Ibuprofen 400 mg dreimal täglich."},
		{"Amara credit", segs("Untertitel der Amara.org-Community"), ""},
		{"thanks for watching", segs("Vielen Dank fürs Zuschauen!"), ""},
		{"thanks, typographic apostrophe", segs("Vielen Dank für’s Zuschauen."), ""},
		{"copyright", segs("Copyright WDR 2021"), ""},
		{"real thanks stay", segs("Vielen Dank für die Überweisung."), "Vielen Dank für die Überweisung."},
		{"long real sentence starting with Untertitel stays",
			segs("Untertitel des Befunds lautet Kniegelenk rechts mit leichtem Erguss und Schwellung seit zwei Wochen nach dem Sturz."),
			"Untertitel des Befunds lautet Kniegelenk rechts mit leichtem Erguss und Schwellung seit zwei Wochen nach dem Sturz."},
		{"loop folded", segs("Dreimal täglich.", "Dreimal täglich.", "dreimal täglich", "Dreimal täglich."), "Dreimal täglich."},
		{"two repeats stay", segs("Bitte noch einmal.", "Bitte noch einmal."), "Bitte noch einmal. Bitte noch einmal."},
		{"word cut between two segments (real whisper-server output)",
			[]Segment{
				{Start: 0, End: 3.56, Text: " Arztbrief vom 12. November 2026."},
				{Start: 4.14, End: 8.33, Text: " Kniegelenk rechts, leichter Erguss, Ibuprofen 400 mg dre"},
				{Start: 8.33, End: 9.58, Text: "imal täglich."},
			},
			"Arztbrief vom 12. November 2026. Kniegelenk rechts, leichter Erguss, Ibuprofen 400 mg dreimal täglich."},
		{"first segment without space stays", []Segment{{Text: "Puls 64."}}, "Puls 64."},
		{"loop between other text",
			segs("Blutdruck 122 zu 79.", "Danke.", "Danke.", "Danke.", "Puls 64."),
			"Blutdruck 122 zu 79. Danke. Puls 64."},
	}
	for _, tt := range tests {
		if got := CleanTranscript(tt.in); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

func TestTextContains(t *testing.T) {
	text := "Rezept vom 18. Mai 2026: Musterol 400 mg, 1-0-1. Puls 164, TSH 1,8 mU/l. 101 Stück, keiner Guss, 20mg."
	tests := []struct {
		term string
		want bool
	}{
		{"Musterol", true},
		{"musterol", true},
		{"400 mg", true},
		{"18. Mai 2026", true}, // across punctuation at the edges
		{"1-0-1", true},
		{"TSH 1.8", true}, // decimal comma = point
		{"mU/l", true},
		{"64", false},          // not inside 164
		{"Mai 20", false},      // a word must match completely
		{"1 0 1", false},       // word boundaries count …
		{"kein Erguss", false}, // … in both directions
		{"20 mg", false},
		{"Ibuprofen", false},
		{"", false},
		{" . ", false},
	}
	for _, tt := range tests {
		if got := TextContains(text, tt.term); got != tt.want {
			t.Errorf("TextContains(%q) = %v, want %v", tt.term, got, tt.want)
		}
	}
}

func TestWordErrorRate(t *testing.T) {
	tests := []struct {
		name     string
		ref, hyp string
		want     float64
	}{
		{"identical", "Ibuprofen 400 mg dreimal täglich", "Ibuprofen 400 mg dreimal täglich", 0},
		{"case and punctuation do not count", "Kniegelenk rechts, leichter Erguss.", "kniegelenk rechts leichter erguss", 0},
		{"decimal comma = point", "TSH 1,8", "TSH 1.8", 0},
		{"one word replaced", "Ibuprofen 400 mg dreimal täglich", "Ibuprofen 400 mg zweimal täglich", 0.2},
		{"one word missing", "Ibuprofen 400 mg dreimal täglich", "Ibuprofen 400 dreimal täglich", 0.2},
		{"one word inserted", "Ibuprofen 400 mg dreimal täglich", "Ibuprofen 400 mg dreimal täglich bitte", 0.2},
		{"everything invented", "", "Untertitel im Auftrag des ZDF", 1},
		{"both empty", "", "", 0},
		{"nothing recognized", "Puls 64", "", 1},
		{"more errors than reference words", "Puls", "Untertitel im Auftrag", 3},
	}
	for _, tt := range tests {
		if got := WordErrorRate(tt.ref, tt.hyp); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}
