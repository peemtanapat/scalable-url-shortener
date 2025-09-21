package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// Global Redis client and Database connection
var rdb *redis.Client
var db *sql.DB
var ctx = context.Background()

// URL represents a URL mapping in the database
type URL struct {
	ID          int       `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortCode   string    `json:"short_code"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func initDatabase() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/urlshortener?sslmode=disable"
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Connected to PostgreSQL successfully")

	// Create tables if they don't exist
	createTablesQuery := `
		CREATE TABLE IF NOT EXISTS urls (
			id SERIAL PRIMARY KEY,
			original_url TEXT NOT NULL,
			short_code VARCHAR(10) NOT NULL UNIQUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code);
		CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls(created_at);
	`

	if _, err := db.Exec(createTablesQuery); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	fmt.Println("Database tables created/verified successfully")
}

func initRedis() {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default for local development
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddr, // Redis server address
		Password: "",        // No password
		DB:       0,         // Default DB
	})

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis successfully")

	// Initialize counter if it doesn't exist or is less than desired starting value
	startingValue := int64(56800235584)
	currentVal, err := rdb.Get(ctx, "url_counter").Int64()
	if err == redis.Nil || currentVal < startingValue {
		// Key doesn't exist or current value is less than desired starting value
		err = rdb.Set(ctx, "url_counter", startingValue-1, 0).Err()
		if err != nil {
			log.Fatalf("Failed to initialize Redis counter: %v", err)
		}
		log.Printf("Initialized Redis counter to start from %d", startingValue)
	} else {
		log.Printf("Redis counter already exists with value: %d", currentVal)
	}
}

func getNextID() (int, error) {
	// Use Redis INCR to get auto-incrementing ID
	val, err := rdb.Incr(ctx, "url_counter").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get next ID from Redis: %v", err)
	}
	return int(val), nil
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "up",
	})
}

type ConvertRequestBody struct {
	OriginalUrl string `json:"originalUrl" binding:"required"`
}

type ConvertResponseBody struct {
	ShortUrl string `json:"shortUrl" binding:"required"`
}

func encodeBase62(num int) string {
	BASE62_CHARS := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	if num == 0 {
		return "0"
	}

	result := ""
	for num > 0 {
		result = string(BASE62_CHARS[num%62]) + result
		num = num / 62
	}

	return fmt.Sprintf("%07s", result)
}

func generateShortCode(id int) string {
	// random 0-999
	randomSalt := 0 + rand.Intn(1000)

	multiplier := 1000
	combinedNumber := (id * multiplier) + randomSalt

	fmt.Printf("\ncombinedNumber=%d\n", combinedNumber)

	shortCode := encodeBase62(combinedNumber)

	return shortCode
}

// Database operations
func saveURL(originalURL, shortCode string) (*URL, error) {
	query := `
		INSERT INTO urls (original_url, short_code) 
		VALUES ($1, $2) 
		RETURNING id, original_url, short_code, created_at, updated_at
	`

	var url URL
	err := db.QueryRow(query, originalURL, shortCode).Scan(
		&url.ID, &url.OriginalURL, &url.ShortCode, &url.CreatedAt, &url.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to save URL: %v", err)
	}

	return &url, nil
}

func getURLByShortCode(shortCode string) (*URL, error) {
	query := `
		SELECT id, original_url, short_code, created_at, updated_at 
		FROM urls 
		WHERE short_code = $1
	`

	var url URL
	err := db.QueryRow(query, shortCode).Scan(
		&url.ID, &url.OriginalURL, &url.ShortCode, &url.CreatedAt, &url.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("short code not found")
		}
		return nil, fmt.Errorf("failed to get URL: %v", err)
	}

	return &url, nil
}

func main() {
	port := "8080"

	initDatabase()
	initRedis()

	r := gin.Default()

	r.GET("/api/health", healthHandler)

	r.POST("/api/v1/urls", func(c *gin.Context) {
		var requestBody ConvertRequestBody

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		originalUrl := requestBody.OriginalUrl
		_, err := url.Parse(originalUrl)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
			return
		}

		// Get next ID from Redis
		id, err := getNextID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate short URL"})
			return
		}

		// Generate short code
		shortCode := generateShortCode(id)

		// Save to PostgreSQL database
		savedURL, err := saveURL(originalUrl, shortCode)
		if err != nil {
			log.Printf("Failed to save URL to database: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save URL"})
			return
		}

		// Log the generated auto-incremental ID
		log.Printf("auto-incremental ID: %d for URL: %s shortCode: %s, saved with DB ID: %d",
			id, originalUrl, shortCode, savedURL.ID)

		c.JSON(http.StatusCreated, gin.H{
			"shortUrl":    "http://localhost:8000/" + shortCode,
			"shortCode":   shortCode,
			"originalUrl": originalUrl,
			"id":          savedURL.ID,
		})
	})

	// For testing
	r.GET("/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	fmt.Printf("Server starting on port %s", port)

	r.Run(":" + port)
}
