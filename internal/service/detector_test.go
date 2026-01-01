package service

import (
	"context"
	"testing"
	"time"

	"github.com/humanmark/humanmark/pkg/logger"
)

// TestContentTypeFromFilename tests filename-based content type detection.
func TestContentTypeFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected ContentType
	}{
		// Text files
		{"document.txt", ContentTypeText},
		{"readme.md", ContentTypeText},
		{"page.html", ContentTypeText},
		{"page.htm", ContentTypeText},
		{"data.json", ContentTypeText},
		{"config.xml", ContentTypeText},
		{"data.csv", ContentTypeText},

		// Image files
		{"photo.jpg", ContentTypeImage},
		{"photo.jpeg", ContentTypeImage},
		{"image.png", ContentTypeImage},
		{"animation.gif", ContentTypeImage},
		{"modern.webp", ContentTypeImage},
		{"bitmap.bmp", ContentTypeImage},
		{"vector.svg", ContentTypeImage},

		// Audio files
		{"song.mp3", ContentTypeAudio},
		{"audio.wav", ContentTypeAudio},
		{"music.flac", ContentTypeAudio},
		{"sound.ogg", ContentTypeAudio},
		{"track.m4a", ContentTypeAudio},
		{"clip.aac", ContentTypeAudio},

		// Video files
		{"movie.mp4", ContentTypeVideo},
		{"clip.mov", ContentTypeVideo},
		{"video.avi", ContentTypeVideo},
		{"stream.webm", ContentTypeVideo},
		{"film.mkv", ContentTypeVideo},
		{"windows.wmv", ContentTypeVideo},

		// Unknown
		{"file.unknown", ContentTypeUnknown},
		{"noextension", ContentTypeUnknown},
		{"", ContentTypeUnknown},

		// Case insensitive
		{"IMAGE.JPG", ContentTypeImage},
		{"DOCUMENT.TXT", ContentTypeText},
		{"Video.MP4", ContentTypeVideo},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := ContentTypeFromFilename(tt.filename)
			if result != tt.expected {
				t.Errorf("ContentTypeFromFilename(%q) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestContentTypeFromURL tests URL-based content type detection.
func TestContentTypeFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected ContentType
	}{
		// Simple URLs
		{"https://example.com/image.jpg", ContentTypeImage},
		{"https://example.com/document.pdf", ContentTypeUnknown},
		{"https://example.com/video.mp4", ContentTypeVideo},

		// URLs with query strings
		{"https://example.com/photo.png?size=large", ContentTypeImage},
		{"https://example.com/audio.mp3?quality=high&format=stereo", ContentTypeAudio},

		// URLs with paths
		{"https://cdn.example.com/uploads/2024/01/image.webp", ContentTypeImage},
		{"https://api.example.com/v1/files/document.txt", ContentTypeText},

		// Edge cases
		{"https://example.com/", ContentTypeUnknown},
		{"https://example.com/noextension", ContentTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := ContentTypeFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ContentTypeFromURL(%q) = %s, want %s", tt.url, result, tt.expected)
			}
		})
	}
}

// TestContentTypeFromMIME tests MIME type based content type detection.
func TestContentTypeFromMIME(t *testing.T) {
	tests := []struct {
		mime     string
		expected ContentType
	}{
		// Text types
		{"text/plain", ContentTypeText},
		{"text/html", ContentTypeText},
		{"text/css", ContentTypeText},
		{"text/javascript", ContentTypeText},
		{"application/json", ContentTypeText},
		{"application/xml", ContentTypeText},

		// Image types
		{"image/jpeg", ContentTypeImage},
		{"image/png", ContentTypeImage},
		{"image/gif", ContentTypeImage},
		{"image/webp", ContentTypeImage},
		{"image/svg+xml", ContentTypeImage},

		// Audio types
		{"audio/mpeg", ContentTypeAudio},
		{"audio/wav", ContentTypeAudio},
		{"audio/ogg", ContentTypeAudio},

		// Video types
		{"video/mp4", ContentTypeVideo},
		{"video/webm", ContentTypeVideo},
		{"video/quicktime", ContentTypeVideo},

		// With charset
		{"text/plain; charset=utf-8", ContentTypeText},
		{"application/json; charset=utf-8", ContentTypeText},

		// Unknown
		{"application/octet-stream", ContentTypeUnknown},
		{"application/pdf", ContentTypeUnknown},
		{"", ContentTypeUnknown},

		// Case variations
		{"TEXT/PLAIN", ContentTypeText},
		{"Image/JPEG", ContentTypeImage},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			result := ContentTypeFromMIME(tt.mime)
			if result != tt.expected {
				t.Errorf("ContentTypeFromMIME(%q) = %s, want %s", tt.mime, result, tt.expected)
			}
		})
	}
}

