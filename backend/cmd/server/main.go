package main

import (
	"log"
	"net/http"
	"os"

	"ymessage/internal/admin"
	"ymessage/internal/auth"
	"ymessage/internal/chat"
	"ymessage/internal/crypto"
	"ymessage/internal/database"
	"ymessage/internal/media"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting YMessage backend server...")

	// 1. Initialize DB and Redis
	database.InitDB()
	database.InitRedis()

	// 2. Initialize S3 Media Client
	media.InitS3()

	// 3. Initialize & Start WebSocket Hub
	hub := chat.InitHub()
	go hub.Start()

	// 4. Create Gin Router
	r := gin.Default()

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Static folder for local uploads fallback
	r.Static("/uploads", "./uploads")

	// 5. Register Routes
	api := r.Group("/api")
	{
		// Public Auth routes
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", auth.Register)
			authGroup.POST("/login", auth.Login)
			authGroup.POST("/refresh", auth.Refresh)
		}

		// Authenticated routes
		secured := api.Group("")
		secured.Use(auth.AuthMiddleware())
		{
			// Auth, Devices, Profile
			secured.POST("/auth/logout", auth.Logout)
			secured.GET("/auth/devices", auth.GetDevices)
			secured.DELETE("/auth/devices/:deviceId", auth.TerminateDevice)
			secured.GET("/auth/profile", auth.GetProfile)
			secured.PUT("/auth/profile", auth.UpdateProfile)
			secured.GET("/auth/users/:username", auth.GetUserByUsername)

			// E2EE Prekeys (X3DH)
			secured.POST("/crypto/prekey", crypto.UploadPrekeyBundle)
			secured.GET("/crypto/prekey/status", crypto.GetPrekeyStatus)
			secured.GET("/crypto/prekey/:userId", crypto.GetPrekeyBundle)

			// Chats & Messaging history
			secured.GET("/chat/messages", chat.GetMessages)
			secured.GET("/chat/chats", chat.GetChats)

			// Group management
			secured.POST("/chat/groups", chat.CreateGroup)
			secured.POST("/chat/groups/join", chat.JoinGroup)
			secured.GET("/chat/groups", chat.GetMyGroups)
			secured.GET("/chat/groups/:groupId/members", chat.GetGroupMembers)
			secured.PUT("/chat/groups/:groupId/role", chat.UpdateGroupRole)
			secured.DELETE("/chat/groups/:groupId/leave", chat.LeaveGroup)

			// WebSocket Real-time messaging
			secured.GET("/chat/ws", func(c *gin.Context) {
				chat.ServeWs(hub, c)
			})

			// Media uploading
			secured.POST("/media/upload", media.UploadAttachment)
		}

		// Admin Moderation and Telemetry
		adm := api.Group("/admin")
		adm.Use(auth.AuthMiddleware(), admin.AdminMiddleware())
		{
			adm.GET("/metrics", admin.GetSystemMetrics)
			adm.GET("/users", admin.GetUsers)
			adm.POST("/users/ban", admin.BanUser)
			adm.DELETE("/messages/:messageId", admin.DeleteMessage)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("YMessage API Gateway running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
