package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"node_monitor_go/internal/db"
	"node_monitor_go/internal/misc"
)

func (r *SqliteRepository) GetGroup(ctx context.Context, tenantID, id int64) (*db.TargetGroup, error) {
	query := "SELECT id, tenant_id, name, dns_record, dns_provider_id, check_interval_secs, enabled, created_at FROM target_groups WHERE tenant_id = ? AND id = ?"
	row := r.db.QueryRowContext(ctx, query, tenantID, id)

	var g db.TargetGroup
	var createdAtStr string
	err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.DnsRecord, &g.DnsProviderID, &g.CheckIntervalSecs, &g.Enabled, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	t, err := parseSQLiteTime(createdAtStr)
	if err == nil {
		g.CreatedAt = t
	}
	return &g, nil
}

func (r *SqliteRepository) GetGroupDirect(ctx context.Context, id int64) (*db.TargetGroup, error) {
	query := "SELECT id, tenant_id, name, dns_record, dns_provider_id, check_interval_secs, enabled, created_at FROM target_groups WHERE id = ?"
	row := r.db.QueryRowContext(ctx, query, id)

	var g db.TargetGroup
	var createdAtStr string
	err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.DnsRecord, &g.DnsProviderID, &g.CheckIntervalSecs, &g.Enabled, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	t, err := parseSQLiteTime(createdAtStr)
	if err == nil {
		g.CreatedAt = t
	}
	return &g, nil
}

func (r *SqliteRepository) ListGroups(ctx context.Context, tenantID int64) ([]db.TargetGroup, error) {
	query := "SELECT id, tenant_id, name, dns_record, dns_provider_id, check_interval_secs, enabled, created_at FROM target_groups WHERE tenant_id = ?"
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []db.TargetGroup
	for rows.Next() {
		var g db.TargetGroup
		var createdAtStr string
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.DnsRecord, &g.DnsProviderID, &g.CheckIntervalSecs, &g.Enabled, &createdAtStr); err != nil {
			return nil, err
		}
		t, err := parseSQLiteTime(createdAtStr)
		if err == nil {
			g.CreatedAt = t
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (r *SqliteRepository) ListAllGroups(ctx context.Context) ([]db.TargetGroup, error) {
	query := "SELECT id, tenant_id, name, dns_record, dns_provider_id, check_interval_secs, enabled, created_at FROM target_groups"
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []db.TargetGroup
	for rows.Next() {
		var g db.TargetGroup
		var createdAtStr string
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.DnsRecord, &g.DnsProviderID, &g.CheckIntervalSecs, &g.Enabled, &createdAtStr); err != nil {
			return nil, err
		}
		t, err := parseSQLiteTime(createdAtStr)
		if err == nil {
			g.CreatedAt = t
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (r *SqliteRepository) CreateGroup(ctx context.Context, tenantID int64, name, dnsRecord string, dnsProviderID, checkIntervalSecs int64) (*db.TargetGroup, error) {
	id := misc.GenerateRandomID()
	query := "INSERT INTO target_groups (id, tenant_id, name, dns_record, dns_provider_id, check_interval_secs, enabled) VALUES (?, ?, ?, ?, ?, ?, 1)"
	_, err := r.db.ExecContext(ctx, query, id, tenantID, name, dnsRecord, dnsProviderID, checkIntervalSecs)
	if err != nil {
		return nil, err
	}
	return r.GetGroup(ctx, tenantID, id)
}

func (r *SqliteRepository) UpdateGroup(ctx context.Context, tenantID, id int64, name, dnsRecord string, dnsProviderID, checkIntervalSecs int64) (*db.TargetGroup, error) {
	query := "UPDATE target_groups SET name = ?, dns_record = ?, dns_provider_id = ?, check_interval_secs = ? WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, name, dnsRecord, dnsProviderID, checkIntervalSecs, tenantID, id)
	if err != nil {
		return nil, err
	}
	return r.GetGroup(ctx, tenantID, id)
}

func (r *SqliteRepository) DeleteGroup(ctx context.Context, tenantID, id int64) error {
	query := "DELETE FROM target_groups WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, tenantID, id)
	return err
}

func (r *SqliteRepository) UpdateGroupEnabled(ctx context.Context, tenantID, id int64, enabled bool) error {
	query := "UPDATE target_groups SET enabled = ? WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, enabled, tenantID, id)
	return err
}

func (r *SqliteRepository) ListGroupAnomalies(ctx context.Context, groupID int64) ([]string, error) {
	query := "SELECT ip FROM group_anomalies WHERE group_id = ?"
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		ips = append(ips, ip)
	}
	return ips, rows.Err()
}

func (r *SqliteRepository) AddGroupAnomaly(ctx context.Context, groupID int64, ip string) error {
	query := "INSERT OR IGNORE INTO group_anomalies (group_id, ip) VALUES (?, ?)"
	_, err := r.db.ExecContext(ctx, query, groupID, ip)
	return err
}

func (r *SqliteRepository) DeleteGroupAnomaly(ctx context.Context, groupID int64, ip string) error {
	query := "DELETE FROM group_anomalies WHERE group_id = ? AND ip = ?"
	_, err := r.db.ExecContext(ctx, query, groupID, ip)
	return err
}
