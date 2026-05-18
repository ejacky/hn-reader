package main

import (
	"flag"
	"log"

	"hn-reader/internal/api"
	"hn-reader/internal/config"
	"hn-reader/internal/service"
	"hn-reader/internal/storage"

	"github.com/gin-gonic/gin"
)

// Version is set at build time via -ldflags "-X main.Version=1.2.0".
var Version = "1.0.0"

func main() {
	installFlag := flag.Bool("install", false, "Install as Windows service")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall Windows service")
	runServiceFlag := flag.Bool("run-service", false, "Run as Windows service (called by SCM)")
	flag.Parse()

	if *installFlag {
		if err := service.Install(); err != nil {
			log.Fatalf("failed to install service: %v", err)
		}
		return
	}

	if *uninstallFlag {
		if err := service.Uninstall(); err != nil {
			log.Fatalf("failed to uninstall service: %v", err)
		}
		return
	}

	cfg := config.Load()

	st, err := storage.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer st.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware())

	server := api.New(cfg, st, Version)
	server.RegisterRoutes(r)

	addr := ":" + cfg.Port

	if *runServiceFlag {
		log.Printf("HN Reader server starting as Windows service on %s", addr)
		if err := service.Run(r, addr); err != nil {
			log.Fatalf("service failed: %v", err)
		}
		return
	}

	log.Printf("HN Reader server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
