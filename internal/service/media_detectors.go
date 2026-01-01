package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/humanmark/humanmark/pkg/logger"
)

// =============================================================================
// Image Detector
// =============================================================================

// imageDetector implements ImageDetector.
type imageDetector struct {
	config     DetectorConfig
	logger     *logger.Logger
	httpClient *http.Client
}

// NewImageDetector creates a new image detector.
func NewImageDetector(config DetectorConfig, log *logger.Logger) ImageDetector {
	return &imageDetector{
		config: config,
		logger: log,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// DetectImage analyzes image content for AI generation.
func (d *imageDetector) DetectImage(ctx context.Context, input DetectionInput) (*DetectionResult, error) {
	var imageData []byte
	var err error

	// Get image data
	if len(input.Data) > 0 {
		imageData = input.Data
	} else if input.URL != "" {
		imageData, err = d.fetchImageFromURL(ctx, input.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch image: %w", err)
		}
	} else {
		return nil, errors.New("no image data provided")
	}

	var scores []float64
	var detectors []string

	// ==========================================================================
	// PRIMARY: Our own HumanMark forensic analyzer
	// This runs locally with no external dependencies
	// ==========================================================================
	analyzer := NewImageAnalyzer()
	analysis := analyzer.Analyze(imageData)
	
	scores = append(scores, analysis.AIScore)
	detectors = append(detectors, "humanmark")
	
	d.logger.Debug("humanmark image analysis complete",
		"ai_score", analysis.AIScore,
		"has_exif", analysis.Metadata.HasEXIF,
		"camera_make", analysis.Metadata.CameraMake,
		"format", analysis.Metadata.FileFormat,
		"width", analysis.Stats.Width,
		"height", analysis.Stats.Height,
	)

	// ==========================================================================
	// SECONDARY: External APIs (optional, for higher accuracy)
	// ==========================================================================

	// Try Hive API for image detection
	if d.config.HiveAPIKey != "" {
		score, err := d.detectWithHive(ctx, imageData)
		if err != nil {
			d.logger.Warn("hive image detection failed", "error", err)
		} else {
			scores = append(scores, score)
			detectors = append(detectors, "hive")
		}
	}

	// Aggregate scores with weighted average
	aiScore := d.aggregateScoresWeighted(scores, detectors)
	human := aiScore < 0.5
	confidence := abs(aiScore-0.5) * 2

	return &DetectionResult{
		Human:       human,
		Confidence:  confidence,
		AIScore:     aiScore,
		ContentType: ContentTypeImage,
		Detectors:   detectors,
	}, nil
}

// aggregateScoresWeighted combines scores with detector-specific weights.
func (d *imageDetector) aggregateScoresWeighted(scores []float64, detectors []string) float64 {
	if len(scores) == 0 {
		return 0.5
	}

	weights := map[string]float64{
		"humanmark": 1.0, // Our forensic analysis
		"hive":      1.3, // External API - ML-based
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for i, score := range scores {
		w := 1.0
		if i < len(detectors) {
			if detectorWeight, ok := weights[detectors[i]]; ok {
				w = detectorWeight
			}
		}
		weightedSum += score * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0.5
	}

	return weightedSum / totalWeight
}

// detectWithHive calls Hive AI for image detection.
func (d *imageDetector) detectWithHive(ctx context.Context, imageData []byte) (float64, error) {
	// Encode image as base64
	encoded := base64.StdEncoding.EncodeToString(imageData)

	body, _ := json.Marshal(map[string]string{"image": encoded})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.thehive.ai/api/v2/task/sync", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Token "+d.config.HiveAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("hive API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Status []struct {
			Response struct {
				AIGenerated float64 `json:"ai_generated"`
			} `json:"response"`
		} `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Status) > 0 {
		return result.Status[0].Response.AIGenerated, nil
	}

	return 0, errors.New("no result from Hive API")
}

// fetchImageFromURL downloads an image from a URL.
func (d *imageDetector) fetchImageFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	// Limit size to 50MB
	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (d *imageDetector) aggregateScores(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}

// =============================================================================
// Audio Detector
// =============================================================================

// audioDetector implements AudioDetector.
type audioDetector struct {
	config     DetectorConfig
	logger     *logger.Logger
	httpClient *http.Client
}

// NewAudioDetector creates a new audio detector.
func NewAudioDetector(config DetectorConfig, log *logger.Logger) AudioDetector {
	return &audioDetector{
		config: config,
		logger: log,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// DetectAudio analyzes audio content for AI generation.
func (d *audioDetector) DetectAudio(ctx context.Context, input DetectionInput) (*DetectionResult, error) {
	var audioData []byte
	var err error

	// Get audio data
	if len(input.Data) > 0 {
		audioData = input.Data
	} else if input.URL != "" {
		audioData, err = d.fetchAudioFromURL(ctx, input.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch audio: %w", err)
		}
	} else {
		return nil, errors.New("no audio data provided")
	}

	var scores []float64
	var detectors []string

	// ==========================================================================
	// PRIMARY: Our own HumanMark forensic analyzer
	// This runs locally with no external dependencies
	// ==========================================================================
	analyzer := NewAudioAnalyzer()
	analysis := analyzer.Analyze(audioData)
	
	scores = append(scores, analysis.AIScore)
	detectors = append(detectors, "humanmark")
	
	d.logger.Debug("humanmark audio analysis complete",
		"ai_score", analysis.AIScore,
		"format", analysis.Metadata.Format,
		"sample_rate", analysis.Metadata.SampleRate,
		"channels", analysis.Metadata.Channels,
		"encoder", analysis.Metadata.EncoderName,
		"is_ai_marked", analysis.Metadata.IsAIMarked,
	)

	// ==========================================================================
	// SECONDARY: External APIs (optional, for higher accuracy)
	// ==========================================================================

	// Try Hive API for audio detection
	if d.config.HiveAPIKey != "" {
		score, err := d.detectWithHive(ctx, audioData)
		if err != nil {
			d.logger.Warn("hive audio detection failed", "error", err)
		} else {
			scores = append(scores, score)
			detectors = append(detectors, "hive")
		}
	}

	// Aggregate scores with weighted average
	aiScore := d.aggregateScoresWeighted(scores, detectors)
	human := aiScore < 0.5
	confidence := abs(aiScore-0.5) * 2

	return &DetectionResult{
		Human:       human,
		Confidence:  confidence,
		AIScore:     aiScore,
		ContentType: ContentTypeAudio,
		Detectors:   detectors,
	}, nil
}

// aggregateScoresWeighted combines scores with detector-specific weights.
func (d *audioDetector) aggregateScoresWeighted(scores []float64, detectors []string) float64 {
	if len(scores) == 0 {
		return 0.5
	}

	weights := map[string]float64{
		"humanmark": 1.0, // Our forensic analysis
		"hive":      1.3, // External API - ML-based
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for i, score := range scores {
		w := 1.0
		if i < len(detectors) {
			if detectorWeight, ok := weights[detectors[i]]; ok {
				w = detectorWeight
			}
		}
		weightedSum += score * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0.5
	}

	return weightedSum / totalWeight
}

// detectWithHive calls Hive AI for audio detection.
func (d *audioDetector) detectWithHive(ctx context.Context, audioData []byte) (float64, error) {
	encoded := base64.StdEncoding.EncodeToString(audioData)
	body, _ := json.Marshal(map[string]string{"audio": encoded})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.thehive.ai/api/v2/task/sync", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Token "+d.config.HiveAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("hive API returned status %d", resp.StatusCode)
	}

	var result struct {
		Status []struct {
			Response struct {
				AIGenerated float64 `json:"ai_generated"`
			} `json:"response"`
		} `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Status) > 0 {
		return result.Status[0].Response.AIGenerated, nil
	}

	return 0, errors.New("no result from Hive API")
}

// fetchAudioFromURL downloads audio from a URL.
func (d *audioDetector) fetchAudioFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch audio: status %d", resp.StatusCode)
	}

	// Limit to 100MB
	return io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024))
}

