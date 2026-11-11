package core

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ChunkMaxChars is the upper limit for a chunk, counted in characters
// (runes), not bytes. "ä" is one character but two bytes.
//
// Why 500? Smaller chunks make the source citation more precise (the excerpt
// shows the matching passage instead of the whole document). Larger ones give
// the language model more context. 500 is a starting value for the prototype
// that we adjust with real (synthetic) documents.
const ChunkMaxChars = 500

// blankLines separates paragraphs: a line break, then at least one line that
// is empty or contains only whitespace.
var blankLines = regexp.MustCompile(`\n[ \t]*\n\s*`)

// piece is a piece of text plus the separator that preceded it in the
// original.
type piece struct {
	sep  string // "\n\n" between paragraphs, "\n" between lines, " " between words, "" after a hard cut
	text string
}

// SplitIntoChunks splits a full text into chunks for search.
//
//  1. Paragraphs (separated by blank lines) are the natural units.
//  2. A paragraph that is too long is split at lines, a line that is too
//     long at words, a word that is too long hard after max characters.
//  3. The pieces are merged in order as long as the chunk has at most max
//     characters.
//
// No chunk is empty or longer than max. Empty text gives an empty list (not
// nil). The same text always gives the same chunks.
func SplitIntoChunks(fullText string, max int) []string {
	if max < 1 {
		max = ChunkMaxChars
	}
	text := strings.ReplaceAll(fullText, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var pieces []piece
	for _, paragraph := range blankLines.Split(text, -1) {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		pieces = append(pieces, split(paragraph, "\n\n", max)...)
	}
	return merge(pieces, max)
}

// split breaks text down until every piece has at most max characters. sep is
// the separator before the first piece.
func split(text, sep string, max int) []piece {
	if length(text) <= max {
		return []piece{{sep, text}}
	}

	var parts []string
	var inner string
	switch {
	case strings.Contains(text, "\n"):
		parts, inner = nonEmpty(strings.Split(text, "\n")), "\n"
	case len(strings.Fields(text)) > 1:
		parts, inner = strings.Fields(text), " "
	default:
		return hardCut(text, sep, max)
	}

	var out []piece
	for i, p := range parts {
		s := inner
		if i == 0 {
			s = sep
		}
		out = append(out, split(p, s, max)...)
	}
	return out
}

// hardCut splits a single overlong word (e.g. a long string without spaces)
// after max characters. It cuts at character boundaries, never in the middle
// of an umlaut.
func hardCut(text, sep string, max int) []piece {
	var out []piece
	runes := []rune(text)
	for start := 0; start < len(runes); start += max {
		end := min(start+max, len(runes))
		s := ""
		if start == 0 {
			s = sep
		}
		out = append(out, piece{s, string(runes[start:end])})
	}
	return out
}

func merge(pieces []piece, max int) []string {
	chunks := []string{}
	var current strings.Builder
	currentLen := 0

	for _, p := range pieces {
		textLen := length(p.text)
		if currentLen > 0 && currentLen+length(p.sep)+textLen <= max {
			current.WriteString(p.sep)
			current.WriteString(p.text)
			currentLen += length(p.sep) + textLen
			continue
		}
		if currentLen > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(p.text) // no separator at the start of a chunk
		currentLen = textLen
	}
	if currentLen > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func nonEmpty(lines []string) []string {
	out := lines[:0:0]
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func length(s string) int { return utf8.RuneCountInString(s) }
