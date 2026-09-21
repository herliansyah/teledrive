package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"teledrive/internal/crypto"
	"teledrive/internal/telegram"
)

func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FileID     string  `json:"file_id"`
		Password   *string `json:"password"`
		ExpiryDays *int    `json:"expiry_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.FileID == "" {
		http.Error(w, "invalid payload", http.StatusBadRequest)
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
	if body.ExpiryDays != nil && *body.ExpiryDays > 0 {
		exp := time.Now().Add(time.Duration(*body.ExpiryDays) * 24 * time.Hour)
		expiresAt = &exp
	}

	share, err := s.db.CreateShareLink(body.FileID, pwdHash, expiresAt, nil)
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

	file, err := s.db.GetFile(share.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	needPassword := share.PasswordHash != nil && !s.isTokenUnlocked(r, token)

	data := map[string]any{
		"Token":          token,
		"FileName":       file.Name,
		"FileSize":       file.Size,
		"FormattedSize":  formatBytes(file.Size),
		"MimeType":       file.MimeType,
		"NeedPassword":   needPassword,
		"IsVideo":        strings.HasPrefix(file.MimeType, "video/"),
		"IsAudio":        strings.HasPrefix(file.MimeType, "audio/"),
		"IsImage":        strings.HasPrefix(file.MimeType, "image/"),
		"IsPDF":          file.MimeType == "application/pdf",
		"IsText":         strings.HasPrefix(file.MimeType, "text/") || strings.HasSuffix(file.Name, ".json") || strings.HasSuffix(file.Name, ".md") || strings.HasSuffix(file.Name, ".txt"),
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
			file, _ := s.db.GetFile(share.FileID)
			_ = s.templates.ExecuteTemplate(w, "share.html", map[string]any{
				"Token":        token,
				"FileName":     file.Name,
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
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	file, err := s.db.GetFile(share.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	docID, _ := strconv.ParseInt(file.TelegramFileID, 10, 64)
	docHash, _ := strconv.ParseInt(file.TelegramAccessHash, 10, 64)

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Accept-Ranges", "bytes")

	rangeHeader := r.Header.Get("Range")
	start, end, hasRange, err := telegram.ParseRange(rangeHeader, file.Size)
	if err != nil {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if hasRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)

		if file.IsEncrypted == 1 {
			key, iv := crypto.DeriveFileStreamKeyAndIV(s.cfg.SecretKey, file.ID)
			_ = s.tg.DownloadRange(r.Context(), docID, docHash, start, end, w, s.limiter, func(data []byte, offset int64) ([]byte, error) {
				return crypto.TransformBytes(data, key, iv, offset)
			})
		} else {
			_ = s.tg.DownloadRange(r.Context(), docID, docHash, start, end, w, s.limiter)
		}
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	w.WriteHeader(http.StatusOK)

	if file.IsEncrypted == 1 {
		key, iv := crypto.DeriveFileStreamKeyAndIV(s.cfg.SecretKey, file.ID)
		dw, _ := crypto.DecryptStreamWriter(w, key, iv, 0)
		_ = s.tg.DownloadFull(r.Context(), docID, docHash, dw)
	} else {
		_ = s.tg.DownloadFull(r.Context(), docID, docHash, w)
	}
}

func (s *Server) handleShareDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareLink(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if share.PasswordHash != nil && !s.isTokenUnlocked(r, token) {
		http.Error(w, "Password required", http.StatusForbidden)
		return
	}

	file, err := s.db.GetFile(share.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = s.db.IncrementShareDownload(token)

	docID, _ := strconv.ParseInt(file.TelegramFileID, 10, 64)
	docHash, _ := strconv.ParseInt(file.TelegramAccessHash, 10, 64)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))

	if file.IsEncrypted == 1 {
		key, iv := crypto.DeriveFileStreamKeyAndIV(s.cfg.SecretKey, file.ID)
		dw, _ := crypto.DecryptStreamWriter(w, key, iv, 0)
		_ = s.tg.DownloadFull(r.Context(), docID, docHash, dw)
	} else {
		_ = s.tg.DownloadFull(r.Context(), docID, docHash, w)
	}
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

