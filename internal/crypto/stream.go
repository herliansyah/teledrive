package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"io"
)

// ponytail: AES-CTR seekable stream cipher provides 1-to-1 byte parity without size expansion,
// maintaining strict Telegram MTProto 512 KB part alignment and O(1) HTTP 206 range-seeking.
// Ceiling: stream cipher without per-part MAC relies on MTProto transport checksums for integrity.
// Upgrade path: add header payload MAC if zero-trust against Telegram storage modification is required.

// DeriveFileStreamKeyAndIV derives a 32-byte AES key and 16-byte base IV
// deterministically from the instance secret key and the unique file ID.
func DeriveFileStreamKeyAndIV(secretKey, fileID string) ([]byte, []byte) {
	keyHash := sha256.Sum256([]byte(secretKey + ":key:" + fileID))
	ivHash := sha256.Sum256([]byte(secretKey + ":iv:" + fileID))
	return keyHash[:], ivHash[:16]
}

// addCounter increments a 16-byte base IV by blocks (treating it as a big-endian 128-bit int).
func addCounter(baseIV []byte, blocks uint64) []byte {
	iv := make([]byte, 16)
	copy(iv, baseIV)
	carry := blocks
	for i := 15; i >= 0 && carry > 0; i-- {
		sum := uint64(iv[i]) + (carry & 0xFF)
		iv[i] = byte(sum)
		carry = (carry >> 8) + (sum >> 8)
	}
	return iv
}

// NewSeekableCipher creates an AES-CTR cipher.Stream positioned at an exact byte offset.
func NewSeekableCipher(key, baseIV []byte, offset int64) (cipher.Stream, error) {
	if len(key) != 32 || len(baseIV) != 16 {
		return nil, fmt.Errorf("invalid key (len %d) or iv (len %d)", len(key), len(baseIV))
	}
	if offset < 0 {
		return nil, fmt.Errorf("negative stream offset %d", offset)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}

	blockIndex := uint64(offset / 16)
	remainder := int(offset % 16)

	iv := addCounter(baseIV, blockIndex)
	stream := cipher.NewCTR(block, iv)

	if remainder > 0 {
		var discard [16]byte
		stream.XORKeyStream(discard[:remainder], discard[:remainder])
	}

	return stream, nil
}

// TransformBytes transforms data in-place or returns transformed copy using AES-CTR at offset.
func TransformBytes(data []byte, key, baseIV []byte, offset int64) ([]byte, error) {
	stream, err := NewSeekableCipher(key, baseIV, offset)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	stream.XORKeyStream(out, data)
	return out, nil
}

// DecryptStreamWriter wraps an io.Writer so that all bytes written to it are decrypted
// with AES-CTR starting at offset.
func DecryptStreamWriter(w io.Writer, key, baseIV []byte, offset int64) (io.Writer, error) {
	stream, err := NewSeekableCipher(key, baseIV, offset)
	if err != nil {
		return nil, err
	}
	return &cipher.StreamWriter{
		S: stream,
		W: w,
	}, nil
}

// EncryptStream wraps an io.Reader so that all bytes read from it are encrypted with AES-CTR.
func EncryptStream(r io.Reader, key, baseIV []byte, offset int64) (io.Reader, error) {
	stream, err := NewSeekableCipher(key, baseIV, offset)
	if err != nil {
		return nil, err
	}
	return &cipher.StreamReader{
		S: stream,
		R: r,
	}, nil
}

// DecryptStream is identical to EncryptStream as AES-CTR is symmetric.
func DecryptStream(r io.Reader, key, baseIV []byte, offset int64) (io.Reader, error) {
	return EncryptStream(r, key, baseIV, offset)
}
