package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/agent"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

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

	validClusterSig := createMockSignature(privKey, pubKey, 1, 1, keys)

	baseItem := transport.ClusterSyncItem{
		ClusterID:   1,
		ClusterName: "test-cluster",
		PublicName:  "test.com",
		Status:      "NEW_KEY",
		Version:     1,
		Keys:        keys,
		Signature:   validClusterSig,
	}

	tests := []struct {
		name           string
		cfgKey         string
		setupState     func(st *agent.State)
		buildResponse  func() transport.SyncResponse
		expectedErrSub string
	}{
		{
			name:   "Missing pinned public key in agent config",
			cfgKey: "",
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{baseItem},
				}
				signMockResponse(privKey, pubKey, &resp)
				return resp
			},
			expectedErrSub: "missing required parameter: --server-public-key",
		},
		{
			name:   "Missing top-level signature in server payload",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				return transport.SyncResponse{
					Status:    "UPDATES_AVAILABLE",
					Clusters:  []transport.ClusterSyncItem{baseItem},
					Signature: nil,
				}
			},
			expectedErrSub: "sync response signature missing",
		},
		{
			name:   "DNS Poisoning attacker signed top-level response with rogue key",
			cfgKey: pubKey, // Agent pinned to legit pubKey
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{baseItem},
				}
				signMockResponse(roguePrivKey, roguePubKey, &resp)
				return resp
			},
			expectedErrSub: "sync response signature verification failed",
		},
		{
			name:   "Tampered top-level checksum in signature",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{baseItem},
				}
				signMockResponse(privKey, pubKey, &resp)
				resp.Signature.Checksum = "tampered_checksum_value"
				return resp
			},
			expectedErrSub: "sync response payload checksum mismatch",
		},
		{
			name:   "Tampered RemovedClusterIDs injected into signed response",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{baseItem},
				}
				signMockResponse(privKey, pubKey, &resp)
				resp.RemovedClusterIDs = []int64{999}
				return resp
			},
			expectedErrSub: "sync response payload checksum mismatch",
		},
		{
			name:   "Tampered Status injected into signed response",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{baseItem},
				}
				signMockResponse(privKey, pubKey, &resp)
				resp.Status = "UP_TO_DATE"
				return resp
			},
			expectedErrSub: "sync response payload checksum mismatch",
		},
		{
			name:   "Tampered cluster key payload with original top-level signature",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{baseItem},
				}
				signMockResponse(privKey, pubKey, &resp)
				resp.Clusters[0].Keys = &transport.SyncKeysPayload{
					Base64ECH:     "TAMPERED_KEY",
					ECHCurrentPEM: "TAMPERED_PEM",
				}
				return resp
			},
			expectedErrSub: "sync response payload checksum mismatch",
		},
		{
			name:   "Stale or replayed sync response detected",
			cfgKey: pubKey,
			setupState: func(st *agent.State) {
				st.LastSyncTimestamp = 100000
			},
			buildResponse: func() transport.SyncResponse {
				resp := transport.SyncResponse{
					Timestamp: 90000,
					Status:    "UP_TO_DATE",
				}
				signMockResponse(privKey, pubKey, &resp)
				return resp
			},
			expectedErrSub: "stale or replayed sync response detected",
		},
		{
			name:   "Missing per-cluster key signature",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				itemWithoutSig := baseItem
				itemWithoutSig.Signature = nil
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{itemWithoutSig},
				}
				signMockResponse(privKey, pubKey, &resp)
				return resp
			},
			expectedErrSub: "payload signature missing for cluster",
		},
		{
			name:   "Per-cluster key signature verification failed with rogue key",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				itemRogueSig := baseItem
				itemRogueSig.Signature = createMockSignature(roguePrivKey, roguePubKey, 1, 1, keys)
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{itemRogueSig},
				}
				signMockResponse(privKey, pubKey, &resp)
				return resp
			},
			expectedErrSub: "payload signature verification failed for cluster",
		},
		{
			name:   "Per-cluster key checksum mismatch",
			cfgKey: pubKey,
			buildResponse: func() transport.SyncResponse {
				itemBadChecksum := baseItem
				itemBadChecksum.Signature = &transport.SyncSignature{
					Type:      transport.PayloadTypeSyncKeyUpdate,
					Algorithm: "ed25519",
					Checksum:  "tampered_cluster_checksum",
					SigBase64: validClusterSig.SigBase64,
					PublicKey: pubKey,
				}
				resp := transport.SyncResponse{
					Status:   "UPDATES_AVAILABLE",
					Clusters: []transport.ClusterSyncItem{itemBadChecksum},
				}
				signMockResponse(privKey, pubKey, &resp)
				return resp
			},
			expectedErrSub: "payload checksum mismatch for cluster",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storageDir := t.TempDir()
			stateFile := filepath.Join(storageDir, "agent_state.json")

			if tc.setupState != nil {
				st := agent.State{Clusters: make(map[string]agent.ClusterState)}
				tc.setupState(&st)
				_ = agent.SaveState(stateFile, st)
			}

			resp := tc.buildResponse()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			cfg := agent.Config{
				ServerURL:       server.URL,
				Transport:       "https",
				StorageDir:      storageDir,
				ProxyType:       "nginx",
				ReloadCmd:       "true",
				ServerPublicKey: tc.cfgKey,
			}

			err := agent.RunSyncCycle(context.Background(), cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, but got nil", tc.expectedErrSub)
			}
			if !strings.Contains(err.Error(), tc.expectedErrSub) {
				t.Fatalf("expected error containing %q, got: %v", tc.expectedErrSub, err)
			}
		})
	}
}
