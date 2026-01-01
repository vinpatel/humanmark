// Package logger provides structured logging for the HumanMark API.
//
// This logger outputs JSON in production for easy parsing by log aggregators (DataDog, Splunk, etc.)
// and human-readable format in development.
//
// Usage:
//
//	log := logger.New("info")
//	log.Info("user created", "user_id", 123, "email", "user@example.com")
//
// Output (production):
//
//	{"level":"info","msg":"user created","user_id":123,"email":"user@example.com","time":"2025-01-01T00:00:00Z"}
//
// Output (development):
//
//	2025/01/01 00:00:00 INFO user created user_id=123 email=user@example.com
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger is a structured logger wrapper.
// It provides a simple interface for logging at different levels with key-value pairs.
type Logger struct {
	*slog.Logger
}

// New creates a new Logger with the specified level.
// Valid levels: debug, info, warn, error (case-insensitive)
// Defaults to info if invalid level provided.
func New(level string) *Logger {
	return NewWithWriter(level, os.Stdout)
}

// NewWithWriter creates a new Logger that writes to the specified writer.
// Useful for testing or writing to files.
func NewWithWriter(level string, w io.Writer) *Logger {
	lvl := parseLevel(level)

	// Use JSON handler in production, text handler in development
	var handler slog.Handler
	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level:     lvl,
			AddSource: false, // Don't add source file info in production
		})
	} else {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{
			Level:     lvl,
			AddSource: false,
		})
	}

	return &Logger{slog.New(handler)}
}

// parseLevel converts a string level to slog.Level.
// Defaults to Info if the level string is not recognized.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Debug logs at debug level.
// Use for detailed debugging information.
// Example: log.Debug("processing item", "item_id", 42, "status", "pending")
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Info logs at info level.
// Use for general operational information.
// Example: log.Info("server started", "port", 8080)
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn logs at warn level.
// Use for potentially harmful situations.
// Example: log.Warn("rate limit approaching", "current", 58, "limit", 60)
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs at error level.
// Use for error events that might still allow the application to continue.
// Example: log.Error("failed to send email", "error", err, "user_id", 123)
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// With returns a new Logger with the given attributes added.
// Useful for adding context that applies to multiple log calls.
// Example:
//
//	reqLog := log.With("request_id", "abc123", "user_id", 456)
//	reqLog.Info("processing request")
//	reqLog.Info("request completed")
func (l *Logger) With(args ...any) *Logger {
	return &Logger{l.Logger.With(args...)}
}

// WithContext returns a new Logger with context values added.
// Extracts request_id if present in context.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Check for request ID in context
	if reqID := ctx.Value(ContextKeyRequestID); reqID != nil {
		return l.With("request_id", reqID)
	}
	return l
}

// ContextKey is the type for context keys to avoid collisions.
type ContextKey string

// Context keys for common values
const (
	ContextKeyRequestID ContextKey = "request_id"
	ContextKeyUserID    ContextKey = "user_id"
	ContextKeyAPIKey    ContextKey = "api_key"
)

// NopLogger returns a logger that discards all output.
// Useful for testing when log output is not needed.
func NopLogger() *Logger {
	return NewWithWriter("error", io.Discard)
}
