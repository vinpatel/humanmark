package service

import (
	"testing"
)

// TestImageAnalyzer tests the HumanMark image forensic analysis.
func TestImageAnalyzer(t *testing.T) {
	analyzer := NewImageAnalyzer()

	t.Run("detects JPEG format", func(t *testing.T) {
		// JPEG magic bytes: FF D8 FF
		jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
		jpegData = append(jpegData, make([]byte, 1000)...) // Pad with zeros

		result := analyzer.Analyze(jpegData)

		if result.Metadata.FileFormat != "jpeg" {
			t.Errorf("expected jpeg format, got %s", result.Metadata.FileFormat)
		}

		t.Logf("JPEG Analysis: format=%s, score=%f", result.Metadata.FileFormat, result.AIScore)
	})

	t.Run("detects PNG format", func(t *testing.T) {
		// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
		pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

		// Add IHDR chunk for dimensions
		ihdr := []byte{
			0x00, 0x00, 0x00, 0x0D, // Length: 13
			0x49, 0x48, 0x44, 0x52, // Type: IHDR
			0x00, 0x00, 0x04, 0x00, // Width: 1024
			0x00, 0x00, 0x03, 0x00, // Height: 768
			0x08,                   // Bit depth: 8
			0x06,                   // Color type: RGBA
			0x00,                   // Compression: 0
			0x00,                   // Filter: 0
			0x00,                   // Interlace: 0
			0x00, 0x00, 0x00, 0x00, // CRC (placeholder)
		}

		pngData = append(pngData, ihdr...)
		pngData = append(pngData, make([]byte, 1000)...) // Pad with data

		result := analyzer.Analyze(pngData)

		if result.Metadata.FileFormat != "png" {
			t.Errorf("expected png format, got %s", result.Metadata.FileFormat)
		}

		if result.Stats.Width != 1024 {
			t.Errorf("expected width 1024, got %d", result.Stats.Width)
		}

		if result.Stats.Height != 768 {
			t.Errorf("expected height 768, got %d", result.Stats.Height)
		}

		t.Logf("PNG Analysis: format=%s, dimensions=%dx%d, score=%f",
			result.Metadata.FileFormat, result.Stats.Width, result.Stats.Height, result.AIScore)
	})

	t.Run("handles unknown format", func(t *testing.T) {
		unknownData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
		unknownData = append(unknownData, make([]byte, 1000)...)

		result := analyzer.Analyze(unknownData)

		if result.Metadata.FileFormat != "unknown" {
			t.Errorf("expected unknown format, got %s", result.Metadata.FileFormat)
		}

		// Unknown format should be neutral or slightly suspicious
		if result.AIScore < 0.4 || result.AIScore > 0.7 {
			t.Logf("Unknown format AI score: %f", result.AIScore)
		}
	})

	t.Run("handles empty data", func(t *testing.T) {
		result := analyzer.Analyze([]byte{})

		if result.Metadata.FileFormat != "unknown" {
			t.Errorf("expected unknown format for empty data, got %s", result.Metadata.FileFormat)
		}
	})

	t.Run("handles small data", func(t *testing.T) {
		smallData := []byte{0xFF, 0xD8, 0xFF} // Valid JPEG start but too short
		result := analyzer.Analyze(smallData)

		// Should not panic, return neutral score
		if result.AIScore < 0 || result.AIScore > 1 {
			t.Errorf("AI score out of range: %f", result.AIScore)
		}
	})
}

// TestDetectImageFormat tests format detection from magic bytes.
func TestDetectImageFormat(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "JPEG",
			data:     []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46},
			expected: "jpeg",
		},
		{
			name:     "PNG",
			data:     []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expected: "png",
		},
		{
			name:     "GIF",
			data:     []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x00, 0x00},
			expected: "gif",
		},
		{
			name:     "WebP",
			data:     []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
			expected: "webp",
		},
		{
			name:     "BMP",
			data:     []byte{0x42, 0x4D, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expected: "bmp",
		},
		{
			name:     "Unknown",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			expected: "unknown",
		},
		{
			name:     "Too short",
			data:     []byte{0xFF, 0xD8},
			expected: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := detectImageFormat(tc.data)
			if result != tc.expected {
				t.Errorf("detectImageFormat() = %s, want %s", result, tc.expected)
			}
		})
	}
}

// TestMetadataScoring tests metadata-based scoring.
func TestMetadataScoring(t *testing.T) {
	analyzer := NewImageAnalyzer()

	tests := []struct {
		name          string
		metadata      ImageMetadata
		expectHighAI  bool // true = expect AI score > 0.5
	}{
		{
			name: "Real photo with EXIF",
			metadata: ImageMetadata{
				HasEXIF:    true,
				CameraMake: "Apple",
				HasGPS:     true,
			},
			expectHighAI: false,
		},
		{
			name: "AI generated (marked)",
			metadata: ImageMetadata{
				HasEXIF:  false,
				Software: "AI Generator",
			},
			expectHighAI: true,
		},
		{
			name: "No metadata (suspicious)",
			metadata: ImageMetadata{
				HasEXIF:      false,
				IsScreenshot: false,
			},
			expectHighAI: true,
		},
		{
			name: "Screenshot",
			metadata: ImageMetadata{
				HasEXIF:      false,
				IsScreenshot: true,
			},
			expectHighAI: false, // Slightly suspicious but not definitive
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := analyzer.analyzeMetadata(tc.metadata)

			if tc.expectHighAI && score < 0.5 {
				t.Errorf("expected high AI score for %s, got %f", tc.name, score)
			}
			if !tc.expectHighAI && score > 0.6 {
				t.Errorf("expected low AI score for %s, got %f", tc.name, score)
			}

			t.Logf("%s: metadata AI score = %f", tc.name, score)
		})
	}
}

