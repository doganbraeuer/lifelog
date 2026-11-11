package core

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckText(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want error
	}{
		{"normal", "Überweisung vom 21.10.2026", nil},
		{"whitespace at the edges allowed", "  \n Text \r\n", nil},
		{"empty", "", ErrFileEmpty},
		{"only spaces", " \t\r\n ", ErrFileEmpty},
		{"no-break space counts as whitespace", "  ", ErrFileEmpty},
		{"invalid UTF-8", "Text \xff\xfe", ErrTextNotUTF8},
		{"truncated character", "Gr\xc3", ErrTextNotUTF8},
		{"exactly 100,000 characters", strings.Repeat("a", MaxTextChars), nil},
		{"100,001 characters", strings.Repeat("a", MaxTextChars+1), ErrTextTooLong},
		// 200,000 bytes but 100,000 characters: allowed.
		{"100,000 umlauts", strings.Repeat("ü", MaxTextChars), nil},
		{"100,001 umlauts", strings.Repeat("ü", MaxTextChars+1), ErrTextTooLong},
		// 4 bytes per character.
		{"100,000 emoji", strings.Repeat("🩺", MaxTextChars), nil},
		// Empty comes before invalid, invalid before too long.
		{"too long and invalid", strings.Repeat("a", MaxTextChars+1) + "\xff", ErrTextNotUTF8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckText(tc.text); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Errorf("CheckText = %v, want %v", err, tc.want)
			}
		})
	}
}
