# TeleDrive Architecture

TeleDrive is a single-binary cloud storage system that bridges a web interface and virtual file system to Telegram's distributed cloud infrastructure via MTProto.

---

## 1. System Overview

```
+---------------------------------------------------------------------------------------------------+
|                                          TeleDrive Host                                           |
|                                                                                                   |
|  +--------------------+   +-----------------------+   +-------------------+   +----------------+  |
|  |  Web UI (Browser)  |   |  WebDAV Client (OS)   |   |   CLI Commands    |   | Share Visitor  |  |
|  | (Vanilla HTML/CSS) |   | (Win/macOS/davfs/rcl) |   | (upload/download) |   |  (/s/{token})  |  |
|  +--------------------+   +-----------------------+   +-------------------+   +----------------+  |
|            |                          |                         |                     |           |
|            v                          v                         v                     v           |
|  +--------------------+   +-----------------------+   +-------------------+   +----------------+  |
|  |  HTTP Web Handler  |   |    WebDAV Gateway     |   |   CLI Execution   |   | Share Handler  |  |
|  | (Signed HMAC-SHA)  |   | (Basic Auth /webdav)  |   |  (Local Secret)   |   | (Rate-Limited) |  |
|  +--------------------+   +-----------------------+   +-------------------+   +----------------+  |
|            |                          |                         |                     |           |
|            +--------------------------+------------+------------+---------------------+           |
|                                                    |                                              |
|                                                    v                                              |
|                                   +---------------------------------+                             |
|                                   |       Drive Core Engine         |                             |
|                                   | - Virtual Folder Tree & Trash   |                             |
|                                   | - Zero-Knowledge AES-CTR Stream |                             |
|                                   | - Resumable Upload Coordinator  |                             |
|                                   | - Range Request Streamer (206)  |                             |
|                                   +---------------------------------+                             |
|                                          |                   |                                    |
|                                          v                   v                                    |
|                             +----------------------+  +---------------------+                     |
|                             | SQLite Metadata (WAL)|  | Telegram MTProto    |                     |
|                             | - modernc.org/sqlite |  | - gotd/td client    |                     |
|                             | - Soft-delete Trash  |  | - 512KB Part Worker |                     |
|                             | - AES-GCM Sessions   |  | - FLOOD_WAIT Backoff|                     |
|                             +----------------------+  +---------------------+                     |
+------------------------------------------------------------------|--------------------------------+
                                                                   | MTProto TCP/TLS
                                                                   v
                                                     +---------------------------+
                                                     | Telegram Cloud Platform   |
                                                     | (Storage Channel Vault)   |
                                                     +---------------------------+
```

---

## 2. Core Workflows

### 2.1 Resumable Upload Flow

The client browser slices large files into 5 MB–10 MB **Chunks**. TeleDrive receives each chunk, slices it into 512 KB MTProto **Parts**, streams them directly to Telegram via `gotd/td`, and tracks the upload session in SQLite.

```mermaid
sequenceDiagram
    autonumber
    actor User as Web Client
    participant API as TeleDrive HTTP (net/http)
    participant Core as Drive Engine
    participant DB as SQLite (modernc)
    participant TG as Telegram MTProto (gotd/td)

    User->>API: POST /api/upload/init (name, size, folder_id, mime)
    API->>DB: Create upload_session (total_parts, status=pending)
    API-->>User: Return session_id & chunk_size (5MB)

    loop Every 5MB Chunk
        User->>API: POST /api/upload/chunk (session_id, chunk_index, bytes)
        API->>Core: Split 5MB into 10x 512KB Parts
        loop Every 512KB Part
            Core->>TG: upload.saveBigFilePart (file_id, part_index, bytes)
        end
        API->>DB: Update uploaded_parts count
        API-->>User: 200 OK (chunk confirmed)
    end

    User->>API: POST /api/upload/complete (session_id)
    API->>TG: messages.sendMedia (InputMediaUploadedDocument)
    TG-->>API: Message confirmation (telegram_message_id, file_id)
    API->>DB: Insert files record & Delete upload_session
    API-->>User: 201 Created (Virtual File ready)
```

---

### 2.2 Download & Media Streaming Flow (HTTP 206 Range)

To preview videos or resume partial downloads, TeleDrive translates HTTP byte-range requests directly into MTProto part offsets without buffering the entire file.

```mermaid
sequenceDiagram
    autonumber
    actor Client as Browser / Media Player
    participant API as TeleDrive HTTP
    participant DB as SQLite
    participant TG as Telegram MTProto

    Client->>API: GET /api/files/{id}/stream (Range: bytes=1048576-2097151)
    API->>DB: Query file metadata (telegram_file_id, size, mime_type)
    API->>API: Calculate MTProto Part offsets (part 2 to 3, 512KB each)
    API->>TG: upload.getFile (offset=1048576, limit=1048576)
    TG-->>API: Stream raw 512KB parts
    API-->>Client: 206 Partial Content (Content-Range, stream bytes)
```

