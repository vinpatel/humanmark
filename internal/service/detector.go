// Package service implements the core detection logic for HumanMark.
//
// The Detector interface abstracts the detection process, allowing different
// implementations for production (external APIs) and testing (mocks).
//
// Detection flow:
//  1. Determine content type (text, image, audio, video)
//  2. Route to appropriate detector(s)
//  3. Aggregate results from multiple detectors
//  4. Return final verdict: Human or Non-Human
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/humanmark/humanmark/pkg/logger"
)

// ContentType represents the type of content being analyzed.
type ContentType string

const (
	ContentTypeText    ContentType = "text"
	ContentTypeImage   ContentType = "image"
	ContentTypeAudio   ContentType = "audio"
	ContentTypeVideo   ContentType = "video"
	ContentTypeUnknown ContentType = "unknown"
)

// DetectionInput represents input to the detection system.
type DetectionInput struct {
	// URL of content to fetch and analyze
	URL string

	// Text content to analyze directly
	Text string

	// Binary data for uploaded files
	Data []byte

	// Original filename for uploaded files
	Filename string

	// Detected or specified content type
	ContentType ContentType
}

// DetectionResult represents the output of detection.
type DetectionResult struct {
	// Human is true if content was created by a human
	Human bool

	// Confidence is how confident we are (0.0-1.0)
	// Higher = more confident in the verdict
	Confidence float64

	// AIScore is the raw AI probability (0.0-1.0)
	// 0.0 = definitely human, 1.0 = definitely AI
	AIScore float64

	// ContentType is what type of content was analyzed
	ContentType ContentType

	// Detectors lists which detection methods were used
	Detectors []string

	// ContentHash is SHA256 hash of the analyzed content
	ContentHash string

	// ProcessingTime is how long detection took
	ProcessingTime time.Duration
}

// Detector is the interface for content detection.
// Different implementations can use different detection backends.
type Detector interface {
	Detect(ctx context.Context, input DetectionInput) (*DetectionResult, error)
}

// DetectorConfig holds configuration for the detector.
type DetectorConfig struct {
	HiveAPIKey    string
	OpenAIAPIKey  string
	GPTZeroAPIKey string
	Timeout       time.Duration
}

// detector is the main implementation of Detector.
type detector struct {
	config DetectorConfig
	logger *logger.Logger

	// Backend detectors
	textDetector  TextDetector
	imageDetector ImageDetector
	audioDetector AudioDetector
	videoDetector VideoDetector
}

// NewDetector creates a new Detector with the given configuration.
func NewDetector(config DetectorConfig, log *logger.Logger) (Detector, error) {
	d := &detector{
		config: config,
		logger: log,
	}

	// Initialize backend detectors based on available API keys
	d.textDetector = NewTextDetector(config, log)
	d.imageDetector = NewImageDetector(config, log)
	d.audioDetector = NewAudioDetector(config, log)
	d.videoDetector = NewVideoDetector(config, log)

	return d, nil
}

// Detect analyzes content and returns whether it was human-created.
func (d *detector) Detect(ctx context.Context, input DetectionInput) (*DetectionResult, error) {
	start := time.Now()

	// Determine content type if not specified
	if input.ContentType == ContentTypeUnknown || input.ContentType == "" {
		input.ContentType = d.detectContentType(input)
	}

	d.logger.Debug("starting detection",
		"content_type", input.ContentType,
		"has_url", input.URL != "",
		"text_length", len(input.Text),
		"data_length", len(input.Data),
	)

	// Route to appropriate detector
	var result *DetectionResult
	var err error

	switch input.ContentType {
	case ContentTypeText:
		result, err = d.textDetector.DetectText(ctx, input)
	case ContentTypeImage:
		result, err = d.imageDetector.DetectImage(ctx, input)
	case ContentTypeAudio:
		result, err = d.audioDetector.DetectAudio(ctx, input)
	case ContentTypeVideo:
		result, err = d.videoDetector.DetectVideo(ctx, input)
	default:
		return nil, errors.New("unsupported content type: " + string(input.ContentType))
	}

	if err != nil {
		return nil, fmt.Errorf("detection failed: %w", err)
	}

	// Calculate content hash
	result.ContentHash = d.hashContent(input)
	result.ProcessingTime = time.Since(start)

	d.logger.Debug("detection complete",
		"human", result.Human,
		"confidence", result.Confidence,
		"ai_score", result.AIScore,
		"processing_time_ms", result.ProcessingTime.Milliseconds(),
	)

	return result, nil
}

