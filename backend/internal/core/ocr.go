package core

import "unicode"

// OCRResult is what the second reader read from a file: the text layer of
// a PDF, or OCR for photos and scanned pages. The interface in package ocr
// returns this type, whichever engine is behind it, just as the model
// interface returns Extraction.
type OCRResult struct {
	Engine string    // programs with version, e.g. "pdftotext 26.11.0" or "tesseract 5.5.4 deu"
	Pages  []OCRPage // one per page; a photo has one page
	Words  []OCRWord // in reading order
}

// OCRPage describes a page as it was read. Positions in OCRWord do not
// depend on its size (fractions of the page); the size is only kept for
// diagnostics.
type OCRPage struct {
	Number        int     // starting at 1
	Source        string  // SourcePDFText or SourceOCR
	Width, Height float64 // pixels (OCR) or PDF points (text layer)
}

// OCRWord is a recognized word with its position.
//
// For a file, Page starts at 1 and Box holds the position. For a text
// import (WordsFromText), Page is 0 and Start/End hold the range in the
// text instead.
type OCRWord struct {
	Text       string
	Source     string  // SourcePDFText, SourceOCR or SourceText
	Confidence float64 // 0 … 100, only for SourceOCR
	Page       int
	Line       int // increases in reading order; words with the same Line are in the same line
	Box        Box
	Start, End int // code point offsets, End exclusive (text import only)
}

// Box is a rectangle as fractions of the page width and height (0 … 1,
// origin top left). Fractions instead of pixels, so the app can use it at
// any display size.
type Box struct {
	X, Y, W, H float64
}

// union returns the smallest box that contains both.
func (b Box) union(o Box) Box {
	x0, y0 := min(b.X, o.X), min(b.Y, o.Y)
	x1, y1 := max(b.X+b.W, o.X+o.W), max(b.Y+b.H, o.Y+o.H)
	return Box{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// WordsFromText splits an entered text (text import) into words, so that
// the same comparison works against text as against OCR. Words are
// separated by whitespace; Start and End count code points (runes), not
// bytes, so "ü" counts as one. Every line break starts a new Line.
func WordsFromText(text string) []OCRWord {
	words := []OCRWord{}
	line, pos, start := 0, 0, -1
	var current []rune

	flush := func() {
		if start >= 0 {
			words = append(words, OCRWord{Text: string(current), Source: SourceText, Line: line, Start: start, End: pos})
			current, start = current[:0], -1
		}
	}
	for _, r := range text {
		if unicode.IsSpace(r) {
			flush()
			if r == '\n' {
				line++
			}
		} else {
			if start < 0 {
				start = pos
			}
			current = append(current, r)
		}
		pos++
	}
	flush()
	return words
}
