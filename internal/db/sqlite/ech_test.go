package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/db/sqlite"
)

func setupTestRepo(t *testing.T) *sqlite.SqliteRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_ech.db")
	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open test sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	if err := repo.InitDB(context.Background()); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	return repo
}

func TestECHClusterAndKeyLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := setupTestRepo(t)

	// Create user
	user, err := repo.CreateUser(ctx, "admin_user", "hash123", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 1. Create ECH Cluster
	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Alpha Cluster", "cover.alpha.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, "pubkey_ed25519_base64", "privkey_ed25519_base64")
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}
	if cluster.ID == 0 || cluster.CurrentVersion != 0 {
		t.Errorf("unexpected initial cluster state: %+v", cluster)
	}

	// 2. Increment Version
	now := time.Now().UTC()
	next := now.Add(168 * time.Hour)
	newVer, err := repo.IncrementClusterVersion(ctx, cluster.ID, now, next)
	if err != nil {
		t.Fatalf("IncrementClusterVersion failed: %v", err)
	}
	if newVer != 1 {
		t.Errorf("expected version 1, got %d", newVer)
	}

	// 3. Save Active Key
	key1, err := repo.SaveNewECHKey(ctx, cluster.ID, 1, "BASE64_ECH_1", "PRIV_KEY_1", "ECH_CONFIG_1", "FULL_PEM_1")
	if err != nil {
		t.Fatalf("SaveNewECHKey failed: %v", err)
	}
	if key1.Status != db.KeyStatusActive {
		t.Errorf("expected status ACTIVE, got %s", key1.Status)
	}

	// 4. Save Version 2 Key -> Version 1 becomes PREVIOUS
	now2 := now.Add(24 * time.Hour)
	_, err = repo.IncrementClusterVersion(ctx, cluster.ID, now2, next)
	if err != nil {
		t.Fatalf("IncrementClusterVersion 2 failed: %v", err)
	}
	key2, err := repo.SaveNewECHKey(ctx, cluster.ID, 2, "BASE64_ECH_2", "PRIV_KEY_2", "ECH_CONFIG_2", "FULL_PEM_2")
	if err != nil {
		t.Fatalf("SaveNewECHKey 2 failed: %v", err)
	}
	if key2.Status != db.KeyStatusActive {
		t.Errorf("expected key2 ACTIVE, got %s", key2.Status)
	}

	// Check active and previous keys
	activeKey, err := repo.GetActiveECHKey(ctx, cluster.ID)
	if err != nil || activeKey == nil || activeKey.Version != 2 {
		t.Fatalf("GetActiveECHKey expected version 2, got: %+v, err: %v", activeKey, err)
	}
	prevKey, err := repo.GetPreviousECHKey(ctx, cluster.ID)
	if err != nil || prevKey == nil || prevKey.Version != 1 {
		t.Fatalf("GetPreviousECHKey expected version 1, got: %+v, err: %v", prevKey, err)
	}
	if prevKey.Status != db.KeyStatusPrevious {
		t.Errorf("expected prevKey status PREVIOUS, got %s", prevKey.Status)
	}
}

