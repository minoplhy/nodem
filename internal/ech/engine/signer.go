package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// GenerateSigningKeyPair generates an Ed25519 public/private key pair encoded in Base64.
func GenerateSigningKeyPair() (pubKeyBase64, privKeyBase64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate ed25519 key pair: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)
	return pubB64, privB64, nil
}

// SignPayload creates an Ed25519 digital signature over the given payload bytes using a Base64-encoded private key.
func SignPayload(privKeyBase64 string, payload []byte) (string, error) {
	privBytes, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key base64: %w", err)
	}

	if len(privBytes) != ed25519.PrivateKeySize {
		return "", errors.New("invalid ed25519 private key length")
	}

	privKey := ed25519.PrivateKey(privBytes)
	sig := ed25519.Sign(privKey, payload)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifySignature verifies that the given signature was produced over payload by the public key.
func VerifySignature(pubKeyBase64 string, payload []byte, signatureBase64 string) bool {
	pubBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}

	pubKey := ed25519.PublicKey(pubBytes)
	return ed25519.Verify(pubKey, payload, sigBytes)
}
