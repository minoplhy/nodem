package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/providers"
)

func TestHTTPSRecordSerialization(t *testing.T) {
	params := providers.HTTPSRecordParams{
		Domain:     "app.example.com",
		Base64ECH:  "AED+DQBIAAgABQAQAAwAAgABAAIAAQAAAAECAw==",
		PublicName: "cover.example.com",
		TTL:        300,
		ALPN:       []string{"h2", "h3"},
		IPv4Hint:   []string{"198.51.100.1"},
	}

	// RFC 9460 string
	rfcStr := params.ToRFC9460String()
	if !strings.Contains(rfcStr, "1 .") {
		t.Errorf("expected priority and target '1 .', got: %s", rfcStr)
	}
	if !strings.Contains(rfcStr, `alpn="h2,h3"`) {
		t.Errorf("expected alpn=\"h2,h3\", got: %s", rfcStr)
	}
	if !strings.Contains(rfcStr, `ech="AED+DQBIAAgABQAQAAwAAgABAAIAAQAAAAECAw=="`) {
		t.Errorf("expected ech param, got: %s", rfcStr)
	}
	if !strings.Contains(rfcStr, `ipv4hint="198.51.100.1"`) {
		t.Errorf("expected ipv4hint, got: %s", rfcStr)
	}

	// Technitium Pipe-separated syntax
	techStr := params.ToTechnitiumParams()
	if !strings.Contains(techStr, "alpn|h2,h3") {
		t.Errorf("expected Technitium alpn, got: %s", techStr)
	}
	if !strings.Contains(techStr, "ech|") {
		t.Errorf("expected Technitium ech hex, got: %s", techStr)
	}
}

func TestHookProviderHTTPS(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "dns_hook.sh")

	// Write mock shell script that outputs parameters
	scriptContent := `#!/bin/sh
if [ "$1" = "update_https" ]; then
    echo "SUCCESS: $DNS_DOMAIN -> $DNS_VALUE"
    exit 0
fi
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write hook script: %v", err)
	}

	hookProv := providers.NewHookProvider(scriptPath)
	params := providers.HTTPSRecordParams{
		Domain:    "test.example.com",
		Base64ECH: "mock_ech",
		TTL:       60,
		ALPN:      []string{"h2"},
	}

	err := hookProv.UpdateHTTPSRecord(context.Background(), params)
	if err != nil {
		t.Fatalf("UpdateHTTPSRecord failed: %v", err)
	}
}
