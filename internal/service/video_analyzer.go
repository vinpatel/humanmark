package service

import (
	"bytes"
	"encoding/binary"
	"math"
)

// =============================================================================
// HumanMark Video Detection Algorithm
// =============================================================================
//
// Video detection is challenging because AI-generated videos have evolved rapidly.
// We use forensic analysis that doesn't require ML models:
//
// Key signals:
//   1. Container/codec metadata analysis
//   2. Frame consistency (extracted from container structure)
//   3. Audio track presence and characteristics
//   4. Temporal patterns in compressed data
//   5. Encoding signatures
//   6. Duration and bitrate anomalies
//
// Limitations:
//   - Full frame-by-frame analysis requires decoding (ffmpeg)
//   - Our analysis works on container metadata and byte patterns
//   - For production accuracy, combine with external APIs
//
// =============================================================================

// VideoAnalyzer performs forensic analysis on video files.
type VideoAnalyzer struct {
	weights VideoAnalyzerWeights
}

// VideoAnalyzerWeights controls signal importance.
type VideoAnalyzerWeights struct {
	MetadataScore      float64
	ContainerAnalysis  float64
	AudioPresence      float64
	TemporalPattern    float64
	EncodingSignature  float64
	BitrateConsistency float64
}

// DefaultVideoWeights returns tuned weights.
func DefaultVideoWeights() VideoAnalyzerWeights {
	return VideoAnalyzerWeights{
		MetadataScore:      0.25,
		ContainerAnalysis:  0.20,
		AudioPresence:      0.15,
		TemporalPattern:    0.15,
		EncodingSignature:  0.15,
		BitrateConsistency: 0.10,
	}
}

// NewVideoAnalyzer creates a new analyzer.
func NewVideoAnalyzer() *VideoAnalyzer {
	return &VideoAnalyzer{
		weights: DefaultVideoWeights(),
	}
}

// VideoAnalysisResult contains analysis results.
type VideoAnalysisResult struct {
	AIScore  float64
	Signals  VideoSignals
	Metadata VideoMetadata
	Stats    VideoStats
}

// VideoSignals contains individual signal scores.
type VideoSignals struct {
	MetadataScore      float64 // Missing/fake metadata = AI-like
	ContainerAnalysis  float64 // Unusual container structure = AI-like
	AudioPresence      float64 // Missing audio = suspicious
	TemporalPattern    float64 // Unusual patterns = AI-like
	EncodingSignature  float64 // Unknown encoder = suspicious
	BitrateConsistency float64 // Unusual bitrate = AI-like
}

// VideoMetadata contains extracted metadata.
type VideoMetadata struct {
	Format       string // mp4, webm, avi, etc.
	HasAudio     bool
	HasVideo     bool
	EncoderName  string
	CreationTime string
	Duration     float64 // seconds (estimated)
	IsAIMarked   bool    // Contains AI generator markers
}

// VideoStats contains video statistics.
type VideoStats struct {
	FileSize       int64
	EstimatedFPS   float64
	Width          int
	Height         int
	VideoBitrate   int // estimated kbps
	AudioBitrate   int // estimated kbps
	KeyframeCount  int
	ChunkCount     int
}

// Analyze performs forensic analysis on video data.
func (a *VideoAnalyzer) Analyze(data []byte) VideoAnalysisResult {
	result := VideoAnalysisResult{}
	result.Stats.FileSize = int64(len(data))

	// Detect format
	format := a.detectVideoFormat(data)
	result.Metadata.Format = format

	// Extract metadata based on format
	switch format {
	case "mp4", "mov":
		result.Metadata, result.Stats = a.analyzeMP4(data)
	case "webm":
		result.Metadata, result.Stats = a.analyzeWebM(data)
	case "avi":
		result.Metadata, result.Stats = a.analyzeAVI(data)
	default:
		result.Metadata.Format = format
	}

	// Calculate signals
	result.Signals.MetadataScore = a.analyzeMetadata(result.Metadata)
	result.Signals.ContainerAnalysis = a.analyzeContainer(data, format)
	result.Signals.AudioPresence = a.analyzeAudioPresence(result.Metadata, data, format)
	result.Signals.TemporalPattern = a.analyzeTemporalPattern(data, format)
	result.Signals.EncodingSignature = a.analyzeEncodingSignature(data, result.Metadata)
	result.Signals.BitrateConsistency = a.analyzeBitrateConsistency(data, result.Stats)

	// Calculate weighted score
	result.AIScore = a.calculateWeightedScore(result.Signals)

	return result
}

