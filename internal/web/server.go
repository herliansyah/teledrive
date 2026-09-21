package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"
	"teledrive/internal/app"
	"teledrive/internal/crypto"
	"teledrive/internal/db"
	"teledrive/internal/telegram"
)

//go:embed templates/* static/*
var contentFS embed.FS

type Server struct {
	cfg       *app.Config
	db        *db.DB
	tg        *telegram.ClientManager
	limiter   *telegram.SafeLimiter
	mux       *http.ServeMux
	templates *template.Template

	// Concurrency safety for hot-restore and snapshot operations
	dbMu       sync.RWMutex
	snapshotMu sync.Mutex

	// In-memory unlock cache for password-protected share tokens
	unlockedMu     sync.RWMutex
	unlockedTokens map[string]time.Time
}

func NewServer(cfg *app.Config, database *db.DB, tgManager *telegram.ClientManager) (*Server, error) {
	tmpl, err := template.ParseFS(contentFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse embedded templates: %w", err)
	}

	s := &Server{
		cfg:            cfg,
		db:             database,
		tg:             tgManager,
		limiter:        telegram.NewSafeLimiter(),
		mux:            http.NewServeMux(),
		templates:      tmpl,
		unlockedTokens: make(map[string]time.Time),
	}

	s.routes()
	return s, nil
}

