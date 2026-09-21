package main

import (
	"fmt"
	"os"

	"teledrive/internal/app"
	"teledrive/internal/db"
)

const banner = `
=====================================================
  TeleDrive - Cloud Storage Powered by Telegram MTProto
  Created by Herliansyah (https://github.com/herliansyah)
  License: MIT | Repository: https://github.com/herliansyah/teledrive
=====================================================
`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg := app.LoadConfig()

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("TeleDrive v%s\n", app.Version)
		fmt.Println("Created by Herliansyah (https://github.com/herliansyah)")
		fmt.Println("Licensed under the MIT License")
		fmt.Println("Repository: https://github.com/herliansyah/teledrive")
	case "login":
		runLogin(cfg, os.Args[2:])
	case "logout":
		runLogout(cfg, os.Args[2:])
	case "server":
		runServer(cfg)
	case "upload":
		runUpload(cfg, os.Args[2:])
	case "download":
		runDownload(cfg, os.Args[2:])
	case "list":
		runList(cfg, os.Args[2:])
	case "backup":
		runBackup(cfg)
	case "restore":
		runRestore(cfg)
	case "update":
		runUpdate(cfg)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(banner)
	fmt.Println(`Usage: teledrive <command> [arguments]

Commands:
  login     Authenticate Telegram account via MTProto wizard (use --force to switch account)
  logout    Disconnect and revoke current Telegram MTProto session (use --clean to wipe local data)
  server    Start the web dashboard and streaming server
  upload    Upload a file or folder to TeleDrive storage (e.g. teledrive upload ./folder)
  download  Download a file from TeleDrive (e.g. teledrive download <file_id>)
  list      List files and folders (e.g. teledrive list)
  backup    Export SQLite snapshot and upload to Telegram Storage Channel
  restore   Restore SQLite database from Telegram Storage Channel
  update    Check and update TeleDrive to the latest version
  version   Show TeleDrive version`)
}


func openDatabase(cfg *app.Config) *db.DB {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database at %s: %v\n", cfg.DBPath, err)
		os.Exit(1)
	}
	return database
}
