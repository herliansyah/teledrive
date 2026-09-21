package web

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"teledrive/internal/app"
	"teledrive/internal/crypto"
	"teledrive/internal/db"
)

func TestWebServer_AuthAndFolderAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db failed: %v", err)
	}
	defer database.Close()

	cfg := &app.Config{
		Port:          "8080",
		DBPath:        dbPath,
		SecretKey:     "test-secret-key-32b",
		AdminPassword: "supersecretpassword",
	}

	server, err := NewServer(cfg, database, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// 1. Unauthenticated request to / should redirect to /login
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("Expected redirect to /login, got code %d, loc %s", rec.Code, rec.Header().Get("Location"))
	}

	// 2. Submit wrong password
	form := url.Values{"password": {"wrong"}}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Invalid admin password") {
		t.Fatalf("Expected error message in login HTML, got body: %s", rec.Body.String())
	}

	// 3. Submit correct password
	form = url.Values{"password": {"supersecretpassword"}}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "teledrive_session" {
			authCookie = c
			break
		}
	}
	if authCookie == nil || !crypto.ValidateSessionToken(authCookie.Value, cfg.SecretKey) {
		t.Fatalf("Expected valid signed auth cookie to be set, got: %v", cookies)
	}

	// 3b. Forged cookie should be rejected and redirected to /login
	forgedReq := httptest.NewRequest("GET", "/", nil)
	forgedReq.AddCookie(&http.Cookie{Name: "teledrive_session", Value: "forged.value"})
	forgedRec := httptest.NewRecorder()
	server.ServeHTTP(forgedRec, forgedReq)
	if forgedRec.Code != http.StatusSeeOther {
		t.Fatalf("Expected forged cookie to be redirected with 303, got %d", forgedRec.Code)
	}

	// 4. Authenticated request to / should render dashboard HTML
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "TeleDrive") {
		t.Fatalf("Expected dashboard HTML 200 OK, got code %d", rec.Code)
	}

	// 5. Create folder via API
	folderPayload, _ := json.Marshal(map[string]any{"name": "Projects"})
	req = httptest.NewRequest("POST", "/api/folders", bytes.NewReader(folderPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for folder, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. List folders via API
	req = httptest.NewRequest("GET", "/api/folders", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Projects") {
		t.Fatalf("Expected folder list with 'Projects', got: %s", rec.Body.String())
	}

	// 7. Create a file & share link, then test GET /api/shares & DELETE /api/shares/{id}
	file, err := database.CreateFile(nil, "doc.pdf", 2048, "application/pdf", 11, "tg_1", "hash_1", "sha_1")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	sharePayload, _ := json.Marshal(map[string]any{"file_id": file.ID})
	req = httptest.NewRequest("POST", "/api/share", bytes.NewReader(sharePayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for share, got %d", rec.Code)
	}

	// GET /api/shares
	req = httptest.NewRequest("GET", "/api/shares", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "doc.pdf") {
		t.Fatalf("Expected shares list containing 'doc.pdf', got %d: %s", rec.Code, rec.Body.String())
	}

	var shares []db.ShareLinkInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &shares)
	if len(shares) == 0 {
		t.Fatalf("Expected at least 1 share in list")
	}

	// DELETE /api/shares/{id}
	req = httptest.NewRequest("DELETE", "/api/shares/"+shares[0].ID, nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Expected 204 No Content for delete share, got %d", rec.Code)
	}
}

func TestWebServer_SnapshotsAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db failed: %v", err)
	}
	defer database.Close()

	cfg := &app.Config{
		Port:          "8080",
		DBPath:        dbPath,
		SecretKey:     "test-secret-key-32b",
		AdminPassword: "supersecretpassword",
	}

	server, err := NewServer(cfg, database, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	authCookie := &http.Cookie{Name: "teledrive_session", Value: crypto.GenerateSessionToken(cfg.SecretKey, 1*time.Hour)}

	// 1. GET /api/snapshots with nil tg -> returns 200 OK and []
	req := httptest.NewRequest("GET", "/api/snapshots", nil)
	req.AddCookie(authCookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for snapshots list, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("Expected empty json array [], got %s", rec.Body.String())
	}

	// 2. POST /api/snapshots with nil tg -> returns 400 Bad Request
	req = httptest.NewRequest("POST", "/api/snapshots", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request without TG, got %d", rec.Code)
	}

	// 3. POST /api/snapshots/invalid/restore -> returns 400 Bad Request
	req = httptest.NewRequest("POST", "/api/snapshots/abc/restore", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for invalid snapshot ID, got %d", rec.Code)
	}

	// 4. Create snapshot locally and test /api/snapshots/upload-restore
	gzPath, err := database.CreateSnapshot()
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	defer os.Remove(gzPath)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("snapshot", filepath.Base(gzPath))
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	gzFile, err := os.Open(gzPath)
	if err != nil {
		t.Fatalf("Open gzPath failed: %v", err)
	}
	_, _ = io.Copy(part, gzFile)
	gzFile.Close()
	writer.Close()

	req = httptest.NewRequest("POST", "/api/snapshots/upload-restore", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for upload-restore, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "restored successfully") {
		t.Fatalf("Expected success message in response, got: %s", rec.Body.String())
	}
}