func (s *Server) routes() {
	// Embedded static assets
	staticFS, _ := fs.Sub(contentFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Authentication
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLoginSubmit)
	s.mux.HandleFunc("GET /logout", s.handleLogout)

	// Protected Dashboard (exact root match)
	s.mux.HandleFunc("GET /{$}", s.authMiddleware(s.handleDashboard))

	// Protected Drive API
	s.mux.HandleFunc("GET /api/folders", s.authMiddleware(s.handleListFolders))
	s.mux.HandleFunc("POST /api/folders", s.authMiddleware(s.handleCreateFolder))
	s.mux.HandleFunc("PUT /api/folders/{id}", s.authMiddleware(s.handleUpdateFolder))
	s.mux.HandleFunc("DELETE /api/folders/{id}", s.authMiddleware(s.handleDeleteFolder))

	s.mux.HandleFunc("GET /api/files", s.authMiddleware(s.handleListFiles))
	s.mux.HandleFunc("PUT /api/files/{id}", s.authMiddleware(s.handleUpdateFile))
	s.mux.HandleFunc("DELETE /api/files/{id}", s.authMiddleware(s.handleDeleteFile))
	s.mux.HandleFunc("GET /api/files/{id}/stream", s.authMiddleware(s.handleFileStream))
	s.mux.HandleFunc("GET /api/files/{id}/download", s.authMiddleware(s.handleFileDownload))

	// Resumable Chunked Upload API
	s.mux.HandleFunc("POST /api/upload/init", s.authMiddleware(s.handleUploadInit))
	s.mux.HandleFunc("POST /api/upload/chunk", s.authMiddleware(s.handleUploadChunk))
	s.mux.HandleFunc("POST /api/upload/complete", s.authMiddleware(s.handleUploadComplete))

	// Share Links
	s.mux.HandleFunc("POST /api/share", s.authMiddleware(s.handleCreateShare))
	s.mux.HandleFunc("GET /api/shares", s.authMiddleware(s.handleListShares))
	s.mux.HandleFunc("DELETE /api/shares/{id}", s.authMiddleware(s.handleDeleteShare))
	s.mux.HandleFunc("GET /s/{token}", s.handleShareLanding)
	s.mux.HandleFunc("POST /s/{token}/unlock", s.handleShareUnlock)
	s.mux.HandleFunc("GET /s/{token}/stream", s.handleShareStream)
	s.mux.HandleFunc("GET /s/{token}/download", s.handleShareDownload)

	// Database Snapshots & Point-in-Time Recovery
	s.mux.HandleFunc("GET /api/snapshots", s.authMiddleware(s.handleListSnapshots))
	s.mux.HandleFunc("POST /api/snapshots", s.authMiddleware(s.handleCreateSnapshot))
	s.mux.HandleFunc("POST /api/snapshots/{id}/restore", s.authMiddleware(s.handleRestoreSnapshot))
	s.mux.HandleFunc("GET /api/snapshots/{id}/download", s.authMiddleware(s.handleDownloadSnapshot))
	s.mux.HandleFunc("DELETE /api/snapshots/{id}", s.authMiddleware(s.handleDeleteSnapshot))
	s.mux.HandleFunc("POST /api/snapshots/upload-restore", s.authMiddleware(s.handleUploadRestoreSnapshot))

	// Virtual Trash API
	s.mux.HandleFunc("GET /api/trash", s.authMiddleware(s.handleListTrash))
	s.mux.HandleFunc("POST /api/trash/folders/{id}/restore", s.authMiddleware(s.handleRestoreFolder))
	s.mux.HandleFunc("POST /api/trash/files/{id}/restore", s.authMiddleware(s.handleRestoreFile))
	s.mux.HandleFunc("DELETE /api/trash/folders/{id}", s.authMiddleware(s.handleHardDeleteFolder))
	s.mux.HandleFunc("DELETE /api/trash/files/{id}", s.authMiddleware(s.handleHardDeleteFile))
	s.mux.HandleFunc("DELETE /api/trash", s.authMiddleware(s.handleEmptyTrash))

	// Root OPTIONS for Windows WebClient discovery
	s.mux.HandleFunc("OPTIONS /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("DAV", "1, 2")
		w.Header().Set("MS-Author-Via", "DAV")
		w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND")
		w.WriteHeader(http.StatusOK)
	})

	// WebDAV Gateway
	wdHandler := &webdav.Handler{
		Prefix:     "/webdav",
		FileSystem: s.newWebDAVFS(),
		LockSystem: webdav.NewMemLS(),
	}
	s.mux.HandleFunc("/webdav/", s.webdavAuthMiddleware(wdHandler.ServeHTTP))
	s.mux.HandleFunc("/webdav", s.webdavAuthMiddleware(wdHandler.ServeHTTP))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Support Windows WebClient DavWWWRoot syntax
	if strings.HasPrefix(r.URL.Path, "/DavWWWRoot") {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/DavWWWRoot")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) webdavAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("DAV", "1, 2")
			w.Header().Set("MS-Author-Via", "DAV")
			next(w, r)
			return
		}
		_, password, ok := r.BasicAuth()
		if !ok || password != s.cfg.AdminPassword {
			w.Header().Set("WWW-Authenticate", `Basic realm="TeleDrive WebDAV"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("teledrive_session")
		if err != nil || !crypto.ValidateSessionToken(cookie.Value, s.cfg.SecretKey) {
			if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || r.URL.Path != "/" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	_ = s.templates.ExecuteTemplate(w, "login.html", nil)
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	pwd := r.FormValue("password")
	if pwd == s.cfg.AdminPassword {
		token := crypto.GenerateSessionToken(s.cfg.SecretKey, 30*24*time.Hour)
		http.SetCookie(w, &http.Cookie{
			Name:     "teledrive_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400 * 30, // 30 days
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	_ = s.templates.ExecuteTemplate(w, "login.html", map[string]any{
		"Error": "Invalid admin password",
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "teledrive_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	_ = s.templates.ExecuteTemplate(w, "index.html", nil)
}

// StartPeriodicBackup starts a background scheduler that creates and uploads
// a Database Snapshot every 24 hours (or configured interval) with automatic rolling retention.
func (s *Server) StartPeriodicBackup(ctx context.Context) {
	interval := 24 * time.Hour
	if envInterval := os.Getenv("TELEDRIVE_BACKUP_INTERVAL"); envInterval != "" {
		if d, err := time.ParseDuration(envInterval); err == nil && d >= time.Minute {
			interval = d
		}
	}

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if s.tg == nil {
					continue
				}
				if err := s.PerformAutomatedSnapshot(ctx); err != nil {
					fmt.Printf("[Periodic Backup] Warning: automated snapshot failed: %v\n", err)
				} else {
					fmt.Println("[Periodic Backup] Automated snapshot successfully created and retention applied")
				}
			}
		}
	}()
}

// PerformAutomatedSnapshot creates a point-in-time snapshot, uploads it to Telegram, and prunes older snapshots.
func (s *Server) PerformAutomatedSnapshot(ctx context.Context) error {
	if s.tg == nil {
		return fmt.Errorf("telegram client manager is not initialized")
	}

	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()

	s.dbMu.RLock()
	gzPath, err := s.db.CreateSnapshot()
	s.dbMu.RUnlock()
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	defer os.Remove(gzPath)

	return s.tg.Run(ctx, func(runCtx context.Context) error {
		if err := s.tg.EnsureStorageChannel(runCtx); err != nil {
			return err
		}
		_, err := s.tg.UploadSnapshot(runCtx, gzPath, s.limiter)
		return err
	})
}

