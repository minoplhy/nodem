package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

var tableDefinitions = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		public_id TEXT,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		ip_address TEXT,
		user_agent TEXT,
		last_active DATETIME
	);`,
	`CREATE TABLE IF NOT EXISTS dns_providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		provider_type TEXT NOT NULL,
		api_url TEXT NOT NULL,
		token TEXT NOT NULL,
		zone TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS target_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		dns_record TEXT NOT NULL,
		dns_provider_id INTEGER NOT NULL REFERENCES dns_providers(id) ON DELETE CASCADE,
		check_interval_secs INTEGER NOT NULL DEFAULT 60,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE IF NOT EXISTS target_ips (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES target_groups(id) ON DELETE CASCADE,
		ip TEXT NOT NULL,
		dns_added BOOLEAN NOT NULL DEFAULT 0,
		last_checked DATETIME,
		status TEXT NOT NULL DEFAULT 'UNKNOWN',
		display_order INTEGER NOT NULL DEFAULT 0,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		UNIQUE(group_id, ip)
	);`,
	`CREATE TABLE IF NOT EXISTS checks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES target_groups(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		protocol TEXT NOT NULL,
		domain TEXT,
		port INTEGER NOT NULL,
		path TEXT,
		down_threshold INTEGER NOT NULL DEFAULT 2,
		up_threshold INTEGER NOT NULL DEFAULT 3,
		bypass_on_global_failure BOOLEAN NOT NULL DEFAULT 0
	);`,
	`CREATE TABLE IF NOT EXISTS check_states (
		ip_id INTEGER NOT NULL REFERENCES target_ips(id) ON DELETE CASCADE,
		check_id INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
		consecutive_up INTEGER NOT NULL DEFAULT 0,
		consecutive_down INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'UNKNOWN',
		message TEXT,
		PRIMARY KEY(ip_id, check_id)
	);`,
	`CREATE TABLE IF NOT EXISTS group_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES target_groups(id) ON DELETE CASCADE,
		expression_json TEXT NOT NULL,
		action TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS check_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ip_id INTEGER NOT NULL REFERENCES target_ips(id) ON DELETE CASCADE,
		check_id INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
		timestamp DATETIME NOT NULL,
		success BOOLEAN NOT NULL,
		message TEXT
	);`,
	`CREATE TABLE IF NOT EXISTS notification_channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		channel_type TEXT NOT NULL,
		config_json TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS group_notifications (
		group_id INTEGER NOT NULL REFERENCES target_groups(id) ON DELETE CASCADE,
		channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
		notify_on_up BOOLEAN NOT NULL DEFAULT 1,
		notify_on_down BOOLEAN NOT NULL DEFAULT 1,
		PRIMARY KEY(group_id, channel_id)
	);`,
	`CREATE TABLE IF NOT EXISTS group_anomalies (
		group_id INTEGER NOT NULL REFERENCES target_groups(id) ON DELETE CASCADE,
		ip TEXT NOT NULL,
		detected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (group_id, ip)
	);`,
	`CREATE TABLE IF NOT EXISTS ech_clusters (
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
	);`,
	`CREATE TABLE IF NOT EXISTS ech_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id) ON DELETE CASCADE,
		version INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'ACTIVE',
		base64_ech TEXT NOT NULL,
		private_key_pem TEXT NOT NULL,
		ech_config_pem TEXT NOT NULL,
		full_pem TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(cluster_id, version)
	);`,
	`CREATE TABLE IF NOT EXISTS ech_nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		pull_transport TEXT NOT NULL CHECK(pull_transport IN ('HTTPS', 'SSH')),
		auth_token_hash TEXT UNIQUE,
		ssh_public_key TEXT,
		proxy_type TEXT NOT NULL DEFAULT 'NGINX' CHECK(proxy_type IN ('NGINX', 'CADDY', 'HAPROXY', 'HOOK')),
		last_seen_at DATETIME,
		last_ip TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, name)
	);`,
	`CREATE TABLE IF NOT EXISTS ech_cluster_nodes (
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id) ON DELETE CASCADE,
		node_id INTEGER NOT NULL REFERENCES ech_nodes(id) ON DELETE CASCADE,
		last_applied_version INTEGER NOT NULL DEFAULT 0,
		sync_status TEXT NOT NULL DEFAULT 'PENDING' CHECK(sync_status IN ('IN_SYNC', 'OUTDATED', 'FAILED', 'PENDING')),
		last_error TEXT,
		last_synced_at DATETIME,
		PRIMARY KEY (cluster_id, node_id)
	);`,
	`CREATE TABLE IF NOT EXISTS ech_domains (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id) ON DELETE CASCADE,
		dns_provider_id INTEGER NOT NULL REFERENCES dns_providers(id) ON DELETE CASCADE,
		target_group_id INTEGER REFERENCES target_groups(id) ON DELETE SET NULL,
		domain TEXT NOT NULL UNIQUE,
		ttl INTEGER NOT NULL DEFAULT 300,
		alpn TEXT NOT NULL DEFAULT 'h2,h3',
		ipv4_hint TEXT,
		ipv6_hint TEXT,
		last_synced_at DATETIME,
		dns_status TEXT NOT NULL DEFAULT 'PENDING',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE IF NOT EXISTS ech_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id) ON DELETE CASCADE,
		node_id INTEGER REFERENCES ech_nodes(id) ON DELETE SET NULL,
		domain_id INTEGER REFERENCES ech_domains(id) ON DELETE SET NULL,
		event_type TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
}

