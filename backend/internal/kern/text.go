package kern

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Text-Import (API.md 3.1): Der Text wird als text.txt gespeichert.
const (
	MaxTextZeichen = 100_000 // Unicode-Zeichen (Runes), nicht Bytes
	TextDateiname  = "text.txt"
	MimeText       = "text/plain; charset=utf-8"
)

var (
	ErrTextKeinUTF8 = errors.New("Text ist kein gültiges UTF-8")
	ErrTextZuLang   = errors.New("Text ist länger als 100 000 Zeichen")
)

// PruefeText prüft einen eingegebenen Text. Er wird dabei nicht verändert;
// gespeichert und gehasht wird genau, was ankommt.
//
// Fehler, in dieser Reihenfolge: ErrDateiLeer (leer oder nur Leerraum, →
// 400 DATEI_FEHLT), ErrTextKeinUTF8 (→ 400 UNGUELTIGE_ANFRAGE),
// ErrTextZuLang (→ 413 DATEI_ZU_GROSS).
func PruefeText(text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrDateiLeer
	}
	if !utf8.ValidString(text) {
		return ErrTextKeinUTF8
	}
	// Zeichen, nicht Bytes: "ü" ist ein Zeichen, aber zwei Bytes.
	if utf8.RuneCountInString(text) > MaxTextZeichen {
		return ErrTextZuLang
	}
	return nil
}
