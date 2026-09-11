package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/agent"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func TestAgentNginxIncludesGeneration(t *testing.T) {
	storageDir := t.TempDir()

	clusters := []transport.ClusterSyncItem{
		{
			ClusterID:   1,
			ClusterName: "cluster-alpha",
			PublicName:  "cover1.test.com",
			Status:      "NEW_KEY",
			Version:     1,
			Keys: &transport.SyncKeysPayload{
				Base64ECH:     "MOCK_BASE64_1",
				ECHCurrentPEM: "-----BEGIN ECHCONFIG-----\nCURR1\n-----END ECHCONFIG-----",
			},
		},
		{
			ClusterID:   2,
			ClusterName: "cluster-beta",
			PublicName:  "cover2.test.com",
			Status:      "NEW_KEY",
			Version:     1,
			Keys: &transport.SyncKeysPayload{
				Base64ECH:     "MOCK_BASE64_2",
				ECHCurrentPEM: "-----BEGIN ECHCONFIG-----\nCURR2\n-----END ECHCONFIG-----",
			},
		},
	}

	cfg := agent.Config{
		StorageDir: storageDir,
		ProxyType:  "nginx",
		ReloadCmd:  "true",
	}

	err := agent.DeployLocally(cfg, clusters)
	if err != nil {
		t.Fatalf("deployLocally nginx failed: %v", err)
	}

	includesPath := filepath.Join(storageDir, "ech_includes.conf")
	data, err := os.ReadFile(includesPath)
	if err != nil {
		t.Fatalf("failed to read ech_includes.conf: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "cluster_1") || !strings.Contains(content, "cluster_2") {
		t.Errorf("expected both cluster directories in ech_includes.conf: %s", content)
	}
	if strings.Count(content, "ssl_ech_file") < 2 {
		t.Errorf("expected at least 2 ssl_ech_file directives, got: %s", content)
	}
}

func TestGenerateMasterIncludesConf_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "non_existent_subdir")

	if err := agent.GenerateMasterIncludesConf(storageDir); err != nil {
		t.Fatalf("generateMasterIncludesConf on empty dir failed: %v", err)
	}

	includesPath := filepath.Join(storageDir, "ech_includes.conf")
	data, err := os.ReadFile(includesPath)
	if err != nil {
		t.Fatalf("failed reading generated ech_includes.conf: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Automatically generated") {
		t.Errorf("expected header comment in empty config, got: %s", content)
	}
	if !strings.Contains(content, "No active ECH clusters") {
		t.Errorf("expected notice comment for no active clusters, got: %s", content)
	}
	if strings.Contains(content, "ssl_ech_file") {
		t.Errorf("expected no ssl_ech_file directives in empty config, got: %s", content)
	}
}