---

### 2.3 Share Link Flow & Virtual Folder Jailing

Public visitors access files or folders via cryptographic tokens. Passwords and expiry are validated at the edge before rendering content or streaming bytes. For shared folders, recursive CTE boundary checks prevent guest escape from the shared subtree.

```mermaid
sequenceDiagram
    autonumber
    actor Guest as Guest User
    participant Web as TeleDrive (/s/{token})
    participant DB as SQLite (Recursive CTE)
    participant TG as Telegram MTProto

    Guest->>Web: GET /s/{token}
    Web->>DB: Lookup share_token (check expires_at, is_password_protected)
    alt Password Protected
        Web-->>Guest: Render Password Prompt Form
        Guest->>Web: POST /s/{token}/unlock (password)
        Web->>DB: Verify bcrypt password hash
    end
    alt Shared File
        Web-->>Guest: Render File Landing Page (metadata + stream preview + download button)
        opt Direct Download or Stream
            Guest->>Web: GET /s/{token}/download
            Web->>DB: Increment download_count
            Web->>TG: Stream file parts
            TG-->>Guest: Pass-through file bytes
        end
    else Shared Virtual Folder
        Web->>DB: List files & folders at root of share
        Web-->>Guest: Render Folder Explorer (share.html)
        opt Guest Subfolder Navigation
            Guest->>Web: GET /s/{token}?folder_id={sub_id}
            Web->>DB: IsFolderInFolderHierarchy(sub_id, shared_root_id)
            alt In Jail Subtree
                Web-->>Guest: Render Subfolder & Breadcrumb Path
            else Outside Jail
                Web-->>Guest: 404 Not Found (Access Denied)
            end
        end
        opt Download File inside Shared Folder
            Guest->>Web: GET /s/{token}/files/{file_id}/download
            Web->>DB: IsFileInFolderHierarchy(file_id, shared_root_id)
            Web->>TG: Stream file parts
            TG-->>Guest: Pass-through file bytes
        end
    end
```

---

### 2.4 Disaster Recovery & Backup Flow

The SQLite database is backed up to the dedicated Storage Channel periodically and on shutdown.

```mermaid
sequenceDiagram
    autonumber
    participant Scheduler as Backup Scheduler (Cron/Shutdown)
    participant DB as SQLite (WAL)
    participant TG as Telegram Storage Channel

    Scheduler->>DB: PRAGMA wal_checkpoint(TRUNCATE)
    Scheduler->>Scheduler: Vacuum into temporary snapshot + gzip
    Scheduler->>TG: Send document (teledrive-backup-{timestamp}.db.gz)
    Scheduler->>TG: Pin message (pin_id)
    Scheduler->>DB: Update last_backup_timestamp
```

---

### 2.5 WebDAV Gateway Flow

TeleDrive implements `golang.org/x/net/webdav.FileSystem` over SQLite and MTProto, permitting native OS mounting (Windows Explorer, macOS Finder, Linux `davfs2`, `rclone`) via standard HTTP Basic Authentication.

```mermaid
sequenceDiagram
    autonumber
    actor OS as OS WebDAV Client (Explorer/Finder)
    participant WebDAV as WebDAV Gateway (/webdav)
    participant Auth as HTTP Basic Auth Validator
    participant DB as SQLite Virtual FS
    participant TG as Telegram MTProto

    OS->>WebDAV: PROPFIND /webdav/
    WebDAV->>Auth: Validate admin credentials
    Auth-->>WebDAV: Authenticated
    WebDAV->>DB: Resolve root folder & list items (deleted_at IS NULL)
    WebDAV-->>OS: 207 Multi-Status (XML directory listing)

    opt Read/Stream File
        OS->>WebDAV: GET /webdav/Documents/report.pdf
        WebDAV->>DB: ResolvePath("/Documents/report.pdf")
        WebDAV->>TG: Stream 512KB parts (decrypt on-the-fly if is_encrypted)
        TG-->>OS: HTTP 200 / 206 stream bytes
    end

    opt Write/Upload File
        OS->>WebDAV: PUT /webdav/Documents/new.pdf
        WebDAV->>WebDAV: Stream chunks, encrypt parts with AES-CTR
        WebDAV->>TG: upload.saveBigFilePart + sendMedia
        WebDAV->>DB: CreateFileWithID (parent folder, size, encrypted=1)
        WebDAV-->>OS: 201 Created
    end
```

---

### 2.6 Zero-Knowledge Seekable Stream Encryption Flow

