package chat

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512KB limit for encrypted payloads
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Configured via API Gateway CORS in production
	},
}

type WSAction struct {
	Type    string          `json:"type"` // message, typing, receipt, reaction, sync
	Payload json.RawMessage `json:"payload"`
}

type WSMessagePayload struct {
	ReceiverID       *uuid.UUID `json:"receiver_id,omitempty"`
	GroupID          *uuid.UUID `json:"group_id,omitempty"`
	ContentType      string     `json:"content_type"` // text, photo, video, etc.
	EncryptedPayload string     `json:"encrypted_payload"`
	EphemeralKey     string     `json:"ephemeral_key"`
	Counter          uint32     `json:"counter"`
	SenderRatchetKey string     `json:"sender_ratchet_key"`
}

type WSTypingPayload struct {
	ChatID   uuid.UUID `json:"chat_id"` // UserID or GroupID
	IsTyping bool      `json:"is_typing"`
}

type WSReceiptPayload struct {
	MessageIDs []uuid.UUID `json:"message_ids"`
	Status     string      `json:"status"` // delivered, read
}

type WSReactionPayload struct {
	MessageID uuid.UUID `json:"message_id"`
	Reaction  string    `json:"reaction"`
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Websocket error on read pump: %v", err)
			}
			break
		}

		var action WSAction
		if err := json.Unmarshal(message, &action); err != nil {
			log.Printf("Failed to unmarshal websocket action: %v", err)
			continue
		}

		switch action.Type {
		case "message":
			c.handleMessage(action.Payload)
		case "typing":
			c.handleTyping(action.Payload)
		case "receipt":
			c.handleReceipt(action.Payload)
		case "reaction":
			c.handleReaction(action.Payload)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages, stores in DB and forwards to destination
func (c *Client) handleMessage(rawPayload json.RawMessage) {
	var payload WSMessagePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		log.Printf("WS message unmarshal error: %v", err)
		return
	}

	msg := models.Message{
		SenderID:         c.UserID,
		ReceiverID:       payload.ReceiverID,
		GroupID:          payload.GroupID,
		ContentType:      payload.ContentType,
		EncryptedPayload: payload.EncryptedPayload,
		EphemeralKey:     payload.EphemeralKey,
		Counter:          payload.Counter,
		SenderRatchetKey: payload.SenderRatchetKey,
		Status:           "sent",
	}

	// Persist in DB
	if err := database.DB.Create(&msg).Error; err != nil {
		log.Printf("Failed to save message to DB: %v", err)
		return
	}

	// Prepare outbound broadcast payload
	resp, _ := json.Marshal(gin.H{
		"type": "message",
		"payload": gin.H{
			"id":                 msg.ID,
			"sender_id":          msg.SenderID,
			"receiver_id":        msg.ReceiverID,
			"group_id":           msg.GroupID,
			"content_type":       msg.ContentType,
			"encrypted_payload":  msg.EncryptedPayload,
			"ephemeral_key":      msg.EphemeralKey,
			"counter":            msg.Counter,
			"sender_ratchet_key": msg.SenderRatchetKey,
			"status":             msg.Status,
			"created_at":         msg.CreatedAt,
		},
	})

	if msg.GroupID != nil {
		c.Hub.BroadcastToGroup(*msg.GroupID, resp, c.UserID)
	} else if msg.ReceiverID != nil {
		// Send to recipient
		c.Hub.PublishToUser(*msg.ReceiverID, resp)
		// Send to sender's other devices for sync
		c.Hub.PublishToUser(c.UserID, resp)
	}
}

// handleTyping forwards typing status to the target user or group
func (c *Client) handleTyping(rawPayload json.RawMessage) {
	var payload WSTypingPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return
	}

	resp, _ := json.Marshal(gin.H{
		"type": "typing",
		"payload": gin.H{
			"sender_id": c.UserID,
			"chat_id":   payload.ChatID,
			"is_typing": payload.IsTyping,
		},
	})

	// Check if Group vs Direct typing
	var groupCount int64
	database.DB.Model(&models.Group{}).Where("id = ?", payload.ChatID).Count(&groupCount)

	if groupCount > 0 {
		c.Hub.BroadcastToGroup(payload.ChatID, resp, c.UserID)
	} else {
		c.Hub.PublishToUser(payload.ChatID, resp)
	}
}

// handleReceipt processes read/delivered receipts and updates DB
func (c *Client) handleReceipt(rawPayload json.RawMessage) {
	var payload WSReceiptPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return
	}

	if len(payload.MessageIDs) == 0 {
		return
	}

	// Update DB
	database.DB.Model(&models.Message{}).
		Where("id IN ?", payload.MessageIDs).
		Update("status", payload.Status)

	// Notify senders about the status changes
	var messages []models.Message
	database.DB.Where("id IN ?", payload.MessageIDs).Find(&messages)

	// Group messages by original sender to batch dispatch notifications
	senderMap := make(map[uuid.UUID][]uuid.UUID)
	for _, m := range messages {
		senderMap[m.SenderID] = append(senderMap[m.SenderID], m.ID)
	}

	for senderID, msgIDs := range senderMap {
		resp, _ := json.Marshal(gin.H{
			"type": "receipt",
			"payload": gin.H{
				"status":      payload.Status,
				"message_ids": msgIDs,
				"reader_id":   c.UserID,
			},
		})
		c.Hub.PublishToUser(senderID, resp)
	}
}

// handleReaction persists reactions and broadcasts them
func (c *Client) handleReaction(rawPayload json.RawMessage) {
	var payload WSReactionPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return
	}

	reaction := models.MessageReaction{
		MessageID: payload.MessageID,
		UserID:    c.UserID,
		Reaction:  payload.Reaction,
		CreatedAt: time.Now(),
	}

	database.DB.Save(&reaction)

	// Fetch message to know where to broadcast
	var msg models.Message
	if err := database.DB.First(&msg, payload.MessageID).Error; err != nil {
		return
	}

	resp, _ := json.Marshal(gin.H{
		"type": "reaction",
		"payload": gin.H{
			"message_id": payload.MessageID,
			"user_id":    c.UserID,
			"reaction":   payload.Reaction,
		},
	})

	if msg.GroupID != nil {
		c.Hub.BroadcastToGroup(*msg.GroupID, resp, c.UserID)
	} else {
		c.Hub.PublishToUser(msg.SenderID, resp)
		if msg.ReceiverID != nil {
			c.Hub.PublishToUser(*msg.ReceiverID, resp)
		}
	}
}

// ServeWs upgrades HTTP request and handles register
func ServeWs(hub *Hub, c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	deviceIDVal, _ := c.Get("deviceID")

	userID := userIDVal.(uuid.UUID)
	deviceID := deviceIDVal.(uuid.UUID)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Websocket upgrade error: %v", err)
		return
	}

	client := &Client{
		Hub:      hub,
		Conn:     conn,
		UserID:   userID,
		DeviceID: deviceID,
		Send:     make(chan []byte, 256),
	}

	client.Hub.register <- client

	// Start reading and writing concurrently
	go client.writePump()
	go client.readPump()
}
