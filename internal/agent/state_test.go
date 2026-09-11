package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/minoplhy/nodem/internal/agent"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func TestParseClusterIDAndPaths(t *testing.T) {
	if id, ok := agent.ParseClusterIDFromDir("cluster_42"); !ok || id != 42 {
		t.Errorf("expected 42, true; got %d, %v", id, ok)
	}
	if _, ok := agent.ParseClusterIDFromDir("invalid"); ok {
		t.Errorf("expected false for invalid dir")
	}
	if name := agent.ClusterDirName(123); name != "cluster_123" {
		t.Errorf("expected cluster_123, got %s", name)
	}
	if path := agent.ClusterDirPath("/opt/ech", 123); path != filepath.Join("/opt/ech", "cluster_123") {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestAgentLegacyFolderMigration(t *testing.T) {
	storageDir := t.TempDir()

	pubKey, privKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	// 1. Create a legacy name-based folder
	legacyDir := filepath.Join(storageDir, "cluster-legacy")
	_ = os.MkdirAll(legacyDir, 0700)
	_ = os.WriteFile(filepath.Join(legacyDir, "ech_current.pem"), []byte("LEGACY_KEY"), 0600)

	// 2. Create legacy state keyed by name
	stateFile := filepath.Join(storageDir, "agent_state.json")
	initialState := agent.State{
		Clusters: map[string]agent.ClusterState{
			"cluster-legacy": {
				AppliedVersion: 1,
				ClusterName:    "cluster-legacy",
			},
		},
	}
	_ = agent.SaveState(stateFile, initialState)

	// 3. Central server returns ClusterID: 42 for "cluster-legacy"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/ech/sync" {
			w.Header().Set("Content-Type", "application/json")
			resp := transport.SyncResponse{
				Status: "UP_TO_DATE",
				Clusters: []transport.ClusterSyncItem{
					{
						ClusterID:   42,
						ClusterName: "cluster-legacy",
						Status:      "UP_TO_DATE",
						Version:     1,
					},
				},
			}
			signMockResponse(privKey, pubKey, &resp)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/v1/agent/ech/ack" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
	}))
	defer server.Close()

	cfg := agent.Config{
		ServerURL:       server.URL,
		Transport:       "https",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	if err := agent.RunSyncCycle(context.Background(), cfg); err != nil {
		t.Fatalf("runSyncCycle failed: %v", err)
	}

	// Verify legacy directory moved to cluster_42
	targetDir := filepath.Join(storageDir, "cluster_42")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("expected cluster_42 directory to exist, but was not found")
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("expected legacy cluster-legacy directory to be removed/renamed")
	}

	// Verify state updated
	st := agent.LoadState(stateFile)
	if _, hasLegacy := st.Clusters["cluster-legacy"]; hasLegacy {
		t.Errorf("legacy key still in state: %+v", st.Clusters)
	}
	if cs, has42 := st.Clusters["42"]; !has42 || cs.ClusterID != 42 || cs.AppliedVersion != 1 {
		t.Errorf("expected cluster 42 in state with version 1, got: %+v", st.Clusters)
	}
}

func TestAgentClusterRenameResilience(t *testing.T) {
	storageDir := t.TempDir()

	pubKey, privKey, _ := engine.GenerateSigningKeyPair()
	keys := &transport.SyncKeysPayload{
		Base64ECH:     "KEY_ECH",
		ECHCurrentPEM: "CURRENT_KEY",
	}

	clusterName := "Initial-Alpha-Name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/ech/sync" {
			w.Header().Set("Content-Type", "application/json")
			resp := transport.SyncResponse{
				Status: "UPDATES_AVAILABLE",
				Clusters: []transport.ClusterSyncItem{
					{
						ClusterID:   10,
						ClusterName: clusterName,
						Status:      "NEW_KEY",
						Version:     1,
						Keys:        keys,
						Signature:   createMockSignature(privKey, pubKey, 10, 1, keys),
					},
				},
			}
			signMockResponse(privKey, pubKey, &resp)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/v1/agent/ech/ack" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
	}))
	defer server.Close()

	cfg := agent.Config{
		ServerURL:       server.URL,
		Transport:       "https",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	// 1. First sync with Initial-Alpha-Name
	if err := agent.RunSyncCycle(context.Background(), cfg); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	expectedDir := filepath.Join(storageDir, "cluster_10")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Fatalf("expected cluster_10 directory to exist")
	}

	// 2. Central server renames cluster to "Production-Renamed-Name"
	clusterName = "Production-Renamed-Name"
	if err := agent.RunSyncCycle(context.Background(), cfg); err != nil {
		t.Fatalf("second sync after rename failed: %v", err)
	}

	// Verify no new directory was created
	renamedDir := filepath.Join(storageDir, "Production-Renamed-Name")
	if _, err := os.Stat(renamedDir); !os.IsNotExist(err) {
		t.Errorf("unexpected directory created with cluster name: %s", renamedDir)
	}

	// Verify cluster_10 is still intact
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("cluster_10 directory vanished after rename")
	}

	// Verify state updated cluster name
	stateFile := filepath.Join(storageDir, "agent_state.json")
	st := agent.LoadState(stateFile)
	if st.Clusters["10"].ClusterName != "Production-Renamed-Name" {
		t.Errorf("expected state to reflect renamed cluster name, got: %s", st.Clusters["10"].ClusterName)
	}
}
