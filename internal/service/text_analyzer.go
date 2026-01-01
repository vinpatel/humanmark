package service

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// =============================================================================
// HumanMark Text Detection Algorithm
// =============================================================================
//
// This is our own detection algorithm based on statistical analysis of text.
// No external APIs required.
//
// Key insight: AI-generated text is statistically "average" - it optimizes for
// the most likely next token, resulting in predictable patterns.
//
// Human text is messy, variable, and personal.
//
// We measure several signals:
//   1. Sentence length variance (humans vary more)
//   2. Vocabulary richness (humans use rare words, slang)
//   3. Burstiness (humans cluster related words)
//   4. Punctuation patterns (humans use more variety)
//   5. AI phrase detection (common AI patterns)
//   6. Perplexity proxy (word predictability)
//
// =============================================================================

// TextAnalyzer performs statistical analysis on text to detect AI generation.
type TextAnalyzer struct {
	// Weights for each signal (tuned based on testing)
	weights TextAnalyzerWeights
}

// TextAnalyzerWeights controls the importance of each signal.
type TextAnalyzerWeights struct {
	SentenceVariance   float64
	VocabularyRichness float64
	Burstiness         float64
	PunctuationVariety float64
	AIPhraseDetection  float64
	WordLengthVariance float64
	ContractionsUsage  float64
	RepetitionPenalty  float64
}

// DefaultWeights returns tuned weights for the analyzer.
func DefaultWeights() TextAnalyzerWeights {
	return TextAnalyzerWeights{
		SentenceVariance:   0.15,
		VocabularyRichness: 0.20,
		Burstiness:         0.10,
		PunctuationVariety: 0.10,
		AIPhraseDetection:  0.20,
		WordLengthVariance: 0.05,
		ContractionsUsage:  0.10,
		RepetitionPenalty:  0.10,
	}
}

// NewTextAnalyzer creates a new analyzer with default weights.
func NewTextAnalyzer() *TextAnalyzer {
	return &TextAnalyzer{
		weights: DefaultWeights(),
	}
}

// TextAnalysisResult contains detailed analysis results.
type TextAnalysisResult struct {
	// Final AI probability (0.0 = human, 1.0 = AI)
	AIScore float64

	// Individual signal scores (0.0 = human-like, 1.0 = AI-like)
	Signals TextSignals

	// Detected AI phrases
	DetectedAIPhrases []string

	// Statistics
	Stats TextStats
}

// TextSignals contains individual signal scores.
type TextSignals struct {
	SentenceVariance   float64 // Low variance = AI-like
	VocabularyRichness float64 // Low richness = AI-like
	Burstiness         float64 // Low burstiness = AI-like
	PunctuationVariety float64 // Low variety = AI-like
	AIPhraseScore      float64 // High = AI-like
	WordLengthVariance float64 // Low variance = AI-like
	ContractionsUsage  float64 // Low usage = AI-like
	RepetitionScore    float64 // High repetition = AI-like
}

// TextStats contains raw statistics about the text.
type TextStats struct {
	CharCount        int
	WordCount        int
	SentenceCount    int
	AvgSentenceLen   float64
	AvgWordLen       float64
	UniqueWords      int
	UniqueRatio      float64
	PunctuationCount int
}

// Analyze performs comprehensive text analysis.
func (a *TextAnalyzer) Analyze(text string) TextAnalysisResult {
	result := TextAnalysisResult{}

	// Calculate basic stats
	result.Stats = a.calculateStats(text)

	// Calculate individual signals
	result.Signals.SentenceVariance = a.analyzeSentenceVariance(text)
	result.Signals.VocabularyRichness = a.analyzeVocabularyRichness(text)
	result.Signals.Burstiness = a.analyzeBurstiness(text)
	result.Signals.PunctuationVariety = a.analyzePunctuationVariety(text)
	result.Signals.AIPhraseScore, result.DetectedAIPhrases = a.detectAIPhrases(text)
	result.Signals.WordLengthVariance = a.analyzeWordLengthVariance(text)
	result.Signals.ContractionsUsage = a.analyzeContractions(text)
	result.Signals.RepetitionScore = a.analyzeRepetition(text)

	// Calculate weighted AI score
	result.AIScore = a.calculateWeightedScore(result.Signals)

	return result
}

