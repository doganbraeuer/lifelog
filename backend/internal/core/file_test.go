package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// ftyp builds the start of a HEIF file with a major brand and compatible
// brands.
func ftyp(major string, compatible ...string) []byte {
	size := 16 + 4*len(compatible)
	b := make([]byte, 0, size+16)
	b = binary.BigEndian.AppendUint32(b, uint32(size))
	b = append(b, "ftyp"...)
	b = append(b, major...)
	b = append(b, 0, 0, 0, 0) // version
	for _, c := range compatible {
		b = append(b, c...)
	}
	return append(b, "....mdat-data..."...)
}

func TestDetectFileType(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string // "" = not supported
	}{
		{"JPEG", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0}, 20)...), MimeJPEG},
		{"PNG", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 20)...), MimePNG},
		{"PDF", []byte("%PDF-1.7\n%synthetic\n"), MimePDF},
		{"HEIC from an iPhone (major brand heic)", ftyp("heic", "mif1", "heic"), MimeHEIC},
		{"HEIC with major brand heix", ftyp("heix", "mif1"), MimeHEIC},
		{"generic HEIF with HEVC brand", ftyp("mif1", "mif1", "heic"), MimeHEIC},
		{"HEIF sequence with HEVC brand", ftyp("msf1", "msf1", "hevc"), MimeHEIC},
		{"AVIF is not HEIC", ftyp("avif", "mif1", "avif"), ""},
		{"generic HEIF without HEVC (AVIF)", ftyp("mif1", "mif1", "avif"), ""},
		{"MP4 video also has ftyp", ftyp("isom", "isom", "mp41"), ""},
		{"text", []byte("Laborbefund vom 14.10.2026"), ""},
		{"GIF", []byte("GIF89a......................"), ""},
		{"WebP", []byte("RIFF\x24\x00\x00\x00WEBPVP8 ............"), ""},
		{"too short for ftyp", []byte("\x00\x00\x00\x10ftyp"), ""},
		{"a .pdf extension is not enough, only the content counts", []byte("not a pdf"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DetectFileType(c.data)
			if c.want == "" {
				if !errors.Is(err, ErrUnsupportedFileType) {
					t.Errorf("= %q, %v; want ErrUnsupportedFileType", got, err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("= %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestDetectFileType_BoxSizeLies(t *testing.T) {
	// The box claims to be huge. Reading must not go past the end.
	d := ftyp("mif1", "mif1")
	binary.BigEndian.PutUint32(d[0:4], 0xFFFFFFFF)
	if _, err := DetectFileType(d); !errors.Is(err, ErrUnsupportedFileType) {
		t.Errorf("want unsupported, got %v", err)
	}

	// Box too small: the HEVC brand lies outside the box and does not count.
	d = ftyp("mif1", "mif1", "heic")
	binary.BigEndian.PutUint32(d[0:4], 20)
	if _, err := DetectFileType(d); !errors.Is(err, ErrUnsupportedFileType) {
		t.Errorf("brand outside the box must not count, got %v", err)
	}
}

func TestCheckFile(t *testing.T) {
	pdf := []byte("%PDF-1.7\n")

	if _, err := CheckFile(nil); !errors.Is(err, ErrFileEmpty) {
		t.Errorf("empty: %v", err)
	}
	if _, err := CheckFile([]byte{}); !errors.Is(err, ErrFileEmpty) {
		t.Errorf("empty: %v", err)
	}

	exact := append(pdf, bytes.Repeat([]byte{' '}, MaxFileBytes-len(pdf))...)
	if got, err := CheckFile(exact); err != nil || got != MimePDF {
		t.Errorf("exactly 10 MB: %q, %v", got, err)
	}
	if _, err := CheckFile(append(exact, ' ')); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("10 MB + 1 byte: %v", err)
	}

	if _, err := CheckFile([]byte("hello")); !errors.Is(err, ErrUnsupportedFileType) {
		t.Errorf("text: %v", err)
	}
}
