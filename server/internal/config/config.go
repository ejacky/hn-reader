package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

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

	// Cloud hardening / cost controls.
	TrustedProxies        []string
	LLMDailyBudget        int
	LLMMaxConcurrency     int
	LLMTimeout            time.Duration
	LLMMaxOutputTokens    int
	// LLMReasoningEffort is forwarded as reasoning_effort when non-empty.
	// Summary work does not need chain-of-thought, so "none" is the recommended
	// setting for reasoning models: it removes the empty-content failure mode
	// and cuts output cost.
	LLMReasoningEffort    string
	FreeQuotaLimit        int
	MaxBodyBytes          int64
	SignupRatePerHour     int
	SignupRatePerDay      int
	APIRatePerMin         int
	SummarizeRatePerMin   int
	PreloadRatePerMin     int
	PreloadMaxPerRequest  int
	PreloadMaxPending     int
	PreloadQueueSize      int
	HNBaseURL             string
}

// getEnv reads an environment variable or returns a default.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return defaultVal
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