// calculateStats computes basic text statistics.
func (a *TextAnalyzer) calculateStats(text string) TextStats {
	stats := TextStats{}

	stats.CharCount = len(text)

	// Count words
	words := tokenize(text)
	stats.WordCount = len(words)

	// Count sentences
	sentences := splitSentences(text)
	stats.SentenceCount = len(sentences)

	// Average sentence length
	if stats.SentenceCount > 0 {
		totalWords := 0
		for _, s := range sentences {
			totalWords += len(tokenize(s))
		}
		stats.AvgSentenceLen = float64(totalWords) / float64(stats.SentenceCount)
	}

	// Average word length
	if stats.WordCount > 0 {
		totalChars := 0
		for _, w := range words {
			totalChars += len(w)
		}
		stats.AvgWordLen = float64(totalChars) / float64(stats.WordCount)
	}

	// Unique words
	unique := make(map[string]bool)
	for _, w := range words {
		unique[strings.ToLower(w)] = true
	}
	stats.UniqueWords = len(unique)

	if stats.WordCount > 0 {
		stats.UniqueRatio = float64(stats.UniqueWords) / float64(stats.WordCount)
	}

	// Punctuation count
	for _, r := range text {
		if unicode.IsPunct(r) {
			stats.PunctuationCount++
		}
	}

	return stats
}

// analyzeSentenceVariance measures variance in sentence lengths.
// Humans write with varied sentence lengths; AI tends to be uniform.
func (a *TextAnalyzer) analyzeSentenceVariance(text string) float64 {
	sentences := splitSentences(text)
	if len(sentences) < 3 {
		return 0.5 // Not enough data
	}

	// Calculate sentence lengths
	lengths := make([]float64, len(sentences))
	sum := 0.0
	for i, s := range sentences {
		words := tokenize(s)
		lengths[i] = float64(len(words))
		sum += lengths[i]
	}

	// Calculate mean
	mean := sum / float64(len(lengths))

	// Calculate variance
	variance := 0.0
	for _, l := range lengths {
		variance += (l - mean) * (l - mean)
	}
	variance /= float64(len(lengths))

	// Standard deviation
	stdDev := math.Sqrt(variance)

	// Coefficient of variation (normalized)
	cv := 0.0
	if mean > 0 {
		cv = stdDev / mean
	}

	// Human text typically has CV > 0.5
	// AI text typically has CV < 0.3
	// Convert to AI score (low variance = high AI score)
	aiScore := 1.0 - math.Min(cv/0.8, 1.0)

	return aiScore
}

// analyzeVocabularyRichness measures lexical diversity.
// Humans use more varied vocabulary; AI uses "safe" common words.
func (a *TextAnalyzer) analyzeVocabularyRichness(text string) float64 {
	words := tokenize(text)
	if len(words) < 10 {
		return 0.5 // Not enough data
	}

	// Type-Token Ratio (TTR)
	unique := make(map[string]bool)
	for _, w := range words {
		unique[strings.ToLower(w)] = true
	}
	ttr := float64(len(unique)) / float64(len(words))

	// Check for rare/unusual words (not in common vocabulary)
	uncommonCount := 0
	for w := range unique {
		if !isCommonWord(w) && len(w) > 3 {
			uncommonCount++
		}
	}
	uncommonRatio := float64(uncommonCount) / float64(len(unique))

	// Human text: higher TTR, more uncommon words
	// Normalize TTR (human typically > 0.5, AI typically < 0.4)
	ttrScore := 1.0 - math.Min(ttr/0.6, 1.0)

	// Combine scores
	aiScore := (ttrScore*0.6 + (1.0-uncommonRatio)*0.4)

	return math.Max(0, math.Min(1, aiScore))
}