// detectContentType determines content type from input.
func (d *detector) detectContentType(input DetectionInput) ContentType {
	// If we have text, it's text
	if input.Text != "" {
		return ContentTypeText
	}

	// Check filename extension
	if input.Filename != "" {
		return ContentTypeFromFilename(input.Filename)
	}

	// Check URL extension
	if input.URL != "" {
		return ContentTypeFromURL(input.URL)
	}

	// Check magic bytes if we have data
	if len(input.Data) > 0 {
		return ContentTypeFromMagicBytes(input.Data)
	}

	return ContentTypeUnknown
}

// hashContent creates a SHA256 hash of the content.
func (d *detector) hashContent(input DetectionInput) string {
	h := sha256.New()

	if input.Text != "" {
		h.Write([]byte(input.Text))
	} else if len(input.Data) > 0 {
		h.Write(input.Data)
	} else if input.URL != "" {
		h.Write([]byte(input.URL))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ContentTypeFromFilename determines content type from filename extension.
func ContentTypeFromFilename(filename string) ContentType {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".txt", ".md", ".html", ".htm", ".json", ".xml", ".csv":
		return ContentTypeText
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return ContentTypeImage
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac":
		return ContentTypeAudio
	case ".mp4", ".mov", ".avi", ".webm", ".mkv", ".wmv":
		return ContentTypeVideo
	default:
		return ContentTypeUnknown
	}
}

// ContentTypeFromURL determines content type from URL.
func ContentTypeFromURL(url string) ContentType {
	// Extract path from URL
	path := url
	if idx := strings.Index(url, "?"); idx != -1 {
		path = url[:idx]
	}

	return ContentTypeFromFilename(path)
}

// ContentTypeFromMIME determines content type from MIME type.
func ContentTypeFromMIME(mimeType string) ContentType {
	// Normalize MIME type
	mime := strings.ToLower(mimeType)
	if idx := strings.Index(mime, ";"); idx != -1 {
		mime = mime[:idx]
	}
	mime = strings.TrimSpace(mime)

	switch {
	case strings.HasPrefix(mime, "text/"):
		return ContentTypeText
	case strings.HasPrefix(mime, "image/"):
		return ContentTypeImage
	case strings.HasPrefix(mime, "audio/"):
		return ContentTypeAudio
	case strings.HasPrefix(mime, "video/"):
		return ContentTypeVideo
	case mime == "application/json", mime == "application/xml":
		return ContentTypeText
	default:
		return ContentTypeUnknown
	}
}

// ContentTypeFromMagicBytes determines content type from file magic bytes.
func ContentTypeFromMagicBytes(data []byte) ContentType {
	if len(data) < 4 {
		return ContentTypeUnknown
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return ContentTypeImage
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return ContentTypeImage
	}

	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return ContentTypeImage
	}

	// WebP: 52 49 46 46 ... 57 45 42 50
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return ContentTypeImage
		}
	}

	// MP3: FF FB, FF FA, FF F3, FF F2 or ID3
	if (data[0] == 0xFF && (data[1]&0xE0) == 0xE0) || (data[0] == 0x49 && data[1] == 0x44 && data[2] == 0x33) {
		return ContentTypeAudio
	}

	// MP4/MOV: ftyp
	if len(data) >= 8 && data[4] == 0x66 && data[5] == 0x74 && data[6] == 0x79 && data[7] == 0x70 {
		return ContentTypeVideo
	}

	// WAV: 52 49 46 46 ... 57 41 56 45
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if data[8] == 0x57 && data[9] == 0x41 && data[10] == 0x56 && data[11] == 0x45 {
			return ContentTypeAudio
		}
	}

	return ContentTypeUnknown
}

// TextDetector handles text content detection.
type TextDetector interface {
	DetectText(ctx context.Context, input DetectionInput) (*DetectionResult, error)
}

// ImageDetector handles image content detection.
type ImageDetector interface {
	DetectImage(ctx context.Context, input DetectionInput) (*DetectionResult, error)
}

// AudioDetector handles audio content detection.
type AudioDetector interface {
	DetectAudio(ctx context.Context, input DetectionInput) (*DetectionResult, error)
}

// VideoDetector handles video content detection.
type VideoDetector interface {
	DetectVideo(ctx context.Context, input DetectionInput) (*DetectionResult, error)
}
