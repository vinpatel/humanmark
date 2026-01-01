package service

import (
	"testing"
)

// TestVideoAnalyzer tests the HumanMark video forensic analysis.
func TestVideoAnalyzer(t *testing.T) {
	analyzer := NewVideoAnalyzer()

	t.Run("detects MP4 format", func(t *testing.T) {
		// MP4 with ftyp atom
		mp4Data := []byte{
			0x00, 0x00, 0x00, 0x18, // Size: 24
			'f', 't', 'y', 'p',     // Type: ftyp
			'i', 's', 'o', 'm',     // Brand: isom
			0x00, 0x00, 0x00, 0x00, // Version
			'i', 's', 'o', 'm',     // Compatible brand
			'a', 'v', 'c', '1',     // Compatible brand
		}
		// Add more data
		mp4Data = append(mp4Data, make([]byte, 5000)...)

		result := analyzer.Analyze(mp4Data)

		if result.Metadata.Format != "mp4" {
			t.Errorf("expected mp4 format, got %s", result.Metadata.Format)
		}

		t.Logf("MP4 Analysis: format=%s, score=%f", result.Metadata.Format, result.AIScore)
	})

	t.Run("detects WebM format", func(t *testing.T) {
		// WebM EBML header
		webmData := []byte{
			0x1A, 0x45, 0xDF, 0xA3, // EBML magic
			0x01, 0x00, 0x00, 0x00,
		}
		webmData = append(webmData, []byte("webm")...)
		webmData = append(webmData, make([]byte, 5000)...)

		result := analyzer.Analyze(webmData)

		if result.Metadata.Format != "webm" {
			t.Errorf("expected webm format, got %s", result.Metadata.Format)
		}

		t.Logf("WebM Analysis: format=%s, score=%f", result.Metadata.Format, result.AIScore)
	})

	t.Run("detects AVI format", func(t *testing.T) {
		// AVI RIFF header
		aviData := []byte{
			'R', 'I', 'F', 'F',
			0x00, 0x00, 0x00, 0x00, // Size
			'A', 'V', 'I', ' ',
		}
		aviData = append(aviData, make([]byte, 5000)...)

		result := analyzer.Analyze(aviData)

		if result.Metadata.Format != "avi" {
			t.Errorf("expected avi format, got %s", result.Metadata.Format)
		}

		t.Logf("AVI Analysis: format=%s, score=%f", result.Metadata.Format, result.AIScore)
	})

	t.Run("handles unknown format", func(t *testing.T) {
		unknownData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
		unknownData = append(unknownData, make([]byte, 5000)...)

		result := analyzer.Analyze(unknownData)

		if result.Metadata.Format != "unknown" {
			t.Errorf("expected unknown format, got %s", result.Metadata.Format)
		}
	})

	t.Run("handles empty data", func(t *testing.T) {
		result := analyzer.Analyze([]byte{})

		if result.Metadata.Format != "unknown" {
			t.Errorf("expected unknown format for empty data, got %s", result.Metadata.Format)
		}
	})

	t.Run("handles small data", func(t *testing.T) {
		smallData := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'}
		result := analyzer.Analyze(smallData)

		// Should not panic
		if result.AIScore < 0 || result.AIScore > 1 {
			t.Errorf("AI score out of range: %f", result.AIScore)
		}
	})
}

