package auth

import (
	"net/netip"
	"testing"
)

func prefixes(t *testing.T, list ...string) []netip.Prefix {
	t.Helper()
	var out []netip.Prefix
	for _, s := range list {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

func TestClientIP(t *testing.T) {
	local := prefixes(t, "127.0.0.1/32", "::1/128")
	cases := []struct {
		name    string
		remote  string
		xff     []string
		trusted []netip.Prefix
		want    string
	}{
		{"no proxy configured", "203.0.113.7:51234", nil, nil, "203.0.113.7"},
		{"header ignored without trusted proxy", "203.0.113.7:51234", []string{"198.51.100.1"}, nil, "203.0.113.7"},
		{"header ignored from an untrusted peer", "203.0.113.7:51234", []string{"198.51.100.1"}, local, "203.0.113.7"},
		// Caddy on the same machine: the connection comes from 127.0.0.1,
		// Caddy writes the real client into X-Forwarded-For.
		{"behind Caddy", "127.0.0.1:40000", []string{"203.0.113.7"}, local, "203.0.113.7"},
		{"behind Caddy over IPv6 loopback", "[::1]:40000", []string{"203.0.113.7"}, local, "203.0.113.7"},
		// A client sends its own X-Forwarded-For; Caddy appends the real
		// address. The rightmost untrusted entry wins, not the forged one.
		{"forged entry on the left", "127.0.0.1:40000", []string{"10.9.9.9, 203.0.113.7"}, local, "203.0.113.7"},
		{"several header lines", "127.0.0.1:40000", []string{"10.9.9.9", "203.0.113.7"}, local, "203.0.113.7"},
		{"chain of trusted proxies", "127.0.0.1:40000", []string{"203.0.113.7, 10.0.0.5"},
			prefixes(t, "127.0.0.1/32", "10.0.0.0/8"), "203.0.113.7"},
		{"all trusted: leftmost", "127.0.0.1:40000", []string{"10.0.0.6, 10.0.0.5"},
			prefixes(t, "127.0.0.1/32", "10.0.0.0/8"), "10.0.0.6"},
		{"trusted peer without header", "127.0.0.1:40000", nil, local, "127.0.0.1"},
		{"broken header: connection address", "127.0.0.1:40000", []string{"nonsense"}, local, "127.0.0.1"},
		{"empty entry: connection address", "127.0.0.1:40000", []string{"203.0.113.7, "}, local, "127.0.0.1"},
		{"entry with port", "127.0.0.1:40000", []string{"203.0.113.7:5555"}, local, "203.0.113.7"},
		{"IPv6 client", "127.0.0.1:40000", []string{"2001:db8::7"}, local, "2001:db8::7"},
		{"IPv4 written as IPv6", "[::ffff:203.0.113.7]:51234", nil, nil, "203.0.113.7"},
		{"remote without port", "203.0.113.7", nil, nil, "203.0.113.7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClientIP(c.remote, c.xff, c.trusted); got != c.want {
				t.Errorf("ClientIP = %q, want %q", got, c.want)
			}
		})
	}
}