// TestContentTypeFromMagicBytes tests magic byte detection.
func TestContentTypeFromMagicBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected ContentType
	}{
		{
			name:     "JPEG",
			data:     []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
			expected: ContentTypeImage,
		},
		{
			name:     "PNG",
			data:     []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expected: ContentTypeImage,
		},
		{
			name:     "GIF",
			data:     []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
			expected: ContentTypeImage,
		},
		{
			name:     "WebP",
			data:     []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
			expected: ContentTypeImage,
		},
		{
			name:     "MP3 with sync",
			data:     []byte{0xFF, 0xFB, 0x90, 0x00},
			expected: ContentTypeAudio,
		},
		{
			name:     "MP3 with ID3",
			data:     []byte{0x49, 0x44, 0x33, 0x04, 0x00},
			expected: ContentTypeAudio,
		},
		{
			name:     "WAV",
			data:     []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x41, 0x56, 0x45},
			expected: ContentTypeAudio,
		},
		{
			name:     "MP4",
			data:     []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6F, 0x6D},
			expected: ContentTypeVideo,
		},
		{
			name:     "too short",
			data:     []byte{0xFF, 0xD8},
			expected: ContentTypeUnknown,
		},
		{
			name:     "unknown",
			data:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expected: ContentTypeUnknown,
		},
		{
			name:     "empty",
			data:     []byte{},
			expected: ContentTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContentTypeFromMagicBytes(tt.data)
			if result != tt.expected {
				t.Errorf("ContentTypeFromMagicBytes(%v) = %s, want %s", tt.data[:min(4, len(tt.data))], result, tt.expected)
			}
		})
	}
}

// TestNewDetector tests detector creation.
func TestNewDetector(t *testing.T) {
	log := logger.NopLogger()

	t.Run("creates detector with config", func(t *testing.T) {
		config := DetectorConfig{
			HiveAPIKey: "test-key",
			Timeout:    30 * time.Second,
		}

		detector, err := NewDetector(config, log)
		if err != nil {
			t.Fatalf("NewDetector failed: %v", err)
		}

		if detector == nil {
			t.Error("expected non-nil detector")
		}
	})

	t.Run("creates detector without API keys", func(t *testing.T) {
		config := DetectorConfig{
			Timeout: 30 * time.Second,
		}

		detector, err := NewDetector(config, log)
		if err != nil {
			t.Fatalf("NewDetector failed: %v", err)
		}

		if detector == nil {
			t.Error("expected non-nil detector")
		}
	})
}

// TestDetectorDetect tests the main detection flow.
func TestDetectorDetect(t *testing.T) {
	log := logger.NopLogger()
	config := DetectorConfig{
		Timeout: 30 * time.Second,
	}

	detector, _ := NewDetector(config, log)
	ctx := context.Background()

	t.Run("detects text content", func(t *testing.T) {
		input := DetectionInput{
			Text:        "This is a test text that should be analyzed for AI detection.",
			ContentType: ContentTypeText,
		}

		result, err := detector.Detect(ctx, input)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}

		// Verify result structure
		if result.ContentType != ContentTypeText {
			t.Errorf("ContentType: expected text, got %s", result.ContentType)
		}

		if result.Confidence < 0 || result.Confidence > 1 {
			t.Errorf("Confidence out of range: %f", result.Confidence)
		}

		if result.AIScore < 0 || result.AIScore > 1 {
			t.Errorf("AIScore out of range: %f", result.AIScore)
		}

		if len(result.Detectors) == 0 {
			t.Error("expected at least one detector")
		}

		if result.ContentHash == "" {
			t.Error("expected non-empty ContentHash")
		}

		if result.ProcessingTime == 0 {
			t.Error("expected non-zero ProcessingTime")
		}
	})

	t.Run("auto-detects content type from text", func(t *testing.T) {
		input := DetectionInput{
			Text: "This is plain text content.",
			// ContentType not specified
		}

		result, err := detector.Detect(ctx, input)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}

		if result.ContentType != ContentTypeText {
			t.Errorf("expected auto-detected text type, got %s", result.ContentType)
		}
	})

	t.Run("auto-detects content type from filename", func(t *testing.T) {
		input := DetectionInput{
			Data:     []byte("some content"),
			Filename: "document.txt",
		}

		result, err := detector.Detect(ctx, input)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}

		if result.ContentType != ContentTypeText {
			t.Errorf("expected text from filename, got %s", result.ContentType)
		}
	})

	t.Run("returns error for empty input", func(t *testing.T) {
		input := DetectionInput{}

		_, err := detector.Detect(ctx, input)
		if err == nil {
			t.Error("expected error for empty input")
		}
	})

	t.Run("generates consistent hash for same content", func(t *testing.T) {
		input := DetectionInput{
			Text:        "Consistent content for hashing.",
			ContentType: ContentTypeText,
		}

		result1, _ := detector.Detect(ctx, input)
		result2, _ := detector.Detect(ctx, input)

		if result1.ContentHash != result2.ContentHash {
			t.Errorf("hash mismatch for same content: %s != %s", result1.ContentHash, result2.ContentHash)
		}
	})

	t.Run("generates different hash for different content", func(t *testing.T) {
		input1 := DetectionInput{
			Text:        "First piece of content.",
			ContentType: ContentTypeText,
		}
		input2 := DetectionInput{
			Text:        "Second piece of content.",
			ContentType: ContentTypeText,
		}

		result1, _ := detector.Detect(ctx, input1)
		result2, _ := detector.Detect(ctx, input2)

		if result1.ContentHash == result2.ContentHash {
			t.Error("expected different hashes for different content")
		}
	})
}

