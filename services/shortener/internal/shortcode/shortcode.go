// Package shortcode generates random base58 short codes for links.
package shortcode

import (
	"crypto/rand"
	"fmt"
)

const (
	// Length is the number of characters in a generated short code.
	Length = 7

	// Alphabet is the base58 character set (Bitcoin-style, no 0OIl).
	Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	alphabetLen = byte(len(Alphabet))
)

// Generate returns a cryptographically random short code of length 7 using the base58 alphabet.
func Generate() (string, error) {
	buf := make([]byte, Length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("shortcode: read random bytes: %w", err)
	}

	code := make([]byte, Length)
	for i, b := range buf {
		code[i] = Alphabet[b%alphabetLen]
	}
	return string(code), nil
}

// IsValid checks whether s is a valid short code (correct length and all characters in alphabet).
func IsValid(s string) bool {
	if len(s) != Length {
		return false
	}
	for _, c := range []byte(s) {
		if !inAlphabet(c) {
			return false
		}
	}
	return true
}

func inAlphabet(c byte) bool {
	for _, a := range []byte(Alphabet) {
		if c == a {
			return true
		}
	}
	return false
}
