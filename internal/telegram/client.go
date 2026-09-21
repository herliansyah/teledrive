package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"teledrive/internal/db"
)

type ClientManager struct {
	client    *telegram.Client
	api       *tg.Client
	db        *db.DB
	appID     int
	appHash   string
	secretKey string
	storage   *EncryptedSessionStorage

	mu                  sync.Mutex
	channelID           int64
	accessHash          int64
	configuredChannelID int64
}

// NewClientManager configures a new MTProto client instance with Safe Mode client headers.
func NewClientManager(database *db.DB, appID int, appHash, secretKey string) *ClientManager {
	storage := NewEncryptedSessionStorage(database, secretKey)

	// Organic client fingerprinting per ToS safe mode
	device := telegram.DeviceConfig{
		DeviceModel:    "PC 64bit",
		SystemVersion:  "Linux/x86_64",
		AppVersion:     "5.0.0",
		LangCode:       "en",
		SystemLangCode: "en",
	}

	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: storage,
		Device:         device,
	})

	return &ClientManager{
		client:    client,
		api:       tg.NewClient(client),
		db:        database,
		appID:     appID,
		appHash:   appHash,
		secretKey: secretKey,
		storage:   storage,
	}
}

// Client returns the underlying gotd telegram.Client.
func (m *ClientManager) Client() *telegram.Client {
	return m.client
}

// API returns the raw tg.Client for MTProto RPC calls.
func (m *ClientManager) API() *tg.Client {
	return m.api
}

// Run executes the client event loop until ctx is canceled.
func (m *ClientManager) Run(ctx context.Context, f func(ctx context.Context) error) error {
	return m.client.Run(ctx, f)
}

// SetStorageChannel caches the active storage channel credentials.
func (m *ClientManager) SetStorageChannel(channelID, accessHash int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channelID = channelID
	m.accessHash = accessHash
}

// SetConfiguredChannelID sets an explicit storage channel ID from configuration or env var.
func (m *ClientManager) SetConfiguredChannelID(channelID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configuredChannelID = channelID
}

// SetDB updates the database reference (e.g. after snapshot restoration).
func (m *ClientManager) SetDB(database *db.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.db = database
}

// StorageChannel returns the active storage channel channel_id and access_hash.
func (m *ClientManager) StorageChannel() (int64, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.channelID == 0 {
		return 0, 0, fmt.Errorf("storage channel not initialized")
	}
	return m.channelID, m.accessHash, nil
}

// AccountInfo holds metadata about the connected Telegram user and active storage channel.
type AccountInfo struct {
	Authorized bool   `json:"authorized"`
	ID         int64  `json:"id,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	Username   string `json:"username,omitempty"`
	Phone      string `json:"phone,omitempty"`
	ChannelID  int64  `json:"channel_id,omitempty"`
}

// GetAccountInfo returns the authorization status and user profile of the current Telegram session.
func (m *ClientManager) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	sessionStr, err := m.db.GetSetting("telegram_session")
	if err != nil || sessionStr == "" {
		return &AccountInfo{Authorized: false}, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	status, err := m.client.Auth().Status(timeoutCtx)
	if err != nil || !status.Authorized {
		return &AccountInfo{Authorized: false}, nil
	}

	info := &AccountInfo{
		Authorized: true,
	}
	if status.User != nil {
		info.ID = status.User.ID
		info.FirstName = status.User.FirstName
		info.LastName = status.User.LastName
		info.Username = status.User.Username
		info.Phone = status.User.Phone
	}

	m.mu.Lock()
	info.ChannelID = m.channelID
	m.mu.Unlock()

	return info, nil
}

// Disconnect revokes the active MTProto session on Telegram and clears local session credentials.
func (m *ClientManager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Attempt graceful remote MTProto session revocation if session exists
	if sessionStr, err := m.db.GetSetting("telegram_session"); err == nil && sessionStr != "" {
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if status, err := m.client.Auth().Status(timeoutCtx); err == nil && status.Authorized {
			_, _ = m.api.AuthLogOut(timeoutCtx)
		}
	}

	// 2. Wipe local session and storage channel settings from SQLite
	_ = m.db.DeleteSetting("telegram_session")
	_ = m.db.DeleteSetting("storage_channel_id")
	_ = m.db.DeleteSetting("storage_channel_hash")

	// 3. Clear in-memory active channel state
	m.channelID = 0
	m.accessHash = 0
	return nil
}


