package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FileID     *string `json:"file_id"`
		FolderID   *string `json:"folder_id"`
		Password   *string `json:"password"`
		ExpiryDays *int    `json:"expiry_days"`
		ExpiresAt  *string `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if body.FileID != nil && *body.FileID == "" {
		body.FileID = nil
	}
	if body.FolderID != nil && *body.FolderID == "" {
		body.FolderID = nil
	}
	if body.FileID == nil && body.FolderID == nil {
		http.Error(w, "either file_id or folder_id is required", http.StatusBadRequest)
		return
	}

	var pwdHash *string
	if body.Password != nil && *body.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "hash password failed", http.StatusInternalServerError)
			return
		}
		hStr := string(hash)
		pwdHash = &hStr
	}

	var expiresAt *time.Time
	if body.ExpiresAt != nil && *body.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			t, err = time.Parse("2006-01-02", *body.ExpiresAt)
		}
		if err == nil {
			expiresAt = &t
		}
	} else if body.ExpiryDays != nil && *body.ExpiryDays > 0 {
		exp := time.Now().Add(time.Duration(*body.ExpiryDays) * 24 * time.Hour)
		expiresAt = &exp
	}

	share, err := s.db.CreateShareLink(body.FileID, body.FolderID, pwdHash, expiresAt, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token": share.Token,
	})
}

func (s *Server) handleShareLanding(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareLink(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusGone)
		return
	}

	needPassword := share.PasswordHash != nil && !s.isTokenUnlocked(r, token)

	if share.FolderID != nil {
		folder, err := s.db.GetFolder(*share.FolderID)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		data := map[string]any{
			"Token":        token,
			"IsFolder":     true,
			"FileName":     folder.Name,
			"FolderName":   folder.Name,
			"FolderID":     folder.ID,
			"NeedPassword": needPassword,
		}
		_ = s.templates.ExecuteTemplate(w, "share.html", data)
		return
	}

	if share.FileID == nil {
		http.NotFound(w, r)
		return
	}

	file, err := s.db.GetFile(*share.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := map[string]any{
		"Token":         token,
		"IsFolder":      false,
		"FileName":      file.Name,
		"FileSize":      file.Size,
		"FormattedSize": formatBytes(file.Size),
		"MimeType":      file.MimeType,
		"NeedPassword":  needPassword,
		"IsVideo":       strings.HasPrefix(file.MimeType, "video/"),
		"IsAudio":       strings.HasPrefix(file.MimeType, "audio/"),
		"IsImage":       strings.HasPrefix(file.MimeType, "image/"),
		"IsPDF":         file.MimeType == "application/pdf",
		"IsText":        strings.HasPrefix(file.MimeType, "text/") || strings.HasSuffix(file.Name, ".json") || strings.HasSuffix(file.Name, ".md") || strings.HasSuffix(file.Name, ".txt"),
	}

	_ = s.templates.ExecuteTemplate(w, "share.html", data)
}

func (s *Server) handleShareUnlock(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareLink(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pwd := r.FormValue("password")
	if share.PasswordHash != nil {
		if err := bcrypt.CompareHashAndPassword([]byte(*share.PasswordHash), []byte(pwd)); err != nil {
			targetName := "Item"
			if share.FileID != nil {
				if file, err := s.db.GetFile(*share.FileID); err == nil {
					targetName = file.Name
				}
			} else if share.FolderID != nil {
				if folder, err := s.db.GetFolder(*share.FolderID); err == nil {
					targetName = folder.Name
				}
			}
			_ = s.templates.ExecuteTemplate(w, "share.html", map[string]any{
				"Token":        token,
				"FileName":     targetName,
				"IsFolder":     share.FolderID != nil,
				"NeedPassword": true,
				"Error":        "Incorrect password",
			})
			return
		}
	}

	s.setTokenUnlocked(w, token)
	http.Redirect(w, r, fmt.Sprintf("/s/%s", token), http.StatusSeeOther)
}

func (s *Server) handleShareStream(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareLink(token)
	if err != nil || share.FileID == nil {
		http.NotFound(w, r)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	file, err := s.db.GetFile(*share.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	s.serveFileStream(w, r, file)
}

func (s *Server) handleShareDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareLink(token)
	if err != nil || share.FileID == nil {
		http.NotFound(w, r)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	file, err := s.db.GetFile(*share.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = s.db.IncrementShareDownload(token)
	s.serveFileDownload(w, r, file)
}

func (s *Server) handleShareFolderContents(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareLink(token)
	if err != nil || share.FolderID == nil {
		http.NotFound(w, r)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	rootFolderID := *share.FolderID
	targetFolderID := rootFolderID

	reqFolderID := r.URL.Query().Get("folder_id")
	if reqFolderID != "" && reqFolderID != rootFolderID {
		isDescendant, err := s.db.IsFolderInFolderHierarchy(reqFolderID, rootFolderID)
		if err != nil || !isDescendant {
			http.Error(w, "Access denied outside shared folder", http.StatusForbidden)
			return
		}
		targetFolderID = reqFolderID
	}

	folders, err := s.db.ListFolders(&targetFolderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := s.db.ListFiles(&targetFolderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build breadcrumbs path relative to shared root
	type crumb struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var breadcrumbs []crumb
	currID := &targetFolderID
	for currID != nil {
		f, err := s.db.GetFolder(*currID)
		if err != nil {
			break
		}
		breadcrumbs = append([]crumb{{ID: f.ID, Name: f.Name}}, breadcrumbs...)
		if f.ID == rootFolderID {
			break
		}
		currID = f.ParentID
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"root_id":     rootFolderID,
		"folder_id":   targetFolderID,
		"folders":     folders,
		"files":       files,
		"breadcrumbs": breadcrumbs,
	})
}

func (s *Server) handleShareFolderFileStream(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	fileID := r.PathValue("id")
	share, err := s.db.GetShareLink(token)
	if err != nil || share.FolderID == nil {
		http.NotFound(w, r)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	valid, err := s.db.IsFileInFolderHierarchy(fileID, *share.FolderID)
	if err != nil || !valid {
		http.Error(w, "File not in shared folder", http.StatusForbidden)
		return
	}

	file, err := s.db.GetFile(fileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	s.serveFileStream(w, r, file)
}

func (s *Server) handleShareFolderFileDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	fileID := r.PathValue("id")
	share, err := s.db.GetShareLink(token)
	if err != nil || share.FolderID == nil {
		http.NotFound(w, r)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	valid, err := s.db.IsFileInFolderHierarchy(fileID, *share.FolderID)
	if err != nil || !valid {
		http.Error(w, "File not in shared folder", http.StatusForbidden)
		return
	}

	file, err := s.db.GetFile(fileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = s.db.IncrementShareDownload(token)
	s.serveFileDownload(w, r, file)
}

func (s *Server) isTokenUnlocked(r *http.Request, token string) bool {
	cookie, err := r.Cookie(fmt.Sprintf("td_unlocked_%s", token))
	return err == nil && cookie.Value == "1"
}

func (s *Server) setTokenUnlocked(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     fmt.Sprintf("td_unlocked_%s", token),
		Value:    "1",
		Path:     fmt.Sprintf("/s/%s", token),
		HttpOnly: true,
		MaxAge:   86400, // 24 hours
	})
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (s *Server) handleListShares(w http.ResponseWriter, r *http.Request) {
	shares, err := s.db.ListShareLinks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(shares)
}

func (s *Server) handleDeleteShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing share id", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteShareLink(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