// TestDetectVideoFormat tests format detection.
func TestDetectVideoFormat(t *testing.T) {
	analyzer := NewVideoAnalyzer()

	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name: "MP4 isom",
			data: []byte{
				0x00, 0x00, 0x00, 0x18,
				'f', 't', 'y', 'p',
				'i', 's', 'o', 'm',
			},
			expected: "mp4",
		},
		{
			name: "MP4 avc1",
			data: []byte{
				0x00, 0x00, 0x00, 0x18,
				'f', 't', 'y', 'p',
				'a', 'v', 'c', '1',
			},
			expected: "mp4",
		},
		{
			name: "QuickTime MOV",
			data: []byte{
				0x00, 0x00, 0x00, 0x18,
				'f', 't', 'y', 'p',
				'q', 't', ' ', ' ',
			},
			expected: "mov",
		},
		{
			name:     "WebM",
			data:     append([]byte{0x1A, 0x45, 0xDF, 0xA3}, append(make([]byte, 20), []byte("webm")...)...),
			expected: "webm",
		},
		{
			name: "AVI",
			data: []byte{
				'R', 'I', 'F', 'F',
				0x00, 0x00, 0x00, 0x00,
				'A', 'V', 'I', ' ',
			},
			expected: "avi",
		},
		{
			name:     "FLV",
			data:     []byte{'F', 'L', 'V', 0x01, 0x05, 0x00, 0x00, 0x00, 0x09},
			expected: "flv",
		},
		{
			name:     "Unknown",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B},
			expected: "unknown",
		},
		{
			name:     "Too short",
			data:     []byte{0x00, 0x00},
			expected: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.detectVideoFormat(tc.data)
			if result != tc.expected {
				t.Errorf("detectVideoFormat() = %s, want %s", result, tc.expected)
			}
		})
	}
}

