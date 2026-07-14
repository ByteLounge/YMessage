package admin

import (
	"net/http"
	"runtime"
	"time"

	"ymessage/internal/chat"
	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BanUserReq struct {
	UserID string `json:"user_id" binding:"required"`
	Ban    bool   `json:"ban"`
}

// AdminMiddleware ensures only authorized admin users can access the route
func AdminMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		username, exists := c.Get("username")
		if !exists || username.(string) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied. Administrator privileges required."})
			c.Abort()
			return
		}
		c.Next()
	})
}

// GetSystemMetrics returns runtime system stats and messaging totals
func GetSystemMetrics(c *gin.Context) {
	var totalUsers int64
	var totalMessages int64
	var activeDevices int64

	database.DB.Model(&models.User{}).Count(&totalUsers)
	database.DB.Model(&models.Message{}).Count(&totalMessages)
	database.DB.Model(&models.Device{}).Where("token_expiry > ?", time.Now()).Count(&activeDevices)

	// Memory usage stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.JSON(http.StatusOK, gin.H{
		"users_total":        totalUsers,
		"messages_total":     totalMessages,
		"active_connections": activeDevices,
		"runtime_stats": gin.H{
			"alloc_mb":       m.Alloc / 1024 / 1024,
			"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
			"sys_mb":         m.Sys / 1024 / 1024,
			"num_gc":         m.NumGC,
			"goroutines":     runtime.NumGoroutine(),
		},
	})
}

// GetUsers lists all users in the system with pagination
func GetUsers(c *gin.Context) {
	var users []models.User
	if err := database.DB.Preload("Devices").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// BanUser deactivates a user and forces logout on all their devices
func BanUser(c *gin.Context) {
	var req BanUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetUserID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	if req.Ban {
		// Demote user status to offline
		database.DB.Model(&models.User{}).Where("id = ?", targetUserID).Update("status", "banned")

		// Revoke all refresh tokens to force logouts
		database.DB.Model(&models.Device{}).
			Where("user_id = ?", targetUserID).
			Updates(map[string]interface{}{
				"refresh_token": "",
				"token_expiry":  time.Now(),
			})

		// Log audit trail
		adminID, _ := c.Get("userID")
		audit := models.AuditLog{
			UserID:    adminID.(uuid.UUID),
			Action:    "BAN_USER",
			Details:   "Banned user ID: " + targetUserID.String(),
			IPAddress: c.ClientIP(),
		}
		database.DB.Create(&audit)
	} else {
		database.DB.Model(&models.User{}).Where("id = ?", targetUserID).Update("status", "offline")

		adminID, _ := c.Get("userID")
		audit := models.AuditLog{
			UserID:    adminID.(uuid.UUID),
			Action:    "UNBAN_USER",
			Details:   "Unbanned user ID: " + targetUserID.String(),
			IPAddress: c.ClientIP(),
		}
		database.DB.Create(&audit)
	}

	c.JSON(http.StatusOK, gin.H{"message": "User ban status updated successfully"})
}

// DeleteMessage allows admins to delete specific messages (content moderation)
func DeleteMessage(c *gin.Context) {
	msgIDStr := c.Param("messageId")
	msgID, err := uuid.Parse(msgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID format"})
		return
	}

	// Delete from DB (soft delete)
	if err := database.DB.Delete(&models.Message{}, msgID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}

	// Log audit trail
	adminID, _ := c.Get("userID")
	audit := models.AuditLog{
		UserID:    adminID.(uuid.UUID),
		Action:    "DELETE_MESSAGE",
		Details:   "Deleted message ID: " + msgIDStr,
		IPAddress: c.ClientIP(),
	}
	database.DB.Create(&audit)

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted by administrator"})
}
