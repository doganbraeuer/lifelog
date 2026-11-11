package ocr

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register the decoders for image.Decode
	"image/png"

	"github.com/gen2brain/heic"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"lifelog/backend/internal/core"
)

func init() {
	// Always decode HEIC with the WASM build embedded in the module. By
	// default the package would prefer a libheif installed on the machine;
	// then results would differ between machines, and a missing or old
	// libheif would only show up at runtime.
	heic.ForceWasmMode = true
}

// ErrImageTooLarge: the image has more pixels than ImageOptions.MaxPixels
// and is not decoded.
var ErrImageTooLarge = errors.New("image too large for ocr")

// Defaults of ImageOptions, measured on 2026-11-11 (docs/ocr/README.md).
const (
	// DefaultMinLongSide: smaller images are scaled up to this long side.
	// At 2000 and 2500 every value of the demo photos is read; at 3000 a
	// whole table row disappears, from 3508 on the results get worse again.
	DefaultMinLongSide = 2500

	// DefaultMaxLongSide: larger images are scaled down to this long side.
	// A 12-megapixel phone photo (4032) is read as it is (100 % on the
	// real phone photo); a 48-megapixel photo is halved.
	DefaultMaxLongSide = 4096

	// DefaultMaxPixels: images with more pixels are not decoded. A decoded
	// image needs 4 bytes per pixel: 50 megapixels are 200 MB. Covers the
	// 48-megapixel photos of current phones.
	DefaultMaxPixels = 50_000_000

	// maxUpscale: enlarge at most to twice the size. Enlarging more invents
	// letter shapes (a 612 × 792 image scaled to 2500 read "14.18.2076"
	// instead of "14.10.2026").
	maxUpscale = 2
)

// ImageOptions: how PrepareImage sizes an image.
type ImageOptions struct {
	MinLongSide int // scale up if the long side is shorter; 0 = never scale up
	MaxLongSide int // scale down if the long side is longer; 0 = never scale down
	MaxPixels   int // ErrImageTooLarge above this; 0 = DefaultMaxPixels
}

// DefaultImageOptions are the settings of the server.
func DefaultImageOptions() ImageOptions {
	return ImageOptions{MinLongSide: DefaultMinLongSide, MaxLongSide: DefaultMaxLongSide, MaxPixels: DefaultMaxPixels}
}

// PrepareImage turns a photo (JPEG, PNG, HEIC, WebP) into the one format
// every OCR engine gets: upright, with a long side between MinLongSide and
// MaxLongSide, as an RGB PNG. It stays in colour: a grey version read the
// demo photos and the HEIC phone photo worse (docs/ocr/README.md).
//
//   - Upright: a JPEG's EXIF orientation is applied; a HEIC file's rotation
//     is applied by the decoder. Positions from OCR then refer to the photo
//     as it is shown (docs/API.md, verification).
//   - Transparent pixels (PNG, WebP) are put on white, so dark text on a
//     transparent background stays readable.
//
// Only the image is converted, no metadata is carried over. The file stored
// and sent to the vision LLM is not changed.
func PrepareImage(data []byte, mimeType string, o ImageOptions) ([]byte, error) {
	maxPixels := o.MaxPixels
	if maxPixels <= 0 {
		maxPixels = DefaultMaxPixels
	}

	// Check the size before decoding (only the header is read), so that a
	// small file with a huge image cannot fill the memory.
	var cfg image.Config
	var err error
	if mimeType == core.MimeHEIC {
		cfg, err = heic.DecodeConfig(bytes.NewReader(data))
	} else {
		cfg, _, err = image.DecodeConfig(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("read image size: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("image has no pixels")
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("%d × %d pixels: %w", cfg.Width, cfg.Height, ErrImageTooLarge)
	}

	var src image.Image
	if mimeType == core.MimeHEIC {
		src, err = heic.Decode(bytes.NewReader(data))
	} else {
		src, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	img := resize(onWhite(src), o)
	if mimeType == core.MimeJPEG {
		img = Orientation(exifOrientation(data)).apply(img)
	}

	var out bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed} // goes straight to the engine, size does not matter
	if err := enc.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}
	return out.Bytes(), nil
}

// resize scales img so that its long side lies between MinLongSide (at
// most maxUpscale times) and MaxLongSide. The rotation does not matter:
// it does not change the long side.
func resize(img *image.RGBA, o ImageOptions) *image.RGBA {
	b := img.Bounds()
	long := max(b.Dx(), b.Dy())
	target := long
	switch {
	case o.MinLongSide > 0 && long < o.MinLongSide:
		target = min(o.MinLongSide, long*maxUpscale)
	case o.MaxLongSide > 0 && long > o.MaxLongSide:
		target = o.MaxLongSide
	}
	if target == long {
		return img
	}
	w := max(1, b.Dx()*target/long)
	h := max(1, b.Dy()*target/long)
	// Catmull-Rom keeps the edges of letters sharper than bilinear scaling;
	// scaling down, it also averages over the pixels it leaves out.
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// onWhite draws src on a white background: transparent pixels (PNG, WebP)
// become white, so dark text on a transparent background stays readable.
// The result is opaque, so the PNG encoder writes plain RGB. draw.Draw has
// fast paths for the types the decoders return (YCbCr, NRGBA, RGBA, Gray).
func onWhite(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
