package crypto

import (
	"testing"
	"time"
)

func TestSessionTokenLifecycle(t *testing.T) {
	secret := "test-secret-key-12345"

	// 1. Valid token
	token := GenerateSessionToken(secret, 1*time.Hour)
	if !ValidateSessionToken(token, secret) {
		t.Fatalf("Expected valid token to pass verification, got false")
	}

	// 2. Expired token
	expiredToken := GenerateSessionToken(secret, -1*time.Second)
	if ValidateSessionToken(expiredToken, secret) {
		t.Fatalf("Expected expired token to fail verification, got true")
	}

	// 3. Wrong secret key
	if ValidateSessionToken(token, "wrong-secret-key") {
		t.Fatalf("Expected verification with wrong secret to fail, got true")
	}

	// 4. Tampered payload
	tampered := "9999999999." + token[stringsIndex(token, ".")+1:]
	if ValidateSessionToken(tampered, secret) {
		t.Fatalf("Expected tampered token to fail verification, got true")
	}

	// 5. Malformed tokens
	if ValidateSessionToken("malformed", secret) {
		t.Fatalf("Expected malformed token without dot to fail")
	}
	if ValidateSessionToken("", secret) {
		t.Fatalf("Expected empty token to fail")
	}
}

func stringsIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
