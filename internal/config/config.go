// Package config handles application configuration.
//
// Configuration is loaded from environment variables following the 12-factor app methodology.
// This makes the application easy to configure in different environments (local, staging, production)
// without code changes.
//
// All configuration is validated at startup to fail fast if misconfigured.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration.
// Fields are documented with their environment variable names and defaults.
type Config struct {
	// Environment is the deployment environment: development, staging, production
	// Env var: ENV (default: development)
	Environment string

	// Port is the HTTP server port
	// Env var: PORT (default: 8080)
	Port int

	// DatabaseURL is the PostgreSQL connection string
	// Env var: DATABASE_URL (optional - falls back to in-memory storage)
	// Format: postgres://user:password@host:port/database?sslmode=disable
	DatabaseURL string

	// RedisURL is the Redis connection string for caching
	// Env var: REDIS_URL (optional)
	// Format: redis://user:password@host:port/db
	RedisURL string

	// HiveAPIKey is the API key for Hive AI detection service
	// Env var: HIVE_API_KEY (required for image/video/audio detection)
	HiveAPIKey string

	// OpenAIAPIKey is the API key for OpenAI detection
	// Env var: OPENAI_API_KEY (optional - improves text detection)
	OpenAIAPIKey string

	// GPTZeroAPIKey is the API key for GPTZero detection
	// Env var: GPTZERO_API_KEY (optional - improves text detection)
	GPTZeroAPIKey string

	// MaxUploadSize is the maximum file upload size in bytes
	// Env var: MAX_UPLOAD_SIZE (default: 104857600 = 100MB)
	MaxUploadSize int64

	// RateLimitPerMinute is the maximum requests per minute per IP/API key
	// Env var: RATE_LIMIT_PER_MINUTE (default: 60)
	RateLimitPerMinute int

	// AllowedOrigins is a comma-separated list of allowed CORS origins
	// Env var: ALLOWED_ORIGINS (default: * in development, must be set in production)
	AllowedOrigins []string

	// APIKeyRequired determines if API key authentication is required
	// Env var: API_KEY_REQUIRED (default: false in development, true in production)
	APIKeyRequired bool
}

// Load reads configuration from environment variables.
// Missing optional values get sensible defaults.
// This function never returns an error - use Validate() to check required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Environment:        getEnvOrDefault("ENV", "development"),
		Port:               getEnvAsInt("PORT", 8080),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           os.Getenv("REDIS_URL"),
		HiveAPIKey:         os.Getenv("HIVE_API_KEY"),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		GPTZeroAPIKey:      os.Getenv("GPTZERO_API_KEY"),
		MaxUploadSize:      getEnvAsInt64("MAX_UPLOAD_SIZE", 100*1024*1024), // 100MB
		RateLimitPerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 60),
		AllowedOrigins:     getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),
		APIKeyRequired:     getEnvAsBool("API_KEY_REQUIRED", false),
	}

	// Production defaults
	if cfg.IsProduction() {
		if cfg.AllowedOrigins[0] == "*" {
			cfg.AllowedOrigins = []string{} // Require explicit origins in production
		}
		cfg.APIKeyRequired = true
	}

	return cfg, nil
}

// Validate checks that all required configuration is present and valid.
// Returns an error describing what's missing or invalid.
func (c *Config) Validate() error {
	var errors []string

	// Port must be valid
	if c.Port < 1 || c.Port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid port: %d (must be 1-65535)", c.Port))
	}

	// Environment must be recognized
	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[c.Environment] {
		errors = append(errors, fmt.Sprintf("invalid environment: %s (must be development, staging, or production)", c.Environment))
	}

	// Production requirements
	if c.IsProduction() {
		if c.DatabaseURL == "" {
			errors = append(errors, "DATABASE_URL is required in production")
		}
		if len(c.AllowedOrigins) == 0 {
			errors = append(errors, "ALLOWED_ORIGINS must be set in production (not *)")
		}
	}

	// At least one detection backend should be configured
	if c.HiveAPIKey == "" && c.OpenAIAPIKey == "" && c.GPTZeroAPIKey == "" {
		// This is a warning, not an error - we can still run with mock detection
		// In production this would be an error
		if c.IsProduction() {
			errors = append(errors, "at least one detection API key is required (HIVE_API_KEY, OPENAI_API_KEY, or GPTZERO_API_KEY)")
		}
	}

	// MaxUploadSize must be reasonable
	if c.MaxUploadSize < 1024 { // Less than 1KB
		errors = append(errors, fmt.Sprintf("MAX_UPLOAD_SIZE too small: %d (minimum 1024)", c.MaxUploadSize))
	}
	if c.MaxUploadSize > 1024*1024*1024 { // More than 1GB
		errors = append(errors, fmt.Sprintf("MAX_UPLOAD_SIZE too large: %d (maximum 1GB)", c.MaxUploadSize))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// IsProduction returns true if running in production environment.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development environment.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// getEnvOrDefault returns the environment variable value or a default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt returns the environment variable as an integer or a default if not set/invalid.
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsInt64 returns the environment variable as an int64 or a default if not set/invalid.
func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBool returns the environment variable as a boolean or a default if not set.
// Accepts: true, false, 1, 0, yes, no (case-insensitive)
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return defaultValue
}

// getEnvAsSlice returns the environment variable as a string slice, split by comma.
func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}
