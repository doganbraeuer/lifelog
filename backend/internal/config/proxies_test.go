package config

import (
	"net/netip"
	"strings"
	"testing"
)

func TestLoad_TrustedProxiesDefaultEmpty(t *testing.T) {
	c, err := Load(env(complete()))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want empty: without configuration X-Forwarded-For is never believed", c.TrustedProxies)
	}
}

func TestLoad_TrustedProxies(t *testing.T) {
	e := complete()
	// Caddy on the same machine, plus a range and an IPv4-mapped address.
	e["TRUSTED_PROXIES"] = " 127.0.0.1, ::1 ,10.1.2.3/8,, ::ffff:192.0.2.1 "
	c, err := Load(env(e))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "192.0.2.1/32"}
	if len(c.TrustedProxies) != len(want) {
		t.Fatalf("TrustedProxies = %v, want %v", c.TrustedProxies, want)
	}
	for i, w := range want {
		if c.TrustedProxies[i] != netip.MustParsePrefix(w) {
			t.Errorf("TrustedProxies[%d] = %v, want %s", i, c.TrustedProxies[i], w)
		}
	}
}

func TestLoad_TrustedProxiesInvalid(t *testing.T) {
	for _, value := range []string{"caddy", "127.0.0.1:443", "10.0.0.0/33", "300.1.1.1", "fe80::1%eth0"} {
		e := complete()
		e["TRUSTED_PROXIES"] = value
		_, err := Load(env(e))
		if err == nil {
			t.Errorf("%q: want an error", value)
			continue
		}
		if !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
			t.Errorf("%q: error does not name the variable: %v", value, err)
		}
		// Secrets of other variables never appear in an error.
		if strings.Contains(err.Error(), "SECRET") {
			t.Errorf("%q: error contains a secret: %v", value, err)
		}
	}
}
