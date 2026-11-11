package core

import (
	"errors"
	"strings"
	"testing"
)

func TestPruefeText(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want error
	}{
		{"normal", "Überweisung vom 21.10.2026", nil},
		{"Leerraum am Rand erlaubt", "  \n Text \r\n", nil},
		{"leer", "", ErrDateiLeer},
		{"nur Leerzeichen", " \t\r\n ", ErrDateiLeer},
		{"geschütztes Leerzeichen zählt als Leerraum", "  ", ErrDateiLeer},
		{"ungültiges UTF-8", "Text \xff\xfe", ErrTextKeinUTF8},
		{"abgeschnittenes Zeichen", "Gr\xc3", ErrTextKeinUTF8},
		{"genau 100 000 Zeichen", strings.Repeat("a", MaxTextZeichen), nil},
		{"100 001 Zeichen", strings.Repeat("a", MaxTextZeichen+1), ErrTextZuLang},
		// 200 000 Bytes, aber 100 000 Zeichen: erlaubt.
		{"100 000 Umlaute", strings.Repeat("ü", MaxTextZeichen), nil},
		{"100 001 Umlaute", strings.Repeat("ü", MaxTextZeichen+1), ErrTextZuLang},
		// 4 Bytes pro Zeichen.
		{"100 000 Emoji", strings.Repeat("🩺", MaxTextZeichen), nil},
		// Leer geht vor ungültig, ungültig vor zu lang.
		{"zu lang und ungültig", strings.Repeat("a", MaxTextZeichen+1) + "\xff", ErrTextKeinUTF8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := PruefeText(tc.text); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Errorf("PruefeText = %v, erwartet %v", err, tc.want)
			}
		})
	}
}
