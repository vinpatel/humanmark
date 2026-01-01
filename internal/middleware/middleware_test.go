package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/humanmark/humanmark/pkg/logger"
)

// TestRequestID verifies request ID generation and propagation.
func TestRequestID(t *testing.T) {
	t.Run("generates request ID when not present", func(t *testing.T) {
		handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request ID is in context
			if r.Context().Value(logger.ContextKeyRequestID) == nil {
				t.Error("request ID not in context")
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Verify request ID in response header
		if rec.Header().Get("X-Request-ID") == "" {
			t.Error("X-Request-ID header not set")
		}

		// Verify it's a valid hex string (16 chars)
		reqID := rec.Header().Get("X-Request-ID")
		if len(reqID) != 16 {
			t.Errorf("expected 16 char request ID, got %d: %s", len(reqID), reqID)
		}
	})

	t.Run("preserves existing request ID", func(t *testing.T) {
		existingID := "existing-request-id"

		handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Context().Value(logger.ContextKeyRequestID)
			if id != existingID {
				t.Errorf("expected %s, got %v", existingID, id)
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Request-ID", existingID)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-Request-ID") != existingID {
			t.Errorf("expected %s, got %s", existingID, rec.Header().Get("X-Request-ID"))
		}
	})
}

// TestLogging verifies request logging.
func TestLogging(t *testing.T) {
	t.Run("logs request details", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.NewWithWriter("info", &buf)

		handler := Logging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test-path", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		output := buf.String()
		if !strings.Contains(output, "GET") {
			t.Error("method not logged")
		}
		if !strings.Contains(output, "/test-path") {
			t.Error("path not logged")
		}
		if !strings.Contains(output, "200") {
			t.Error("status code not logged")
		}
	})

	t.Run("captures correct status code", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.NewWithWriter("info", &buf)

		handler := Logging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !strings.Contains(buf.String(), "404") {
			t.Error("status 404 not logged")
		}
	})
}

// TestRecovery verifies panic recovery.
func TestRecovery(t *testing.T) {
	t.Run("recovers from panic", func(t *testing.T) {
		log := logger.NopLogger()

		handler := Recovery(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(rec, req)

		// Should return 500
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}
	})

	t.Run("logs panic", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.NewWithWriter("error", &buf)

		handler := Recovery(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic message")
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !strings.Contains(buf.String(), "panic") {
			t.Error("panic not logged")
		}
	})
}

// TestCORS verifies CORS header handling.
func TestCORS(t *testing.T) {
	t.Run("allows all origins when configured", func(t *testing.T) {
		handler := CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("expected * for Access-Control-Allow-Origin")
		}
	})

	t.Run("allows specific origins", func(t *testing.T) {
		handler := CORS([]string{"https://allowed.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Allowed origin
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://allowed.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "https://allowed.com" {
			t.Error("allowed origin not reflected")
		}
	})

	t.Run("rejects disallowed origins", func(t *testing.T) {
		handler := CORS([]string{"https://allowed.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://disallowed.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("disallowed origin should not have CORS header")
		}
	})

	t.Run("handles preflight requests", func(t *testing.T) {
		handler := CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("preflight should return 204, got %d", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("Access-Control-Allow-Methods not set")
		}
	})
}

// TestRateLimit verifies rate limiting.
func TestRateLimit(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		handler := RateLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("request %d should be allowed, got %d", i, rec.Code)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		handler := RateLimit(3)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Make requests until rate limited
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.2:12345" // Different IP to avoid previous test
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if i < 3 && rec.Code != http.StatusOK {
				t.Errorf("request %d should be allowed", i)
			}
			if i >= 3 && rec.Code != http.StatusTooManyRequests {
				t.Errorf("request %d should be rate limited, got %d", i, rec.Code)
			}
		}
	})

	t.Run("skips rate limiting for health checks", func(t *testing.T) {
		handler := RateLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Make many health check requests
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/health", nil)
			req.RemoteAddr = "192.168.1.3:12345"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("health check %d should not be rate limited", i)
			}
		}
	})

	t.Run("rate limits per IP", func(t *testing.T) {
		handler := RateLimit(2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// IP 1 - should be rate limited after 2
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.0.0.1:12345"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if i == 2 && rec.Code != http.StatusTooManyRequests {
				t.Error("IP 1 request 3 should be rate limited")
			}
		}

		// IP 2 - should still be allowed
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.2:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Error("different IP should not be rate limited")
		}
	})
}

// TestGetClientIP verifies client IP extraction.
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		xff         string
		xRealIP     string
		expectedIP  string
	}{
		{
			name:       "uses RemoteAddr when no headers",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "uses X-Forwarded-For first IP",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.195, 70.41.3.18, 150.172.238.178",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "uses X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.50",
			expectedIP: "203.0.113.50",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.195",
			xRealIP:    "203.0.113.50",
			expectedIP: "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// TestChain verifies middleware chaining.
func TestChain(t *testing.T) {
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := Chain(mw1, mw2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d items, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %s, got %s", i, v, order[i])
		}
	}
}

// TestMaxBodySize verifies request body size limiting.
func TestMaxBodySize(t *testing.T) {
	t.Run("allows small body", func(t *testing.T) {
		handler := MaxBodySize(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := make([]byte, 100)
			_, err := r.Body.Read(buf)
			if err != nil && err.Error() != "EOF" {
				http.Error(w, "read error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))

		body := bytes.NewReader([]byte("small body"))
		req := httptest.NewRequest("POST", "/", body)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

// TestFormatInt verifies integer formatting.
func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{12345, "12345"},
		{-1, "-1"},
		{-42, "-42"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatInt(tt.input)
			if result != tt.expected {
				t.Errorf("formatInt(%d) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
