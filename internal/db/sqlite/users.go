package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"node_monitor_go/internal/db"
	"node_monitor_go/internal/misc"
)

func (r *SqliteRepository) GetUserByID(ctx context.Context, id int64) (*db.User, error) {
	query := "SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?"
	row := r.db.QueryRowContext(ctx, query, id)

	var u db.User
	var createdAtStr string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	t, err := parseSQLiteTime(createdAtStr)
	if err == nil {
		u.CreatedAt = t
	}
	return &u, nil
}

func (r *SqliteRepository) GetUserByUsername(ctx context.Context, username string) (*db.User, error) {
	query := "SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?"
	row := r.db.QueryRowContext(ctx, query, username)

	var u db.User
	var createdAtStr string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	t, err := parseSQLiteTime(createdAtStr)
	if err == nil {
		u.CreatedAt = t
	}
	return &u, nil
}

func (r *SqliteRepository) CreateUser(ctx context.Context, username, passwordHash, role string) (*db.User, error) {
	id := misc.GenerateRandomID()
	query := "INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, id, username, passwordHash, role)
	if err != nil {
		return nil, err
	}
	return r.GetUserByID(ctx, id)
}

func (r *SqliteRepository) UserExists(ctx context.Context) (bool, error) {
	query := "SELECT COUNT(*) as count FROM users"
	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
