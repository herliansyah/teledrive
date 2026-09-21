package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS folders (
    id TEXT PRIMARY KEY,
    parent_id TEXT NULL REFERENCES folders(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);
CREATE INDEX IF NOT EXISTS idx_folders_parent ON folders(parent_id);

CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    folder_id TEXT NULL REFERENCES folders(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    telegram_message_id INTEGER NOT NULL,
    telegram_file_id TEXT NOT NULL,
    telegram_access_hash TEXT NOT NULL,
    sha256 TEXT,
    is_encrypted INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);
CREATE INDEX IF NOT EXISTS idx_files_folder ON files(folder_id);
CREATE INDEX IF NOT EXISTS idx_files_name ON files(name);

CREATE TABLE IF NOT EXISTS upload_sessions (
    id TEXT PRIMARY KEY,
    folder_id TEXT REFERENCES folders(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    total_parts INTEGER NOT NULL,
    uploaded_parts INTEGER DEFAULT 0,
    telegram_file_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS share_links (
    id TEXT PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    file_id TEXT NULL REFERENCES files(id) ON DELETE CASCADE,
    folder_id TEXT NULL REFERENCES folders(id) ON DELETE CASCADE,
    password_hash TEXT NULL,
    expires_at DATETIME NULL,
    download_count INTEGER DEFAULT 0,
    max_downloads INTEGER NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_share_token ON share_links(token);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// Open initializes the SQLite database with WAL journal mode and executes the schema.
func Open(path string) (*DB, error) {
	// ponytail: SQLite with WAL mode and 5s busy timeout handles concurrent readers
	// and serialized writes cleanly without external database servers.
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// Limit connection pool to ensure thread safety
	sdb.SetMaxOpenConns(10)
	sdb.SetMaxIdleConns(5)
	sdb.SetConnMaxLifetime(time.Hour)

	if _, err := sdb.Exec(schema); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	// Run lightweight migrations for existing databases
	_, _ = sdb.Exec("ALTER TABLE folders ADD COLUMN deleted_at DATETIME NULL")
	_, _ = sdb.Exec("ALTER TABLE files ADD COLUMN deleted_at DATETIME NULL")
	_, _ = sdb.Exec("ALTER TABLE files ADD COLUMN is_encrypted INTEGER DEFAULT 0")
	_, _ = sdb.Exec("ALTER TABLE share_links ADD COLUMN folder_id TEXT NULL REFERENCES folders(id) ON DELETE CASCADE")
	_, _ = sdb.Exec("CREATE INDEX IF NOT EXISTS idx_folders_deleted ON folders(deleted_at)")
	_, _ = sdb.Exec("CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(deleted_at)")

	return &DB{DB: sdb}, nil
}

// GetSetting retrieves a setting value by key.
func (d *DB) GetSetting(key string) (string, error) {
	var val string
	err := d.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

// SetSetting stores or updates a setting key-value pair.
func (d *DB) SetSetting(key, val string) error {
	_, err := d.Exec(`
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, val)
	return err
}

// DeleteSetting removes a setting key-value pair from the database.
func (d *DB) DeleteSetting(key string) error {
	_, err := d.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

// PurgeAllData deletes all virtual folders, files, upload sessions, share links, and Telegram session settings.
func (d *DB) PurgeAllData() error {
	queries := []string{
		"DELETE FROM share_links",
		"DELETE FROM upload_sessions",
		"DELETE FROM files",
		"DELETE FROM folders",
		"DELETE FROM settings WHERE key IN ('telegram_session', 'storage_channel_id', 'storage_channel_hash')",
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

