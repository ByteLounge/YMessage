package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"ymessage/internal/models"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var RedisClient *redis.Client

// InitDB initializes PostgreSQL connection and runs auto-migrations
func InitDB() *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=ymessage port=5432 sslmode=disable"
	}

	var db *gorm.DB
	var err error
	maxRetries := 5

	// Retry connecting to DB (crucial for Docker Compose startup order)
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

	if err != nil {
		log.Fatalf("Could not connect to PostgreSQL database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to fetch underlying sql.DB: %v", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

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

// InitRedis initializes the Redis client
func InitRedis() *redis.Client {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	opts, err := redis.ParseURL(redisURL)
	var client *redis.Client
	if err != nil {
		// Fallback to plain address
		client = redis.NewClient(&redis.Options{
			Addr: redisURL,
		})
	} else {
		client = redis.NewClient(opts)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("Warning: Redis connection ping failed: %v", err)
	} else {
		log.Println("Redis connection established.")
	}

	RedisClient = client
	return client
}
