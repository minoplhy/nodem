package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/minoplhy/nodem/internal/db"
)

// scanECHNode parses a row from ech_nodes into an ECHNode struct.
func scanECHNode(scanner interface{ Scan(dest ...any) error }) (*db.ECHNode, error) {
	var n db.ECHNode
	var authTokenHashStr, sshPubKeyStr, lastSeenStr, lastIPStr sql.NullString
	var createdAtStr string

	err := scanner.Scan(
		&n.ID,
		&n.TenantID,
		&n.Name,
		&n.PullTransport,
		&authTokenHashStr,
		&sshPubKeyStr,
		&n.ProxyType,
		&lastSeenStr,
		&lastIPStr,
		&createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if authTokenHashStr.Valid {
		n.AuthTokenHash = authTokenHashStr.String
	}
	if sshPubKeyStr.Valid {
		n.SSHPublicKey = &sshPubKeyStr.String
	}
	n.LastSeenAt = scanNullTime(lastSeenStr)
	if lastIPStr.Valid {
		n.LastIP = &lastIPStr.String
	}
	if t, err := parseSQLiteTime(createdAtStr); err == nil {
		n.CreatedAt = t
	}
	n.Clusters = make([]db.ECHClusterStatus, 0)

	return &n, nil
}

func (r *SqliteRepository) loadNodeClusters(ctx context.Context, nodeID int64) ([]db.ECHClusterStatus, error) {
	query := `SELECT cn.cluster_id, c.name, c.public_name, cn.last_applied_version,
	                 cn.sync_status, cn.last_error, cn.last_synced_at
	          FROM ech_cluster_nodes cn
	          JOIN ech_clusters c ON c.id = cn.cluster_id
	          WHERE cn.node_id = ?
	          ORDER BY c.id ASC`
	rows, err := r.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []db.ECHClusterStatus
	for rows.Next() {
		var cs db.ECHClusterStatus
		var lastErrStr, lastSyncedStr sql.NullString
		if err := rows.Scan(
			&cs.ClusterID,
			&cs.ClusterName,
			&cs.PublicName,
			&cs.LastAppliedVersion,
			&cs.SyncStatus,
			&lastErrStr,
			&lastSyncedStr,
		); err != nil {
			return nil, err
		}
		if lastErrStr.Valid {
			cs.LastError = &lastErrStr.String
		}
		cs.LastSyncedAt = scanNullTime(lastSyncedStr)
		statuses = append(statuses, cs)
	}
	if statuses == nil {
		statuses = make([]db.ECHClusterStatus, 0)
	}
	return statuses, rows.Err()
}

func (r *SqliteRepository) GetECHNode(ctx context.Context, id int64) (*db.ECHNode, error) {
	query := `SELECT id, tenant_id, name, pull_transport, auth_token_hash, ssh_public_key,
	                 proxy_type, last_seen_at, last_ip, created_at
	          FROM ech_nodes WHERE id = ?`
	n, err := scanECHNode(r.db.QueryRowContext(ctx, query, id))
	if err != nil || n == nil {
		return n, err
	}
	clusters, err := r.loadNodeClusters(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	n.Clusters = clusters
	return n, nil
}

func (r *SqliteRepository) GetECHNodeByTokenHash(ctx context.Context, hash string) (*db.ECHNode, error) {
	query := `SELECT id, tenant_id, name, pull_transport, auth_token_hash, ssh_public_key,
	                 proxy_type, last_seen_at, last_ip, created_at
	          FROM ech_nodes WHERE auth_token_hash = ?`
	n, err := scanECHNode(r.db.QueryRowContext(ctx, query, hash))
	if err != nil || n == nil {
		return n, err
	}
	clusters, err := r.loadNodeClusters(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	n.Clusters = clusters
	return n, nil
}

func (r *SqliteRepository) GetECHNodeBySSHPublicKey(ctx context.Context, sshPubKey string) (*db.ECHNode, error) {
	query := `SELECT id, tenant_id, name, pull_transport, auth_token_hash, ssh_public_key,
	                 proxy_type, last_seen_at, last_ip, created_at
	          FROM ech_nodes WHERE TRIM(ssh_public_key) = TRIM(?)`
	n, err := scanECHNode(r.db.QueryRowContext(ctx, query, sshPubKey))
	if err != nil || n == nil {
		return n, err
	}
	clusters, err := r.loadNodeClusters(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	n.Clusters = clusters
	return n, nil
}

func (r *SqliteRepository) ListTenantECHNodes(ctx context.Context, tenantID int64) ([]db.ECHNode, error) {
	query := `SELECT id, tenant_id, name, pull_transport, auth_token_hash, ssh_public_key,
	                 proxy_type, last_seen_at, last_ip, created_at
	          FROM ech_nodes WHERE tenant_id = ? ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]db.ECHNode, 0)
	for rows.Next() {
		n, err := scanECHNode(rows)
		if err != nil {
			return nil, err
		}
		if n != nil {
			nodes = append(nodes, *n)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range nodes {
		clusters, err := r.loadNodeClusters(ctx, nodes[i].ID)
		if err != nil {
			return nil, err
		}
		nodes[i].Clusters = clusters
	}

	return nodes, nil
}

func (r *SqliteRepository) CreateECHNode(ctx context.Context, tenantID int64, name, pullTransport string, authTokenHash *string, sshPublicKey *string, proxyType string) (*db.ECHNode, error) {
	pullTransport = strings.ToUpper(strings.TrimSpace(pullTransport))
	if pullTransport != db.PullTransportHTTPS && pullTransport != db.PullTransportSSH {
		return nil, fmt.Errorf("invalid pull_transport '%s': must be strictly 'HTTPS' or 'SSH'", pullTransport)
	}

	proxyType = strings.ToUpper(strings.TrimSpace(proxyType))
	if proxyType == "" {
		proxyType = db.ProxyTypeNginx
	}

	query := `INSERT INTO ech_nodes (
		tenant_id, name, pull_transport, auth_token_hash, ssh_public_key, proxy_type
	) VALUES (?, ?, ?, ?, ?, ?)`

	res, err := r.db.ExecContext(ctx, query, tenantID, name, pullTransport, authTokenHash, sshPublicKey, proxyType)
	if err != nil {
		return nil, fmt.Errorf("failed to create ech node: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetECHNode(ctx, id)
}

func (r *SqliteRepository) UpdateECHNode(ctx context.Context, tenantID, nodeID int64, name, pullTransport, proxyType string, sshPublicKey *string) (*db.ECHNode, error) {
	pullTransport = strings.ToUpper(strings.TrimSpace(pullTransport))
	if pullTransport != db.PullTransportHTTPS && pullTransport != db.PullTransportSSH {
		return nil, fmt.Errorf("invalid pull_transport '%s': must be strictly 'HTTPS' or 'SSH'", pullTransport)
	}

	proxyType = strings.ToUpper(strings.TrimSpace(proxyType))
	if proxyType == "" {
		proxyType = db.ProxyTypeNginx
	}

	query := `UPDATE ech_nodes SET
		name = ?,
		pull_transport = ?,
		proxy_type = ?,
		ssh_public_key = ?
		WHERE id = ? AND tenant_id = ?`

	res, err := r.db.ExecContext(ctx, query, name, pullTransport, proxyType, sshPublicKey, nodeID, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, errors.New("ech node not found")
	}

	return r.GetECHNode(ctx, nodeID)
}

func (r *SqliteRepository) DeleteECHNode(ctx context.Context, tenantID, id int64) error {
	query := `DELETE FROM ech_nodes WHERE id = ? AND tenant_id = ?`
	res, err := r.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("ech node not found")
	}
	return nil
}

func (r *SqliteRepository) UpdateECHNodeLastSeen(ctx context.Context, nodeID int64, lastIP string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `UPDATE ech_nodes SET last_seen_at = ?, last_ip = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, now, lastIP, nodeID)
	return err
}

func (r *SqliteRepository) ListClusterNodes(ctx context.Context, clusterID int64) ([]db.ECHClusterNode, error) {
	query := `SELECT cn.cluster_id, cn.node_id, n.name, n.pull_transport, n.proxy_type,
	                 cn.last_applied_version, cn.sync_status, cn.last_error, cn.last_synced_at
	          FROM ech_cluster_nodes cn
	          JOIN ech_nodes n ON n.id = cn.node_id
	          WHERE cn.cluster_id = ?
	          ORDER BY n.id ASC`
	rows, err := r.db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]db.ECHClusterNode, 0)
	for rows.Next() {
		var cn db.ECHClusterNode
		var lastErrStr, lastSyncedStr sql.NullString
		if err := rows.Scan(
			&cn.ClusterID,
			&cn.NodeID,
			&cn.NodeName,
			&cn.PullTransport,
			&cn.ProxyType,
			&cn.LastAppliedVersion,
			&cn.SyncStatus,
			&lastErrStr,
			&lastSyncedStr,
		); err != nil {
			return nil, err
		}
		if lastErrStr.Valid {
			cn.LastError = &lastErrStr.String
		}
		cn.LastSyncedAt = scanNullTime(lastSyncedStr)
		nodes = append(nodes, cn)
	}
	return nodes, rows.Err()
}

func (r *SqliteRepository) ListNodeClusters(ctx context.Context, nodeID int64) ([]db.ECHCluster, error) {
	query := `SELECT c.id, c.tenant_id, c.name, c.public_name, c.cipher_suite, c.max_name_len,
	                 c.rotation_interval_hours, c.last_rotated_at, c.next_rotation_at, c.auto_rotate,
	                 c.current_version, c.signing_public_key, c.signing_private_key, c.created_at
	          FROM ech_clusters c
	          JOIN ech_cluster_nodes cn ON cn.cluster_id = c.id
	          WHERE cn.node_id = ?
	          ORDER BY c.id ASC`
	rows, err := r.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clusters := make([]db.ECHCluster, 0)
	for rows.Next() {
		c, err := scanECHCluster(rows)
		if err != nil {
			return nil, err
		}
		if c != nil {
			clusters = append(clusters, *c)
		}
	}
	return clusters, rows.Err()
}

func (r *SqliteRepository) AssignNodeToCluster(ctx context.Context, clusterID, nodeID int64) error {
	query := `INSERT INTO ech_cluster_nodes (cluster_id, node_id, last_applied_version, sync_status)
	          VALUES (?, ?, 0, 'PENDING')
	          ON CONFLICT(cluster_id, node_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, clusterID, nodeID)
	return err
}

func (r *SqliteRepository) UnassignNodeFromCluster(ctx context.Context, clusterID, nodeID int64) error {
	query := `DELETE FROM ech_cluster_nodes WHERE cluster_id = ? AND node_id = ?`
	res, err := r.db.ExecContext(ctx, query, clusterID, nodeID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("node not assigned to this cluster")
	}
	return nil
}

func (r *SqliteRepository) SetNodeClusters(ctx context.Context, nodeID int64, clusterIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing assignments not in the new list
	if len(clusterIDs) == 0 {
		if _, err := tx.ExecContext(ctx, "DELETE FROM ech_cluster_nodes WHERE node_id = ?", nodeID); err != nil {
			return err
		}
		return tx.Commit()
	}

	// Remove unselected clusters
	deleteQuery := fmt.Sprintf("DELETE FROM ech_cluster_nodes WHERE node_id = ? AND cluster_id NOT IN (%s)",
		strings.Repeat("?,", len(clusterIDs)-1)+"?")
	args := make([]any, 0, len(clusterIDs)+1)
	args = append(args, nodeID)
	for _, cid := range clusterIDs {
		args = append(args, cid)
	}
	if _, err := tx.ExecContext(ctx, deleteQuery, args...); err != nil {
		return err
	}

	// Insert new assignments
	insertQuery := `INSERT INTO ech_cluster_nodes (cluster_id, node_id, last_applied_version, sync_status)
	                VALUES (?, ?, 0, 'PENDING')
	                ON CONFLICT(cluster_id, node_id) DO NOTHING`
	for _, cid := range clusterIDs {
		if _, err := tx.ExecContext(ctx, insertQuery, cid, nodeID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *SqliteRepository) RecordClusterNodeAck(ctx context.Context, clusterID, nodeID, appliedVersion int64, status, lastIP string, lastError *string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `UPDATE ech_cluster_nodes SET
		last_applied_version = ?,
		sync_status = ?,
		last_error = ?,
		last_synced_at = ?
		WHERE cluster_id = ? AND node_id = ?`

	if _, err := r.db.ExecContext(ctx, query, appliedVersion, status, lastError, now, clusterID, nodeID); err != nil {
		return err
	}

	nodeQuery := `UPDATE ech_nodes SET last_seen_at = ?, last_ip = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, nodeQuery, now, lastIP, nodeID)
	return err
}

func (r *SqliteRepository) AreAllNodesInSync(ctx context.Context, clusterID, expectedVersion int64) (bool, error) {
	// Query ech_cluster_nodes for nodes assigned to this cluster
	query := `SELECT COUNT(*),
	                 COALESCE(SUM(CASE WHEN last_applied_version = ? AND sync_status = 'IN_SYNC' THEN 1 ELSE 0 END), 0)
	          FROM ech_cluster_nodes WHERE cluster_id = ?`

	var totalNodes, inSyncNodes int64
	err := r.db.QueryRowContext(ctx, query, expectedVersion, clusterID).Scan(&totalNodes, &inSyncNodes)
	if err != nil {
		return false, err
	}

	if totalNodes == 0 {
		// No nodes assigned to this cluster; ready for DNS publication
		return true, nil
	}

	return totalNodes == inSyncNodes, nil
}
