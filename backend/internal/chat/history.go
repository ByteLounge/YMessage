package chat

import (
	"net/http"
	"time"

	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ChatListItem struct {
	ChatID       uuid.UUID  `json:"chat_id"`       // UserID (Direct) or GroupID (Group)
	Name         string     `json:"name"`          // User DisplayName or Group Name
	AvatarURL    string     `json:"avatar_url"`    // User Avatar or Group Avatar
	Type         string     `json:"type"`          // "direct" or "group"
	LastMessage  *models.Message `json:"last_message"`
	UnreadCount  int64      `json:"unread_count"`
	LastActiveAt time.Time  `json:"last_active_at"`
}

// GetMessages retrieves paginated message history for a direct chat or group chat
func GetMessages(c *gin.Context) {
	userID, _ := c.Get("userID")

	targetIDStr := c.Query("chat_id") // Can be recipient user ID or group ID
	if targetIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat_id parameter required"})
		return
	}

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chat_id format"})
		return
	}

	// Pagination params
	cursorStr := c.Query("cursor") // timestamp format RF3339
	limit := 50
	var cursorTime time.Time
	if cursorStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, cursorStr)
		if err == nil {
			cursorTime = parsedTime
		}
	}

	var messages []models.Message
	query := database.DB.Order("created_at DESC").Limit(limit)

	if !cursorTime.IsZero() {
		query = query.Where("created_at < ?", cursorTime)
	}

	// Check if this ID is a group
	var groupCount int64
	database.DB.Model(&models.Group{}).Where("id = ?", targetID).Count(&groupCount)

	if groupCount > 0 {
		// Verify requester is a member of the group
		var memberCount int64
		database.DB.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", targetID, userID).Count(&memberCount)
		if memberCount == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this group history"})
			return
		}
		query = query.Where("group_id = ?", targetID)
	} else {
		// Direct Message between current user and target user
		query = query.Where(
			"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			userID, targetID, targetID, userID,
		)
	}

	if err := query.Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	// Reverse messages to return chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	nextCursor := ""
	if len(messages) > 0 {
		// Use the earliest message timestamp as next cursor
		nextCursor = messages[0].CreatedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"messages":    messages,
		"next_cursor": nextCursor,
	})
}

// GetChats lists all active conversations for the user
func GetChats(c *gin.Context) {
	userID, _ := c.Get("userID")

	// 1. Get Group Chats
	var groupMemberships []models.GroupMember
	database.DB.Where("user_id = ?", userID).Find(&groupMemberships)

	var groupIDs []uuid.UUID
	for _, gm := range groupMemberships {
		groupIDs = append(groupIDs, gm.GroupID)
	}

	var groups []models.Group
	if len(groupIDs) > 0 {
		database.DB.Where("id IN ?", groupIDs).Find(&groups)
	}

	// 2. Get distinct user IDs we have messaged with
	type DMContact struct {
		ContactID uuid.UUID
	}
	var dmContacts []DMContact
	database.DB.Raw(`
		SELECT DISTINCT CASE 
			WHEN sender_id = ? THEN receiver_id 
			ELSE sender_id 
		END as contact_id
		FROM messages 
		WHERE (sender_id = ? AND receiver_id IS NOT NULL) OR (receiver_id = ? AND sender_id IS NOT NULL)
	`, userID, userID, userID).Scan(&dmContacts)

	var contactIDs []uuid.UUID
	for _, dc := range dmContacts {
		contactIDs = append(contactIDs, dc.ContactID)
	}

	var users []models.User
	if len(contactIDs) > 0 {
		database.DB.Where("id IN ?", contactIDs).Find(&users)
	}

	var chatList []ChatListItem

	// Populate group chat metadata & last message
	for _, g := range groups {
		var lastMsg models.Message
		err := database.DB.Where("group_id = ?", g.ID).Order("created_at DESC").First(&lastMsg).Error
		var lastMsgPtr *models.Message = nil
		lastActive := g.CreatedAt
		if err == nil {
			lastMsgPtr = &lastMsg
			lastActive = lastMsg.CreatedAt
		}

		chatList = append(chatList, ChatListItem{
			ChatID:       g.ID,
			Name:         g.Name,
			AvatarURL:    g.AvatarURL,
			Type:         "group",
			LastMessage:  lastMsgPtr,
			LastActiveAt: lastActive,
			UnreadCount:  0, // Set to 0 or calculate unreads
		})
	}

	// Populate DM chat metadata & last message
	for _, u := range users {
		var lastMsg models.Message
		err := database.DB.Where(
			"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			userID, u.ID, u.ID, userID,
		).Order("created_at DESC").First(&lastMsg).Error

		var lastMsgPtr *models.Message = nil
		lastActive := u.CreatedAt
		if err == nil {
			lastMsgPtr = &lastMsg
			lastActive = lastMsg.CreatedAt
		}

		// Count unreads where receiver is me and status is not "read"
		var unreadCount int64
		database.DB.Model(&models.Message{}).
			Where("sender_id = ? AND receiver_id = ? AND status != ?", u.ID, userID, "read").
			Count(&unreadCount)

		chatList = append(chatList, ChatListItem{
			ChatID:       u.ID,
			Name:         u.DisplayName,
			AvatarURL:    u.AvatarURL,
			Type:         "direct",
			LastMessage:  lastMsgPtr,
			LastActiveAt: lastActive,
			UnreadCount:  unreadCount,
		})
	}

	c.JSON(http.StatusOK, chatList)
}
