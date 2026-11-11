package ocr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

// words builds the words of one page: one line per string, Line from 0.
func words(lines ...string) []core.OCRWord {
	var out []core.OCRWord
	for l, line := range lines {
		for i, w := range strings.Fields(line) {
			out = append(out, core.OCRWord{Text: w, Confidence: 80, Line: l,
				Box: core.Box{X: 0.1 * float64(i), Y: 0.05 * float64(l), W: 0.08, H: 0.02}})
		}
	}
	return out
}

type fakeEngine struct {
	page   Page
	err    error
	images [][]byte
}

func (e *fakeEngine) RecognizeImage(_ context.Context, image []byte) (Page, error) {
	e.images = append(e.images, image)
	return e.page, e.err
}
func (e *fakeEngine) Name() string { return "engine 1.0 deu" }

type fakePDF struct {
	pages    []Page
	err      error
	rendered []int
	maxPages int
}

func (p *fakePDF) TextLayer(_ context.Context, _ []byte, maxPages int) ([]Page, error) {
	p.maxPages = maxPages
	if p.err != nil {
		return nil, p.err
	}
	return p.pages[:min(len(p.pages), maxPages)], nil
}
func (p *fakePDF) RenderPage(_ context.Context, _ []byte, n int) ([]byte, error) {
	p.rendered = append(p.rendered, n)
	return []byte("png"), nil
}
func (p *fakePDF) Name() string { return "pdftotext 1.0" }

var digital = Page{Width: 595, Height: 842, Words: words("Labor Muster GmbH", "Befunddatum 14.10.2026")}

func TestPipeline_Image(t *testing.T) {
	e := &fakeEngine{page: Page{Width: 1000, Height: 1400, Words: words("Hb 14,2 g/dl")}}
	p := &Pipeline{Engine: e}
	r, err := p.Recognize(context.Background(), []byte("img"), core.MimePNG)
	if err != nil {
		t.Fatal(err)
	}
	if r.Engine != "engine 1.0 deu" || len(r.Pages) != 1 || r.Pages[0].Source != core.SourceOCR {
		t.Errorf("engine %q, pages %+v", r.Engine, r.Pages)
	}
	for _, w := range r.Words {
		if w.Page != 1 || w.Source != core.SourceOCR || w.Confidence != 80 {
			t.Errorf("word %+v", w)
		}
	}
}

func TestPipeline_DigitalPDF(t *testing.T) {
	e := &fakeEngine{}
	pdf := &fakePDF{pages: []Page{digital}}
	r, err := (&Pipeline{Engine: e, PDF: pdf}).Recognize(context.Background(), []byte("%PDF"), core.MimePDF)
	if err != nil {
		t.Fatal(err)
	}
	if r.Engine != "pdftotext 1.0" || len(pdf.rendered) != 0 || len(e.images) != 0 {
		t.Errorf("engine %q, rendered %v, OCR calls %d; want text layer only", r.Engine, pdf.rendered, len(e.images))
	}
	if pdf.maxPages != DefaultMaxPages+1 {
		t.Errorf("asked for %d pages, want %d", pdf.maxPages, DefaultMaxPages+1)
	}
	for _, w := range r.Words {
		if w.Source != core.SourcePDFText || w.Confidence != 0 {
			t.Errorf("word %+v", w)
		}
	}
}

// TestPipeline_MixedPDF: page 1 digital, page 2 scanned (no text layer),
// page 3 with only a short stamp.
func TestPipeline_MixedPDF(t *testing.T) {
	e := &fakeEngine{page: Page{Width: 3508, Height: 2480, Words: words("Hb 14,2 g/dl", "CRP <0,5 mg/dl")}}
	stamp := Page{Width: 595, Height: 842, Words: words("Seite 3")}
	pdf := &fakePDF{pages: []Page{digital, {Width: 595, Height: 842}, stamp}}
	r, err := (&Pipeline{Engine: e, PDF: pdf}).Recognize(context.Background(), []byte("%PDF"), core.MimePDF)
	if err != nil {
		t.Fatal(err)
	}
	if got := pdf.rendered; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("rendered pages %v, want [2 3]", got)
	}
	if r.Engine != "pdftotext 1.0, engine 1.0 deu" {
		t.Errorf("engine %q", r.Engine)
	}
	wantSources := []string{core.SourcePDFText, core.SourceOCR, core.SourceOCR}
	for i, p := range r.Pages {
		if p.Number != i+1 || p.Source != wantSources[i] {
			t.Errorf("page %d: %+v", i+1, p)
		}
	}
	// Lines continue over pages: page 1 has lines 0–1, page 2 lines 2–3,
	// page 3 lines 4–5.
	last := map[int]int{}
	for _, w := range r.Words {
		last[w.Page] = max(last[w.Page], w.Line)
	}
	if last[1] != 1 || last[2] != 3 || last[3] != 5 {
		t.Errorf("last line per page = %v", last)
	}
}