func TestWebServer_VirtualTrashAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_trash_api.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db failed: %v", err)
	}
	defer database.Close()

	cfg := &app.Config{
		SecretKey:     "test-secret-key-1234567890123456",
		AdminPassword: "admin",
	}

	server, err := NewServer(cfg, database, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	authCookie := &http.Cookie{Name: "teledrive_session", Value: crypto.GenerateSessionToken(cfg.SecretKey, 1*time.Hour)}

	// 1. Create a test file
	file, err := database.CreateFile(nil, "notes.txt", 100, "text/plain", 1, "tg_1", "hash_1", "sha_1")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// 2. Soft delete the file via standard DELETE /api/files/{id}
	delReq := httptest.NewRequest("DELETE", "/api/files/"+file.ID, nil)
	delReq.AddCookie(authCookie)
	delRec := httptest.NewRecorder()
	server.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("Expected 204 No Content for delete file, got %d", delRec.Code)
	}

	// 3. GET /api/trash should list the file
	trashReq := httptest.NewRequest("GET", "/api/trash", nil)
	trashReq.AddCookie(authCookie)
	trashRec := httptest.NewRecorder()
	server.ServeHTTP(trashRec, trashReq)
	if trashRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for trash list, got %d", trashRec.Code)
	}
	if !strings.Contains(trashRec.Body.String(), "notes.txt") {
		t.Fatalf("Expected trash to contain notes.txt, got %s", trashRec.Body.String())
	}

	// 4. Restore file via POST /api/trash/files/{id}/restore
	restoreReq := httptest.NewRequest("POST", "/api/trash/files/"+file.ID+"/restore", nil)
	restoreReq.AddCookie(authCookie)
	restoreRec := httptest.NewRecorder()
	server.ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for restore, got %d", restoreRec.Code)
	}

	// 5. GET /api/files should list notes.txt again
	listReq := httptest.NewRequest("GET", "/api/files", nil)
	listReq.AddCookie(authCookie)
	listRec := httptest.NewRecorder()
	server.ServeHTTP(listRec, listReq)
	if !strings.Contains(listRec.Body.String(), "notes.txt") {
		t.Fatalf("Expected notes.txt to be restored in active files list, got %s", listRec.Body.String())
	}

	// 6. Soft delete again and empty trash via DELETE /api/trash
	delReq = httptest.NewRequest("DELETE", "/api/files/"+file.ID, nil)
	delReq.AddCookie(authCookie)
	server.ServeHTTP(httptest.NewRecorder(), delReq)

	emptyReq := httptest.NewRequest("DELETE", "/api/trash", nil)
	emptyReq.AddCookie(authCookie)
	emptyRec := httptest.NewRecorder()
	server.ServeHTTP(emptyRec, emptyReq)
	if emptyRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for empty trash, got %d", emptyRec.Code)
	}

	// 7. Verify trash is now empty
	trashRec = httptest.NewRecorder()
	server.ServeHTTP(trashRec, trashReq)
	if strings.Contains(trashRec.Body.String(), "notes.txt") {
		t.Fatalf("Expected trash to be empty, but found notes.txt in %s", trashRec.Body.String())
	}
}

