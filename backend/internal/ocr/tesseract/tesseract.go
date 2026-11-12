// Package tesseract implements ocr.Engine with the Tesseract command line
// program (open source, runs locally). The image goes in via stdin, the
// words come out as TSV via stdout; no temporary file is written.
//
// Install: macOS `brew install tesseract tesseract-lang`, Debian/Ubuntu
// `apt install tesseract-ocr tesseract-ocr-deu`.
package tesseract

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/ocr"
)

// DefaultLanguage: German, because the documents are German.
const DefaultLanguage = "deu"

// Tesseract implements ocr.Engine. It reads the image as it gets it:
// photos are prepared (upright, sized, grey) by ocr.PrepareImage first.
type Tesseract struct {
	path, lang, version string
}

// New finds tesseract in PATH and checks that the language pack is
// installed. If not, the error is ocr.ErrUnavailable.
func New(lang string) (*Tesseract, error) {
	if lang == "" {
		lang = DefaultLanguage
	}
	path, err := exec.LookPath("tesseract")
	if err != nil {
		return nil, fmt.Errorf("tesseract not found: %w", ocr.ErrUnavailable)
	}
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tesseract --version: %v: %w", err, ocr.ErrUnavailable)
	}
	version := parseVersion(string(out))

	out, err = exec.Command(path, "--list-langs").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tesseract --list-langs: %v: %w", err, ocr.ErrUnavailable)
	}
	if !hasLanguage(string(out), lang) {
		return nil, fmt.Errorf("tesseract language %q not installed: %w", lang, ocr.ErrUnavailable)
	}
	return &Tesseract{path: path, lang: lang, version: version}, nil
}

// parseVersion takes "5.5.4" from "tesseract 5.5.4\n leptonica-1.87.0 …".
func parseVersion(out string) string {
	first, _, _ := strings.Cut(out, "\n")
	if v, ok := strings.CutPrefix(strings.TrimSpace(first), "tesseract "); ok {
		return strings.TrimSpace(v)
	}
	return "unknown"
}

// hasLanguage: is lang one of the lines after the heading of --list-langs?
func hasLanguage(out, lang string) bool {
	lines := strings.Split(out, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return slices.Contains(lines, lang)
}

// Name is e.g. "tesseract 5.5.4 deu".
func (t *Tesseract) Name() string { return "tesseract " + t.version + " " + t.lang }

// DPI is passed to Tesseract for every image. The prepared PNGs carry no
// resolution, and Tesseract then estimates one: on the real phone photo it
// guessed 441 dpi and misread a unit and dropped a column, while 200, 300
// and 400 all read it completely (docs/ocr/README.md). 300 is exact for an
// A4 page rendered at 3508 pixels and close for the prepared photos.
const DPI = 300

// RecognizeImage runs `tesseract stdin stdout -l <lang> --dpi 300 tsv`.
func (t *Tesseract) RecognizeImage(ctx context.Context, image []byte) (ocr.Page, error) {
	cmd := exec.CommandContext(ctx, t.path, "stdin", "stdout", "-l", t.lang, "--dpi", strconv.Itoa(DPI), "tsv")
	cmd.Stdin = bytes.NewReader(image)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ocr.Page{}, ctx.Err()
		}
		// stderr holds messages like "Error during processing.", no text
		// from the image.
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return ocr.Page{}, fmt.Errorf("tesseract: %v: %s", err, msg)
	}
	return parseTSV(stdout.String())
}

// TSV columns (Tesseract 4 and 5).
const (
	colLevel = iota
	colPage
	colBlock
	colPar
	colLine
	colWord
	colLeft
	colTop
	colWidth
	colHeight
	colConf
	colText
)

// parseTSV turns Tesseract's TSV into a page. Row level 1 is the page
// (its size), level 5 a word. A word that is only whitespace (Tesseract
// reports ruled lines like that) is skipped. Every (block, paragraph,
// line) gets its own Line number in the order of appearance.
func parseTSV(tsv string) (ocr.Page, error) {
	page := ocr.Page{Words: []core.OCRWord{}}
	lines := map[[3]string]int{}

	for i, row := range strings.Split(tsv, "\n") {
		if i == 0 || strings.TrimSpace(row) == "" {
			continue // header
		}
		f := strings.Split(strings.TrimRight(row, "\r"), "\t")
		if len(f) < colText {
			return ocr.Page{}, fmt.Errorf("tesseract tsv row %d: %d columns", i+1, len(f))
		}
		switch f[colLevel] {
		case "1":
			w, errW := strconv.ParseFloat(f[colWidth], 64)
			h, errH := strconv.ParseFloat(f[colHeight], 64)
			if errW != nil || errH != nil || w <= 0 || h <= 0 {
				return ocr.Page{}, fmt.Errorf("tesseract tsv row %d: invalid page size", i+1)
			}
			page.Width, page.Height = w, h
		case "5":
			if len(f) <= colText || strings.TrimSpace(f[colText]) == "" {
				continue
			}
			if page.Width == 0 {
				return ocr.Page{}, fmt.Errorf("tesseract tsv row %d: word before page size", i+1)
			}
			nums, err := floats(f[colLeft:colText])
			if err != nil {
				return ocr.Page{}, fmt.Errorf("tesseract tsv row %d: %w", i+1, err)
			}
			left, top, width, height, conf := nums[0], nums[1], nums[2], nums[3], nums[4]

			key := [3]string{f[colBlock], f[colPar], f[colLine]}
			line, ok := lines[key]
			if !ok {
				line = len(lines)
				lines[key] = line
			}
			page.Words = append(page.Words, core.OCRWord{
				Text:       strings.TrimSpace(f[colText]),
				Source:     core.SourceOCR,
				Confidence: max(conf, 0),
				Line:       line,
				Box: core.Box{
					X: left / page.Width, Y: top / page.Height,
					W: width / page.Width, H: height / page.Height,
				},
			})
		}
	}
	if page.Width == 0 {
		return ocr.Page{}, fmt.Errorf("tesseract tsv: no page row")
	}
	return page, nil
}

func floats(fields []string) ([]float64, error) {
	out := make([]float64, len(fields))
	for i, s := range fields {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("not a number: column %d", colLeft+i+1)
		}
		out[i] = v
	}
	return out, nil
}