To prevent Telegram or intermediate proxies from inspecting content while preserving real-time video seeking, TeleDrive uses AES-128/256-CTR with 1-to-1 byte parity and zero size expansion.

```mermaid
sequenceDiagram
    autonumber
    actor Client as Browser / Media Player
    participant Engine as TeleDrive Stream Engine
    participant Crypto as AES-CTR Stream Cipher
    participant TG as Telegram Storage Channel

    Note over Client,Engine: Range Request: bytes=1048576-2097151 (Part 2 to 3)
    Client->>Engine: GET /api/files/{id}/stream (Range header)
    Engine->>Engine: Derive stream key & initial IV from secret_key + file_id
    Engine->>TG: upload.getFile (offset=1048576, limit=1048576)
    TG-->>Engine: Raw encrypted MTProto bytes
    Engine->>Crypto: Seek(offset=1048576) -> Adjust 128-bit counter by (1048576/16)
    Crypto-->>Engine: Decrypted plaintext byte stream
    Engine-->>Client: 206 Partial Content (Immediate O(1) seekable video)
```

---

### 2.7 Virtual Trash Staging & Automated Purge Flow

Deletions do not destroy data instantly. Soft-deleted items are marked with `deleted_at`, hidden from active views and WebDAV, and permanently purged after 30 days.

```mermaid
sequenceDiagram
    autonumber
    actor User as Dashboard / WebDAV
    participant Web as TeleDrive API
    participant DB as SQLite Virtual FS
    participant Worker as Trash Purge Worker (24h)
    participant TG as Telegram Storage Channel

    User->>Web: DELETE /api/files/{id}
    Web->>DB: SoftDeleteFile(id) -> SET deleted_at = CURRENT_TIMESTAMP
    Web-->>User: 200 OK (Moved to Trash)

    opt User Restores File
        User->>Web: POST /api/trash/{id}/restore
        Web->>DB: RestoreFile(id) -> SET deleted_at = NULL
    end

    opt Automated 30-Day Purge
        Worker->>DB: ListTrash(older_than=30 days)
        loop Each Expired File
            Worker->>TG: DeleteMessages(channel_id, telegram_message_id)
            Worker->>DB: PurgeFile(id) -> DELETE FROM files WHERE id = ?
        end
    end
```

---

### 2.8 HMAC-SHA256 Signed Session Flow

Stateless, tamper-proof session tokens authenticate web requests without requiring server-side session table lookups.

```mermaid
sequenceDiagram
    autonumber
    actor Browser as Web Browser
    participant Auth as TeleDrive Auth Handler
    participant Crypto as Crypto Package (HMAC-SHA256)

    Browser->>Auth: POST /api/auth/login (password)
    Auth->>Auth: Compare password with TELEDRIVE_ADMIN_PASSWORD
    Auth->>Crypto: GenerateSignedSession(username, 30 days, secret_key)
    Crypto-->>Auth: token = base64(payload) + "." + base64(signature)
    Auth-->>Browser: Set-Cookie: teledrive_session=token (HttpOnly, SameSite=Lax)

    Browser->>Auth: GET /api/files (Cookie: teledrive_session)
    Auth->>Crypto: ValidateSignedSession(token, secret_key)
    Crypto->>Crypto: Constant-time HMAC comparison + expiry check
    Crypto-->>Auth: Valid (user=admin)
    Auth-->>Browser: 200 OK (Authorized)
```

---

### 2.9 Bulk Operations & Drag-and-Drop Organization Flow

TeleDrive provides high-throughput batch operations for desktop-style file organization via drag-and-drop or multi-selection.

```mermaid
sequenceDiagram
    autonumber
    actor User as User / Browser
    participant API as TeleDrive HTTP
    participant DB as SQLite (Transaction)

    alt Bulk Move / Drag-and-Drop
        User->>API: POST /api/batch/move {file_ids: [...], folder_ids: [...], target_folder_id: "..."}
        API->>DB: BatchMove(fileIDs, folderIDs, targetFolderID)
        loop Each File & Folder
            DB->>DB: Check cycle prevention & update folder_id
        end
        DB-->>API: Success
        API-->>User: 200 OK {success: true}
    else Bulk Trash
        User->>API: POST /api/batch/trash {file_ids: [...], folder_ids: [...]}
        API->>DB: BatchTrash(fileIDs, folderIDs)
        loop Each File & Folder
            DB->>DB: Set deleted_at = CURRENT_TIMESTAMP
        end
        DB-->>API: Success
        API-->>User: 200 OK {success: true}
    end
```

---

## 3. Database Schema (SQLite)

