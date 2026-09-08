package sqlite

import (
	"context"
	"database/sql"
	"time"

	"node_monitor_go/internal/db"
	"node_monitor_go/internal/misc"
)

func (r *SqliteRepository) ListIPs(ctx context.Context, groupID int64) ([]db.TargetIp, error) {
	query := "SELECT id, group_id, ip, dns_added, last_checked, status, display_order, enabled FROM target_ips WHERE group_id = ? ORDER BY display_order ASC, id ASC"
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []db.TargetIp
	for rows.Next() {
		var item db.TargetIp
		var lastCheckedStr sql.NullString
		if err := rows.Scan(&item.ID, &item.GroupID, &item.IP, &item.DnsAdded, &lastCheckedStr, &item.Status, &item.DisplayOrder, &item.Enabled); err != nil {
			return nil, err
		}
		item.LastChecked = scanNullTime(lastCheckedStr)
		ips = append(ips, item)
	}
	return ips, rows.Err()
}

func (r *SqliteRepository) AddIP(ctx context.Context, groupID int64, ip string) (*db.TargetIp, error) {
	orderQuery := "SELECT COALESCE(MAX(display_order), 0) + 1 FROM target_ips WHERE group_id = ?"
	var nextOrder int64
	if err := r.db.QueryRowContext(ctx, orderQuery, groupID).Scan(&nextOrder); err != nil {
		nextOrder = 1
	}

	id := misc.GenerateRandomID()
	insertQuery := "INSERT INTO target_ips (id, group_id, ip, display_order) VALUES (?, ?, ?, ?)"
	if _, err := r.db.ExecContext(ctx, insertQuery, id, groupID, ip, nextOrder); err != nil {
		return nil, err
	}

	selQuery := "SELECT id, group_id, ip, dns_added, last_checked, status, display_order, enabled FROM target_ips WHERE id = ?"
	row := r.db.QueryRowContext(ctx, selQuery, id)

	var item db.TargetIp
	var lastCheckedStr sql.NullString
	if err := row.Scan(&item.ID, &item.GroupID, &item.IP, &item.DnsAdded, &lastCheckedStr, &item.Status, &item.DisplayOrder, &item.Enabled); err != nil {
		return nil, err
	}
	item.LastChecked = scanNullTime(lastCheckedStr)
	return &item, nil
}

func (r *SqliteRepository) DeleteIP(ctx context.Context, _ int64, id int64) error {
	query := "DELETE FROM target_ips WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *SqliteRepository) UpdateIPStatus(ctx context.Context, id int64, status string, dnsAdded bool, lastChecked *time.Time) error {
	var lastCheckedStr *string
	if lastChecked != nil {
		s := lastChecked.UTC().Format(time.RFC3339)
		lastCheckedStr = &s
	}
	query := "UPDATE target_ips SET status = ?, dns_added = ?, last_checked = ? WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, status, dnsAdded, lastCheckedStr, id)
	return err
}

func (r *SqliteRepository) SyncIPs(ctx context.Context, groupID int64, newIPs []db.TargetIpInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Fetch existing IPs for group
	rows, err := tx.QueryContext(ctx, "SELECT ip FROM target_ips WHERE group_id = ?", groupID)
	if err != nil {
		return err
	}
	var currentIPs []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			rows.Close()
			return err
		}
		currentIPs = append(currentIPs, ip)
	}
	rows.Close()

	// 2. Delete IPs not in new list
	newIPMap := make(map[string]bool)
	for _, item := range newIPs {
		newIPMap[item.IP] = true
	}
	for _, current := range currentIPs {
		if !newIPMap[current] {
			if _, err := tx.ExecContext(ctx, "DELETE FROM target_ips WHERE group_id = ? AND ip = ?", groupID, current); err != nil {
				return err
			}
		}
	}

	// 3. Update existing or insert new
	currentIPMap := make(map[string]bool)
	for _, ip := range currentIPs {
		currentIPMap[ip] = true
	}

	for idx, item := range newIPs {
		displayOrder := int64(idx)
		if currentIPMap[item.IP] {
			upd := "UPDATE target_ips SET display_order = ?, enabled = ? WHERE group_id = ? AND ip = ?"
			if _, err := tx.ExecContext(ctx, upd, displayOrder, item.Enabled, groupID, item.IP); err != nil {
				return err
			}
		} else {
			id := misc.GenerateRandomID()
			ins := "INSERT OR IGNORE INTO target_ips (id, group_id, ip, display_order, enabled) VALUES (?, ?, ?, ?, ?)"
			if _, err := tx.ExecContext(ctx, ins, id, groupID, item.IP, displayOrder, item.Enabled); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
