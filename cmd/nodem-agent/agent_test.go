package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func createMockSignature(privKey string, pubKey string, clusterID, version int64, keys *transport.SyncKeysPayload) *transport.SyncSignature {
	checksum := keys.Checksum()
	msg := transport.BuildSignableMessage(transport.PayloadTypeSyncKeyUpdate, clusterID, version, checksum)
	sigBase64, _ := engine.SignPayload(privKey, msg)
	return &transport.SyncSignature{
		Type:      transport.PayloadTypeSyncKeyUpdate,
		Algorithm: "ed25519",
		Checksum:  checksum,
		SigBase64: sigBase64,
		PublicKey: pubKey,
	}
}

func TestAgentHTTPSWorkflow(t *testing.T) {
	storageDir := t.TempDir()

	pubKey, privKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	keys1 := &transport.SyncKeysPayload{
		Base64ECH:      "MOCK_BASE64_ECH_1",
		ECHCurrentPEM:  "-----BEGIN ECHCONFIG-----\nNEW_KEY_1\n-----END ECHCONFIG-----",
		ECHPreviousPEM: "-----BEGIN ECHCONFIG-----\nOLD_KEY_1\n-----END ECHCONFIG-----",
	}

	keys2 := &transport.SyncKeysPayload{
		Base64ECH:      "MOCK_BASE64_ECH_2",
		ECHCurrentPEM:  "-----BEGIN ECHCONFIG-----\nNEW_KEY_2\n-----END ECHCONFIG-----",
		ECHPreviousPEM: "-----BEGIN ECHCONFIG-----\nOLD_KEY_2\n-----END ECHCONFIG-----",
	}

	// Mock Central Server
	acksReceived := make(map[string]int64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Agent-Token")
		if token != "valid_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/api/v1/agent/ech/sync" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(transport.SyncResponse{
				Status: "UPDATES_AVAILABLE",
				Clusters: []transport.ClusterSyncItem{
					{
						ClusterID:   1,
						ClusterName: "cluster-alpha",
						PublicName:  "cover1.test.com",
						Status:      "NEW_KEY",
						Version:     2,
						Keys:        keys1,
						Signature:   createMockSignature(privKey, pubKey, 1, 2, keys1),
					},
					{
						ClusterID:   2,
						ClusterName: "cluster-beta",
						PublicName:  "cover2.test.com",
						Status:      "NEW_KEY",
						Version:     3,
						Keys:        keys2,
						Signature:   createMockSignature(privKey, pubKey, 2, 3, keys2),
					},
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/agent/ech/ack" {
			var ackReq transport.AckRequest
			_ = json.NewDecoder(r.Body).Decode(&ackReq)
			for _, a := range ackReq.Acks {
				if a.Status == "SUCCESS" {
					acksReceived[a.ClusterName] = a.AppliedVersion
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Write mock hook script to test proxy=hook
	hookPath := filepath.Join(storageDir, "mock_hook.sh")
	hookContent := `#!/bin/sh
echo "Hook executed for $ECH_CLUSTER_NAME with $1 and $2"
exit 0
`
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		t.Fatalf("failed to write hook script: %v", err)
	}

	cfg := AgentConfig{
		ServerURL:       server.URL,
		Transport:       "https",
		Token:           "valid_token",
		StorageDir:      storageDir,
		ProxyType:       "hook",
		HookScript:      hookPath,
		ServerPublicKey: pubKey,
	}

	err = runSyncCycle(context.Background(), cfg)
	if err != nil {
		t.Fatalf("runSyncCycle failed: %v", err)
	}

	if acksReceived["cluster-alpha"] != 2 || acksReceived["cluster-beta"] != 3 {
		t.Errorf("expected ACKs for both clusters, got: %+v", acksReceived)
	}

	// Verify key files written to disk for cluster-alpha (cluster_1)
	currPath1 := filepath.Join(storageDir, "cluster_1", "ech_current.pem")
	currData1, err := os.ReadFile(currPath1)
	if err != nil || !strings.Contains(string(currData1), "NEW_KEY_1") {
		t.Errorf("cluster_1 ech_current.pem content mismatch: %s, err: %v", string(currData1), err)
	}

	// Verify key files written to disk for cluster-beta (cluster_2)
	currPath2 := filepath.Join(storageDir, "cluster_2", "ech_current.pem")
	currData2, err := os.ReadFile(currPath2)
	if err != nil || !strings.Contains(string(currData2), "NEW_KEY_2") {
		t.Errorf("cluster_2 ech_current.pem content mismatch: %s, err: %v", string(currData2), err)
	}

	// Verify local agent_state.json written
	stateFile := filepath.Join(storageDir, "agent_state.json")
	st := loadLocalState(stateFile)
	if st.Clusters["1"].AppliedVersion != 2 || st.Clusters["2"].AppliedVersion != 3 {
		t.Errorf("expected local state versions, got %+v", st.Clusters)
	}
}

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

	cfg := AgentConfig{
		StorageDir: storageDir,
		ProxyType:  "nginx",
		ReloadCmd:  "true", // Simulate successful reload command
	}

	err := deployLocally(cfg, clusters)
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

func TestResolveEndpointURL(t *testing.T) {
	testCases := []struct {
		server   string
		endpoint string
		expected string
	}{
		{"http://localhost:8080", "/api/v1/agent/ech/sync", "http://localhost:8080/api/v1/agent/ech/sync"},
		{"http://localhost:8080/", "/api/v1/agent/ech/sync", "http://localhost:8080/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath/", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath/api", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath/api/", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"monitor.example.com/subpath", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"localhost:8080/subpath", "/api/v1/agent/ech/ack", "http://localhost:8080/subpath/api/v1/agent/ech/ack"},
	}

	for _, tc := range testCases {
		got := resolveEndpointURL(tc.server, tc.endpoint)
		if got != tc.expected {
			t.Errorf("resolveEndpointURL(%q, %q) = %q, expected %q", tc.server, tc.endpoint, got, tc.expected)
		}
	}
}

func TestAgentHTTPSWorkflowWithSubpath(t *testing.T) {
	storageDir := t.TempDir()

	pubKey, privKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	keys := &transport.SyncKeysPayload{
		Base64ECH:      "MOCK_BASE64_ECH",
		ECHCurrentPEM:  "-----BEGIN ECHCONFIG-----\nNEW_KEY\n-----END ECHCONFIG-----",
		ECHPreviousPEM: "-----BEGIN ECHCONFIG-----\nOLD_KEY\n-----END ECHCONFIG-----",
	}

	// Mock Central Server hosted on subpath /custom/monitor
	ackReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Agent-Token")
		if token != "valid_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/custom/monitor/api/v1/agent/ech/sync" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(transport.SyncResponse{
				Status: "UPDATES_AVAILABLE",
				Clusters: []transport.ClusterSyncItem{
					{
						ClusterID:   1,
						ClusterName: "subpath-cluster",
						PublicName:  "cover.subpath.com",
						Status:      "NEW_KEY",
						Version:     3,
						Keys:        keys,
						Signature:   createMockSignature(privKey, pubKey, 1, 3, keys),
					},
				},
			})
			return
		}

		if r.URL.Path == "/custom/monitor/api/v1/agent/ech/ack" {
			var ackReq transport.AckRequest
			_ = json.NewDecoder(r.Body).Decode(&ackReq)
			for _, a := range ackReq.Acks {
				if a.AppliedVersion == 3 && a.Status == "SUCCESS" {
					ackReceived = true
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := AgentConfig{
		ServerURL:       server.URL + "/custom/monitor",
		Transport:       "https",
		Token:           "valid_token",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	err = runSyncCycle(context.Background(), cfg)
	if err != nil {
		t.Fatalf("runSyncCycle with subpath failed: %v", err)
	}

	if !ackReceived {
		t.Errorf("expected server with subpath to receive ACK for version 3")
	}
}

func TestAgentEnvFileLoading(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env.agent")
	envContent := "ECH_SERVER=https://custom-monitor.internal\nECH_TOKEN=secret_agent_token\nECH_PROXY=caddy\nECH_INTERVAL=120\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed writing test env file: %v", err)
	}

	// Change to tempDir to test loading
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	_ = os.Chdir(tempDir)

	// Clear any existing env
	os.Unsetenv("ECH_SERVER")
	os.Unsetenv("ECH_TOKEN")
	os.Unsetenv("ECH_PROXY")
	os.Unsetenv("ECH_INTERVAL")

	// Call godotenv.Load as in main()
	if err := godotenv.Load(".env.agent"); err != nil {
		t.Fatalf("failed loading .env.agent: %v", err)
	}

	if val := os.Getenv("ECH_SERVER"); val != "https://custom-monitor.internal" {
		t.Errorf("expected https://custom-monitor.internal, got %s", val)
	}
	if val := os.Getenv("ECH_TOKEN"); val != "secret_agent_token" {
		t.Errorf("expected secret_agent_token, got %s", val)
	}
	if val := os.Getenv("ECH_PROXY"); val != "caddy" {
		t.Errorf("expected caddy, got %s", val)
	}
	if val := os.Getenv("ECH_INTERVAL"); val != "120" {
		t.Errorf("expected 120, got %s", val)
	}
}

func TestAgentLegacyFolderMigration(t *testing.T) {
	storageDir := t.TempDir()

	// 1. Create a legacy name-based folder
	legacyDir := filepath.Join(storageDir, "cluster-legacy")
	_ = os.MkdirAll(legacyDir, 0700)
	_ = os.WriteFile(filepath.Join(legacyDir, "ech_current.pem"), []byte("LEGACY_KEY"), 0600)

	// 2. Create legacy state keyed by name
	stateFile := filepath.Join(storageDir, "agent_state.json")
	initialState := AgentState{
		Clusters: map[string]ClusterState{
			"cluster-legacy": {
				AppliedVersion: 1,
				ClusterName:    "cluster-legacy",
			},
		},
	}
	_ = saveLocalState(stateFile, initialState)

	// 3. Central server returns ClusterID: 42 for "cluster-legacy"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/ech/sync" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(transport.SyncResponse{
				Status: "UP_TO_DATE",
				Clusters: []transport.ClusterSyncItem{
					{
						ClusterID:   42,
						ClusterName: "cluster-legacy",
						Status:      "UP_TO_DATE",
						Version:     1,
					},
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/agent/ech/ack" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
	}))
	defer server.Close()

	cfg := AgentConfig{
		ServerURL:       server.URL,
		Transport:       "https",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: "mock_pub_key",
	}

	if err := runSyncCycle(context.Background(), cfg); err != nil {
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
	st := loadLocalState(stateFile)
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
			_ = json.NewEncoder(w).Encode(transport.SyncResponse{
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
			})
			return
		}
		if r.URL.Path == "/api/v1/agent/ech/ack" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
	}))
	defer server.Close()

	cfg := AgentConfig{
		ServerURL:       server.URL,
		Transport:       "https",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	// 1. First sync with Initial-Alpha-Name
	if err := runSyncCycle(context.Background(), cfg); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	expectedDir := filepath.Join(storageDir, "cluster_10")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Fatalf("expected cluster_10 directory to exist")
	}

	// 2. Central server renames cluster to "Production-Renamed-Name"
	clusterName = "Production-Renamed-Name"
	if err := runSyncCycle(context.Background(), cfg); err != nil {
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
	st := loadLocalState(stateFile)
	if st.Clusters["10"].ClusterName != "Production-Renamed-Name" {
		t.Errorf("expected state to reflect renamed cluster name, got: %s", st.Clusters["10"].ClusterName)
	}
}

func TestAgentClusterRemovalWorkflow(t *testing.T) {
	storageDir := t.TempDir()

	pubKey, privKey, _ := engine.GenerateSigningKeyPair()
	keys1 := &transport.SyncKeysPayload{
		Base64ECH:     "KEY_ECH_1",
		ECHCurrentPEM: "CURRENT_KEY_1",
	}

	keys2 := &transport.SyncKeysPayload{
		Base64ECH:     "KEY_ECH_2",
		ECHCurrentPEM: "CURRENT_KEY_2",
	}

	serverPhase := 1
	var receivedAcks []transport.ClusterAckItem

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/ech/sync" {
			w.Header().Set("Content-Type", "application/json")
			if serverPhase == 1 {
				// Phase 1: Two clusters assigned
				_ = json.NewEncoder(w).Encode(transport.SyncResponse{
					Status: "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{
						{
							ClusterID:   1,
							ClusterName: "cluster-alpha",
							Status:      "NEW_KEY",
							Version:     1,
							Keys:        keys1,
							Signature:   createMockSignature(privKey, pubKey, 1, 1, keys1),
						},
						{
							ClusterID:   2,
							ClusterName: "cluster-beta",
							Status:      "NEW_KEY",
							Version:     1,
							Keys:        keys2,
							Signature:   createMockSignature(privKey, pubKey, 2, 1, keys2),
						},
					},
				})
				return
			}

			// Phase 2: Cluster 2 unassigned/deleted
			_ = json.NewEncoder(w).Encode(transport.SyncResponse{
				Status: "UPDATES_AVAILABLE",
				Clusters: []transport.ClusterSyncItem{
					{
						ClusterID:   1,
						ClusterName: "cluster-alpha",
						Status:      "UP_TO_DATE",
						Version:     1,
					},
				},
				RemovedClusterIDs: []int64{2},
				RemovedClusters:   []string{"cluster-beta"},
			})
			return
		}

		if r.URL.Path == "/api/v1/agent/ech/ack" {
			var req transport.AckRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			receivedAcks = append(receivedAcks, req.Acks...)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
	}))
	defer server.Close()

	cfg := AgentConfig{
		ServerURL:       server.URL,
		Transport:       "https",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	// 1. Initial sync (deploy clusters 1 and 2)
	if err := runSyncCycle(context.Background(), cfg); err != nil {
		t.Fatalf("phase 1 sync failed: %v", err)
	}

	dir1 := filepath.Join(storageDir, "cluster_1")
	dir2 := filepath.Join(storageDir, "cluster_2")
	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Fatalf("expected cluster_1 directory to exist")
	}
	if _, err := os.Stat(dir2); os.IsNotExist(err) {
		t.Fatalf("expected cluster_2 directory to exist")
	}

	// Verify ech_includes.conf has both
	includesData, err := os.ReadFile(filepath.Join(storageDir, "ech_includes.conf"))
	if err != nil {
		t.Fatalf("failed reading includes conf: %v", err)
	}
	if !strings.Contains(string(includesData), "cluster_1") || !strings.Contains(string(includesData), "cluster_2") {
		t.Fatalf("expected both clusters in includes conf: %s", string(includesData))
	}

	// 2. Phase 2: Cluster 2 removed
	serverPhase = 2
	receivedAcks = nil
	if err := runSyncCycle(context.Background(), cfg); err != nil {
		t.Fatalf("phase 2 sync failed: %v", err)
	}

	// Verify cluster_2 is deleted from disk
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Errorf("expected cluster_2 to be deleted from disk, but still exists")
	}
	// Verify cluster_1 remains on disk
	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Errorf("expected cluster_1 to remain on disk")
	}

	// Verify ech_includes.conf regenerated without cluster_2
	includesData2, _ := os.ReadFile(filepath.Join(storageDir, "ech_includes.conf"))
	if strings.Contains(string(includesData2), "cluster_2") {
		t.Errorf("ech_includes.conf still contains cluster_2 after removal: %s", string(includesData2))
	}
	if !strings.Contains(string(includesData2), "cluster_1") {
		t.Errorf("ech_includes.conf missing cluster_1 after removal: %s", string(includesData2))
	}

	// Verify state file
	st := loadLocalState(filepath.Join(storageDir, "agent_state.json"))
	if _, has2 := st.Clusters["2"]; has2 {
		t.Errorf("state still has cluster 2: %+v", st.Clusters)
	}
	if _, has1 := st.Clusters["1"]; !has1 {
		t.Errorf("state missing cluster 1: %+v", st.Clusters)
	}

	// Verify ACK for removal
	foundRemovalAck := false
	for _, a := range receivedAcks {
		if a.ClusterID == 2 && a.Status == "REMOVED" {
			foundRemovalAck = true
		}
	}
	if !foundRemovalAck {
		t.Errorf("did not find REMOVED ack for cluster 2: %+v", receivedAcks)
	}
}

