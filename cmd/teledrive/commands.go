package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"teledrive/internal/app"
	"teledrive/internal/crypto"
	"teledrive/internal/db"
	"teledrive/internal/telegram"
	"teledrive/internal/update"
	"teledrive/internal/web"
)

func runLogin(cfg *app.Config, args []string) {
	fmt.Println("=== TeleDrive Telegram MTProto Authentication Wizard ===")
	forceReauth := false
	for _, arg := range args {
		if arg == "--force" || arg == "-f" || arg == "--reauth" {
			forceReauth = true
		}
	}

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
		return mgr.AuthenticateInteractive(runCtx, reader, cfg.DBPath, forceReauth)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login error: %v\n", err)
		return
	}

	fmt.Println("TeleDrive is successfully paired with Telegram! You can now run `teledrive server`.")
}

func runLogout(cfg *app.Config, args []string) {
	fmt.Println("=== TeleDrive Telegram MTProto Disconnect & Logout ===")
	clean := false
	for _, arg := range args {
		if arg == "--clean" || arg == "-c" {
			clean = true
		}
	}

	reader := bufio.NewReader(os.Stdin)
	if clean {
		fmt.Println("\n⚠️  WARNING: You specified the --clean flag.")
		fmt.Println("This will revoke the MTProto session AND permanently delete all virtual folders,")
		fmt.Println("files, share links, and upload sessions from your local SQLite database.")
		fmt.Print("Are you absolutely sure you want to proceed? [y/N]: ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) != "y" && strings.ToLower(strings.TrimSpace(ans)) != "yes" {
			fmt.Println("Logout canceled.")
			return
		}
	}

	database := openDatabase(cfg)
	defer database.Close()

	appID := cfg.TelegramAppID
	appHash := cfg.TelegramAppHash
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

	if appID != 0 && appHash != "" {
		mgr := telegram.NewClientManager(database, appID, appHash, cfg.SecretKey)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = mgr.Run(ctx, func(runCtx context.Context) error {
			return mgr.Disconnect(runCtx)
		})
	}

	// Guarantee local cleanup even if offline or network timeout
	_ = database.DeleteSetting("telegram_session")
	_ = database.DeleteSetting("storage_channel_id")
	_ = database.DeleteSetting("storage_channel_hash")

	if clean {
		if err := database.PurgeAllData(); err != nil {
			fmt.Fprintf(os.Stderr, "Error purging database data: %v\n", err)
		} else {
			fmt.Println("✓ All virtual files, folders, and metadata have been purged from local database.")
		}
	} else {
		fmt.Println("✓ Local virtual files and folder structures have been preserved.")
	}

	fmt.Println("✓ Telegram MTProto session successfully disconnected.")
	fmt.Println("  Run `teledrive login` when you are ready to connect a Telegram account.")
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

	// Start background periodic snapshot and trash purge schedulers
	srv.StartPeriodicBackup(ctx)
	srv.StartPeriodicTrashPurge(ctx)

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
		fmt.Println("Usage: teledrive upload <filepath_or_dir> [--folder <folder_id>]")
		return
	}

	targetPath := args[0]
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		fmt.Printf("Error accessing path %s: %v\n", targetPath, err)
		return
	}

	var folderID *string
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "--folder" {
			val := args[i+1]
			folderID = &val
		}
	}

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

	if fileInfo.IsDir() {
		runUploadDir(cfg, database, mgr, limiter, targetPath, folderID)
		return
	}

	uploadSingleFile(cfg, database, mgr, limiter, targetPath, folderID)
}

func getOrCreateFolder(database *db.DB, name string, parentID *string) (string, error) {
	existing, err := database.ListFolders(parentID)
	if err == nil {
		for _, f := range existing {
			if f.Name == name {
				return f.ID, nil
			}
		}
	}
	newFolder, err := database.CreateFolder(name, parentID)
	if err != nil {
		return "", err
	}
	return newFolder.ID, nil
}

