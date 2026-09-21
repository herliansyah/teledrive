package telegram

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gotd/td/session"
	"teledrive/internal/db"
)

func TestEncryptedSessionStorage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer database.Close()

	secret := "test-secret-key-for-session"
	storage := NewEncryptedSessionStorage(database, secret)

	ctx := context.Background()

	// 1. Initially, session should return session.ErrNotFound
	_, err = storage.LoadSession(ctx)
	if err != session.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got: %v", err)
	}

	// 2. Store session
	sampleData := []byte("mtproto-auth-key-binary-data-simulation")
	if err := storage.StoreSession(ctx, sampleData); err != nil {
		t.Fatalf("StoreSession failed: %v", err)
	}

	// 3. Load session and verify
	loaded, err := storage.LoadSession(ctx)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if string(loaded) != string(sampleData) {
		t.Fatalf("Loaded session mismatch: got %q, want %q", string(loaded), string(sampleData))
	}

	// 4. Verify raw DB contains encrypted text, not raw session
	rawSetting, err := database.GetSetting("telegram_session")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if rawSetting == string(sampleData) {
		t.Fatalf("Session was stored in plaintext, expected encrypted ciphertext!")
	}
}

func TestClientManager_DisconnectAndAccountInfo(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer database.Close()

	secret := "test-secret-key-for-session"
	mgr := NewClientManager(database, 12345, "apphash", secret)

	ctx := context.Background()

	// 1. Without session stored, GetAccountInfo returns Authorized: false immediately
	info, err := mgr.GetAccountInfo(ctx)
	if err != nil {
		t.Fatalf("GetAccountInfo failed: %v", err)
	}
	if info.Authorized {
		t.Fatalf("Expected Authorized to be false without stored session")
	}

	// 2. Set dummy session and channel info in database
	_ = database.SetSetting("telegram_session", "dummy-encrypted-session")
	_ = database.SetSetting("storage_channel_id", "987654")
	_ = database.SetSetting("storage_channel_hash", "112233")
	mgr.SetStorageChannel(987654, 112233)

	cID, cHash, err := mgr.StorageChannel()
	if err != nil || cID != 987654 || cHash != 112233 {
		t.Fatalf("Expected storage channel to be set, got ID: %d, Hash: %d", cID, cHash)
	}

	// 3. Disconnect
	if err := mgr.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	// 4. Verify settings and in-memory state are cleared
	if _, err := database.GetSetting("telegram_session"); err == nil {
		t.Fatalf("Expected telegram_session to be removed from db")
	}
	if _, err := database.GetSetting("storage_channel_id"); err == nil {
		t.Fatalf("Expected storage_channel_id to be removed from db")
	}
	if _, _, err := mgr.StorageChannel(); err == nil {
		t.Fatalf("Expected StorageChannel to return error after Disconnect")
	}
}