func TestPipeline_ForceOCR(t *testing.T) {
	e := &fakeEngine{page: Page{Width: 1, Height: 1, Words: words("Labor")}}
	pdf := &fakePDF{pages: []Page{digital}}
	r, err := (&Pipeline{Engine: e, PDF: pdf, ForceOCR: true}).Recognize(context.Background(), nil, core.MimePDF)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdf.rendered) != 1 || r.Engine != "engine 1.0 deu" || r.Pages[0].Source != core.SourceOCR {
		t.Errorf("rendered %v, engine %q, pages %+v", pdf.rendered, r.Engine, r.Pages)
	}
}

func TestPipeline_Errors(t *testing.T) {
	many := make([]Page, 12)
	for i := range many {
		many[i] = digital
	}
	scanned := []Page{{Width: 595, Height: 842}}
	broken := errors.New("syntax error")
	tests := []struct {
		name   string
		p      *Pipeline
		mime   string
		reason string
	}{
		{"HEIC", &Pipeline{Engine: &fakeEngine{}, PDF: &fakePDF{}}, core.MimeHEIC, core.ReasonUnsupportedFileType},
		{"image without engine", &Pipeline{PDF: &fakePDF{}}, core.MimeJPEG, core.ReasonOCRUnavailable},
		{"PDF without poppler", &Pipeline{Engine: &fakeEngine{}}, core.MimePDF, core.ReasonOCRUnavailable},
		{"scan without engine", &Pipeline{PDF: &fakePDF{pages: scanned}}, core.MimePDF, core.ReasonOCRUnavailable},
		{"too many pages", &Pipeline{Engine: &fakeEngine{}, PDF: &fakePDF{pages: many}}, core.MimePDF, core.ReasonTooManyPages},
		{"own limit", &Pipeline{Engine: &fakeEngine{}, PDF: &fakePDF{pages: many[:3]}, MaxPages: 2}, core.MimePDF, core.ReasonTooManyPages},
		{"damaged PDF", &Pipeline{Engine: &fakeEngine{}, PDF: &fakePDF{err: broken}}, core.MimePDF, core.ReasonOCRFailed},
		{"engine fails", &Pipeline{Engine: &fakeEngine{err: broken}}, core.MimePNG, core.ReasonOCRFailed},
		{"time limit", &Pipeline{Engine: &fakeEngine{err: context.DeadlineExceeded}}, core.MimePNG, core.ReasonOCRFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.p.Recognize(context.Background(), nil, tt.mime)
			if err == nil {
				t.Fatal("no error")
			}
			if got := Reason(err); got != tt.reason {
				t.Errorf("reason %q, want %q (error: %v)", got, tt.reason, err)
			}
		})
	}
	// Exactly at the limit is allowed.
	if _, err := (&Pipeline{PDF: &fakePDF{pages: many[:10]}}).Recognize(context.Background(), nil, core.MimePDF); err != nil {
		t.Errorf("10 pages: %v", err)
	}
}

func TestHasTextLayer(t *testing.T) {
	tests := map[string]bool{
		"":                           false,
		"Seite 1 von 2":              false, // 10 characters
		"12345678901234567 89":       false, // 19
		"12345678901234567 890":      true,  // 20
		"Labor Muster GmbH Salzburg": true,
	}
	for text, want := range tests {
		if got := HasTextLayer(Page{Words: words(text)}); got != want {
			t.Errorf("%q: %v, want %v", text, got, want)
		}
	}
}
