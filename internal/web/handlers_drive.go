package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"teledrive/internal/crypto"
	"teledrive/internal/db"
	"teledrive/internal/telegram"
)

func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	var parentID *string
	if p := r.URL.Query().Get("folder_id"); p != "" {
		parentID = &p
	}

	folders, err := s.db.ListFolders(parentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(folders)
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, "invalid folder data", http.StatusBadRequest)
		return
	}

	folder, err := s.db.CreateFolder(body.Name, body.ParentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(folder)
}

func (s *Server) handleUpdateFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name     *string `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if body.Name != nil && *body.Name != "" {
		if err := s.db.RenameFolder(id, *body.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if body.ParentID != nil {
		if err := s.db.MoveFolder(id, body.ParentID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.db.DeleteFolder(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	if search != "" {
		files, err := s.db.SearchFiles(search)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)
		return
	}

	var folderID *string
	if f := r.URL.Query().Get("folder_id"); f != "" {
		folderID = &f
	}

	files, err := s.db.ListFiles(folderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

func (s *Server) handleUpdateFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name     *string `json:"name"`
		FolderID *string `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if body.Name != nil && *body.Name != "" {
		if err := s.db.RenameFile(id, *body.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if body.FolderID != nil {
		if err := s.db.MoveFile(id, body.FolderID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.db.DeleteFile(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFileStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	file, err := s.db.GetFile(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	s.serveFileStream(w, r, file)
}

func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	file, err := s.db.GetFile(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	s.serveFileDownload(w, r, file)
}

func (s *Server) serveFileStream(w http.ResponseWriter, r *http.Request, file *db.File) {
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

func (s *Server) serveFileDownload(w http.ResponseWriter, r *http.Request, file *db.File) {
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

func (s *Server) handleBatchTrash(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FileIDs   []string `json:"file_ids"`
		FolderIDs []string `json:"folder_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := s.db.BatchTrash(body.FileIDs, body.FolderIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleBatchMove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FileIDs        []string `json:"file_ids"`
		FolderIDs      []string `json:"folder_ids"`
		TargetFolderID *string  `json:"target_folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := s.db.BatchMove(body.FileIDs, body.FolderIDs, body.TargetFolderID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

