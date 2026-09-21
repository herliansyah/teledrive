package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

type Folder struct {
	ID        string     `json:"id"`
	ParentID  *string    `json:"parent_id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type File struct {
	ID                 string     `json:"id"`
	FolderID           *string    `json:"folder_id"`
	Name               string     `json:"name"`
	Size               int64      `json:"size"`
	MimeType           string     `json:"mime_type"`
	TelegramMessageID  int        `json:"telegram_message_id"`
	TelegramFileID     string     `json:"telegram_file_id"`
	TelegramAccessHash string     `json:"telegram_access_hash"`
	SHA256             string     `json:"sha256"`
	IsEncrypted        int        `json:"is_encrypted"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

type UploadSession struct {
	ID             string    `json:"id"`
	FolderID       *string   `json:"folder_id"`
	Name           string    `json:"name"`
	Size           int64     `json:"size"`
	MimeType       string    `json:"mime_type"`
	TotalParts     int       `json:"total_parts"`
	UploadedParts  int       `json:"uploaded_parts"`
	TelegramFileID int64     `json:"telegram_file_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type ShareLink struct {
	ID            string     `json:"id"`
	Token         string     `json:"token"`
	FileID        string     `json:"file_id"`
	PasswordHash  *string    `json:"password_hash"`
	ExpiresAt     *time.Time `json:"expires_at"`
	DownloadCount int        `json:"download_count"`
	MaxDownloads  *int       `json:"max_downloads"`
	CreatedAt     time.Time  `json:"created_at"`
}

// GenerateID produces a random 16-hex string ID.
func GenerateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateID() string {
	return GenerateID()
}

// CreateFolder adds a new virtual folder.
func (d *DB) CreateFolder(name string, parentID *string) (*Folder, error) {
	id := generateID()
	_, err := d.Exec("INSERT INTO folders (id, parent_id, name) VALUES (?, ?, ?)", id, parentID, name)
	if err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	return d.GetFolder(id)
}

// GetFolder retrieves a single folder by ID.
func (d *DB) GetFolder(id string) (*Folder, error) {
	row := d.QueryRow("SELECT id, parent_id, name, created_at, updated_at, deleted_at FROM folders WHERE id = ?", id)
	var f Folder
	if err := row.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListFolders lists subfolders inside parentID (or root if parentID is nil) excluding trashed folders.
func (d *DB) ListFolders(parentID *string) ([]Folder, error) {
	var rows *sql.Rows
	var err error
	if parentID == nil {
		rows, err = d.Query("SELECT id, parent_id, name, created_at, updated_at, deleted_at FROM folders WHERE parent_id IS NULL AND deleted_at IS NULL ORDER BY name ASC")
	} else {
		rows, err = d.Query("SELECT id, parent_id, name, created_at, updated_at, deleted_at FROM folders WHERE parent_id = ? AND deleted_at IS NULL ORDER BY name ASC", *parentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []Folder
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// RenameFolder updates the name of a folder.
func (d *DB) RenameFolder(id, newName string) error {
	_, err := d.Exec("UPDATE folders SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newName, id)
	return err
}

// MoveFolder moves a folder under a new parent (with cycle prevention check).
func (d *DB) MoveFolder(id string, newParentID *string) error {
	if newParentID != nil && *newParentID == id {
		return fmt.Errorf("cannot move folder inside itself")
	}
	// Check for ancestor cycle
	curr := newParentID
	for curr != nil {
		var p *string
		err := d.QueryRow("SELECT parent_id FROM folders WHERE id = ?", *curr).Scan(&p)
		if err != nil {
			break
		}
		if p != nil && *p == id {
			return fmt.Errorf("cannot move folder into its own descendant")
		}
		curr = p
	}

	_, err := d.Exec("UPDATE folders SET parent_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newParentID, id)
	return err
}

// FindFolderByName locates an active (non-trashed) folder by its exact name under parentID.
func (d *DB) FindFolderByName(name string, parentID *string) (*Folder, error) {
	var row *sql.Row
	if parentID == nil {
		row = d.QueryRow("SELECT id, parent_id, name, created_at, updated_at, deleted_at FROM folders WHERE name = ? AND parent_id IS NULL AND deleted_at IS NULL", name)
	} else {
		row = d.QueryRow("SELECT id, parent_id, name, created_at, updated_at, deleted_at FROM folders WHERE name = ? AND parent_id = ? AND deleted_at IS NULL", name, *parentID)
	}
	var f Folder
	if err := row.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

// FindFileByName locates an active (non-trashed) file by its exact name under folderID.
func (d *DB) FindFileByName(name string, folderID *string) (*File, error) {
	var row *sql.Row
	if folderID == nil {
		row = d.QueryRow(`
			SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
			FROM files WHERE name = ? AND folder_id IS NULL AND deleted_at IS NULL
		`, name)
	} else {
		row = d.QueryRow(`
			SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
			FROM files WHERE name = ? AND folder_id = ? AND deleted_at IS NULL
		`, name, *folderID)
	}
	var f File
	if err := row.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

// ResolvePath resolves a virtual clean path (e.g. "/Projects/notes.txt") to a Folder, a File, or os.ErrNotExist.
// Root path ("/" or "") returns (nil, nil, nil) representing the virtual root directory.
func (d *DB) ResolvePath(virtualPath string) (*Folder, *File, error) {
	cleaned := path.Clean("/" + virtualPath)
	if cleaned == "/" || cleaned == "." {
		return nil, nil, nil
	}

	parts := strings.Split(strings.Trim(cleaned, "/"), "/")
	var currentParentID *string

	for i, part := range parts {
		isLast := (i == len(parts)-1)

		if isLast {
			if folder, err := d.FindFolderByName(part, currentParentID); err == nil {
				return folder, nil, nil
			}
			if file, err := d.FindFileByName(part, currentParentID); err == nil {
				return nil, file, nil
			}
			return nil, nil, os.ErrNotExist
		}

		folder, err := d.FindFolderByName(part, currentParentID)
		if err != nil {
			return nil, nil, os.ErrNotExist
		}
		currentParentID = &folder.ID
	}

	return nil, nil, os.ErrNotExist
}

// SoftDeleteFolder moves a folder and all its descendants to Virtual Trash.
func (d *DB) SoftDeleteFolder(id string) error {
	_, err := d.Exec("UPDATE folders SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return err
	}
	// Cascade soft-delete to all subfolders and files
	_, _ = d.Exec(`
		WITH RECURSIVE subfolders AS (
			SELECT id FROM folders WHERE id = ?
			UNION ALL
			SELECT f.id FROM folders f JOIN subfolders s ON f.parent_id = s.id
		)
		UPDATE files SET deleted_at = CURRENT_TIMESTAMP WHERE folder_id IN (SELECT id FROM subfolders)
	`, id)
	_, _ = d.Exec(`
		WITH RECURSIVE subfolders AS (
			SELECT id FROM folders WHERE id = ?
			UNION ALL
			SELECT f.id FROM folders f JOIN subfolders s ON f.parent_id = s.id
		)
		UPDATE folders SET deleted_at = CURRENT_TIMESTAMP WHERE id IN (SELECT id FROM subfolders)
	`, id)
	return nil
}

// RestoreFolder restores a folder and its contents from Virtual Trash.
func (d *DB) RestoreFolder(id string) error {
	_, err := d.Exec("UPDATE folders SET deleted_at = NULL WHERE id = ?", id)
	if err != nil {
		return err
	}
	_, _ = d.Exec(`
		WITH RECURSIVE subfolders AS (
			SELECT id FROM folders WHERE id = ?
			UNION ALL
			SELECT f.id FROM folders f JOIN subfolders s ON f.parent_id = s.id
		)
		UPDATE files SET deleted_at = NULL WHERE folder_id IN (SELECT id FROM subfolders)
	`, id)
	_, _ = d.Exec(`
		WITH RECURSIVE subfolders AS (
			SELECT id FROM folders WHERE id = ?
			UNION ALL
			SELECT f.id FROM folders f JOIN subfolders s ON f.parent_id = s.id
		)
		UPDATE folders SET deleted_at = NULL WHERE id IN (SELECT id FROM subfolders)
	`, id)
	return nil
}

// HardDeleteFolder permanently deletes a folder record from the database.
func (d *DB) HardDeleteFolder(id string) error {
	_, err := d.Exec("DELETE FROM folders WHERE id = ?", id)
	return err
}

// DeleteFolder defaults to soft-deleting the folder into Virtual Trash.
func (d *DB) DeleteFolder(id string) error {
	return d.SoftDeleteFolder(id)
}

// CreateFileWithID inserts a new file record with a specified ID.
func (d *DB) CreateFileWithID(id string, folderID *string, name string, size int64, mimeType string, msgID int, fileID, accessHash, sha256 string, isEncrypted ...int) (*File, error) {
	enc := 0
	if len(isEncrypted) > 0 {
		enc = isEncrypted[0]
	}
	_, err := d.Exec(`
		INSERT INTO files (id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, folderID, name, size, mimeType, msgID, fileID, accessHash, sha256, enc)
	if err != nil {
		return nil, fmt.Errorf("create file record: %w", err)
	}
	return d.GetFile(id)
}

// CreateFile inserts a new file record linked to Telegram object metadata.
func (d *DB) CreateFile(folderID *string, name string, size int64, mimeType string, msgID int, fileID, accessHash, sha256 string, isEncrypted ...int) (*File, error) {
	return d.CreateFileWithID(generateID(), folderID, name, size, mimeType, msgID, fileID, accessHash, sha256, isEncrypted...)
}

// GetFile retrieves a file by ID.
func (d *DB) GetFile(id string) (*File, error) {
	row := d.QueryRow(`
		SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
		FROM files WHERE id = ?
	`, id)
	var f File
	if err := row.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListFiles returns all files within a specific folder excluding trashed files.
func (d *DB) ListFiles(folderID *string) ([]File, error) {
	var rows *sql.Rows
	var err error
	if folderID == nil {
		rows, err = d.Query(`
			SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
			FROM files WHERE folder_id IS NULL AND deleted_at IS NULL ORDER BY name ASC
		`)
	} else {
		rows, err = d.Query(`
			SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
			FROM files WHERE folder_id = ? AND deleted_at IS NULL ORDER BY name ASC
		`, *folderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// RenameFile updates a file's name.
func (d *DB) RenameFile(id, newName string) error {
	_, err := d.Exec("UPDATE files SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newName, id)
	return err
}

// MoveFile changes the parent folder of a file.
func (d *DB) MoveFile(id string, newFolderID *string) error {
	_, err := d.Exec("UPDATE files SET folder_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newFolderID, id)
	return err
}

// SoftDeleteFile moves a file into Virtual Trash.
func (d *DB) SoftDeleteFile(id string) error {
	_, err := d.Exec("UPDATE files SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// RestoreFile restores a file from Virtual Trash.
func (d *DB) RestoreFile(id string) error {
	_, err := d.Exec("UPDATE files SET deleted_at = NULL WHERE id = ?", id)
	return err
}

// HardDeleteFile permanently deletes a file record from the database.
func (d *DB) HardDeleteFile(id string) error {
	_, err := d.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

// DeleteFile defaults to soft-deleting the file into Virtual Trash.
func (d *DB) DeleteFile(id string) error {
	return d.SoftDeleteFile(id)
}

// SearchFiles finds files matching a name query, excluding trashed files.
func (d *DB) SearchFiles(query string) ([]File, error) {
	rows, err := d.Query(`
		SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
		FROM files WHERE name LIKE ? AND deleted_at IS NULL ORDER BY name ASC LIMIT 50
	`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ListTrash returns all folders and files currently marked as deleted.
func (d *DB) ListTrash() ([]Folder, []File, error) {
	folderRows, err := d.Query("SELECT id, parent_id, name, created_at, updated_at, deleted_at FROM folders WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC")
	if err != nil {
		return nil, nil, err
	}
	defer folderRows.Close()

	var folders []Folder
	for folderRows.Next() {
		var f Folder
		if err := folderRows.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, nil, err
		}
		folders = append(folders, f)
	}

	fileRows, err := d.Query(`
		SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
		FROM files WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC
	`)
	if err != nil {
		return nil, nil, err
	}
	defer fileRows.Close()

	var files []File
	for fileRows.Next() {
		var f File
		if err := fileRows.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, nil, err
		}
		files = append(files, f)
	}

	return folders, files, nil
}

// GetExpiredTrashFiles returns files in Virtual Trash older than cutoff duration.
func (d *DB) GetExpiredTrashFiles(cutoff time.Time) ([]File, error) {
	rows, err := d.Query(`
		SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
		FROM files WHERE deleted_at IS NOT NULL AND deleted_at < ?
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// PurgeExpiredTrash permanently removes trashed folders and files older than cutoff.
func (d *DB) PurgeExpiredTrash(cutoff time.Time) (int64, error) {
	res1, err := d.Exec("DELETE FROM files WHERE deleted_at IS NOT NULL AND deleted_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	n1, _ := res1.RowsAffected()

	res2, err := d.Exec("DELETE FROM folders WHERE deleted_at IS NOT NULL AND deleted_at < ?", cutoff)
	if err != nil {
		return n1, err
	}
	n2, _ := res2.RowsAffected()

	return n1 + n2, nil
}

// EmptyTrash removes all items currently in Virtual Trash and returns the deleted files.
func (d *DB) EmptyTrash() ([]File, error) {
	rows, err := d.Query(`
		SELECT id, folder_id, name, size, mime_type, telegram_message_id, telegram_file_id, telegram_access_hash, sha256, is_encrypted, created_at, updated_at, deleted_at
		FROM files WHERE deleted_at IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.TelegramMessageID, &f.TelegramFileID, &f.TelegramAccessHash, &f.SHA256, &f.IsEncrypted, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	_, _ = d.Exec("DELETE FROM files WHERE deleted_at IS NOT NULL")
	_, _ = d.Exec("DELETE FROM folders WHERE deleted_at IS NOT NULL")

	return files, nil
}

// CreateUploadSession initializes an in-flight upload session.
func (d *DB) CreateUploadSession(folderID *string, name string, size int64, mimeType string, totalParts int, tgFileID int64) (*UploadSession, error) {
	id := generateID()
	_, err := d.Exec(`
		INSERT INTO upload_sessions (id, folder_id, name, size, mime_type, total_parts, uploaded_parts, telegram_file_id)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?)
	`, id, folderID, name, size, mimeType, totalParts, tgFileID)
	if err != nil {
		return nil, err
	}
	return &UploadSession{
		ID:             id,
		FolderID:       folderID,
		Name:           name,
		Size:           size,
		MimeType:       mimeType,
		TotalParts:     totalParts,
		UploadedParts:  0,
		TelegramFileID: tgFileID,
		CreatedAt:      time.Now(),
	}, nil
}

// IncrementUploadPart marks an additional part as received.
func (d *DB) IncrementUploadPart(id string) (int, error) {
	var count int
	err := d.QueryRow(`
		UPDATE upload_sessions SET uploaded_parts = uploaded_parts + 1 WHERE id = ? RETURNING uploaded_parts
	`, id).Scan(&count)
	return count, err
}

// GetUploadSession retrieves an active upload session.
func (d *DB) GetUploadSession(id string) (*UploadSession, error) {
	var s UploadSession
	err := d.QueryRow(`
		SELECT id, folder_id, name, size, mime_type, total_parts, uploaded_parts, telegram_file_id, created_at
		FROM upload_sessions WHERE id = ?
	`, id).Scan(&s.ID, &s.FolderID, &s.Name, &s.Size, &s.MimeType, &s.TotalParts, &s.UploadedParts, &s.TelegramFileID, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteUploadSession cleans up an upload session.
func (d *DB) DeleteUploadSession(id string) error {
	_, err := d.Exec("DELETE FROM upload_sessions WHERE id = ?", id)
	return err
}

// CreateShareLink generates a public token for a file.
func (d *DB) CreateShareLink(fileID string, passwordHash *string, expiresAt *time.Time, maxDownloads *int) (*ShareLink, error) {
	id := generateID()
	token := generateID() + generateID() // 32 hex chars
	_, err := d.Exec(`
		INSERT INTO share_links (id, token, file_id, password_hash, expires_at, max_downloads)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, token, fileID, passwordHash, expiresAt, maxDownloads)
	if err != nil {
		return nil, err
	}
	return d.GetShareLink(token)
}

// GetShareLink finds a share link by its public token.
func (d *DB) GetShareLink(token string) (*ShareLink, error) {
	var sl ShareLink
	err := d.QueryRow(`
		SELECT id, token, file_id, password_hash, expires_at, download_count, max_downloads, created_at
		FROM share_links WHERE token = ?
	`, token).Scan(&sl.ID, &sl.Token, &sl.FileID, &sl.PasswordHash, &sl.ExpiresAt, &sl.DownloadCount, &sl.MaxDownloads, &sl.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &sl, nil
}

// IncrementShareDownload tracks download counts on a share link.
func (d *DB) IncrementShareDownload(token string) error {
	_, err := d.Exec("UPDATE share_links SET download_count = download_count + 1 WHERE token = ?", token)
	return err
}

type ShareLinkInfo struct {
	ID            string     `json:"id"`
	Token         string     `json:"token"`
	FileID        string     `json:"file_id"`
	FileName      string     `json:"file_name"`
	FileSize      int64      `json:"file_size"`
	HasPassword   bool       `json:"has_password"`
	ExpiresAt     *time.Time `json:"expires_at"`
	DownloadCount int        `json:"download_count"`
	MaxDownloads  *int       `json:"max_downloads"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ListShareLinks returns all active public share links joined with file metadata.
func (d *DB) ListShareLinks() ([]ShareLinkInfo, error) {
	rows, err := d.Query(`
		SELECT s.id, s.token, s.file_id, COALESCE(f.name, 'Deleted File'), COALESCE(f.size, 0),
		       (s.password_hash IS NOT NULL AND s.password_hash != ''),
		       s.expires_at, s.download_count, s.max_downloads, s.created_at
		FROM share_links s
		LEFT JOIN files f ON s.file_id = f.id
		ORDER BY s.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []ShareLinkInfo
	for rows.Next() {
		var s ShareLinkInfo
		if err := rows.Scan(&s.ID, &s.Token, &s.FileID, &s.FileName, &s.FileSize, &s.HasPassword, &s.ExpiresAt, &s.DownloadCount, &s.MaxDownloads, &s.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	return shares, rows.Err()
}

// DeleteShareLink removes a public share link by its ID or token.
func (d *DB) DeleteShareLink(id string) error {
	_, err := d.Exec("DELETE FROM share_links WHERE id = ? OR token = ?", id, id)
	return err
}

