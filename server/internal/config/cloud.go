package config

import (
	"log"
	"os"
	"time"
)

// LoadCloud loads configuration for the cloud deployment.
// DATABASE_URL and LLM_API_KEY are required; missing either is fatal.
func LoadCloud() *Config {
	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		LLMBaseURL:   getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:    getEnv("LLM_API_KEY", ""),
		LLMModel:     getEnv("LLM_MODEL", "gpt-4o-mini"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RedisURL:     os.Getenv("REDIS_URL"),
		TrustedProxies: splitCSV(getEnv("TRUSTED_PROXIES", "127.0.0.1,172.16.0.0/12")),
		LLMDailyBudget:     getEnvInt("LLM_DAILY_BUDGET", 2000),
		LLMMaxConcurrency:  getEnvInt("LLM_MAX_CONCURRENCY", 4),
		LLMTimeout:         time.Duration(getEnvInt("LLM_TIMEOUT_SECONDS", 90)) * time.Second,
		LLMMaxOutputTokens: getEnvInt("LLM_MAX_OUTPUT_TOKENS", 1500),
		LLMReasoningEffort: getEnv("LLM_REASONING_EFFORT", "none"),
		// Set LLM_REASONING_EFFORT=off to omit the parameter entirely for models
		// or gateways that reject it.
		FreeQuotaLimit:     getEnvInt("FREE_QUOTA_LIMIT", 50),
		MaxBodyBytes:       getEnvInt64("MAX_BODY_BYTES", 256*1024),
		SignupRatePerHour:  getEnvInt("SIGNUP_RATE_PER_HOUR", 5),
		SignupRatePerDay:   getEnvInt("SIGNUP_RATE_PER_DAY", 20),
		APIRatePerMin:      getEnvInt("API_RATE_PER_MIN", 120),
		SummarizeRatePerMin: getEnvInt("SUMMARIZE_RATE_PER_MIN", 20),
		PreloadRatePerMin:   getEnvInt("PRELOAD_RATE_PER_MIN", 10),
		PreloadMaxPerRequest: getEnvInt("PRELOAD_MAX_PER_REQUEST", 20),
		PreloadMaxPending:    getEnvInt("PRELOAD_MAX_PENDING_PER_USER", 20),
		PreloadQueueSize:     getEnvInt("PRELOAD_QUEUE_SIZE", 200),
		HNBaseURL:            getEnv("HN_BASE_URL", "https://hacker-news.firebaseio.com/v0"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.LLMAPIKey == "" {
		log.Fatal("LLM_API_KEY is required")
	}
	if cfg.LLMTimeout <= 0 {
		cfg.LLMTimeout = 90 * time.Second
	}
	if cfg.LLMMaxConcurrency <= 0 {
		cfg.LLMMaxConcurrency = 4
	}
	if cfg.LLMDailyBudget < 0 {
		cfg.LLMDailyBudget = 0
	}
	return cfg
}
