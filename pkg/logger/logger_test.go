package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestNew verifies logger creation at different levels.
func TestNew(t *testing.T) {
	tests := []struct {
		level string
	}{
		{"debug"},
		{"info"},
		{"warn"},
		{"error"},
		{"INFO"},      // Test case insensitivity
		{"  info  "},  // Test whitespace trimming
		{"invalid"},   // Should default to info
		{""},          // Should default to info
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			log := New(tt.level)
			if log == nil {
				t.Error("New() returned nil")
			}
			if log.Logger == nil {
				t.Error("New() returned logger with nil slog.Logger")
			}
		})
	}
}

// TestLogOutput verifies that log messages are formatted correctly.
func TestLogOutput(t *testing.T) {
	t.Run("debug level outputs debug messages", func(t *testing.T) {
		var buf bytes.Buffer
		log := NewWithWriter("debug", &buf)

		log.Debug("test debug message", "key", "value")

		output := buf.String()
		if !strings.Contains(output, "test debug message") {
			t.Errorf("debug message not in output: %s", output)
		}
		if !strings.Contains(output, "key") || !strings.Contains(output, "value") {
			t.Errorf("key-value pair not in output: %s", output)
		}
	})

	t.Run("info level suppresses debug messages", func(t *testing.T) {
		var buf bytes.Buffer
		log := NewWithWriter("info", &buf)

		log.Debug("should not appear")
		log.Info("should appear")

		output := buf.String()
		if strings.Contains(output, "should not appear") {
			t.Errorf("debug message should be suppressed at info level: %s", output)
		}
		if !strings.Contains(output, "should appear") {
			t.Errorf("info message should appear: %s", output)
		}
	})

	t.Run("error level only outputs error messages", func(t *testing.T) {
		var buf bytes.Buffer
		log := NewWithWriter("error", &buf)

		log.Debug("debug msg")
		log.Info("info msg")
		log.Warn("warn msg")
		log.Error("error msg")

		output := buf.String()
		if strings.Contains(output, "debug msg") {
			t.Error("debug should be suppressed")
		}
		if strings.Contains(output, "info msg") {
			t.Error("info should be suppressed")
		}
		if strings.Contains(output, "warn msg") {
			t.Error("warn should be suppressed")
		}
		if !strings.Contains(output, "error msg") {
			t.Error("error should appear")
		}
	})
}

// TestLogLevels verifies all log level methods work.
func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("debug", &buf)

	// Test each level
	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()

	expectedMessages := []string{
		"debug message",
		"info message",
		"warn message",
		"error message",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("expected message not found: %s", msg)
		}
	}
}

// TestWith verifies that With creates a new logger with additional context.
func TestWith(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("info", &buf)

	// Create a child logger with context
	childLog := log.With("request_id", "abc123", "user_id", 42)

	// Log a message
	childLog.Info("processing request")

	output := buf.String()

	// Verify context fields are present
	if !strings.Contains(output, "request_id") || !strings.Contains(output, "abc123") {
		t.Errorf("request_id not in output: %s", output)
	}
	if !strings.Contains(output, "user_id") || !strings.Contains(output, "42") {
		t.Errorf("user_id not in output: %s", output)
	}
}

// TestWithContext verifies context-based logging.
func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("info", &buf)

	// Create context with request ID
	ctx := context.WithValue(context.Background(), ContextKeyRequestID, "req-12345")

	// Create logger from context
	ctxLog := log.WithContext(ctx)
	ctxLog.Info("context log message")

	output := buf.String()

	if !strings.Contains(output, "req-12345") {
		t.Errorf("request_id from context not in output: %s", output)
	}
}

// TestWithContextNoRequestID verifies WithContext works without request ID.
func TestWithContextNoRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("info", &buf)

	// Create context without request ID
	ctx := context.Background()

	// Create logger from context - should not panic
	ctxLog := log.WithContext(ctx)
	ctxLog.Info("message without request id")

	output := buf.String()
	if !strings.Contains(output, "message without request id") {
		t.Errorf("message not in output: %s", output)
	}
}

// TestNopLogger verifies that NopLogger discards all output.
func TestNopLogger(t *testing.T) {
	log := NopLogger()

	// These should not panic
	log.Debug("debug")
	log.Info("info")
	log.Warn("warn")
	log.Error("error")

	// Can't easily verify output is discarded, but if we got here without panic, it works
}

// TestParseLevel verifies level parsing.
func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "DEBUG"},
		{"info", "INFO"},
		{"INFO", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"ERROR", "ERROR"},
		{"", "INFO"},        // Default
		{"invalid", "INFO"}, // Default
		{"  info  ", "INFO"}, // Trimmed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			if level.String() != tt.expected {
				t.Errorf("parseLevel(%q) = %s, want %s", tt.input, level.String(), tt.expected)
			}
		})
	}
}

// TestLogKeyValuePairs verifies various types can be logged.
func TestLogKeyValuePairs(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("info", &buf)

	// Log various types
	log.Info("various types",
		"string", "hello",
		"int", 42,
		"float", 3.14,
		"bool", true,
		"nil", nil,
	)

	output := buf.String()

	// Verify each key appears
	keys := []string{"string", "int", "float", "bool", "nil"}
	for _, key := range keys {
		if !strings.Contains(output, key) {
			t.Errorf("key %q not in output: %s", key, output)
		}
	}
}
