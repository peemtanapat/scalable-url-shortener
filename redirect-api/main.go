package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Global Redis client and Database connection
var db *sql.DB

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

func main() {
	port := "8080"

	initDatabase()

	r := gin.Default()

	r.GET("/api/health", healthHandler)

	// Redirect endpoint (for actual URL shortening usage)
	r.GET("/:shortCode", func(c *gin.Context) {
		shortCode := c.Param("shortCode")

		if shortCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "short code is required"})
			return
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

		// Redirect to original URL
		c.Redirect(http.StatusFound, urlData.OriginalURL)
	})

	// New endpoint to retrieve original URL by short code
	r.GET("/api/v1/urls/:shortCode", func(c *gin.Context) {
		shortCode := c.Param("shortCode")

		if shortCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "short code is required"})
			return
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

		c.JSON(http.StatusOK, gin.H{
			"id":          urlData.ID,
			"originalUrl": urlData.OriginalURL,
			"shortCode":   urlData.ShortCode,
			"createdAt":   urlData.CreatedAt,
			"updatedAt":   urlData.UpdatedAt,
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
