package misc

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

const maxSafeInteger = 9007199254740991 // 2^53 - 1

// GenerateRandomID generates a positive random int64 within the JavaScript safe integer range [1, 9007199254740991].
func GenerateRandomID() int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(maxSafeInteger))
	if err != nil {
		// Fallback should never happen in practice
		return 1
	}
	return n.Int64() + 1
}

const alphanumericCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateBootstrapToken generates a secure 12-character alphanumeric bootstrap token.
func GenerateBootstrapToken() string {
	b := make([]byte, 12)
	charsetLen := big.NewInt(int64(len(alphanumericCharset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			b[i] = alphanumericCharset[0]
			continue
		}
		b[i] = alphanumericCharset[idx.Int64()]
	}
	return string(b)
}

// GenerateSessionID generates a 32-byte cryptographically secure random session ID formatted as 64 hex characters.
func GenerateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GeneratePublicSessionID generates a 16-byte random public session identifier with a 'sess_' prefix.
func GeneratePublicSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sess_" + hex.EncodeToString(bytes), nil
}

