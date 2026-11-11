package ocr

import (
	"bytes"
	"encoding/binary"
	"image"
)

// Orientation is the EXIF orientation (tag 0x0112): how the stored pixels
// must be turned to be shown upright. A phone stores a portrait photo as a
// landscape image with orientation 6 ("rotate 90° clockwise").
//
//	1 as stored          5 transposed (mirrored on the main diagonal)
//	2 mirrored left/right 6 rotated 90° clockwise
//	3 rotated 180°        7 transversed (mirrored on the other diagonal)
//	4 mirrored top/bottom 8 rotated 90° counter-clockwise
type Orientation int

// apply returns the upright image. 1 and unknown values return img.
func (o Orientation) apply(img *image.RGBA) *image.RGBA {
	if o < 2 || o > 8 {
		return img
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w // 5–8 swap width and height
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			// (sx, sy): the stored pixel shown at (dx, dy).
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-dx, dy
			case 3:
				sx, sy = w-1-dx, h-1-dy
			case 4:
				sx, sy = dx, h-1-dy
			case 5:
				sx, sy = dy, dx
			case 6:
				sx, sy = dy, h-1-dx
			case 7:
				sx, sy = w-1-dy, h-1-dx
			case 8:
				sx, sy = w-1-dy, dx
			}
			from := img.PixOffset(img.Rect.Min.X+sx, img.Rect.Min.Y+sy)
			to := dst.PixOffset(dx, dy)
			copy(dst.Pix[to:to+4], img.Pix[from:from+4])
		}
	}
	return dst
}

// exifOrientation reads the orientation from the EXIF data of a JPEG
// (segment APP1, "Exif\0\0", then a TIFF structure whose first directory
// holds tag 0x0112). Anything missing or damaged gives 1: the image is then
// read as it is stored, which is what happened before.
func exifOrientation(jpeg []byte) Orientation {
	if len(jpeg) < 4 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(jpeg) {
		if jpeg[i] != 0xFF {
			return 1
		}
		marker := jpeg[i+1]
		if marker == 0xFF { // fill byte
			i++
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // image data or end: no EXIF before it
			return 1
		}
		size := int(binary.BigEndian.Uint16(jpeg[i+2 : i+4])) // includes the 2 size bytes
		if size < 2 || i+2+size > len(jpeg) {
			return 1
		}
		segment := jpeg[i+4 : i+2+size]
		if marker == 0xE1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return tiffOrientation(segment[6:])
		}
		i += 2 + size
	}
	return 1
}

// tiffOrientation finds tag 0x0112 in the first directory (IFD0) of a TIFF
// structure. Offsets count from the start of the TIFF header.
func tiffOrientation(t []byte) Orientation {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(t[2:4]) != 42 {
		return 1
	}
	ifd := int(order.Uint32(t[4:8]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(order.Uint16(t[ifd : ifd+2]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + 12*k // entry: tag (2), type (2), count (4), value (4)
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:e+2]) != 0x0112 {
			continue
		}
		if order.Uint16(t[e+2:e+4]) != 3 { // SHORT; the value sits in the first 2 value bytes
			return 1
		}
		o := Orientation(order.Uint16(t[e+8 : e+10]))
		if o < 1 || o > 8 {
			return 1
		}
		return o
	}
	return 1
}
