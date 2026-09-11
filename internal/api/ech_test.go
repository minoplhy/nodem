package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/minoplhy/nodem/internal/api"
	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/db/sqlite"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func setupAPITestDB(t *testing.T) (*sqlite.SqliteRepository, *db.User, *db.ECHCluster, http.Handler) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_api_ech.db")
	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed opening test db: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := context.Background()
	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	user, err := repo.CreateUser(ctx, "api_admin", "hash", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	pubKey, privKey, _ := engine.GenerateSigningKeyPair()
	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Web Cluster", "cover.web.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, pubKey, privKey)
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	_, _ = repo.IncrementClusterVersion(ctx, cluster.ID, time.Now().UTC(), time.Now().UTC().Add(168*time.Hour))
	_, _ = repo.SaveNewECHKey(ctx, cluster.ID, 1, "BASE64_KEY", "PRIV", "CONF", "-----BEGIN ECHCONFIG-----\nKEY1\n-----END ECHCONFIG-----")

	tmpDir := t.TempDir()
	fakeOpenSSL := filepath.Join(tmpDir, "fake_openssl.sh")
	fakeScript := `#!/bin/sh
if [ "$1" = "ech" ] && [ "$2" = "-help" ]; then
  exit 0
fi
out=""
while [ $# -gt 0 ]; do
  if [ "$1" = "-out" ]; then
    out="$2"
    shift 2
  else
    shift
  fi
done
if [ -n "$out" ]; then
  cat << 'PEM' > "$out"
-----BEGIN PRIVATE KEY-----
dGVzdF9wcml2YXRlX2tleV8zMl9ieXRlc19sb25nISE=
-----END PRIVATE KEY-----
-----BEGIN ECHCONFIG-----
dGVzdF9lY2hjb25maWdfYnl0ZXNfbG9uZ19oZXJlISE=
-----END ECHCONFIG-----
PEM
fi
exit 0
`
	if err := os.WriteFile(fakeOpenSSL, []byte(fakeScript), 0755); err != nil {
		t.Fatalf("failed creating fake openssl: %v", err)
	}

	state := &api.AppState{
		Repo:           repo,
		BootstrapToken: "bootstrap_token",
		OpenSSLPath:    fakeOpenSSL,
	}

	router := api.BuildRouter(state)
	return repo, user, cluster, router
}

func TestAgentHTTPSPullAndAck(t *testing.T) {
	repo, user, cluster, router := setupAPITestDB(t)
	ctx := context.Background()

	// 1. Create HTTPS node (independent entity under user/tenant)
	token := "agt_live_secret12345"
	tokenHash := transport.HashAgentToken(token)
	node, err := repo.CreateECHNode(ctx, user.ID, "web-edge-01", db.PullTransportHTTPS, &tokenHash, nil, db.ProxyTypeCaddy)
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}

	// Assign node to cluster
	if err := repo.AssignNodeToCluster(ctx, cluster.ID, node.ID); err != nil {
		t.Fatalf("AssignNodeToCluster failed: %v", err)
	}

	// 2. Request without header -> 401 Unauthorized
	req1 := httptest.NewRequest("POST", "/api/v1/agent/ech/sync", bytes.NewReader([]byte(`{"clusters":{"Web Cluster":0}}`)))
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", rec1.Code)
	}

	// 3. Request with valid X-Agent-Token -> 200 OK with UPDATES_AVAILABLE and cluster keys
	req2 := httptest.NewRequest("POST", "/api/v1/agent/ech/sync", bytes.NewReader([]byte(`{"clusters":{"Web Cluster":0}}`)))
	req2.Header.Set("X-Agent-Token", token)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d (body: %s)", rec2.Code, rec2.Body.String())
	}

	var syncResp transport.SyncResponse
	if err := json.NewDecoder(rec2.Body).Decode(&syncResp); err != nil {
		t.Fatalf("failed decoding sync response: %v", err)
	}
	if syncResp.Status != "UPDATES_AVAILABLE" || len(syncResp.Clusters) != 1 {
		t.Fatalf("unexpected sync response: %+v", syncResp)
	}
	cItem := syncResp.Clusters[0]
	if cItem.Status != "NEW_KEY" || cItem.Version != 1 || cItem.Keys == nil {
		t.Errorf("unexpected cluster sync item: %+v", cItem)
	}

	// 4. Send Ack -> 200 OK
	ackReqObj := transport.AckRequest{
		Acks: []transport.ClusterAckItem{
			{
				ClusterID:      cluster.ID,
				ClusterName:    "Web Cluster",
				AppliedVersion: 1,
				Status:         "SUCCESS",
				Message:        "caddy reloaded successfully",
			},
		},
	}
	ackBytes, _ := json.Marshal(ackReqObj)
	req3 := httptest.NewRequest("POST", "/api/v1/agent/ech/ack", bytes.NewReader(ackBytes))
	req3.Header.Set("X-Agent-Token", token)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid ack, got %d (body: %s)", rec3.Code, rec3.Body.String())
	}

	// 5. Verify Node state updated in DB (in ech_cluster_nodes)
	cNodes, err := repo.ListClusterNodes(ctx, cluster.ID)
	if err != nil || len(cNodes) != 1 {
		t.Fatalf("unexpected cluster nodes: %+v, err: %v", cNodes, err)
	}
	if cNodes[0].SyncStatus != db.SyncStatusInSync || cNodes[0].LastAppliedVersion != 1 {
		t.Errorf("unexpected cluster node state: %+v", cNodes[0])
	}

	// 6. Request sync again with version 1 -> UP_TO_DATE
	req4 := httptest.NewRequest("POST", "/api/v1/agent/ech/sync", bytes.NewReader([]byte(`{"clusters":{"Web Cluster":1}}`)))
	req4.Header.Set("X-Agent-Token", token)
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec4.Code)
	}
	var syncResp2 transport.SyncResponse
	_ = json.NewDecoder(rec4.Body).Decode(&syncResp2)
	if syncResp2.Status != "UP_TO_DATE" {
		t.Errorf("expected UP_TO_DATE, got %s", syncResp2.Status)
	}
}

