package config

import (
	"os"
	"testing"
)

// TestLoad verifies that configuration loads correctly from environment variables.
func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range originalEnv {
			pair := splitEnvPair(e)
			os.Setenv(pair[0], pair[1])
		}
	}()

	t.Run("loads defaults when no env vars set", func(t *testing.T) {
		os.Clearenv()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		// Check defaults
		assertEqual(t, "Environment", cfg.Environment, "development")
		assertEqual(t, "Port", cfg.Port, 8080)
		assertEqual(t, "MaxUploadSize", cfg.MaxUploadSize, int64(100*1024*1024))
		assertEqual(t, "RateLimitPerMinute", cfg.RateLimitPerMinute, 60)
	})

	t.Run("loads values from environment", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("ENV", "production")
		os.Setenv("PORT", "3000")
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("HIVE_API_KEY", "test-hive-key")
		os.Setenv("MAX_UPLOAD_SIZE", "52428800") // 50MB
		os.Setenv("RATE_LIMIT_PER_MINUTE", "120")
		os.Setenv("ALLOWED_ORIGINS", "https://example.com,https://app.example.com")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		assertEqual(t, "Environment", cfg.Environment, "production")
		assertEqual(t, "Port", cfg.Port, 3000)
		assertEqual(t, "DatabaseURL", cfg.DatabaseURL, "postgres://localhost/test")
		assertEqual(t, "HiveAPIKey", cfg.HiveAPIKey, "test-hive-key")
		assertEqual(t, "MaxUploadSize", cfg.MaxUploadSize, int64(52428800))
		assertEqual(t, "RateLimitPerMinute", cfg.RateLimitPerMinute, 120)

		if len(cfg.AllowedOrigins) != 2 {
			t.Errorf("AllowedOrigins: expected 2, got %d", len(cfg.AllowedOrigins))
		}
		if cfg.AllowedOrigins[0] != "https://example.com" {
			t.Errorf("AllowedOrigins[0]: expected https://example.com, got %s", cfg.AllowedOrigins[0])
		}
	})

	t.Run("handles invalid integer values gracefully", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("PORT", "not-a-number")
		os.Setenv("MAX_UPLOAD_SIZE", "invalid")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		// Should fall back to defaults
		assertEqual(t, "Port", cfg.Port, 8080)
		assertEqual(t, "MaxUploadSize", cfg.MaxUploadSize, int64(100*1024*1024))
	})
}

// TestValidate verifies configuration validation.
func TestValidate(t *testing.T) {
	t.Run("accepts valid development config", func(t *testing.T) {
		cfg := &Config{
			Environment:        "development",
			Port:               8080,
			MaxUploadSize:      100 * 1024 * 1024,
			RateLimitPerMinute: 60,
			AllowedOrigins:     []string{"*"},
		}

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid config: %v", err)
		}
	})

	t.Run("rejects invalid port", func(t *testing.T) {
		cfg := &Config{
			Environment:   "development",
			Port:          0,
			MaxUploadSize: 100 * 1024 * 1024,
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should reject port 0")
		}

		cfg.Port = 70000
		err = cfg.Validate()
		if err == nil {
			t.Error("Validate() should reject port > 65535")
		}
	})

	t.Run("rejects invalid environment", func(t *testing.T) {
		cfg := &Config{
			Environment:   "invalid",
			Port:          8080,
			MaxUploadSize: 100 * 1024 * 1024,
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should reject invalid environment")
		}
	})

	t.Run("requires DATABASE_URL in production", func(t *testing.T) {
		cfg := &Config{
			Environment:    "production",
			Port:           8080,
			MaxUploadSize:  100 * 1024 * 1024,
			AllowedOrigins: []string{"https://example.com"},
			HiveAPIKey:     "test-key",
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should require DATABASE_URL in production")
		}
	})

	t.Run("requires detection API key in production", func(t *testing.T) {
		cfg := &Config{
			Environment:    "production",
			Port:           8080,
			MaxUploadSize:  100 * 1024 * 1024,
			DatabaseURL:    "postgres://localhost/test",
			AllowedOrigins: []string{"https://example.com"},
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should require detection API key in production")
		}
	})

	t.Run("rejects too small MaxUploadSize", func(t *testing.T) {
		cfg := &Config{
			Environment:   "development",
			Port:          8080,
			MaxUploadSize: 100, // Too small
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should reject MaxUploadSize < 1024")
		}
	})

	t.Run("rejects too large MaxUploadSize", func(t *testing.T) {
		cfg := &Config{
			Environment:   "development",
			Port:          8080,
			MaxUploadSize: 2 * 1024 * 1024 * 1024, // 2GB - too large
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should reject MaxUploadSize > 1GB")
		}
	})
}

