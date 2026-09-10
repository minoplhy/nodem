package transport_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/db/sqlite"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func setupTestDB(t *testing.T) (*sqlite.SqliteRepository, *db.User, *db.ECHCluster) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_transport.db")
	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := context.Background()
	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	user, err := repo.CreateUser(ctx, "admin_transport", "hash", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	pubKey, privKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("GenerateSigningKeyPair failed: %v", err)
	}

	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Transport Cluster", "cover.trans.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, pubKey, privKey)
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	// Increment cluster version to 1 and save an active key
	now := time.Now().UTC()
	next := now.Add(168 * time.Hour)
	_, _ = repo.IncrementClusterVersion(ctx, cluster.ID, now, next)
	_, _ = repo.SaveNewECHKey(ctx, cluster.ID, 1, "BASE64_ECH_KEY", "PRIV_KEY", "ECH_CONFIG", "-----BEGIN ECHCONFIG-----\nKEY1\n-----END ECHCONFIG-----")

	return repo, user, cluster
}

func TestPullServiceTransportExclusivity(t *testing.T) {
	repo, user, cluster := setupTestDB(t)
	service := transport.NewPullService(repo)
	ctx := context.Background()

	// 1. Create HTTPS node
	httpsToken := "token_https_123"
	httpsHash := transport.HashAgentToken(httpsToken)
	nodeHTTPS, err := repo.CreateECHNode(ctx, user.ID, "node-https", db.PullTransportHTTPS, &httpsHash, nil, db.ProxyTypeCaddy)
	if err != nil {
		t.Fatalf("CreateECHNode HTTPS failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, nodeHTTPS.ID)

	// 2. Create SSH node
	sshPubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG... agent@test"
	nodeSSH, err := repo.CreateECHNode(ctx, user.ID, "node-ssh", db.PullTransportSSH, nil, &sshPubKey, db.ProxyTypeNginx)
	if err != nil {
		t.Fatalf("CreateECHNode SSH failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, nodeSSH.ID)

	// HTTPS node calling via HTTPS transport -> SUCCESS
	authNode, err := service.AuthenticateByToken(ctx, httpsToken, db.PullTransportHTTPS)
	if err != nil || authNode.ID != nodeHTTPS.ID {
		t.Fatalf("expected HTTPS auth to succeed, got: %v", err)
	}

	// HTTPS node calling via SSH transport -> REJECTED (ErrTransportMismatch)
	_, err = service.AuthenticateByToken(ctx, httpsToken, db.PullTransportSSH)
	if err == nil || !strings.Contains(err.Error(), "transport mismatch") {
		t.Fatalf("expected transport mismatch error, got: %v", err)
	}

	// SSH node calling via SSH transport -> SUCCESS
	authSSHNode, err := service.AuthenticateBySSHPublicKey(ctx, sshPubKey, db.PullTransportSSH)
	if err != nil || authSSHNode.ID != nodeSSH.ID {
		t.Fatalf("expected SSH auth to succeed, got: %v", err)
	}

	// SSH node calling via HTTPS transport -> REJECTED
	_, err = service.AuthenticateBySSHPublicKey(ctx, sshPubKey, db.PullTransportHTTPS)
	if err == nil || !strings.Contains(err.Error(), "transport mismatch") {
		t.Fatalf("expected transport mismatch error, got: %v", err)
	}
}

func TestPullServiceSyncAndAck(t *testing.T) {
	repo, user, cluster := setupTestDB(t)
	service := transport.NewPullService(repo)
	ctx := context.Background()

	token := "token_test_abc"
	tokenHash := transport.HashAgentToken(token)
	node, err := repo.CreateECHNode(ctx, user.ID, "node-test", db.PullTransportHTTPS, &tokenHash, nil, db.ProxyTypeNginx)
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, node.ID)

	// 1. Sync without reported version -> expects NEW_KEY in Clusters
	syncResp, err := service.SyncNode(ctx, node, transport.SyncRequest{
		Clusters: map[string]int64{},
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("SyncNode failed: %v", err)
	}
	if syncResp.Status != "UPDATES_AVAILABLE" || len(syncResp.Clusters) != 1 {
		t.Fatalf("unexpected SyncResponse: %+v", syncResp)
	}
	cItem := syncResp.Clusters[0]
	if cItem.Status != "NEW_KEY" || cItem.Version != 1 || cItem.Keys == nil || cItem.Signature == nil {
		t.Fatalf("unexpected ClusterSyncItem: %+v", cItem)
	}

	// Verify Server-Signed Checksum & Domain-Separated Signature
	serverPubKey, _, err := repo.GetOrCreateServerSigningKey(ctx)
	if err != nil {
		t.Fatalf("failed getting server signing key: %v", err)
	}
	if cItem.Signature.PublicKey != serverPubKey {
		t.Fatalf("expected server public key %s, got %s", serverPubKey, cItem.Signature.PublicKey)
	}
	if cItem.Signature.Checksum != cItem.Keys.Checksum() {
		t.Fatalf("checksum mismatch: expected %s, got %s", cItem.Keys.Checksum(), cItem.Signature.Checksum)
	}
	signedMsg := transport.BuildSignableMessage(cItem.Signature.Type, cItem.ClusterID, cItem.Version, cItem.Signature.Checksum)
	verified := engine.VerifySignature(serverPubKey, signedMsg, cItem.Signature.SigBase64)
	if !verified {
		t.Fatalf("failed to verify server ed25519 signature on sync keys payload")
	}

	// 2. Ack version 1
	err = service.AckNode(ctx, node, transport.AckRequest{
		Acks: []transport.ClusterAckItem{
			{
				ClusterID:      cluster.ID,
				ClusterName:    cluster.Name,
				AppliedVersion: 1,
				Status:         "SUCCESS",
				Message:        "nginx reloaded",
			},
		},
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("AckNode failed: %v", err)
	}

	// 3. Sync with version 1 -> expects UP_TO_DATE
	syncResp2, err := service.SyncNode(ctx, node, transport.SyncRequest{
		Clusters: map[string]int64{cluster.Name: 1},
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("SyncNode 2 failed: %v", err)
	}
	if syncResp2.Status != "UP_TO_DATE" || syncResp2.Clusters[0].Status != "UP_TO_DATE" {
		t.Fatalf("expected UP_TO_DATE, got %+v", syncResp2)
	}
}

func TestEmbeddedSSHServer(t *testing.T) {
	repo, user, cluster := setupTestDB(t)
	service := transport.NewPullService(repo)

	// Pick a free random local TCP port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := uint16(l.Addr().(*net.TCPAddr).Port)
	_ = l.Close()

	// Generate client SSH key
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatalf("NewSignerFromKey failed: %v", err)
	}
	clientPubKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(clientSigner.PublicKey())))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register SSH node in DB and assign to cluster
	nodeSSH, err := repo.CreateECHNode(ctx, user.ID, "node-ssh-test", db.PullTransportSSH, nil, &clientPubKeyStr, db.ProxyTypeNginx)
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, nodeSSH.ID)

	sshServer, err := transport.NewSSHServer(service, port)
	if err != nil {
		t.Fatalf("NewSSHServer failed: %v", err)
	}

	serverErrChan := make(chan error, 1)
	go func() {
		serverErrChan <- sshServer.Start(ctx)
	}()

	// Wait briefly for server listener to spin up
	time.Sleep(100 * time.Millisecond)

	// Connect SSH Client
	clientConfig := &ssh.ClientConfig{
		User:            "sync",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port), clientConfig)
	if err != nil {
		t.Fatalf("ssh.Dial failed: %v", err)
	}
	defer client.Close()

	// 1. Run sync command
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	out, err := session.CombinedOutput("sync")
	if err != nil {
		t.Fatalf("session.CombinedOutput sync failed: %v (output: %s)", err, string(out))
	}

	var resp transport.SyncResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed unmarshaling sync response from SSH: %v, raw: %s", err, string(out))
	}
	if resp.Status != "UPDATES_AVAILABLE" || len(resp.Clusters) == 0 || resp.Clusters[0].Status != "NEW_KEY" {
		t.Errorf("unexpected ssh sync response: %+v", resp)
	}

	// 2. Run ack command
	session2, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession 2 failed: %v", err)
	}
	ackOut, err := session2.CombinedOutput(fmt.Sprintf("ack --cluster %q --version 1 --status SUCCESS --msg \"reloaded via ssh\"", cluster.Name))
	if err != nil {
		t.Fatalf("session.CombinedOutput ack failed: %v (output: %s)", err, string(ackOut))
	}
	if !strings.Contains(string(ackOut), "OK") {
		t.Errorf("unexpected ack output: %s", string(ackOut))
	}

	// Verify DB state updated
	clusterNodes, err := repo.ListClusterNodes(ctx, cluster.ID)
	if err != nil || len(clusterNodes) == 0 || clusterNodes[0].SyncStatus != db.SyncStatusInSync || clusterNodes[0].LastAppliedVersion != 1 {
		t.Errorf("unexpected clusterNodes state: %+v, err: %v", clusterNodes, err)
	}

	// Cancel context to stop SSH server
	cancel()
	select {
	case err := <-serverErrChan:
		if err != nil {
			t.Errorf("unexpected server exit error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("sshServer.Start did not exit promptly after context cancel")
	}
}

func TestPullServiceSyncNodeDetectsRemovedClusters(t *testing.T) {
	repo, user, cluster1 := setupTestDB(t)
	service := transport.NewPullService(repo)
	ctx := context.Background()

	pubKey2, privKey2, _ := engine.GenerateSigningKeyPair()
	cluster2, err := repo.CreateECHCluster(ctx, user.ID, "Second Cluster", "cover2.trans.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, pubKey2, privKey2)
	if err != nil {
		t.Fatalf("CreateECHCluster 2 failed: %v", err)
	}

	token := "node_token_detect_removed"
	tokenHash := transport.HashAgentToken(token)
	node, err := repo.CreateECHNode(ctx, user.ID, "node-multi", db.PullTransportHTTPS, &tokenHash, nil, db.ProxyTypeNginx)
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}

	// Initially assign node to cluster 1 and cluster 2
	_ = repo.SetNodeClusters(ctx, node.ID, []int64{cluster1.ID, cluster2.ID})

	// 1. Sync when both clusters assigned and up to date
	req := transport.SyncRequest{
		Clusters: map[string]int64{
			fmt.Sprintf("%d", cluster1.ID): 1,
			fmt.Sprintf("%d", cluster2.ID): 0,
		},
	}
	resp, err := service.SyncNode(ctx, node, req, "127.0.0.1")
	if err != nil {
		t.Fatalf("SyncNode failed: %v", err)
	}
	if len(resp.RemovedClusterIDs) > 0 || len(resp.RemovedClusters) > 0 {
		t.Errorf("expected no removed clusters, got: IDs=%v, names=%v", resp.RemovedClusterIDs, resp.RemovedClusters)
	}

	// 2. Unassign cluster 2 from node (node only has cluster 1 now)
	_ = repo.SetNodeClusters(ctx, node.ID, []int64{cluster1.ID})

	// Agent still reports cluster 2 (both by ID and by legacy name)
	reqRemoved := transport.SyncRequest{
		Clusters: map[string]int64{
			fmt.Sprintf("%d", cluster1.ID): 1,
			fmt.Sprintf("%d", cluster2.ID): 0,
			"Old Cluster Name":             1,
		},
	}
	resp2, err := service.SyncNode(ctx, node, reqRemoved, "127.0.0.1")
	if err != nil {
		t.Fatalf("SyncNode 2 failed: %v", err)
	}

	if resp2.Status != "UPDATES_AVAILABLE" {
		t.Errorf("expected UPDATES_AVAILABLE due to removed clusters, got: %s", resp2.Status)
	}

	foundID := false
	for _, id := range resp2.RemovedClusterIDs {
		if id == cluster2.ID {
			foundID = true
		}
	}
	if !foundID {
		t.Errorf("expected cluster2.ID %d in RemovedClusterIDs: %v", cluster2.ID, resp2.RemovedClusterIDs)
	}

	foundName := false
	for _, name := range resp2.RemovedClusters {
		if name == "Old Cluster Name" {
			foundName = true
		}
	}
	if !foundName {
		t.Errorf("expected 'Old Cluster Name' in RemovedClusters: %v", resp2.RemovedClusters)
	}

	// 3. Test AckNode with REMOVED status
	ackReq := transport.AckRequest{
		Acks: []transport.ClusterAckItem{
			{
				ClusterID:   cluster2.ID,
				ClusterName: cluster2.Name,
				Status:      "REMOVED",
				Message:     "decommissioned",
			},
		},
	}
	if err := service.AckNode(ctx, node, ackReq, "127.0.0.1"); err != nil {
		t.Fatalf("AckNode failed on REMOVED: %v", err)
	}
}

func TestServerSignedChecksum_SecuritySuite(t *testing.T) {
	ctx := context.Background()
	repo, err := sqlite.New(filepath.Join(t.TempDir(), "security_test.db"))
	if err != nil {
		t.Fatalf("failed creating sqlite repo: %v", err)
	}
	defer repo.Close()

	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("failed initializing db: %v", err)
	}

	user, err := repo.CreateUser(ctx, "sec_admin", "hash", "ADMIN")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	pubKey, privKey, _ := engine.GenerateSigningKeyPair()
	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Sec Cluster", "cover.sec.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, pubKey, privKey)
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	_, err = repo.SaveNewECHKey(ctx, cluster.ID, 1, "base64ech", "privkeypem", "configpem", "fullpem")
	if err != nil {
		t.Fatalf("SaveNewECHKey failed: %v", err)
	}
	_, _ = repo.IncrementClusterVersion(ctx, cluster.ID, time.Now(), time.Now().Add(168*time.Hour))

	tokenHash := transport.HashAgentToken("sec-token")
	node, err := repo.CreateECHNode(ctx, user.ID, "sec-edge", db.PullTransportHTTPS, &tokenHash, nil, "NGINX")
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	_ = repo.AssignNodeToCluster(ctx, cluster.ID, node.ID)

	service := transport.NewPullService(repo)

	// Fetch server key (this is the pinned key that edge agents hold)
	pinnedServerPubKey, _, err := repo.GetOrCreateServerSigningKey(ctx)
	if err != nil {
		t.Fatalf("GetOrCreateServerSigningKey failed: %v", err)
	}

	syncResp, err := service.SyncNode(ctx, node, transport.SyncRequest{}, "127.0.0.1")
	if err != nil {
		t.Fatalf("SyncNode failed: %v", err)
	}
	if len(syncResp.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(syncResp.Clusters))
	}
	item := syncResp.Clusters[0]

	// 1. Valid key signature & checksum verification succeeds
	if item.Signature.Checksum != item.Keys.Checksum() {
		t.Fatalf("expected checksum match")
	}
	validMsg := transport.BuildSignableMessage(item.Signature.Type, item.ClusterID, item.Version, item.Signature.Checksum)
	if !engine.VerifySignature(pinnedServerPubKey, validMsg, item.Signature.SigBase64) {
		t.Fatalf("valid signature failed verification against pinned server public key")
	}

	// 2. Full SyncResponse Root Payload signature & checksum verification succeeds
	if syncResp.Signature == nil {
		t.Fatalf("expected non-nil syncResp.Signature")
	}
	if syncResp.Signature.Type != transport.PayloadTypeSyncPayload {
		t.Fatalf("expected signature type %s, got %s", transport.PayloadTypeSyncPayload, syncResp.Signature.Type)
	}
	if syncResp.Signature.Checksum != syncResp.PayloadChecksum() {
		t.Fatalf("expected payload checksum match")
	}
	validPayloadMsg := transport.BuildSignableMessage(transport.PayloadTypeSyncPayload, syncResp.NodeID, syncResp.Timestamp, syncResp.Signature.Checksum)
	if !engine.VerifySignature(pinnedServerPubKey, validPayloadMsg, syncResp.Signature.SigBase64) {
		t.Fatalf("valid full payload signature failed verification against pinned server public key")
	}

	// 3. Full Payload Content Tampering Detection:
	// (a) Tampering with RemovedClusterIDs changes PayloadChecksum
	tamperedResp := *syncResp
	tamperedResp.RemovedClusterIDs = []int64{999}
	if tamperedResp.PayloadChecksum() == syncResp.Signature.Checksum {
		t.Fatalf("expected tampered RemovedClusterIDs to break payload checksum")
	}

	// (b) Tampering with Status changes PayloadChecksum
	tamperedStatus := *syncResp
	tamperedStatus.Status = "FORGED_STATUS"
	if tamperedStatus.PayloadChecksum() == syncResp.Signature.Checksum {
		t.Fatalf("expected tampered Status to break payload checksum")
	}

	// (c) Tampering with cluster PublicName changes PayloadChecksum
	tamperedClusters := []transport.ClusterSyncItem{syncResp.Clusters[0]}
	tamperedClusters[0].PublicName = "attacker-domain.com"
	tamperedClusterResp := *syncResp
	tamperedClusterResp.Clusters = tamperedClusters
	if tamperedClusterResp.PayloadChecksum() == syncResp.Signature.Checksum {
		t.Fatalf("expected tampered PublicName to break payload checksum")
	}

	// (d) Tampering with ECH key PEM changes PayloadChecksum
	tamperedPEMKeys := *syncResp.Clusters[0].Keys
	tamperedPEMKeys.ECHCurrentPEM = "INJECTED_KEY"
	tamperedPEMClusters := []transport.ClusterSyncItem{syncResp.Clusters[0]}
	tamperedPEMClusters[0].Keys = &tamperedPEMKeys
	tamperedPEMResp := *syncResp
	tamperedPEMResp.Clusters = tamperedPEMClusters
	if tamperedPEMResp.PayloadChecksum() == syncResp.Signature.Checksum {
		t.Fatalf("expected tampered ECHCurrentPEM to break payload checksum")
	}

	// 4. Tampered Key data detected by individual item checksum
	tamperedKeys := *item.Keys
	tamperedKeys.PrivateKeyPEM = "tampered_evil_private_key"
	if tamperedKeys.Checksum() == item.Signature.Checksum {
		t.Fatalf("expected tampered keys to produce different checksum")
	}

	// 5. Domain separation / PayloadType tampering fails signature
	wrongTypeMsg := transport.BuildSignableMessage("FORGED_TYPE", item.ClusterID, item.Version, item.Signature.Checksum)
	if engine.VerifySignature(pinnedServerPubKey, wrongTypeMsg, item.Signature.SigBase64) {
		t.Fatalf("signature verification should have failed with forged payload type")
	}

	// 6. Cross-cluster or version replay tampering fails signature
	wrongClusterMsg := transport.BuildSignableMessage(item.Signature.Type, 9999, item.Version, item.Signature.Checksum)
	if engine.VerifySignature(pinnedServerPubKey, wrongClusterMsg, item.Signature.SigBase64) {
		t.Fatalf("signature verification should have failed with mismatched cluster ID")
	}
	wrongVersionMsg := transport.BuildSignableMessage(item.Signature.Type, item.ClusterID, 9999, item.Signature.Checksum)
	if engine.VerifySignature(pinnedServerPubKey, wrongVersionMsg, item.Signature.SigBase64) {
		t.Fatalf("signature verification should have failed with mismatched version")
	}

	// 7. DNS Poisoning / Rogue Server simulation:
	// Attacker controls the server, signs with attacker's private key, and supplies attacker's public key in the payload.
	roguePub, roguePriv, _ := engine.GenerateSigningKeyPair()
	rogueSig, _ := engine.SignPayload(roguePriv, validPayloadMsg)

	// If the agent verifies against the rogue public key sent over the wire (the old vulnerability):
	if !engine.VerifySignature(roguePub, validPayloadMsg, rogueSig) {
		t.Fatalf("rogue signature should be self-consistent with rogue key")
	}

	// But when the agent enforces its PINNED server public key (our mitigation):
	if engine.VerifySignature(pinnedServerPubKey, validPayloadMsg, rogueSig) {
		t.Fatalf("CRITICAL: Pinned server public key must reject rogue signature from spoofed server!")
	}
}