func TestECHNodeTransportExclusivity(t *testing.T) {
	ctx := context.Background()
	repo := setupTestRepo(t)

	user, err := repo.CreateUser(ctx, "admin2", "hash", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Beta Cluster", "cover.beta.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, "pubkey", "privkey")
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	// 0. Cluster with 0 nodes returns inSync=true without NULL scan error
	zeroSync, err := repo.AreAllNodesInSync(ctx, cluster.ID, 1)
	if err != nil {
		t.Fatalf("AreAllNodesInSync on cluster with 0 nodes failed: %v", err)
	}
	if !zeroSync {
		t.Fatalf("expected inSync=true for cluster with 0 nodes, got %v", zeroSync)
	}

	// 1. Invalid Transport rejected
	_, err = repo.CreateECHNode(ctx, user.ID, "node-invalid", "BOTH", nil, nil, db.ProxyTypeNginx)
	if err == nil {
		t.Fatal("expected error creating node with invalid transport 'BOTH', got nil")
	}

	// 2. HTTPS Node
	tokenHash := "sha256_of_token_123"
	nodeHTTPS, err := repo.CreateECHNode(ctx, user.ID, "node-https-01", db.PullTransportHTTPS, &tokenHash, nil, db.ProxyTypeCaddy)
	if err != nil {
		t.Fatalf("CreateECHNode HTTPS failed: %v", err)
	}
	if nodeHTTPS.PullTransport != db.PullTransportHTTPS || nodeHTTPS.ProxyType != db.ProxyTypeCaddy {
		t.Errorf("unexpected nodeHTTPS properties: %+v", nodeHTTPS)
	}

	// Lookup by token hash
	foundNode, err := repo.GetECHNodeByTokenHash(ctx, tokenHash)
	if err != nil || foundNode == nil || foundNode.ID != nodeHTTPS.ID {
		t.Fatalf("GetECHNodeByTokenHash failed: %+v, err: %v", foundNode, err)
	}

	// 3. SSH Node
	sshPubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG... agent@host"
	nodeSSH, err := repo.CreateECHNode(ctx, user.ID, "node-ssh-01", db.PullTransportSSH, nil, &sshPubKey, db.ProxyTypeHAProxy)
	if err != nil {
		t.Fatalf("CreateECHNode SSH failed: %v", err)
	}
	if nodeSSH.PullTransport != db.PullTransportSSH || nodeSSH.ProxyType != db.ProxyTypeHAProxy {
		t.Errorf("unexpected nodeSSH properties: %+v", nodeSSH)
	}

	// Lookup by SSH public key
	foundSSHNode, err := repo.GetECHNodeBySSHPublicKey(ctx, sshPubKey)
	if err != nil || foundSSHNode == nil || foundSSHNode.ID != nodeSSH.ID {
		t.Fatalf("GetECHNodeBySSHPublicKey failed: %+v, err: %v", foundSSHNode, err)
	}

	// Assign both nodes to cluster
	if err := repo.AssignNodeToCluster(ctx, cluster.ID, nodeHTTPS.ID); err != nil {
		t.Fatalf("AssignNodeToCluster nodeHTTPS failed: %v", err)
	}
	if err := repo.AssignNodeToCluster(ctx, cluster.ID, nodeSSH.ID); err != nil {
		t.Fatalf("AssignNodeToCluster nodeSSH failed: %v", err)
	}

	// Also assign nodeHTTPS to cluster2 (multi-cluster test)
	cluster2, err := repo.CreateECHCluster(ctx, user.ID, "Gamma Cluster", "cover.gamma.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, "pub2", "priv2")
	if err != nil {
		t.Fatalf("CreateECHCluster 2 failed: %v", err)
	}
	if err := repo.AssignNodeToCluster(ctx, cluster2.ID, nodeHTTPS.ID); err != nil {
		t.Fatalf("AssignNodeToCluster nodeHTTPS to cluster2 failed: %v", err)
	}

	nodeClusters, err := repo.ListNodeClusters(ctx, nodeHTTPS.ID)
	if err != nil || len(nodeClusters) != 2 {
		t.Fatalf("expected 2 clusters for nodeHTTPS, got %d, err: %v", len(nodeClusters), err)
	}

	// 4. Test ACK and AreAllNodesInSync
	inSync, err := repo.AreAllNodesInSync(ctx, cluster.ID, 1)
	if err != nil || inSync {
		t.Fatalf("expected inSync=false before acks, got %v", inSync)
	}

	// Node 1 acks version 1 for cluster
	err = repo.RecordClusterNodeAck(ctx, cluster.ID, nodeHTTPS.ID, 1, db.SyncStatusInSync, "10.0.0.1", nil)
	if err != nil {
		t.Fatalf("RecordClusterNodeAck 1 failed: %v", err)
	}

	inSync, err = repo.AreAllNodesInSync(ctx, cluster.ID, 1)
	if err != nil || inSync {
		t.Fatalf("expected inSync=false when only 1 of 2 nodes acked, got %v", inSync)
	}

	// Node 2 acks version 1 for cluster
	err = repo.RecordClusterNodeAck(ctx, cluster.ID, nodeSSH.ID, 1, db.SyncStatusInSync, "10.0.0.2", nil)
	if err != nil {
		t.Fatalf("RecordClusterNodeAck 2 failed: %v", err)
	}

	inSync, err = repo.AreAllNodesInSync(ctx, cluster.ID, 1)
	if err != nil || !inSync {
		t.Fatalf("expected inSync=true when all nodes acked, got %v", inSync)
	}
}

func TestECHDomainOperations(t *testing.T) {
	ctx := context.Background()
	repo := setupTestRepo(t)

	user, err := repo.CreateUser(ctx, "admin3", "hash", "Admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	prov, err := repo.CreateProvider(ctx, user.ID, "Cloudflare Main", "Cloudflare", "https://api.cloudflare.com", "token123", "zone123")
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	cluster, err := repo.CreateECHCluster(ctx, user.ID, "Gamma Cluster", "cover.gamma.com", "x25519,hkdf-sha256,aes-128-gcm", 64, 168, true, "pubkey", "privkey")
	if err != nil {
		t.Fatalf("CreateECHCluster failed: %v", err)
	}

	ipv4 := "192.0.2.1"
	dom, err := repo.CreateECHDomain(ctx, cluster.ID, prov.ID, nil, "app.example.com", 300, "h2,h3", &ipv4, nil)
	if err != nil {
		t.Fatalf("CreateECHDomain failed: %v", err)
	}
	if dom.Domain != "app.example.com" || dom.DNSStatus != "PENDING" {
		t.Errorf("unexpected domain state: %+v", dom)
	}

	now := time.Now().UTC()
	err = repo.UpdateECHDomainSyncStatus(ctx, dom.ID, "SYNCED", &now)
	if err != nil {
		t.Fatalf("UpdateECHDomainSyncStatus failed: %v", err)
	}

	updatedDom, err := repo.GetECHDomain(ctx, dom.ID)
	if err != nil || updatedDom == nil || updatedDom.DNSStatus != "SYNCED" || updatedDom.LastSyncedAt == nil {
		t.Fatalf("GetECHDomain updated failed: %+v, err: %v", updatedDom, err)
	}

	// Create a node assigned to this cluster
	tokenHash := "hash_val"
	node, err := repo.CreateECHNode(ctx, user.ID, "Node-1", "HTTPS", &tokenHash, nil, "NGINX")
	if err != nil {
		t.Fatalf("CreateECHNode failed: %v", err)
	}
	if err := repo.SetNodeClusters(ctx, node.ID, []int64{cluster.ID}); err != nil {
		t.Fatalf("SetNodeClusters failed: %v", err)
	}
	// Node reports in-sync for version 0
	if err := repo.RecordClusterNodeAck(ctx, cluster.ID, node.ID, 0, "IN_SYNC", "127.0.0.1", nil); err != nil {
		t.Fatalf("RecordClusterNodeAck failed: %v", err)
	}

	// 5. Increment Cluster Version: must reset domains to PENDING and outdate nodes in ech_cluster_nodes
	rotTime := now.Add(1 * time.Hour)
	newVer, err := repo.IncrementClusterVersion(ctx, cluster.ID, rotTime, rotTime.Add(168*time.Hour))
	if err != nil {
		t.Fatalf("IncrementClusterVersion failed: %v", err)
	}
	if newVer != 1 {
		t.Fatalf("expected new version 1, got %d", newVer)
	}

	// Verify domain reset to PENDING
	reloadedDom, err := repo.GetECHDomain(ctx, dom.ID)
	if err != nil || reloadedDom.DNSStatus != "PENDING" {
		t.Fatalf("expected domain dns_status to be PENDING after version increment, got: %+v", reloadedDom)
	}

	// Verify node status in ech_cluster_nodes is OUTDATED
	nodeEntries, err := repo.ListClusterNodes(ctx, cluster.ID)
	if err != nil || len(nodeEntries) == 0 {
		t.Fatalf("ListClusterNodes failed: %v", err)
	}
	if nodeEntries[0].SyncStatus != "OUTDATED" {
		t.Errorf("expected node sync_status OUTDATED, got: %s", nodeEntries[0].SyncStatus)
	}
}

func TestLegacyECHNodesMigration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy_migration.db")

	// 1. Create a legacy database manually that contains:
	// - ech_nodes with cluster_id and WITHOUT tenant_id
	// - target_groups WITHOUT tenant_id
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite db: %v", err)
	}

	legacySchema := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'hash', 'Admin');

	CREATE TABLE target_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		dns_record TEXT NOT NULL
	);
	INSERT INTO target_groups (id, name, dns_record) VALUES (1, 'Legacy Target Group', 'app.example.com');

	CREATE TABLE ech_clusters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		public_name TEXT NOT NULL,
		cipher_suite TEXT NOT NULL DEFAULT 'x25519,hkdf-sha256,aes-128-gcm',
		max_name_len INTEGER NOT NULL DEFAULT 64,
		rotation_interval_hours INTEGER NOT NULL DEFAULT 168,
		last_rotated_at DATETIME,
		next_rotation_at DATETIME,
		auto_rotate BOOLEAN NOT NULL DEFAULT 1,
		current_version INTEGER NOT NULL DEFAULT 0,
		signing_public_key TEXT NOT NULL,
		signing_private_key TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO ech_clusters (id, tenant_id, name, public_name, signing_public_key, signing_private_key)
	VALUES (10, 1, 'Legacy Cluster', 'cover.legacy.com', 'pub10', 'priv10');

	-- Legacy ech_nodes table with cluster_id and NO tenant_id
	CREATE TABLE ech_nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		pull_transport TEXT NOT NULL CHECK(pull_transport IN ('HTTPS', 'SSH')),
		auth_token_hash TEXT UNIQUE,
		ssh_public_key TEXT,
		proxy_type TEXT NOT NULL DEFAULT 'NGINX' CHECK(proxy_type IN ('NGINX', 'CADDY', 'HAPROXY', 'HOOK')),
		last_applied_version INTEGER NOT NULL DEFAULT 0,
		sync_status TEXT NOT NULL DEFAULT 'PENDING' CHECK(sync_status IN ('IN_SYNC', 'OUTDATED', 'FAILED', 'PENDING')),
		last_error TEXT,
		last_seen_at DATETIME,
		last_ip TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(cluster_id, name)
	);

	INSERT INTO ech_nodes (id, cluster_id, name, pull_transport, auth_token_hash, ssh_public_key, proxy_type, last_applied_version, sync_status, last_error, last_seen_at, last_ip)
	VALUES (100, 10, 'legacy-edge-https', 'HTTPS', 'tokenhash100', NULL, 'NGINX', 1, 'IN_SYNC', NULL, '2026-09-08 12:00:00', '1.2.3.4'),
	       (101, 10, 'legacy-edge-ssh', 'SSH', NULL, 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG1', 'NGINX', 0, 'PENDING', 'connection timeout', '2026-09-08 11:00:00', '5.6.7.8');
	`

	if _, err := rawDB.Exec(legacySchema); err != nil {
		_ = rawDB.Close()
		t.Fatalf("failed to setup legacy schema: %v", err)
	}
	_ = rawDB.Close()

	// 2. Open via sqlite.New and run InitDB
	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo on legacy db: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("InitDB failed on legacy database: %v", err)
	}

	// 3. Verify target_groups now has tenant_id column added
	var tgTenantID int
	err = rawDBQueryRow(t, dbPath, "SELECT tenant_id FROM target_groups WHERE id = 1", &tgTenantID)
	if err != nil {
		t.Fatalf("failed to query target_groups tenant_id: %v", err)
	}
	if tgTenantID != 1 {
		t.Errorf("expected target_groups tenant_id = 1, got %d", tgTenantID)
	}

	// 4. Verify ech_nodes has tenant_id and preserved records
	node100, err := repo.GetECHNode(ctx, 100)
	if err != nil || node100 == nil {
		t.Fatalf("GetECHNode 100 failed: %+v, err: %v", node100, err)
	}
	if node100.TenantID != 1 || node100.Name != "legacy-edge-https" || node100.PullTransport != "HTTPS" || node100.AuthTokenHash != "tokenhash100" {
		t.Errorf("unexpected node100 data: %+v", node100)
	}
	if len(node100.Clusters) != 1 || node100.Clusters[0].ClusterID != 10 || node100.Clusters[0].LastAppliedVersion != 1 || node100.Clusters[0].SyncStatus != "IN_SYNC" {
		t.Errorf("unexpected node100 clusters: %+v", node100.Clusters)
	}

	node101, err := repo.GetECHNode(ctx, 101)
	if err != nil || node101 == nil {
		t.Fatalf("GetECHNode 101 failed: %+v, err: %v", node101, err)
	}
	if node101.TenantID != 1 || node101.Name != "legacy-edge-ssh" || node101.PullTransport != "SSH" || node101.SSHPublicKey == nil {
		t.Errorf("unexpected node101 data: %+v", node101)
	}
	if len(node101.Clusters) != 1 || node101.Clusters[0].ClusterID != 10 || node101.Clusters[0].SyncStatus != "PENDING" || node101.Clusters[0].LastError == nil || *node101.Clusters[0].LastError != "connection timeout" {
		t.Errorf("unexpected node101 clusters: %+v", node101.Clusters)
	}

	// 5. Verify cluster perspective via ListClusterNodes
	cNodes, err := repo.ListClusterNodes(ctx, 10)
	if err != nil || len(cNodes) != 2 {
		t.Fatalf("expected 2 cluster nodes for cluster 10, got: %d, err: %v", len(cNodes), err)
	}

	// 6. Verify InitDB is idempotent when executed again
	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("second InitDB failed: %v", err)
	}
}

func rawDBQueryRow(t *testing.T, dbPath, query string, dest interface{}) error {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.QueryRow(query).Scan(dest)
}

func TestLegacyECHNodesDuplicateNameMigration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "duplicate_names.db")

	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite db: %v", err)
	}

	legacySchema := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL
	);
	INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'hash', 'Admin');

	CREATE TABLE ech_clusters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		public_name TEXT NOT NULL,
		signing_public_key TEXT NOT NULL,
		signing_private_key TEXT NOT NULL
	);
	INSERT INTO ech_clusters (id, tenant_id, name, public_name, signing_public_key, signing_private_key)
	VALUES (1, 1, 'Cluster 1', 'cover1.com', 'pub1', 'priv1'),
	       (2, 1, 'Cluster 2', 'cover2.com', 'pub2', 'priv2');

	-- In legacy schema, names only had to be unique per cluster (UNIQUE(cluster_id, name))
	CREATE TABLE ech_nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id),
		name TEXT NOT NULL,
		pull_transport TEXT NOT NULL,
		auth_token_hash TEXT UNIQUE,
		UNIQUE(cluster_id, name)
	);

	-- Two nodes in different clusters with the identical name "edge-sg"
	INSERT INTO ech_nodes (id, cluster_id, name, pull_transport, auth_token_hash)
	VALUES (10, 1, 'edge-sg', 'HTTPS', 'hash10'),
	       (20, 2, 'edge-sg', 'HTTPS', 'hash20');
	`
	if _, err := rawDB.Exec(legacySchema); err != nil {
		_ = rawDB.Close()
		t.Fatalf("failed to setup legacy schema: %v", err)
	}
	_ = rawDB.Close()

	repo, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	// This should successfully resolve name collision rather than crashing on UNIQUE(tenant_id, name)
	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("InitDB failed with duplicate names: %v", err)
	}

	node10, err := repo.GetECHNode(ctx, 10)
	if err != nil || node10 == nil {
		t.Fatalf("GetECHNode 10 failed: %v", err)
	}
	if node10.Name != "edge-sg" {
		t.Errorf("expected node 10 to keep original name 'edge-sg', got '%s'", node10.Name)
	}

	node20, err := repo.GetECHNode(ctx, 20)
	if err != nil || node20 == nil {
		t.Fatalf("GetECHNode 20 failed: %v", err)
	}
	if node20.Name != "edge-sg-20" {
		t.Errorf("expected node 20 to be disambiguated to 'edge-sg-20', got '%s'", node20.Name)
	}
}


