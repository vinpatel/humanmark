package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/humanmark/humanmark/pkg/logger"
)

// textDetector implements TextDetector using multiple backends.
type textDetector struct {
	config     DetectorConfig
	logger     *logger.Logger
	httpClient *http.Client
}

// NewTextDetector creates a new text detector.
func NewTextDetector(config DetectorConfig, log *logger.Logger) TextDetector {
	return &textDetector{
		config: config,
		logger: log,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// DetectText analyzes text content for AI generation.
func (d *textDetector) DetectText(ctx context.Context, input DetectionInput) (*DetectionResult, error) {
	text := input.Text
	if text == "" && len(input.Data) > 0 {
		text = string(input.Data)
	}

	if text == "" && input.URL != "" {
		// Fetch text from URL
		var err error
		text, err = d.fetchTextFromURL(ctx, input.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch text from URL: %w", err)
		}
	}

	if text == "" {
		return nil, errors.New("no text content to analyze")
	}

	// Collect results from available detectors
	var scores []float64
	var detectors []string

	// ==========================================================================
	// PRIMARY: Our own HumanMark statistical analyzer
	// This runs locally with no external dependencies
	// ==========================================================================
	analyzer := NewTextAnalyzer()
	analysis := analyzer.Analyze(text)
	
	scores = append(scores, analysis.AIScore)
	detectors = append(detectors, "humanmark")
	
	d.logger.Debug("humanmark analysis complete",
		"ai_score", analysis.AIScore,
		"sentence_variance", analysis.Signals.SentenceVariance,
		"vocabulary_richness", analysis.Signals.VocabularyRichness,
		"ai_phrases_detected", len(analysis.DetectedAIPhrases),
		"word_count", analysis.Stats.WordCount,
	)

	// ==========================================================================
	// SECONDARY: External APIs (optional, for higher accuracy)
	// These are weighted together with our algorithm
	// ==========================================================================

	// Try Hive API
	if d.config.HiveAPIKey != "" {
		score, err := d.detectWithHive(ctx, text)
		if err != nil {
			d.logger.Warn("hive detection failed", "error", err)
		} else {
			scores = append(scores, score)
			detectors = append(detectors, "hive")
		}
	}

	// Try GPTZero API
	if d.config.GPTZeroAPIKey != "" {
		score, err := d.detectWithGPTZero(ctx, text)
		if err != nil {
			d.logger.Warn("gptzero detection failed", "error", err)
		} else {
			scores = append(scores, score)
			detectors = append(detectors, "gptzero")
		}
	}

	// Try OpenAI-based detection
	if d.config.OpenAIAPIKey != "" {
		score, err := d.detectWithOpenAI(ctx, text)
		if err != nil {
			d.logger.Warn("openai detection failed", "error", err)
		} else {
			scores = append(scores, score)
			detectors = append(detectors, "openai")
		}
	}

	// Aggregate scores with weighted average
	// Our algorithm has slightly higher weight since it's always available
	aiScore := d.aggregateScoresWeighted(scores, detectors)

	// Determine verdict
	// AI score > 0.5 means likely AI-generated
	human := aiScore < 0.5
	confidence := abs(aiScore-0.5) * 2 // Convert to 0-1 confidence scale

	return &DetectionResult{
		Human:       human,
		Confidence:  confidence,
		AIScore:     aiScore,
		ContentType: ContentTypeText,
		Detectors:   detectors,
	}, nil
}

// aggregateScoresWeighted combines scores with detector-specific weights.
func (d *textDetector) aggregateScoresWeighted(scores []float64, detectors []string) float64 {
	if len(scores) == 0 {
		return 0.5
	}

	// Detector weights (based on reliability)
	weights := map[string]float64{
		"humanmark": 1.0, // Our algorithm - always runs
		"hive":      1.2, // External API - trained on large dataset
		"gptzero":   1.1, // External API - specialized for GPT
		"openai":    0.9, // External API - using LLM to detect
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

// detectWithHive calls the Hive AI API for text detection.
func (d *textDetector) detectWithHive(ctx context.Context, text string) (float64, error) {
	body, _ := json.Marshal(map[string]string{"text_data": text})

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

// detectWithGPTZero calls the GPTZero API for text detection.
func (d *textDetector) detectWithGPTZero(ctx context.Context, text string) (float64, error) {
	body, _ := json.Marshal(map[string]string{"document": text})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.gptzero.me/v2/predict/text", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("x-api-key", d.config.GPTZeroAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("gptzero API returned status %d", resp.StatusCode)
	}

	var result struct {
		Documents []struct {
			CompletelyGeneratedProb float64 `json:"completely_generated_prob"`
		} `json:"documents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Documents) > 0 {
		return result.Documents[0].CompletelyGeneratedProb, nil
	}

	return 0, errors.New("no result from GPTZero API")
}

// detectWithOpenAI uses OpenAI to analyze text for AI characteristics.
func (d *textDetector) detectWithOpenAI(ctx context.Context, text string) (float64, error) {
	// Truncate text if too long
	if len(text) > 4000 {
		text = text[:4000]
	}

	requestBody := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{
				"role": "system",
				"content": `You are an AI content detector. Analyze the text and determine if it was written by an AI or a human.
				
Respond with ONLY a JSON object:
{"ai_probability": 0.0-1.0, "reasoning": "brief explanation"}

ai_probability should be:
- 0.0-0.3: Clearly human-written
- 0.3-0.5: Probably human-written
- 0.5-0.7: Uncertain
- 0.7-0.9: Probably AI-generated
- 0.9-1.0: Clearly AI-generated`,
			},
			{
				"role":    "user",
				"content": text,
			},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.1,
	}

	body, _ := json.Marshal(requestBody)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+d.config.OpenAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("openai API returned status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Choices) == 0 {
		return 0, errors.New("no response from OpenAI")
	}

	// Parse the JSON response
	var detection struct {
		AIProbability float64 `json:"ai_probability"`
		Reasoning     string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &detection); err != nil {
		return 0, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	return detection.AIProbability, nil
}

// mockDetection provides a simple heuristic-based detection for development.
// This is NOT suitable for production use.
func (d *textDetector) mockDetection(text string) float64 {
	// Simple heuristics for development/testing
	// In production, always use real detection APIs

	score := 0.5 // Start neutral

	// Check for common AI patterns (very simplistic)
	aiPhrases := []string{
		"as an ai",
		"i cannot",
		"i don't have personal",
		"it's important to note",
		"in conclusion",
		"furthermore",
		"moreover",
		"in summary",
		"to summarize",
	}

	lowerText := string(bytes.ToLower([]byte(text)))
	for _, phrase := range aiPhrases {
		if bytes.Contains([]byte(lowerText), []byte(phrase)) {
			score += 0.1
		}
	}

	// Check text variability
	words := bytes.Fields([]byte(text))
	if len(words) > 0 {
		avgWordLen := 0
		for _, w := range words {
			avgWordLen += len(w)
		}
		avgWordLen /= len(words)

		// AI tends to use more uniform word lengths
		if avgWordLen > 4 && avgWordLen < 7 {
			score += 0.05
		}
	}

	// Clamp to valid range
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// fetchTextFromURL fetches text content from a URL.
func (d *textDetector) fetchTextFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: status %d", resp.StatusCode)
	}

	// Limit response size
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB max
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// aggregateScores combines multiple detector scores.
// Currently uses simple average; could be weighted based on detector accuracy.
func (d *textDetector) aggregateScores(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.5 // Neutral if no scores
	}

	sum := 0.0
	for _, s := range scores {
		sum += s
	}

	return sum / float64(len(scores))
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