// TestVideoMetadataScoring tests metadata-based scoring.
func TestVideoMetadataScoring(t *testing.T) {
	analyzer := NewVideoAnalyzer()

	tests := []struct {
		name         string
		metadata     VideoMetadata
		expectHighAI bool
	}{
		{
			name: "Real video with audio",
			metadata: VideoMetadata{
				Format:      "mp4",
				HasAudio:    true,
				HasVideo:    true,
				EncoderName: "ffmpeg",
			},
			expectHighAI: false,
		},
		{
			name: "AI marked video",
			metadata: VideoMetadata{
				Format:     "mp4",
				HasAudio:   false,
				HasVideo:   true,
				IsAIMarked: true,
			},
			expectHighAI: true,
		},
		{
			name: "Video without audio (suspicious)",
			metadata: VideoMetadata{
				Format:   "mp4",
				HasAudio: false,
				HasVideo: true,
			},
			expectHighAI: true, // Slightly suspicious
		},
		{
			name: "AI encoder detected",
			metadata: VideoMetadata{
				Format:      "mp4",
				HasAudio:    true,
				HasVideo:    true,
				EncoderName: "Runway",
			},
			expectHighAI: true,
		},
		{
			name: "Professional encoder",
			metadata: VideoMetadata{
				Format:      "mp4",
				HasAudio:    true,
				HasVideo:    true,
				EncoderName: "Adobe Premiere",
			},
			expectHighAI: false,
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

// TestContainsAIMarker tests AI marker detection.
func TestContainsAIMarker(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Created with Runway Gen-2", true},
		{"Pika Labs video", true},
		{"OpenAI Sora generated", true},
		{"Made with stable video diffusion", true},
		{"AI-generated content", true},
		{"Recorded on iPhone", false},
		{"Encoded with ffmpeg", false},
		{"Adobe Premiere Pro", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := containsAIMarker(tc.input)
			if result != tc.expected {
				t.Errorf("containsAIMarker(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

// TestExtractEncoder tests encoder name extraction.
func TestExtractEncoder(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Lavf58.76.100", "ffmpeg"},
		{"HandBrake 1.5.0", "HandBrake"},
		{"Adobe Premiere Pro CC", "Adobe Premiere"},
		{"x264 - core 164", "x264"},
		{"Runway Gen-2", "Runway"},
		{"Unknown software", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := extractEncoder(tc.input)
			if result != tc.expected {
				t.Errorf("extractEncoder(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestCalculateEntropy tests entropy calculation.
func TestCalculateEntropy(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		entropy := calculateEntropy([]byte{})
		if entropy != 0 {
			t.Errorf("expected 0 entropy for empty data, got %f", entropy)
		}
	})

	t.Run("uniform data", func(t *testing.T) {
		// All same byte = 0 entropy
		data := make([]byte, 1000)
		for i := range data {
			data[i] = 0x42
		}
		entropy := calculateEntropy(data)
		if entropy != 0 {
			t.Errorf("expected 0 entropy for uniform data, got %f", entropy)
		}
	})

	t.Run("random data", func(t *testing.T) {
		// Varied bytes = high entropy
		data := make([]byte, 256)
		for i := range data {
			data[i] = byte(i)
		}
		entropy := calculateEntropy(data)
		// Should be close to 1.0 (maximum for 256 equally distributed values)
		if entropy < 0.9 {
			t.Errorf("expected high entropy for varied data, got %f", entropy)
		}
	})
}

// TestVideoAnalyzerWeights tests weight configuration.
func TestVideoAnalyzerWeights(t *testing.T) {
	weights := DefaultVideoWeights()

	// All weights should be positive
	if weights.MetadataScore <= 0 {
		t.Error("MetadataScore should be positive")
	}
	if weights.ContainerAnalysis <= 0 {
		t.Error("ContainerAnalysis should be positive")
	}
	if weights.AudioPresence <= 0 {
		t.Error("AudioPresence should be positive")
	}
	if weights.TemporalPattern <= 0 {
		t.Error("TemporalPattern should be positive")
	}
	if weights.EncodingSignature <= 0 {
		t.Error("EncodingSignature should be positive")
	}
	if weights.BitrateConsistency <= 0 {
		t.Error("BitrateConsistency should be positive")
	}

	// Weights should sum to approximately 1
	sum := weights.MetadataScore + weights.ContainerAnalysis + weights.AudioPresence +
		weights.TemporalPattern + weights.EncodingSignature + weights.BitrateConsistency

	if sum < 0.99 || sum > 1.01 {
		t.Errorf("weights should sum to ~1.0, got %f", sum)
	}
}

// TestAudioPresenceSignal tests audio presence scoring.
func TestAudioPresenceSignal(t *testing.T) {
	analyzer := NewVideoAnalyzer()

	t.Run("video with audio scores lower", func(t *testing.T) {
		metaWithAudio := VideoMetadata{HasAudio: true}
		metaNoAudio := VideoMetadata{HasAudio: false}

		scoreWithAudio := analyzer.analyzeAudioPresence(metaWithAudio, nil, "mp4")
		scoreNoAudio := analyzer.analyzeAudioPresence(metaNoAudio, nil, "mp4")

		if scoreWithAudio >= scoreNoAudio {
			t.Errorf("video with audio should score lower: with=%f, without=%f",
				scoreWithAudio, scoreNoAudio)
		}
	})
}

// BenchmarkVideoAnalyzer benchmarks video analysis.
func BenchmarkVideoAnalyzer(b *testing.B) {
	analyzer := NewVideoAnalyzer()

	// Create test MP4-like data
	data := []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x00,
		'i', 's', 'o', 'm',
		'a', 'v', 'c', '1',
	}
	// Add more data to simulate real video
	data = append(data, make([]byte, 100000)...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(data)
	}
}

// TestVideoWeightedScore tests final score calculation.
func TestVideoWeightedScore(t *testing.T) {
	analyzer := NewVideoAnalyzer()

	t.Run("neutral signals produce neutral score", func(t *testing.T) {
		signals := VideoSignals{
			MetadataScore:      0.5,
			ContainerAnalysis:  0.5,
			AudioPresence:      0.5,
			TemporalPattern:    0.5,
			EncodingSignature:  0.5,
			BitrateConsistency: 0.5,
		}

		score := analyzer.calculateWeightedScore(signals)

		if score < 0.45 || score > 0.55 {
			t.Errorf("neutral signals should produce ~0.5, got %f", score)
		}
	})

	t.Run("high AI signals produce high score", func(t *testing.T) {
		signals := VideoSignals{
			MetadataScore:      0.9,
			ContainerAnalysis:  0.8,
			AudioPresence:      0.9,
			TemporalPattern:    0.8,
			EncodingSignature:  0.7,
			BitrateConsistency: 0.8,
		}

		score := analyzer.calculateWeightedScore(signals)

		if score < 0.7 {
			t.Errorf("high AI signals should produce high score, got %f", score)
		}
	})
}
