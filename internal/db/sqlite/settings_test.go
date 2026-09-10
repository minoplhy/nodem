package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSystemSettingsAndServerSigningKey(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_settings.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	if err := repo.InitDB(ctx); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// 1. Test basic Get/Set
	val, err := repo.GetSystemSetting(ctx, "custom_key")
	if err != nil {
		t.Fatalf("unexpected error on missing key: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty value, got %s", val)
	}

	if err := repo.SetSystemSetting(ctx, "custom_key", "custom_val"); err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}
	val, err = repo.GetSystemSetting(ctx, "custom_key")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}
	if val != "custom_val" {
		t.Fatalf("expected custom_val, got %s", val)
	}

	// 2. Test GetOrCreateServerSigningKey
	pubKey1, privKey1, err := repo.GetOrCreateServerSigningKey(ctx)
	if err != nil {
		t.Fatalf("GetOrCreateServerSigningKey failed: %v", err)
	}
	if pubKey1 == "" || privKey1 == "" {
		t.Fatalf("expected non-empty key pair, got pub=%q, priv=%q", pubKey1, privKey1)
	}

	// Call again, should be identical (persistent)
	pubKey2, privKey2, err := repo.GetOrCreateServerSigningKey(ctx)
	if err != nil {
		t.Fatalf("GetOrCreateServerSigningKey second call failed: %v", err)
	}
	if pubKey1 != pubKey2 || privKey1 != privKey2 {
		t.Fatalf("expected persistent keypair across calls, got (%s, %s) vs (%s, %s)", pubKey1, privKey1, pubKey2, privKey2)
	}
}
