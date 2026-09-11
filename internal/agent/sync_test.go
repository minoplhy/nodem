package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/agent"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

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
			resp := transport.SyncResponse{
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
			}
			signMockResponse(privKey, pubKey, &resp)
			_ = json.NewEncoder(w).Encode(resp)
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

	cfg := agent.Config{
		ServerURL:       server.URL,
		Transport:       "https",
		Token:           "valid_token",
		StorageDir:      storageDir,
		ProxyType:       "hook",
		HookScript:      hookPath,
		ServerPublicKey: pubKey,
	}

	err = agent.RunSyncCycle(context.Background(), cfg)
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
	st := agent.LoadState(stateFile)
	if st.Clusters["1"].AppliedVersion != 2 || st.Clusters["2"].AppliedVersion != 3 {
		t.Errorf("expected local state versions, got %+v", st.Clusters)
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
			resp := transport.SyncResponse{
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
			}
			signMockResponse(privKey, pubKey, &resp)
			_ = json.NewEncoder(w).Encode(resp)
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

	cfg := agent.Config{
		ServerURL:       server.URL + "/custom/monitor",
		Transport:       "https",
		Token:           "valid_token",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	err = agent.RunSyncCycle(context.Background(), cfg)
	if err != nil {
		t.Fatalf("runSyncCycle with subpath failed: %v", err)
	}

	if !ackReceived {
		t.Errorf("expected server with subpath to receive ACK for version 3")
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
				resp := transport.SyncResponse{
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
				}
				signMockResponse(privKey, pubKey, &resp)
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Phase 2: Cluster 2 unassigned/deleted
			resp := transport.SyncResponse{
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
			}
			signMockResponse(privKey, pubKey, &resp)
			_ = json.NewEncoder(w).Encode(resp)
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

	cfg := agent.Config{
		ServerURL:       server.URL,
		Transport:       "https",
		StorageDir:      storageDir,
		ProxyType:       "nginx",
		ReloadCmd:       "true",
		ServerPublicKey: pubKey,
	}

	// 1. Initial sync (deploy clusters 1 and 2)
	if err := agent.RunSyncCycle(context.Background(), cfg); err != nil {
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
	if err := agent.RunSyncCycle(context.Background(), cfg); err != nil {
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
	st := agent.LoadState(filepath.Join(storageDir, "agent_state.json"))
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
