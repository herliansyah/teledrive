# TeleDrive Implementation Milestones

This roadmap outlines the implementation phases for TeleDrive. In accordance with the Ponytail principle (lazy senior dev mode), each milestone delivers the minimal working code required, uses standard libraries wherever possible, and concludes with a runnable self-check.

---

## Milestone 1: Foundation & Core Storage Engine

**Goal**: Establish the Go 1.22 module, single-binary CLI entry point, pure-Go SQLite database with WAL mode, schema migrations, and AES-256-GCM cryptographic primitives.

* **Key Deliverables**:
  - `go.mod` with Go 1.22+ and minimal dependencies (`modernc.org/sqlite`).
  - `cmd/teledrive/main.go` supporting basic subcommand routing (`server`, `login`, `upload`, `list`).
  - `internal/db`: SQLite initializer, WAL configuration, and schema migration runner (`folders`, `files`, `upload_sessions`, `share_links`, `settings`).
  - `internal/crypto`: AES-256-GCM encryption and decryption helpers with SHA-256 key derivation.
* **Runnable Verification**:
  - Test verifying schema creation and AES-256-GCM round-trip encryption/decryption of arbitrary byte payloads.

---

## Milestone 2: MTProto Client & Interactive Auth Wizard

**Goal**: Connect to Telegram servers using pure Go MTProto (`gotd/td`), authenticate via terminal CLI, encrypt and persist session credentials, and verify the dedicated Storage Channel.

* **Key Deliverables**:
  - `internal/telegram/client.go`: `gotd/td` client lifecycle manager with automatic reconnect and organic client fingerprinting (`DeviceModel`, `AppVersion`, `LangCode`).
  - `cmd/teledrive/login.go`: Interactive terminal login wizard collecting personal `API_ID`, `API_HASH`, phone number, OTP code, and 2FA password.
  - `internal/telegram/auth.go`: Serialization and AES-256-GCM encryption of the MTProto `auth_key` into SQLite `settings`.
  - Storage Channel bootstrap: Auto-detect or create a strictly private channel named `TeleDrive Vault`.
* **Runnable Verification**:
  - Running `teledrive login` successfully writes the encrypted session to the database, and a subsequent startup authenticates against Telegram without re-prompting.

---

## Milestone 3: Chunked Resumable Upload Pipeline

**Goal**: Implement the 512 KB MTProto part uploader with bounded concurrency, stream pass-through, adaptive pacing, and automated `FLOOD_WAIT` backoff.

* **Key Deliverables**:
  - `internal/telegram/uploader.go`: Streaming uploader using `upload.saveBigFilePart` (512 KB parts) with support for files up to 2 GB (free) / 4 GB (premium).
  - `internal/telegram/limiter.go`: Concurrency-limiting worker queue (strictly 1 upload stream in Primary Safe Mode) with adaptive 20ms–50ms inter-part pacing and `FLOOD_WAIT_X` defensive backoff.
  - `cmd/teledrive/upload.go`: CLI command `teledrive upload <path> --folder <path>` to test chunked upload of large files from the command line.
* **Runnable Verification**:
  - Upload a generated 50 MB random test file via CLI; verify that all parts arrive in the Telegram Storage Channel and a corresponding file record appears in SQLite.

---

## Milestone 4: Range-Aware Download & Media Streaming Engine

**Goal**: Implement zero-spooling file download and HTTP 206 Partial Content (Range request) translation directly from MTProto.

* **Key Deliverables**:
  - `internal/telegram/downloader.go`: Streamer fetching specific 512 KB parts using `upload.getFile` based on requested byte offsets.
  - HTTP Range Parser: Translates incoming HTTP `Range: bytes=start-end` headers into exact Telegram part chunks and byte slices.
  - `cmd/teledrive/download.go`: CLI command `teledrive download <file_id> --output <path>` with resume support.
* **Runnable Verification**:
  - Perform an HTTP GET with `Range: bytes=1048576-2097151` against an uploaded test file; verify that HTTP 206 is returned with the exact 1 MB slice and correct SHA-256 checksum.

---

## Milestone 5: Virtual File System & Disaster Recovery

