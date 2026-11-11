package ocr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"slices"
	"testing"

	"lifelog/backend/internal/core"
)

// encode writes img as PNG or JPEG.
func encode(t *testing.T, img image.Image, asJPEG bool) []byte {
	t.Helper()
	var b bytes.Buffer
	var err error
	if asJPEG {
		err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 100})
	} else {
		err = png.Encode(&b, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// halves: left half black, right half white.
func halves(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if x < w/2 {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// withEXIF inserts an APP1 segment with orientation o right after SOI.
func withEXIF(t *testing.T, jpg []byte, order binary.ByteOrder, o uint16) []byte {
	t.Helper()
	tiff := make([]byte, 8+2+12+4)
	if order == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)       // IFD0 right after the header
	order.PutUint16(tiff[8:], 1)       // one entry
	order.PutUint16(tiff[10:], 0x0112) // orientation
	order.PutUint16(tiff[12:], 3)      // SHORT
	order.PutUint32(tiff[14:], 1)      // count
	order.PutUint16(tiff[18:], o)      // value, left-aligned
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func TestExifOrientation(t *testing.T) {
	jpg := encode(t, halves(8, 4), true)
	for o := uint16(1); o <= 8; o++ {
		for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
			if got := exifOrientation(withEXIF(t, jpg, order, o)); got != Orientation(o) {
				t.Errorf("%v %d: got %d", order, o, got)
			}
		}
	}
	damaged := withEXIF(t, jpg, binary.BigEndian, 6)
	damaged[2+4+6+3] = 0 // TIFF magic 42 (bytes 0x00 0x2A) broken
	tests := map[string][]byte{
		"no EXIF":          jpg,
		"PNG":              encode(t, halves(8, 4), false),
		"empty":            nil,
		"invalid value 9":  withEXIF(t, jpg, binary.BigEndian, 9),
		"damaged TIFF":     damaged,
		"truncated header": withEXIF(t, jpg, binary.BigEndian, 6)[:12],
	}
	for name, data := range tests {
		if got := exifOrientation(data); got != 1 {
			t.Errorf("%s: got %d, want 1", name, got)
		}
	}
}

// The stored image 1 2 3 / 4 5 6 shown upright for every orientation.
func TestOrientationApply(t *testing.T) {
	stored := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i, v := range []uint8{1, 2, 3, 4, 5, 6} {
		stored.SetRGBA(i%3, i/3, color.RGBA{v, 10 * v, 0, 255})
	}
	tests := []struct {
		o    Orientation
		w, h int
		want []uint8
	}{
		{1, 3, 2, []uint8{1, 2, 3, 4, 5, 6}},
		{2, 3, 2, []uint8{3, 2, 1, 6, 5, 4}},
		{3, 3, 2, []uint8{6, 5, 4, 3, 2, 1}},
		{4, 3, 2, []uint8{4, 5, 6, 1, 2, 3}},
		{5, 2, 3, []uint8{1, 4, 2, 5, 3, 6}},
		{6, 2, 3, []uint8{4, 1, 5, 2, 6, 3}},
		{7, 2, 3, []uint8{6, 3, 5, 2, 4, 1}},
		{8, 2, 3, []uint8{3, 6, 2, 5, 1, 4}},
		{0, 3, 2, []uint8{1, 2, 3, 4, 5, 6}}, // unknown: as stored
	}
	for _, tt := range tests {
		got := tt.o.apply(stored)
		var values []uint8
		for i := 0; i < len(got.Pix); i += 4 {
			if got.Pix[i+1] != 10*got.Pix[i] || got.Pix[i+3] != 255 {
				t.Errorf("orientation %d: pixel %v torn apart", tt.o, got.Pix[i:i+4])
			}
			values = append(values, got.Pix[i])
		}
		if got.Bounds().Dx() != tt.w || got.Bounds().Dy() != tt.h || !slices.Equal(values, tt.want) {
			t.Errorf("orientation %d: %d×%d %v, want %d×%d %v", tt.o, got.Bounds().Dx(), got.Bounds().Dy(), values, tt.w, tt.h, tt.want)
		}
	}
}

// decodePNG reads the prepared image; it must be a PNG without alpha
// (Go reads an RGB PNG as *image.RGBA).
func decodePNG(t *testing.T, data []byte) *image.RGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("result is %T, want an RGB PNG", img)
	}
	return g
}

func TestPrepareImage_Size(t *testing.T) {
	opts := ImageOptions{MinLongSide: 2500, MaxLongSide: 4096}
	tests := []struct {
		name         string
		w, h         int
		jpeg         bool
		o            ImageOptions
		wantW, wantH int
	}{
		{"portrait PNG scaled up", 1000, 1400, false, opts, 1785, 2500},
		{"landscape JPEG scaled up", 1400, 1000, true, opts, 2500, 1785},
		{"at most twice as large", 100, 140, false, opts, 200, 280},
		{"between the limits: unchanged", 3024, 2000, true, opts, 3024, 2000},
		{"large photo scaled down", 8064, 6048, true, opts, 4096, 3072},
		{"no limits", 100, 5000, false, ImageOptions{}, 100, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mime := core.MimePNG
			if tt.jpeg {
				mime = core.MimeJPEG
			}
			out, err := PrepareImage(encode(t, halves(tt.w, tt.h), tt.jpeg), mime, tt.o)
			if err != nil {
				t.Fatal(err)
			}
			g := decodePNG(t, out)
			if g.Bounds().Dx() != tt.wantW || g.Bounds().Dy() != tt.wantH {
				t.Errorf("size %d×%d, want %d×%d", g.Bounds().Dx(), g.Bounds().Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

// A JPEG stored as landscape with orientation 6 comes out upright: the
// stored left (black) half is shown at the top.
func TestPrepareImage_JPEGOrientation(t *testing.T) {
	jpg := withEXIF(t, encode(t, halves(40, 20), true), binary.BigEndian, 6)
	g := decodePNG(t, mustPrepare(t, jpg, core.MimeJPEG))
	if g.Bounds().Dx() != 20 || g.Bounds().Dy() != 40 {
		t.Fatalf("size %d×%d, want 20×40", g.Bounds().Dx(), g.Bounds().Dy())
	}
	if top, bottom := g.RGBAAt(10, 5).R, g.RGBAAt(10, 35).R; top > 30 || bottom < 225 {
		t.Errorf("top %d (want black), bottom %d (want white)", top, bottom)
	}
	// The same bytes as PNG: EXIF is only read from JPEG.
	g = decodePNG(t, mustPrepare(t, encode(t, halves(40, 20), false), core.MimePNG))
	if g.Bounds().Dx() != 40 {
		t.Errorf("PNG turned: %v", g.Bounds())
	}
}

// Transparent pixels become white, even if their colour is black; colours
// stay.
func TestPrepareImage_TransparentOnWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	img.SetNRGBA(0, 0, color.NRGBA{0, 0, 0, 255})   // black text
	img.SetNRGBA(1, 0, color.NRGBA{0, 0, 0, 0})     // transparent, black colour
	img.SetNRGBA(2, 0, color.NRGBA{0, 0, 0, 128})   // half transparent
	img.SetNRGBA(3, 0, color.NRGBA{255, 0, 0, 255}) // red
	g := decodePNG(t, mustPrepare(t, encode(t, img, false), core.MimePNG))
	want := []color.RGBA{{0, 0, 0, 255}, {255, 255, 255, 255}, {127, 127, 127, 255}, {255, 0, 0, 255}}
	near := func(a, b uint8) bool { return int(a)-int(b) <= 1 && int(b)-int(a) <= 1 }
	for x, w := range want {
		if got := g.RGBAAt(x, 0); !near(got.R, w.R) || !near(got.G, w.G) || !near(got.B, w.B) || got.A != 255 {
			t.Errorf("pixel %d: %v, want %v", x, got, w)
		}
	}
}

func TestPrepareImage_TooLarge(t *testing.T) {
	data := encode(t, halves(100, 100), false)
	_, err := PrepareImage(data, core.MimePNG, ImageOptions{MaxPixels: 9_999})
	if !errors.Is(err, ErrImageTooLarge) || Reason(err) != core.ReasonImageTooLarge {
		t.Errorf("error %v, reason %q", err, Reason(err))
	}
	if _, err := PrepareImage(data, core.MimePNG, ImageOptions{MaxPixels: 10_000}); err != nil {
		t.Errorf("exactly at the limit: %v", err)
	}
}

func TestPrepareImage_NotAnImage(t *testing.T) {
	for _, mime := range []string{core.MimePNG, core.MimeJPEG, core.MimeHEIC, core.MimeWebP} {
		_, err := PrepareImage([]byte("not an image"), mime, DefaultImageOptions())
		if err == nil || Reason(err) != core.ReasonOCRFailed {
			t.Errorf("%s: error %v, reason %q", mime, err, Reason(err))
		}
	}
}

func mustPrepare(t *testing.T, data []byte, mime string) []byte {
	t.Helper()
	out, err := PrepareImage(data, mime, ImageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
