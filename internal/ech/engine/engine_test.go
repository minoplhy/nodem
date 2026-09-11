package engine_test

import (
	"os"
	"path/filepath"
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

func TestNewEngine_CustomOpenSSLPath(t *testing.T) {
	// 1. Explicit parameter
	eng1 := engine.NewEngine("", "auto", "/custom/openssl/bin/openssl")
	if eng1.OpenSSLPath != "/custom/openssl/bin/openssl" {
		t.Errorf("expected /custom/openssl/bin/openssl, got %s", eng1.OpenSSLPath)
	}

	// 2. OPENSSL_PATH environment variable fallback
	t.Setenv("OPENSSL_PATH", "/env/openssl/bin/openssl")
	eng2 := engine.NewEngine("", "auto")
	if eng2.OpenSSLPath != "/env/openssl/bin/openssl" {
		t.Errorf("expected /env/openssl/bin/openssl, got %s", eng2.OpenSSLPath)
	}

	// 3. OPENSSL_BIN environment variable fallback
	t.Setenv("OPENSSL_PATH", "")
	t.Setenv("OPENSSL_BIN", "/env/bin/openssl")
	eng3 := engine.NewEngine("", "auto")
	if eng3.OpenSSLPath != "/env/bin/openssl" {
		t.Errorf("expected /env/bin/openssl, got %s", eng3.OpenSSLPath)
	}
}

func TestDetectRuntime_CustomOpenSSLPath(t *testing.T) {
	// 1. Non-existent path fails with descriptive error
	engBad := engine.NewEngine("", "auto", "/nonexistent/path/to/openssl_custom")
	_, err := engBad.DetectRuntime()
	if err == nil || !strings.Contains(err.Error(), "custom OpenSSL binary not found") {
		t.Errorf("expected 'custom OpenSSL binary not found' error, got: %v", err)
	}

	// 2. Executable that fails `ech -help` returns descriptive error
	tmpDir := t.TempDir()
	failScript := filepath.Join(tmpDir, "fake_openssl_fail.sh")
	scriptContent := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(failScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create fake script: %v", err)
	}

	engFail := engine.NewEngine("", "auto", failScript)
	_, err = engFail.DetectRuntime()
	if err == nil || !strings.Contains(err.Error(), "does not support ECH") {
		t.Errorf("expected 'does not support ECH' error, got: %v", err)
	}

	// 3. Executable that succeeds on `ech -help` is accepted
	successScript := filepath.Join(tmpDir, "fake_openssl_ok.sh")
	okContent := "#!/bin/sh\nif [ \"$1\" = \"ech\" ] && [ \"$2\" = \"-help\" ]; then\n  exit 0\nfi\nexit 0\n"
	if err := os.WriteFile(successScript, []byte(okContent), 0755); err != nil {
		t.Fatalf("failed to create fake script: %v", err)
	}

	engOK := engine.NewEngine("", "auto", successScript)
	rt, err := engOK.DetectRuntime()
	if err != nil {
		t.Fatalf("expected successful detection, got error: %v", err)
	}
	if rt != "host-openssl" {
		t.Errorf("expected 'host-openssl', got %s", rt)
	}

	resolved, err := engOK.ResolveOpenSSLPath()
	if err != nil {
		t.Fatalf("failed resolving path: %v", err)
	}
	if resolved != successScript {
		t.Errorf("expected resolved path %s, got %s", successScript, resolved)
	}
}
