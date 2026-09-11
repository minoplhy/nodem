package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/minoplhy/nodem/internal/db/sqlite"
	"github.com/minoplhy/nodem/internal/ech/engine"
)

func TestECHCoordinator_DNSUpdateFlows(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_coordinator.db")
	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New failed: %v", err)
	}
	defer repo.Close()

	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	user, err := repo.CreateUser(ctx, "admin", "hash", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 1. Setup mock hook script
	logFile := filepath.Join(t.TempDir(), "hook.log")
	hookScript := filepath.Join(t.TempDir(), "dns_hook.sh")
	scriptContent := "#!/bin/sh\necho \"$DNS_ACTION $DNS_DOMAIN $DNS_ECH_BASE64\" >> " + logFile + "\nexit 0\n"
	if err := os.WriteFile(hookScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed writing hook script: %v", err)
	}

	prov, err := repo.CreateProvider(ctx, user.ID, "Hook-DNS", "HOOK", hookScript, "", "")
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	pubKey, privKey, _ := engine.GenerateSigningKeyPair()
	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Prod-Cluster", "cover.prod.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, false, pubKey, privKey)
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	now := time.Now().UTC()
	v1, err := repo.IncrementClusterVersion(ctx, cluster.ID, now, now.Add(168*time.Hour))
	if err != nil {
		t.Fatalf("IncrementClusterVersion failed: %v", err)
	}
	_, err = repo.SaveNewECHKey(ctx, cluster.ID, v1, "BASE64_KEY_V1", "PRIV", "CONF", "FULL")
	if err != nil {
		t.Fatalf("SaveNewECHKey failed: %v", err)
	}

	ipv4 := "203.0.113.1"
	dom, err := repo.CreateECHDomain(ctx, cluster.ID, prov.ID, nil, "service.prod.com", 300, "h2,h3", &ipv4, nil)
	if err != nil {
		t.Fatalf("CreateECHDomain failed: %v", err)
	}

	tokenHash := "node_token_hash"
	node, err := repo.CreateECHNode(ctx, user.ID, "Edge-1", "HTTPS", &tokenHash, nil, "NGINX")
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	if err := repo.SetNodeClusters(ctx, node.ID, []int64{cluster.ID}); err != nil {
		t.Fatalf("SetNodeClusters failed: %v", err)
	}

	// Scenario 1: Node has NOT synced yet (status OUTDATED) -> DNS must NOT shoot
	reconcileECHClusters(ctx, repo)
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Fatalf("expected no DNS update while node is OUTDATED")
	}
	d1, _ := repo.GetECHDomain(ctx, dom.ID)
	if d1.DNSStatus != "PENDING" {
		t.Fatalf("expected domain to remain PENDING, got %s", d1.DNSStatus)
	}

	// Scenario 2: Node syncs and ACKs version 1 -> reconcileECHClusters publishes to DNS
	if err := repo.RecordClusterNodeAck(ctx, cluster.ID, node.ID, v1, "IN_SYNC", "127.0.0.1", nil); err != nil {
		t.Fatalf("RecordClusterNodeAck failed: %v", err)
	}

	reconcileECHClusters(ctx, repo)

	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("expected hook.log to be created on sync, err: %v", err)
	}
	if !strings.Contains(string(logData), "update_https service.prod.com BASE64_KEY_V1") {
		t.Fatalf("unexpected hook log content: %s", string(logData))
	}
	d2, _ := repo.GetECHDomain(ctx, dom.ID)
	if d2.DNSStatus != "SYNCED" || d2.LastSyncedAt == nil {
		t.Fatalf("expected domain to be SYNCED, got status=%s, last_synced=%v", d2.DNSStatus, d2.LastSyncedAt)
	}

	// Scenario 3: Manual key rotation to Version 2!
	// Node syncs to Version 2.
	// Ensure reconcileECHClusters immediately shoots DNS with V2!
	_ = os.Remove(logFile)
	rotTime := time.Now().UTC().Add(-10 * time.Minute)
	v2, err := repo.IncrementClusterVersion(ctx, cluster.ID, rotTime, rotTime.Add(168*time.Hour))
	if err != nil {
		t.Fatalf("IncrementClusterVersion v2 failed: %v", err)
	}
	_, err = repo.SaveNewECHKey(ctx, cluster.ID, v2, "BASE64_KEY_V2", "PRIV", "CONF", "FULL")
	if err != nil {
		t.Fatalf("SaveNewECHKey v2 failed: %v", err)
	}

	// Node reports in-sync for v2
	if err := repo.RecordClusterNodeAck(ctx, cluster.ID, node.ID, v2, "IN_SYNC", "127.0.0.1", nil); err != nil {
		t.Fatalf("RecordClusterNodeAck v2 failed: %v", err)
	}

	reconcileECHClusters(ctx, repo)

	logDataV2, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("expected DNS update for V2 key, but hook.log was not created")
	}
	if !strings.Contains(string(logDataV2), "update_https service.prod.com BASE64_KEY_V2") {
		t.Fatalf("expected DNS update with BASE64_KEY_V2, got: %s", string(logDataV2))
	}

	// Scenario 4: Simulated legacy state where domain was SYNCED but with older timestamp than LastRotatedAt
	// Even if domain is in SYNCED status, reconcileECHClusters must detect stale key and update DNS
	_ = os.Remove(logFile)
	oldSyncTime := rotTime.Add(-5 * time.Minute)
	_ = repo.UpdateECHDomainSyncStatus(ctx, dom.ID, "SYNCED", &oldSyncTime)

	reconcileECHClusters(ctx, repo)

	logDataResync, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("expected stale SYNCED domain to be re-synced, but hook.log was not created")
	}
	if !strings.Contains(string(logDataResync), "update_https service.prod.com BASE64_KEY_V2") {
		t.Fatalf("expected re-sync with BASE64_KEY_V2, got: %s", string(logDataResync))
	}

	// Scenario 5: Domain is now genuinely up to date (LastSyncedAt > LastRotatedAt).
	// Next reconciliation must be idempotent and skip without calling DNS update.
	_ = os.Remove(logFile)
	reconcileECHClusters(ctx, repo)
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Fatalf("expected no DNS update for already up-to-date domain")
	}
}
