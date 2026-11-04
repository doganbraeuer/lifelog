package kern

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// ftyp baut den Anfang einer HEIF-Datei mit Hauptmarke und kompatiblen Marken.
func ftyp(hauptmarke string, kompatibel ...string) []byte {
	groesse := 16 + 4*len(kompatibel)
	b := make([]byte, 0, groesse+16)
	b = binary.BigEndian.AppendUint32(b, uint32(groesse))
	b = append(b, "ftyp"...)
	b = append(b, hauptmarke...)
	b = append(b, 0, 0, 0, 0) // Version
	for _, k := range kompatibel {
		b = append(b, k...)
	}
	return append(b, "....mdat-Daten..."...)
}

func TestErkenneDateityp(t *testing.T) {
	faelle := []struct {
		name     string
		daten    []byte
		erwartet string // "" = nicht unterstützt
	}{
		{"JPEG", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0}, 20)...), MimeJPEG},
		{"PNG", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 20)...), MimePNG},
		{"PDF", []byte("%PDF-1.7\n%synthetisch\n"), MimePDF},
		{"HEIC vom iPhone (Hauptmarke heic)", ftyp("heic", "mif1", "heic"), MimeHEIC},
		{"HEIC mit Hauptmarke heix", ftyp("heix", "mif1"), MimeHEIC},
		{"HEIF allgemein mit HEVC-Marke", ftyp("mif1", "mif1", "heic"), MimeHEIC},
		{"HEIF-Folge mit HEVC-Marke", ftyp("msf1", "msf1", "hevc"), MimeHEIC},
		{"AVIF ist kein HEIC", ftyp("avif", "mif1", "avif"), ""},
		{"HEIF allgemein ohne HEVC (AVIF)", ftyp("mif1", "mif1", "avif"), ""},
		{"MP4-Video hat auch ftyp", ftyp("isom", "isom", "mp41"), ""},
		{"Text", []byte("Laborbefund vom 14.10.2026"), ""},
		{"GIF", []byte("GIF89a......................"), ""},
		{"WebP", []byte("RIFF\x24\x00\x00\x00WEBPVP8 ............"), ""},
		{"Zu kurz für ftyp", []byte("\x00\x00\x00\x10ftyp"), ""},
		{"PDF-Endung reicht nicht, nur der Inhalt zählt", []byte("kein pdf"), ""},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			got, err := ErkenneDateityp(f.daten)
			if f.erwartet == "" {
				if !errors.Is(err, ErrDateitypNichtUnterstuetzt) {
					t.Errorf("= %q, %v; erwartet ErrDateitypNichtUnterstuetzt", got, err)
				}
				return
			}
			if err != nil || got != f.erwartet {
				t.Errorf("= %q, %v; erwartet %q", got, err, f.erwartet)
			}
		})
	}
}

func TestErkenneDateityp_BoxgroesseLuegt(t *testing.T) {
	// Die Box behauptet, riesig zu sein. Es darf nicht über das Ende gelesen werden.
	d := ftyp("mif1", "mif1")
	binary.BigEndian.PutUint32(d[0:4], 0xFFFFFFFF)
	if _, err := ErkenneDateityp(d); !errors.Is(err, ErrDateitypNichtUnterstuetzt) {
		t.Errorf("erwartet nicht unterstützt, bekommen %v", err)
	}

	// Box zu klein: Die HEVC-Marke liegt außerhalb der Box und zählt nicht.
	d = ftyp("mif1", "mif1", "heic")
	binary.BigEndian.PutUint32(d[0:4], 20)
	if _, err := ErkenneDateityp(d); !errors.Is(err, ErrDateitypNichtUnterstuetzt) {
		t.Errorf("Marke außerhalb der Box darf nicht zählen, bekommen %v", err)
	}
}

func TestPruefeDatei(t *testing.T) {
	pdf := []byte("%PDF-1.7\n")

	if _, err := PruefeDatei(nil); !errors.Is(err, ErrDateiLeer) {
		t.Errorf("leer: %v", err)
	}
	if _, err := PruefeDatei([]byte{}); !errors.Is(err, ErrDateiLeer) {
		t.Errorf("leer: %v", err)
	}

	genau := append(pdf, bytes.Repeat([]byte{' '}, MaxDateiBytes-len(pdf))...)
	if got, err := PruefeDatei(genau); err != nil || got != MimePDF {
		t.Errorf("genau 10 MB: %q, %v", got, err)
	}
	if _, err := PruefeDatei(append(genau, ' ')); !errors.Is(err, ErrDateiZuGross) {
		t.Errorf("10 MB + 1 Byte: %v", err)
	}

	if _, err := PruefeDatei([]byte("hallo")); !errors.Is(err, ErrDateitypNichtUnterstuetzt) {
		t.Errorf("Text: %v", err)
	}
}
