package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ponytail: stateless HMAC session token without server-side revocation list.
// Ceiling: cannot revoke an individual active token without rotating secretKey.
// Upgrade path: store jti/token revocation list in SQLite settings if per-session revocation is needed.

// GenerateSessionToken creates a tamper-proof HMAC-SHA256 signed session token.
// The format is: <expiry_unix>.<signature_hex>
func GenerateSessionToken(secretKey string, duration time.Duration) string {
	expiry := time.Now().Add(duration).Unix()
	payload := strconv.FormatInt(expiry, 10)

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", payload, sig)
}

// ValidateSessionToken verifies the cryptographic authenticity and expiration of a session token.
func ValidateSessionToken(token, secretKey string) bool {
	if token == "" || secretKey == "" {
		return false
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	expiryStr, sigHex := parts[0], parts[1]
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix() > expiry {
		return false
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(expiryStr))
	expectedSig := mac.Sum(nil)

	return hmac.Equal(sigBytes, expectedSig)
}
