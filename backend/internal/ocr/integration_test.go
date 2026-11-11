package ocr_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/ocr"
	"lifelog/backend/internal/ocr/poppler"
	"lifelog/backend/internal/ocr/tesseract"
)

// End to end with the installed programs. Skipped if Poppler or Tesseract
// is missing.
//
// The digital PDF is backend/testdata/lab_report.pdf. For OCR the test uses
// the demo photo of the January lab report: testdata/lab_report.png has
// only 612 × 792 pixels (72 dpi), too coarse for OCR ("14.18.2976"). How
// well OCR reads values and units is measured by cmd/ocrcheck, not here.

func pipelineOrSkip(t *testing.T) *ocr.Pipeline {
	t.Helper()
	pdf, err := poppler.New()
	if err != nil {
		t.Skipf("Poppler not installed: %v", err)
	}
	engine, err := tesseract.New(tesseract.DefaultLanguage)
	if err != nil {
		t.Skipf("Tesseract not installed: %v", err)
	}
	return &ocr.Pipeline{Engine: engine, PDF: pdf, Image: ocr.DefaultImageOptions()}
}

// labEntry holds the values of backend/testdata/lab_report.txt.
func labEntry() core.Entry {
	s := func(v string) *string { return &v }
	return core.Entry{
		DocumentDate: s("2026-10-14"),
		Issuer:       s("Labor Muster GmbH, Salzburg"),
		Facts: []core.Fact{
			{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: s("g/dl")},
			{ID: "f2", Name: "Leukozyten", Value: "6,8", Unit: s("G/l")},
			{ID: "f3", Name: "Thrombozyten", Value: "245", Unit: s("G/l")},
			{ID: "f4", Name: "CRP", Value: "<0,5", Unit: s("mg/dl")},
			{ID: "f5", Name: "Befundart", Value: "Blutbild klein"},
		},
	}
}

// photoEntry holds robust values of demo/photos/01_lab_report_2026-01-20_photo.png.
func photoEntry() core.Entry {
	s := func(v string) *string { return &v }
	return core.Entry{
		DocumentDate: s("2026-01-20"),
		Issuer:       s("Labor Muster GmbH"),
		Facts:        []core.Fact{{ID: "f1", Name: "Befundart", Value: "Lipidprofil"}},
	}
}

const demoPhoto = "../../../demo/photos/01_lab_report_2026-01-20_photo.png"

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readPhoto(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(demoPhoto)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPipeline_DigitalPDF_UsesTextLayer(t *testing.T) {
	p := pipelineOrSkip(t)
	r, err := p.Recognize(context.Background(), read(t, "lab_report.pdf"), core.MimePDF)
	if err != nil {
		t.Fatal(err)
	}
	v := core.Verify(labEntry(), r, time.Now())
	if v.Source != core.SourcePDFText {
		t.Errorf("source %q, want pdf_text (engine %q)", v.Source, v.Engine)
	}
	// The text layer is exact: everything is confirmed.
	if v.Confirmed != v.Total {
		for _, f := range v.Fields {
			t.Logf("%s: %s", f.Field, f.Status)
		}
		t.Errorf("confirmed %d of %d, want all", v.Confirmed, v.Total)
	}
}

func TestPipeline_ScannedPDF_UsesOCR(t *testing.T) {
	p := pipelineOrSkip(t)
	scan := imagePDF(t, readPhoto(t))
	r, err := p.Recognize(context.Background(), scan, core.MimePDF)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Pages) != 1 || r.Pages[0].Source != core.SourceOCR {
		t.Fatalf("pages %+v, want one OCR page", r.Pages)
	}
	checkAllByOCR(t, core.Verify(photoEntry(), r, time.Now()))
}

func TestPipeline_Photo_UsesOCR(t *testing.T) {
	p := pipelineOrSkip(t)
	r, err := p.Recognize(context.Background(), readPhoto(t), core.MimePNG)
	if err != nil {
		t.Fatal(err)
	}
	checkAllByOCR(t, core.Verify(photoEntry(), r, time.Now()))
}

// The real iPhone photo as HEIC: stored as a 4032 × 3024 landscape image
// plus "rotate by 90°". OCR must read it upright (3024 × 4032).
func TestPipeline_HEIC_PhonePhoto(t *testing.T) {
	p := pipelineOrSkip(t)
	data, err := os.ReadFile("../../../demo/phone/01_lab_report_2026-01-20_phone.heic")
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Recognize(context.Background(), data, core.MimeHEIC)
	if err != nil {
		t.Fatal(err)
	}
	if pg := r.Pages[0]; pg.Width != 3024 || pg.Height != 4032 {
		t.Errorf("page %v × %v, want 3024 × 4032 (upright)", pg.Width, pg.Height)
	}
	checkAllByOCR(t, core.Verify(photoEntry(), r, time.Now()))
}

