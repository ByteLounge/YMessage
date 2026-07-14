package chat

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreateGroupReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	Type        string `json:"type"` // group, channel
}

type JoinGroupReq struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

type UpdateRoleReq struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"` // owner, admin, moderator, member
}

// GenerateInviteCode generates a random 8-character code
func GenerateInviteCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateGroup creates a new group and joins the creator as owner
func CreateGroup(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req CreateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Type == "" {
		req.Type = "group"
	}

	group := models.Group{
		Name:        req.Name,
		Description: req.Description,
		AvatarURL:   req.AvatarURL,
		OwnerID:     userID.(uuid.UUID),
		Type:        req.Type,
		InviteCode:  GenerateInviteCode(),
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&group).Error; err != nil {
			return err
		}

		member := models.GroupMember{
			GroupID:  group.ID,
			UserID:   userID.(uuid.UUID),
			Role:     "owner",
			JoinedAt: time.Now(),
		}

		return tx.Create(&member).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

// JoinGroup joins a group using its invite code
func JoinGroup(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req JoinGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var group models.Group
	if err := database.DB.Where("invite_code = ?", req.InviteCode).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid invite code"})
		return
	}

	// Check if already member
	var count int64
	database.DB.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", group.ID, userID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Already a member of this group"})
		return
	}

	member := models.GroupMember{
		GroupID:  group.ID,
		UserID:   userID.(uuid.UUID),
		Role:     "member",
		JoinedAt: time.Now(),
	}

	if err := database.DB.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined group successfully", "group": group})
}

// GetMyGroups returns all groups the user belongs to
func GetMyGroups(c *gin.Context) {
	userID, _ := c.Get("userID")
	var memberships []models.GroupMember
	if err := database.DB.Preload("User").Where("user_id = ?", userID).Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query memberships"})
		return
	}

	var groupIDs []uuid.UUID
	for _, m := range memberships {
		groupIDs = append(groupIDs, m.GroupID)
	}

	var groups []models.Group
	if len(groupIDs) > 0 {
		database.DB.Where("id IN ?", groupIDs).Find(&groups)
	} else {
		groups = []models.Group{}
	}

	c.JSON(http.StatusOK, groups)
}

// GetGroupMembers lists all members of a group
func GetGroupMembers(c *gin.Context) {
	groupIDStr := c.Param("groupId")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	// Check if requester is member of the group
	userID, _ := c.Get("userID")
	var count int64
	database.DB.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not belong to this group"})
		return
	}

	var members []models.GroupMember
	if err := database.DB.Preload("User").Where("group_id = ?", groupID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

// UpdateGroupRole updates a member's role
func UpdateGroupRole(c *gin.Context) {
	groupIDStr := c.Param("groupId")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetUserID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target user ID"})
		return
	}

	userID, _ := c.Get("userID")

	// Get requester's role
	var requesterMember models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&requesterMember).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Validate authority: Only Owners and Admins can promote/demote
	if requesterMember.Role != "owner" && requesterMember.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to change roles"})
		return
	}

	// Owner role can't be easily overwritten unless transferred
	if req.Role == "owner" && requesterMember.Role != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the current group owner can transfer ownership"})
		return
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if req.Role == "owner" {
			// Demote previous owner to admin
			if err := tx.Model(&models.GroupMember{}).Where("group_id = ? AND role = ?", groupID, "owner").Update("role", "admin").Error; err != nil {
				return err
			}
			// Update group owner ID
			if err := tx.Model(&models.Group{}).Where("id = ?", groupID).Update("owner_id", targetUserID).Error; err != nil {
				return err
			}
		}

		return tx.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, targetUserID).Update("role", req.Role).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role updated successfully"})
}

// LeaveGroup removes user from the group membership list
func LeaveGroup(c *gin.Context) {
	groupIDStr := c.Param("groupId")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	userID, _ := c.Get("userID")

	var member models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Membership not found"})
		return
	}

	if member.Role == "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Owner cannot leave without transferring ownership"})
		return
	}

	if err := database.DB.Delete(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Left group successfully"})
}
