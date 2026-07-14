package auth

import (
	"net/http"
	"time"

	"ymessage/internal/crypto"
	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RegisterReq struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Phone       string `json:"phone"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"display_name"`
}

type LoginReq struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	Platform    string `json:"platform" binding:"required"` // ios, android, web, desktop
	DeviceName  string `json:"device_name" binding:"required"`
	IdentityKey string `json:"identity_key" binding:"required"` // E2EE client identity key
}

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ProfileUpdateReq struct {
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Bio         string `json:"bio"`
	Status      string `json:"status"`
}

// Register creates a new user account
func Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hashedPassword,
		DisplayName:  req.DisplayName,
		Status:       "offline",
		LastSeen:     time.Now(),
	}

	if req.DisplayName == "" {
		user.DisplayName = req.Username
	}

	tx := database.DB.Create(&user)
	if tx.Error != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username or Email already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user_id": user.ID,
	})
}

// Login authenticates a user and registers/updates device session
func Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if !crypto.CheckPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate Device ID
	deviceID := uuid.New()

	// Generate Access and Refresh Tokens
	accessToken, refreshToken, expiry, err := crypto.GenerateTokens(user.ID, deviceID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session tokens"})
		return
	}

	// Save new device session
	device := models.Device{
		UserID:       user.ID,
		Platform:     req.Platform,
		DeviceName:   req.DeviceName,
		IdentityKey:  req.IdentityKey,
		RefreshToken: refreshToken,
		TokenExpiry:  expiry,
		LastActiveAt: time.Now(),
	}
	device.ID = deviceID

	if err := database.DB.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save device session"})
		return
	}

	// Update online status
	database.DB.Model(&user).Updates(map[string]interface{}{
		"status":    "online",
		"last_seen": time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"device_id":     deviceID,
		"user": gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"avatar_url":   user.AvatarURL,
			"bio":          user.Bio,
		},
	})
}

// Refresh renews access token using refresh token
func Refresh(c *gin.Context) {
	var req RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var device models.Device
	if err := database.DB.Where("refresh_token = ? AND token_expiry > ?", req.RefreshToken, time.Now()).First(&device).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, device.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Rotate access and refresh tokens
	accessToken, newRefreshToken, expiry, err := crypto.GenerateTokens(user.ID, device.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate tokens"})
		return
	}

	device.RefreshToken = newRefreshToken
	device.TokenExpiry = expiry
	device.LastActiveAt = time.Now()
	database.DB.Save(&device)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// Logout revokes device refresh token
func Logout(c *gin.Context) {
	deviceID, exists := c.Get("deviceID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID missing from context"})
		return
	}

	database.DB.Model(&models.Device{}).Where("id = ?", deviceID).Updates(map[string]interface{}{
		"refresh_token": "",
		"token_expiry":  time.Now(),
	})

	userID, _ := c.Get("userID")
	// If no other devices are active, set status to offline
	var activeCount int64
	database.DB.Model(&models.Device{}).Where("user_id = ? AND token_expiry > ?", userID, time.Now()).Count(&activeCount)
	if activeCount == 0 {
		database.DB.Model(&models.User{}).Where("id = ?", userID).Update("status", "offline")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetDevices returns all active user sessions
func GetDevices(c *gin.Context) {
	userID, _ := c.Get("userID")
	var devices []models.Device
	database.DB.Where("user_id = ? AND token_expiry > ?", userID, time.Now()).Find(&devices)
	c.JSON(http.StatusOK, devices)
}

// TerminateDevice remotely logs out a device session
func TerminateDevice(c *gin.Context) {
	userID, _ := c.Get("userID")
	targetDeviceIDStr := c.Param("deviceId")
	targetDeviceID, err := uuid.Parse(targetDeviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target device ID"})
		return
	}

	// Revoke refresh token for target device
	res := database.DB.Model(&models.Device{}).
		Where("id = ? AND user_id = ?", targetDeviceID, userID).
		Updates(map[string]interface{}{
			"refresh_token": "",
			"token_expiry":  time.Now(),
		})

	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device session terminated"})
}

// GetProfile retrieves logged in user's profile info
func GetProfile(c *gin.Context) {
	userID, _ := c.Get("userID")
	var user models.User
	if err := database.DB.Preload("Devices").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateProfile updates user profile information
func UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req ProfileUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	if len(updates) > 0 {
		database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// GetUserByUsername retrieves another user's display name & avatar for chat initialization
func GetUserByUsername(c *gin.Context) {
	username := c.Param("username")
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"bio":          user.Bio,
		"status":       user.Status,
		"last_seen":    user.LastSeen,
	})
}
