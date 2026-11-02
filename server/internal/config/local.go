package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadLocal loads configuration for the local desktop deployment.
// It searches for .env files in several locations and uses SQLite.
func LoadLocal() *Config {
	loadEnvFile()

	dbDefault := "./hn-reader.db"
	if execPath, err := os.Executable(); err == nil {
		dbDefault = filepath.Join(filepath.Dir(execPath), "hn-reader.db")
	}

	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		LLMBaseURL:   getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:    getEnv("LLM_API_KEY", ""),
		LLMModel:     getEnv("LLM_MODEL", "gpt-4o-mini"),
		// Local mode leaves this empty by default: the target model may not be a
		// reasoning model, and some providers reject the parameter outright.
		LLMReasoningEffort: getEnv("LLM_REASONING_EFFORT", ""),
		DatabasePath: getEnv("DB_PATH", dbDefault),
	}

	if cfg.LLMAPIKey == "" {
		log.Println("config: LLM_API_KEY is empty — summarization will fail")
	}
	return cfg
}

func loadEnvFile() {
	candidates := []string{}

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
				return
			}
			log.Printf("config: .env loaded from %s", path)
			return
		}
	}

	log.Println("config: no .env file found, using defaults")
}
