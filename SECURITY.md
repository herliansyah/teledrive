# TeleDrive Security & Abuse Mitigation Guide

This document defines the security architecture, threat model, cryptographic practices, and Telegram Terms of Service (ToS) abuse mitigation strategies for TeleDrive.

---

## 1. Threat Model & Asset Protection

| Asset | Primary Threat | Mitigation |
| :--- | :--- | :--- |
| **MTProto Session (`auth_key`)** | Extraction from database leads to total Telegram account takeover. | Encrypted at rest via **AES-256-GCM**. Master key held in environment variable or restricted keyfile. |
| **Telegram API Credentials** | Public exposure of `API_ID` and `API_HASH`. | Loaded via environment variables (`TELEDRIVE_TG_APP_ID`, `TELEDRIVE_TG_APP_HASH`). Excluded from repository. |
| **Stored User Files** | Telegram inspects media or unauthorized parties view cloud storage. | Files stored in private channels; transparent **AES-CTR seekable stream encryption** with zero-knowledge keys and 1-to-1 byte parity. |
| **Admin Dashboard & WebDAV** | Unauthorized web or WebDAV access to virtual drive and upload endpoints. | Bcrypt password hashing (`cost=12`), tamper-proof **HMAC-SHA256 signed session tokens** (`HttpOnly`, `SameSite=Lax`), and WebDAV HTTP Basic Auth. |
| **Public Share Links** | Brute-force scanning of share URLs or guessable passwords. | 128-bit high-entropy random tokens, bcrypt password hashing, IP-based rate limiting on unlock attempts. |
| **Telegram Account Status** | Account ban or permanent freeze triggered by Telegram SpamBot. | Strict concurrency caps (concurrency=1), pacing delays (30ms), and automated `FLOOD_WAIT` backoff. |
| **Accidental Data Loss** | User or automated script accidentally deletes important files. | **Virtual Trash** soft-delete staging (`deleted_at`); files are recoverable until explicitly emptied or purged by 30-day worker. |

---

## 2. Session Encryption at Rest (AES-256-GCM)

The MTProto session string contains the cryptographic authorization key negotiated with Telegram data centers. TeleDrive enforces encryption before storing this string in SQLite:

```
[Plaintext MTProto Session]
            │
            ▼
[AES-256-GCM Encryptor] ◄── Key derived from TELEDRIVE_SECRET_KEY (SHA-256)
            │
            ├─► 12-byte random cryptographic nonce
            ├─► Ciphertext
            └─► 16-byte authentication tag
            │
            ▼
[Base64 Combined Payload] ──► Stored in SQLite `settings` table
```

### Key Management Rules:
1. Master secret MUST be provided via `TELEDRIVE_SECRET_KEY` environment variable.
2. If `TELEDRIVE_SECRET_KEY` is not provided, TeleDrive generates a 32-byte cryptographically secure key, writes it to `.teledrive.key` with Unix file permissions `0600` (read/write only by owner), and logs a security warning.
3. If the database is backed up to Telegram, the `.teledrive.key` is **never included** in the backup snapshot.

---

## 3. Telegram ToS Compliance & Primary Account "Safe Mode"

If using your **primary personal Telegram account**, TeleDrive MUST operate under **Safe Mode** to ensure 100% compliance with Telegram's API Terms of Service (Section 1.4) and avoid triggering automated anti-abuse or SpamBot penalties.

### 3.1 Five Cardinal Rules for Primary Account Safety