func TestTransportMismatchRejection(t *testing.T) {
	repo, user, cluster, router := setupAPITestDB(t)
	ctx := context.Background()

	// Register an SSH-only node
	sshKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG... ssh_agent"
	token := "agt_ssh_only_token"
	tokenHash := transport.HashAgentToken(token)
	node, err := repo.CreateECHNode(ctx, user.ID, "ssh-edge-01", db.PullTransportSSH, &tokenHash, &sshKey, db.ProxyTypeNginx)
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, node.ID)

	// Attempt to call HTTPS endpoint with SSH node's token -> MUST BE 403 Forbidden!
	req := httptest.NewRequest("POST", "/api/v1/agent/ech/sync", bytes.NewReader([]byte(`{"clusters":{"Web Cluster":0}}`)))
	req.Header.Set("X-Agent-Token", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for SSH node calling HTTPS endpoint, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestAgentHTTPSPullAndAckWithBasePath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_api_ech_subpath.db")
	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed opening test db: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := context.Background()
	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	user, err := repo.CreateUser(ctx, "subpath_admin", "hash", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	pubKey, privKey, _ := engine.GenerateSigningKeyPair()
	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Subpath Cluster", "cover.subpath.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, pubKey, privKey)
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	_, _ = repo.IncrementClusterVersion(ctx, cluster.ID, time.Now().UTC(), time.Now().UTC().Add(168*time.Hour))
	_, _ = repo.SaveNewECHKey(ctx, cluster.ID, 1, "BASE64_KEY", "PRIV", "CONF", "-----BEGIN ECHCONFIG-----\nKEY1\n-----END ECHCONFIG-----")

	// Router configured with BASE_PATH = "/monitor"
	state := &api.AppState{
		Repo:           repo,
		BootstrapToken: "bootstrap_token",
		BasePath:       "/monitor",
	}
	router := api.BuildRouter(state)

	token := "agt_subpath_token_123"
	tokenHash := transport.HashAgentToken(token)
	node, err := repo.CreateECHNode(ctx, user.ID, "edge-subpath-01", db.PullTransportHTTPS, &tokenHash, nil, db.ProxyTypeNginx)
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, node.ID)

	// 1. Sync over subpath
	syncReq := httptest.NewRequest("POST", "/monitor/api/v1/agent/ech/sync", bytes.NewReader([]byte(`{"clusters":{"Subpath Cluster":0}}`)))
	syncReq.Header.Set("X-Agent-Token", token)
	recSync := httptest.NewRecorder()
	router.ServeHTTP(recSync, syncReq)

	if recSync.Code != http.StatusOK {
		t.Fatalf("expected 200 on /monitor/api/v1/agent/ech/sync, got %d (body: %s)", recSync.Code, recSync.Body.String())
	}

	var syncResp transport.SyncResponse
	if err := json.NewDecoder(recSync.Body).Decode(&syncResp); err != nil {
		t.Fatalf("failed decoding sync response: %v", err)
	}
	if syncResp.Status != "UPDATES_AVAILABLE" || len(syncResp.Clusters) != 1 || syncResp.Clusters[0].Version != 1 {
		t.Errorf("unexpected sync response on subpath: %+v", syncResp)
	}

	// 2. Ack over subpath
	ackReqObj := transport.AckRequest{
		Acks: []transport.ClusterAckItem{
			{
				ClusterID:      cluster.ID,
				ClusterName:    "Subpath Cluster",
				AppliedVersion: 1,
				Status:         "SUCCESS",
			},
		},
	}
	ackBytes, _ := json.Marshal(ackReqObj)
	ackReq := httptest.NewRequest("POST", "/monitor/api/v1/agent/ech/ack", bytes.NewReader(ackBytes))
	ackReq.Header.Set("X-Agent-Token", token)
	recAck := httptest.NewRecorder()
	router.ServeHTTP(recAck, ackReq)

	if recAck.Code != http.StatusOK {
		t.Fatalf("expected 200 on /monitor/api/v1/agent/ech/ack, got %d (body: %s)", recAck.Code, recAck.Body.String())
	}
}

