package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/minoplhy/nodem/internal/db"
)

// scanECHCluster parses a row into an ECHCluster struct.
func scanECHCluster(scanner interface{ Scan(dest ...any) error }) (*db.ECHCluster, error) {
	var c db.ECHCluster
	var lastRotatedStr, nextRotationStr sql.NullString
	var createdAtStr string

	err := scanner.Scan(
		&c.ID,
		&c.TenantID,
		&c.Name,
		&c.PublicName,
		&c.CipherSuite,
		&c.MaxNameLen,
		&c.RotationIntervalHours,
		&lastRotatedStr,
		&nextRotationStr,
		&c.AutoRotate,
		&c.CurrentVersion,
		&c.SigningPublicKey,
		&c.SigningPrivateKey,
		&createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	c.LastRotatedAt = scanNullTime(lastRotatedStr)
	c.NextRotationAt = scanNullTime(nextRotationStr)
	if t, err := parseSQLiteTime(createdAtStr); err == nil {
		c.CreatedAt = t
	}

	return &c, nil
}

func (r *SqliteRepository) GetECHCluster(ctx context.Context, tenantID, id int64) (*db.ECHCluster, error) {
	query := `SELECT id, tenant_id, name, public_name, cipher_suite, max_name_len,
	                 rotation_interval_hours, last_rotated_at, next_rotation_at,
	                 auto_rotate, current_version, signing_public_key, signing_private_key, created_at
	          FROM ech_clusters WHERE tenant_id = ? AND id = ?`
	return scanECHCluster(r.db.QueryRowContext(ctx, query, tenantID, id))
}

func (r *SqliteRepository) GetECHClusterDirect(ctx context.Context, id int64) (*db.ECHCluster, error) {
	query := `SELECT id, tenant_id, name, public_name, cipher_suite, max_name_len,
	                 rotation_interval_hours, last_rotated_at, next_rotation_at,
	                 auto_rotate, current_version, signing_public_key, signing_private_key, created_at
	          FROM ech_clusters WHERE id = ?`
	return scanECHCluster(r.db.QueryRowContext(ctx, query, id))
}

func (r *SqliteRepository) ListECHClusters(ctx context.Context, tenantID int64) ([]db.ECHCluster, error) {
	query := `SELECT id, tenant_id, name, public_name, cipher_suite, max_name_len,
	                 rotation_interval_hours, last_rotated_at, next_rotation_at,
	                 auto_rotate, current_version, signing_public_key, signing_private_key, created_at
	          FROM ech_clusters WHERE tenant_id = ? ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query, tenantID)
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

func (r *SqliteRepository) ListAllECHClusters(ctx context.Context) ([]db.ECHCluster, error) {
	query := `SELECT id, tenant_id, name, public_name, cipher_suite, max_name_len,
	                 rotation_interval_hours, last_rotated_at, next_rotation_at,
	                 auto_rotate, current_version, signing_public_key, signing_private_key, created_at
	          FROM ech_clusters ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query)
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

func (r *SqliteRepository) CreateECHCluster(ctx context.Context, tenantID int64, name, publicName, cipherSuite string, maxNameLen, rotationIntervalHours int, autoRotate bool, signingPublicKey, signingPrivateKey string) (*db.ECHCluster, error) {
	if cipherSuite == "" {
		cipherSuite = "x25519,hkdf-sha256,aes-128-gcm"
	}
	if maxNameLen <= 0 {
		maxNameLen = 64
	}
	if rotationIntervalHours <= 0 {
		rotationIntervalHours = 168
	}

	query := `INSERT INTO ech_clusters (
		tenant_id, name, public_name, cipher_suite, max_name_len,
		rotation_interval_hours, auto_rotate, current_version, signing_public_key, signing_private_key
	) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`

	res, err := r.db.ExecContext(ctx, query,
		tenantID, name, publicName, cipherSuite, maxNameLen,
		rotationIntervalHours, autoRotate, signingPublicKey, signingPrivateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ech cluster: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetECHCluster(ctx, tenantID, id)
}

func (r *SqliteRepository) UpdateECHCluster(ctx context.Context, tenantID, id int64, name, publicName, cipherSuite string, maxNameLen, rotationIntervalHours int, autoRotate bool) (*db.ECHCluster, error) {
	query := `UPDATE ech_clusters SET
		name = ?, public_name = ?, cipher_suite = ?, max_name_len = ?,
		rotation_interval_hours = ?, auto_rotate = ?
		WHERE tenant_id = ? AND id = ?`

	_, err := r.db.ExecContext(ctx, query,
		name, publicName, cipherSuite, maxNameLen,
		rotationIntervalHours, autoRotate, tenantID, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update ech cluster: %w", err)
	}

	return r.GetECHCluster(ctx, tenantID, id)
}

func (r *SqliteRepository) DeleteECHCluster(ctx context.Context, tenantID, id int64) error {
	query := `DELETE FROM ech_clusters WHERE tenant_id = ? AND id = ?`
	res, err := r.db.ExecContext(ctx, query, tenantID, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("cluster not found or access denied")
	}
	return nil
}

func (r *SqliteRepository) IncrementClusterVersion(ctx context.Context, clusterID int64, lastRotated, nextRotation time.Time) (int64, error) {
	query := `UPDATE ech_clusters SET
		current_version = current_version + 1,
		last_rotated_at = ?,
		next_rotation_at = ?
		WHERE id = ? RETURNING current_version`

	var newVersion int64
	err := r.db.QueryRowContext(ctx, query,
		lastRotated.UTC().Format("2006-01-02 15:04:05"),
		nextRotation.UTC().Format("2006-01-02 15:04:05"),
		clusterID,
	).Scan(&newVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to increment cluster version: %w", err)
	}

	// Flag all nodes on older versions as OUTDATED
	updateNodesQuery := `UPDATE ech_nodes SET sync_status = 'OUTDATED'
	                     WHERE cluster_id = ? AND last_applied_version < ?`
	_, _ = r.db.ExecContext(ctx, updateNodesQuery, clusterID, newVersion)

	return newVersion, nil
}

// scanECHKey parses a row into an ECHKey struct.
func scanECHKey(scanner interface{ Scan(dest ...any) error }) (*db.ECHKey, error) {
	var k db.ECHKey
	var createdAtStr string

	err := scanner.Scan(
		&k.ID,
		&k.ClusterID,
		&k.Version,
		&k.Status,
		&k.Base64ECH,
		&k.PrivateKeyPEM,
		&k.ECHConfigPEM,
		&k.FullPEM,
		&createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if t, err := parseSQLiteTime(createdAtStr); err == nil {
		k.CreatedAt = t
	}
	return &k, nil
}

func (r *SqliteRepository) GetActiveECHKey(ctx context.Context, clusterID int64) (*db.ECHKey, error) {
	query := `SELECT id, cluster_id, version, status, base64_ech, private_key_pem, ech_config_pem, full_pem, created_at
	          FROM ech_keys WHERE cluster_id = ? AND status = 'ACTIVE' ORDER BY version DESC LIMIT 1`
	return scanECHKey(r.db.QueryRowContext(ctx, query, clusterID))
}

func (r *SqliteRepository) GetPreviousECHKey(ctx context.Context, clusterID int64) (*db.ECHKey, error) {
	query := `SELECT id, cluster_id, version, status, base64_ech, private_key_pem, ech_config_pem, full_pem, created_at
	          FROM ech_keys WHERE cluster_id = ? AND status = 'PREVIOUS' ORDER BY version DESC LIMIT 1`
	return scanECHKey(r.db.QueryRowContext(ctx, query, clusterID))
}

func (r *SqliteRepository) GetECHKeyByVersion(ctx context.Context, clusterID, version int64) (*db.ECHKey, error) {
	query := `SELECT id, cluster_id, version, status, base64_ech, private_key_pem, ech_config_pem, full_pem, created_at
	          FROM ech_keys WHERE cluster_id = ? AND version = ?`
	return scanECHKey(r.db.QueryRowContext(ctx, query, clusterID, version))
}

func (r *SqliteRepository) ListECHKeys(ctx context.Context, clusterID int64) ([]db.ECHKey, error) {
	query := `SELECT id, cluster_id, version, status, base64_ech, private_key_pem, ech_config_pem, full_pem, created_at
	          FROM ech_keys WHERE cluster_id = ? ORDER BY version DESC`
	rows, err := r.db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []db.ECHKey
	for rows.Next() {
		k, err := scanECHKey(rows)
		if err != nil {
			return nil, err
		}
		if k != nil {
			keys = append(keys, *k)
		}
	}
	return keys, rows.Err()
}

func (r *SqliteRepository) SaveNewECHKey(ctx context.Context, clusterID, version int64, base64ECH, privateKeyPEM, echConfigPEM, fullPEM string) (*db.ECHKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Move current PREVIOUS keys to ARCHIVED
	if _, err := tx.ExecContext(ctx, `UPDATE ech_keys SET status = 'ARCHIVED' WHERE cluster_id = ? AND status = 'PREVIOUS'`, clusterID); err != nil {
		return nil, err
	}

	// 2. Move current ACTIVE keys to PREVIOUS
	if _, err := tx.ExecContext(ctx, `UPDATE ech_keys SET status = 'PREVIOUS' WHERE cluster_id = ? AND status = 'ACTIVE'`, clusterID); err != nil {
		return nil, err
	}

	// 3. Insert new ACTIVE key
	insertQuery := `INSERT INTO ech_keys (
		cluster_id, version, status, base64_ech, private_key_pem, ech_config_pem, full_pem
	) VALUES (?, ?, 'ACTIVE', ?, ?, ?, ?)`

	res, err := tx.ExecContext(ctx, insertQuery, clusterID, version, base64ECH, privateKeyPEM, echConfigPEM, fullPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to insert new ech key: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return scanECHKey(r.db.QueryRowContext(ctx, `SELECT id, cluster_id, version, status, base64_ech, private_key_pem, ech_config_pem, full_pem, created_at FROM ech_keys WHERE id = ?`, id))
}

func (r *SqliteRepository) AddECHLog(ctx context.Context, clusterID int64, nodeID, domainID *int64, eventType, message string) (*db.ECHLog, error) {
	query := `INSERT INTO ech_logs (cluster_id, node_id, domain_id, event_type, message) VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, clusterID, nodeID, domainID, eventType, message)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var log db.ECHLog
	var createdAtStr string
	err = r.db.QueryRowContext(ctx, `SELECT id, cluster_id, node_id, domain_id, event_type, message, created_at FROM ech_logs WHERE id = ?`, id).
		Scan(&log.ID, &log.ClusterID, &log.NodeID, &log.DomainID, &log.EventType, &log.Message, &createdAtStr)
	if err != nil {
		return nil, err
	}

	if t, err := parseSQLiteTime(createdAtStr); err == nil {
		log.CreatedAt = t
	}
	return &log, nil
}

func (r *SqliteRepository) ListRecentECHLogs(ctx context.Context, clusterID int64, limit int64) ([]db.ECHLog, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, cluster_id, node_id, domain_id, event_type, message, created_at
	          FROM ech_logs WHERE cluster_id = ? ORDER BY id DESC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, clusterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]db.ECHLog, 0)
	for rows.Next() {
		var l db.ECHLog
		var createdAtStr string
		if err := rows.Scan(&l.ID, &l.ClusterID, &l.NodeID, &l.DomainID, &l.EventType, &l.Message, &createdAtStr); err != nil {
			return nil, err
		}
		if t, err := parseSQLiteTime(createdAtStr); err == nil {
			l.CreatedAt = t
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
