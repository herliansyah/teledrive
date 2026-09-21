package db

import (
	"path/filepath"
	"testing"
)

func TestDB_FolderAndFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer database.Close()

	// 1. Create root folder
	root, err := database.CreateFolder("Work", nil)
	if err != nil {
		t.Fatalf("CreateFolder root failed: %v", err)
	}

	// 2. Create subfolder
	sub, err := database.CreateFolder("Projects", &root.ID)
	if err != nil {
		t.Fatalf("CreateFolder sub failed: %v", err)
	}

	// 3. Test cycle prevention (cannot move root into its subfolder)
	err = database.MoveFolder(root.ID, &sub.ID)
	if err == nil {
		t.Fatalf("Expected error when moving parent into its child, got nil")
	}

	// 4. Create file in subfolder
	file, err := database.CreateFile(&sub.ID, "report.pdf", 1048576, "application/pdf", 1234, "file_xyz", "access_hash_abc", "dummy_sha256")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// 5. Search file
	results, err := database.SearchFiles("report")
	if err != nil || len(results) != 1 {
		t.Fatalf("SearchFiles failed, expected 1 result, got %d (err: %v)", len(results), err)
	}
	if results[0].ID != file.ID {
		t.Fatalf("SearchFiles returned wrong file ID %s, expected %s", results[0].ID, file.ID)
	}

	// 6. Test share link creation
	share, err := database.CreateShareLink(file.ID, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateShareLink failed: %v", err)
	}

	loadedShare, err := database.GetShareLink(share.Token)
	if err != nil || loadedShare.FileID != file.ID {
		t.Fatalf("GetShareLink failed or returned mismatch")
	}

	// 6b. List and Delete ShareLink
	shares, err := database.ListShareLinks()
	if err != nil || len(shares) != 1 {
		t.Fatalf("ListShareLinks failed, expected 1, got %d: %v", len(shares), err)
	}
	if shares[0].FileName != "report.pdf" {
		t.Fatalf("ListShareLinks expected FileName 'report.pdf', got '%s'", shares[0].FileName)
	}
	if err := database.DeleteShareLink(share.ID); err != nil {
		t.Fatalf("DeleteShareLink failed: %v", err)
	}
	shares, err = database.ListShareLinks()
	if err != nil || len(shares) != 0 {
		t.Fatalf("Expected 0 shares after delete, got %d", len(shares))
	}

	// 7. Delete folder (cascading check)
	err = database.DeleteFolder(root.ID)
	if err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	_, err = database.GetFile(file.ID)
	// Foreign key set NULL or file remains if on delete set null
	// In schema: ON DELETE SET NULL for files(folder_id)
	if err != nil {
		t.Fatalf("File query after folder deletion failed: %v", err)
	}
}

func TestDB_VirtualTrash(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "trash_test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer database.Close()

	folder, err := database.CreateFolder("Documents", nil)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	file, err := database.CreateFile(&folder.ID, "contract.pdf", 5000, "application/pdf", 101, "f1", "h1", "sha1")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// 1. Initially visible
	files, err := database.ListFiles(&folder.ID)
	if err != nil || len(files) != 1 {
		t.Fatalf("Expected 1 active file, got %d", len(files))
	}

	// 2. Soft delete file
	if err := database.SoftDeleteFile(file.ID); err != nil {
		t.Fatalf("SoftDeleteFile failed: %v", err)
	}

	// 3. File should disappear from ListFiles and SearchFiles
	files, _ = database.ListFiles(&folder.ID)
	if len(files) != 0 {
		t.Fatalf("Expected 0 active files after soft delete, got %d", len(files))
	}
	searchResults, _ := database.SearchFiles("contract")
	if len(searchResults) != 0 {
		t.Fatalf("Expected 0 search results after soft delete, got %d", len(searchResults))
	}

	// 4. File should appear in ListTrash
	trashedFolders, trashedFiles, err := database.ListTrash()
	if err != nil || len(trashedFiles) != 1 || len(trashedFolders) != 0 {
		t.Fatalf("Expected 1 trashed file and 0 trashed folders, got %d files, %d folders", len(trashedFiles), len(trashedFolders))
	}

	// 5. Restore file
	if err := database.RestoreFile(file.ID); err != nil {
		t.Fatalf("RestoreFile failed: %v", err)
	}
	files, _ = database.ListFiles(&folder.ID)
	if len(files) != 1 {
		t.Fatalf("Expected file restored to active list, got %d", len(files))
	}

	// 6. Soft delete folder cascades to its contents
	if err := database.SoftDeleteFolder(folder.ID); err != nil {
		t.Fatalf("SoftDeleteFolder failed: %v", err)
	}
	activeFolders, _ := database.ListFolders(nil)
	if len(activeFolders) != 0 {
		t.Fatalf("Expected 0 active folders after soft delete, got %d", len(activeFolders))
	}
	files, _ = database.ListFiles(&folder.ID)
	if len(files) != 0 {
		t.Fatalf("Expected 0 active files inside trashed folder, got %d", len(files))
	}

	// 7. Empty trash
	deletedFiles, err := database.EmptyTrash()
	if err != nil || len(deletedFiles) != 1 {
		t.Fatalf("Expected 1 file permanently deleted on EmptyTrash, got %d", len(deletedFiles))
	}
	trashedFolders, trashedFiles, _ = database.ListTrash()
	if len(trashedFolders) != 0 || len(trashedFiles) != 0 {
		t.Fatalf("Expected empty trash, got %d folders, %d files", len(trashedFolders), len(trashedFiles))
	}
}