var legacyAlters = []string{
	`ALTER TABLE target_groups ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;`,
	`ALTER TABLE target_ips ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;`,
	`ALTER TABLE target_ips ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;`,
	`ALTER TABLE check_states ADD COLUMN message TEXT;`,
	`ALTER TABLE sessions ADD COLUMN ip_address TEXT;`,
	`ALTER TABLE sessions ADD COLUMN user_agent TEXT;`,
	`ALTER TABLE sessions ADD COLUMN last_active DATETIME;`,
	`ALTER TABLE sessions ADD COLUMN public_id TEXT;`,
	`UPDATE sessions SET public_id = hex(randomblob(16)) WHERE public_id IS NULL OR public_id = '';`,
}

var indexDefinitions = []string{
	`CREATE INDEX IF NOT EXISTS idx_check_logs_ip_id_timestamp ON check_logs(ip_id, timestamp DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_target_groups_tenant_id ON target_groups(tenant_id);`,
	`CREATE INDEX IF NOT EXISTS idx_target_ips_group_id ON target_ips(group_id);`,
	`CREATE INDEX IF NOT EXISTS idx_checks_group_id ON checks(group_id);`,
	`CREATE INDEX IF NOT EXISTS idx_group_rules_group_id ON group_rules(group_id);`,
	`CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_id ON notification_channels(tenant_id);`,
	`CREATE INDEX IF NOT EXISTS idx_dns_providers_tenant_id ON dns_providers(tenant_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_clusters_tenant_id ON ech_clusters(tenant_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_keys_cluster_version ON ech_keys(cluster_id, version DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_nodes_tenant_id ON ech_nodes(tenant_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_nodes_token_hash ON ech_nodes(auth_token_hash);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_cluster_nodes_cluster_id ON ech_cluster_nodes(cluster_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_cluster_nodes_node_id ON ech_cluster_nodes(node_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_domains_cluster_id ON ech_domains(cluster_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ech_logs_cluster_id ON ech_logs(cluster_id, created_at DESC);`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_public_id ON sessions(public_id);`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`,
}