func TestAgentCLIClusterCommands(t *testing.T) {
	storageDir := t.TempDir()

	// Pre-create cluster_5 directory and state
	clusterDir := filepath.Join(storageDir, "cluster_5")
	_ = os.MkdirAll(clusterDir, 0700)
	_ = os.WriteFile(filepath.Join(clusterDir, "ech_current.pem"), []byte("KEY_DATA"), 0600)

	stateFile := filepath.Join(storageDir, "agent_state.json")
	initialState := AgentState{
		Clusters: map[string]ClusterState{
			"5": {
				ClusterID:      5,
				ClusterName:    "CLI-Cluster",
				AppliedVersion: 1,
			},
		},
	}
	_ = saveLocalState(stateFile, initialState)

	// 1. Test cluster list
	if err := handleClusterCommand([]string{"cluster", "list", "--storage-dir", storageDir}); err != nil {
		t.Errorf("cluster list failed: %v", err)
	}

	// 2. Test cluster remove
	if err := handleClusterCommand([]string{"cluster", "remove", "5", "--storage-dir", storageDir, "--reload-cmd", "true", "--yes"}); err != nil {
		t.Fatalf("cluster remove failed: %v", err)
	}

	// Verify directory deleted
	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		t.Errorf("cluster_5 directory was not removed")
	}

	// Verify state updated
	st := loadLocalState(stateFile)
	if _, has5 := st.Clusters["5"]; has5 {
		t.Errorf("cluster 5 still present in state after remove")
	}
}

