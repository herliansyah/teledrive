package crypto

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

func TestStreamCipher_RoundTripAndSeek(t *testing.T) {
	secretKey := "my-secret-key-32-chars-length!!"
	fileID := "file-abc-12345"

	key, iv := DeriveFileStreamKeyAndIV(secretKey, fileID)
	if len(key) != 32 || len(iv) != 16 {
		t.Fatalf("Expected key len 32 and iv len 16, got %d and %d", len(key), len(iv))
	}

	// Create 1 MB random payload (simulating 2 MTProto parts + extra)
	original := make([]byte, 1024*1024)
	if _, err := io.ReadFull(rand.Reader, original); err != nil {
		t.Fatalf("Failed to generate random test data: %v", err)
	}

	// 1. Encrypt whole payload sequentially
	encrypted, err := TransformBytes(original, key, iv, 0)
	if err != nil {
		t.Fatalf("TransformBytes encrypt failed: %v", err)
	}
	if len(encrypted) != len(original) {
		t.Fatalf("Expected exact 1-to-1 byte length %d, got %d", len(original), len(encrypted))
	}
	if bytes.Equal(encrypted, original) {
		t.Fatalf("Encrypted bytes should not equal original")
	}

	// 2. Decrypt whole payload from offset 0
	decrypted, err := TransformBytes(encrypted, key, iv, 0)
	if err != nil {
		t.Fatalf("TransformBytes decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, original) {
		t.Fatalf("Full decrypted payload does not match original")
	}

	// 3. Test Seekable Range Decryption (e.g. Range Request bytes 500,000 to 750,000)
	offsets := []int64{0, 15, 16, 17, 512 * 1024, 524300, 750123}
	sliceLengths := []int64{10, 50, 1024, 65536, 100000}

	for _, off := range offsets {
		for _, slen := range sliceLengths {
			if off+slen > int64(len(original)) {
				continue
			}

			cipherSlice := encrypted[off : off+slen]
			expectedPlain := original[off : off+slen]

			decryptedSlice, err := TransformBytes(cipherSlice, key, iv, off)
			if err != nil {
				t.Fatalf("TransformBytes at offset %d len %d failed: %v", off, slen, err)
			}

			if !bytes.Equal(decryptedSlice, expectedPlain) {
				t.Fatalf("Decrypted slice at offset %d (len %d) does NOT match original plain slice!", off, slen)
			}
		}
	}

	// 4. Test DecryptStreamWriter
	var buf bytes.Buffer
	sw, err := DecryptStreamWriter(&buf, key, iv, 512*1024)
	if err != nil {
		t.Fatalf("DecryptStreamWriter failed: %v", err)
	}
	part2Cipher := encrypted[512*1024 : 1024*1024]
	if _, err := sw.Write(part2Cipher); err != nil {
		t.Fatalf("Write to DecryptStreamWriter failed: %v", err)
	}
	expectedPart2 := original[512*1024 : 1024*1024]
	if !bytes.Equal(buf.Bytes(), expectedPart2) {
		t.Fatalf("DecryptStreamWriter output does not match expected second part")
	}

	// 5. Test EncryptStream and DecryptStream readers
	readerInput := bytes.NewReader(original)
	encR, err := EncryptStream(readerInput, key, iv, 0)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}
	encFromReader, err := io.ReadAll(encR)
	if err != nil || !bytes.Equal(encFromReader, encrypted) {
		t.Fatalf("EncryptStream output does not match expected encrypted bytes")
	}

	decR, err := DecryptStream(bytes.NewReader(encFromReader), key, iv, 0)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}
	decFromReader, err := io.ReadAll(decR)
	if err != nil || !bytes.Equal(decFromReader, original) {
		t.Fatalf("DecryptStream output does not match original bytes")
	}
}