// detectVideoFormat identifies video format from magic bytes.
func (a *VideoAnalyzer) detectVideoFormat(data []byte) string {
	if len(data) < 12 {
		return "unknown"
	}

	// MP4/MOV: ftyp atom at offset 4
	if len(data) >= 8 && data[4] == 'f' && data[5] == 't' && data[6] == 'y' && data[7] == 'p' {
		return "mp4"
	}

	// Also check for ftyp at start (some files)
	if len(data) >= 12 {
		if bytes.Equal(data[4:8], []byte("ftyp")) {
			// Check brand
			brand := string(data[8:12])
			switch brand {
			case "isom", "iso2", "mp41", "mp42", "avc1", "M4V ":
				return "mp4"
			case "qt  ":
				return "mov"
			}
			return "mp4" // Default to mp4 for ftyp
		}
	}

	// WebM/MKV: EBML header (1A 45 DF A3)
	if data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		// Check for webm doctype
		if bytes.Contains(data[:min(100, len(data))], []byte("webm")) {
			return "webm"
		}
		return "mkv"
	}

	// AVI: RIFF....AVI
	if len(data) >= 12 {
		if bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("AVI ")) {
			return "avi"
		}
	}

	// FLV: FLV + version
	if len(data) >= 4 && data[0] == 'F' && data[1] == 'L' && data[2] == 'V' {
		return "flv"
	}

	// MPEG-TS: sync byte 0x47 repeating
	if data[0] == 0x47 {
		// Check for more sync bytes at 188-byte intervals
		if len(data) > 376 && data[188] == 0x47 && data[376] == 0x47 {
			return "ts"
		}
	}

	return "unknown"
}

// analyzeMP4 extracts metadata from MP4/MOV containers.
func (a *VideoAnalyzer) analyzeMP4(data []byte) (VideoMetadata, VideoStats) {
	meta := VideoMetadata{Format: "mp4", HasVideo: true}
	stats := VideoStats{FileSize: int64(len(data))}

	// Parse MP4 atoms/boxes
	offset := 0
	for offset < len(data)-8 {
		if offset+8 > len(data) {
			break
		}

		// Atom size (4 bytes) + type (4 bytes)
		atomSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		atomType := string(data[offset+4 : offset+8])

		if atomSize < 8 {
			break // Invalid atom
		}
		if offset+atomSize > len(data) {
			atomSize = len(data) - offset // Truncated file
		}

		switch atomType {
		case "moov":
			// Movie atom - contains metadata
			meta, stats = a.parseMovieAtom(data[offset:offset+atomSize], meta, stats)

		case "mdat":
			// Media data - actual video/audio content
			stats.ChunkCount++

		case "ftyp":
			// File type - check for AI markers
			if atomSize > 8 {
				ftypData := string(data[offset+8 : min(offset+atomSize, offset+100)])
				if containsAIMarker(ftypData) {
					meta.IsAIMarked = true
				}
			}

		case "udta":
			// User data - may contain encoder info
			if atomSize > 8 {
				udtaData := string(data[offset+8 : min(offset+atomSize, offset+500)])
				meta.EncoderName = extractEncoder(udtaData)
				if containsAIMarker(udtaData) {
					meta.IsAIMarked = true
				}
			}
		}

		offset += atomSize
	}

	return meta, stats
}

