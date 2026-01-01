package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/humanmark/humanmark/internal/repository"
	"github.com/humanmark/humanmark/internal/service"
	"github.com/humanmark/humanmark/pkg/logger"
)

// mockDetector implements service.Detector for testing.
type mockDetector struct {
	result *service.DetectionResult
	err    error
}

func (m *mockDetector) Detect(ctx context.Context, input service.DetectionInput) (*service.DetectionResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &service.DetectionResult{
		Human:       true,
		Confidence:  0.95,
		AIScore:     0.05,
		ContentType: input.ContentType,
		Detectors:   []string{"mock"},
	}, nil
}

// mockRepository implements repository.Repository for testing.
type mockRepository struct {
	jobs map[string]*repository.Job
}

func newMockRepository() *mockRepository {
	return &mockRepository{jobs: make(map[string]*repository.Job)}
}

func (m *mockRepository) CreateJob(ctx context.Context, job repository.Job) (*repository.Job, error) {
	job.ID = "test-job-id"
	job.CreatedAt = time.Now()
	m.jobs[job.ID] = &job
	return &job, nil
}

func (m *mockRepository) GetJob(ctx context.Context, id string) (*repository.Job, error) {
	if job, ok := m.jobs[id]; ok {
		return job, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockRepository) Ping(ctx context.Context) error {
	return nil
}

func (m *mockRepository) Close() error {
	return nil
}

// newTestHandler creates a Handler with mock dependencies for testing.
func newTestHandler() *Handler {
	return New(Config{
		Detector:      &mockDetector{},
		Repository:    newMockRepository(),
		Logger:        logger.NopLogger(),
		MaxUploadSize: 10 * 1024 * 1024, // 10MB
	})
}

// TestVerify_JSONText tests text verification via JSON.
func TestVerify_JSONText(t *testing.T) {
	h := newTestHandler()

	body := `{"text": "This is a test text that should be verified as human-written content."}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response VerifyResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID == "" {
		t.Error("expected non-empty ID")
	}
	if response.ContentType != "text" {
		t.Errorf("expected content_type 'text', got %s", response.ContentType)
	}
	if !response.Human {
		t.Error("expected human=true")
	}
	if response.Confidence < 0 || response.Confidence > 1 {
		t.Errorf("confidence out of range: %f", response.Confidence)
	}
}

// TestVerify_JSONUrl tests URL verification via JSON.
func TestVerify_JSONUrl(t *testing.T) {
	h := newTestHandler()

	body := `{"url": "https://example.com/image.jpg"}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response VerifyResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ContentType != "image" {
		t.Errorf("expected content_type 'image', got %s", response.ContentType)
	}
}

// TestVerify_FileUpload tests file upload verification.
func TestVerify_FileUpload(t *testing.T) {
	h := newTestHandler()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte("This is test content for file upload verification."))
	writer.Close()

	req := httptest.NewRequest("POST", "/verify", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestVerify_EmptyBody tests error handling for empty request.
func TestVerify_EmptyBody(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest("POST", "/verify", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == "" {
		t.Error("expected error message")
	}
}

// TestVerify_InvalidJSON tests error handling for invalid JSON.
func TestVerify_InvalidJSON(t *testing.T) {
	h := newTestHandler()

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// TestVerify_NoContent tests error handling when no content provided.
func TestVerify_NoContent(t *testing.T) {
	h := newTestHandler()

	body := `{}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// TestVerify_TextTooShort tests validation for short text.
func TestVerify_TextTooShort(t *testing.T) {
	h := newTestHandler()

	body := `{"text": "short"}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// TestVerify_InvalidURL tests validation for invalid URL.
func TestVerify_InvalidURL(t *testing.T) {
	h := newTestHandler()

	body := `{"url": "not-a-valid-url"}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// TestVerify_Detailed tests detailed response parameter.
func TestVerify_Detailed(t *testing.T) {
	h := newTestHandler()

	body := `{"text": "This is test content for detailed response testing."}`
	req := httptest.NewRequest("POST", "/verify?detailed=true", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response VerifyResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Details == nil {
		t.Error("expected details in response")
	}
	if response.Details != nil && len(response.Details.Detectors) == 0 {
		t.Error("expected detectors in details")
	}
}

// TestVerify_DetectorError tests handling of detector errors.
func TestVerify_DetectorError(t *testing.T) {
	h := New(Config{
		Detector: &mockDetector{
			err: io.ErrUnexpectedEOF, // Simulate detector error
		},
		Repository:    newMockRepository(),
		Logger:        logger.NopLogger(),
		MaxUploadSize: 10 * 1024 * 1024,
	})

	body := `{"text": "This is test content that will cause a detector error."}`
	req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// TestGetResult tests retrieving verification results.
func TestGetResult(t *testing.T) {
	repo := newMockRepository()
	h := New(Config{
		Detector:      &mockDetector{},
		Repository:    repo,
		Logger:        logger.NopLogger(),
		MaxUploadSize: 10 * 1024 * 1024,
	})

	// First, create a job
	repo.jobs["existing-id"] = &repository.Job{
		ID:          "existing-id",
		Human:       true,
		Confidence:  0.9,
		ContentType: "text",
		CreatedAt:   time.Now(),
	}

	t.Run("returns existing job", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/verify/existing-id", nil)
		req.SetPathValue("id", "existing-id")
		rec := httptest.NewRecorder()

		h.GetResult(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response VerifyResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.ID != "existing-id" {
			t.Errorf("expected ID 'existing-id', got %s", response.ID)
		}
	})

	t.Run("returns 404 for non-existent job", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/verify/non-existent", nil)
		req.SetPathValue("id", "non-existent")
		rec := httptest.NewRecorder()

		h.GetResult(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("returns 400 for missing ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/verify/", nil)
		req.SetPathValue("id", "")
		rec := httptest.NewRecorder()

		h.GetResult(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})
}

// TestHealth tests the health check endpoint.
func TestHealth(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", response["status"])
	}
}

// TestIndex tests the index endpoint.
func TestIndex(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["name"] != "HumanMark API" {
		t.Errorf("expected name 'HumanMark API', got %v", response["name"])
	}
}

// TestValidateInput tests input validation.
func TestValidateInput(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name    string
		input   service.DetectionInput
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   service.DetectionInput{},
			wantErr: true,
		},
		{
			name:    "valid text",
			input:   service.DetectionInput{Text: "This is valid test content for validation."},
			wantErr: false,
		},
		{
			name:    "text too short",
			input:   service.DetectionInput{Text: "short"},
			wantErr: true,
		},
		{
			name:    "valid URL",
			input:   service.DetectionInput{URL: "https://example.com/image.jpg"},
			wantErr: false,
		},
		{
			name:    "invalid URL",
			input:   service.DetectionInput{URL: "not-a-url"},
			wantErr: true,
		},
		{
			name:    "valid data",
			input:   service.DetectionInput{Data: []byte("file content")},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkVerify benchmarks the verify endpoint.
func BenchmarkVerify(b *testing.B) {
	h := newTestHandler()

	body := `{"text": "This is benchmark test content for performance testing of the verify endpoint."}`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/verify", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.Verify(rec, req)
	}
}