func (d *audioDetector) aggregateScores(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}

// =============================================================================
// Video Detector
// =============================================================================

// videoDetector implements VideoDetector.
type videoDetector struct {
	config     DetectorConfig
	logger     *logger.Logger
	httpClient *http.Client
}

// NewVideoDetector creates a new video detector.
func NewVideoDetector(config DetectorConfig, log *logger.Logger) VideoDetector {
	return &videoDetector{
		config: config,
		logger: log,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// DetectVideo analyzes video content for AI generation.
func (d *videoDetector) DetectVideo(ctx context.Context, input DetectionInput) (*DetectionResult, error) {
	var videoData []byte
	var err error

	// Get video data
	if len(input.Data) > 0 {
		videoData = input.Data
	} else if input.URL != "" {
		// For videos, we need to fetch the data
		videoData, err = d.fetchVideoFromURL(ctx, input.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch video: %w", err)
		}
	} else {
		return nil, errors.New("video data or URL required for detection")
	}

	var scores []float64
	var detectors []string

	// ==========================================================================
	// PRIMARY: Our own HumanMark forensic analyzer
	// This runs locally with no external dependencies
	// ==========================================================================
	analyzer := NewVideoAnalyzer()
	analysis := analyzer.Analyze(videoData)
	
	scores = append(scores, analysis.AIScore)
	detectors = append(detectors, "humanmark")
	
	d.logger.Debug("humanmark video analysis complete",
		"ai_score", analysis.AIScore,
		"format", analysis.Metadata.Format,
		"has_audio", analysis.Metadata.HasAudio,
		"encoder", analysis.Metadata.EncoderName,
		"is_ai_marked", analysis.Metadata.IsAIMarked,
		"file_size", analysis.Stats.FileSize,
	)

	// ==========================================================================
	// SECONDARY: External APIs (optional, for higher accuracy)
	// ==========================================================================

	// Try Hive API for video detection
	if d.config.HiveAPIKey != "" && input.URL != "" {
		score, err := d.detectWithHive(ctx, input.URL)
		if err != nil {
			d.logger.Warn("hive video detection failed", "error", err)
		} else {
			scores = append(scores, score)
			detectors = append(detectors, "hive")
		}
	}

	// Aggregate scores with weighted average
	aiScore := d.aggregateScoresWeighted(scores, detectors)
	human := aiScore < 0.5
	confidence := abs(aiScore-0.5) * 2

	return &DetectionResult{
		Human:       human,
		Confidence:  confidence,
		AIScore:     aiScore,
		ContentType: ContentTypeVideo,
		Detectors:   detectors,
	}, nil
}

// fetchVideoFromURL downloads video from a URL.
func (d *videoDetector) fetchVideoFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch video: status %d", resp.StatusCode)
	}

	// Limit to 500MB for video
	return io.ReadAll(io.LimitReader(resp.Body, 500*1024*1024))
}