**Goal**: Implement hierarchical virtual folder navigation, CRUD operations (rename, move, delete), and automated database snapshots to the Storage Channel.

* **Key Deliverables**:
  - `internal/db/files.go`: Virtual folder tree traversal, circular reference checks, file move, file rename, and cascading deletion (deleting messages in Telegram).
  - `internal/db/backup.go`: Automated SQLite snapshot exporter (`PRAGMA wal_checkpoint(TRUNCATE)` + gzip) and Telegram upload/pinning.
  - `cmd/teledrive/backup.go` and `cmd/teledrive/restore.go`: CLI commands to trigger manual backup and restore SQLite state from the Telegram Storage Channel.
* **Runnable Verification**:
  - Create nested virtual folders (`/work/docs/2026`), move files between them, trigger a backup to Telegram, wipe the local SQLite database, and execute `teledrive restore` to verify complete hierarchy recovery.

---

## Milestone 6: Modern Embedded Web Dashboard & Share Link Management (Completed)

**Goal**: Deliver a polished, modern, zero-build web interface using Go 1.22 `net/http`, embedded HTML5/CSS design tokens, modern Vanilla JS, Lucide SVG iconography, resumable chunked upload drawer, and full share link management.

* **Key Deliverables**:
  - `internal/web/server.go`: Go 1.22+ `http.ServeMux` router serving embedded assets (`templates/*` and `static/*`) with zero external build tooling.
  - Modern Design System (`style.css`): Slate/Zinc palette, Dark & Light theme toggle with `localStorage` persistence, responsive mobile drawer, and accessible custom modals.
  - Explorer Interface (`app.js`, `index.html`): Dual-view toggle (Grid View vs Table/List View), instant in-memory column sorting (Name, Size, Modified Date), and debounced search with clear state.
  - Floating Upload Manager: Bottom-right collapsible drawer with real-time chunk assembly tracking (`Chunk 3/8`), minimizable pill state, and sequential safe-mode processing.
  - Expanded Media & Document Viewer: Pass-through range-seek video streaming (HTTP 206), audio player, image/PDF viewer, and monospaced code/text preview.
  - Public Share & Management (`/s/{token}`, `GET /api/shares`, `DELETE /api/shares/{id}`): Public landing page with mobile QR Code generator, password protection, and dashboard view to audit and revoke active links.
  - Architecture Record: Documented in `docs/adr/0009-zero-build-embedded-modern-ui.md`.
* **Runnable Verification**:
  - Full automated integration suite passing (`go test ./...`), including authenticated API operations, share link listing and revocation, and successful compilation into a standalone static binary (`go build ./cmd/teledrive`).

---

## Milestone 7: Zero-Knowledge Encryption, Virtual Trash, Signed Sessions & WebDAV Gateway (Completed)

**Goal**: Elevate TeleDrive into an end-to-end private personal cloud with OS-level disk mounting capabilities, robust data safety, and zero-trust storage.

* **Key Deliverables**:
  - **Fase 1: Signed Session Token**: Replace static session cookie with HMAC-SHA256 signed tokens tied to `TELEDRIVE_SECRET_KEY` with expiration and HTTP-only protections.
  - **Fase 2: Virtual Trash**: Add `deleted_at` soft-delete staging to `folders` and `files`, web UI Trash tab, restore/empty actions, and 30-day background auto-purge scheduler.
  - **Fase 3: Zero-Knowledge Part Encryption**: Implement AES-CTR seekable stream cipher in `internal/crypto`, schema migration for `is_encrypted`, and seamless O(1) range-seek decryption in downloader and pass-through streamer.
  - **Fase 4: WebDAV Gateway**: Embed RFC 4918 WebDAV handler on `/webdav` using `golang.org/x/net/webdav` with HTTP Basic Auth, enabling direct drive mounting on Windows, macOS, Linux, and rclone.
* **Runnable Verification**:
  - Self-check suite verifying: (1) token forgery rejection, (2) soft-delete and restore cycle without Storage Channel message loss, (3) byte-for-byte encrypted round-trip upload and HTTP 206 range-seeking, and (4) WebDAV PROPFIND/GET/PUT integration.


