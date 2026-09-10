package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/misc"
)

func (r *SqliteRepository) CreateSession(ctx context.Context, sessionID, publicID string, userID int64, expiresAt time.Time, ipAddress, userAgent *string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	expiresAtStr := expiresAt.UTC().Format(time.RFC3339)
	if publicID == "" {
		if pid, err := misc.GeneratePublicSessionID(); err == nil {
			publicID = pid
		} else {
			publicID = "sess_" + sessionID[:16]
		}
	}
	query := "INSERT OR REPLACE INTO sessions (session_id, public_id, user_id, expires_at, ip_address, user_agent, last_active) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, sessionID, publicID, userID, expiresAtStr, ipAddress, userAgent, now)
	return err
}

func (r *SqliteRepository) GetSession(ctx context.Context, sessionID string) (*db.Session, error) {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := "SELECT session_id, COALESCE(public_id, session_id), user_id, expires_at, ip_address, user_agent, last_active FROM sessions WHERE session_id = ? AND expires_at > ?"
	row := r.db.QueryRowContext(ctx, query, sessionID, nowStr)

	var s db.Session
	var expiresAtStr string
	var ipAddress, userAgent, lastActiveStr sql.NullString

	err := row.Scan(&s.SessionID, &s.PublicID, &s.UserID, &expiresAtStr, &ipAddress, &userAgent, &lastActiveStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	exp, _ := parseSQLiteTime(expiresAtStr)
	s.ExpiresAt = exp

	if ipAddress.Valid {
		s.IPAddress = &ipAddress.String
	}
	if userAgent.Valid {
		s.UserAgent = &userAgent.String
	}
	s.LastActive = scanNullTime(lastActiveStr)

	// Asynchronously update last_active if not updated recently (throttled to 10s)
	if s.LastActive == nil || time.Since(*s.LastActive) > 10*time.Second {
		go func(sid string) {
			updateQuery := "UPDATE sessions SET last_active = ? WHERE session_id = ?"
			_, _ = r.db.Exec(updateQuery, time.Now().UTC().Format(time.RFC3339), sid)
		}(sessionID)
	}

	return &s, nil
}

func (r *SqliteRepository) DeleteSession(ctx context.Context, sessionID string) error {
	query := "DELETE FROM sessions WHERE session_id = ?"
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

func (r *SqliteRepository) DeleteSessionByPublicID(ctx context.Context, userID int64, id string) (bool, error) {
	query := "DELETE FROM sessions WHERE user_id = ? AND (public_id = ? OR session_id = ?)"
	res, err := r.db.ExecContext(ctx, query, userID, id, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *SqliteRepository) ListSessions(ctx context.Context, userID int64) ([]db.Session, error) {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := "SELECT session_id, COALESCE(public_id, session_id), user_id, expires_at, ip_address, user_agent, last_active FROM sessions WHERE user_id = ? AND expires_at > ? ORDER BY last_active DESC, expires_at DESC"
	rows, err := r.db.QueryContext(ctx, query, userID, nowStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []db.Session
	for rows.Next() {
		var s db.Session
		var expiresAtStr string
		var ipAddress, userAgent, lastActiveStr sql.NullString

		if err := rows.Scan(&s.SessionID, &s.PublicID, &s.UserID, &expiresAtStr, &ipAddress, &userAgent, &lastActiveStr); err != nil {
			return nil, err
		}

		exp, _ := parseSQLiteTime(expiresAtStr)
		s.ExpiresAt = exp

		if ipAddress.Valid {
			s.IPAddress = &ipAddress.String
		}
		if userAgent.Valid {
			s.UserAgent = &userAgent.String
		}
		s.LastActive = scanNullTime(lastActiveStr)

		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

func (r *SqliteRepository) CleanupExpiredSessions(ctx context.Context) error {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := "DELETE FROM sessions WHERE expires_at <= ?"
	_, err := r.db.ExecContext(ctx, query, nowStr)
	return err
}
