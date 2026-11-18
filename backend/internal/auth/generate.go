package auth

import "crypto/rand"

// ReadableAlphabet for generated passwords: digits, upper and lower case
// letters without 0, O, 1 and l, which are easy to confuse when a start
// password is read from paper or a screen. 58 characters, about 5.9 bits
// each.
const ReadableAlphabet = "23456789ABCDEFGHIJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Lengths of generated passwords (cmd/users).
const (
	StartPasswordLength = 12 // role user, about 70 bits; changed at the first login
	AdminPasswordLength = 24 // role admin, about 140 bits; no forced change
)

// GeneratePassword returns n characters from ReadableAlphabet, each chosen
// uniformly from crypto/rand.
//
// Uniform: a random byte has 256 values, 256 is not a multiple of 58. Taking
// byte % 58 would make the first 24 characters slightly more likely.
// Bytes from 232 (= 4 × 58) upwards are therefore thrown away and drawn
// again (rejection sampling).
func GeneratePassword(n int) string {
	const size = len(ReadableAlphabet)
	const limit = 256 - 256%size
	out := make([]byte, 0, n)
	buf := make([]byte, 2*n)
	for len(out) < n {
		_, _ = rand.Read(buf)
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, ReadableAlphabet[int(b)%size])
			if len(out) == n {
				break
			}
		}
	}
	return string(out)
}
