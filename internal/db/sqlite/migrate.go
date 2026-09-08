package sqlite

import (
	"context"
	"database/sql"
)

// InitDB applies table definitions, indexes, and schema alterations.
func InitDB(ctx context.Context, db *sql.DB) error {
	queries := []string{
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
		`CREATE INDEX IF NOT EXISTS idx_check_logs_ip_id_timestamp ON check_logs(ip_id, timestamp DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_target_groups_tenant_id ON target_groups(tenant_id);`,
		`CREATE INDEX IF NOT EXISTS idx_target_ips_group_id ON target_ips(group_id);`,
		`CREATE INDEX IF NOT EXISTS idx_checks_group_id ON checks(group_id);`,
		`CREATE INDEX IF NOT EXISTS idx_group_rules_group_id ON group_rules(group_id);`,
		`CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_id ON notification_channels(tenant_id);`,
		`CREATE INDEX IF NOT EXISTS idx_dns_providers_tenant_id ON dns_providers(tenant_id);`,
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	// Schema alterations for backward compatibility with older DB versions (ignoring errors if columns already exist)
	alters := []string{
		`ALTER TABLE target_groups ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;`,
		`ALTER TABLE target_ips ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;`,
		`ALTER TABLE target_ips ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE check_states ADD COLUMN message TEXT;`,
		`ALTER TABLE sessions ADD COLUMN ip_address TEXT;`,
		`ALTER TABLE sessions ADD COLUMN user_agent TEXT;`,
		`ALTER TABLE sessions ADD COLUMN last_active DATETIME;`,
		`ALTER TABLE sessions ADD COLUMN public_id TEXT;`,
		`UPDATE sessions SET public_id = hex(randomblob(16)) WHERE public_id IS NULL OR public_id = '';`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_public_id ON sessions(public_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`,
	}

	for _, alter := range alters {
		_, _ = db.ExecContext(ctx, alter)
	}

	return nil
}
