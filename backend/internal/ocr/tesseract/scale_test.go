package tesseract

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func testImage(t *testing.T, w, h int, asJPEG bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	var b bytes.Buffer
	var err error
	if asJPEG {
		err = jpeg.Encode(&b, img, nil)
	} else {
		err = png.Encode(&b, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func size(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestUpscale(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		jpeg         bool
		min          int
		wantW, wantH int
	}{
		{"portrait PNG", 1000, 1400, false, 2500, 1785, 2500},
		{"landscape JPEG", 1400, 1000, true, 2500, 2500, 1785},
		{"at most twice as large", 100, 140, false, 2500, 200, 280},
		{"already large enough", 200, 300, false, 300, 200, 300},
		{"off", 100, 140, false, 0, 100, 140},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testImage(t, tt.w, tt.h, tt.jpeg)
			out, err := upscale(in, tt.min)
			if err != nil {
				t.Fatal(err)
			}
			if w, h := size(t, out); w != tt.wantW || h != tt.wantH {
				t.Errorf("size %d×%d, want %d×%d", w, h, tt.wantW, tt.wantH)
			}
			if tt.wantW == tt.w && !bytes.Equal(out, in) {
				t.Error("unchanged size, but the bytes changed")
			}
		})
	}
}

func TestUpscale_NotAnImage(t *testing.T) {
	if _, err := upscale([]byte("not an image"), 2500); err == nil {
		t.Error("no error")
	}
	// Scaling off: not even looked at.
	if out, err := upscale([]byte("not an image"), 0); err != nil || string(out) != "not an image" {
		t.Errorf("off: %q, %v", out, err)
	}
}
