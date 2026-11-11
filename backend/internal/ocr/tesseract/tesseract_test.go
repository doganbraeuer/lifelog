package tesseract

import (
	"context"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

// tsvSample: a page of 1000 × 2000 pixels with a ruled line (whitespace
// word), two words in line 1 and one word in another block.
var tsvSample = strings.Join([]string{
	"level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext",
	"1\t1\t0\t0\t0\t0\t0\t0\t1000\t2000\t-1\t",
	"2\t1\t1\t0\t0\t0\t100\t200\t500\t40\t-1\t",
	"5\t1\t1\t1\t1\t1\t100\t200\t500\t4\t95.000000\t ",
	"4\t1\t2\t1\t1\t0\t100\t400\t300\t40\t-1\t",
	"5\t1\t2\t1\t1\t1\t100\t400\t100\t40\t96.5\tHb",
	"5\t1\t2\t1\t1\t2\t250\t400\t150\t40\t61.2\t14,2",
	"5\t1\t3\t1\t1\t1\t100\t600\t200\t40\t90\tg/dl",
	"",
}, "\n")

func TestParseTSV(t *testing.T) {
	page, err := parseTSV(tsvSample)
	if err != nil {
		t.Fatal(err)
	}
	if page.Width != 1000 || page.Height != 2000 {
		t.Errorf("size %v×%v", page.Width, page.Height)
	}
	var texts []string
	for _, w := range page.Words {
		texts = append(texts, w.Text)
		if w.Source != core.SourceOCR {
			t.Errorf("%q: source %q", w.Text, w.Source)
		}
	}
	if !slices.Equal(texts, []string{"Hb", "14,2", "g/dl"}) {
		t.Fatalf("texts %q (the ruled line must be skipped)", texts)
	}
	w := page.Words
	if w[0].Line != w[1].Line || w[1].Line == w[2].Line {
		t.Errorf("lines %d %d %d: first two together, third apart", w[0].Line, w[1].Line, w[2].Line)
	}
	if w[1].Confidence != 61.2 {
		t.Errorf("confidence %v", w[1].Confidence)
	}
	want := core.Box{X: 0.25, Y: 0.2, W: 0.15, H: 0.02}
	if b := w[1].Box; math.Abs(b.X-want.X) > 1e-9 || math.Abs(b.Y-want.Y) > 1e-9 ||
		math.Abs(b.W-want.W) > 1e-9 || math.Abs(b.H-want.H) > 1e-9 {
		t.Errorf("box %+v, want %+v", b, want)
	}
}

func TestParseTSV_Invalid(t *testing.T) {
	header := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"
	tests := map[string]string{
		"empty":           "",
		"no page row":     header + "5\t1\t1\t1\t1\t1\t1\t1\t1\t1\t90\tHb\n",
		"too few columns": header + "1\t1\t0\n",
		"not a number":    header + "1\t1\t0\t0\t0\t0\t0\t0\t1000\t2000\t-1\t\n5\t1\t1\t1\t1\t1\tx\t1\t1\t1\t90\tHb\n",
		"page size zero":  header + "1\t1\t0\t0\t0\t0\t0\t0\t0\t0\t-1\t\n",
	}
	for name, tsv := range tests {
		if _, err := parseTSV(tsv); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestParseVersionAndLanguage(t *testing.T) {
	if got := parseVersion("tesseract 5.5.4\n leptonica-1.87.0\n"); got != "5.5.4" {
		t.Errorf("version %q", got)
	}
	langs := "List of available languages in \"/opt/homebrew/share/tessdata/\" (3):\ndeu\neng\nosd\n"
	if !hasLanguage(langs, "deu") || hasLanguage(langs, "fra") || hasLanguage(langs, "de") {
		t.Error("hasLanguage wrong")
	}
}

// Integration with the installed Tesseract, on the synthetic lab report.
func TestRecognizeImage_LabReport(t *testing.T) {
	tess, err := New(DefaultLanguage)
	if err != nil {
		t.Skipf("Tesseract with deu not installed: %v", err)
	}
	if !strings.HasPrefix(tess.Name(), "tesseract ") || !strings.HasSuffix(tess.Name(), " deu") {
		t.Errorf("name %q", tess.Name())
	}
	img, err := os.ReadFile("../../../testdata/lab_report.png")
	if err != nil {
		t.Fatal(err)
	}
	// This tests the call and the TSV, not the scaling: the image has only
	// 612 × 792 pixels (72 dpi), far below real photos, and any enlargement
	// garbles parts of it. The scaling is measured with cmd/ocrcheck on the
	// demo photos, and internal/ocr's end-to-end test uses the default.
	tess.MinLongSide = 0
	page, err := tess.RecognizeImage(context.Background(), img)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, w := range page.Words {
		texts = append(texts, w.Text)
	}
	for _, want := range []string{"Labor", "Befunddatum:", "14.10.2026"} {
		if !slices.Contains(texts, want) {
			t.Errorf("%q not recognized; got %q", want, texts)
		}
	}
}

func TestNew_UnknownLanguage(t *testing.T) {
	if _, err := New(DefaultLanguage); err != nil {
		t.Skip("Tesseract not installed")
	}
	if _, err := New("xx_none"); err == nil {
		t.Error("no error for a missing language pack")
	}
}