// aggregateScoresWeighted combines scores with detector-specific weights.
func (d *videoDetector) aggregateScoresWeighted(scores []float64, detectors []string) float64 {
	if len(scores) == 0 {
		return 0.5
	}

	weights := map[string]float64{
		"humanmark": 1.0, // Our forensic analysis
		"hive":      1.4, // External API - ML-based, better for video
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for i, score := range scores {
		w := 1.0
		if i < len(detectors) {
			if detectorWeight, ok := weights[detectors[i]]; ok {
				w = detectorWeight
			}
		}
		weightedSum += score * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0.5
	}

	return weightedSum / totalWeight
}

// detectWithHive calls Hive AI for video detection.
func (d *videoDetector) detectWithHive(ctx context.Context, videoURL string) (float64, error) {
	body, _ := json.Marshal(map[string]string{"url": videoURL})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.thehive.ai/api/v2/task/sync", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Token "+d.config.HiveAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("hive API returned status %d", resp.StatusCode)
	}

	var result struct {
		Status []struct {
			Response struct {
				AIGenerated float64 `json:"ai_generated"`
			} `json:"response"`
		} `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Status) > 0 {
		return result.Status[0].Response.AIGenerated, nil
	}

	return 0, errors.New("no result from Hive API")
}

func (d *videoDetector) aggregateScores(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}
