package service

import (
	"bytes"
	"encoding/binary"
	"math"
)

// =============================================================================
// HumanMark Audio Detection Algorithm
// =============================================================================
//
// Audio AI detection focuses on identifying synthetic speech and AI-generated music.
//
// Key signals:
//   1. Format and codec metadata
//   2. Audio characteristics (sampling rate, channels, bitrate)
//   3. Spectral patterns (via byte analysis as proxy)
//   4. Silence/noise patterns
//   5. Known AI audio signatures
//   6. Recording device markers
//
// Note: Full spectral analysis requires decoding. Our analysis works on
// container metadata and compressed byte patterns.
//
// =============================================================================

// AudioAnalyzer performs forensic analysis on audio files.
type AudioAnalyzer struct {
	weights AudioAnalyzerWeights
}

// AudioAnalyzerWeights controls signal importance.
type AudioAnalyzerWeights struct {
	MetadataScore     float64
	FormatAnalysis    float64
	PatternAnalysis   float64
	QualityIndicators float64
	AISignatures      float64
	NoiseProfile      float64
}

// DefaultAudioWeights returns tuned weights.
func DefaultAudioWeights() AudioAnalyzerWeights {
	return AudioAnalyzerWeights{
		MetadataScore:     0.25,
		FormatAnalysis:    0.15,
		PatternAnalysis:   0.20,
		QualityIndicators: 0.15,
		AISignatures:      0.15,
		NoiseProfile:      0.10,
	}
}

// NewAudioAnalyzer creates a new analyzer.
func NewAudioAnalyzer() *AudioAnalyzer {
	return &AudioAnalyzer{
		weights: DefaultAudioWeights(),
	}
}

// AudioAnalysisResult contains analysis results.
type AudioAnalysisResult struct {
	AIScore  float64
	Signals  AudioSignals
	Metadata AudioMetadata
	Stats    AudioStats
}

// AudioSignals contains individual signal scores.
type AudioSignals struct {
	MetadataScore     float64 // Missing/fake metadata = AI-like
	FormatAnalysis    float64 // Unusual format = suspicious
	PatternAnalysis   float64 // Unusual patterns = AI-like
	QualityIndicators float64 // Unusual quality = suspicious
	AISignatures      float64 // Known AI markers = AI
	NoiseProfile      float64 // Unnatural noise = AI-like
}

// AudioMetadata contains extracted metadata.
type AudioMetadata struct {
	Format       string // mp3, wav, flac, ogg, etc.
	SampleRate   int    // Hz
	Channels     int    // 1=mono, 2=stereo
	BitDepth     int    // bits per sample
	Bitrate      int    // kbps (for compressed formats)
	Duration     float64 // seconds (estimated)
	HasID3       bool
	Artist       string
	Title        string
	EncoderName  string
	IsAIMarked   bool
	HasRecording bool // Markers of real recording
}

// AudioStats contains audio statistics.
type AudioStats struct {
	FileSize        int64
	EstimatedFrames int
	SilenceRatio    float64 // Proportion of silence
	PeakAmplitude   float64 // Normalized 0-1
	DynamicRange    float64 // Difference between loud/quiet
}

// Analyze performs forensic analysis on audio data.
func (a *AudioAnalyzer) Analyze(data []byte) AudioAnalysisResult {
	result := AudioAnalysisResult{}
	result.Stats.FileSize = int64(len(data))

	// Detect format
	format := a.detectAudioFormat(data)
	result.Metadata.Format = format

	// Extract metadata based on format
	switch format {
	case "mp3":
		result.Metadata, result.Stats = a.analyzeMP3(data)
	case "wav":
		result.Metadata, result.Stats = a.analyzeWAV(data)
	case "flac":
		result.Metadata, result.Stats = a.analyzeFLAC(data)
	case "ogg":
		result.Metadata, result.Stats = a.analyzeOGG(data)
	case "m4a", "aac":
		result.Metadata, result.Stats = a.analyzeM4A(data)
	default:
		result.Metadata.Format = format
	}

	// Calculate signals
	result.Signals.MetadataScore = a.analyzeMetadata(result.Metadata)
	result.Signals.FormatAnalysis = a.analyzeFormat(result.Metadata, data)
	result.Signals.PatternAnalysis = a.analyzePatterns(data, format)
	result.Signals.QualityIndicators = a.analyzeQuality(result.Metadata, result.Stats)
	result.Signals.AISignatures = a.detectAISignatures(data, result.Metadata)
	result.Signals.NoiseProfile = a.analyzeNoiseProfile(data, format)

	// Calculate weighted score
	result.AIScore = a.calculateWeightedScore(result.Signals)

	return result
}

