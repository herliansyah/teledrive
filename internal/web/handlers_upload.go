package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"teledrive/internal/crypto"
	"teledrive/internal/telegram"
)

const ClientChunkSize = 5 * 1024 * 1024 // 5 MB per client HTTP chunk

func (s *Server) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string  `json:"name"`
		Size     int64   `json:"size"`
		MimeType string  `json:"mime_type"`
		FolderID *string `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.Size <= 0 {
		http.Error(w, "invalid upload init payload", http.StatusBadRequest)
		return
	}

	totalParts := int((body.Size + telegram.PartSize - 1) / telegram.PartSize)
	if totalParts == 0 {
		totalParts = 1
	}

	tgFileID := telegram.GenerateRandomID()
	session, err := s.db.CreateUploadSession(body.FolderID, body.Name, body.Size, body.MimeType, totalParts, tgFileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          session.ID,
		"chunk_size":  ClientChunkSize,
		"total_parts": totalParts,
	})
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	// Parse 10 MB max memory for chunk
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "parse multipart form error", http.StatusBadRequest)
		return
	}

	sessionID := r.FormValue("session_id")
	chunkIndex, err := strconv.Atoi(r.FormValue("chunk_index"))
	if err != nil {
		http.Error(w, "invalid chunk_index", http.StatusBadRequest)
		return
	}

	session, err := s.db.GetUploadSession(sessionID)
	if err != nil {
		http.Error(w, "upload session not found", http.StatusNotFound)
		return
	}

	file, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "chunk payload missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	chunkData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read chunk data failed", http.StatusInternalServerError)
		return
	}

	// Slices 5MB chunk into 512KB MTProto parts
	partsPerChunk := ClientChunkSize / telegram.PartSize // 10
	basePartIndex := chunkIndex * partsPerChunk

	// Derive file stream encryption key and IV using session.ID
	encKey, encIV := crypto.DeriveFileStreamKeyAndIV(s.cfg.SecretKey, session.ID)

	ctx := r.Context()
	for offset := 0; offset < len(chunkData); offset += telegram.PartSize {
		end := offset + telegram.PartSize
		if end > len(chunkData) {
			end = len(chunkData)
		}
		partSlice := chunkData[offset:end]
		currentPartIndex := basePartIndex + (offset / telegram.PartSize)

		if currentPartIndex >= session.TotalParts {
			break
		}

		// Encrypt part slice at exact byte offset in file
		fileByteOffset := int64(currentPartIndex) * int64(telegram.PartSize)
		encSlice, encErr := crypto.TransformBytes(partSlice, encKey, encIV, fileByteOffset)
		if encErr != nil {
			http.Error(w, fmt.Sprintf("encrypt part %d: %v", currentPartIndex, encErr), http.StatusInternalServerError)
			return
		}

		if err := s.tg.UploadPart(ctx, session.TelegramFileID, currentPartIndex, session.TotalParts, encSlice, s.limiter); err != nil {
			http.Error(w, fmt.Sprintf("upload MTProto part %d: %v", currentPartIndex, err), http.StatusInternalServerError)
			return
		}

		_, _ = s.db.IncrementUploadPart(session.ID)
		_ = s.limiter.Pace(ctx)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleUploadComplete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SessionID == "" {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	session, err := s.db.GetUploadSession(body.SessionID)
	if err != nil {
		http.Error(w, "upload session not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	msgID, docID, accessHash, err := s.tg.CompleteUpload(ctx, session.TelegramFileID, session.TotalParts, session.Name, session.MimeType)
	if err != nil {
		http.Error(w, fmt.Sprintf("finalize upload on Telegram: %v", err), http.StatusInternalServerError)
		return
	}

	file, err := s.db.CreateFileWithID(
		session.ID,
		session.FolderID,
		session.Name,
		session.Size,
		session.MimeType,
		msgID,
		strconv.FormatInt(docID, 10),
		strconv.FormatInt(accessHash, 10),
		"",
		1, // is_encrypted = 1
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("save file metadata: %v", err), http.StatusInternalServerError)
		return
	}

	_ = s.db.DeleteUploadSession(session.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(file)
}
