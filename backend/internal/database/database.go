package database

import (
	"context"
	"log"
	"os"
	"time"

	"ymessage/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var RedisClient *redis.Client
var IsUsingRedis bool

// InitDB initializes PostgreSQL connection or falls back to SQLite
func InitDB() *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	var db *gorm.DB
	var err error

	if dsn == "" {
		log.Println("DATABASE_URL not set. Falling back to local SQLite database (ymessage.db)...")
		db, err = gorm.Open(sqlite.Open("ymessage.db"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
	} else {
		maxRetries := 5
		for i := 1; i <= maxRetries; i++ {
			db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Info),
			})
			if err == nil {
				break
			}
			log.Printf("Failed to connect to database (attempt %d/%d): %v. Retrying in 5 seconds...", i, maxRetries, err)
			time.Sleep(5 * time.Second)
		}
	}

	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	log.Println("Database connection established.")

	// Run migrations
	err = db.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.PrekeyBundle{},
		&models.OneTimePrekey{},
		&models.Group{},
		&models.GroupMember{},
		&models.Message{},
		&models.MessageReaction{},
		&models.AuditLog{},
	)
	if err != nil {
		log.Fatalf("Failed to run DB auto-migrations: %v", err)
	}

	log.Println("Database migrations applied successfully.")
	DB = db
	return db
}

// InitRedis initializes the Redis client or sets local fallback flag
func InitRedis() *redis.Client {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("REDIS_URL not set. WebSocket server will run in local-routing mode (no Redis PubSub required).")
		IsUsingRedis = false
		return nil
	}

	opts, err := redis.ParseURL(redisURL)
	var client *redis.Client
	if err != nil {
		client = redis.NewClient(&redis.Options{
			Addr: redisURL,
		})
	} else {
		client = redis.NewClient(opts)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("Redis ping failed: %v. Running WebSocket in local-routing mode.", err)
		IsUsingRedis = false
		return nil
	}

	log.Println("Redis connection established.")
	IsUsingRedis = true
	RedisClient = client
	return client
}
