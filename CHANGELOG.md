# Changelog

All notable changes to TeleDrive are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.5.0] - 2026-09-21

### Added
- **Recursive Folder Upload (Web & CLI)**:
  - Upload entire directory trees and all contained files directly from the browser or terminal.
  - Web UI: Dedicated "Upload Folder" button in sidebar using HTML5 directory selection.
  - Dropzone: Recursive drag & drop folder traversal (`webkitGetAsEntry` / FileSystem API).
  - CLI: `teledrive upload <path>` automatically detects directories and recursively walks subfolders (`filepath.WalkDir`).
- **Upload Conflict Resolution Modal**:
  - Interactive conflict dialog when an uploaded item collides with an existing file or Virtual Folder.
  - Three resolution strategies: **Replace** (overwrite), **Keep Both** (auto-incremented `name (1).ext`), or **Skip**.
  - Includes an "Apply to all remaining conflicts" batch toggle.
- **In-App & CLI Self-Update**:
  - Auto-update checker querying GitHub Releases API (`herliansyah/teledrive`) via `/api/system/update`.
  - 1-click update in Web dashboard with seamless binary replacement and graceful restart.
  - Terminal command `teledrive update` for self-updating binary in place.
  - Update banner and indicator in Web dashboard when a newer version is detected.
- **In-App Changelog Viewer**:
  - Built-in "What's New / Changelog" interactive modal in Web UI, accessible via sidebar version link and Help dialog.
  - API endpoint `GET /api/changelog` delivering markdown-backed changelog directly to the dashboard.

### Changed
- **Share Link Expiration Label**:
  - Removed third-party brand reference ("like Google Drive") from the expiration dropdown. Cleaned up to pure `Never expires`.
- **Version Bump**:
  - Updated application and package metadata to `v1.5.0`.

---

## [1.4.0] - 2026-09-20

### Added
- **Virtual Folder Sharing & Jailed Guest Traversal**:
  - Share entire Virtual Folders via public or password-protected Share Links.
  - Jailed guest browsing: guests can navigate nested subfolders inside shared folders without leaking root or parent data.
- **Bulk Operations**:
  - Multi-select files and folders in list view for batched deletion to Virtual Trash or moving between folders.
- **Extended Link Expiration**:
  - Custom expiration dates and flexible presets (1 day, 7 days, 30 days, 90 days, 1 year, Never).
- **WebDAV Gateway**:
  - Embedded RFC 4918 WebDAV HTTP gateway allowing direct mounting as an OS network drive.
- **Zero-Knowledge Part Encryption**:
  - Client-side AES-256-GCM encryption of file parts before transfer to Telegram MTProto servers.
- **Virtual Trash**:
  - Staging area for deleted items with soft deletion and point-and-click restoration.

---

## [1.3.1] - 2026-09-18

### Changed
- Session stability improvements for MTProto client connections.
- Minor fixes in npm wrapper binary launcher.

---

## [1.3.0] - 2026-09-15

### Added
- **Storage Channel Auto-Discovery**:
  - Automated detection of existing TeleDrive storage channels during login onboarding.
  - Interactive channel binding and automatic database snapshot restoration on initial pairing.

---

## [1.2.0] - 2026-09-10

### Added
- **Database Snapshots & Rolling Retention**:
  - Automated point-in-time SQLite snapshot uploads to the Storage Channel.
  - Rolling retention policy preserving recent snapshots while pruning obsolete messages.
  - Snapshot history dashboard with 1-click point-in-time database restoration and file backup download.

---

## [1.1.0] - 2026-09-05

### Added
- UI attribution footer with creator profile, MIT license link, and GitHub repository reference.

---

## [1.0.2] - 2026-08-30

### Added
- LAN IP discovery and multi-interface network binding.
- Port auto-scanning to gracefully detect and avoid port collisions on startup.

---

## [1.0.1] - 2026-08-25

### Changed
- Streamlined GitHub Actions automated release pipeline for multi-architecture binary artifacts.

---

## [1.0.0] - 2026-08-20

### Added
- Initial public release of TeleDrive.
- High-capacity cloud storage using Telegram MTProto object storage.
- Chunked resumable HTTP upload pipeline with 512 KB MTProto part alignment.
- Zero-spool pass-through streaming with HTTP 206 Range requests for instant video seeking.
- Single-binary distribution with zero external runtime dependencies.
- Embedded modern dark-mode Web dashboard and full CLI toolset.