func TestIndependentECHNodeCRUDAndClusterAssignment(t *testing.T) {
	repo, user, cluster, router := setupAPITestDB(t)
	ctx := context.Background()

	// Authenticate via session cookie
	sessionID := "test_session_cookie_token"
	_ = repo.CreateSession(ctx, sessionID, "pub_test", user.ID, time.Now().Add(24*time.Hour), nil, nil)

	// Create second cluster
	pubKey2, privKey2, _ := engine.GenerateSigningKeyPair()
	cluster2, err := repo.CreateECHCluster(ctx, user.ID, "Secondary Cluster", "cover2.web.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, pubKey2, privKey2)
	if err != nil {
		t.Fatalf("Create second cluster failed: %v", err)
	}

	// 1. POST /api/ech/nodes (create independent node with initial cluster)
	createBody := api.CreateNodeRequest{
		Name:          "edge-global-01",
		PullTransport: db.PullTransportHTTPS,
		ProxyType:     db.ProxyTypeNginx,
		ClusterIDs:    []int64{cluster.ID},
	}
	createJSON, _ := json.Marshal(createBody)
	req1 := httptest.NewRequest("POST", "/api/ech/nodes", bytes.NewReader(createJSON))
	req1.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("POST /api/ech/nodes failed: %d, body: %s", rec1.Code, rec1.Body.String())
	}

	var createResp api.CreateNodeResponse
	if err := json.NewDecoder(rec1.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode create node response: %v", err)
	}
	if createResp.Node == nil || createResp.AgentToken == "" {
		t.Fatalf("invalid create node response: %+v", createResp)
	}
	nodeID := createResp.Node.ID

	// 2. GET /api/ech/nodes (list all tenant nodes)
	req2 := httptest.NewRequest("GET", "/api/ech/nodes", nil)
	req2.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /api/ech/nodes failed: %d", rec2.Code)
	}
	var nodesList []db.ECHNode
	_ = json.NewDecoder(rec2.Body).Decode(&nodesList)
	if len(nodesList) != 1 || nodesList[0].ID != nodeID {
		t.Errorf("expected 1 node in list, got %+v", nodesList)
	}

	// 3. PUT /api/ech/nodes/{id}/clusters (assign both clusters)
	setClustersBody := api.SetNodeClustersRequest{
		ClusterIDs: []int64{cluster.ID, cluster2.ID},
	}
	setJSON, _ := json.Marshal(setClustersBody)
	req3 := httptest.NewRequest("PUT", "/api/ech/nodes/"+strconv.FormatInt(nodeID, 10)+"/clusters", bytes.NewReader(setJSON))
	req3.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("PUT /api/ech/nodes/{id}/clusters failed: %d, body: %s", rec3.Code, rec3.Body.String())
	}

	// 4. GET /api/ech/nodes/{id} (check detail has both clusters)
	req4 := httptest.NewRequest("GET", "/api/ech/nodes/"+strconv.FormatInt(nodeID, 10), nil)
	req4.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusOK {
		t.Fatalf("GET /api/ech/nodes/{id} failed: %d", rec4.Code)
	}
	var nodeDetail db.ECHNode
	_ = json.NewDecoder(rec4.Body).Decode(&nodeDetail)
	if len(nodeDetail.Clusters) != 2 {
		t.Errorf("expected 2 clusters on node detail, got %+v", nodeDetail.Clusters)
	}

	// 5. GET /api/ech/clusters/{id}/nodes
	req5 := httptest.NewRequest("GET", "/api/ech/clusters/"+strconv.FormatInt(cluster2.ID, 10)+"/nodes", nil)
	req5.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)

	if rec5.Code != http.StatusOK {
		t.Fatalf("GET /api/ech/clusters/{id}/nodes failed: %d", rec5.Code)
	}
	var clusterNodes []db.ECHClusterNode
	_ = json.NewDecoder(rec5.Body).Decode(&clusterNodes)
	if len(clusterNodes) != 1 || clusterNodes[0].NodeID != nodeID {
		t.Errorf("expected node in cluster2 nodes list, got %+v", clusterNodes)
	}

	// 6. DELETE /api/ech/clusters/{id}/nodes/{node_id} (unassign from cluster2)
	req6 := httptest.NewRequest("DELETE", "/api/ech/clusters/"+strconv.FormatInt(cluster2.ID, 10)+"/nodes/"+strconv.FormatInt(nodeID, 10), nil)
	req6.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec6 := httptest.NewRecorder()
	router.ServeHTTP(rec6, req6)

	if rec6.Code != http.StatusOK {
		t.Fatalf("DELETE /api/ech/clusters/{id}/nodes/{node_id} failed: %d", rec6.Code)
	}

	// Verify unassigned from cluster2
	cluster2Nodes, _ := repo.ListClusterNodes(ctx, cluster2.ID)
	if len(cluster2Nodes) != 0 {
		t.Errorf("expected 0 nodes in cluster2, got %d", len(cluster2Nodes))
	}

	// 7. DELETE /api/ech/nodes/{id} (delete node entirely)
	req7 := httptest.NewRequest("DELETE", "/api/ech/nodes/"+strconv.FormatInt(nodeID, 10), nil)
	req7.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec7 := httptest.NewRecorder()
	router.ServeHTTP(rec7, req7)

	if rec7.Code != http.StatusOK {
		t.Fatalf("DELETE /api/ech/nodes/{id} failed: %d", rec7.Code)
	}

	deletedNode, _ := repo.GetECHNode(ctx, nodeID)
	if deletedNode != nil {
		t.Errorf("expected node to be deleted, got %+v", deletedNode)
	}
}

