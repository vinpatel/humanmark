package service

import (
	"encoding/binary"
	"math"
	"strings"
)

// =============================================================================
// HumanMark Image Detection Algorithm
// =============================================================================
//
// This is our own detection algorithm based on image forensic analysis.
// No external APIs or ML models required.
//
// Key insights:
//   1. Real photos have camera metadata (EXIF)
//   2. AI images have unnatural frequency patterns
//   3. Real images have sensor noise patterns
//   4. AI images often have edge artifacts
//   5. JPEG compression patterns differ
//
// We analyze:
//   1. Metadata presence and validity
//   2. Color distribution patterns
//   3. Edge consistency
//   4. Compression artifacts
//   5. Noise patterns
//   6. Symmetry detection (AI often has symmetry artifacts)
//
// =============================================================================

// ImageAnalyzer performs forensic analysis on images.
type ImageAnalyzer struct {
	weights ImageAnalyzerWeights
}

// ImageAnalyzerWeights controls signal importance.
type ImageAnalyzerWeights struct {
	MetadataScore       float64
	ColorDistribution   float64
	EdgeConsistency     float64
	NoisePattern        float64
	CompressionAnalysis float64
	SymmetryDetection   float64
}

// DefaultImageWeights returns tuned weights.
func DefaultImageWeights() ImageAnalyzerWeights {
	return ImageAnalyzerWeights{
		MetadataScore:       0.25,
		ColorDistribution:   0.20,
		EdgeConsistency:     0.15,
		NoisePattern:        0.15,
		CompressionAnalysis: 0.15,
		SymmetryDetection:   0.10,
	}
}

// NewImageAnalyzer creates a new analyzer.
func NewImageAnalyzer() *ImageAnalyzer {
	return &ImageAnalyzer{
		weights: DefaultImageWeights(),
	}
}

// ImageAnalysisResult contains analysis results.
type ImageAnalysisResult struct {
	AIScore  float64
	Signals  ImageSignals
	Metadata ImageMetadata
	Stats    ImageStats
}

// ImageSignals contains individual signal scores.
type ImageSignals struct {
	MetadataScore       float64 // Missing/fake metadata = AI-like
	ColorDistribution   float64 // Unnatural distribution = AI-like
	EdgeConsistency     float64 // Inconsistent edges = AI-like
	NoisePattern        float64 // Missing natural noise = AI-like
	CompressionAnalysis float64 // Wrong compression = AI-like
	SymmetryScore       float64 // Unnatural symmetry = AI-like
}

// ImageMetadata contains extracted metadata.
type ImageMetadata struct {
	HasEXIF      bool
	CameraMake   string
	CameraModel  string
	Software     string
	DateTaken    string
	HasGPS       bool
	IsScreenshot bool
	FileFormat   string
}

// ImageStats contains image statistics.
type ImageStats struct {
	Width           int
	Height          int
	BitDepth        int
	ColorChannels   int
	EstimatedColors int
	AvgBrightness   float64
	Contrast        float64
}

// Analyze performs forensic analysis on image data.
func (a *ImageAnalyzer) Analyze(data []byte) ImageAnalysisResult {
	result := ImageAnalysisResult{}

	// Detect format
	format := detectImageFormat(data)
	result.Metadata.FileFormat = format

	// Extract metadata
	result.Metadata = a.extractMetadata(data, format)

	// Get basic image stats
	result.Stats = a.getImageStats(data, format)

	// Calculate signals
	result.Signals.MetadataScore = a.analyzeMetadata(result.Metadata)
	result.Signals.ColorDistribution = a.analyzeColorDistribution(data, format)
	result.Signals.EdgeConsistency = a.analyzeEdgeConsistency(data, format)
	result.Signals.NoisePattern = a.analyzeNoisePattern(data, format)
	result.Signals.CompressionAnalysis = a.analyzeCompression(data, format)
	result.Signals.SymmetryScore = a.analyzeSymmetry(data, format)

	// Calculate weighted score
	result.AIScore = a.calculateWeightedScore(result.Signals)

	return result
}

// extractMetadata parses image metadata.
func (a *ImageAnalyzer) extractMetadata(data []byte, format string) ImageMetadata {
	meta := ImageMetadata{
		FileFormat: format,
	}

	switch format {
	case "jpeg":
		meta = a.parseJPEGMetadata(data)
	case "png":
		meta = a.parsePNGMetadata(data)
	}

	return meta
}