// detectAudioFormat identifies audio format from magic bytes.
func (a *AudioAnalyzer) detectAudioFormat(data []byte) string {
	if len(data) < 12 {
		return "unknown"
	}

	// MP3: FF FB, FF FA, FF F3, FF F2 (frame sync) or ID3 tag
	if data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
		return "mp3"
	}
	if data[0] == 'I' && data[1] == 'D' && data[2] == '3' {
		return "mp3"
	}

	// WAV: RIFF....WAVE
	if bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
		return "wav"
	}

	// FLAC: fLaC
	if bytes.Equal(data[0:4], []byte("fLaC")) {
		return "flac"
	}

	// OGG: OggS
	if bytes.Equal(data[0:4], []byte("OggS")) {
		return "ogg"
	}

	// M4A/AAC: ftyp M4A or similar
	if len(data) >= 8 && bytes.Equal(data[4:8], []byte("ftyp")) {
		if len(data) >= 12 {
			brand := string(data[8:12])
			if brand == "M4A " || brand == "mp42" || brand == "isom" {
				return "m4a"
			}
		}
		return "m4a"
	}

	// AAC ADTS: sync word 0xFFF
	if data[0] == 0xFF && (data[1]&0xF0) == 0xF0 {
		return "aac"
	}

	return "unknown"
}

// analyzeMP3 extracts metadata from MP3 files.
func (a *AudioAnalyzer) analyzeMP3(data []byte) (AudioMetadata, AudioStats) {
	meta := AudioMetadata{Format: "mp3"}
	stats := AudioStats{FileSize: int64(len(data))}

	// Check for ID3v2 tag at start
	if len(data) > 10 && data[0] == 'I' && data[1] == 'D' && data[2] == '3' {
		meta.HasID3 = true

		// ID3v2 size is syncsafe integer at bytes 6-9
		id3Size := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])

		// Look for AI markers in ID3 tags
		if id3Size > 0 && id3Size+10 < len(data) {
			id3Data := string(data[10 : 10+id3Size])
			if containsAIAudioMarker(id3Data) {
				meta.IsAIMarked = true
			}
			meta.EncoderName = extractAudioEncoder(id3Data)

			// Check for recording indicators
			if containsRecordingMarker(id3Data) {
				meta.HasRecording = true
			}
		}
	}

	// Find first MP3 frame to get audio params
	for i := 0; i < len(data)-4; i++ {
		if data[i] == 0xFF && (data[i+1]&0xE0) == 0xE0 {
			// Found frame sync
			header := binary.BigEndian.Uint32(data[i : i+4])

			// Extract bitrate index and sample rate index
			version := (header >> 19) & 0x03
			layer := (header >> 17) & 0x03
			bitrateIdx := (header >> 12) & 0x0F
			srIdx := (header >> 10) & 0x03
			channelMode := (header >> 6) & 0x03

			// Set sample rate (simplified - MPEG1 Layer 3)
			sampleRates := []int{44100, 48000, 32000, 0}
			if srIdx < 3 {
				meta.SampleRate = sampleRates[srIdx]
			}

			// Set channels
			if channelMode == 3 {
				meta.Channels = 1 // Mono
			} else {
				meta.Channels = 2 // Stereo
			}

			// Bitrate table for MPEG1 Layer 3
			bitrates := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
			if bitrateIdx < 15 {
				meta.Bitrate = bitrates[bitrateIdx]
			}

			// Avoid unused variable warnings
			_ = version
			_ = layer

			break
		}
	}

	// Check ID3v1 tag at end
	if len(data) >= 128 {
		if bytes.Equal(data[len(data)-128:len(data)-125], []byte("TAG")) {
			meta.HasID3 = true
		}
	}

	return meta, stats
}

