package chat

import (
	"context"
	"log"
	"sync"
	"time"

	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents a connected user device websocket session
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	UserID   uuid.UUID
	DeviceID uuid.UUID
	Send     chan []byte
}

// Hub maintains active client connections and routes messages
type Hub struct {
	clients    map[uuid.UUID]map[uuid.UUID]*Client // UserID -> DeviceID -> Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

var GlobalHub *Hub

func InitHub() *Hub {
	GlobalHub = &Hub{
		clients:    make(map[uuid.UUID]map[uuid.UUID]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	return GlobalHub
}

func (h *Hub) Start() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; !ok {
				h.clients[client.UserID] = make(map[uuid.UUID]*Client)
				// Start Redis PubSub subscriber for this user
				go h.subscribeUserRedis(client.UserID)
			}
			h.clients[client.UserID][client.DeviceID] = client
			h.mu.Unlock()
			log.Printf("Device %s connected for user %s", client.DeviceID, client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if devices, ok := h.clients[client.UserID]; ok {
				if _, exists := devices[client.DeviceID]; exists {
					delete(devices, client.DeviceID)
					close(client.Send)
					log.Printf("Device %s disconnected for user %s", client.DeviceID, client.UserID)
				}
				if len(devices) == 0 {
					delete(h.clients, client.UserID)
				}
			}
			h.mu.Unlock()
		}
	}
}

// subscribeUserRedis listens to user's direct messages published on Redis Pub/Sub
func (h *Hub) subscribeUserRedis(userID uuid.UUID) {
	ctx := context.Background()
	pubsub := database.RedisClient.Subscribe(ctx, "user:"+userID.String())
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		// Deliver message to all active devices of this user
		h.mu.RLock()
		devices, online := h.clients[userID]
		if online {
			for _, client := range devices {
				select {
				case client.Send <- []byte(msg.Payload):
				default:
					// If channel is blocked, clean up
					go func(c *Client) { h.unregister <- c }(client)
				}
			}
		}
		h.mu.RUnlock()

		// If user is offline, pubsub will terminate when they disconnect.
		// If they have no active devices at all, unsubscribe.
		h.mu.RLock()
		_, exists := h.clients[userID]
		h.mu.RUnlock()
		if !exists {
			break
		}
	}
}

// PublishToUser sends message payload to a user via Redis PubSub
func (h *Hub) PublishToUser(userID uuid.UUID, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	database.RedisClient.Publish(ctx, "user:"+userID.String(), payload)
}

// BroadcastToGroup sends a message to all group members
func (h *Hub) BroadcastToGroup(groupID uuid.UUID, payload []byte, senderID uuid.UUID) {
	var members []models.GroupMember
	database.DB.Where("group_id = ?", groupID).Find(&members)

	for _, member := range members {
		// Do not echoes back to the sender device (or send to all devices of sender for sync)
		h.PublishToUser(member.UserID, payload)
	}
}
