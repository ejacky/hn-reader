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

	dbDefault := "./hn-reader.db"
	if execPath, err := os.Executable(); err == nil {
		dbDefault = filepath.Join(filepath.Dir(execPath), "hn-reader.db")
	}

	return &Config{
		Port:         getEnv("PORT", "8080"),
		LLMBaseURL:   getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:    getEnv("LLM_API_KEY", ""),
		LLMModel:     getEnv("LLM_MODEL", "gpt-4o-mini"),
		DatabasePath: getEnv("DB_PATH", dbDefault),
	}
}

func loadEnvFile() error {
	candidates := []string{}

	// 优先搜索可执行文件所在目录（Windows 服务场景下 cwd 是 System32）
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), ".env"))
	}

	candidates = append(candidates,
		".env",
		"server/.env",
		filepath.Join(os.Getenv("HOME"), ".hn-reader", ".env"),
		filepath.Join(os.Getenv("USERPROFILE"), ".hn-reader", ".env"),
	)
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
