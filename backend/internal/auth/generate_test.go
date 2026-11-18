package auth

import (
	"strings"
	"testing"
)

func TestReadableAlphabet(t *testing.T) {
	if len(ReadableAlphabet) != 58 {
		t.Errorf("alphabet has %d characters, want 58", len(ReadableAlphabet))
	}
	for _, c := range "0O1l" {
		if strings.ContainsRune(ReadableAlphabet, c) {
			t.Errorf("alphabet contains %q", c)
		}
	}
	seen := map[rune]bool{}
	for _, c := range ReadableAlphabet {
		if seen[c] {
			t.Errorf("%q appears twice", c)
		}
		seen[c] = true
	}
}

func TestGeneratePassword(t *testing.T) {
	for _, n := range []int{StartPasswordLength, AdminPasswordLength, 1} {
		p := GeneratePassword(n)
		if len(p) != n {
			t.Errorf("length = %d, want %d", len(p), n)
		}
		for _, c := range p {
			if !strings.ContainsRune(ReadableAlphabet, c) {
				t.Errorf("%q is not in the alphabet", c)
			}
		}
	}
	if StartPasswordLength != 12 || AdminPasswordLength != 24 {
		t.Error("lengths changed")
	}
}

// Every character of the alphabet appears, and none far more often than the
// others. 58 000 characters: about 1000 each; a bias like byte % 58 would
// not be visible here, but a broken mapping (e.g. only half the alphabet)
// would.
func TestGeneratePassword_UsesWholeAlphabet(t *testing.T) {
	counts := map[rune]int{}
	for i := 0; i < 1000; i++ {
		for _, c := range GeneratePassword(58) {
			counts[c]++
		}
	}
	for _, c := range ReadableAlphabet {
		if counts[c] < 700 || counts[c] > 1300 {
			t.Errorf("%q appeared %d times, expected about 1000", c, counts[c])
		}
	}
}

func TestGeneratePassword_Different(t *testing.T) {
	if GeneratePassword(12) == GeneratePassword(12) {
		t.Error("two generated passwords are equal")
	}
}
