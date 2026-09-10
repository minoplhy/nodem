package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
	"github.com/minoplhy/nodem/internal/db"
)

// SqliteRepository implements db.Repository using SQLite.
type SqliteRepository struct {
	db *sql.DB
}

// Ensure SqliteRepository implements db.Repository at compile time.
var _ db.Repository = (*SqliteRepository)(nil)

// New opens a connection pool to the specified SQLite database file.
func New(dbPath string) (*SqliteRepository, error) {
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=NORMAL&_txlock=immediate", dbPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	return &SqliteRepository{db: conn}, nil
}

// InitDB initializes the schema tables and migrations.
func (r *SqliteRepository) InitDB(ctx context.Context) error {
	return InitDB(ctx, r.db)
}

// Close checkpoints WAL pages to the database file and closes all connections.
func (r *SqliteRepository) Close() error {
	if _, err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Error("Failed to run WAL checkpoint during shutdown", "error", err)
	}
	return r.db.Close()
}