func runUploadDir(cfg *app.Config, database *db.DB, mgr *telegram.ClientManager, limiter *telegram.SafeLimiter, dirPath string, rootFolderID *string) {
	cleanDir := filepath.Clean(dirPath)
	baseDir := filepath.Base(cleanDir)
	fmt.Printf("Uploading directory %q and all subfolders...\n", baseDir)

	folderMap := make(map[string]string)
	topID, err := getOrCreateFolder(database, baseDir, rootFolderID)
	if err != nil {
		fmt.Printf("Error creating root virtual folder %s: %v\n", baseDir, err)
		return
	}
	folderMap["."] = topID

	count := 0
	err = filepath.WalkDir(cleanDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(cleanDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			parentRel := filepath.Dir(rel)
			parentID := folderMap[parentRel]
			fID, err := getOrCreateFolder(database, d.Name(), &parentID)
			if err != nil {
				return err
			}
			folderMap[rel] = fID
			return nil
		}

		parentRel := filepath.Dir(rel)
		targetFolderID := folderMap[parentRel]
		uploadSingleFile(cfg, database, mgr, limiter, path, &targetFolderID)
		count++
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nDirectory upload error: %v\n", err)
	} else {
		fmt.Printf("\n✓ Directory upload complete! (%d files processed)\n", count)
	}
}

func uploadSingleFile(cfg *app.Config, database *db.DB, mgr *telegram.ClientManager, limiter *telegram.SafeLimiter, filePath string, folderID *string) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error accessing file %s: %v\n", filePath, err)
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer f.Close()

	fileName := filepath.Base(filePath)
	mimeType := detectMimeType(fileName)
	size := fileInfo.Size()

	fileUID := db.GenerateID()
	encKey, encIV := crypto.DeriveFileStreamKeyAndIV(cfg.SecretKey, fileUID)
	encReader, encErr := crypto.EncryptStream(f, encKey, encIV, 0)
	if encErr != nil {
		fmt.Fprintf(os.Stderr, "Encryption initialization failed: %v\n", encErr)
		return
	}

	fmt.Printf("Uploading %s (%.2f MB) [Zero-Knowledge Encrypted]...\n", fileName, float64(size)/(1024*1024))

	ctx := context.Background()
	err = mgr.Run(ctx, func(runCtx context.Context) error {
		if err := mgr.EnsureStorageChannel(runCtx); err != nil {
			return err
		}

		msgID, docID, accessHash, shaHex, err := mgr.UploadFromReader(runCtx, encReader, size, fileName, mimeType, limiter, func(uploaded, total int64) {
			pct := float64(uploaded) / float64(total) * 100
			fmt.Printf("\rProgress: %.1f%% (%d / %d bytes)", pct, uploaded, total)
		})
		if err != nil {
			return err
		}

		_, err = database.CreateFileWithID(fileUID, folderID, fileName, size, mimeType, msgID, strconv.FormatInt(docID, 10), strconv.FormatInt(accessHash, 10), shaHex, 1)
		return err
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nUpload failed for %s: %v\n", fileName, err)
		return
	}

	fmt.Printf("\n✓ %s safely stored in Telegram Storage Channel.\n", fileName)
}

func runUpdate(cfg *app.Config) {
	fmt.Printf("Checking for updates (current version: v%s)...\n", app.Version)
	res, err := update.CheckForUpdate(nil)
	if err != nil {
		fmt.Printf("Error checking for updates: %v\n", err)
		return
	}

	if !res.UpdateAvailable {
		fmt.Printf("TeleDrive is up to date (v%s).\n", app.Version)
		return
	}

	fmt.Printf("New version available: %s (Current: v%s)\n", res.LatestVersion, app.Version)
	if res.Release != nil {
		fmt.Printf("Release: %s\n", res.Release.Name)
	}

	name, downloadURL, err := update.FindAssetForCurrentPlatform(res.Release)
	if err != nil {
		fmt.Printf("Error finding compatible release asset: %v\n", err)
		return
	}

	fmt.Printf("Downloading %s from %s...\n", name, downloadURL)
	if err := update.ApplyUpdate(downloadURL); err != nil {
		fmt.Printf("Failed to apply update: %v\n", err)
		return
	}

	fmt.Printf("✓ TeleDrive successfully updated to %s!\n", res.LatestVersion)
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

	var destWriter io.Writer = out
	if fileRecord.IsEncrypted == 1 {
		encKey, encIV := crypto.DeriveFileStreamKeyAndIV(cfg.SecretKey, fileRecord.ID)
		dw, dwErr := crypto.DecryptStreamWriter(out, encKey, encIV, 0)
		if dwErr != nil {
			fmt.Printf("Error initializing decryption: %v\n", dwErr)
			return
		}
		destWriter = dw
	}

	ctx := context.Background()
	err = mgr.Run(ctx, func(runCtx context.Context) error {
		return mgr.DownloadFull(runCtx, docID, docHash, destWriter)
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