// parseMovieAtom parses the moov atom for metadata.
func (a *VideoAnalyzer) parseMovieAtom(data []byte, meta VideoMetadata, stats VideoStats) (VideoMetadata, VideoStats) {
	offset := 8 // Skip moov header

	for offset < len(data)-8 {
		if offset+8 > len(data) {
			break
		}

		atomSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		atomType := string(data[offset+4 : offset+8])

		if atomSize < 8 || offset+atomSize > len(data) {
			break
		}

		switch atomType {
		case "mvhd":
			// Movie header - contains duration, timescale
			if atomSize >= 32 {
				// Duration and timescale for calculating length
				// Version 0: timescale at 20, duration at 24
				// Version 1: timescale at 28, duration at 32
			}

		case "trak":
			// Track - check if audio or video
			trackData := data[offset : offset+atomSize]
			if bytes.Contains(trackData, []byte("soun")) {
				meta.HasAudio = true
			}
			if bytes.Contains(trackData, []byte("vide")) {
				meta.HasVideo = true
			}

			// Look for resolution in video track
			if bytes.Contains(trackData, []byte("avc1")) || bytes.Contains(trackData, []byte("hvc1")) {
				// H.264 or H.265 video
			}

		case "meta":
			// Metadata atom
			if atomSize > 8 {
				metaData := string(data[offset+8 : min(offset+atomSize, offset+1000)])
				if containsAIMarker(metaData) {
					meta.IsAIMarked = true
				}
				if enc := extractEncoder(metaData); enc != "" {
					meta.EncoderName = enc
				}
			}
		}

		offset += atomSize
	}

	return meta, stats
}

// analyzeWebM extracts metadata from WebM/MKV containers.
func (a *VideoAnalyzer) analyzeWebM(data []byte) (VideoMetadata, VideoStats) {
	meta := VideoMetadata{Format: "webm", HasVideo: true}
	stats := VideoStats{FileSize: int64(len(data))}

	// Look for common elements in EBML structure
	dataStr := string(data[:min(len(data), 2000)])

	// Check for audio
	if bytes.Contains(data, []byte{0x81}) { // Audio track type
		meta.HasAudio = true
	}

	// Check for encoder
	meta.EncoderName = extractEncoder(dataStr)

	// Check for AI markers
	if containsAIMarker(dataStr) {
		meta.IsAIMarked = true
	}

	return meta, stats
}

// analyzeAVI extracts metadata from AVI containers.
func (a *VideoAnalyzer) analyzeAVI(data []byte) (VideoMetadata, VideoStats) {
	meta := VideoMetadata{Format: "avi", HasVideo: true}
	stats := VideoStats{FileSize: int64(len(data))}

	// AVI uses RIFF structure
	// Look for audio stream
	if bytes.Contains(data, []byte("auds")) {
		meta.HasAudio = true
	}

	// Look for encoder info in headers
	if len(data) > 500 {
		headerData := string(data[:500])
		meta.EncoderName = extractEncoder(headerData)
		if containsAIMarker(headerData) {
			meta.IsAIMarked = true
		}
	}

	return meta, stats
}

// analyzeMetadata scores based on metadata presence and quality.
func (a *VideoAnalyzer) analyzeMetadata(meta VideoMetadata) float64 {
	score := 0.5 // Start neutral

	// AI-generated videos often marked
	if meta.IsAIMarked {
		score += 0.4
	}

	// Real videos usually have audio
	if !meta.HasAudio {
		score += 0.15 // Many AI videos lack audio
	}

	// Known AI video generators
	aiEncoders := []string{
		"runway", "pika", "sora", "gen-2", "gen2",
		"stable video", "stablevideo", "modelscope",
		"deforum", "animatediff", "zeroscope",
	}

	encoderLower := bytes.ToLower([]byte(meta.EncoderName))
	for _, ai := range aiEncoders {
		if bytes.Contains(encoderLower, []byte(ai)) {
			score += 0.3
			break
		}
	}

	// Professional encoders suggest real video
	proEncoders := []string{
		"premiere", "final cut", "davinci", "avid",
		"ffmpeg", "handbrake", "x264", "x265",
	}
	for _, pro := range proEncoders {
		if bytes.Contains(encoderLower, []byte(pro)) {
			score -= 0.1
			break
		}
	}

	return math.Max(0, math.Min(1, score))
}

