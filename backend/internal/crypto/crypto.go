package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecretKey []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "ymessage_super_secure_production_secret_key"
	}
	jwtSecretKey = []byte(secret)
}

// Claims represent JWT payload
type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	DeviceID uuid.UUID `json:"device_id"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

// HashPassword hashes plain passwords using Bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash verifies a plain text password against a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRandomString produces a hex encoded cryptographically secure string of a given length
func GenerateRandomString(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateTokens creates both an AccessToken and a RefreshToken
func GenerateTokens(userID, deviceID uuid.UUID, username string) (string, string, time.Time, error) {
	// Access Token expires in 1 hour
	accessExpiry := time.Now().Add(time.Hour)
	accessClaims := &Claims{
		UserID:   userID,
		DeviceID: deviceID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString(jwtSecretKey)
	if err != nil {
		return "", "", time.Time{}, err
	}

	// Refresh Token expires in 30 days
	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
	refreshTokenStr, err := GenerateRandomString(32)
	if err != nil {
		return "", "", time.Time{}, err
	}

	return accessStr, refreshTokenStr, refreshExpiry, nil
}

// ValidateAccessToken checks validity of access token and returns claims
func ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}