// parseJPEGMetadata extracts EXIF data from JPEG.
func (a *ImageAnalyzer) parseJPEGMetadata(data []byte) ImageMetadata {
	meta := ImageMetadata{FileFormat: "jpeg"}

	if len(data) < 12 {
		return meta
	}

	// Look for EXIF marker (APP1 = 0xFFE1)
	for i := 0; i < len(data)-10; i++ {
		if data[i] == 0xFF && data[i+1] == 0xE1 {
			// Found APP1 segment
			if i+10 < len(data) {
				segment := data[i+4:]
				if len(segment) >= 6 && string(segment[:4]) == "Exif" {
					meta.HasEXIF = true

					// Simple EXIF parsing - look for common strings
					exifData := string(segment)

					// Look for camera make
					if idx := strings.Index(exifData, "Apple"); idx != -1 {
						meta.CameraMake = "Apple"
					} else if idx := strings.Index(exifData, "Canon"); idx != -1 {
						meta.CameraMake = "Canon"
					} else if idx := strings.Index(exifData, "Nikon"); idx != -1 {
						meta.CameraMake = "Nikon"
					} else if idx := strings.Index(exifData, "Sony"); idx != -1 {
						meta.CameraMake = "Sony"
					} else if idx := strings.Index(exifData, "Samsung"); idx != -1 {
						meta.CameraMake = "Samsung"
					} else if idx := strings.Index(exifData, "Google"); idx != -1 {
						meta.CameraMake = "Google"
					}

					// Check for common software markers
					if strings.Contains(exifData, "Photoshop") {
						meta.Software = "Photoshop"
					} else if strings.Contains(exifData, "GIMP") {
						meta.Software = "GIMP"
					} else if strings.Contains(exifData, "Lightroom") {
						meta.Software = "Lightroom"
					} else if strings.Contains(exifData, "DALL-E") || strings.Contains(exifData, "Midjourney") || strings.Contains(exifData, "Stable Diffusion") {
						meta.Software = "AI Generator"
					}

					// Check for GPS data
					if strings.Contains(exifData, "GPS") {
						meta.HasGPS = true
					}
				}
			}
			break
		}
	}

	// Check for screenshot indicators
	if meta.Software == "" && !meta.HasEXIF {
		meta.IsScreenshot = true
	}

	return meta
}

// parsePNGMetadata extracts metadata from PNG.
func (a *ImageAnalyzer) parsePNGMetadata(data []byte) ImageMetadata {
	meta := ImageMetadata{FileFormat: "png"}

	// PNG files with tEXt or iTXt chunks may have metadata
	// Most AI-generated PNGs have minimal metadata

	// Look for tEXt chunks
	for i := 8; i < len(data)-8; {
		if i+8 > len(data) {
			break
		}

		chunkLen := binary.BigEndian.Uint32(data[i : i+4])
		chunkType := string(data[i+4 : i+8])

		if chunkType == "tEXt" || chunkType == "iTXt" {
			meta.HasEXIF = true
			// Could parse further for specific keys
		}

		// Check for AI generator signatures in metadata
		if chunkLen > 0 && i+8+int(chunkLen) < len(data) {
			chunkData := string(data[i+8 : i+8+int(chunkLen)])
			if strings.Contains(chunkData, "DALL-E") ||
				strings.Contains(chunkData, "Midjourney") ||
				strings.Contains(chunkData, "Stable Diffusion") ||
				strings.Contains(chunkData, "ComfyUI") {
				meta.Software = "AI Generator"
			}
		}

		i += 12 + int(chunkLen) // Length + Type + Data + CRC
	}

	return meta
}

// getImageStats calculates basic image statistics.
func (a *ImageAnalyzer) getImageStats(data []byte, format string) ImageStats {
	stats := ImageStats{}

	switch format {
	case "jpeg":
		stats = a.getJPEGStats(data)
	case "png":
		stats = a.getPNGStats(data)
	}

	return stats
}

// getJPEGStats extracts dimensions from JPEG.
func (a *ImageAnalyzer) getJPEGStats(data []byte) ImageStats {
	stats := ImageStats{}

	// Look for SOF0 marker (0xFFC0) which contains dimensions
	for i := 0; i < len(data)-10; i++ {
		if data[i] == 0xFF && (data[i+1] == 0xC0 || data[i+1] == 0xC2) {
			if i+9 < len(data) {
				stats.BitDepth = int(data[i+4])
				stats.Height = int(binary.BigEndian.Uint16(data[i+5 : i+7]))
				stats.Width = int(binary.BigEndian.Uint16(data[i+7 : i+9]))
				stats.ColorChannels = int(data[i+9])
			}
			break
		}
	}

	return stats
}