// analyzeContainer checks container structure for anomalies.
func (a *VideoAnalyzer) analyzeContainer(data []byte, format string) float64 {
	if len(data) < 1000 {
		return 0.5
	}

	score := 0.5

	switch format {
	case "mp4", "mov":
		// Check for proper atom structure
		hasMoviAtom := bytes.Contains(data[:min(len(data), 100000)], []byte("moov"))
		hasMdatAtom := bytes.Contains(data, []byte("mdat"))

		if !hasMoviAtom {
			score += 0.2 // Unusual
		}
		if !hasMdatAtom {
			score += 0.2 // Very unusual
		}

	case "webm":
		// Check for proper EBML structure
		hasSegment := bytes.Contains(data[:min(len(data), 1000)], []byte{0x18, 0x53, 0x80, 0x67})
		if !hasSegment {
			score += 0.2
		}
	}

	return score
}

// analyzeAudioPresence evaluates audio track characteristics.
func (a *VideoAnalyzer) analyzeAudioPresence(meta VideoMetadata, data []byte, format string) float64 {
	// Most real videos have audio
	// AI-generated videos often lack audio or have synthetic audio

	if meta.HasAudio {
		return 0.3 // Good sign
	}

	// No audio is suspicious but not definitive
	// Short clips and specific content may legitimately lack audio
	return 0.7
}

// analyzeTemporalPattern looks for unusual patterns in the video data.
func (a *VideoAnalyzer) analyzeTemporalPattern(data []byte, format string) float64 {
	if len(data) < 10000 {
		return 0.5
	}

	// Sample different regions of the file
	// AI videos sometimes have unusual byte patterns

	regions := 5
	regionSize := len(data) / (regions + 1)
	samples := make([][]byte, regions)

	for i := 0; i < regions; i++ {
		start := (i + 1) * regionSize
		end := start + min(1000, len(data)-start)
		if end > start {
			samples[i] = data[start:end]
		}
	}

	// Check for unusual repetition between regions
	repetitionScore := 0.0
	comparisons := 0

	for i := 0; i < len(samples)-1; i++ {
		for j := i + 1; j < len(samples); j++ {
			if samples[i] != nil && samples[j] != nil {
				sim := byteSimilarity(samples[i], samples[j])
				if sim > 0.5 { // Unusually similar
					repetitionScore += sim
				}
				comparisons++
			}
		}
	}

	if comparisons > 0 {
		avgRepetition := repetitionScore / float64(comparisons)
		if avgRepetition > 0.3 {
			return 0.7 // Suspicious repetition
		}
	}

	return 0.4
}

// analyzeEncodingSignature checks for known encoder signatures.
func (a *VideoAnalyzer) analyzeEncodingSignature(data []byte, meta VideoMetadata) float64 {
	score := 0.5

	// Check for H.264/H.265 encoding (common in both real and AI)
	hasH264 := bytes.Contains(data, []byte("avc1")) || bytes.Contains(data, []byte("h264"))
	hasH265 := bytes.Contains(data, []byte("hvc1")) || bytes.Contains(data, []byte("hevc"))
	hasVP9 := bytes.Contains(data, []byte("vp09"))
	hasAV1 := bytes.Contains(data, []byte("av01"))

	// Standard codecs are neutral
	if hasH264 || hasH265 || hasVP9 || hasAV1 {
		score = 0.45
	}

	// Unknown/missing codec is suspicious
	if !hasH264 && !hasH265 && !hasVP9 && !hasAV1 {
		if meta.Format == "mp4" || meta.Format == "webm" {
			score = 0.6
		}
	}

	return score
}