// tableExists checks whether a table exists in sqlite_master.
func tableExists(ctx context.Context, db *sql.DB, tableName string) bool {
	var count int
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?"
	if err := db.QueryRowContext(ctx, query, tableName).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// tableHasColumn checks whether a column exists in a given table.
func tableHasColumn(ctx context.Context, db *sql.DB, tableName, columnName string) bool {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", tableName)
	if err := db.QueryRowContext(ctx, query, columnName).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// connHasColumn checks whether a column exists on a specific connection.
func connHasColumn(ctx context.Context, conn *sql.Conn, tableName, columnName string) bool {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", tableName)
	if err := conn.QueryRowContext(ctx, query, columnName).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// migrateEchNodes migrates legacy ech_nodes (which had cluster_id and lacked tenant_id)
// to the decoupled schema with ech_cluster_nodes and tenant_id.
func migrateEchNodes(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection for ech_nodes migration: %w", err)
	}
	defer conn.Close()

	hasClusterID := connHasColumn(ctx, conn, "ech_nodes", "cluster_id")
	hasTenantID := connHasColumn(ctx, conn, "ech_nodes", "tenant_id")
	if !hasClusterID && hasTenantID {
		return nil
	}

	slog.Info("Migrating legacy ech_nodes table to multi-cluster schema", "hasClusterID", hasClusterID, "hasTenantID", hasTenantID)

	// Disable foreign keys on this connection during schema alteration
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF;"); err != nil {
		return fmt.Errorf("failed to disable foreign keys: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 1. Ensure ech_cluster_nodes table exists
	createClusterNodesSQL := `CREATE TABLE IF NOT EXISTS ech_cluster_nodes (
		cluster_id INTEGER NOT NULL REFERENCES ech_clusters(id) ON DELETE CASCADE,
		node_id INTEGER NOT NULL REFERENCES ech_nodes(id) ON DELETE CASCADE,
		last_applied_version INTEGER NOT NULL DEFAULT 0,
		sync_status TEXT NOT NULL DEFAULT 'PENDING' CHECK(sync_status IN ('IN_SYNC', 'OUTDATED', 'FAILED', 'PENDING')),
		last_error TEXT,
		last_synced_at DATETIME,
		PRIMARY KEY (cluster_id, node_id)
	);`
	if _, err := tx.ExecContext(ctx, createClusterNodesSQL); err != nil {
		return fmt.Errorf("failed to ensure ech_cluster_nodes table exists: %w", err)
	}

	// 2. If cluster_id was present on ech_nodes, copy cluster associations to ech_cluster_nodes
	if hasClusterID {
		hasAppliedVersion := connHasColumn(ctx, conn, "ech_nodes", "last_applied_version")
		hasSyncStatus := connHasColumn(ctx, conn, "ech_nodes", "sync_status")
		hasLastError := connHasColumn(ctx, conn, "ech_nodes", "last_error")
		hasLastSeen := connHasColumn(ctx, conn, "ech_nodes", "last_seen_at")

		appliedVersionExpr := "0"
		if hasAppliedVersion {
			appliedVersionExpr = "COALESCE(n.last_applied_version, 0)"
		}
		syncStatusExpr := "'PENDING'"
		if hasSyncStatus {
			syncStatusExpr = "CASE WHEN n.sync_status IN ('IN_SYNC', 'OUTDATED', 'FAILED', 'PENDING') THEN n.sync_status ELSE 'PENDING' END"
		}
		lastErrorExpr := "NULL"
		if hasLastError {
			lastErrorExpr = "n.last_error"
		}
		lastSeenExpr := "NULL"
		if hasLastSeen {
			lastSeenExpr = "n.last_seen_at"
		}

		copyBindingsSQL := fmt.Sprintf(`INSERT OR IGNORE INTO ech_cluster_nodes
			(cluster_id, node_id, last_applied_version, sync_status, last_error, last_synced_at)
			SELECT n.cluster_id, n.id, %s, %s, %s, %s
			FROM ech_nodes n
			WHERE n.cluster_id IS NOT NULL;`, appliedVersionExpr, syncStatusExpr, lastErrorExpr, lastSeenExpr)

		if _, err := tx.ExecContext(ctx, copyBindingsSQL); err != nil {
			return fmt.Errorf("failed to copy legacy cluster node associations: %w", err)
		}
	}

	// 3. Create the new ech_nodes_new table
	createNodesNewSQL := `CREATE TABLE ech_nodes_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		pull_transport TEXT NOT NULL CHECK(pull_transport IN ('HTTPS', 'SSH')),
		auth_token_hash TEXT UNIQUE,
		ssh_public_key TEXT,
		proxy_type TEXT NOT NULL DEFAULT 'NGINX' CHECK(proxy_type IN ('NGINX', 'CADDY', 'HAPROXY', 'HOOK')),
		last_seen_at DATETIME,
		last_ip TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, name)
	);`
	if _, err := tx.ExecContext(ctx, createNodesNewSQL); err != nil {
		return fmt.Errorf("failed to create ech_nodes_new table: %w", err)
	}

	// 4. Copy node rows into ech_nodes_new
	hasPullTransport := connHasColumn(ctx, conn, "ech_nodes", "pull_transport")
	hasAuthTokenHash := connHasColumn(ctx, conn, "ech_nodes", "auth_token_hash")
	hasSSHPublicKey := connHasColumn(ctx, conn, "ech_nodes", "ssh_public_key")
	hasProxyType := connHasColumn(ctx, conn, "ech_nodes", "proxy_type")
	hasLastSeen := connHasColumn(ctx, conn, "ech_nodes", "last_seen_at")
	hasLastIP := connHasColumn(ctx, conn, "ech_nodes", "last_ip")
	hasCreatedAt := connHasColumn(ctx, conn, "ech_nodes", "created_at")

	pullTransportExpr := "'HTTPS'"
	if hasPullTransport {
		pullTransportExpr = "COALESCE(NULLIF(UPPER(n.pull_transport), ''), 'HTTPS')"
	}
	authTokenHashExpr := "NULL"
	if hasAuthTokenHash {
		authTokenHashExpr = "NULLIF(n.auth_token_hash, '')"
	}
	sshPubKeyExpr := "NULL"
	if hasSSHPublicKey {
		sshPubKeyExpr = "n.ssh_public_key"
	}
	proxyTypeExpr := "'NGINX'"
	if hasProxyType {
		proxyTypeExpr = "COALESCE(NULLIF(UPPER(n.proxy_type), ''), 'NGINX')"
	}
	lastSeenExpr := "NULL"
	if hasLastSeen {
		lastSeenExpr = "n.last_seen_at"
	}
	lastIPExpr := "NULL"
	if hasLastIP {
		lastIPExpr = "n.last_ip"
	}
	createdAtExpr := "CURRENT_TIMESTAMP"
	if hasCreatedAt {
		createdAtExpr = "COALESCE(n.created_at, CURRENT_TIMESTAMP)"
	}

	var copyNodesSQL string
	if hasClusterID {
		copyNodesSQL = fmt.Sprintf(`INSERT INTO ech_nodes_new
			(id, tenant_id, name, pull_transport, auth_token_hash, ssh_public_key, proxy_type, last_seen_at, last_ip, created_at)
			SELECT n.id,
			       COALESCE(c.tenant_id, 1),
			       CASE 
			           WHEN (SELECT COUNT(*) FROM ech_nodes n2 WHERE n2.name = n.name AND n2.id < n.id) > 0 
			           THEN n.name || '-' || CAST(n.id AS TEXT) 
			           ELSE n.name 
			       END,
			       %s,
			       %s,
			       %s,
			       %s,
			       %s,
			       %s,
			       %s
			FROM ech_nodes n
			LEFT JOIN ech_clusters c ON c.id = n.cluster_id;`, pullTransportExpr, authTokenHashExpr, sshPubKeyExpr, proxyTypeExpr, lastSeenExpr, lastIPExpr, createdAtExpr)
	} else {
		copyNodesSQL = fmt.Sprintf(`INSERT INTO ech_nodes_new
			(id, tenant_id, name, pull_transport, auth_token_hash, ssh_public_key, proxy_type, last_seen_at, last_ip, created_at)
			SELECT n.id,
			       1,
			       CASE 
			           WHEN (SELECT COUNT(*) FROM ech_nodes n2 WHERE n2.name = n.name AND n2.id < n.id) > 0 
			           THEN n.name || '-' || CAST(n.id AS TEXT) 
			           ELSE n.name 
			       END,
			       %s,
			       %s,
			       %s,
			       %s,
			       %s,
			       %s,
			       %s
			FROM ech_nodes n;`, pullTransportExpr, authTokenHashExpr, sshPubKeyExpr, proxyTypeExpr, lastSeenExpr, lastIPExpr, createdAtExpr)
	}

	if _, err := tx.ExecContext(ctx, copyNodesSQL); err != nil {
		return fmt.Errorf("failed to copy node records to ech_nodes_new: %w", err)
	}

	// 5. Drop old ech_nodes table and rename ech_nodes_new
	if _, err := tx.ExecContext(ctx, "DROP TABLE ech_nodes;"); err != nil {
		return fmt.Errorf("failed to drop legacy ech_nodes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "ALTER TABLE ech_nodes_new RENAME TO ech_nodes;"); err != nil {
		return fmt.Errorf("failed to rename ech_nodes_new to ech_nodes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit ech_nodes migration transaction: %w", err)
	}

	slog.Info("Successfully migrated ech_nodes schema to decoupled multi-cluster format")
	return nil
}

// InitDB applies table definitions, schema migrations, and indexes.
func InitDB(ctx context.Context, db *sql.DB) error {
	// 1. Create base tables (without tenant indexes that depend on columns being migrated)
	for _, query := range tableDefinitions {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to create base table: %w", err)
		}
	}

	// 2. Ensure tenant_id column exists on multi-tenant tables
	tablesWithTenant := []string{"target_groups", "dns_providers", "notification_channels", "ech_clusters"}
	for _, tbl := range tablesWithTenant {
		if tableExists(ctx, db, tbl) && !tableHasColumn(ctx, db, tbl, "tenant_id") {
			alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 1;", tbl)
			if _, err := db.ExecContext(ctx, alterSQL); err != nil {
				return fmt.Errorf("failed to add tenant_id to %s: %w", tbl, err)
			}
			slog.Info("Added missing tenant_id column to table", "table", tbl)
		}
	}

	// 3. Migrate legacy ech_nodes table if it exists and lacks tenant_id or still has cluster_id
	if tableExists(ctx, db, "ech_nodes") {
		hasClusterID := tableHasColumn(ctx, db, "ech_nodes", "cluster_id")
		hasTenantID := tableHasColumn(ctx, db, "ech_nodes", "tenant_id")
		if hasClusterID || !hasTenantID {
			if err := migrateEchNodes(ctx, db); err != nil {
				return fmt.Errorf("failed to migrate ech_nodes table: %w", err)
			}
		}
	}

	// 4. Apply legacy schema alters (backward compatibility for older node_monitor DBs)
	for _, alter := range legacyAlters {
		_, _ = db.ExecContext(ctx, alter)
	}

	// 5. Create all indexes safely after all tables and columns are guaranteed to exist
	for _, idx := range indexDefinitions {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}
