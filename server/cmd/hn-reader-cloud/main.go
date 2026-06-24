package main

import (
	"log"
	"os"

	"hn-reader/internal/api"
	"hn-reader/internal/config"
	"hn-reader/internal/storage"

	"github.com/gin-gonic/gin"
)

const version = "1.1.0"

func main() {
	cfg := config.LoadCloud()

	repo, err := storage.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer repo.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	server := api.NewCloud(cfg, repo, version)
	server.RegisterRoutes(r)

	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	log.Printf("HN Reader Cloud v%s listening on :%s", version, port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
		os.Exit(1)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