// analyzeBitrateConsistency estimates bitrate consistency.
func (a *VideoAnalyzer) analyzeBitrateConsistency(data []byte, stats VideoStats) float64 {
	// Very rough estimation without full parsing
	// AI videos sometimes have unusual bitrate characteristics

	if stats.FileSize < 100000 { // Less than 100KB
		return 0.6 // Suspiciously small for video
	}

	// Estimate bits per byte ratio in different regions
	if len(data) < 10000 {
		return 0.5
	}

	// Check entropy in different parts
	entropies := make([]float64, 0)
	chunkSize := len(data) / 5

	for i := 0; i < 5; i++ {
		start := i * chunkSize
		end := start + min(chunkSize, len(data)-start)
		if end > start {
			entropy := calculateEntropy(data[start:end])
			entropies = append(entropies, entropy)
		}
	}

	if len(entropies) < 2 {
		return 0.5
	}

	// Calculate variance of entropy
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

	// High variance in entropy might indicate AI-generated content
	// (inconsistent compression)
	if variance > 0.1 {
		return 0.6
	}

	return 0.4
}

// calculateWeightedScore combines signals into final score.
func (a *VideoAnalyzer) calculateWeightedScore(signals VideoSignals) float64 {
	w := a.weights

	score := signals.MetadataScore*w.MetadataScore +
		signals.ContainerAnalysis*w.ContainerAnalysis +
		signals.AudioPresence*w.AudioPresence +
		signals.TemporalPattern*w.TemporalPattern +
		signals.EncodingSignature*w.EncodingSignature +
		signals.BitrateConsistency*w.BitrateConsistency

	totalWeight := w.MetadataScore + w.ContainerAnalysis + w.AudioPresence +
		w.TemporalPattern + w.EncodingSignature + w.BitrateConsistency

	if totalWeight > 0 {
		score /= totalWeight
	}

	return math.Max(0, math.Min(1, score))
}

// =============================================================================
// Helper Functions
// =============================================================================

// containsAIMarker checks for known AI video generator markers.
func containsAIMarker(s string) bool {
	markers := []string{
		"runway", "pika", "sora", "gen-2", "gen2",
		"stable video", "stablevideo", "modelscope",
		"deforum", "animatediff", "zeroscope",
		"ai generated", "ai-generated", "synthetic",
		"dall-e", "midjourney", // Sometimes in video metadata
	}

	lower := bytes.ToLower([]byte(s))
	for _, marker := range markers {
		if bytes.Contains(lower, []byte(marker)) {
			return true
		}
	}
	return false
}

// extractEncoder tries to find encoder name in metadata string.
func extractEncoder(s string) string {
	// Common encoder identifiers
	encoders := map[string]string{
		"lavf":        "ffmpeg",
		"ffmpeg":      "ffmpeg",
		"handbrake":   "HandBrake",
		"premiere":    "Adobe Premiere",
		"final cut":   "Final Cut Pro",
		"davinci":     "DaVinci Resolve",
		"x264":        "x264",
		"x265":        "x265",
		"runway":      "Runway",
		"pika":        "Pika Labs",
		"sora":        "OpenAI Sora",
		"modelscope":  "ModelScope",
	}

	lower := bytes.ToLower([]byte(s))
	for key, name := range encoders {
		if bytes.Contains(lower, []byte(key)) {
			return name
		}
	}

	return ""
}

// calculateEntropy computes Shannon entropy of byte data.
func calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make([]int, 256)
	for _, b := range data {
		freq[b]++
	}

	entropy := 0.0
	total := float64(len(data))

	for _, f := range freq {
		if f > 0 {
			p := float64(f) / total
			entropy -= p * math.Log2(p)
		}
	}

	// Normalize to 0-1 (max entropy for bytes is 8)
	return entropy / 8.0
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
