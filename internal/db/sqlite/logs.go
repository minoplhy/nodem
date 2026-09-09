package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/misc"
)

func (r *SqliteRepository) AddCheckLog(ctx context.Context, ipID, checkID int64, success bool, message *string) (*db.CheckLog, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	id := misc.GenerateRandomID()

	var ipAddress *string
	var ipStr string
	if err := r.db.QueryRowContext(ctx, "SELECT ip FROM target_ips WHERE id = ?", ipID).Scan(&ipStr); err == nil {
		ipAddress = &ipStr
	}

	query := "INSERT INTO check_logs (id, ip_id, check_id, timestamp, success, message) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, id, ipID, checkID, nowStr, success, message)
	if err != nil {
		return nil, err
	}

	return &db.CheckLog{
		ID:        id,
		IPID:      ipID,
		CheckID:   checkID,
		Timestamp: now,
		Success:   success,
		Message:   message,
		IPAddress: ipAddress,
	}, nil
}

func (r *SqliteRepository) ListRecentGroupLogs(ctx context.Context, groupID int64, ipID *int64, limit int64) ([]db.CheckLog, error) {
	query := `SELECT cl.id, cl.ip_id, cl.check_id, cl.timestamp, cl.success, cl.message, ti.ip as ip_address
              FROM check_logs cl
              JOIN target_ips ti ON cl.ip_id = ti.id
              WHERE ti.group_id = ? AND (? IS NULL OR cl.ip_id = ?)
              ORDER BY cl.timestamp DESC
              LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, groupID, ipID, ipID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []db.CheckLog
	for rows.Next() {
		var l db.CheckLog
		var timestampStr string
		var msg, ipStr sql.NullString

		if err := rows.Scan(&l.ID, &l.IPID, &l.CheckID, &timestampStr, &l.Success, &msg, &ipStr); err != nil {
			return nil, err
		}

		t, err := parseSQLiteTime(timestampStr)
		if err == nil {
			l.Timestamp = t
		}
		if msg.Valid {
			l.Message = &msg.String
		}
		if ipStr.Valid {
			l.IPAddress = &ipStr.String
		}
		logs = append(logs, l)
	}

	return logs, rows.Err()
}