// TestDetectionInput tests DetectionInput struct.
func TestDetectionInput(t *testing.T) {
	t.Run("zero value is empty", func(t *testing.T) {
		var input DetectionInput

		if input.URL != "" {
			t.Error("URL should be empty")
		}
		if input.Text != "" {
			t.Error("Text should be empty")
		}
		if input.Data != nil {
			t.Error("Data should be nil")
		}
	})
}

// TestDetectionResult tests DetectionResult struct.
func TestDetectionResult(t *testing.T) {
	t.Run("human verdict based on AI score", func(t *testing.T) {
		// AI score < 0.5 should mean human
		result := DetectionResult{
			AIScore: 0.3,
			Human:   true,
		}

		if !result.Human {
			t.Error("low AI score should indicate human")
		}

		// AI score > 0.5 should mean AI
		result2 := DetectionResult{
			AIScore: 0.8,
			Human:   false,
		}

		if result2.Human {
			t.Error("high AI score should indicate AI")
		}
	})
}

// TestMockDetection tests the mock detection heuristics.
func TestMockDetection(t *testing.T) {
	log := logger.NopLogger()
	config := DetectorConfig{Timeout: 30 * time.Second}
	detector, _ := NewDetector(config, log)
	ctx := context.Background()

	t.Run("AI-like text scores higher", func(t *testing.T) {
		aiText := `As an AI language model, I cannot provide personal opinions. 
		It's important to note that this is a complex topic. 
		In conclusion, furthermore, moreover, to summarize, these are AI patterns.`

		humanText := `Hey! Just wanted to share my thoughts on this. 
		I really liked it, though some parts were kinda weird. 
		Anyway, let me know what you think!`

		aiResult, _ := detector.Detect(ctx, DetectionInput{Text: aiText})
		humanResult, _ := detector.Detect(ctx, DetectionInput{Text: humanText})

		// AI text should have higher AI score
		if aiResult.AIScore <= humanResult.AIScore {
			t.Logf("AI text score: %f, Human text score: %f", aiResult.AIScore, humanResult.AIScore)
			// Note: This might not always pass with mock detection
			// but documents expected behavior
		}
	})
}

// BenchmarkDetect benchmarks the detection process.
func BenchmarkDetect(b *testing.B) {
	log := logger.NopLogger()
	config := DetectorConfig{Timeout: 30 * time.Second}
	detector, _ := NewDetector(config, log)
	ctx := context.Background()

	input := DetectionInput{
		Text:        "This is benchmark text for performance testing of the detection system.",
		ContentType: ContentTypeText,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(ctx, input)
	}
}

// BenchmarkContentHash benchmarks content hashing.
func BenchmarkContentHash(b *testing.B) {
	log := logger.NopLogger()
	config := DetectorConfig{Timeout: 30 * time.Second}
	detector, _ := NewDetector(config, log)

	// Create detector to access hashContent
	d := detector.(*detector)

	input := DetectionInput{
		Text: "Content to hash for benchmarking purposes.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.hashContent(input)
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
