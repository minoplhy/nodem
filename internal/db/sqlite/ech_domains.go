package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/minoplhy/nodem/internal/db"
)

// scanECHDomain parses a row into an ECHDomain struct.
func scanECHDomain(scanner interface{ Scan(dest ...any) error }) (*db.ECHDomain, error) {
	var d db.ECHDomain
	var targetGroupIDNull sql.NullInt64
	var ipv4HintNull, ipv6HintNull, lastSyncedStr sql.NullString
	var createdAtStr string

	err := scanner.Scan(
		&d.ID,
		&d.ClusterID,
		&d.DNSProviderID,
		&targetGroupIDNull,
		&d.Domain,
		&d.TTL,
		&d.ALPN,
		&ipv4HintNull,
		&ipv6HintNull,
		&lastSyncedStr,
		&d.DNSStatus,
		&createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if targetGroupIDNull.Valid {
		d.TargetGroupID = &targetGroupIDNull.Int64
	}
	if ipv4HintNull.Valid {
		d.IPv4Hint = &ipv4HintNull.String
	}
	if ipv6HintNull.Valid {
		d.IPv6Hint = &ipv6HintNull.String
	}
	d.LastSyncedAt = scanNullTime(lastSyncedStr)
	if t, err := parseSQLiteTime(createdAtStr); err == nil {
		d.CreatedAt = t
	}

	return &d, nil
}

func (r *SqliteRepository) GetECHDomain(ctx context.Context, id int64) (*db.ECHDomain, error) {
	query := `SELECT id, cluster_id, dns_provider_id, target_group_id, domain, ttl, alpn,
	                 ipv4_hint, ipv6_hint, last_synced_at, dns_status, created_at
	          FROM ech_domains WHERE id = ?`
	return scanECHDomain(r.db.QueryRowContext(ctx, query, id))
}

func (r *SqliteRepository) ListECHDomains(ctx context.Context, clusterID int64) ([]db.ECHDomain, error) {
	query := `SELECT id, cluster_id, dns_provider_id, target_group_id, domain, ttl, alpn,
	                 ipv4_hint, ipv6_hint, last_synced_at, dns_status, created_at
	          FROM ech_domains WHERE cluster_id = ? ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	domains := make([]db.ECHDomain, 0)
	for rows.Next() {
		d, err := scanECHDomain(rows)
		if err != nil {
			return nil, err
		}
		if d != nil {
			domains = append(domains, *d)
		}
	}
	return domains, rows.Err()
}

func (r *SqliteRepository) CreateECHDomain(ctx context.Context, clusterID, dnsProviderID int64, targetGroupID *int64, domain string, ttl int, alpn string, ipv4Hint, ipv6Hint *string) (*db.ECHDomain, error) {
	if ttl <= 0 {
		ttl = 300
	}
	if alpn == "" {
		alpn = "h2,h3"
	}

	query := `INSERT INTO ech_domains (
		cluster_id, dns_provider_id, target_group_id, domain, ttl, alpn, ipv4_hint, ipv6_hint, dns_status
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'PENDING')`

	res, err := r.db.ExecContext(ctx, query, clusterID, dnsProviderID, targetGroupID, domain, ttl, alpn, ipv4Hint, ipv6Hint)
	if err != nil {
		return nil, fmt.Errorf("failed to create ech domain: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetECHDomain(ctx, id)
}

func (r *SqliteRepository) DeleteECHDomain(ctx context.Context, id int64) error {
	query := `DELETE FROM ech_domains WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("ech domain not found")
	}
	return nil
}

func (r *SqliteRepository) UpdateECHDomainSyncStatus(ctx context.Context, id int64, status string, lastSyncedAt *time.Time) error {
	var lastSyncedStr sql.NullString
	if lastSyncedAt != nil {
		lastSyncedStr = sql.NullString{
			String: lastSyncedAt.UTC().Format("2006-01-02 15:04:05"),
			Valid:  true,
		}
	}

	query := `UPDATE ech_domains SET dns_status = ?, last_synced_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, lastSyncedStr, id)
	return err
}
