package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleListTrash returns all folders and files currently marked as deleted in Virtual Trash.
func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request) {
	folders, files, err := s.db.ListTrash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"folders": folders,
		"files":   files,
	})
}

// handleRestoreFolder restores a soft-deleted folder and its contents.
func (s *Server) handleRestoreFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing folder id", http.StatusBadRequest)
		return
	}

	if err := s.db.RestoreFolder(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleRestoreFile restores a soft-deleted file.
func (s *Server) handleRestoreFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing file id", http.StatusBadRequest)
		return
	}

	if err := s.db.RestoreFile(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleHardDeleteFolder permanently deletes a folder and its contents.
func (s *Server) handleHardDeleteFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing folder id", http.StatusBadRequest)
		return
	}

	if err := s.db.HardDeleteFolder(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleHardDeleteFile permanently deletes a file and its Telegram document message.
func (s *Server) handleHardDeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing file id", http.StatusBadRequest)
		return
	}

	file, err := s.db.GetFile(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// If Telegram client is active, delete the message from the Storage Channel
	if s.tg != nil && file.TelegramMessageID > 0 {
		_ = s.tg.Run(r.Context(), func(runCtx context.Context) error {
			_ = s.tg.EnsureStorageChannel(runCtx)
			return s.tg.DeleteMessages(runCtx, file.TelegramMessageID)
		})
	}

	if err := s.db.HardDeleteFile(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleEmptyTrash permanently removes all trashed items and deletes their Telegram storage messages.
func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request) {
	files, err := s.db.EmptyTrash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.tg != nil && len(files) > 0 {
		var msgIDs []int
		for _, f := range files {
			if f.TelegramMessageID > 0 {
				msgIDs = append(msgIDs, f.TelegramMessageID)
			}
		}
		if len(msgIDs) > 0 {
			_ = s.tg.Run(r.Context(), func(runCtx context.Context) error {
				_ = s.tg.EnsureStorageChannel(runCtx)
				return s.tg.DeleteMessages(runCtx, msgIDs...)
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"deleted_count": len(files),
	})
}

// StartPeriodicTrashPurge schedules daily deletion of items in Virtual Trash older than 30 days.
func (s *Server) StartPeriodicTrashPurge(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-30 * 24 * time.Hour)
				expiredFiles, err := s.db.GetExpiredTrashFiles(cutoff)
				if err == nil && len(expiredFiles) > 0 && s.tg != nil {
					var msgIDs []int
					for _, f := range expiredFiles {
						if f.TelegramMessageID > 0 {
							msgIDs = append(msgIDs, f.TelegramMessageID)
						}
					}
					if len(msgIDs) > 0 {
						_ = s.tg.Run(ctx, func(runCtx context.Context) error {
							_ = s.tg.EnsureStorageChannel(runCtx)
							return s.tg.DeleteMessages(runCtx, msgIDs...)
						})
					}
				}
				count, purgeErr := s.db.PurgeExpiredTrash(cutoff)
				if purgeErr != nil {
					fmt.Printf("[Virtual Trash Purge] Error: %v\n", purgeErr)
				} else if count > 0 {
					fmt.Printf("[Virtual Trash Purge] Purged %d expired items from trash\n", count)
				}
			}
		}
	}()
}