1. **Exclusive Use of Personal API Credentials**:
   - You MUST generate your own `API_ID` and `API_HASH` directly from [my.telegram.org](https://my.telegram.org).
   - **Never** use public or shared API keys found in open-source projects or online tutorials. Telegram periodically mass-bans accounts attached to blacklisted or revoked API application keys.

2. **Strictly Personal Storage (No Public / High-Traffic CDN Proxying)**:
   - Telegram's ToS strictly prohibits using MTProto as an automated public content distribution network (CDN).
   - When using a primary account, **Share Links must remain private** (intended for personal backup access or sharing specific files with family/colleagues). Do not post share links to public forums or use TeleDrive to host high-traffic downloads for strangers.

3. **Sequential Single-Worker Uploads (Zero Aggressive Multi-Threading)**:
   - TeleDrive caps upload concurrency to **1 active upload stream** at a time.
   - 512 KB Parts are transferred sequentially, exactly mimicking the network pattern of the official Telegram Desktop client.
   - Do not enable multi-threaded connection pooling when connected to a primary account.

4. **Organic Client Fingerprinting**:
   - The MTProto connection explicitly sets legitimate desktop client identity metadata (`DeviceModel: "PC 64bit"`, `AppVersion: "5.0.0"`, `LangCode: "en"`).
   - TeleDrive never sends empty or generic library strings that could be flagged by Telegram DC heuristics as an automated scraper.

5. **Channel Isolation & Zero External Exposure**:
   - The Storage Channel must remain **strictly private** with zero external members.
   - Never invite third-party bots or external users into the Storage Channel.

---

### 3.2 Automated `FLOOD_WAIT` Backoff Protocol

Telegram returns `FLOOD_WAIT_X` (where `X` is cooldown in seconds) when request frequency exceeds server-side thresholds:
1. **Immediate Queue Freeze**: The upload/download queue for that account enters `PAUSED` state immediately.
2. **Deterministic Sleep**: The worker pauses execution for `X + 5` seconds (adding a 5-second defensive margin).
3. **Zero Retry Spamming**: TeleDrive will NEVER make alternative API calls during a cooldown. Violating or ignoring `FLOOD_WAIT` is the #1 cause of escalating penalties to permanent account bans.
4. **UI Notification**: The Web Dashboard displays a clear, calm status indicator: `"Telegram Cooldown Active: Paused for Xs"`.

---

### 3.3 Adaptive Pacing Delay

To ensure network traffic matches normal human desktop usage:
- TeleDrive introduces an adaptive **20ms–50ms pacing delay** between consecutive 512 KB `upload.saveBigFilePart` requests.
- Batch deletions or metadata updates are paced with a minimum 200ms delay between API calls to avoid spam burst detection.

---

## 4. End-to-End File Data Privacy (Zero-Knowledge Seekable Stream Encryption)

To protect stored files against cloud provider inspection or unauthorized access, TeleDrive implements zero-knowledge payload encryption using **AES-CTR (Counter Mode)** stream cipher.

### 4.1 Cryptographic Design & Guarantees

1. **1-to-1 Byte Parity (Zero Size Expansion)**:
   - Unlike block modes (CBC) or AEAD modes (GCM, ChaCha20-Poly1305) which append 16-byte authentication tags per chunk or require padding, AES-CTR produces ciphertext of the exact same length as plaintext (`len(ciphertext) == len(plaintext)`).
   - This preserves Telegram's strict MTProto requirement: every part except the final part must be an exact multiple of 1 KB (typically 512 KB = 524,288 bytes). Any size overhead would misalign part boundaries.

2. **$O(1)$ Instant Seekability for HTTP 206 Streaming**:
   - CTR mode transforms AES into a synchronous stream cipher by encrypting a succession of counter blocks.
   - For any byte offset $K$, the exact 128-bit counter state is calculated in $O(1)$ time:
     $$\text{Block Index} = \lfloor K / 16 \rfloor$$
     $$\text{Counter} = \text{Initial IV} + \text{Block Index}$$
     $$\text{Keystream Skip} = K \pmod{16}$$
   - This allows video and audio streaming with HTTP 206 Range requests to jump to any minute of a 4 GB video with zero CPU decryption latency and zero full-file pre-buffering.

3. **Key & IV Derivation via HKDF-SHA256**:
   - Master secret is loaded from `TELEDRIVE_SECRET_KEY`.
   - Each file has a unique UUID `file_id`.
   - Per-file 32-byte encryption key and 16-byte initial IV are derived via HMAC-based Key Derivation:
     $$\text{Key} = \text{HMAC-SHA256}(\text{SecretKey}, \text{file\_id} \parallel \text{"key"})$$
     $$\text{IV} = \text{HMAC-SHA256}(\text{SecretKey}, \text{file\_id} \parallel \text{"iv"})[0:16]$$
   - Telegram servers only ever receive opaque pseudorandom binary blobs. Even with full access to channel messages, Telegram engineers or attackers cannot decrypt or identify file signatures without `TELEDRIVE_SECRET_KEY`.

---

## 5. Web Application Security Controls

* **HMAC-SHA256 Signed Session Tokens**: Web authentication uses cryptographically signed session cookies (`teledrive_session`) containing `username`, `expiration_timestamp`, and an `HMAC-SHA256` signature. Tokens are verified using constant-time comparison (`crypto/subtle.ConstantTimeCompare`), preventing timing attacks and tampering.
* **Cookie Flags**: All session cookies are flagged with `HttpOnly` (preventing XSS access) and `SameSite=Lax` (defending against cross-site request forgery).
* **WebDAV HTTP Basic Authentication**: The embedded WebDAV gateway at `/webdav/` requires HTTP Basic Auth validated against the administrative credentials. Unauthorized requests immediately receive `401 Unauthorized` with `WWW-Authenticate: Basic realm="TeleDrive WebDAV"`.
* **Path Traversal Defense**: All virtual folder and file paths are resolved through UUID keys in SQLite, eliminating directory traversal attacks (`../`). WebDAV paths are normalized with `path.Clean` before hierarchical resolution.
* **Range Request Boundary Validation**: Requests containing `Range: bytes=start-end` are strictly validated against total file size to prevent integer overflow and memory exhaustion attacks.
* **Share Link Rate Limiting**: The password unlock endpoint for protected share links limits attempts to 5 failures per minute per IP to prevent dictionary attacks.

