package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"teledrive/internal/app"
	"teledrive/internal/update"
)

func (s *Server) handleGetChangelog(w http.ResponseWriter, r *http.Request) {
	candidates := []string{
		"CHANGELOG.md",
		"../CHANGELOG.md",
		"../../CHANGELOG.md",
	}

	var content []byte
	var err error
	for _, path := range candidates {
		content, err = os.ReadFile(path)
		if err == nil && len(content) > 0 {
			break
		}
	}

	if len(content) == 0 {
		content, _ = contentFS.ReadFile("static/CHANGELOG.md")
	}

	if len(content) == 0 {
		content = []byte(fmt.Sprintf("# Changelog\n\nTeleDrive v%s\n\nFor release history, visit: https://github.com/herliansyah/teledrive/releases", app.Version))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version": app.Version,
		"content": string(content),
	})
}

func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	result, err := update.CheckForUpdate(nil)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":           err.Error(),
			"current_version": app.Version,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	check, err := update.CheckForUpdate(nil)
	if err != nil {
		http.Error(w, "Failed to check update: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !check.UpdateAvailable || check.Release == nil {
		http.Error(w, "No update available", http.StatusBadRequest)
		return
	}

	_, downloadURL, err := update.FindAssetForCurrentPlatform(check.Release)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := update.ApplyUpdate(downloadURL); err != nil {
		http.Error(w, "Failed to apply update: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"message":        fmt.Sprintf("TeleDrive updated to %s. Server is restarting...", check.LatestVersion),
		"target_version": check.LatestVersion,
	})

	go func() {
		time.Sleep(1 * time.Second)
		_ = update.RestartServer()
	}()
}

func (s *Server) handleTelegramStatus(w http.ResponseWriter, r *http.Request) {
	info, err := s.tg.GetAccountInfo(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err != nil || info == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authorized": false,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleTelegramDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := s.tg.Disconnect(r.Context()); err != nil {
		http.Error(w, "Failed to disconnect Telegram: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Telegram account disconnected successfully",
	})
}

