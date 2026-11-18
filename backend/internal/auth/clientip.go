package auth

import (
	"net"
	"net/netip"
	"strings"
)

// ClientIP returns the client's IP address for counting failed attempts
// (API.md 3.10).
//
// Normally that is the address of the connection (remoteAddr, "ip:port").
// The header X-Forwarded-For is only believed if the connection comes from a
// trusted proxy (TRUSTED_PROXIES), e.g. Caddy on the same machine: then
// remoteAddr is always 127.0.0.1, and the real client is in the header. Any
// client can send that header itself, so without a trusted proxy it is
// ignored; otherwise an attacker could pick a new "IP" for every attempt.
//
// Each proxy appends the address it received the request from, so the
// header reads "client, proxy1, proxy2". The result is the rightmost entry
// that is not a trusted proxy: entries further left could have been written
// by the client. If an entry cannot be read, remoteAddr is used.
//
// IPv4 addresses written as IPv6 (::ffff:1.2.3.4) are returned as IPv4.
func ClientIP(remoteAddr string, forwardedFor []string, trusted []netip.Prefix) string {
	peer, ok := parseHostPort(remoteAddr)
	if !ok {
		return remoteAddr
	}
	if !isTrusted(peer, trusted) {
		return peer.String()
	}

	var hops []string
	for _, value := range forwardedFor {
		for _, part := range strings.Split(value, ",") {
			hops = append(hops, strings.TrimSpace(part))
		}
	}
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		addr, ok := parseAddr(hops[i])
		if !ok {
			return peer.String()
		}
		client = addr
		if !isTrusted(addr, trusted) {
			break
		}
	}
	return client.String()
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// parseHostPort reads "1.2.3.4:5678" or "[::1]:5678".
func parseHostPort(s string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		return parseAddr(s)
	}
	return parseAddr(host)
}

// parseAddr reads an address with or without port, as proxies write it in
// X-Forwarded-For.
func parseAddr(s string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap(), true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}
