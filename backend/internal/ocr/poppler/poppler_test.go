package poppler

import (
	"bytes"
	"context"
	"image/png"
	"math"
	"os"
	"slices"
	"testing"

	"lifelog/backend/internal/core"
)

// bboxSample is shortened output of pdftotext -bbox-layout: page 1 with two
// lines (one in a second block), page 2 without text.
const bboxSample = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"><html xmlns="http://www.w3.org/1999/xhtml">
<head>
<title>untitled</title>
<meta name="Producer" content="ReportLab PDF Library - (opensource)"/>
</head>
<body>
<doc>
  <page width="600.000000" height="800.000000">
    <flow>
      <block xMin="60" yMin="40" xMax="300" yMax="56">
        <line xMin="60" yMin="40" xMax="300" yMax="56">
          <word xMin="60.000000" yMin="40.000000" xMax="120.000000" yMax="56.000000">Labor</word>
          <word xMin="126.000000" yMin="40.000000" xMax="186.000000" yMax="56.000000">Muster&amp;Co</word>
        </line>
      </block>
      <block xMin="60" yMin="80" xMax="300" yMax="90">
        <line xMin="60" yMin="80" xMax="300" yMax="90">
          <word xMin="60.000000" yMin="80.000000" xMax="90.000000" yMax="90.000000">14,2</word>
          <word xMin="96.000000" yMin="80.000000" xMax="120.000000" yMax="90.000000">g/dl</word>
        </line>
      </block>
    </flow>
  </page>
  <page width="600.000000" height="800.000000">
  </page>
</doc>
</body>
</html>
`

func TestParseBBoxLayout(t *testing.T) {
	pages, err := parseBBoxLayout([]byte(bboxSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0].Width != 600 || pages[0].Height != 800 {
		t.Fatalf("pages %+v", pages)
	}
	if pages[1].Words == nil || len(pages[1].Words) != 0 {
		t.Errorf("page 2: %#v, want empty list", pages[1].Words)
	}
	w := pages[0].Words
	var texts []string
	for _, x := range w {
		texts = append(texts, x.Text)
		if x.Source != core.SourcePDFText || x.Confidence != 0 {
			t.Errorf("%q: source %q, confidence %v", x.Text, x.Source, x.Confidence)
		}
	}
	if !slices.Equal(texts, []string{"Labor", "Muster&Co", "14,2", "g/dl"}) {
		t.Errorf("texts %q", texts)
	}
	if w[0].Line != 0 || w[1].Line != 0 || w[2].Line != 1 || w[3].Line != 1 {
		t.Errorf("lines %d %d %d %d", w[0].Line, w[1].Line, w[2].Line, w[3].Line)
	}
	want := core.Box{X: 0.1, Y: 0.05, W: 0.1, H: 0.02}
	if b := w[0].Box; !near(b, want) {
		t.Errorf("box %+v, want %+v", b, want)
	}
}

func near(a, b core.Box) bool {
	const eps = 1e-9
	return math.Abs(a.X-b.X) < eps && math.Abs(a.Y-b.Y) < eps && math.Abs(a.W-b.W) < eps && math.Abs(a.H-b.H) < eps
}

func TestParseBBoxLayout_Invalid(t *testing.T) {
	if _, err := parseBBoxLayout([]byte("<html><body><doc><page>")); err == nil {
		t.Error("no error for cut-off output")
	}
}

func TestParseVersion(t *testing.T) {
	out := "pdftotext version 26.11.0\nCopyright 2005-2026 The Poppler Developers\n"
	if got := parseVersion(out); got != "26.11.0" {
		t.Errorf("got %q", got)
	}
	if got := parseVersion("something else"); got != "unknown" {
		t.Errorf("got %q", got)
	}
}

// Integration with the installed Poppler, on the synthetic lab report.
func newOrSkip(t *testing.T) *Poppler {
	t.Helper()
	p, err := New()
	if err != nil {
		t.Skipf("Poppler not installed: %v", err)
	}
	return p
}

func TestTextLayer_LabReport(t *testing.T) {
	p := newOrSkip(t)
	pdf, err := os.ReadFile("../../../testdata/lab_report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	pages, err := p.TextLayer(context.Background(), pdf, 11)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("%d pages, want 1", len(pages))
	}
	i := slices.IndexFunc(pages[0].Words, func(w core.OCRWord) bool { return w.Text == "Hämoglobin" })
	if i < 0 {
		t.Fatalf("Hämoglobin not in the text layer")
	}
	// pdftotext reports xMin 16.6, yMin 148.716, xMax 88.6108, yMax 163.716
	// on a 612 × 792 pt page.
	want := core.Box{X: 16.6 / 612, Y: 148.716 / 792, W: (88.6108 - 16.6) / 612, H: 15.0 / 792}
	if b := pages[0].Words[i].Box; !near(b, want) {
		t.Errorf("box %+v, want %+v", b, want)
	}
}

func TestTextLayer_NotAPDF(t *testing.T) {
	p := newOrSkip(t)
	if _, err := p.TextLayer(context.Background(), []byte("garbage"), 11); err == nil {
		t.Error("no error for garbage")
	}
}

func TestRenderPage(t *testing.T) {
	p := newOrSkip(t)
	pdf, err := os.ReadFile("../../../testdata/lab_report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	img, err := p.RenderPage(context.Background(), pdf, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(img))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	if max(cfg.Width, cfg.Height) != RenderLongSide {
		t.Errorf("size %d×%d, want long side %d", cfg.Width, cfg.Height, RenderLongSide)
	}
}
