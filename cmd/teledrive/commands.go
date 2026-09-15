package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"teledrive/internal/app"
	"teledrive/internal/telegram"
	"teledrive/internal/web"
)

func runLogin(cfg *app.Config) {
	fmt.Println("=== TeleDrive Telegram MTProto Authentication Wizard ===")
	database := openDatabase(cfg)
	defer database.Close()

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n[⚠️  DISCLAIMER & USAGE NOTICE]")
	fmt.Println("• Disarankan menggunakan akun Telegram SEKUNDER/CADANGAN, bukan akun utama Anda.")
	fmt.Println("• Segala risiko penggunaan (limitasi ToS, banned Telegram, kehilangan data) sepenuhnya adalah tanggung jawab pribadi pengguna.")
	fmt.Println("• Strongly recommended to use a dedicated secondary account. All risks are your personal responsibility.")
	fmt.Print("\nApakah Anda memahami & menyetujui disclaimer ini? / Do you agree? [y/N]: ")
	consent, _ := reader.ReadString('\n')
	if consent = strings.TrimSpace(strings.ToLower(consent)); consent != "y" && consent != "yes" {
		fmt.Println("Login dibatalkan / Login aborted. Persetujuan disclaimer diperlukan.")
		return
	}
	fmt.Println()

	// Check DB if not in config/env
	if appID == 0 {
		if stored, err := database.GetSetting("telegram_app_id"); err == nil && stored != "" {
			appID, _ = strconv.Atoi(stored)
		}
	}
	if appHash == "" {
		if stored, err := database.GetSetting("telegram_app_hash"); err == nil && stored != "" {
			appHash = stored
		}
	}

	if appID == 0 {
		fmt.Print("Enter your Telegram App ID (from https://my.telegram.org): ")
		val, _ := reader.ReadString('\n')
		appID, _ = strconv.Atoi(strings.TrimSpace(val))
		_ = database.SetSetting("telegram_app_id", strconv.Itoa(appID))
	}

	if appHash == "" {
		fmt.Print("Enter your Telegram App Hash (from https://my.telegram.org): ")
		val, _ := reader.ReadString('\n')
		appHash = strings.TrimSpace(val)
		_ = database.SetSetting("telegram_app_hash", appHash)
	}

	if appID == 0 || appHash == "" {
		fmt.Println("Error: Valid Telegram App ID and App Hash are required.")
		return
	}

	mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
	if cfg.StorageChannelID != 0 {
		mgr.SetConfiguredChannelID(cfg.StorageChannelID)
	}

	ctx := context.Background()
	err := mgr.Run(ctx, func(runCtx context.Context) error {
		return mgr.AuthenticateInteractive(runCtx, reader, cfg.DBPath)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login error: %v\n", err)
		return
	}

	fmt.Println("TeleDrive is successfully paired with Telegram! You can now run `teledrive server`.")
}

func runServer(cfg *app.Config) {
	database := openDatabase(cfg)
	defer database.Close()

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash
	if appID == 0 {
		if stored, err := database.GetSetting("telegram_app_id"); err == nil {
			appID, _ = strconv.Atoi(stored)
		}
	}
	if appHash == "" {
		if stored, err := database.GetSetting("telegram_app_hash"); err == nil {
			appHash = stored
		}
	}

	if appID == 0 || appHash == "" {
		fmt.Println("Telegram credentials not configured. Please run `teledrive login` first.")
		return
	}

	mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
	if cfg.StorageChannelID != 0 {
		mgr.SetConfiguredChannelID(cfg.StorageChannelID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start MTProto client in background
	go func() {
		err := mgr.Run(ctx, func(runCtx context.Context) error {
			_ = mgr.EnsureStorageChannel(runCtx)
			<-runCtx.Done()
			return nil
		})
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "MTProto background runner stopped: %v\n", err)
		}
	}()

	startPort, _ := strconv.Atoi(cfg.Port)
	if startPort <= 0 {
		startPort = 8080
	}

	listener, boundPort, err := web.FindAvailableListener(cfg.Host, startPort, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network error: %v\n", err)
		return
	}
	defer listener.Close()

	if boundPort != startPort {
		fmt.Printf("\n⚠️ Port %d is already in use. Automatically switched to available port: %d\n", startPort, boundPort)
	}
	cfg.Port = strconv.Itoa(boundPort)

	srv, err := web.NewServer(cfg, database, mgr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize web server: %v\n", err)
		return
	}

	// Start background periodic snapshot scheduler
	srv.StartPeriodicBackup(ctx)

	httpServer := &http.Server{
		Handler: srv,
	}

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal. Creating final database snapshot & shutting down...")
		shutdownCtx, sCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer sCancel()
		_ = srv.PerformAutomatedSnapshot(shutdownCtx)
		_ = httpServer.Shutdown(shutdownCtx)
		cancel()
	}()

	fmt.Printf("\n🚀 TeleDrive Web Dashboard is running:\n")
	fmt.Printf("   > Local:   http://localhost:%d\n", boundPort)
	for _, ip := range web.GetLocalIPs() {
		fmt.Printf("   > Network: http://%s:%d\n", ip, boundPort)
	}
	fmt.Printf("\n🔒 Storage Mode: Telegram MTProto Safe Mode (Primary Account Protected)\n")
	fmt.Printf("🔑 Default Admin Password: %s\n\n", cfg.AdminPassword)

	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
	}
}

