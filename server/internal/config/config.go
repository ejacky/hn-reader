package config

import "os"

// Config holds all server configuration, shared by local and cloud modes.
// Fields specific to one mode are simply left at their zero value in the other.
type Config struct {
	Port         string
	LLMBaseURL   string
	LLMAPIKey    string
	LLMModel     string
	DatabasePath string // local SQLite path
	DatabaseURL  string // cloud Postgres DSN
	RedisURL     string // cloud Redis URL (reserved)
}

// getEnv reads an environment variable or returns a default.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
