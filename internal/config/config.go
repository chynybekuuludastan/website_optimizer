package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration
type Config struct {
	// Server
	Port         string
	Environment  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Database
	PostgresURI string
	RedisURI    string

	// JWT
	JWTSecret     string
	JWTExpiration time.Duration

	// External APIs
	OpenAIAPIKey         string
	LighthouseURL        string
	LighthouseAPIKey     string
	LighthouseMobileMode bool
	LighthouseTimeout    int

	// Cache
	CacheTTL time.Duration

	// Analysis
	AnalysisTimeout time.Duration
}

// NewConfig creates a new configuration from environment variables
func NewConfig() *Config {
	readTimeoutSec, _ := strconv.Atoi(getEnv("READ_TIMEOUT", "5"))
	writeTimeoutSec, _ := strconv.Atoi(getEnv("WRITE_TIMEOUT", "10"))
	jwtExpirationHours, _ := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "24"))
	cacheTTLMin, _ := strconv.Atoi(getEnv("CACHE_TTL_MINUTES", "10"))
	analysisTimeoutSec, _ := strconv.Atoi(getEnv("ANALYSIS_TIMEOUT", "60"))

	return &Config{
		// Server
		Port:         getEnv("PORT", "8080"),
		Environment:  getEnv("ENVIRONMENT", "development"),
		ReadTimeout:  time.Duration(readTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(writeTimeoutSec) * time.Second,

		// Database
		PostgresURI: getEnv("POSTGRES_URI", "postgres://postgres:postgres@localhost:5432/website_analyzer?sslmode=disable"),
		RedisURI:    getEnv("REDIS_URI", "redis://localhost:6379/0"),

		// JWT
		JWTSecret:     getEnv("JWT_SECRET", "your-secret-key"),
		JWTExpiration: time.Duration(jwtExpirationHours) * time.Hour,

		// External APIs
		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
		LighthouseURL:    getEnv("LIGHTHOUSE_API_URL", "https://www.googleapis.com/pagespeedonline/v5/runPagespeed"),
		LighthouseAPIKey: getEnv("LIGHTHOUSE_API_KEY", "default-key"),
		LighthouseTimeout: func() int {
			timeout, _ := strconv.Atoi(getEnv("LIGHTHOUSE_TIMEOUT", "60"))
			return timeout
		}(),

		// Cache
		CacheTTL: time.Duration(cacheTTLMin) * time.Minute,

		// Analysis
		AnalysisTimeout: time.Duration(analysisTimeoutSec) * time.Second,
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
