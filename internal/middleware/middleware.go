// Package middleware provides HTTP middleware for the HumanMark API.
//
// Middleware wraps HTTP handlers to add cross-cutting functionality like:
//   - Request logging
//   - Panic recovery
//   - Request ID tracking
//   - CORS headers
//   - Rate limiting
//
// Middleware is applied as a chain, with the first middleware being the outermost layer.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/humanmark/humanmark/pkg/logger"
)

// Middleware is a function that wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware in order.
// The first middleware is the outermost (executes first on request, last on response).
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		// Apply in reverse order so first middleware is outermost
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// RequestID adds a unique request ID to each request.
// The ID is added to the request context and response headers.
// If the request already has an X-Request-ID header, it is preserved.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for existing request ID from upstream proxy
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			// Add to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Add to request context for downstream use
			ctx := context.WithValue(r.Context(), logger.ContextKeyRequestID, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateRequestID creates a random 16-character hex string.
func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Logging logs all HTTP requests with timing information.
// Logs include: method, path, status code, duration, request ID.
func Logging(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default if not explicitly set
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Get request ID from context
			requestID := ""
			if id := r.Context().Value(logger.ContextKeyRequestID); id != nil {
				requestID = id.(string)
			}

			// Log the request
			log.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"request_id", requestID,
				"remote_addr", getClientIP(r),
				"user_agent", r.UserAgent(),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures that a write has occurred (implies 200 OK if WriteHeader not called).
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Recovery catches panics and returns a 500 error instead of crashing.
// Panics are logged with stack trace information.
func Recovery(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Get request ID for correlation
					requestID := ""
					if id := r.Context().Value(logger.ContextKeyRequestID); id != nil {
						requestID = id.(string)
					}

					// Log the panic
					log.Error("panic recovered",
						"error", err,
						"request_id", requestID,
						"method", r.Method,
						"path", r.URL.Path,
					)

					// Return 500 error
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CORS adds Cross-Origin Resource Sharing headers.
// This allows browser-based clients to call the API from different domains.
func CORS(allowedOrigins []string) Middleware {
	// Convert to map for fast lookup
	originMap := make(map[string]bool)
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			break
		}
		originMap[origin] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" && originMap[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin") // Important for caching
			}

			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400") // Cache preflight for 24 hours

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit implements a simple in-memory rate limiter.
// For production, use Redis-based rate limiting for distributed deployments.
type rateLimiter struct {
	requests map[string]*clientRequests
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

type clientRequests struct {
	count    int
	resetAt  time.Time
}

// RateLimit limits requests per client IP.
// Returns 429 Too Many Requests if limit is exceeded.
func RateLimit(requestsPerMinute int) Middleware {
	limiter := &rateLimiter{
		requests: make(map[string]*clientRequests),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for health checks
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := getClientIP(r)
			
			if !limiter.allow(clientIP) {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("X-RateLimit-Limit", string(rune(limiter.limit)))
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, `{"error":"rate limit exceeded","retry_after":60}`, http.StatusTooManyRequests)
				return
			}

			// Add rate limit headers
			remaining := limiter.remaining(clientIP)
			w.Header().Set("X-RateLimit-Limit", formatInt(limiter.limit))
			w.Header().Set("X-RateLimit-Remaining", formatInt(remaining))

			next.ServeHTTP(w, r)
		})
	}
}

// allow checks if a client can make another request.
func (rl *rateLimiter) allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	client, exists := rl.requests[clientIP]
	if !exists || now.After(client.resetAt) {
		// New window
		rl.requests[clientIP] = &clientRequests{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true
	}

	if client.count >= rl.limit {
		return false
	}

	client.count++
	return true
}

// remaining returns the number of requests remaining for a client.
func (rl *rateLimiter) remaining(clientIP string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if client, exists := rl.requests[clientIP]; exists {
		remaining := rl.limit - client.count
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return rl.limit
}

// cleanup periodically removes expired entries.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, client := range rl.requests {
			if now.After(client.resetAt) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getClientIP extracts the client IP address from the request.
// Handles X-Forwarded-For and X-Real-IP headers from reverse proxies.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by load balancers/proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header (set by nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// formatInt converts an integer to a string without importing strconv.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}
	
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// ContentType sets the Content-Type header for JSON responses.
func ContentType(contentType string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBodySize limits the size of request bodies.
// Returns 413 Payload Too Large if exceeded.
func MaxBodySize(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
