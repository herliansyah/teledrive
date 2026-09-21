package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/net/webdav"
	"teledrive/internal/crypto"
	"teledrive/internal/db"
)

// vfsFileInfo implements os.FileInfo for virtual folders and files.
type vfsFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi vfsFileInfo) Name() string       { return fi.name }
func (fi vfsFileInfo) Size() int64        { return fi.size }
func (fi vfsFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi vfsFileInfo) ModTime() time.Time { return fi.modTime }
func (fi vfsFileInfo) IsDir() bool        { return fi.isDir }
func (fi vfsFileInfo) Sys() any           { return nil }

// TeleDriveFS implements webdav.FileSystem bridging OS requests to TeleDrive SQLite metadata
// and Telegram MTProto chunked storage.
type TeleDriveFS struct {
	server *Server
}

func (s *Server) newWebDAVFS() webdav.FileSystem {
	return &TeleDriveFS{server: s}
}

func (fs *TeleDriveFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	cleaned := path.Clean("/" + name)
	if cleaned == "/" || cleaned == "." {
		return vfsFileInfo{
			name:    "/",
			size:    0,
			mode:    0755 | os.ModeDir,
			modTime: time.Now(),
			isDir:   true,
		}, nil
	}

	folder, file, err := fs.server.db.ResolvePath(cleaned)
	if err != nil {
		return nil, os.ErrNotExist
	}

	if folder != nil {
		return vfsFileInfo{
			name:    folder.Name,
			size:    0,
			mode:    0755 | os.ModeDir,
			modTime: folder.UpdatedAt,
			isDir:   true,
		}, nil
	}

	if file != nil {
		return vfsFileInfo{
			name:    file.Name,
			size:    file.Size,
			mode:    0644,
			modTime: file.UpdatedAt,
			isDir:   false,
		}, nil
	}

	return nil, os.ErrNotExist
}

func (fs *TeleDriveFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	cleaned := path.Clean("/" + name)

	// Opening root directory
	if cleaned == "/" || cleaned == "." {
		return &vfsDirFile{
			fs:       fs,
			folderID: nil,
			info: vfsFileInfo{
				name:    "/",
				size:    0,
				mode:    0755 | os.ModeDir,
				modTime: time.Now(),
				isDir:   true,
			},
		}, nil
	}

	folder, file, err := fs.server.db.ResolvePath(cleaned)

	// Writing a new file or overwriting
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) != 0 {
		// Resolve parent folder
		parentPath := path.Dir(cleaned)
		var parentFolderID *string
		if parentPath != "/" && parentPath != "." {
			pFolder, _, pErr := fs.server.db.ResolvePath(parentPath)
			if pErr != nil || pFolder == nil {
				return nil, os.ErrNotExist
			}
			parentFolderID = &pFolder.ID
		}

		baseName := path.Base(cleaned)
		tmpFile, tmpErr := os.CreateTemp("", "teledrive-webdav-*")
		if tmpErr != nil {
			return nil, tmpErr
		}

		return &vfsFileWriter{
			fs:             fs,
			parentFolderID: parentFolderID,
			name:           baseName,
			tempFile:       tmpFile,
		}, nil
	}

	if err != nil {
		return nil, os.ErrNotExist
	}

	// Opening a virtual folder
	if folder != nil {
		return &vfsDirFile{
			fs:       fs,
			folderID: &folder.ID,
			info: vfsFileInfo{
				name:    folder.Name,
				size:    0,
				mode:    0755 | os.ModeDir,
				modTime: folder.UpdatedAt,
				isDir:   true,
			},
		}, nil
	}

	// Opening an existing file for reading
	if file != nil {
		return &vfsFileReader{
			fs:     fs,
			file:   file,
			ctx:    ctx,
			offset: 0,
		}, nil
	}

	return nil, os.ErrNotExist
}

func (fs *TeleDriveFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	cleaned := path.Clean("/" + name)
	parentPath := path.Dir(cleaned)
	baseName := path.Base(cleaned)

	var parentFolderID *string
	if parentPath != "/" && parentPath != "." {
		parentFolder, _, err := fs.server.db.ResolvePath(parentPath)
		if err != nil || parentFolder == nil {
			return os.ErrNotExist
		}
		parentFolderID = &parentFolder.ID
	}

	_, err := fs.server.db.CreateFolder(baseName, parentFolderID)
	return err
}

