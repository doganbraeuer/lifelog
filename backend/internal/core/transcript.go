package core

import (
	"strings"
	"unicode"
)

// Segment is one piece of a recognized recording, with its time in seconds.
type Segment struct {
	Start, End float64
	Text       string
}

// Transcript is the result of a transcription (docs/API.md, 3.9). Nothing
// of it is stored.
type Transcript struct {
	Text            string
	DurationSeconds float64
	Language        string // always "de"
	Engine          string // e.g. "whisper.cpp large-v3-turbo"
}

// TranscriptLanguage: recordings are always German.
const TranscriptLanguage = "de"

// hallucinationPrefixes are sentences Whisper is known to invent in silence
// or noise: it was trained on subtitled videos, whose credits end up in the
// output ("Untertitel im Auftrag des ZDF, 2017"). Compared as normalized
// keys (normalizeKey); a segment is dropped if it starts with one of them
// and is short, so a long real sentence is never removed.
var hallucinationPrefixes = []string{
	"Untertitel",    // "Untertitel im Auftrag des ZDF", "Untertitel der Amara.org-Community"
	"Untertitelung", // "Untertitelung des ZDF für funk, 2017"
	"Vielen Dank fürs Zuschauen",
	"Vielen Dank für's Zuschauen",
	"Danke fürs Zuschauen",
	"Copyright",
	"©",
}

// maxHallucinationWords: only segments up to this many words can be
// dropped as a known invention.
const maxHallucinationWords = 12

// minRepeats: this many identical segments in a row are folded into one.
// Whisper can get stuck in a loop and repeat a sentence; a person rarely
// says the same sentence three times in a row.
const minRepeats = 3

// CleanTranscript joins the segments into one text, without known
// inventions and without loops (see above). Segments are separated by a
// space, except when a segment does not start with whitespace: Whisper
// marks the start of a word with a space, so such a segment continues the
// word of the one before (the voice activity detection can cut inside a
// word: "… 400 mg dre" + "imal täglich."). The result is empty if nothing
// is left: the caller answers NO_SPEECH then.
func CleanTranscript(segments []Segment) string {
	var pieces []string
	for _, s := range segments {
		t := strings.Join(strings.Fields(s.Text), " ")
		if t == "" {
			continue
		}
		if r := []rune(s.Text); len(pieces) > 0 && !unicode.IsSpace(r[0]) {
			pieces[len(pieces)-1] += t
			continue
		}
		pieces = append(pieces, t)
	}

	var texts []string
	for _, t := range pieces {
		if !isHallucination(t) {
			texts = append(texts, t)
		}
	}

	var out []string
	for i := 0; i < len(texts); {
		j := i
		for j < len(texts) && normalizeKey(texts[j]) == normalizeKey(texts[i]) {
			j++
		}
		n := j - i
		if n >= minRepeats {
			n = 1
		}
		for k := 0; k < n; k++ {
			out = append(out, texts[i])
		}
		i = j
	}
	return strings.Join(out, " ")
}

func isHallucination(text string) bool {
	if len(strings.Fields(text)) > maxHallucinationWords {
		return false
	}
	key := normalizeKey(text)
	for _, p := range hallucinationPrefixes {
		if strings.HasPrefix(key, normalizeKey(p)) {
			return true
		}
	}
	return false
}

// WordErrorRate compares a recognized text with the reference: the number
// of words that must be replaced, deleted or inserted (Levenshtein distance
// on words), divided by the number of reference words. Words are
// normalized like in the verification (lowercase, punctuation at the
// edges, decimal comma). 0 is perfect; it can be above 1 if much is
// invented. For the measurement (cmd/speechcheck).
func WordErrorRate(reference, hypothesis string) float64 {
	ref, hyp := words(reference), words(hypothesis)
	if len(ref) == 0 {
		if len(hyp) == 0 {
			return 0
		}
		return 1
	}
	prev := make([]int, len(hyp)+1)
	cur := make([]int, len(hyp)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ref); i++ {
		cur[0] = i
		for j := 1; j <= len(hyp); j++ {
			cost := 1
			if ref[i-1] == hyp[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return float64(prev[len(hyp)]) / float64(len(ref))
}

// words splits into normalized words and drops empty ones (a lone "-").
func words(s string) []string {
	var out []string
	for _, w := range strings.Fields(s) {
		if n := normalizeWord(w); n != "" {
			out = append(out, n)
		}
	}
	return out
}