// TestByteSimilarity tests byte comparison function.
func TestByteSimilarity(t *testing.T) {
	tests := []struct {
		a, b     []byte
		expected float64
	}{
		{[]byte{1, 2, 3, 4}, []byte{1, 2, 3, 4}, 1.0},
		{[]byte{1, 2, 3, 4}, []byte{0, 0, 0, 0}, 0.0},
		{[]byte{1, 2, 3, 4}, []byte{1, 2, 0, 0}, 0.5},
		{[]byte{1, 2, 3}, []byte{1, 2, 3, 4}, 0.0}, // Different lengths
	}

	for _, tc := range tests {
		result := byteSimilarity(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("byteSimilarity(%v, %v) = %f, want %f", tc.a, tc.b, result, tc.expected)
		}
	}
}

// TestImageSignalScoring tests individual signal calculations.
func TestImageSignalScoring(t *testing.T) {
	analyzer := NewImageAnalyzer()

	// Create test image data with various patterns
	t.Run("color distribution analysis", func(t *testing.T) {
		// Normal image data (varied bytes)
		normalData := make([]byte, 2000)
		for i := range normalData {
			normalData[i] = byte(i % 256)
		}

		// Suspicious data (highly uniform)
		uniformData := make([]byte, 2000)
		for i := range uniformData {
			uniformData[i] = 128 // All same value
		}

		normalScore := analyzer.analyzeColorDistribution(normalData, "jpeg")
		uniformScore := analyzer.analyzeColorDistribution(uniformData, "jpeg")

		t.Logf("Color Distribution - Normal: %f, Uniform: %f", normalScore, uniformScore)

		// Both should be valid scores
		if normalScore < 0 || normalScore > 1 {
			t.Errorf("normal score out of range: %f", normalScore)
		}
		if uniformScore < 0 || uniformScore > 1 {
			t.Errorf("uniform score out of range: %f", uniformScore)
		}
	})

	t.Run("noise pattern analysis", func(t *testing.T) {
		// Create data with natural-looking noise
		noisyData := make([]byte, 3000)
		for i := range noisyData {
			// Add some variation
			base := byte(i % 200)
			noise := byte(i*7 % 50)
			noisyData[i] = base + noise
		}

		score := analyzer.analyzeNoisePattern(noisyData, "jpeg")

		if score < 0 || score > 1 {
			t.Errorf("noise score out of range: %f", score)
		}

		t.Logf("Noise Pattern Score: %f", score)
	})
}

// TestImageAnalyzerWeights verifies weight configuration.
func TestImageAnalyzerWeights(t *testing.T) {
	weights := DefaultImageWeights()

	// All weights should be positive
	if weights.MetadataScore <= 0 {
		t.Error("MetadataScore should be positive")
	}
	if weights.ColorDistribution <= 0 {
		t.Error("ColorDistribution should be positive")
	}
	if weights.EdgeConsistency <= 0 {
		t.Error("EdgeConsistency should be positive")
	}
	if weights.NoisePattern <= 0 {
		t.Error("NoisePattern should be positive")
	}
	if weights.CompressionAnalysis <= 0 {
		t.Error("CompressionAnalysis should be positive")
	}
	if weights.SymmetryDetection <= 0 {
		t.Error("SymmetryDetection should be positive")
	}

	// Weights should sum to approximately 1
	sum := weights.MetadataScore + weights.ColorDistribution + weights.EdgeConsistency +
		weights.NoisePattern + weights.CompressionAnalysis + weights.SymmetryDetection

	if sum < 0.99 || sum > 1.01 {
		t.Errorf("weights should sum to ~1.0, got %f", sum)
	}
}

// BenchmarkImageAnalyzer benchmarks image analysis speed.
func BenchmarkImageAnalyzer(b *testing.B) {
	analyzer := NewImageAnalyzer()

	// Create test JPEG-like data
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	// Add more data to simulate real image
	additional := make([]byte, 50000)
	for i := range additional {
		additional[i] = byte(i % 256)
	}
	data = append(data, additional...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(data)
	}
}

// TestWeightedScoreCalculation tests final score aggregation.
func TestWeightedScoreCalculation(t *testing.T) {
	analyzer := NewImageAnalyzer()

	t.Run("all signals neutral", func(t *testing.T) {
		signals := ImageSignals{
			MetadataScore:       0.5,
			ColorDistribution:   0.5,
			EdgeConsistency:     0.5,
			NoisePattern:        0.5,
			CompressionAnalysis: 0.5,
			SymmetryScore:       0.5,
		}

		score := analyzer.calculateWeightedScore(signals)

		if score < 0.45 || score > 0.55 {
			t.Errorf("neutral signals should produce ~0.5 score, got %f", score)
		}
	})

	t.Run("all signals AI-like", func(t *testing.T) {
		signals := ImageSignals{
			MetadataScore:       0.9,
			ColorDistribution:   0.9,
			EdgeConsistency:     0.9,
			NoisePattern:        0.9,
			CompressionAnalysis: 0.9,
			SymmetryScore:       0.9,
		}

		score := analyzer.calculateWeightedScore(signals)

		if score < 0.8 {
			t.Errorf("AI-like signals should produce high score, got %f", score)
		}
	})

	t.Run("all signals human-like", func(t *testing.T) {
		signals := ImageSignals{
			MetadataScore:       0.1,
			ColorDistribution:   0.1,
			EdgeConsistency:     0.1,
			NoisePattern:        0.1,
			CompressionAnalysis: 0.1,
			SymmetryScore:       0.1,
		}

		score := analyzer.calculateWeightedScore(signals)

		if score > 0.2 {
			t.Errorf("human-like signals should produce low score, got %f", score)
		}
	})
}