func TestAgentSecurityVerification(t *testing.T) {
	pubKey, privKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	roguePubKey, roguePrivKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	keys := &transport.SyncKeysPayload{
		Base64ECH:     "LEGIT_KEY_ECH",
		ECHCurrentPEM: "LEGIT_PEM",
	}

	validSig := createMockSignature(privKey, pubKey, 1, 1, keys)

	tests := []struct {
		name          string
		cfgKey        string
		serverItem    transport.ClusterSyncItem
		expectedErrSub string
	}{
		{
			name:   "Missing pinned public key in agent config",
			cfgKey: "",
			serverItem: transport.ClusterSyncItem{
				ClusterID:   1,
				ClusterName: "test-cluster",
				Status:      "NEW_KEY",
				Version:     1,
				Keys:        keys,
				Signature:   validSig,
			},
			expectedErrSub: "missing required parameter: --server-public-key",
		},
		{
			name:   "Missing signature in server payload",
			cfgKey: pubKey,
			serverItem: transport.ClusterSyncItem{
				ClusterID:   1,
				ClusterName: "test-cluster",
				Status:      "NEW_KEY",
				Version:     1,
				Keys:        keys,
				Signature:   nil,
			},
			expectedErrSub: "payload signature missing",
		},
		{
			name:   "DNS Poisoning attacker signed with rogue key and sent rogue pubkey on wire",
			cfgKey: pubKey, // Agent pinned to legit pubKey
			serverItem: transport.ClusterSyncItem{
				ClusterID:   1,
				ClusterName: "test-cluster",
				Status:      "NEW_KEY",
				Version:     1,
				Keys:        keys,
				Signature:   createMockSignature(roguePrivKey, roguePubKey, 1, 1, keys),
			},
			expectedErrSub: "payload signature verification failed",
		},
		{
			name:   "Tampered checksum in signature",
			cfgKey: pubKey,
			serverItem: transport.ClusterSyncItem{
				ClusterID:   1,
				ClusterName: "test-cluster",
				Status:      "NEW_KEY",
				Version:     1,
				Keys:        keys,
				Signature: &transport.SyncSignature{
					Type:      transport.PayloadTypeSyncKeyUpdate,
					Algorithm: "ed25519",
					Checksum:  "tampered_checksum_value",
					SigBase64: validSig.SigBase64,
					PublicKey: pubKey,
				},
			},
			expectedErrSub: "payload checksum mismatch",
		},
		{
			name:   "Tampered payload data with original signature",
			cfgKey: pubKey,
			serverItem: transport.ClusterSyncItem{
				ClusterID:   1,
				ClusterName: "test-cluster",
				Status:      "NEW_KEY",
				Version:     1,
				Keys: &transport.SyncKeysPayload{
					Base64ECH:     "TAMPERED_INJECTED_KEY",
					ECHCurrentPEM: "TAMPERED_PEM",
				},
				Signature: validSig, // Checksum won't match tampered keys
			},
			expectedErrSub: "payload checksum mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storageDir := t.TempDir()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{tc.serverItem},
				})
			}))
			defer server.Close()

			cfg := AgentConfig{
				ServerURL:       server.URL,
				Transport:       "https",
				StorageDir:      storageDir,
				ProxyType:       "nginx",
				ReloadCmd:       "true",
				ServerPublicKey: tc.cfgKey,
			}

			err := runSyncCycle(context.Background(), cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, but got nil", tc.expectedErrSub)
			}
			if !strings.Contains(err.Error(), tc.expectedErrSub) {
				t.Fatalf("expected error containing %q, got: %v", tc.expectedErrSub, err)
			}
		})
	}
}




