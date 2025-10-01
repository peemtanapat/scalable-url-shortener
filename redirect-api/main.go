package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

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

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "up",
	})
}

type RedirectResponseBody struct {
	OriginalUrl string `json:"originalUrl" binding:"required"`
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

func getURLByShortCodeCache(shortCode string) (string, error) {
	cachedUrl, err := rdb.Get(ctx, "url:"+shortCode).Result()

	return cachedUrl, err
}

func saveURLCache(shortCode string, originalUrl string) {
	rdb.Set(ctx, "url:"+shortCode, originalUrl, time.Minute*30)
}

func main() {
	port := "8080"

	initDatabase()
	initRedis()

	r := gin.Default()

	r.GET("/api/health", healthHandler)

	// Redirect endpoint (for actual URL shortening usage)
	r.GET("/:shortCode", func(c *gin.Context) {
		shortCode := c.Param("shortCode")

		if shortCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "short code is required"})
			return
		}

		cachedUrl, err := getURLByShortCodeCache(shortCode)
		if err == nil {
			// Redirect to cached original URL
			c.Redirect(http.StatusFound, cachedUrl)
		}

		// Get URL from database
		urlData, err := getURLByShortCode(shortCode)
		if err != nil {
			if err.Error() == "short code not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "short code not found"})
			} else {
				log.Printf("Failed to get URL from database: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve URL"})
			}
			return
		}

		// Save cache
		saveURLCache(shortCode, urlData.OriginalURL)

		// Redirect to original URL
		c.Redirect(http.StatusFound, urlData.OriginalURL)
	})

	// New endpoint to retrieve original URL by short code
	// r.GET("/api/v1/urls/:shortCode", func(c *gin.Context) {
	// 	shortCode := c.Param("shortCode")

	// 	if shortCode == "" {
	// 		c.JSON(http.StatusBadRequest, gin.H{"error": "short code is required"})
	// 		return
	// 	}

	// 	// Get URL from database
	// 	urlData, err := getURLByShortCode(shortCode)
	// 	if err != nil {
	// 		if err.Error() == "short code not found" {
	// 			c.JSON(http.StatusNotFound, gin.H{"error": "short code not found"})
	// 		} else {
	// 			log.Printf("Failed to get URL from database: %v", err)
	// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve URL"})
	// 		}
	// 		return
	// 	}

	// 	c.JSON(http.StatusOK, gin.H{
	// 		"id":          urlData.ID,
	// 		"originalUrl": urlData.OriginalURL,
	// 		"shortCode":   urlData.ShortCode,
	// 		"createdAt":   urlData.CreatedAt,
	// 		"updatedAt":   urlData.UpdatedAt,
	// 	})
	// })

	// For testing
	r.GET("/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	fmt.Printf("Server starting on port %s", port)

	r.Run(":" + port)
}