// A JPEG stored lying on its side with EXIF orientation 6, as cameras do.
// Without turning it upright, Tesseract reads nothing useful.
func TestPipeline_JPEG_EXIFOrientation(t *testing.T) {
	p := pipelineOrSkip(t)
	img, err := png.Decode(bytes.NewReader(readPhoto(t)))
	if err != nil {
		t.Fatal(err)
	}
	// Stored = upright turned 90° counter-clockwise; orientation 6 turns it back.
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	stored := image.NewRGBA(image.Rect(0, 0, h, w))
	for sy := 0; sy < w; sy++ {
		for sx := 0; sx < h; sx++ {
			r, g, bl, a := img.At(b.Min.X+w-1-sy, b.Min.Y+sx).RGBA()
			white := 0xffff - a
			stored.Set(sx, sy, rgba(r+white, g+white, bl+white))
		}
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, stored, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	r, err := p.Recognize(context.Background(), withOrientation6(jpg.Bytes()), core.MimeJPEG)
	if err != nil {
		t.Fatal(err)
	}
	if pg := r.Pages[0]; pg.Width >= pg.Height {
		t.Errorf("page %v × %v, want portrait", pg.Width, pg.Height)
	}
	checkAllByOCR(t, core.Verify(photoEntry(), r, time.Now()))
}

// The demo photo as WebP (made with `cwebp -q 90`, with alpha channel).
func TestPipeline_WebP(t *testing.T) {
	p := pipelineOrSkip(t)
	data := read(t, "lab_report_photo.webp")
	if mime, err := core.DetectFileType(data); err != nil || mime != core.MimeWebP {
		t.Fatalf("detected %q, %v", mime, err)
	}
	r, err := p.Recognize(context.Background(), data, core.MimeWebP)
	if err != nil {
		t.Fatal(err)
	}
	checkAllByOCR(t, core.Verify(photoEntry(), r, time.Now()))
}

// withOrientation6 inserts an EXIF segment (big-endian TIFF, IFD0 with
// orientation 6) right after the start marker of a JPEG.
func withOrientation6(jpg []byte) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, // header, IFD0 at 8
		0, 1, // one entry
		0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, // orientation, SHORT, 1, value 6
		0, 0, 0, 0} // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)
	n := len(payload) + 2
	seg := append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func checkAllByOCR(t *testing.T, v core.Verification) {
	t.Helper()
	if v.Source != core.SourceOCR || !strings.HasPrefix(v.Engine, "tesseract ") {
		t.Errorf("source %q, engine %q", v.Source, v.Engine)
	}
	for _, f := range v.Fields {
		if f.Status != core.FieldConfirmed || f.Source != core.SourceOCR || f.Confidence == nil || len(f.Locations) == 0 {
			t.Errorf("%s: %+v, want confirmed by OCR with confidence and location", f.Field, f)
		}
	}
}

func TestPipeline_TimeLimit(t *testing.T) {
	p := pipelineOrSkip(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Recognize(ctx, readPhoto(t), core.MimePNG)
	if err == nil || ocr.Reason(err) != core.ReasonOCRFailed {
		t.Errorf("error %v, want ocr_failed", err)
	}
}

// imagePDF builds a one-page PDF that contains only the image, like a
// scanner does: no text layer. The PNG is embedded as JPEG (DCTDecode),
// which PDF supports directly.
func imagePDF(t *testing.T, pngData []byte) []byte {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		t.Fatal(err)
	}
	// Draw on white: the PNG has an alpha channel, JPEG has none.
	b := img.Bounds()
	rgb := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			white := 0xffff - a
			rgb.Set(x, y, rgba(r+white, g+white, bl+white))
		}
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, rgb, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	w, h := b.Dx(), b.Dy()

	var out bytes.Buffer
	var offsets []int
	obj := func(format string, args ...any) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, format, args...)
	}
	out.WriteString("%PDF-1.4\n")
	obj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	obj("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	obj("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>\nendobj\n", w, h)
	obj("4 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n", w, h, jpg.Len())
	out.Write(jpg.Bytes())
	out.WriteString("\nendstream\nendobj\n")
	content := fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", w, h)
	obj("5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)

	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return out.Bytes()
}

func rgba(r, g, b uint32) colorRGB {
	return colorRGB{uint8(min(r, 0xffff) >> 8), uint8(min(g, 0xffff) >> 8), uint8(min(b, 0xffff) >> 8)}
}

// colorRGB is an opaque color for image.RGBA.Set.
type colorRGB struct{ r, g, b uint8 }

func (c colorRGB) RGBA() (r, g, b, a uint32) {
	return uint32(c.r) * 0x101, uint32(c.g) * 0x101, uint32(c.b) * 0x101, 0xffff
}
