<div align="center">

# ⚡ TeleDrive

**High-Capacity Cloud Storage Powered by Telegram MTProto Infrastructure**

*Turn your Telegram account into a secure, unlimited personal cloud drive with a Google Drive-like Web Dashboard and CLI.*

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Author](https://img.shields.io/badge/Author-Herliansyah-purple?style=flat&logo=github)](https://github.com/herliansyah)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![npm version](https://img.shields.io/npm/v/teledrive.svg?style=flat&logo=npm)](https://www.npmjs.com/package/teledrive)
[![GitHub](https://img.shields.io/badge/GitHub-herliansyah%2Fteledrive-181717?style=flat&logo=github)](https://github.com/herliansyah/teledrive)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-blue)](https://github.com/herliansyah/teledrive)
[![Pure Go](https://img.shields.io/badge/CGO-Zero%20(Pure%20Go)-orange)](https://modernc.org/sqlite)

[English](#english) &bull; [Bahasa Indonesia](#bahasa-indonesia)

---

</div>

<a name="english"></a>
## 🌟 English Overview

**TeleDrive** is a lightweight, single-binary personal cloud storage bridge. It uses Telegram's distributed cloud infrastructure as an object store via native **MTProto** (`gotd/td`), providing up to **2 GB (Free accounts)** or **4 GB (Premium accounts)** per file transfer, without intermediate disk spooling on your server.

> [!WARNING]
> **Disclaimer & Safe Usage Notice**
> - **Use a Secondary Account**: It is strongly recommended to use a secondary or dedicated Telegram account rather than your primary personal account.
> - **Use at Your Own Risk**: TeleDrive is provided for educational and personal use "as is". All risks—including account limitations, temporary or permanent bans by Telegram, or data loss—are solely the user's personal responsibility.
> - **Telegram Terms of Service**: While TeleDrive incorporates safety mechanisms (sequential queues, delay pacing, and automated flood backoff), heavy cloud storage usage may conflict with Telegram's Terms of Service.
> - **Not Affiliated with Telegram**: This project is independent and open-source. It is not affiliated with, endorsed, or sponsored by Telegram FZ-LLC.

### ✨ Key Features

- **🎨 Modern Zero-Build Web Interface**:
  - Dark & Light mode toggle with persistent state in local storage.
  - Dual-view explorer: instant toggle between **Grid View** (visual file cards) and **Table/List View** with instant in-memory column sorting (Name, Size, Modified Date).
  - Floating Upload Manager Drawer at bottom-right with real-time chunk progress (`Chunk 3/8`), minimizable pill state, and sequential safe-mode processing.
  - Crisp embedded Lucide SVG vector icons with zero additional network requests.
  - In-app non-intrusive toast notifications and custom modal dialogs (replacing native browser alerts/prompts).
  - Shared Links Management view in dashboard to audit active links, copy URLs, review downloads, and revoke access.
  - Expanded media and code viewer: seekable video (HTTP 206), audio, image, PDF, and syntax/plain text viewer for code files (`.txt`, `.md`, `.json`, `.go`, `.py`, `.log`).
  - Built-in **Mobile QR Code Generator** on public share links for frictionless smartphone handoff.
- **📁 Virtual File System & Organization**:
  - Full hierarchical folder management backed by SQLite with WAL mode.
  - Native HTML5 **Drag-and-Drop** to move files and folders directly into folders or breadcrumb trails.
  - **Multi-Select & Bulk Operations**: Select multiple items with floating action bar for batch move and batch trash.
  - Interactive "Move to..." destination picker dialog and keyboard shortcuts (`F2` to rename, `Delete` to trash).
- **⚡ Resumable Chunked Upload**: Browser slices files into 5 MB chunks and streams them directly into 512 KB MTProto parts with zero VPS disk wear.
- **🎬 Instant Media Streaming (HTTP 206 Range)**: Stream and seek large videos, audio, images, and PDFs in real-time without downloading the complete file first.
- **🔗 Secure Public Share Links & Folder Sharing**:
  - Share individual files or entire **Virtual Folders** (`/s/:token`) with secure jailed guest traversal and in-folder file streaming/downloads.
  - Flexible expiration options (Default "Never expires", 90 days, 1 year presets, or custom date/time picker).
  - Optional bcrypt password protection and built-in **Mobile QR Code Generator**.
- **🛡️ Primary Account Safe Mode (Strict Telegram ToS Compliance)**:
  - Realistic official desktop client telemetry fingerprinting (`PC 64bit`, `Linux/x86_64`, `AppVersion 5.0.0`).
  - Strict sequential single-worker queue (concurrency = 1) mimicking human desktop usage.
  - Adaptive 30ms pacing delay between 512 KB parts.
  - Automated defensive `FLOOD_WAIT_X` backoff without crashing or retrying aggressively.
  - Strictly private Storage Channel (`TeleDrive Vault`) with zero external members.
- **🔒 Zero-Knowledge Part Encryption**: File parts are encrypted with **AES-CTR** seekable stream cipher before dispatch to Telegram, ensuring zero-trust storage with instantaneous $O(1)$ video seeking and zero size expansion.
- **🗄️ Native WebDAV Gateway (`/webdav`)**: Mount TeleDrive directly as an operating system drive in Windows Explorer, macOS Finder, Linux (`davfs2`), or `rclone` with standard HTTP Basic Auth.
- **🗑️ Virtual Trash (Soft-Delete & Auto-Purge)**: Accidental file and folder deletions are safely staged in Virtual Trash with instant restore, empty trash, and an automated 30-day background purge worker.
- **🔑 Signed Session Tokens**: Tamper-proof **HMAC-SHA256** signed session cookies with 30-day validity, constant-time comparison, and HTTP-only protections.
- **🔐 Military-Grade Security at Rest**: Telegram MTProto session strings (`auth_key`) are encrypted at rest in SQLite using **AES-256-GCM** with SHA-256 key derivation.
- **💾 Automated Snapshots & Point-in-Time Restore**: Online SQLite snapshots stored directly in your private Telegram Storage Channel with 5-snapshot rolling retention, 24-hour background scheduler, automated snapshot on graceful shutdown, web-based point-in-time restore, direct `.db.gz` offsite downloads, and local upload-and-restore.

---

### 📥 Pre-built Binary Downloads

Download the latest pre-compiled binary for your operating system:

| Operating System | Architecture | Binary Archive |
| :--- | :--- | :--- |
| **Linux** | x86_64 (amd64) | [`teledrive-v1.0.0-linux-amd64.tar.gz`](dist/teledrive-v1.0.0-linux-amd64.tar.gz) |
| **Linux** | ARM64 (aarch64) | [`teledrive-v1.0.0-linux-arm64.tar.gz`](dist/teledrive-v1.0.0-linux-arm64.tar.gz) |
| **macOS** | Apple Silicon (arm64) | [`teledrive-v1.0.0-darwin-arm64.tar.gz`](dist/teledrive-v1.0.0-darwin-arm64.tar.gz) |
| **macOS** | Intel (amd64) | [`teledrive-v1.0.0-darwin-amd64.tar.gz`](dist/teledrive-v1.0.0-darwin-amd64.tar.gz) |
| **Windows** | x86_64 (amd64) | [`teledrive-v1.0.0-windows-amd64.tar.gz`](dist/teledrive-v1.0.0-windows-amd64.tar.gz) |

*Checksums are verified in [`sha256sums.txt`](dist/sha256sums.txt).*

---

### 🚀 Quick Start Guide

#### ⚡ Run Instantly with `npx` (No installation or compiler required!)

If you have Node.js installed, you can launch TeleDrive directly without downloading binaries or installing Go:

```bash
# Pair your Telegram account:
npx teledrive login

# Launch the Web Dashboard:
npx teledrive server

# Or install globally as a system command:
npm install -g teledrive
```

---

#### 📦 Or Run the Pre-compiled Binary Directly

##### 1. Setup & Telegram Pairing (First Time Only)

Obtain your personal `API_ID` and `API_HASH` from [my.telegram.org](https://my.telegram.org) (Application Type: Desktop). Then run the interactive terminal wizard:

```bash
./teledrive login
```

<div align="center">
  <img src="docs/assets/teledrive-login.png" alt="TeleDrive Login Terminal Wizard" width="600" style="border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15); margin: 12px 0;">
</div>

The wizard will guide you through:
1. Entering your `API_ID` and `API_HASH`.
2. Entering your Telegram phone number.
3. Submitting the confirmation code sent to your Telegram app.
4. Submitting your 2FA Cloud Password (if enabled).
5. Auto-creating the private `TeleDrive Vault` storage channel.

> 💡 **New to TeleDrive?** Read the [Full User Guide & How It Works (docs/USER_GUIDE.md)](docs/USER_GUIDE.md) for detailed architecture explanations, visual walkthroughs, and security practices.

#### 2. Start the Web Dashboard

```bash
./teledrive server
```

Open your browser and navigate to:
👉 **`http://localhost:8080`**

- **Default Admin Password**: `admin123` (Change via `TELEDRIVE_ADMIN_PASSWORD`).
- **WebDAV Gateway**: Mount in Windows/macOS/Linux at `http://localhost:8080/webdav` (User: `admin`). See [docs/USER_GUIDE.md](docs/USER_GUIDE.md#step-7-mount-teledrive-as-a-local-network-drive-webdav) for mounting instructions.

---

### 💻 Command-Line Interface (CLI)

```bash
# Upload a file to TeleDrive
./teledrive upload ./sample_video.mp4

# Upload to a specific virtual folder
./teledrive upload ./report.pdf --folder <folder_id>

# Download a file back to your local machine
./teledrive download <file_id> --output ./restored_report.pdf

# List root folders and files
./teledrive list

# Create a full SQLite database snapshot and upload it to Telegram
./teledrive backup

# Restore SQLite database from the pinned snapshot in your Telegram channel
./teledrive restore
```

---

### ⚙️ Configuration & Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `TELEDRIVE_PORT` | `8080` | HTTP port for the web dashboard |
| `TELEDRIVE_DB_PATH` | `teledrive.db` | Path to the SQLite database file |
| `TELEDRIVE_ADMIN_PASSWORD` | `admin123` | Password for admin dashboard login |
| `TELEDRIVE_SECRET_KEY` | *(auto-generated)* | 32-byte secret key used for AES-256-GCM session encryption |
| `TELEDRIVE_TG_APP_ID` | *(prompted)* | Telegram App ID from `my.telegram.org` |
| `TELEDRIVE_TG_APP_HASH` | *(prompted)* | Telegram App Hash from `my.telegram.org` |

---

### 👤 Author & Credits

Developed and maintained by **Herliansyah**:
- **Creator Profile**: [@herliansyah](https://github.com/herliansyah)
- **Source Code**: [https://github.com/herliansyah/teledrive](https://github.com/herliansyah/teledrive)
- **License**: Released under the [MIT License](LICENSE). Open-source, free for personal and educational use.

<hr style="margin: 40px 0;">

<a name="bahasa-indonesia"></a>
## 🇮🇩 Panduan Bahasa Indonesia

**TeleDrive** adalah jembatan penyimpanan awan (*cloud storage*) berbasis Go (single binary) yang memanfaatkan infrastruktur Telegram sebagai backend penyimpanan melalui protokol resmi **MTProto** (`gotd/td`). Dengan TeleDrive, Anda dapat menikmati kapasitas upload hingga **2 GB (akun reguler)** atau **4 GB (akun Telegram Premium)** per file dengan tampilan web mirip Google Drive.

> [!WARNING]
> **Penafian & Batasan Tanggung Jawab (Disclaimer)**
> - **Gunakan Akun Sekunder / Cadangan**: Sangat disarankan untuk menggunakan akun Telegram sekunder (khusus) dan bukan akun pribadi utama Anda, guna menghindari dampak negatif jika terjadi pembatasan akun.
> - **Segala Risiko Ditanggung Pribadi**: Aplikasi ini disediakan untuk keperluan edukasi dan personal "sebagaimana adanya" (*as is*). Segala risiko akibat penggunaan TeleDrive—termasuk limitasi akun, pemblokiran/penangguhan (*banned*) oleh Telegram, pembatasan API, maupun kehilangan data—sepenuhnya merupakan tanggung jawab pribadi pengguna.
> - **Ketentuan Layanan (ToS) Telegram**: Meskipun TeleDrive dilengkapi fitur *Safe Mode* (antrean sekuensial tunggal, jeda adaptif, dan penanganan *flood wait* otomatis), penggunaan Telegram sebagai tempat penyimpanan file berkapasitas besar berpotensi bertentangan dengan Ketentuan Layanan Telegram.
> - **Bukan Produk Resmi Telegram**: TeleDrive adalah proyek sumber terbuka independen dan tidak terafiliasi, didukung, maupun disponsori oleh Telegram FZ-LLC.

### 🎯 Fitur Unggulan
 
- **Single Binary Portabel (Zero CGO)**: Kompilasi 100% Pure Go tanpa dependensi compiler C/gcc (`modernc.org/sqlite` dan `gotd/td`).
- **Antarmuka Web Modern (Zero-Build)**:
  - Dukungan Dark & Light Mode dengan toggle instan dan penyimpanan preferensi di browser.
  - Mode tampilan ganda: **Grid View** (kartu file interaktif) dan **Table/List View** (tabel detail dengan sorting instan Nama, Ukuran, dan Tanggal).
  - Floating Upload Manager Drawer di pojok kanan bawah dengan pemantauan per-chunk progresif (`Chunk 3/8`) dan antrean sekuensial aman dari *Flood Wait*.
  - Ikon vektor modern Lucide SVG resolusi tinggi tanpa penambahan request HTTP.
  - Notifikasi toast dan dialog modal elegan (tanpa `alert`, `confirm`, atau `prompt` bawaan browser).
  - Tab Manajemen Shared Links di dashboard untuk memantau, menyalin URL, dan mencabut (*revoke*) link berbagi aktif.
  - Penampil pratinjau media dan dokumen teks/kode (`.txt`, `.md`, `.json`, `.go`, `.py`, dll).
  - Fitur **QR Code Generator** pada link publik untuk unduhan instan langsung dari smartphone.
- **Struktur Folder Virtual & Pengorganisasian**:
  - Pengelolaan hierarki folder virtual berbasis SQLite WAL.
  - Fitur **Drag-and-Drop** berkas dan folder langsung ke baris/kartu folder atau ke *breadcrumb trail*.
  - **Multi-Pilih & Operasi Massal (Bulk Operations)**: Pilih banyak file/folder dengan bilah aksi melayang (*floating action bar*) untuk pemindahan massal (*batch move*) dan penghapusan massal (*batch trash*).
  - Dialog interaktif "Move to..." serta pintasan keyboard (`F2` untuk ganti nama, `Delete` untuk buang ke trash).
- **Upload Chunked Resumable**: File dipotong menjadi chunk 5MB di browser dan dialirkan langsung ke part 512KB MTProto tanpa memenuhi disk server VPS.
- **Streaming Media Langsung (HTTP 206)**: Menonton video besar, memutar audio, atau membuka dokumen PDF langsung di browser tanpa perlu mengunduh seluruh file terlebih dahulu.
- **Link Berbagi Publik & Berbagi Folder Virtual**:
  - Bagikan berkas individual maupun seluruh **Folder Virtual** (`/s/:token`) dengan penjelajahan subfolder guest yang terisolasi (*jailed*) serta streaming dan unduh berkas di dalamnya.
  - Opsi masa kedaluwarsa fleksibel (default Selamanya, preset 90 hari / 1 tahun, serta pemilih tanggal kustom).
  - Proteksi password (bcrypt) opsional dan fitur **QR Code Generator** terintegrasi.
- **Safe Mode Kepatuhan ToS (Aman untuk Akun Utama)**:
  - Menggunakan telemetri perangkat resmi (`PC 64bit`, `Linux/x86_64`, `AppVersion 5.0.0`).
  - Antrean upload strictly 1 koneksi aktif pada satu waktu (meniru aplikasi resmi Telegram Desktop).
  - Jeda adaptif 30ms antar-part untuk menjaga koneksi tetap dingin.
  - Penanganan jeda otomatis `FLOOD_WAIT` dari server Telegram.
  - Channel storage berstatus **Private** dan terisolasi (0 anggota luar).
- **Enkripsi File Zero-Knowledge**: Part data file dienkripsi dengan stream cipher **AES-CTR** yang dapat di-seek sebelum diunggah ke Telegram. Mendukung streaming video Range Request $O(1)$ tanpa penambahan ukuran file.
- **WebDAV Gateway Bawaan (`/webdav`)**: Pasang TeleDrive langsung sebagai Network Drive di Windows Explorer, macOS Finder, Linux (`davfs2`), atau `rclone` menggunakan HTTP Basic Auth bawaan.
- **Virtual Trash (Soft-Delete & Auto-Purge)**: Penghapusan file dan folder tidak langsung menghapus data fisik; item tersimpan aman di Virtual Trash dengan opsi pulihkan (*restore*), kosongkan (*empty trash*), dan pembersihan otomatis setelah 30 hari.
- **Signed Session Token**: Cookie sesi berbasis tanda tangan kriptografis **HMAC-SHA256** tahan manipulasi, masa berlaku 30 hari, dan terproteksi `HttpOnly`.
- **Keamanan Data at Rest**: Kunci session string MTProto dienkripsi menggunakan algoritma **AES-256-GCM** sebelum disimpan di database.
- **Disaster Recovery & Snapshots**: Snapshot online SQLite otomatis (setiap 24 jam dan saat shutdown) tersimpan di Telegram Storage Channel dengan retensi bergulir 5 snapshot, riwayat snapshot di dashboard web, pemulihan point-in-time, serta unduh dan unggah backup database lokal (`teledrive backup` & `teledrive restore`).

---

### 🚀 Cara Penggunaan Singkat

#### ⚡ Jalankan Instan dengan `npx` (Tanpa install Go / unduh manual)

Jika di komputer Anda sudah terpasang Node.js, Anda bisa langsung menjalankan TeleDrive dalam hitungan detik:

```bash
# Pasangkan akun Telegram Anda:
npx teledrive login

# Jalankan server Web Dashboard:
npx teledrive server

# Atau instal secara global di sistem:
npm install -g teledrive
```

---

#### 📦 Atau Jalankan Binary Kompilasi Langsung

##### 1. Pasangkan Akun Telegram Anda (Login Pertama Kali)
Dapatkan `API_ID` dan `API_HASH` Anda dari [my.telegram.org](https://my.telegram.org), lalu jalankan:
```bash
./teledrive login
```

<div align="center">
  <img src="docs/assets/teledrive-login.png" alt="Wizard Login Terminal TeleDrive" width="600" style="border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15); margin: 12px 0;">
</div>

Ikuti petunjuk di layar untuk memasukkan nomor telepon, kode OTP dari Telegram, dan password 2FA Anda.

> 💡 **Butuh panduan lengkap dan penjelasan cara kerja sistem?** Baca [Panduan Pengguna Lengkap & Cara Kerja (docs/USER_GUIDE.md)](docs/USER_GUIDE.md).

#### 2. Jalankan Server Web Dashboard
```bash
./teledrive server
```
Buka browser di **`http://localhost:8080`** (Password admin: `admin123`).
- **WebDAV Gateway**: Pasang sebagai drive lokal di Windows/macOS/Linux di `http://localhost:8080/webdav` (User: `admin`). Lihat [docs/USER_GUIDE.md](docs/USER_GUIDE.md#langkah-7-pasang-teledrive-sebagai-network-drive-komputer-webdav) untuk panduan pemasangan.

#### 3. Perintah Terminal (CLI)
```bash
# Mengunggah file
./teledrive upload ./laporan.pdf

# Menampilkan daftar file dan folder
./teledrive list

# Mengunduh kembali file ke komputer
./teledrive download <file_id> --output ./hasil.pdf

# Cadangkan database metadata ke Telegram
./teledrive backup

# Pulihkan database dari Telegram jika pindah server
./teledrive restore
```

---

### 📚 Dokumentasi Teknis Lanjutan

- [docs/USER_GUIDE.md](docs/USER_GUIDE.md) — Panduan lengkap pengguna, arsitektur, dan cara kerja TeleDrive.
- [CONTEXT.md](CONTEXT.md) — Glosarium kanonik konsep domain TeleDrive.
- [ARCHITECTURE.md](ARCHITECTURE.md) — Diagram alur data upload, streaming HTTP 206, dan skema SQLite.
- [SECURITY.md](SECURITY.md) — Panduan keamanan, enkripsi session, dan mitigasi banned Telegram.
- [MILESTONES.md](MILESTONES.md) — Rincian tahapan pengembangan dan verifikasi.
- [docs/adr/](docs/adr/) — Arsip Catatan Keputusan Arsitektur (*Architectural Decision Records*).

---

### 👤 Pembuat & Lisensi

Diciptakan dan dikembangkan oleh **Herliansyah**:
- **Profil GitHub**: [@herliansyah](https://github.com/herliansyah)
- **Repositori Resmi**: [https://github.com/herliansyah/teledrive](https://github.com/herliansyah/teledrive)
- **Lisensi**: Didistribusikan di bawah [Lisensi MIT](LICENSE). Bebas digunakan, dipelajari, dan dimodifikasi untuk keperluan personal maupun edukasi.