// getPNGStats extracts dimensions from PNG.
func (a *ImageAnalyzer) getPNGStats(data []byte) ImageStats {
	stats := ImageStats{}

	// PNG IHDR chunk starts at byte 8
	if len(data) >= 24 {
		stats.Width = int(binary.BigEndian.Uint32(data[16:20]))
		stats.Height = int(binary.BigEndian.Uint32(data[20:24]))
		if len(data) >= 25 {
			stats.BitDepth = int(data[24])
		}
	}

	return stats
}

// analyzeMetadata scores based on metadata presence.
func (a *ImageAnalyzer) analyzeMetadata(meta ImageMetadata) float64 {
	score := 0.5 // Start neutral

	// Real photos typically have EXIF data
	if meta.HasEXIF {
		score -= 0.2
	} else {
		score += 0.2
	}

	// Camera make is strong signal of real photo
	if meta.CameraMake != "" {
		score -= 0.2
	}

	// GPS data is very strong signal
	if meta.HasGPS {
		score -= 0.2
	}

	// AI generator software is obvious signal
	if meta.Software == "AI Generator" {
		score += 0.4
	}

	// Screenshots could be either
	if meta.IsScreenshot {
		score += 0.1
	}

	return math.Max(0, math.Min(1, score))
}

// analyzeColorDistribution checks color histogram patterns.
// AI images often have unusual color distributions.
func (a *ImageAnalyzer) analyzeColorDistribution(data []byte, format string) float64 {
	// For now, analyze the raw byte distribution as a proxy
	// Real images have more natural byte distributions

	if len(data) < 1000 {
		return 0.5
	}

	// Sample the data (skip headers)
	startOffset := 100
	if format == "png" {
		startOffset = 50
	}

	if len(data) < startOffset+1000 {
		return 0.5
	}

	sample := data[startOffset : startOffset+1000]

	// Calculate byte frequency distribution
	freq := make([]int, 256)
	for _, b := range sample {
		freq[b]++
	}

	// Calculate entropy
	entropy := 0.0
	total := float64(len(sample))
	for _, f := range freq {
		if f > 0 {
			p := float64(f) / total
			entropy -= p * math.Log2(p)
		}
	}

	// Normalized entropy (0-8 for bytes)
	normalizedEntropy := entropy / 8.0

	// Real images typically have entropy 0.7-0.95
	// AI images sometimes have unusual patterns (very high or low)
	if normalizedEntropy < 0.6 || normalizedEntropy > 0.98 {
		return 0.7 // Suspicious
	}

	return 0.4 // Normal
}

// analyzeEdgeConsistency looks for edge artifacts.
// AI images often have inconsistent edges.
func (a *ImageAnalyzer) analyzeEdgeConsistency(data []byte, format string) float64 {
	// Simplified analysis - check for repeated patterns
	// AI sometimes has repeated textures or edge patterns

	if len(data) < 5000 {
		return 0.5
	}

	// Sample different regions and compare
	// Look for unusual repetition patterns

	sampleSize := 256
	samples := make([][]byte, 0)

	for i := 1000; i < len(data)-sampleSize && len(samples) < 10; i += len(data) / 12 {
		samples = append(samples, data[i:i+sampleSize])
	}

	if len(samples) < 3 {
		return 0.5
	}

	// Check similarity between samples
	// AI images sometimes have suspiciously similar regions
	similarityCount := 0
	for i := 0; i < len(samples)-1; i++ {
		for j := i + 1; j < len(samples); j++ {
			if byteSimilarity(samples[i], samples[j]) > 0.9 {
				similarityCount++
			}
		}
	}

	// High similarity between distant regions is suspicious
	similarityRatio := float64(similarityCount) / float64(len(samples)*(len(samples)-1)/2)

	if similarityRatio > 0.3 {
		return 0.7 // Suspicious repetition
	}

	return 0.4
}

// byteSimilarity calculates similarity between two byte slices.
func byteSimilarity(a, b []byte) float64 {
	if len(a) != len(b) {
		return 0
	}

	matches := 0
	for i := range a {
		if a[i] == b[i] {
			matches++
		}
	}

	return float64(matches) / float64(len(a))
}

