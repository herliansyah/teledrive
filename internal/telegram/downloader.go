package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
)

// ParseRange parses an HTTP Range header (e.g. "bytes=0-1023", "bytes=100-", "bytes=-500").
// Returns start, end (inclusive), hasRange boolean, or an error if invalid.
func ParseRange(rangeHeader string, fileSize int64) (int64, int64, bool, error) {
	if rangeHeader == "" {
		return 0, fileSize - 1, false, nil
	}

	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, false, errors.New("invalid range unit")
	}

	raw := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return 0, 0, false, errors.New("invalid range format")
	}

	var start, end int64

	if parts[0] == "" {
		// Suffix range: bytes=-500
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLen <= 0 {
			return 0, 0, false, errors.New("invalid suffix range length")
		}
		if suffixLen > fileSize {
			suffixLen = fileSize
		}
		start = fileSize - suffixLen
		end = fileSize - 1
		return start, end, true, nil
	}

	s, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || s < 0 || s >= fileSize {
		return 0, 0, false, errors.New("range start out of bounds")
	}
	start = s

	if parts[1] == "" {
		// Open-ended range: bytes=100-
		end = fileSize - 1
	} else {
		e, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || e < start {
			return 0, 0, false, errors.New("invalid range end")
		}
		if e >= fileSize {
			e = fileSize - 1
		}
		end = e
	}

	return start, end, true, nil
}

// DownloadFull streams the complete document from Telegram to an io.Writer.
func (m *ClientManager) DownloadFull(ctx context.Context, docID, accessHash int64, w io.Writer) error {
	location := &tg.InputDocumentFileLocation{
		ID:         docID,
		AccessHash: accessHash,
	}

	d := downloader.NewDownloader()
	_, err := d.Download(m.api, location).Stream(ctx, w)
	return err
}

// DecryptorFunc defines a transformer that decrypts a chunk of bytes fetched at a specific offset.
type DecryptorFunc func(data []byte, offset int64) ([]byte, error)

// DownloadRange streams a specific byte range [start, end] from Telegram using aligned chunks.
func (m *ClientManager) DownloadRange(ctx context.Context, docID, accessHash int64, start, end int64, w io.Writer, limiter *SafeLimiter, decryptors ...DecryptorFunc) error {
	location := &tg.InputDocumentFileLocation{
		ID:         docID,
		AccessHash: accessHash,
	}

	const chunkSize = 512 * 1024 // 512 KB per MTProto getFile part
	alignedStart := (start / chunkSize) * chunkSize
	currOffset := alignedStart

	for currOffset <= end {
		if err := ctx.Err(); err != nil {
			return err
		}

		var res tg.UploadFileClass
		err := limiter.ExecuteWithFloodWait(ctx, func() error {
			var rErr error
			res, rErr = m.api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
				Location: location,
				Offset:   currOffset,
				Limit:    chunkSize,
			})
			return rErr
		})
		if err != nil {
			return fmt.Errorf("upload.getFile offset %d: %w", currOffset, err)
		}

		var bytesData []byte
		switch f := res.(type) {
		case *tg.UploadFile:
			bytesData = f.Bytes
		default:
			return fmt.Errorf("unsupported upload file response type: %T", res)
		}

		if len(bytesData) == 0 {
			break
		}

		if len(decryptors) > 0 && decryptors[0] != nil {
			decrypted, dErr := decryptors[0](bytesData, currOffset)
			if dErr != nil {
				return fmt.Errorf("decrypt range chunk offset %d: %w", currOffset, dErr)
			}
			bytesData = decrypted
		}

		// Calculate overlap with requested [start, end]
		chunkEnd := currOffset + int64(len(bytesData)) - 1
		sliceStart := int64(0)
		sliceEnd := int64(len(bytesData))

		if start > currOffset {
			sliceStart = start - currOffset
		}
		if end < chunkEnd {
			sliceEnd = int64(len(bytesData)) - (chunkEnd - end)
		}

		if sliceStart < sliceEnd {
			if _, err := w.Write(bytesData[sliceStart:sliceEnd]); err != nil {
				return err
			}
		}

		currOffset += int64(len(bytesData))
		if currOffset > end {
			break
		}

		_ = limiter.Pace(ctx)
	}

	return nil
}