func TestWebServer_WebDAVGateway(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_webdav.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db failed: %v", err)
	}
	defer database.Close()

	cfg := &app.Config{
		SecretKey:     "test-secret-key-1234567890123456",
		AdminPassword: "adminpassword123",
	}

	server, err := NewServer(cfg, database, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// 1. Unauthenticated request to /webdav/ should return 401 Unauthorized
	req := httptest.NewRequest("PROPFIND", "/webdav/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for unauthenticated WebDAV request, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), "TeleDrive WebDAV") {
		t.Fatalf("Expected WWW-Authenticate header, got: %s", rec.Header().Get("WWW-Authenticate"))
	}

	// 2. Authenticated PROPFIND /webdav/ (Depth: 1) should return 207 Multi-Status
	req = httptest.NewRequest("PROPFIND", "/webdav/", nil)
	req.SetBasicAuth("admin", "adminpassword123")
	req.Header.Set("Depth", "1")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Expected 207 Multi-Status for root PROPFIND, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. MKCOL /webdav/Photos creates folder in SQLite
	req = httptest.NewRequest("MKCOL", "/webdav/Photos", nil)
	req.SetBasicAuth("admin", "adminpassword123")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for MKCOL, got %d: %s", rec.Code, rec.Body.String())
	}

	folder, err := database.FindFolderByName("Photos", nil)
	if err != nil || folder == nil {
		t.Fatalf("Expected folder Photos to be created in database, err: %v", err)
	}

	// 4. PUT /webdav/Photos/sample.txt uploads file
	sampleContent := "Hello from WebDAV test client!"
	req = httptest.NewRequest("PUT", "/webdav/Photos/sample.txt", strings.NewReader(sampleContent))
	req.SetBasicAuth("admin", "adminpassword123")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("Expected 201 or 204 for PUT, got %d: %s", rec.Code, rec.Body.String())
	}

	file, err := database.FindFileByName("sample.txt", &folder.ID)
	if err != nil || file == nil {
		t.Fatalf("Expected sample.txt to be found under Photos in database, err: %v", err)
	}
	if file.Size != int64(len(sampleContent)) {
		t.Fatalf("Expected file size %d, got %d", len(sampleContent), file.Size)
	}

	// 5. PROPFIND /webdav/Photos should list sample.txt
	req = httptest.NewRequest("PROPFIND", "/webdav/Photos", nil)
	req.SetBasicAuth("admin", "adminpassword123")
	req.Header.Set("Depth", "1")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Expected 207 Multi-Status for folder PROPFIND, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "sample.txt") {
		t.Fatalf("Expected sample.txt in PROPFIND response, got %s", rec.Body.String())
	}

	// 6. DELETE /webdav/Photos/sample.txt soft deletes the file into Virtual Trash
	req = httptest.NewRequest("DELETE", "/webdav/Photos/sample.txt", nil)
	req.SetBasicAuth("admin", "adminpassword123")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("Expected 204 No Content for WebDAV DELETE, got %d", rec.Code)
	}

	// 7. Verify file is in Virtual Trash and not in active list
	files, _ := database.ListFiles(&folder.ID)
	if len(files) != 0 {
		t.Fatalf("Expected 0 active files after WebDAV delete, got %d", len(files))
	}
	_, trashedFiles, err := database.ListTrash()
	if err != nil || len(trashedFiles) != 1 {
		t.Fatalf("Expected sample.txt in Virtual Trash, got %d trashed files", len(trashedFiles))
	}
}