```sql
-- Virtual Folder hierarchy
CREATE TABLE folders (
    id TEXT PRIMARY KEY,
    parent_id TEXT NULL REFERENCES folders(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);
CREATE INDEX idx_folders_parent ON folders(parent_id);
CREATE INDEX idx_folders_deleted ON folders(deleted_at);

-- Virtual Files mapped to Telegram objects
CREATE TABLE files (
    id TEXT PRIMARY KEY,
    folder_id TEXT NULL REFERENCES folders(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    telegram_message_id INTEGER NOT NULL,
    telegram_file_id TEXT NOT NULL,
    telegram_access_hash TEXT NOT NULL,
    sha256 TEXT,
    is_encrypted INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);
CREATE INDEX idx_files_folder ON files(folder_id);
CREATE INDEX idx_files_name ON files(name);
CREATE INDEX idx_files_deleted ON files(deleted_at);

-- In-flight resumable upload sessions
CREATE TABLE upload_sessions (
    id TEXT PRIMARY KEY,
    folder_id TEXT REFERENCES folders(id),
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    total_parts INTEGER NOT NULL,
    uploaded_parts INTEGER DEFAULT 0,
    telegram_file_id INTEGER NOT NULL, -- gotd random ID
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Public share links (files or folders)
CREATE TABLE share_links (
    id TEXT PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    file_id TEXT NULL REFERENCES files(id) ON DELETE CASCADE,
    folder_id TEXT NULL REFERENCES folders(id) ON DELETE CASCADE,
    password_hash TEXT NULL,
    expires_at DATETIME NULL,
    download_count INTEGER DEFAULT 0,
    max_downloads INTEGER NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_share_token ON share_links(token);

-- Encrypted Telegram MTProto session & system settings
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 4. Source Directory Structure

Following the Ponytail principle (minimal files, zero unnecessary abstractions):

```
teledrive/
├── CONTEXT.md
├── ARCHITECTURE.md
├── SECURITY.md
├── MILESTONES.md
├── docs/
│   ├── USER_GUIDE.md
│   ├── assets/
│   │   └── teledrive-login.png
│   └── adr/
│       ├── 0001-storage-channel-target.md
│       ├── 0002-pure-go-mtproto-client.md
│       ├── 0003-pass-through-streaming.md
│       ├── 0004-modernc-sqlite-pure-go.md
│       ├── 0005-stdlib-net-http.md
│       ├── 0006-chunked-resumable-http-upload.md
│       ├── 0007-aes-gcm-session-encryption.md
│       ├── 0008-primary-account-tos-safe-mode.md
│       ├── 0009-zero-build-embedded-modern-ui.md
│       ├── 0010-database-snapshots-and-rolling-retention.md
│       ├── 0011-npm-distribution-wrapper.md
│       ├── 0012-signed-session-token.md
│       ├── 0013-zero-knowledge-seekable-stream-encryption.md
│       └── 0014-embedded-webdav-gateway.md
├── bin/
│   └── teledrive.js             # Zero-dependency npm launcher wrapper
├── cmd/
│   └── teledrive/
│       ├── main.go              # CLI router (server, login, upload, list, backup)
│       └── commands.go          # Subcommand implementations
├── internal/
│   ├── app/
│   │   └── config.go            # Minimal configuration loader
│   ├── crypto/
│   │   ├── aes.go               # AES-256-GCM encryption helpers (session string)
│   │   ├── session.go           # HMAC-SHA256 signed session tokens
│   │   └── stream.go            # AES-CTR seekable stream cipher (zero-knowledge)
│   ├── db/
│   │   ├── db.go                # SQLite init (WAL mode & schema migrations)
│   │   ├── files.go             # Virtual folder, file, trash, and VFS CRUD
│   │   └── backup.go            # Snapshot export & restore
│   ├── telegram/
│   │   ├── client.go            # gotd/td connection manager
│   │   ├── auth.go              # CLI interactive login wizard
│   │   ├── uploader.go          # 512KB MTProto part uploader
│   │   ├── downloader.go        # Range-aware MTProto part streamer (with decryptor)
│   │   └── limiter.go           # FloodWait backoff & rate queue
│   └── web/
│       ├── server.go            # net/http ServeMux routes & middleware
│       ├── handlers_drive.go    # File/folder operations & downloads/streams
│       ├── handlers_upload.go   # Resumable chunked upload endpoints
│       ├── handlers_trash.go    # Virtual trash restore, empty, and auto-purge
│       ├── handlers_share.go    # Public /s/{token} & /api/shares management
│       ├── handlers_snapshot.go # Web snapshot history & restore
│       ├── webdav.go            # WebDAV FileSystem bridge (/webdav)
│       └── static/              # Embedded UI assets (CSS design tokens, Lucide SVG, Vanilla JS)
├── package.json                 # npm metadata & bin entry
├── go.mod
└── go.sum
```