func (fs *TeleDriveFS) RemoveAll(ctx context.Context, name string) error {
	cleaned := path.Clean("/" + name)
	folder, file, err := fs.server.db.ResolvePath(cleaned)
	if err != nil {
		return os.ErrNotExist
	}

	if folder != nil {
		return fs.server.db.SoftDeleteFolder(folder.ID)
	}
	if file != nil {
		return fs.server.db.SoftDeleteFile(file.ID)
	}

	return nil
}

func (fs *TeleDriveFS) Rename(ctx context.Context, oldName, newName string) error {
	oldClean := path.Clean("/" + oldName)
	newClean := path.Clean("/" + newName)

	folder, file, err := fs.server.db.ResolvePath(oldClean)
	if err != nil {
		return os.ErrNotExist
	}

	newParentPath := path.Dir(newClean)
	newBaseName := path.Base(newClean)

	var newParentID *string
	if newParentPath != "/" && newParentPath != "." {
		pFolder, _, pErr := fs.server.db.ResolvePath(newParentPath)
		if pErr != nil || pFolder == nil {
			return os.ErrNotExist
		}
		newParentID = &pFolder.ID
	}

	if folder != nil {
		if folder.Name != newBaseName {
			_ = fs.server.db.RenameFolder(folder.ID, newBaseName)
		}
		return fs.server.db.MoveFolder(folder.ID, newParentID)
	}

	if file != nil {
		if file.Name != newBaseName {
			_ = fs.server.db.RenameFile(file.ID, newBaseName)
		}
		return fs.server.db.MoveFile(file.ID, newParentID)
	}

	return os.ErrNotExist
}

// vfsDirFile implements webdav.File for directories.
type vfsDirFile struct {
	fs          *TeleDriveFS
	folderID    *string
	info        vfsFileInfo
	readEntries []os.FileInfo
	readPos     int
}

func (d *vfsDirFile) Close() error { return nil }
func (d *vfsDirFile) Stat() (os.FileInfo, error) { return d.info, nil }
func (d *vfsDirFile) Write(p []byte) (int, error) { return 0, os.ErrPermission }
func (d *vfsDirFile) Read(p []byte) (int, error) { return 0, io.EOF }
func (d *vfsDirFile) Seek(offset int64, whence int) (int64, error) { return 0, os.ErrInvalid }

func (d *vfsDirFile) Readdir(count int) ([]os.FileInfo, error) {
	if d.readEntries == nil {
		var entries []os.FileInfo
		subfolders, err := d.fs.server.db.ListFolders(d.folderID)
		if err == nil {
			for _, sf := range subfolders {
				entries = append(entries, vfsFileInfo{
					name:    sf.Name,
					size:    0,
					mode:    0755 | os.ModeDir,
					modTime: sf.UpdatedAt,
					isDir:   true,
				})
			}
		}

		files, err := d.fs.server.db.ListFiles(d.folderID)
		if err == nil {
			for _, fl := range files {
				entries = append(entries, vfsFileInfo{
					name:    fl.Name,
					size:    fl.Size,
					mode:    0644,
					modTime: fl.UpdatedAt,
					isDir:   false,
				})
			}
		}
		d.readEntries = entries
		d.readPos = 0
	}

	if count <= 0 {
		res := d.readEntries[d.readPos:]
		d.readPos = len(d.readEntries)
		return res, nil
	}

	if d.readPos >= len(d.readEntries) {
		return nil, io.EOF
	}

	end := d.readPos + count
	if end > len(d.readEntries) {
		end = len(d.readEntries)
	}

	res := d.readEntries[d.readPos:end]
	d.readPos = end
	return res, nil
}

// vfsFileReader implements webdav.File for streaming existing files.
type vfsFileReader struct {
	fs     *TeleDriveFS
	file   *db.File
	ctx    context.Context
	offset int64
}

func (r *vfsFileReader) Close() error { return nil }
func (r *vfsFileReader) Write(p []byte) (int, error) { return 0, os.ErrPermission }
func (r *vfsFileReader) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (r *vfsFileReader) Stat() (os.FileInfo, error) {
	return vfsFileInfo{
		name:    r.file.Name,
		size:    r.file.Size,
		mode:    0644,
		modTime: r.file.UpdatedAt,
		isDir:   false,
	}, nil
}

