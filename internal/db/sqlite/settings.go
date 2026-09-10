package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/minoplhy/nodem/internal/ech/engine"
)

const (
	SettingServerSigningPublicKey  = "server_signing_public_key"
	SettingServerSigningPrivateKey = "server_signing_private_key"
)

// GetSystemSetting retrieves the value for a given system setting key.
func (r *SqliteRepository) GetSystemSetting(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM system_settings WHERE key = ?`
	var val string
	err := r.db.QueryRowContext(ctx, query, key).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get system setting %s: %w", key, err)
	}
	return val, nil
}

// SetSystemSetting inserts or updates a system setting value.
func (r *SqliteRepository) SetSystemSetting(ctx context.Context, key, value string) error {
	query := `INSERT INTO system_settings (key, value, updated_at) VALUES (?, ?, ?)
	          ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := r.db.ExecContext(ctx, query, key, value, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set system setting %s: %w", key, err)
	}
	return nil
}

// GetOrCreateServerSigningKey returns the server's Ed25519 root signing key pair, generating and persisting them if not yet created.
func (r *SqliteRepository) GetOrCreateServerSigningKey(ctx context.Context) (pubKey, privKey string, err error) {
	pubKey, err = r.GetSystemSetting(ctx, SettingServerSigningPublicKey)
	if err != nil {
		return "", "", err
	}
	privKey, err = r.GetSystemSetting(ctx, SettingServerSigningPrivateKey)
	if err != nil {
		return "", "", err
	}

	if pubKey != "" && privKey != "" {
		return pubKey, privKey, nil
	}

	// Generate new key pair
	pubKey, privKey, err = engine.GenerateSigningKeyPair()
	if err != nil {
		return "", "", fmt.Errorf("failed generating server signing key pair: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed starting tx for server signing keys: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO system_settings (key, value, updated_at) VALUES (?, ?, ?)
	          ON CONFLICT(key) DO NOTHING`
	now := time.Now()
	if _, err := tx.ExecContext(ctx, query, SettingServerSigningPublicKey, pubKey, now); err != nil {
		return "", "", fmt.Errorf("failed storing server signing public key: %w", err)
	}
	if _, err := tx.ExecContext(ctx, query, SettingServerSigningPrivateKey, privKey, now); err != nil {
		return "", "", fmt.Errorf("failed storing server signing private key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("failed committing server signing keys: %w", err)
	}

	// Re-read in case of race condition with another worker
	finalPub, _ := r.GetSystemSetting(ctx, SettingServerSigningPublicKey)
	finalPriv, _ := r.GetSystemSetting(ctx, SettingServerSigningPrivateKey)
	if finalPub != "" && finalPriv != "" {
		return finalPub, finalPriv, nil
	}

	return pubKey, privKey, nil
}
