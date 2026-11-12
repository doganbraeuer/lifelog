// Package poppler reads PDFs with the Poppler command line tools:
// pdftotext for the text layer, pdftoppm for rendering a page as an image.
// Both run as external programs without a shell; the PDF goes in via stdin
// and the result comes out via stdout, so no temporary file with health
// data is written.
//
// Install: macOS `brew install poppler`, Debian/Ubuntu
// `apt install poppler-utils`.
package poppler

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/ocr"
)

// RenderLongSide: a rendered page is this many pixels on its long side.
// 3508 is A4 at 300 dpi, a good resolution for Tesseract. A fixed size
// instead of a fixed dpi keeps large pages from taking too long.
const RenderLongSide = 3508

// Poppler implements ocr.PDF.
type Poppler struct {
	pdftotext, pdftoppm string
	version             string
}

// New finds pdftotext and pdftoppm in PATH. If one is missing, the error
// is ocr.ErrUnavailable.
func New() (*Poppler, error) {
	text, err := exec.LookPath("pdftotext")
	if err != nil {
		return nil, fmt.Errorf("pdftotext not found: %w", ocr.ErrUnavailable)
	}
	ppm, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, fmt.Errorf("pdftoppm not found: %w", ocr.ErrUnavailable)
	}
	// "pdftotext version 26.11.0" goes to stderr.
	out, err := exec.Command(text, "-v").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdftotext -v: %v: %w", err, ocr.ErrUnavailable)
	}
	return &Poppler{pdftotext: text, pdftoppm: ppm, version: parseVersion(string(out))}, nil
}

// parseVersion takes "26.11.0" from "pdftotext version 26.11.0\n…".
func parseVersion(out string) string {
	first, _, _ := strings.Cut(out, "\n")
	if _, v, ok := strings.Cut(first, "version "); ok {
		return strings.TrimSpace(v)
	}
	return "unknown"
}

// Name is e.g. "pdftotext 26.11.0".
func (p *Poppler) Name() string { return "pdftotext " + p.version }

// TextLayer runs `pdftotext -bbox-layout` on the first maxPages pages.
func (p *Poppler) TextLayer(ctx context.Context, pdf []byte, maxPages int) ([]ocr.Page, error) {
	out, err := run(ctx, pdf, p.pdftotext,
		"-q", "-bbox-layout", "-f", "1", "-l", strconv.Itoa(maxPages), "-", "-")
	if err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}
	return parseBBoxLayout(out)
}

// RenderPage runs pdftoppm for one page and returns it as PNG.
func (p *Poppler) RenderPage(ctx context.Context, pdf []byte, number int) ([]byte, error) {
	n := strconv.Itoa(number)
	out, err := run(ctx, pdf, p.pdftoppm,
		"-q", "-png", "-scale-to", strconv.Itoa(RenderLongSide), "-f", n, "-l", n, "-singlefile", "-")
	if err != nil {
		return nil, fmt.Errorf("pdftoppm: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pdftoppm: page %d: no image", number)
	}
	return out, nil
}

// run starts the program with stdin and returns stdout. On an error, the
// beginning of stderr goes into the error (Poppler only writes messages
// like "Syntax Error" there, never document content).
func run(ctx context.Context, stdin []byte, program string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("%v: %s", err, msg)
	}
	return stdout.Bytes(), nil
}

// The XHTML from pdftotext -bbox-layout:
//
//	<html><body><doc>
//	  <page width="595.27" height="841.88">
//	    <flow><block><line>
//	      <word xMin="56.69" yMin="56.63" xMax="104.81" yMax="71.63">Labor</word>
//
// Coordinates are PDF points with the origin top left.
type bboxHTML struct {
	Pages []bboxPage `xml:"body>doc>page"`
}

type bboxPage struct {
	Width  float64    `xml:"width,attr"`
	Height float64    `xml:"height,attr"`
	Flows  []bboxFlow `xml:"flow"`
}

type bboxFlow struct {
	Blocks []struct {
		Lines []struct {
			Words []bboxWord `xml:"word"`
		} `xml:"line"`
	} `xml:"block"`
}

type bboxWord struct {
	XMin float64 `xml:"xMin,attr"`
	YMin float64 `xml:"yMin,attr"`
	XMax float64 `xml:"xMax,attr"`
	YMax float64 `xml:"yMax,attr"`
	Text string  `xml:",chardata"`
}

// parseBBoxLayout turns the XHTML into pages with words. Positions become
// fractions of the page; every <line> gets its own Line number.
func parseBBoxLayout(out []byte) ([]ocr.Page, error) {
	var doc bboxHTML
	d := xml.NewDecoder(bytes.NewReader(out))
	d.Entity = xml.HTMLEntity
	if err := d.Decode(&doc); err != nil {
		return nil, fmt.Errorf("pdftotext output: %w", err)
	}

	pages := make([]ocr.Page, 0, len(doc.Pages))
	for _, bp := range doc.Pages {
		page := ocr.Page{Width: bp.Width, Height: bp.Height, Words: []core.OCRWord{}}
		if bp.Width <= 0 || bp.Height <= 0 {
			pages = append(pages, page) // no usable size: treat as page without text
			continue
		}
		line := 0
		for _, f := range bp.Flows {
			for _, b := range f.Blocks {
				for _, l := range b.Lines {
					for _, w := range l.Words {
						text := strings.TrimSpace(w.Text)
						if text == "" {
							continue
						}
						page.Words = append(page.Words, core.OCRWord{
							Text:   text,
							Source: core.SourcePDFText,
							Line:   line,
							Box: core.Box{
								X: w.XMin / bp.Width,
								Y: w.YMin / bp.Height,
								W: (w.XMax - w.XMin) / bp.Width,
								H: (w.YMax - w.YMin) / bp.Height,
							},
						})
					}
					line++
				}
			}
		}
		pages = append(pages, page)
	}
	return pages, nil
}
