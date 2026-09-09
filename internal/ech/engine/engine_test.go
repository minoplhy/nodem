package engine_test

import (
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/ech/engine"
)

func TestSigner(t *testing.T) {
	pub, priv, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	payload := []byte(`{"version":15,"cluster_id":1}`)
	sig, err := engine.SignPayload(priv, payload)
	if err != nil {
		t.Fatalf("SignPayload failed: %v", err)
	}

	// Verify valid signature
	if !engine.VerifySignature(pub, payload, sig) {
		t.Errorf("VerifySignature failed for valid signature")
	}

	// Verify tampered payload fails
	tampered := []byte(`{"version":16,"cluster_id":1}`)
	if engine.VerifySignature(pub, tampered, sig) {
		t.Errorf("VerifySignature succeeded for tampered payload")
	}

	// Verify wrong pubkey fails
	pub2, _, _ := engine.GenerateSigningKeyPair()
	if engine.VerifySignature(pub2, payload, sig) {
		t.Errorf("VerifySignature succeeded with wrong public key")
	}
}

func TestNormalizeCipherSuite(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "x25519,hkdf-sha256,aes-128-gcm"},
		{"x25519,hkdf-sha256,aes128gcm", "x25519,hkdf-sha256,aes-128-gcm"},
		{"p256,hkdf-sha256,chacha20poly1305", "p-256,hkdf-sha256,chacha20-poly1305"},
		{"hkdf_sha256/aes_128_gcm", "x25519,hkdf-sha256,aes-128-gcm"},
		{"HKDF_SHA256/AES_128_GCM", "x25519,hkdf-sha256,aes-128-gcm"},
		{"hkdf-sha256,aes-128-gcm", "x25519,hkdf-sha256,aes-128-gcm"},
		{"p256,hkdf_sha256/chacha20_poly1305", "p-256,hkdf-sha256,chacha20-poly1305"},
	}

	for _, tc := range tests {
		got := engine.NormalizeCipherSuite(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeCipherSuite(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseECHPEM(t *testing.T) {
	// Minimal valid ECH PEM mock
	mockPEM := `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIH1t/8+2Y2rP46gL31+wTz97iZ5V9vD4dG97mN6wL8fA
-----END PRIVATE KEY-----
-----BEGIN ECHCONFIG-----
AED+DQBIAAgABQAQAAwAAgABAAIAAQAAAAECAw==
-----END ECHCONFIG-----
`

	key, err := engine.ParseECHPEM([]byte(mockPEM))
	if err != nil {
		t.Fatalf("ParseECHPEM failed: %v", err)
	}

	if key.Base64ECH != "AED+DQBIAAgABQAQAAwAAgABAAIAAQAAAAECAw==" {
		t.Errorf("unexpected Base64ECH: %s", key.Base64ECH)
	}

	if !strings.Contains(string(key.PrivateKeyPEM), "BEGIN PRIVATE KEY") {
		t.Errorf("missing private key pem in parsed result")
	}
	if !strings.Contains(string(key.ECHConfigPEM), "BEGIN ECHCONFIG") {
		t.Errorf("missing echconfig pem in parsed result")
	}
}
