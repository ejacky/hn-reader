package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	LLMBaseURL   string
	LLMAPIKey    string
	LLMModel     string
	DatabasePath string
}

func Load() *Config {
	loadedPath := loadEnvFile()
	if loadedPath != "" {
		log.Printf("config: .env loaded from %s", loadedPath)
	} else {
		log.Println("config: no .env file found, using defaults")
	}

	dbDefault := "./hn-reader.db"
	if execPath, err := os.Executable(); err == nil {
		dbDefault = filepath.Join(filepath.Dir(execPath), "hn-reader.db")
	}

	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		LLMBaseURL:   getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:    getEnv("LLM_API_KEY", ""),
		LLMModel:     getEnv("LLM_MODEL", "gpt-4o-mini"),
		DatabasePath: getEnv("DB_PATH", dbDefault),
	}

	if cfg.LLMAPIKey == "" {
		log.Println("config: LLM_API_KEY is empty — summarization will fail")
	}
	return cfg
}

func loadEnvFile() string {
	candidates := []string{}

	// 优先搜索可执行文件所在目录（Windows 服务场景下 cwd 是 System32）
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), ".env"))
		log.Printf("config: executable at %s", execPath)
	} else {
		log.Printf("config: os.Executable() failed: %v", err)
	}

	candidates = append(candidates,
		".env",
		"server/.env",
		filepath.Join(os.Getenv("HOME"), ".hn-reader", ".env"),
		filepath.Join(os.Getenv("USERPROFILE"), ".hn-reader", ".env"),
	)
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				log.Printf("config: found %s but failed to load: %v", path, err)
				return ""
			}
			return path
		} else {
			log.Printf("config: %s not found: %v", path, err)
		}
	}

	log.Printf("config: searched %d paths, no .env found", len(candidates))
	return ""
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
