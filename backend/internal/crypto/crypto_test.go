package crypto

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {
	password := "super_secret_pass"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == password {
		t.Fatalf("Password and hash should not be equal")
	}

	if !CheckPasswordHash(password, hash) {
		t.Errorf("Password hash check failed for valid password")
	}

	if CheckPasswordHash("wrong_password", hash) {
		t.Errorf("Password hash check should fail for invalid password")
	}
}

func TestGenerateTokensAndValidate(t *testing.T) {
	userID := uuid.New()
	deviceID := uuid.New()
	username := "testuser"

	accessToken, refreshToken, expiry, err := GenerateTokens(userID, deviceID, username)
	if err != nil {
		t.Fatalf("Failed to generate tokens: %v", err)
	}

	if accessToken == "" || refreshToken == "" {
		t.Errorf("Tokens should not be empty")
	}

	if expiry.Before(time.Now()) {
		t.Errorf("Expiry should be in the future")
	}

	// Validate Access Token
	claims, err := ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatalf("Failed to validate access token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("User ID in claims mismatch: got %v, want %v", claims.UserID, userID)
	}

	if claims.DeviceID != deviceID {
		t.Errorf("Device ID in claims mismatch: got %v, want %v", claims.DeviceID, deviceID)
      }

	if claims.Username != username {
		t.Errorf("Username in claims mismatch: got %v, want %v", claims.Username, username)
	}
}

func TestGenerateRandomString(t *testing.T) {
	str1, err := GenerateRandomString(16)
	if err != nil {
		t.Fatalf("Failed to generate random string: %v", err)
	}

	str2, err := GenerateRandomString(16)
	if err != nil {
		t.Fatalf("Failed to generate random string: %v", err)
	}

	if len(str1) != 32 { // 16 bytes encoded as hex = 32 chars
		t.Errorf("Expected length 32, got %d", len(str1))
	}

	if str1 == str2 {
		t.Errorf("Random strings should not match")
	}
}
