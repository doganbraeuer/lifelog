// Package ocr is the second, independent reader of an uploaded file. It
// reads words with their positions, so that the backend can check the
// values the vision LLM extracted (core.Verify, docs/API.md, verification).
//
// Two interfaces keep the programs replaceable, as in package llm:
//
//   - Engine reads one image (OCR). Today Tesseract (subpackage tesseract);
//     PaddleOCR would be a new subpackage and one line in cmd/server.
//   - PDF reads the text layer of a PDF and renders pages as images. Today
//     Poppler (subpackage poppler).
//
// Pipeline combines them: PDF pages with a text layer are read directly,
// pages without one and photos go through the engine. The rest of the
// backend only knows Recognizer.
package ocr

import (
	"context"
	"errors"

	"lifelog/backend/internal/core"
)

// Recognizer reads a whole file (image or PDF) and returns its words.
type Recognizer interface {
	Recognize(ctx context.Context, data []byte, mimeType string) (core.OCRResult, error)
}

// Engine reads the text of one image (PNG or JPEG).
type Engine interface {
	// RecognizeImage returns the words of the image. Line counts from 0
	// within the image; Page is set by the caller; Source is core.SourceOCR.
	RecognizeImage(ctx context.Context, image []byte) (Page, error)
	// Name is program, version and language, e.g. "tesseract 5.5.4 deu".
	Name() string
}

// PDF reads PDF files.
type PDF interface {
	// TextLayer returns the text layer of the first maxPages pages, one Page
	// per page (also for pages without text: then Words is empty). Line
	// counts from 0 within each page; Source is core.SourcePDFText.
	TextLayer(ctx context.Context, pdf []byte, maxPages int) ([]Page, error)
	// RenderPage renders page number (starting at 1) as a PNG image.
	RenderPage(ctx context.Context, pdf []byte, number int) ([]byte, error)
	// Name is program and version, e.g. "pdftotext 26.11.0".
	Name() string
}

// Page is what was read from one page or image. Positions in Words are
// fractions of the page (core.Box), so Width and Height are only kept for
// diagnostics.
type Page struct {
	Width, Height float64
	Words         []core.OCRWord
}

// Kinds of errors. Reason turns them into the reason of an unchecked entry.
var (
	// ErrUnavailable: a needed program is not installed or switched off.
	ErrUnavailable = errors.New("ocr unavailable")
	// ErrUnsupported: this file type cannot be read (e.g. HEIC).
	ErrUnsupported = errors.New("file type not supported by ocr")
	// ErrTooManyPages: the PDF has more pages than the limit.
	ErrTooManyPages = errors.New("too many pages for ocr")
)

// Reason returns the reason for core.Unchecked that matches err. Every
// other error (a damaged PDF, a crash, the time limit) is
// core.ReasonOCRFailed.
func Reason(err error) string {
	switch {
	case errors.Is(err, ErrUnavailable):
		return core.ReasonOCRUnavailable
	case errors.Is(err, ErrUnsupported):
		return core.ReasonUnsupportedFileType
	case errors.Is(err, ErrTooManyPages):
		return core.ReasonTooManyPages
	}
	return core.ReasonOCRFailed
}