func runUpload(cfg *app.Config, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: teledrive upload <filepath> [--folder <folder_id>]")
		return
	}

	filePath := args[0]
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error accessing file %s: %v\n", filePath, err)
		return
	}

	var folderID *string
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "--folder" {
			val := args[i+1]
			folderID = &val
		}
	}

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer f.Close()

	database := openDatabase(cfg)
	defer database.Close()

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash
	if appID == 0 {
		if stored, err := database.GetSetting("telegram_app_id"); err == nil {
			appID, _ = strconv.Atoi(stored)
		}
	}
	if appHash == "" {
		if stored, err := database.GetSetting("telegram_app_hash"); err == nil {
			appHash = stored
		}
	}

	if appID == 0 || appHash == "" {
		fmt.Println("Telegram credentials not configured. Please run `teledrive login` first.")
		return
	}

	mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
	if cfg.StorageChannelID != 0 {
		mgr.SetConfiguredChannelID(cfg.StorageChannelID)
	}
	limiter := telegram.NewSafeLimiter()

	fileName := filepath.Base(filePath)
	mimeType := detectMimeType(fileName)
	size := fileInfo.Size()

	fmt.Printf("Uploading %s (%.2f MB)...\n", fileName, float64(size)/(1024*1024))

	ctx := context.Background()
	err = mgr.Run(ctx, func(runCtx context.Context) error {
		if err := mgr.EnsureStorageChannel(runCtx); err != nil {
			return err
		}

		msgID, docID, accessHash, shaHex, err := mgr.UploadFromReader(runCtx, f, size, fileName, mimeType, limiter, func(uploaded, total int64) {
			pct := float64(uploaded) / float64(total) * 100
			fmt.Printf("\rProgress: %.1f%% (%d / %d bytes)", pct, uploaded, total)
		})
		if err != nil {
			return err
		}

		fmt.Println("\nFinalizing database record...")
		_, err = database.CreateFile(folderID, fileName, size, mimeType, msgID, strconv.FormatInt(docID, 10), strconv.FormatInt(accessHash, 10), shaHex)
		return err
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nUpload failed: %v\n", err)
		return
	}

	fmt.Println("✓ Upload complete! File safely stored in Telegram Storage Channel.")
}

func detectMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar", ".gz":
		return "application/gzip"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func runDownload(cfg *app.Config, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: teledrive download <file_id> [--output <path>]")
		return
	}

	fileID := args[0]
	database := openDatabase(cfg)
	defer database.Close()

	fileRecord, err := database.GetFile(fileID)
	if err != nil {
		fmt.Printf("Error: File not found with ID %s\n", fileID)
		return
	}

	outputPath := fileRecord.Name
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "--output" {
			outputPath = args[i+1]
		}
	}

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash
	if appID == 0 {
		if stored, err := database.GetSetting("telegram_app_id"); err == nil {
			appID, _ = strconv.Atoi(stored)
		}
	}
	if appHash == "" {
		if stored, err := database.GetSetting("telegram_app_hash"); err == nil {
			appHash = stored
		}
	}

	if appID == 0 || appHash == "" {
		fmt.Println("Telegram credentials not configured. Please run `teledrive login` first.")
		return
	}

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("Error creating output file %s: %v\n", outputPath, err)
		return
	}
	defer out.Close()

	docID, _ := strconv.ParseInt(fileRecord.TelegramFileID, 10, 64)
	docHash, _ := strconv.ParseInt(fileRecord.TelegramAccessHash, 10, 64)

	mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
	if cfg.StorageChannelID != 0 {
		mgr.SetConfiguredChannelID(cfg.StorageChannelID)
	}

	fmt.Printf("Downloading %s (%.2f MB) to %s...\n", fileRecord.Name, float64(fileRecord.Size)/(1024*1024), outputPath)

	ctx := context.Background()
	err = mgr.Run(ctx, func(runCtx context.Context) error {
		return mgr.DownloadFull(runCtx, docID, docHash, out)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Download complete: %s\n", outputPath)
}

func runList(cfg *app.Config, args []string) {
	database := openDatabase(cfg)
	defer database.Close()

	folders, err := database.ListFolders(nil)
	if err != nil {
		fmt.Printf("Error listing folders: %v\n", err)
		return
	}
	files, err := database.ListFiles(nil)
	if err != nil {
		fmt.Printf("Error listing files: %v\n", err)
		return
	}

	fmt.Println("Root Folders:")
	for _, f := range folders {
		fmt.Printf("  📁 %s (id: %s)\n", f.Name, f.ID)
	}
	fmt.Println("Root Files:")
	for _, f := range files {
		fmt.Printf("  📄 %s (%d bytes, id: %s)\n", f.Name, f.Size, f.ID)
	}
}

func runBackup(cfg *app.Config) {
	fmt.Println("Exporting SQLite snapshot to Telegram Storage Channel...")
	database := openDatabase(cfg)

	gzPath, err := database.CreateSnapshot()
	if err != nil {
		database.Close()
		fmt.Printf("Failed to create snapshot: %v\n", err)
		return
	}
	defer os.Remove(gzPath)

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash
	if appID == 0 {
		if stored, err := database.GetSetting("telegram_app_id"); err == nil {
			appID, _ = strconv.Atoi(stored)
		}
	}
	if appHash == "" {
		if stored, err := database.GetSetting("telegram_app_hash"); err == nil {
			appHash = stored
		}
	}

	database.Close()

	if appID == 0 || appHash == "" {
		fmt.Println("Telegram credentials not configured. Please run `teledrive login` first.")
		return
	}

	database = openDatabase(cfg)
	defer database.Close()

	mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
	if cfg.StorageChannelID != 0 {
		mgr.SetConfiguredChannelID(cfg.StorageChannelID)
	}
	limiter := telegram.NewSafeLimiter()

	ctx := context.Background()
	err = mgr.Run(ctx, func(runCtx context.Context) error {
		if err := mgr.EnsureStorageChannel(runCtx); err != nil {
			return err
		}
		msgID, err := mgr.UploadSnapshot(runCtx, gzPath, limiter)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Backup snapshot successfully uploaded & pinned (Message ID: %d)\n", msgID)
		return nil
	})

	if err != nil {
		fmt.Printf("Backup failed: %v\n", err)
	}
}

func runRestore(cfg *app.Config) {
	fmt.Printf("Restoring SQLite snapshot to %s from Telegram...\n", cfg.DBPath)

	database := openDatabase(cfg)

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash
	if appID == 0 {
		if stored, err := database.GetSetting("telegram_app_id"); err == nil {
			appID, _ = strconv.Atoi(stored)
		}
	}
	if appHash == "" {
		if stored, err := database.GetSetting("telegram_app_hash"); err == nil {
			appHash = stored
		}
	}

	reader := bufio.NewReader(os.Stdin)
	if appID == 0 || appHash == "" {
		fmt.Print("Enter your Telegram App ID: ")
		val, _ := reader.ReadString('\n')
		appID, _ = strconv.Atoi(strings.TrimSpace(val))
		fmt.Print("Enter your Telegram App Hash: ")
		val, _ = reader.ReadString('\n')
		appHash = strings.TrimSpace(val)
	}

	mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
	if cfg.StorageChannelID != 0 {
		mgr.SetConfiguredChannelID(cfg.StorageChannelID)
	}

	ctx := context.Background()
	err := mgr.Run(ctx, func(runCtx context.Context) error {
		storedID, _ := database.GetSetting("storage_channel_id")
		storedHash, _ := database.GetSetting("storage_channel_hash")

		if storedID != "" && storedHash != "" {
			cID, _ := strconv.ParseInt(storedID, 10, 64)
			cHash, _ := strconv.ParseInt(storedHash, 10, 64)
			mgr.SetStorageChannel(cID, cHash)
		} else if cfg.StorageChannelID != 0 {
			hash, err := mgr.ResolveChannelByID(runCtx, cfg.StorageChannelID)
			if err != nil {
				return fmt.Errorf("could not resolve configured channel ID %d: %w", cfg.StorageChannelID, err)
			}
			mgr.SetStorageChannel(cfg.StorageChannelID, hash)
		} else {
			fmt.Println("Searching for Storage Channel 'TeleDrive Vault' on Telegram...")
			candidates, err := mgr.DiscoverStorageChannels(runCtx)
			if err != nil || len(candidates) == 0 {
				return fmt.Errorf("no Storage Channel 'TeleDrive Vault' found in your Telegram account to restore from")
			}
			var chosen *telegram.DiscoveredChannel
			if len(candidates) == 1 {
				chosen = &candidates[0]
			} else {
				fmt.Printf("\nFound %d Storage Channels:\n", len(candidates))
				for i, c := range candidates {
					snapText := "no snapshots"
					if c.SnapshotCount > 0 {
						snapText = fmt.Sprintf("%d snapshot(s), latest: %s", c.SnapshotCount, c.LatestSnapshot.CreatedAt.Format("2006-01-02 15:04"))
					}
					fmt.Printf("  [%d] Channel ID: %d | Created: %s | %s\n", i+1, c.ID, c.CreatedAt.Format("2006-01-02"), snapText)
				}
				fmt.Printf("Select channel to restore from (1-%d): ", len(candidates))
				choiceStr, _ := reader.ReadString('\n')
				idx, _ := strconv.Atoi(strings.TrimSpace(choiceStr))
				if idx < 1 || idx > len(candidates) {
					idx = 1
				}
				chosen = &candidates[idx-1]
			}
			mgr.SetStorageChannel(chosen.ID, chosen.AccessHash)
		}

		// Close database connection before overwriting file
		database.Close()
		return mgr.RestoreLatestSnapshot(runCtx, cfg.DBPath)
	})

	if err != nil {
		fmt.Printf("Restore failed: %v\n", err)
		return
	}

	fmt.Println("✓ Database successfully restored from Telegram Storage Channel!")
}
