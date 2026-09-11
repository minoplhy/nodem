package dns_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/minoplhy/nodem/internal/provider"
	"github.com/minoplhy/nodem/internal/provider/dns"
)

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

	hookProv := dns.NewHookProvider(scriptPath)
	params := provider.HTTPSRecordParams{
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