// analyzeBurstiness measures topic word clustering.
// Humans tend to cluster related words; AI distributes them evenly.
func (a *TextAnalyzer) analyzeBurstiness(text string) float64 {
	words := tokenize(text)
	if len(words) < 20 {
		return 0.5
	}

	// Find repeated content words
	wordPositions := make(map[string][]int)
	for i, w := range words {
		w = strings.ToLower(w)
		if len(w) > 4 && !isCommonWord(w) {
			wordPositions[w] = append(wordPositions[w], i)
		}
	}

	// Calculate burstiness for words that appear multiple times
	totalBurstiness := 0.0
	count := 0

	for _, positions := range wordPositions {
		if len(positions) < 2 {
			continue
		}

		// Calculate gaps between occurrences
		gaps := make([]float64, len(positions)-1)
		for i := 1; i < len(positions); i++ {
			gaps[i-1] = float64(positions[i] - positions[i-1])
		}

		// Calculate variance of gaps
		mean := 0.0
		for _, g := range gaps {
			mean += g
		}
		mean /= float64(len(gaps))

		variance := 0.0
		for _, g := range gaps {
			variance += (g - mean) * (g - mean)
		}
		variance /= float64(len(gaps))

		// Burstiness: high variance in gaps = bursty (human-like)
		if mean > 0 {
			totalBurstiness += math.Sqrt(variance) / mean
		}
		count++
	}

	if count == 0 {
		return 0.5
	}

	avgBurstiness := totalBurstiness / float64(count)

	// Low burstiness = AI-like
	aiScore := 1.0 - math.Min(avgBurstiness/1.5, 1.0)

	return aiScore
}

// analyzePunctuationVariety measures punctuation diversity.
// Humans use varied punctuation; AI tends to stick to periods and commas.
func (a *TextAnalyzer) analyzePunctuationVariety(text string) float64 {
	punctCounts := make(map[rune]int)
	totalPunct := 0

	for _, r := range text {
		if unicode.IsPunct(r) {
			punctCounts[r]++
			totalPunct++
		}
	}

	if totalPunct < 5 {
		return 0.5
	}

	// Count unique punctuation types
	uniquePunct := len(punctCounts)

	// Check for varied punctuation (!, ?, ;, :, -, etc.)
	variedPunct := 0
	interestingPunct := []rune{'!', '?', ';', ':', '-', '—', '(', ')', '"', '\''}
	for _, p := range interestingPunct {
		if punctCounts[p] > 0 {
			variedPunct++
		}
	}

	// Human text typically has 5+ different punctuation marks
	// AI often uses only . and ,
	varietyScore := float64(uniquePunct) / 8.0
	interestingScore := float64(variedPunct) / 5.0

	// Low variety = AI-like
	aiScore := 1.0 - (varietyScore*0.5 + interestingScore*0.5)

	return math.Max(0, math.Min(1, aiScore))
}

// detectAIPhrases looks for common AI writing patterns.
func (a *TextAnalyzer) detectAIPhrases(text string) (float64, []string) {
	lowerText := strings.ToLower(text)
	detected := []string{}

	// Common AI phrases and patterns
	aiPhrases := []struct {
		pattern string
		weight  float64
	}{
		// Direct AI references
		{"as an ai", 1.0},
		{"as a language model", 1.0},
		{"i don't have personal", 0.9},
		{"i cannot provide", 0.8},
		{"i'm unable to", 0.7},

		// Hedging phrases
		{"it's important to note", 0.8},
		{"it is important to", 0.7},
		{"it's worth noting", 0.7},
		{"it should be noted", 0.7},
		{"keep in mind that", 0.6},

		// Transition phrases (overused by AI)
		{"furthermore", 0.4},
		{"moreover", 0.4},
		{"additionally", 0.4},
		{"in conclusion", 0.5},
		{"to summarize", 0.5},
		{"in summary", 0.5},
		{"overall", 0.3},

		// Generic helpful phrases
		{"i hope this helps", 0.7},
		{"feel free to", 0.5},
		{"don't hesitate to", 0.5},
		{"let me know if", 0.4},

		// Formal/stilted phrasing
		{"utilize", 0.3},
		{"facilitate", 0.3},
		{"leverage", 0.3},
		{"delve into", 0.6},
		{"dive into", 0.4},
		{"explore the", 0.3},

		// List introductions
		{"here are some", 0.5},
		{"here's a list", 0.5},
		{"the following", 0.4},
	}

	totalWeight := 0.0
	matchCount := 0

	for _, phrase := range aiPhrases {
		if strings.Contains(lowerText, phrase.pattern) {
			detected = append(detected, phrase.pattern)
			totalWeight += phrase.weight
			matchCount++
		}
	}

	// Calculate AI score based on matches
	// More matches = higher AI probability
	if matchCount == 0 {
		return 0.0, detected
	}

	// Normalize by text length (longer text might naturally have more matches)
	wordCount := len(tokenize(text))
	normalizedScore := totalWeight / (float64(wordCount) / 100.0)

	aiScore := math.Min(normalizedScore, 1.0)

	return aiScore, detected
}