func (r *vfsFileReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		newOffset = r.file.Size + offset
	default:
		return 0, os.ErrInvalid
	}

	if newOffset < 0 {
		return 0, os.ErrInvalid
	}
	r.offset = newOffset
	return newOffset, nil
}

func (r *vfsFileReader) Read(p []byte) (int, error) {
	if r.offset >= r.file.Size {
		return 0, io.EOF
	}

	toRead := int64(len(p))
	if r.offset+toRead > r.file.Size {
		toRead = r.file.Size - r.offset
	}

	end := r.offset + toRead - 1
	docID, _ := strconv.ParseInt(r.file.TelegramFileID, 10, 64)
	docHash, _ := strconv.ParseInt(r.file.TelegramAccessHash, 10, 64)

	var buf bytes.Buffer
	var dErr error

	if r.file.IsEncrypted == 1 {
		key, iv := crypto.DeriveFileStreamKeyAndIV(r.fs.server.cfg.SecretKey, r.file.ID)
		dErr = r.fs.server.tg.DownloadRange(r.ctx, docID, docHash, r.offset, end, &buf, r.fs.server.limiter, func(data []byte, off int64) ([]byte, error) {
			return crypto.TransformBytes(data, key, iv, off)
		})
	} else {
		dErr = r.fs.server.tg.DownloadRange(r.ctx, docID, docHash, r.offset, end, &buf, r.fs.server.limiter)
	}

	if dErr != nil {
		return 0, dErr
	}

	n := copy(p, buf.Bytes())
	r.offset += int64(n)

	if r.offset >= r.file.Size {
		return n, io.EOF
	}

	return n, nil
}

// vfsFileWriter implements webdav.File for saving new files.
type vfsFileWriter struct {
	fs             *TeleDriveFS
	parentFolderID *string
	name           string
	tempFile       *os.File
}

func (w *vfsFileWriter) Stat() (os.FileInfo, error) {
	return w.tempFile.Stat()
}

func (w *vfsFileWriter) Read(p []byte) (int, error) {
	return w.tempFile.Read(p)
}

func (w *vfsFileWriter) Seek(offset int64, whence int) (int64, error) {
	return w.tempFile.Seek(offset, whence)
}

func (w *vfsFileWriter) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (w *vfsFileWriter) Write(p []byte) (int, error) {
	return w.tempFile.Write(p)
}

func (w *vfsFileWriter) Close() error {
	defer func() {
		_ = w.tempFile.Close()
		_ = os.Remove(w.tempFile.Name())
	}()

	info, err := w.tempFile.Stat()
	if err != nil {
		return err
	}

	size := info.Size()
	_, _ = w.tempFile.Seek(0, io.SeekStart)

	// If telegram client is not active (e.g. during offline testing), record empty mock file
	if w.fs.server.tg == nil {
		_, err = w.fs.server.db.CreateFile(w.parentFolderID, w.name, size, "application/octet-stream", 0, "mock_0", "mock_0", "")
		return err
	}

	fileUID := db.GenerateID()
	encKey, encIV := crypto.DeriveFileStreamKeyAndIV(w.fs.server.cfg.SecretKey, fileUID)
	encReader, encErr := crypto.EncryptStream(w.tempFile, encKey, encIV, 0)
	if encErr != nil {
		return encErr
	}

	mimeType := mime.TypeByExtension(filepath.Ext(w.name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ctx := context.Background()
	return w.fs.server.tg.Run(ctx, func(runCtx context.Context) error {
		if err := w.fs.server.tg.EnsureStorageChannel(runCtx); err != nil {
			return err
		}

		msgID, docID, accessHash, shaHex, uErr := w.fs.server.tg.UploadFromReader(runCtx, encReader, size, w.name, mimeType, w.fs.server.limiter, nil)
		if uErr != nil {
			return uErr
		}

		_, err = w.fs.server.db.CreateFileWithID(
			fileUID,
			w.parentFolderID,
			w.name,
			size,
			mimeType,
			msgID,
			strconv.FormatInt(docID, 10),
			strconv.FormatInt(accessHash, 10),
			shaHex,
			1,
		)
		return err
	})
}
