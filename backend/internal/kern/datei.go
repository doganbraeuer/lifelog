package kern

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/http"
	"slices"
)

// MaxDateiBytes ist die größte erlaubte Datei (API.md 3.1: 10 MB).
const MaxDateiBytes = 10 << 20 // 10 MiB = 10 485 760 Bytes

// Erlaubte Dateitypen (API.md 3.1).
const (
	MimeJPEG = "image/jpeg"
	MimePNG  = "image/png"
	MimeHEIC = "image/heic"
	MimePDF  = "application/pdf"
)

var (
	ErrDateiLeer                 = errors.New("Datei ist leer")
	ErrDateiZuGross              = errors.New("Datei ist größer als 10 MB")
	ErrDateitypNichtUnterstuetzt = errors.New("Dateityp wird nicht unterstützt")
)

// PruefeDatei prüft Größe und Typ einer hochgeladenen Datei und liefert den
// MIME-Typ. Der Typ wird am Inhalt erkannt, nicht an Dateiname oder
// Content-Type des Uploads. Beides kann die App (oder jemand anderes) frei
// setzen, die ersten Bytes einer Datei aber nicht.
//
// Fehler: ErrDateiLeer (→ 400 DATEI_FEHLT), ErrDateiZuGross (→ 413),
// ErrDateitypNichtUnterstuetzt (→ 415).
func PruefeDatei(daten []byte) (string, error) {
	if len(daten) == 0 {
		return "", ErrDateiLeer
	}
	if len(daten) > MaxDateiBytes {
		return "", ErrDateiZuGross
	}
	return ErkenneDateityp(daten)
}

// ErkenneDateityp erkennt JPEG, PNG, PDF und HEIC an den ersten Bytes.
func ErkenneDateityp(daten []byte) (string, error) {
	// http.DetectContentType kennt JPEG, PNG und PDF anhand ihrer
	// „magischen Bytes“ (z. B. beginnt jedes PDF mit "%PDF-").
	switch http.DetectContentType(daten) {
	case MimeJPEG:
		return MimeJPEG, nil
	case MimePNG:
		return MimePNG, nil
	case MimePDF:
		return MimePDF, nil
	}
	// HEIC (Standardformat der iPhone-Kamera) kennt Go nicht, darum selbst.
	if istHEIC(daten) {
		return MimeHEIC, nil
	}
	return "", ErrDateitypNichtUnterstuetzt
}

// HEIC-Dateien beginnen mit einer „ftyp-Box“:
//
//	Byte 0–3   Größe der Box (big-endian)
//	Byte 4–7   "ftyp"
//	Byte 8–11  Hauptmarke (major brand), z. B. "heic"
//	Byte 12–15 Version
//	ab Byte 16 kompatible Marken, je 4 Bytes
//
// "mif1"/"msf1" bedeuten nur „irgendein HEIF-Bild“. Das kann auch AVIF sein
// (anderer Codec). Dann zählt, ob eine HEVC-Marke unter den kompatiblen ist.
var (
	heicMarken    = [][]byte{[]byte("heic"), []byte("heix"), []byte("heim"), []byte("heis"), []byte("hevc"), []byte("hevx")}
	heifAllgemein = [][]byte{[]byte("mif1"), []byte("msf1")}
)

func istHEIC(d []byte) bool {
	if len(d) < 16 || !bytes.Equal(d[4:8], []byte("ftyp")) {
		return false
	}
	hauptmarke := d[8:12]
	if enthaeltMarke(heicMarken, hauptmarke) {
		return true
	}
	if !enthaeltMarke(heifAllgemein, hauptmarke) {
		return false
	}

	// Kompatible Marken lesen, aber nie über die Box oder die Datei hinaus.
	ende := int(binary.BigEndian.Uint32(d[0:4]))
	ende = min(ende, len(d))
	for i := 16; i+4 <= ende; i += 4 {
		if enthaeltMarke(heicMarken, d[i:i+4]) {
			return true
		}
	}
	return false
}

func enthaeltMarke(marken [][]byte, marke []byte) bool {
	return slices.ContainsFunc(marken, func(m []byte) bool { return bytes.Equal(m, marke) })
}