// analyzeWAV extracts metadata from WAV files.
func (a *AudioAnalyzer) analyzeWAV(data []byte) (AudioMetadata, AudioStats) {
	meta := AudioMetadata{Format: "wav"}
	stats := AudioStats{FileSize: int64(len(data))}

	if len(data) < 44 {
		return meta, stats
	}

	// Parse WAV header
	// fmt chunk starts at byte 12
	if bytes.Equal(data[12:16], []byte("fmt ")) {
		// Audio format at 20-21 (1 = PCM)
		audioFormat := binary.LittleEndian.Uint16(data[20:22])
		if audioFormat == 1 {
			meta.EncoderName = "PCM"
		}

		// Channels at 22-23
		meta.Channels = int(binary.LittleEndian.Uint16(data[22:24]))

		// Sample rate at 24-27
		meta.SampleRate = int(binary.LittleEndian.Uint32(data[24:28]))

		// Bits per sample at 34-35
		meta.BitDepth = int(binary.LittleEndian.Uint16(data[34:36]))
	}

	// Look for metadata chunks (LIST, INFO, etc.)
	for i := 36; i < len(data)-8; i++ {
		if bytes.Equal(data[i:i+4], []byte("LIST")) {
			chunkSize := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
			if i+8+chunkSize <= len(data) {
				listData := string(data[i+8 : i+8+chunkSize])
				if containsAIAudioMarker(listData) {
					meta.IsAIMarked = true
				}
				if containsRecordingMarker(listData) {
					meta.HasRecording = true
				}
			}
			break
		}
	}

	return meta, stats
}

// analyzeFLAC extracts metadata from FLAC files.
func (a *AudioAnalyzer) analyzeFLAC(data []byte) (AudioMetadata, AudioStats) {
	meta := AudioMetadata{Format: "flac"}
	stats := AudioStats{FileSize: int64(len(data))}

	if len(data) < 42 {
		return meta, stats
	}

	// FLAC STREAMINFO block starts at byte 4
	// Sample rate at bytes 18-20 (20 bits)
	if len(data) >= 22 {
		sr := (int(data[18]) << 12) | (int(data[19]) << 4) | (int(data[20]) >> 4)
		meta.SampleRate = sr
	}

	// Channels at byte 20 (3 bits) + 1
	if len(data) >= 21 {
		meta.Channels = int((data[20]>>1)&0x07) + 1
	}

	// Bits per sample at bytes 20-21 (5 bits) + 1
	if len(data) >= 22 {
		bps := int((data[20]&0x01)<<4) | int((data[21]>>4)&0x0F)
		meta.BitDepth = bps + 1
	}

	// Look for Vorbis comment block for metadata
	for i := 4; i < len(data)-4 && i < 10000; {
		blockType := data[i] & 0x7F
		isLast := (data[i] & 0x80) != 0
		blockSize := int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])

		if blockType == 4 { // Vorbis comment
			if i+4+blockSize <= len(data) {
				commentData := string(data[i+4 : i+4+blockSize])
				if containsAIAudioMarker(commentData) {
					meta.IsAIMarked = true
				}
				meta.EncoderName = extractAudioEncoder(commentData)
			}
		}

		i += 4 + blockSize
		if isLast {
			break
		}
	}

	return meta, stats
}

// analyzeOGG extracts metadata from OGG files.
func (a *AudioAnalyzer) analyzeOGG(data []byte) (AudioMetadata, AudioStats) {
	meta := AudioMetadata{Format: "ogg"}
	stats := AudioStats{FileSize: int64(len(data))}

	// Look for Vorbis identification header
	if bytes.Contains(data[:min(len(data), 1000)], []byte("vorbis")) {
		meta.EncoderName = "Vorbis"
	}

	// Look for Opus
	if bytes.Contains(data[:min(len(data), 1000)], []byte("OpusHead")) {
		meta.Format = "opus"
		meta.EncoderName = "Opus"
	}

	// Check for AI markers in comments
	if len(data) > 500 {
		headerData := string(data[:min(len(data), 5000)])
		if containsAIAudioMarker(headerData) {
			meta.IsAIMarked = true
		}
	}

	return meta, stats
}