// TestIsProduction verifies environment detection.
func TestIsProduction(t *testing.T) {
	tests := []struct {
		env      string
		isProd   bool
		isDev    bool
	}{
		{"production", true, false},
		{"development", false, true},
		{"staging", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := &Config{Environment: tt.env}

			if cfg.IsProduction() != tt.isProd {
				t.Errorf("IsProduction(): expected %v, got %v", tt.isProd, cfg.IsProduction())
			}
			if cfg.IsDevelopment() != tt.isDev {
				t.Errorf("IsDevelopment(): expected %v, got %v", tt.isDev, cfg.IsDevelopment())
			}
		})
	}
}

// TestGetEnvAsBool verifies boolean environment variable parsing.
func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"NO", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			os.Setenv("TEST_BOOL", tt.value)
			defer os.Unsetenv("TEST_BOOL")

			result := getEnvAsBool("TEST_BOOL", !tt.expected)
			if result != tt.expected {
				t.Errorf("getEnvAsBool(%q): expected %v, got %v", tt.value, tt.expected, result)
			}
		})
	}

	t.Run("returns default for unset var", func(t *testing.T) {
		os.Unsetenv("TEST_BOOL_UNSET")

		if result := getEnvAsBool("TEST_BOOL_UNSET", true); result != true {
			t.Errorf("expected default true, got false")
		}
		if result := getEnvAsBool("TEST_BOOL_UNSET", false); result != false {
			t.Errorf("expected default false, got true")
		}
	})
}

// TestGetEnvAsSlice verifies slice environment variable parsing.
func TestGetEnvAsSlice(t *testing.T) {
	t.Run("parses comma-separated values", func(t *testing.T) {
		os.Setenv("TEST_SLICE", "a,b,c")
		defer os.Unsetenv("TEST_SLICE")

		result := getEnvAsSlice("TEST_SLICE", nil)
		if len(result) != 3 {
			t.Fatalf("expected 3 items, got %d", len(result))
		}
		if result[0] != "a" || result[1] != "b" || result[2] != "c" {
			t.Errorf("unexpected values: %v", result)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		os.Setenv("TEST_SLICE", " a , b , c ")
		defer os.Unsetenv("TEST_SLICE")

		result := getEnvAsSlice("TEST_SLICE", nil)
		if result[0] != "a" || result[1] != "b" || result[2] != "c" {
			t.Errorf("whitespace not trimmed: %v", result)
		}
	})

	t.Run("returns default for unset var", func(t *testing.T) {
		os.Unsetenv("TEST_SLICE_UNSET")

		defaultVal := []string{"default"}
		result := getEnvAsSlice("TEST_SLICE_UNSET", defaultVal)
		if len(result) != 1 || result[0] != "default" {
			t.Errorf("expected default value, got %v", result)
		}
	})
}

// Helper functions for tests

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: expected %v, got %v", name, want, got)
	}
}

func splitEnvPair(env string) [2]string {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return [2]string{env[:i], env[i+1:]}
		}
	}
	return [2]string{env, ""}
}