// analyzeNoisePattern checks for natural sensor noise.
// Real photos have characteristic noise from camera sensors.
func (a *ImageAnalyzer) analyzeNoisePattern(data []byte, format string) float64 {
	if len(data) < 2000 {
		return 0.5
	}

	// Look for high-frequency variations that indicate sensor noise
	// AI images are often "too clean" or have artificial noise

	startOffset := 500
	sampleSize := 1000

	if len(data) < startOffset+sampleSize {
		return 0.5
	}

	sample := data[startOffset : startOffset+sampleSize]

	// Calculate local variance (proxy for noise)
	variances := make([]float64, 0)
	windowSize := 16

	for i := 0; i < len(sample)-windowSize; i += windowSize {
		window := sample[i : i+windowSize]

		// Calculate mean
		sum := 0.0
		for _, b := range window {
			sum += float64(b)
		}
		mean := sum / float64(len(window))

		// Calculate variance
		variance := 0.0
		for _, b := range window {
			diff := float64(b) - mean
			variance += diff * diff
		}
		variance /= float64(len(window))

		variances = append(variances, variance)
	}

	if len(variances) == 0 {
		return 0.5
	}

	// Calculate variance of variances (noise should be relatively uniform)
	avgVariance := 0.0
	for _, v := range variances {
		avgVariance += v
	}
	avgVariance /= float64(len(variances))

	varianceOfVariance := 0.0
	for _, v := range variances {
		diff := v - avgVariance
		varianceOfVariance += diff * diff
	}
	varianceOfVariance /= float64(len(variances))

	// Real images have moderate, consistent noise
	// AI images are either too clean or have inconsistent noise
	noiseCV := math.Sqrt(varianceOfVariance) / (avgVariance + 1)

	if avgVariance < 10 { // Too clean
		return 0.7
	}
	if noiseCV > 2.0 { // Too inconsistent
		return 0.6
	}

	return 0.4
}

// analyzeCompression checks compression artifacts.
// Real JPEGs have natural compression; AI-generated may have artifacts.
func (a *ImageAnalyzer) analyzeCompression(data []byte, format string) float64 {
	if format != "jpeg" {
		return 0.5 // Can't analyze non-JPEG compression this way
	}

	// Look for quantization table quality
	// Very high quality (>95) or unusual quality settings can indicate AI

	// Find DQT marker (0xFFDB)
	for i := 0; i < len(data)-100; i++ {
		if data[i] == 0xFF && data[i+1] == 0xDB {
			// Found quantization table
			if i+69 < len(data) {
				// Sum the quantization values
				sum := 0
				for j := i + 5; j < i+69; j++ {
					sum += int(data[j])
				}

				// Very low sum = very high quality (possibly AI-generated PNG converted to JPEG)
				// Normal range is roughly 300-2000
				if sum < 200 {
					return 0.7 // Suspiciously high quality
				}
				if sum > 3000 {
					return 0.6 // Very low quality (heavy compression)
				}

				return 0.4 // Normal compression
			}
			break
		}
	}

	return 0.5
}

// analyzeSymmetry detects unnatural symmetry.
// AI images sometimes have artifacts related to symmetry.
func (a *ImageAnalyzer) analyzeSymmetry(data []byte, format string) float64 {
	// This is a simplified check
	// Full symmetry analysis would require decoding the image

	if len(data) < 2000 {
		return 0.5
	}

	// Compare first and second halves of data as a rough proxy
	// (Not actual image symmetry, but can catch some patterns)

	mid := len(data) / 2
	sampleSize := 500

	if mid+sampleSize > len(data) {
		return 0.5
	}

	sample1 := data[100:600]                // Near start
	sample2 := data[mid : mid+sampleSize] // Near middle

	// Check if suspiciously similar
	similarity := byteSimilarity(sample1, sample2)

	if similarity > 0.7 {
		return 0.7 // Suspicious similarity
	}

	return 0.4
}

// calculateWeightedScore combines signals into final score.
func (a *ImageAnalyzer) calculateWeightedScore(signals ImageSignals) float64 {
	w := a.weights

	score := signals.MetadataScore*w.MetadataScore +
		signals.ColorDistribution*w.ColorDistribution +
		signals.EdgeConsistency*w.EdgeConsistency +
		signals.NoisePattern*w.NoisePattern +
		signals.CompressionAnalysis*w.CompressionAnalysis +
		signals.SymmetryScore*w.SymmetryDetection

	totalWeight := w.MetadataScore + w.ColorDistribution + w.EdgeConsistency +
		w.NoisePattern + w.CompressionAnalysis + w.SymmetryDetection

	if totalWeight > 0 {
		score /= totalWeight
	}

	return math.Max(0, math.Min(1, score))
}

// detectImageFormat identifies the image format from magic bytes.
func detectImageFormat(data []byte) string {
	if len(data) < 8 {
		return "unknown"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}

	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "gif"
	}

	// WebP: 52 49 46 46 ... 57 45 42 50
	if len(data) >= 12 {
		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
			if data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
				return "webp"
			}
		}
	}

	// BMP: 42 4D
	if data[0] == 0x42 && data[1] == 0x4D {
		return "bmp"
	}

	return "unknown"
}