// analyzeM4A extracts metadata from M4A/AAC files.
func (a *AudioAnalyzer) analyzeM4A(data []byte) (AudioMetadata, AudioStats) {
	meta := AudioMetadata{Format: "m4a"}
	stats := AudioStats{FileSize: int64(len(data))}

	// M4A uses MP4 container - look for metadata atoms
	searchData := string(data[:min(len(data), 10000)])

	if containsAIAudioMarker(searchData) {
		meta.IsAIMarked = true
	}

	meta.EncoderName = extractAudioEncoder(searchData)

	return meta, stats
}

// analyzeMetadata scores based on metadata presence.
func (a *AudioAnalyzer) analyzeMetadata(meta AudioMetadata) float64 {
	score := 0.5

	// AI markers are strong signal
	if meta.IsAIMarked {
		score += 0.35
	}

	// Recording markers suggest real audio
	if meta.HasRecording {
		score -= 0.2
	}

	// ID3 tags suggest real music file
	if meta.HasID3 {
		score -= 0.1
	}

	// Known AI audio tools
	aiTools := []string{
		"elevenlabs", "eleven labs", "murf", "play.ht",
		"resemble", "descript", "synthesia", "wellsaid",
		"amazon polly", "google tts", "azure speech",
		"suno", "udio", "musicgen", "riffusion",
	}

	encoderLower := bytes.ToLower([]byte(meta.EncoderName))
	for _, tool := range aiTools {
		if bytes.Contains(encoderLower, []byte(tool)) {
			score += 0.3
			break
		}
	}

	return math.Max(0, math.Min(1, score))
}

// analyzeFormat checks format-specific indicators.
func (a *AudioAnalyzer) analyzeFormat(meta AudioMetadata, data []byte) float64 {
	score := 0.5

	// Standard sample rates suggest real audio
	standardRates := map[int]bool{44100: true, 48000: true, 96000: true, 22050: true, 16000: true}
	if meta.SampleRate > 0 && !standardRates[meta.SampleRate] {
		score += 0.1 // Unusual sample rate
	}

	// Very high quality might indicate AI (sometimes over-engineered)
	if meta.SampleRate >= 96000 && meta.BitDepth >= 24 {
		score += 0.1
	}

	// Mono audio is more common in AI voice
	if meta.Channels == 1 {
		score += 0.1
	}

	return score
}

// analyzePatterns looks for unusual patterns in audio data.
func (a *AudioAnalyzer) analyzePatterns(data []byte, format string) float64 {
	if len(data) < 5000 {
		return 0.5
	}

	// Sample different regions
	regionSize := len(data) / 5
	entropies := make([]float64, 0)

	for i := 1; i < 5; i++ {
		start := i * regionSize
		end := start + min(2000, len(data)-start)
		if end > start {
			entropy := calculateEntropy(data[start:end])
			entropies = append(entropies, entropy)
		}
	}

	if len(entropies) < 2 {
		return 0.5
	}

	// Calculate variance
	mean := 0.0
	for _, e := range entropies {
		mean += e
	}
	mean /= float64(len(entropies))

	variance := 0.0
	for _, e := range entropies {
		diff := e - mean
		variance += diff * diff
	}
	variance /= float64(len(entropies))

	// High variance in entropy might indicate AI
	if variance > 0.05 {
		return 0.6
	}

	return 0.4
}

// analyzeQuality evaluates audio quality indicators.
func (a *AudioAnalyzer) analyzeQuality(meta AudioMetadata, stats AudioStats) float64 {
	score := 0.5

	// Very low bitrate might indicate AI optimization
	if meta.Bitrate > 0 && meta.Bitrate < 64 {
		score += 0.1
	}

	// Unusually high bitrate
	if meta.Bitrate > 320 {
		score += 0.1
	}

	// File size sanity check
	if stats.FileSize > 0 && stats.FileSize < 10000 {
		score += 0.2 // Very small file is suspicious
	}

	return score
}

