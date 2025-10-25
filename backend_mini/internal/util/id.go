package util

import (
	"crypto/rand"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateShortID() (string, error) {
	b := make([]byte, 6)
	// crypto/rand for production-safe randomness
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}