// analyzeWordLengthVariance measures variance in word lengths.
func (a *TextAnalyzer) analyzeWordLengthVariance(text string) float64 {
	words := tokenize(text)
	if len(words) < 10 {
		return 0.5
	}

	// Calculate word lengths
	sum := 0.0
	for _, w := range words {
		sum += float64(len(w))
	}
	mean := sum / float64(len(words))

	// Calculate variance
	variance := 0.0
	for _, w := range words {
		diff := float64(len(w)) - mean
		variance += diff * diff
	}
	variance /= float64(len(words))
	stdDev := math.Sqrt(variance)

	// Human text has more varied word lengths
	// Coefficient of variation
	cv := 0.0
	if mean > 0 {
		cv = stdDev / mean
	}

	// Low variance = AI-like
	aiScore := 1.0 - math.Min(cv/0.6, 1.0)

	return aiScore
}

// analyzeContractions checks for contraction usage.
// Humans use contractions; formal AI often doesn't.
func (a *TextAnalyzer) analyzeContractions(text string) float64 {
	contractions := []string{
		"i'm", "i'll", "i've", "i'd",
		"you're", "you'll", "you've", "you'd",
		"he's", "she's", "it's", "we're", "they're",
		"don't", "doesn't", "didn't", "won't", "wouldn't",
		"can't", "couldn't", "shouldn't", "isn't", "aren't",
		"wasn't", "weren't", "haven't", "hasn't", "hadn't",
		"let's", "that's", "there's", "here's", "what's",
		"who's", "how's", "where's", "when's",
	}

	lowerText := strings.ToLower(text)
	wordCount := len(tokenize(text))

	if wordCount < 20 {
		return 0.5
	}

	contractionCount := 0
	for _, c := range contractions {
		contractionCount += strings.Count(lowerText, c)
	}

	// Contractions per 100 words
	contractionRate := float64(contractionCount) / (float64(wordCount) / 100.0)

	// Human casual text: 2-5 contractions per 100 words
	// Formal AI: often 0-1 contractions per 100 words
	// Convert to AI score (low contractions = AI-like)
	aiScore := 1.0 - math.Min(contractionRate/3.0, 1.0)

	return aiScore
}

// analyzeRepetition checks for repetitive patterns.
// AI sometimes repeats phrases or structures.
func (a *TextAnalyzer) analyzeRepetition(text string) float64 {
	sentences := splitSentences(text)
	if len(sentences) < 3 {
		return 0.5
	}

	// Check for repeated sentence starts
	starts := make(map[string]int)
	for _, s := range sentences {
		words := tokenize(s)
		if len(words) >= 2 {
			start := strings.ToLower(words[0] + " " + words[1])
			starts[start]++
		}
	}

	// Count repetitions
	repetitions := 0
	for _, count := range starts {
		if count > 1 {
			repetitions += count - 1
		}
	}

	// Repetition rate
	repRate := float64(repetitions) / float64(len(sentences))

	// High repetition = AI-like
	aiScore := math.Min(repRate*2, 1.0)

	return aiScore
}