// detectAISignatures looks for known AI audio signatures.
func (a *AudioAnalyzer) detectAISignatures(data []byte, meta AudioMetadata) float64 {
	score := 0.0

	// Check for AI tool watermarks
	searchData := data[:min(len(data), 50000)]

	aiMarkers := []string{
		"elevenlabs", "murf.ai", "play.ht", "resemble.ai",
		"suno", "udio", "generated", "synthetic", "ai voice",
		"text-to-speech", "tts", "voice clone",
	}

	for _, marker := range aiMarkers {
		if bytes.Contains(bytes.ToLower(searchData), []byte(marker)) {
			score += 0.3
			break
		}
	}

	// Check metadata encoder
	if meta.IsAIMarked {
		score += 0.4
	}

	// Certain exact durations are suspicious (AI often generates exact lengths)
	// This would need actual duration parsing

	return math.Min(1, score)
}

// analyzeNoiseProfile checks for natural noise characteristics.
func (a *AudioAnalyzer) analyzeNoiseProfile(data []byte, format string) float64 {
	if len(data) < 10000 {
		return 0.5
	}

	// Look at byte distribution as proxy for audio characteristics
	// Real recordings have natural noise; AI audio is often "too clean"

	// Sample from middle of file (skip headers)
	start := len(data) / 3
	end := start + min(5000, len(data)-start)
	sample := data[start:end]

	// Check for repeated patterns (AI sometimes has artifacts)
	repeatCount := 0
	windowSize := 100

	for i := 0; i < len(sample)-windowSize*2; i += windowSize {
		window1 := sample[i : i+windowSize]
		window2 := sample[i+windowSize : i+windowSize*2]

		if byteSimilarity(window1, window2) > 0.9 {
			repeatCount++
		}
	}

	if repeatCount > 5 {
		return 0.7 // Suspicious repetition
	}

	return 0.4
}

// calculateWeightedScore combines signals into final score.
func (a *AudioAnalyzer) calculateWeightedScore(signals AudioSignals) float64 {
	w := a.weights

	score := signals.MetadataScore*w.MetadataScore +
		signals.FormatAnalysis*w.FormatAnalysis +
		signals.PatternAnalysis*w.PatternAnalysis +
		signals.QualityIndicators*w.QualityIndicators +
		signals.AISignatures*w.AISignatures +
		signals.NoiseProfile*w.NoiseProfile

	totalWeight := w.MetadataScore + w.FormatAnalysis + w.PatternAnalysis +
		w.QualityIndicators + w.AISignatures + w.NoiseProfile

	if totalWeight > 0 {
		score /= totalWeight
	}

	return math.Max(0, math.Min(1, score))
}

// =============================================================================
// Helper Functions
// =============================================================================

// containsAIAudioMarker checks for AI audio generator markers.
func containsAIAudioMarker(s string) bool {
	markers := []string{
		"elevenlabs", "eleven labs", "murf", "play.ht",
		"resemble", "descript", "synthesia", "wellsaid",
		"suno", "udio", "musicgen", "riffusion",
		"ai generated", "ai-generated", "synthetic voice",
		"text to speech", "text-to-speech", "tts",
		"voice clone", "cloned voice",
	}

	lower := bytes.ToLower([]byte(s))
	for _, marker := range markers {
		if bytes.Contains(lower, []byte(marker)) {
			return true
		}
	}
	return false
}

// containsRecordingMarker checks for real recording indicators.
func containsRecordingMarker(s string) bool {
	markers := []string{
		"recorded", "recording", "studio", "microphone",
		"live", "concert", "session", "interview",
		"iphone", "android", "voice memo",
	}

	lower := bytes.ToLower([]byte(s))
	for _, marker := range markers {
		if bytes.Contains(lower, []byte(marker)) {
			return true
		}
	}
	return false
}

// extractAudioEncoder tries to find encoder name.
func extractAudioEncoder(s string) string {
	encoders := map[string]string{
		"lame":        "LAME",
		"ffmpeg":      "ffmpeg",
		"audacity":    "Audacity",
		"adobe":       "Adobe Audition",
		"logic":       "Logic Pro",
		"pro tools":   "Pro Tools",
		"ableton":     "Ableton Live",
		"fl studio":   "FL Studio",
		"elevenlabs":  "ElevenLabs",
		"suno":        "Suno AI",
		"udio":        "Udio",
	}

	lower := bytes.ToLower([]byte(s))
	for key, name := range encoders {
		if bytes.Contains(lower, []byte(key)) {
			return name
		}
	}

	return ""
}