func TestGetServerPublicKeyEndpoint(t *testing.T) {
	repo, user, _, router := setupAPITestDB(t)
	ctx := context.Background()

	sessionID := "test_session_server_key"
	_ = repo.CreateSession(ctx, sessionID, "pub_test", user.ID, time.Now().Add(24*time.Hour), nil, nil)

	req := httptest.NewRequest("GET", "/api/ech/server-key", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/ech/server-key failed: %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp api.ServerPublicKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if resp.ServerPublicKey == "" {
		t.Errorf("expected non-empty server_public_key")
	}
	if resp.Algorithm != "ed25519" {
		t.Errorf("expected algorithm ed25519, got %s", resp.Algorithm)
	}
}

func TestTriggerClusterRotationResetsDomainsToPending(t *testing.T) {
	repo, user, cluster, router := setupAPITestDB(t)
	ctx := context.Background()

	sessionID := "test_session_rotate"
	_ = repo.CreateSession(ctx, sessionID, "rotate_test", user.ID, time.Now().Add(24*time.Hour), nil, nil)

	// 1. Create a provider and domain for the cluster
	prov, err := repo.CreateProvider(ctx, user.ID, "Cloudflare-Test", "CLOUDFLARE", "", "mock_token", "zone123")
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	ipv4 := "192.0.2.1"
	dom, err := repo.CreateECHDomain(ctx, cluster.ID, prov.ID, nil, "app.example.com", 300, "h2,h3", &ipv4, nil)
	if err != nil {
		t.Fatalf("CreateECHDomain failed: %v", err)
	}

	// 2. Mark domain as SYNCED
	now := time.Now().UTC()
	if err := repo.UpdateECHDomainSyncStatus(ctx, dom.ID, "SYNCED", &now); err != nil {
		t.Fatalf("UpdateECHDomainSyncStatus failed: %v", err)
	}

	// 3. Trigger manual cluster rotation via API
	req := httptest.NewRequest("POST", "/api/ech/clusters/"+strconv.FormatInt(cluster.ID, 10)+"/rotate", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/ech/clusters/{id}/rotate failed: %d, body: %s", rec.Code, rec.Body.String())
	}

	// 4. Verify domain DNS status was reset to PENDING
	reloadedDom, err := repo.GetECHDomain(ctx, dom.ID)
	if err != nil || reloadedDom == nil {
		t.Fatalf("GetECHDomain failed: %v", err)
	}
	if reloadedDom.DNSStatus != "PENDING" {
		t.Errorf("expected domain dns_status to be PENDING after manual rotation, got: %s", reloadedDom.DNSStatus)
	}
}