// calculateWeightedScore combines all signals into final AI score.
func (a *TextAnalyzer) calculateWeightedScore(signals TextSignals) float64 {
	w := a.weights

	score := signals.SentenceVariance*w.SentenceVariance +
		signals.VocabularyRichness*w.VocabularyRichness +
		signals.Burstiness*w.Burstiness +
		signals.PunctuationVariety*w.PunctuationVariety +
		signals.AIPhraseScore*w.AIPhraseDetection +
		signals.WordLengthVariance*w.WordLengthVariance +
		signals.ContractionsUsage*w.ContractionsUsage +
		signals.RepetitionScore*w.RepetitionPenalty

	// Normalize to 0-1
	totalWeight := w.SentenceVariance + w.VocabularyRichness + w.Burstiness +
		w.PunctuationVariety + w.AIPhraseDetection + w.WordLengthVariance +
		w.ContractionsUsage + w.RepetitionPenalty

	if totalWeight > 0 {
		score /= totalWeight
	}

	return math.Max(0, math.Min(1, score))
}

// =============================================================================
// Helper Functions
// =============================================================================

// tokenize splits text into words.
func tokenize(text string) []string {
	// Simple tokenization: split on whitespace and punctuation
	re := regexp.MustCompile(`[a-zA-Z']+`)
	return re.FindAllString(text, -1)
}

// splitSentences splits text into sentences.
func splitSentences(text string) []string {
	// Split on sentence-ending punctuation
	re := regexp.MustCompile(`[.!?]+\s+`)
	parts := re.Split(text, -1)

	// Filter empty strings
	sentences := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 0 {
			sentences = append(sentences, p)
		}
	}

	return sentences
}

// isCommonWord checks if a word is in the common vocabulary.
// Common words are less indicative of human/AI authorship.
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		// Articles
		"a": true, "an": true, "the": true,
		// Pronouns
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "me": true, "him": true, "her": true,
		"us": true, "them": true, "my": true, "your": true, "his": true,
		"its": true, "our": true, "their": true, "this": true, "that": true,
		// Prepositions
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true, "about": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		// Conjunctions
		"and": true, "or": true, "but": true, "so": true, "yet": true,
		"if": true, "when": true, "while": true, "because": true, "although": true,
		// Verbs
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "get": true, "got": true, "make": true, "made": true,
		// Common words
		"not": true, "no": true, "yes": true, "just": true, "only": true,
		"also": true, "very": true, "more": true, "most": true, "some": true,
		"any": true, "all": true, "many": true, "much": true, "other": true,
		"such": true, "than": true, "then": true, "now": true, "here": true,
		"there": true, "where": true, "what": true, "which": true, "who": true,
		"how": true, "why": true, "each": true, "every": true, "both": true,
		"few": true, "new": true, "old": true, "good": true, "bad": true,
		"first": true, "last": true, "long": true, "little": true, "own": true,
		"same": true, "big": true, "high": true, "small": true, "large": true,
		"next": true, "early": true, "young": true, "important": true, "public": true,
		"able": true, "man": true, "woman": true, "time": true, "year": true,
		"people": true, "way": true, "day": true, "thing": true, "world": true,
		"life": true, "hand": true, "part": true, "place": true, "case": true,
		"week": true, "work": true, "fact": true, "group": true, "number": true,
		"night": true, "point": true, "home": true, "water": true, "room": true,
		"mother": true, "area": true, "money": true, "story": true, "month": true,
		"lot": true, "right": true, "study": true, "book": true, "eye": true,
		"job": true, "word": true, "business": true, "issue": true, "side": true,
		"kind": true, "head": true, "house": true, "service": true, "friend": true,
		"father": true, "power": true, "hour": true, "game": true, "line": true,
		"end": true, "member": true, "law": true, "car": true, "city": true,
		"community": true, "name": true,
	}

	return commonWords[strings.ToLower(word)]
}
