package config

import (
	"log"
	"os"
)

// LoadCloud loads configuration for the cloud deployment.
// DATABASE_URL is required; missing it is fatal.
func LoadCloud() *Config {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		LLMBaseURL:  getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:   getEnv("LLM_API_KEY", ""),
		LLMModel:    getEnv("LLM_MODEL", "gpt-4o-mini"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.LLMAPIKey == "" {
		log.Println("config: LLM_API_KEY is empty — summarization will fail")
	}
	return cfg
}
