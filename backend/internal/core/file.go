package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/http"
	"slices"
)

// MaxFileBytes is the largest allowed file (API.md 3.1: 10 MB).
const MaxFileBytes = 10 << 20 // 10 MiB = 10,485,760 bytes

// Allowed file types (API.md 3.1).
const (
	MimeJPEG = "image/jpeg"
	MimePNG  = "image/png"
	MimeHEIC = "image/heic"
	MimePDF  = "application/pdf"
)

var (
	ErrFileEmpty           = errors.New("file is empty")
	ErrFileTooLarge        = errors.New("file is larger than 10 MB")
	ErrUnsupportedFileType = errors.New("file type is not supported")
)

// CheckFile checks size and type of an uploaded file and returns the MIME
// type. The type is detected from the content, not from the file name or the
// Content-Type of the upload. The app (or anyone else) can set both freely,
// but not the first bytes of a file.
//
// Errors: ErrFileEmpty (→ 400 FILE_MISSING), ErrFileTooLarge (→ 413),
// ErrUnsupportedFileType (→ 415).
func CheckFile(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrFileEmpty
	}
	if len(data) > MaxFileBytes {
		return "", ErrFileTooLarge
	}
	return DetectFileType(data)
}

// DetectFileType recognizes JPEG, PNG, PDF and HEIC from the first bytes.
func DetectFileType(data []byte) (string, error) {
	// http.DetectContentType knows JPEG, PNG and PDF by their "magic bytes"
	// (e.g. every PDF starts with "%PDF-").
	switch http.DetectContentType(data) {
	case MimeJPEG:
		return MimeJPEG, nil
	case MimePNG:
		return MimePNG, nil
	case MimePDF:
		return MimePDF, nil
	}
	// Go does not know HEIC (the default format of the iPhone camera), so we
	// check it ourselves.
	if isHEIC(data) {
		return MimeHEIC, nil
	}
	return "", ErrUnsupportedFileType
}

// HEIC files start with an "ftyp box":
//
//	bytes 0–3   size of the box (big-endian)
//	bytes 4–7   "ftyp"
//	bytes 8–11  major brand, e.g. "heic"
//	bytes 12–15 version
//	from byte 16 compatible brands, 4 bytes each
//
// "mif1"/"msf1" only mean "some HEIF image". That can also be AVIF (another
// codec). Then it matters whether an HEVC brand is among the compatible ones.
var (
	heicBrands  = [][]byte{[]byte("heic"), []byte("heix"), []byte("heim"), []byte("heis"), []byte("hevc"), []byte("hevx")}
	heifGeneric = [][]byte{[]byte("mif1"), []byte("msf1")}
)

func isHEIC(d []byte) bool {
	if len(d) < 16 || !bytes.Equal(d[4:8], []byte("ftyp")) {
		return false
	}
	major := d[8:12]
	if containsBrand(heicBrands, major) {
		return true
	}
	if !containsBrand(heifGeneric, major) {
		return false
	}

	// Read compatible brands, but never beyond the box or the file.
	end := int(binary.BigEndian.Uint32(d[0:4]))
	end = min(end, len(d))
	for i := 16; i+4 <= end; i += 4 {
		if containsBrand(heicBrands, d[i:i+4]) {
			return true
		}
	}
	return false
}

func containsBrand(brands [][]byte, brand []byte) bool {
	return slices.ContainsFunc(brands, func(b []byte) bool { return bytes.Equal(b, brand) })
}
