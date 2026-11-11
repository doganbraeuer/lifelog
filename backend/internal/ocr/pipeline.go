package ocr

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"lifelog/backend/internal/core"
)

const (
	// DefaultMaxPages: a PDF with more pages is not checked
	// (core.ReasonTooManyPages). Reading 10 scanned pages takes about 10 s.
	DefaultMaxPages = 10

	// MinTextLayerChars: a PDF page counts as having a text layer if its
	// text layer holds at least this many characters (without whitespace).
	// A scanned page has none, or only a short stamp.
	MinTextLayerChars = 20
)

// Pipeline reads a file with the text layer where possible and OCR where
// needed. It implements Recognizer.
type Pipeline struct {
	Engine   Engine // nil: no OCR; photos and scanned pages give ErrUnavailable
	PDF      PDF    // nil: PDFs give ErrUnavailable
	MaxPages int    // 0: DefaultMaxPages

	// Image: how photos are prepared before OCR (PrepareImage). The zero
	// value only turns them upright and converts them; the server uses
	// DefaultImageOptions.
	Image ImageOptions

	// ForceOCR ignores the text layer and reads every PDF page with OCR.
	// Only for measuring the engine (cmd/ocrcheck), never in the server.
	ForceOCR bool
}

// Recognize reads the file. Pages are numbered from 1, lines increase over
// all pages, so that a line on page 2 never equals one on page 1.
func (p *Pipeline) Recognize(ctx context.Context, data []byte, mimeType string) (core.OCRResult, error) {
	switch mimeType {
	case core.MimeJPEG, core.MimePNG, core.MimeHEIC, core.MimeWebP:
		if p.Engine == nil {
			return core.OCRResult{}, ErrUnavailable
		}
		image, err := PrepareImage(data, mimeType, p.Image)
		if err != nil {
			return core.OCRResult{}, err
		}
		page, err := p.Engine.RecognizeImage(ctx, image)
		if err != nil {
			return core.OCRResult{}, err
		}
		var r core.OCRResult
		addPage(&r, page, core.SourceOCR)
		r.Engine = p.Engine.Name()
		return r, nil
	case core.MimePDF:
		return p.recognizePDF(ctx, data)
	}
	return core.OCRResult{}, fmt.Errorf("%s: %w", mimeType, ErrUnsupported)
}

func (p *Pipeline) recognizePDF(ctx context.Context, data []byte) (core.OCRResult, error) {
	if p.PDF == nil {
		return core.OCRResult{}, ErrUnavailable
	}
	maxPages := p.MaxPages
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	// One page more than allowed, so we know whether there are too many.
	pages, err := p.PDF.TextLayer(ctx, data, maxPages+1)
	if err != nil {
		return core.OCRResult{}, err
	}
	if len(pages) > maxPages {
		return core.OCRResult{}, fmt.Errorf("more than %d pages: %w", maxPages, ErrTooManyPages)
	}

	var r core.OCRResult
	usedText, usedOCR := false, false
	for i, page := range pages {
		if !p.ForceOCR && HasTextLayer(page) {
			addPage(&r, page, core.SourcePDFText)
			usedText = true
			continue
		}
		if p.Engine == nil {
			return core.OCRResult{}, fmt.Errorf("page %d has no text layer: %w", i+1, ErrUnavailable)
		}
		// Rendered pages are already upright PNGs of a fixed size
		// (poppler.RenderLongSide) and need no preparing.
		image, err := p.PDF.RenderPage(ctx, data, i+1)
		if err != nil {
			return core.OCRResult{}, fmt.Errorf("render page %d: %w", i+1, err)
		}
		ocrPage, err := p.Engine.RecognizeImage(ctx, image)
		if err != nil {
			return core.OCRResult{}, fmt.Errorf("page %d: %w", i+1, err)
		}
		addPage(&r, ocrPage, core.SourceOCR)
		usedOCR = true
	}

	var engines []string
	if usedText {
		engines = append(engines, p.PDF.Name())
	}
	if usedOCR {
		engines = append(engines, p.Engine.Name())
	}
	r.Engine = strings.Join(engines, ", ")
	return r, nil
}

// HasTextLayer: does the page hold at least MinTextLayerChars characters
// (without whitespace)?
func HasTextLayer(page Page) bool {
	n := 0
	for _, w := range page.Words {
		for _, c := range w.Text {
			if !unicode.IsSpace(c) {
				n++
			}
		}
	}
	return n >= MinTextLayerChars
}

// addPage appends a page to the result: page number, source, and line
// numbers continued after the previous page.
func addPage(r *core.OCRResult, page Page, source string) {
	number := len(r.Pages) + 1
	r.Pages = append(r.Pages, core.OCRPage{Number: number, Source: source, Width: page.Width, Height: page.Height})

	offset := 0
	for _, w := range r.Words {
		offset = max(offset, w.Line+1)
	}
	for _, w := range page.Words {
		w.Page = number
		w.Source = source
		w.Line += offset
		if source != core.SourceOCR {
			w.Confidence = 0
		}
		r.Words = append(r.Words, w)
	}
}
