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

// Version is set at build time via -ldflags "-X main.Version=1.3.4".
var Version = "1.0.0"

func main() {
	installFlag := flag.Bool("install", false, "Install as Windows service (legacy)")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall Windows service (legacy)")
	runServiceFlag := flag.Bool("run-service", false, "Run as Windows service (called by SCM)")
	backgroundFlag := flag.Bool("background", false, "Run in background without a console (used by the per-user startup entry)")
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

	dataDir := config.LocalDataDir()
	if *runServiceFlag || *backgroundFlag {
		setupLogging(dataDir)
	}

	cfg := config.LoadLocal()
	log.Printf("data dir: %s", cfg.DataDir)
	log.Printf("database: %s", cfg.DatabasePath)

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
		log.Printf("HN Reader server v%s starting as Windows service on %s", Version, addr)
		if err := service.Run(r, addr); err != nil {
			log.Fatalf("service failed: %v", err)
		}
		return
	}

	if *backgroundFlag {
		log.Printf("HN Reader server v%s starting in background on %s", Version, addr)
	} else {
		log.Printf("HN Reader server v%s starting on %s", Version, addr)
	}
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func setupLogging(dataDir string) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("create data dir %s: %v", dataDir, err)
	}
	logFile, err := os.OpenFile(
		filepath.Join(dataDir, "hn-reader.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644,
	)
	if err != nil {
		log.Printf("open log file in %s: %v", dataDir, err)
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
