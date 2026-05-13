package config

import (
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
	_ = loadEnvFile()

	return &Config{
		Port:         getEnv("PORT", "8080"),
		LLMBaseURL:   getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:    getEnv("LLM_API_KEY", ""),
		LLMModel:     getEnv("LLM_MODEL", "gpt-4o-mini"),
		DatabasePath: getEnv("DB_PATH", "./hn-reader.db"),
	}
}

func loadEnvFile() error {
	// 尝试从多个位置加载 .env 文件
	candidates := []string{
		".env",
		"server/.env",
		filepath.Join(os.Getenv("HOME"), ".hn-reader", ".env"),
		filepath.Join(os.Getenv("USERPROFILE"), ".hn-reader", ".env"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return godotenv.Load(path)
		}
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
