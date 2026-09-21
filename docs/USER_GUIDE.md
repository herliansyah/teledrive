# 📖 TeleDrive User Guide & How It Works
# Panduan Pengguna & Cara Kerja TeleDrive

[English](#english) &bull; [Bahasa Indonesia](#bahasa-indonesia)

---

<a name="english"></a>
## 🇬🇧 English: Complete User Guide

### 1. What is TeleDrive?

**TeleDrive** is an open-source, zero-cost personal cloud storage system that transforms your personal Telegram account into an unlimited, high-speed virtual drive. It provides a modern, Google Drive-like Web Dashboard and a robust CLI, all packaged into a single lightweight binary.

#### 💡 The Core Philosophy: Zero Disk Spooling
Traditional cloud bridges download entire files to your VPS or server disk before forwarding them. If you upload a 2 GB file, your server requires at least 2 GB of free disk space and subjects your SSD to heavy wear.

**TeleDrive operates differently:**
- **Zero VPS Disk Wear**: Files streamed through the browser or CLI are sliced into small chunks in memory and forwarded directly to Telegram's distributed data centers via native MTProto.
- **Direct Pipelining**: Your server acts purely as a real-time protocol bridge, keeping RAM usage minimal (~30–50 MB) and disk usage limited solely to a small SQLite metadata database.

---

### 2. Storage Capacities & Telegram Limits

TeleDrive leverages Telegram's official document storage infrastructure:

| Account Type | Maximum File Size | Total Storage Limit | Monthly Bandwidth |
| :--- | :--- | :--- | :--- |
| **Telegram Free** | **2.0 GB** per file | **Unlimited** ♾️ | Free & Unlimited |
| **Telegram Premium** | **4.0 GB** per file | **Unlimited** ♾️ | Free & Unlimited |

> **Note**: While the total number of files and storage capacity is virtually unlimited, Telegram imposes rate limits on aggressive automated transfers. TeleDrive includes a built-in **Safe Mode** to ensure complete compliance.

---

### 3. How It Works Under the Hood

```
[Browser / Web UI]
       │
       ▼ (5 MB Chunks via HTTP Multipart)
[TeleDrive Bridge Server]
       │
       ▼ (512 KB Parts via Native MTProto TCP)
[Telegram Cloud Data Centers] ──► Saved in private "TeleDrive Vault"
```

1. **Client-Side Slicing**: When you upload a file in the web dashboard, the browser reads the file locally and slices it into 5 MB chunks.
2. **MTProto Part Streaming**: The TeleDrive server receives each chunk and streams it into Telegram's MTProto protocol in 512 KB parts (`upload.saveBigFilePart`).
3. **Metadata Persistence**: Once all parts are uploaded, Telegram returns a unique document handle (`DocumentID`, `AccessHash`, `FileReference`). TeleDrive saves this metadata along with folder hierarchy into a local SQLite database (`teledrive.db`) running in Write-Ahead Logging (WAL) mode.
4. **Instant Media Streaming (HTTP 206)**: When you stream a video or audio file in the web player, TeleDrive requests only the required byte offsets from Telegram via MTProto range requests, allowing you to seek through a 2 GB video instantly without waiting for the full file to download.

---

### 4. Step-by-Step Walkthrough

#### Step 1: Initial Login & Telegram Pairing
Before running the web dashboard, link your Telegram account using the interactive terminal wizard:

```bash
# Via npx (recommended for Node.js / npm users):
npx teledrive login

# Or via pre-compiled binary:
./teledrive login
```

<div align="center">
  <img src="assets/teledrive-login.png" alt="TeleDrive Login Terminal Wizard" width="600" style="border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
  <p><em>Figure 1: TeleDrive interactive terminal authentication wizard.</em></p>
</div>

1. Enter your `API_ID` and `API_HASH` (obtained for free from [my.telegram.org](https://my.telegram.org)).
2. Enter your phone number in international format (e.g., `+6281234567890`).
3. Enter the 5-digit verification code sent directly to your Telegram app.
4. If Two-Factor Authentication (2FA) is active, enter your Cloud Password.
5. **Storage Channel Discovery & Onboarding**: TeleDrive automatically scans your Telegram account for an existing `TeleDrive Vault` channel. If an existing channel is detected, it prompts you to link it and optionally restore your latest database snapshot right away. If no channel exists yet, it creates a new private `TeleDrive Vault` channel automatically.

#### Step 2: Launching the Web Dashboard

```bash
# Via npx:
npx teledrive server

# Or via binary:
./teledrive server
```

By default, TeleDrive scans for an available port starting at `8080` and displays both local and LAN IP addresses. Open your web browser at:
👉 **`http://localhost:8080`**

- **Default Admin Password**: `admin123` (Configure via `TELEDRIVE_ADMIN_PASSWORD`).

#### Step 3: Navigating the Dashboard
- **Dual View Modes**: Toggle between **Grid View** (visual file cards with format badges) and **Table/List View** (with instant column sorting by Name, Size, and Date).
- **Search**: Fast real-time client-side search across your virtual drive.
- **Upload Manager**: Minimizable floating drawer at the bottom-right showing chunk-by-chunk progress (`Chunk 3/8`).
- **Media Preview**: Click any file to preview pictures, stream seekable MP4/WebM videos, play audio, view PDF documents, or inspect code/text files with syntax-friendly styling.

#### Step 4: Virtual Folder & File Management
- Click **New Folder** to create hierarchical directories.
- **Drag-and-Drop Organization**: Drag files or folders directly into folder cards/rows or onto the breadcrumb trail to move them instantly.
- **Move to Dialog**: Select **Move** from the item context menu (`⋮`) to pick any destination folder interactively.
- **Multi-Select & Bulk Operations**: Check items individually or use the select-all checkbox to activate the floating bulk action bar:
  - **Batch Move**: Move multiple files and folders to a target folder in one operation.
  - **Batch Trash**: Move multiple selected files and folders to Virtual Trash simultaneously.
- **Keyboard Shortcuts**:
  - `F2`: Quickly rename the currently selected file or folder.
  - `Delete` / `Backspace`: Move selected items directly to Virtual Trash.
- Move files across folders seamlessly (with cycle-prevention logic preventing folders from being moved into themselves or their subfolders).
- Rename or delete files at any time. Deleted files are safely sent to **Virtual Trash**.

#### Step 5: Public Share Links & Virtual Folder Sharing
Need to share files or an entire folder with guests without giving them server account credentials?
1. Open the file or folder context menu (`⋮`) and select **Create Public Share Link**.
2. **Extended Link Expiry**:
   - Default: **Never expires**.
   - Quick presets: **1 hour**, **1 day**, **7 days**, **90 days**, **1 year**.
   - Custom: Choose an exact expiry date and time via the built-in calendar picker.
3. Optionally set a **password** (hashed with bcrypt) for protected access.
4. **Virtual Folder Sharing (Jailed Guest Traversal)**:
   - When sharing a folder, guests receive a clean browsing interface restricted exclusively to that folder and its descendants.
   - Guests can navigate nested subfolders via breadcrumbs and download or stream individual files within the shared tree.
   - Guest requests are strictly jailed: visitors cannot traverse above the shared root folder.
5. Copy the public link (`/s/:token`) or click the **QR Code** button for mobile scan-and-go access.
6. Audit, monitor download counts, or revoke active share links anytime under the **Shared Links** tab in the sidebar.

#### Step 6: Database Snapshots & Point-in-Time Restore
All your folder structures and file references are stored in SQLite. TeleDrive provides built-in online disaster recovery:
1. Navigate to the **Snapshots** tab in the sidebar.
2. Click **Create Snapshot Now**: TeleDrive checkpoints SQLite WAL, creates a consistent compressed `.db.gz` snapshot, and uploads it to your Telegram Storage Channel.
3. **Rolling Retention**: TeleDrive automatically keeps the newest 5 snapshots and deletes older ones to keep your Telegram channel clean.
4. **Point-in-Time Restore**: Click **Restore** on any previous snapshot to safely hot-swap your database without restarting the server.
5. **Offline Backups**: Download `.db.gz` directly to your local computer, or use **Upload & Restore** to recover from an external backup file.

#### Step 7: Mount TeleDrive as a Local Network Drive (WebDAV)
TeleDrive includes an embedded native WebDAV gateway. You can mount your virtual cloud drive directly in your operating system's native file explorer without installing third-party sync agents:

- **WebDAV URL**: `http://localhost:8080/webdav`
- **Username**: `admin`
- **Password**: Your `TELEDRIVE_ADMIN_PASSWORD` (default: `admin123`)

##### Windows 10/11 File Explorer & Command Prompt
1. Open **File Explorer** and click **This PC**.
2. Click **Computer** > **Map network drive** (or `...` > Map network drive).
3. Choose a Drive Letter (e.g., `Z:`).
4. In **Folder**, enter either the standard HTTP URL or Windows native UNC syntax:
   - Standard URL: `http://localhost:8080/webdav` (or `http://192.168.x.x:8080/webdav`)
   - Native UNC syntax (recommended for custom ports or network IPs): `\\192.168.x.x@8080\webdav`
5. Check **Connect using different credentials** and click **Finish**.
6. Enter `admin` and your password.

*Or via Command Prompt / PowerShell:*
```cmd
net use Z: \\localhost@8080\webdav /user:admin admin123 /persistent:yes
```

> ⚠️ **Windows HTTP Troubleshooting ("A device attached to the system is not functioning" / Error 0x8007001F)**:
> Windows WebClient blocks unencrypted HTTP Basic Auth by default. If Windows rejects the connection, open **PowerShell as Administrator** and run:
> ```powershell
> Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" -Name "BasicAuthLevel" -Value 2 -Type DWord
> Restart-Service WebClient
> ```

##### macOS Finder
1. In Finder, press `Cmd + K` (or select **Go** > **Connect to Server...**).
2. Enter: `http://localhost:8080/webdav` and click **Connect**.
3. Choose **Registered User**, enter `admin` and your password.

##### Linux (davfs2 & rclone)
```bash
# Via davfs2:
sudo mount -t davfs http://localhost:8080/webdav /mnt/teledrive -o username=admin

# Via rclone:
rclone config # select WebDAV, URL http://localhost:8080/webdav, vendor other
rclone mount teledrive: /mnt/teledrive --vfs-cache-mode writes
```

#### Step 8: Virtual Trash & Accidental File Recovery
TeleDrive implements safety-first staging for all file and folder deletions:
1. **Soft-Delete**: Deleting a file or folder from the web dashboard or WebDAV does not delete data immediately; it assigns a `deleted_at` timestamp and moves the item to **Virtual Trash**.
2. **Instant Restore**: In the dashboard, open **Trash** to restore any file or folder back to its original location with a single click.
3. **Empty Trash**: Permanently purges all staged items and deletes the corresponding messages from your private Telegram storage channel.
4. **Automated 30-Day Purge**: A background maintenance worker automatically purges trash items older than 30 days every 24 hours.

---

### 5. Security & Telegram Safe Mode

TeleDrive is engineered with strict safeguards to protect your primary Telegram account from bans or restrictions:

1. **Official Telemetry Emulation**: TeleDrive identifies itself using standard Telegram Desktop client parameters (`PC 64bit`, `Linux/x86_64`, `AppVersion 5.0.0`).
2. **Sequential Safe Queue**: Uploads and downloads are processed sequentially (1 transfer at a time) mimicking natural desktop user behavior.
3. **Pacing Delay**: An adaptive 30ms sleep is enforced between 512 KB chunks to keep connection temperatures low.
4. **Automated Flood Control**: If Telegram issues a `FLOOD_WAIT_X` response, TeleDrive gracefully pauses until the cooldown elapses without crashing or hammering the API.
5. **Encrypted Session at Rest**: Your MTProto session authentication keys are encrypted in SQLite using **AES-256-GCM** derived from your secret key.
6. **Isolated Private Vault**: All file transfers go into a private storage channel with 0 external members.
7. **Zero-Knowledge Seekable Stream Encryption (AES-CTR)**: File parts are encrypted with AES-CTR stream cipher using per-file derived keys before dispatch to Telegram. Telegram servers only store opaque random bytes, while $O(1)$ video streaming seeking remains instantaneous.
8. **HMAC-SHA256 Signed Session Tokens**: Web sessions use tamper-proof cryptographically signed cookies with 30-day validity, protected with `HttpOnly` and `SameSite=Lax`.

---

### 6. Troubleshooting & FAQ

#### Q: Is my data private? Can other people see my files?
No. All files are uploaded into your own private Telegram channel (`TeleDrive Vault`). With Zero-Knowledge stream encryption active, Telegram servers only store encrypted ciphertext blobs.

#### Q: Can I use WebDAV with media players or office software?
Yes! Any application supporting WebDAV or mounted network paths (such as VLC player, Kodi, LibreOffice, or backup tools like restic/rclone) can read and write files directly through `http://localhost:8080/webdav`.

#### Q: How does Virtual Trash affect Telegram channel storage?
Soft-deleted files remain in Telegram until you click **Empty Trash** or the 30-day background purge worker runs. Once purged, TeleDrive issues `DeleteMessages` to delete the messages from Telegram.

#### Q: What happens if my server crashes or I move to another PC/VPS?
Because your SQLite metadata database is automatically snapshotted to your Telegram channel (`teledrive backup` or automated snapshots), moving to a new computer is seamless:
1. Run `./teledrive login` on the new machine.
2. TeleDrive automatically discovers your existing `TeleDrive Vault` channel and prompts to restore your latest database snapshot.
3. Confirm `[Y]`, and your files, virtual folders, and configuration are restored instantly without extra steps!

#### Q: How do I change the admin dashboard password?
Set the `TELEDRIVE_ADMIN_PASSWORD` environment variable before launching the server:
```bash
export TELEDRIVE_ADMIN_PASSWORD="my_strong_password"
./teledrive server
```

---

<a name="bahasa-indonesia"></a>
## 🇮🇩 Bahasa Indonesia: Panduan Lengkap Pengguna

### 1. Apa Itu TeleDrive?

**TeleDrive** adalah sistem penyimpanan awan pribadi (*personal cloud storage*) bersumber terbuka (open-source) yang menyulap akun Telegram Anda menjadi media penyimpanan tanpa batas berkecepatan tinggi. TeleDrive dilengkapi antarmuka web modern mirip Google Drive serta perintah baris (CLI), yang semuanya dikompilasi ke dalam satu file binary portabel tanpa ketergantungan runtime tambahan.

#### 💡 Filosofi Utama: Zero Disk Spooling (Hemat Hard Disk)
Jembatan cloud konvensional biasanya mengunduh seluruh file ke hard disk VPS/komputer server sebelum dikirimkan ke tujuan. Jika Anda mengunggah file 2 GB, server Anda membutuhkan ruang kosong minimal 2 GB dan menyebabkan keausan fisik (*wear-out*) pada SSD server.

**TeleDrive bekerja dengan prinsip yang berbeda:**
- **Bebas Beban Disk VPS**: File yang diunggah melalui browser atau CLI dipotong langsung di memori menjadi bagian-bagian kecil dan dialirkan langsung (*pipelined*) ke data center Telegram melalui protokol resmi MTProto.
- **Efisiensi Tinggi**: Server Anda murni bertindak sebagai jembatan protokol real-time. Konsumsi RAM sangat rendah (~30–50 MB) dan penggunaan disk lokal hanya digunakan untuk file database metadata SQLite berukuran beberapa megabyte.

---

### 2. Kapasitas Penyimpanan & Batasan Telegram

TeleDrive memanfaatkan infrastruktur penyimpanan dokumen resmi Telegram:

| Jenis Akun Telegram | Ukuran Maksimal per File | Batas Total Kapasitas | Bandwidth Bulanan |
| :--- | :--- | :--- | :--- |
| **Akun Reguler (Gratis)** | **2.0 GB** per file | **Tanpa Batas** ♾️ | Gratis & Tanpa Kuota |
| **Akun Telegram Premium** | **4.0 GB** per file | **Tanpa Batas** ♾️ | Gratis & Tanpa Kuota |

> **Catatan**: Meskipun kapasitas total dan jumlah file tidak terbatas, Telegram menerapkan aturan batasan frekuensi transfer data. TeleDrive dilengkapi fitur bawaan **Safe Mode** untuk menjaga akun Anda tetap aman 100% sesuai aturan resmi.

---

### 3. Cara Kerja Teknis di Balik Layar

```
[Browser Pengguna]
       │
       ▼ (Chunk 5 MB via HTTP Multipart)
[Server TeleDrive]
       │
       ▼ (Part 512 KB via Protokol Resmi MTProto)
[Pusat Data Telegram] ──► Tersimpan aman di channel privat "TeleDrive Vault"
```

1. **Pemotongan di Sisi Klien (Client Slicing)**: Saat Anda mengunggah file di web, browser membaca file secara lokal dan memotongnya menjadi chunk 5 MB.
2. **Aliran Data MTProto**: Server TeleDrive menerima tiap chunk dan mengalirkannya langsung ke server Telegram dalam bagian 512 KB (`upload.saveBigFilePart`).
3. **Penyimpanan Metadata**: Setelah seluruh bagian selesai terunggah, Telegram mengembalikan pointer dokumen (`DocumentID`, `AccessHash`, `FileReference`). TeleDrive mencatat metadata ini beserta struktur folder ke dalam database SQLite lokal berkecepatan tinggi dengan mode Write-Ahead Logging (WAL).
4. **Streaming Media Seketika (HTTP 206)**: Saat Anda menonton video atau memutar lagu di browser, TeleDrive hanya meminta rentang byte yang sedang diputar dari Telegram. Anda dapat melompati durasi (*seeking*) video 2 GB dalam hitungan detik tanpa perlu mengunduh seluruh file terlebih dahulu.

---

### 4. Panduan Penggunaan Langkah Demi Langkah

#### Langkah 1: Pasangkan Akun Telegram (Login Pertama Kali)
Sebelum menyalakan server web, hubungkan akun Telegram Anda melalui wizard terminal:

```bash
# Jalankan via npx (direkomendasikan untuk pengguna Node.js / npm):
npx teledrive login

# Atau jalankan binary kompilasi langsung:
./teledrive login
```

<div align="center">
  <img src="assets/teledrive-login.png" alt="Tampilan Terminal Wizard Login TeleDrive" width="600" style="border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
  <p><em>Gambar 1: Wizard otentikasi interaktif TeleDrive di terminal.</em></p>
</div>

1. Masukkan `API_ID` dan `API_HASH` Anda (dapat diperoleh gratis dari [my.telegram.org](https://my.telegram.org)).
2. Masukkan nomor telepon Telegram dalam format internasional (contoh: `+6281234567890`).
3. Masukkan kode login 5 digit yang dikirimkan ke aplikasi Telegram Anda.
4. Jika akun Anda menggunakan Two-Factor Authentication (2FA), masukkan Cloud Password Anda.
5. **Deteksi Otomatis & Onboarding Storage Channel**: TeleDrive secara cerdas memindai dialog akun Telegram Anda mencari channel `TeleDrive Vault` yang sudah ada. Jika channel lama ditemukan, TeleDrive menawarkan untuk langsung menyambungkannya dan memulihkan snapshot database terbaru. Jika belum pernah dibuat, TeleDrive otomatis membuat channel privat baru `TeleDrive Vault`.

#### Langkah 2: Menjalankan Server Web Dashboard

```bash
# Jalankan via npx:
npx teledrive server

# Atau jalankan via binary:
./teledrive server
```

Secara otomatis, TeleDrive akan mencari port yang tersedia mulai dari `8080` dan menampilkan alamat akses lokal maupun jaringan Wi-Fi/LAN. Buka browser Anda di:
👉 **`http://localhost:8080`**

- **Password Admin Bawaan**: `admin123` (Dapat diubah melalui variabel `TELEDRIVE_ADMIN_PASSWORD`).

#### Langkah 3: Navigasi Antarmuka Web
- **Pilihan Tampilan (Dual View)**: Pilih antara **Grid View** (kartu file interaktif dengan ikon format) atau **Table/List View** (tabel detail dengan sorting instan Nama, Ukuran, dan Tanggal).
- **Pencarian Cepat**: Cari file secara instan melalui kolom pencarian di bagian atas.
- **Upload Manager**: Drawer melayang di pojok kanan bawah yang menampilkan progres unggahan per-chunk secara transparan (`Chunk 3/8`).
- **Pratinjau Media & Kode**: Klik file apa saja untuk melihat gambar, memutar video MP4/WebM, mendengarkan lagu, membaca dokumen PDF, atau melihat file kode pemrograman (`.go`, `.py`, `.json`, `.txt`).

#### Langkah 4: Manajemen Folder & Berkas Virtual
- Klik tombol **New Folder** untuk membuat subfolder baru.
- **Drag-and-Drop Organisasi Berkas**: Tarik dan lepas (*drag & drop*) file atau folder langsung ke kartu/baris folder atau ke navigasi remah roti (*breadcrumb trail*) untuk memindahkannya secara instan.
- **Dialog "Move to..."**: Pilih opsi **Move** pada menu aksi (`⋮`) untuk memilih folder tujuan secara interaktif.
- **Multi-Pilih & Operasi Massal (Bulk Operations)**: Centang beberapa file/folder atau gunakan kotak centang pilih-semua untuk membuka bilah aksi melayang (*floating bulk action bar*):
  - **Pindah Massal (Batch Move)**: Memindahkan banyak berkas dan folder ke folder tujuan dalam satu klik.
  - **Hapus Massal (Batch Trash)**: Memindahkan seluruh item terpilih ke Virtual Trash secara bersamaan.
- **Pintasan Keyboard (Shortcuts)**:
  - `F2`: Mengubah nama (*rename*) item yang sedang dipilih dengan cepat.
  - `Delete` / `Backspace`: Membuang item terpilih langsung ke Virtual Trash.
- Pindahkan file antar folder dengan mudah (dilengkapi proteksi anti-siklus agar folder tidak dapat dipindahkan ke dalam dirinya sendiri atau subfoldernya).
- Ganti nama (*rename*) atau hapus file yang sudah tidak diperlukan; berkas yang dihapus masuk ke **Virtual Trash**.

#### Langkah 5: Berbagi Link Publik & Berbagi Folder Virtual
Ingin membagikan file atau satu folder penuh kepada orang lain tanpa memberi akun admin?
1. Buka menu aksi berkas atau folder (`⋮`) dan pilih **Create Public Share Link**.
2. **Opsi Batas Waktu Kedaluwarsa yang Luas**:
   - Default: **Selamanya / Never expires**.
   - Pilihan cepat: **1 jam**, **1 hari**, **7 hari**, **90 hari**, **1 tahun**.
   - Kustom: Tentukan tanggal dan jam kedaluwarsa secara spesifik melalui pemilih tanggal (*date picker*).
3. Anda dapat menambahkan **password** (diamankan dengan hashing bcrypt) untuk akses terproteksi.
4. **Berbagi Folder Virtual (Jailed Guest Traversal)**:
   - Saat membagikan folder, pengunjung umum mendapatkan tampilan penjelajah berkas yang terisolasi (*jailed*) hanya pada folder tersebut dan subfolder di dalamnya.
   - Pengunjung dapat menelusuri subfolder melalui breadcrumb, serta mengunduh atau memutar (*streaming*) berkas di dalam folder berbagi.
   - Pengunjung dijamin secara aman tidak dapat keluar atau mengakses hierarki folder di luar folder yang dibagikan.
5. Salin URL publik (`/s/:token`) atau klik tombol **QR Code** agar rekan Anda bisa langsung memindai tautan melalui kamera ponsel.
6. Anda dapat memantau jumlah unduhan atau mencabut (*revoke*) link berbagi kapan saja melalui tab **Shared Links**.

#### Langkah 6: Snapshot Database & Pemulihan Point-in-Time
Seluruh hierarki folder dan penunjuk file tersimpan di database SQLite. TeleDrive menyediakan sistem pencadangan terintegrasi:
1. Buka menu **Snapshots** di sidebar.
2. Klik **Create Snapshot Now**: TeleDrive mengunci WAL SQLite, membuat snapshot `.db.gz` yang konsisten, dan mengunggahnya ke channel Telegram Anda.
3. **Retensi Bergulir (Rolling Retention)**: TeleDrive otomatis mempertahankan **5 snapshot terbaru** dan menghapus cadangan yang lebih lama agar channel tetap rapi dan tidak boros ruang.
4. **Point-in-Time Restore**: Klik tombol **Restore** pada snapshot tanggal tertentu untuk memulihkan seluruh struktur data secara instan tanpa perlu mematikan aplikasi.
5. **Cadangan Offline**: Unduh langsung file `.db.gz` ke laptop/PC Anda, atau gunakan fitur **Upload & Restore** untuk memulihkan database dari file cadangan lokal saat berpindah komputer.

#### Langkah 7: Pasang TeleDrive sebagai Network Drive Komputer (WebDAV)
TeleDrive menyediakan gateway WebDAV terintegrasi. Anda dapat memasang (*mount*) cloud storage virtual Anda langsung sebagai drive lokal di File Explorer atau Finder tanpa perlu aplikasi sinkronisasi pihak ketiga:

- **URL WebDAV**: `http://localhost:8080/webdav`
- **Username**: `admin`
- **Password**: Password admin Anda (`TELEDRIVE_ADMIN_PASSWORD`, default: `admin123`)

##### Windows 10/11 File Explorer & Command Prompt
1. Buka **File Explorer**, klik kanan pada **This PC** (atau klik menu `...`).
2. Pilih **Map network drive...**.
3. Pilih huruf Drive (misal `Z:`).
4. Pada kolom **Folder**, masukkan salah satu format alamat berikut:
   - Format URL standar: `http://localhost:8080/webdav` (atau `http://192.168.x.x:8080/webdav`)
   - Format UNC resmi Windows (sangat disarankan untuk port atau IP non-standar): `\\192.168.x.x@8080\webdav`
5. Centang **Connect using different credentials**, lalu klik **Finish**.
6. Masukkan user `admin` dan password admin Anda.

*Atau melalui Terminal (CMD / PowerShell):*
```cmd
net use Z: \\localhost@8080\webdav /user:admin admin123 /persistent:yes
```

> ⚠️ **Solusi Error Windows ("A device attached to the system is not functioning" / Error 0x8007001F)**:
> Secara default, Windows WebClient memblokir autentikasi Basic Auth melalui HTTP biasa (non-SSL). Jika Windows menolak terhubung, buka **PowerShell sebagai Administrator** dan jalankan:
> ```powershell
> Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" -Name "BasicAuthLevel" -Value 2 -Type DWord
> Restart-Service WebClient
> ```

##### macOS Finder
1. Buka Finder, tekan `Cmd + K` (atau menu **Go** > **Connect to Server...**).
2. Masukkan alamat: `http://localhost:8080/webdav`, klik **Connect**.
3. Pilih **Registered User**, masukkan nama `admin` dan password Anda.

##### Linux (davfs2 & rclone)
```bash
# Menggunakan davfs2:
sudo mount -t davfs http://localhost:8080/webdav /mnt/teledrive -o username=admin

# Menggunakan rclone:
rclone config # pilih WebDAV, URL http://localhost:8080/webdav, vendor other
rclone mount teledrive: /mnt/teledrive --vfs-cache-mode writes
```

#### Langkah 8: Virtual Trash & Pemulihan Berkas Terhapus
TeleDrive mengedepankan keamanan data dengan sistem penampungan sementara (*soft-delete*):
1. **Soft-Delete**: Saat Anda menghapus file atau folder dari web atau WebDAV, data tidak langsung dimusnahkan, melainkan diberi tanda `deleted_at` dan dipindahkan ke **Virtual Trash**.
2. **Pemulihan Instan (Restore)**: Buka tab **Trash** di dashboard web untuk memulihkan berkas atau folder kembali ke lokasi asalnya hanya dengan satu klik.
3. **Kosongkan Sampah (Empty Trash)**: Menghapus seluruh item di tempat sampah secara permanen sekaligus memicu perintah penghapusan pesan (*DeleteMessages*) di channel privat Telegram Anda.
4. **Pembersihan Otomatis 30 Hari**: Worker latar belakang TeleDrive akan otomatis menghapus item tempat sampah yang usianya sudah melewati 30 hari secara berkala setiap 24 jam.

---

### 5. Keamanan & Kepatuhan Safe Mode

TeleDrive dirancang khusus dengan sistem pertahanan berlapis agar akun utama Telegram Anda tetap aman dan terbebas dari sanksi/banned:

1. **Emulasi Telemetri Resmi**: TeleDrive menggunakan identitas klien resmi Telegram Desktop (`PC 64bit`, `Linux/x86_64`, `AppVersion 5.0.0`).
2. **Antrean Sekuensial**: Proses upload dan download berjalan strictly 1 antrean dalam satu waktu, persis seperti kebiasaan manusia saat menggunakan aplikasi desktop resmi.
3. **Pacing Delay**: Jeda adaptif sebesar 30ms diterapkan di antara bagian 512 KB untuk menjaga koneksi tetap stabil dan tidak dianggap aktivitas spamming.
4. **Penanganan Otomatis Flood Wait**: Jika Telegram mengirim sinyal `FLOOD_WAIT_X`, TeleDrive akan otomatis menunggu durasi jeda yang diminta tanpa melakukan serangan permintaan ulang (*hammering*).
5. **Enkripsi Kunci Sesi (AES-256-GCM)**: Kunci otentikasi sesi Telegram MTProto dienkripsi menggunakan AES-256-GCM sebelum disimpan di database lokal.
6. **Channel Pribadi Terisolasi**: File tersimpan di channel private dengan 0 anggota luar, sehingga file Anda tidak dapat diakses atau dicari oleh pengguna Telegram lain.
7. **Enkripsi Stream Zero-Knowledge (AES-CTR)**: Setiap part file dienkripsi dengan stream cipher AES-CTR sebelum dikirim ke Telegram. Server Telegram hanya menyimpan data acak (ciphertext), sementara fitur pemutaran video Range Request $O(1)$ tetap instan.
8. **Token Sesi Bertanda Tangan (HMAC-SHA256)**: Sesi web diamankan dengan cookie bertanda tangan kriptografis tahan manipulasi dengan masa kedaluwarsa 30 hari serta flag `HttpOnly` dan `SameSite=Lax`.

---

### 6. Tanya Jawab Umum (FAQ)

#### T: Apakah file saya bisa dilihat orang lain di Telegram?
Tidak. Semua file disimpan di channel pribadi milik Anda sendiri (`TeleDrive Vault`). Ditambah lagi dengan enkripsi stream Zero-Knowledge, pihak Telegram maupun siapa pun tidak dapat melihat isi file Anda.

#### T: Apakah saya bisa memutar video atau membuka dokumen langsung dari drive WebDAV?
Bisa! Semua aplikasi yang mendukung WebDAV atau drive lokal (seperti VLC Player, pemutar musik, LibreOffice, atau script backup) dapat langsung membaca dan menulis file tanpa kendala.

#### T: Bagaimana pengaruh Virtual Trash terhadap kuota atau penyimpanan di Telegram?
File yang ada di Virtual Trash masih tersimpan di Telegram sampai Anda mengklik **Empty Trash** atau dibersihkan otomatis oleh worker setelah 30 hari. Ketika dibersihkan, TeleDrive akan memanggil API Telegram untuk menghapus pesan terkait secara permanen.

#### T: Bagaimana jika komputer/VPS saya rusak atau saya ingin pindah ke PC baru?
Sangat mudah dan otomatis! Karena database metadata SQLite Anda dicadangkan ke channel Telegram:
1. Jalankan `./teledrive login` di PC baru.
2. TeleDrive secara otomatis mendeteksi channel `TeleDrive Vault` lama Anda dan menawarkan opsi untuk langsung memulihkan snapshot database terbaru.
3. Tekan `[Y]`, seluruh struktur folder dan file Anda akan kembali seperti semula seketika!

#### T: Bagaimana cara mengganti password admin web?
Cukup atur variabel lingkungan `TELEDRIVE_ADMIN_PASSWORD` sebelum menjalankan server:
```bash
export TELEDRIVE_ADMIN_PASSWORD="password_baru_anda"
./teledrive server
```
