package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"hn-reader/internal/api"
	"hn-reader/internal/config"
	"hn-reader/internal/service"
	"hn-reader/internal/storage"

	"github.com/gin-gonic/gin"
)

// Version is set at build time via -ldflags "-X main.Version=1.3.1".
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

	if *runServiceFlag {
		setupLogging()
	}

	cfg := config.LoadLocal()

	repo, err := storage.NewSqlite(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer repo.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware())

	server := api.NewLocal(cfg, repo, Version)
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

func setupLogging() {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)
	if err := os.Chdir(execDir); err != nil {
		log.Printf("chdir to %s: %v", execDir, err)
		return
	}
	logFile, err := os.OpenFile(
		filepath.Join(execDir, "hn-reader.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
	)
	if err != nil {
		return
	}
	log.SetOutput(logFile)
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
