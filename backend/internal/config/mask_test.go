package config

import "testing"

func TestMask(t *testing.T) {
	cases := []struct {
		text, want string
		secrets    []string
	}{
		{"error with key ABC123 inside", "error with key *** inside", []string{"ABC123"}},
		{"ABC123/ABC123", "***/***", []string{"ABC123"}},
		{"nothing secret", "nothing secret", []string{"ABC123"}},
		{"uri pw and key", "uri *** and ***", []string{"pw", "key"}},
		{"empty secret changes nothing", "empty secret changes nothing", []string{""}},
		{"without secrets", "without secrets", nil},
	}
	for _, c := range cases {
		if got := Mask(c.text, c.secrets...); got != c.want {
			t.Errorf("Mask(%q, %q) = %q, want %q", c.text, c.secrets, got, c.want)
		}
	}
}
