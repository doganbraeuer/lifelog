package tesseract

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // register the decoders for image.Decode
	"image/png"

	"golang.org/x/image/draw"
)

// upscale enlarges an image whose long side is shorter than minLongSide, so
// that Tesseract sees the text at roughly the resolution it is made for
// (about 300 dpi; an A4 page then has 3508 pixels on its long side). Photos
// at about 120 dpi lost units and decimal commas (docs/ocr/README.md).
//
// The factor is at most maxUpscale: enlarging more invents letter shapes
// (a 612 × 792 image scaled to 2500 read "14.18.2076" instead of
// "14.10.2026"). Larger images and minLongSide 0 are returned unchanged,
// without decoding.
// The result is a PNG. Positions do not change: Tesseract reports them in
// pixels of the image it reads, and the adapter turns them into fractions
// of that image.
const maxUpscale = 2

func upscale(data []byte, minLongSide int) ([]byte, error) {
	if minLongSide <= 0 {
		return data, nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read image size: %w", err)
	}
	long := max(cfg.Width, cfg.Height)
	if long == 0 || long >= minLongSide {
		return data, nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	target := min(minLongSide, long*maxUpscale)
	w := cfg.Width * target / long
	h := cfg.Height * target / long
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// Catmull-Rom keeps edges of letters sharper than bilinear scaling.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)

	var out bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed} // goes straight to Tesseract, size does not matter
	if err := enc.Encode(&out, dst); err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}
	return out.Bytes(), nil
}
