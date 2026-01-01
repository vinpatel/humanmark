package service

import (
	"testing"
)

// TestTextAnalyzer tests the HumanMark text analysis algorithm.
func TestTextAnalyzer(t *testing.T) {
	analyzer := NewTextAnalyzer()

	t.Run("detects clearly AI-generated text", func(t *testing.T) {
		// Text with many AI patterns
		aiText := `As an AI language model, I cannot provide personal opinions. However, it's important to note that this topic has many facets. Furthermore, we should consider multiple perspectives. In conclusion, I hope this helps you understand the subject better. Feel free to ask if you have any more questions.`

		result := analyzer.Analyze(aiText)

		if result.AIScore < 0.6 {
			t.Errorf("expected high AI score for AI-like text, got %f", result.AIScore)
		}

		if len(result.DetectedAIPhrases) == 0 {
			t.Error("expected AI phrases to be detected")
		}

		t.Logf("AI Text Analysis: score=%f, phrases=%v", result.AIScore, result.DetectedAIPhrases)
	})

	t.Run("detects human-written casual text", func(t *testing.T) {
		// Natural human text with contractions, varied sentences, personality
		humanText := `You know what? I've been thinking about this for a while now. It's weird - sometimes the simplest things are the hardest to explain! 
		
		Like yesterday, I tried explaining why the sky looks blue to my kid. She just stared at me. Didn't get it at all, haha. Kids, man.
		
		Anyway, what I'm trying to say is... don't overthink it. Just go with your gut. That's what I'd do.`

		result := analyzer.Analyze(humanText)

		if result.AIScore > 0.5 {
			t.Errorf("expected low AI score for human text, got %f", result.AIScore)
		}

		// Should have contractions detected
		if result.Signals.ContractionsUsage > 0.5 {
			t.Logf("Contractions score: %f (lower = more contractions = human-like)", result.Signals.ContractionsUsage)
		}

		t.Logf("Human Text Analysis: score=%f, signals=%+v", result.AIScore, result.Signals)
	})

	t.Run("handles short text", func(t *testing.T) {
		shortText := "Hello world"
		result := analyzer.Analyze(shortText)

		// Should return neutral score for insufficient data
		if result.AIScore < 0.3 || result.AIScore > 0.7 {
			t.Errorf("expected neutral score for short text, got %f", result.AIScore)
		}

		t.Logf("Short Text Analysis: score=%f", result.AIScore)
	})

	t.Run("analyzes sentence variance correctly", func(t *testing.T) {
		// Varied sentence lengths (human-like)
		variedText := "Short one. This is a medium length sentence with more words. And here's quite a long sentence that goes on for a bit longer than the others, adding some variety to the text."

		// Uniform sentence lengths (AI-like)
		uniformText := "This sentence has exactly ten words in it. This sentence has exactly ten words in it. This sentence has exactly ten words in it."

		variedResult := analyzer.Analyze(variedText)
		uniformResult := analyzer.Analyze(uniformText)

		// Varied text should have lower AI score for sentence variance
		if variedResult.Signals.SentenceVariance > uniformResult.Signals.SentenceVariance {
			t.Errorf("varied text should have lower variance AI score: varied=%f, uniform=%f",
				variedResult.Signals.SentenceVariance, uniformResult.Signals.SentenceVariance)
		}

		t.Logf("Sentence Variance - Varied: %f, Uniform: %f",
			variedResult.Signals.SentenceVariance, uniformResult.Signals.SentenceVariance)
	})

	t.Run("detects AI phrases correctly", func(t *testing.T) {
		phrases := []struct {
			text          string
			expectAI      bool
			minPhrases    int
		}{
			{"As an AI, I cannot provide that.", true, 1},
			{"It's important to note that furthermore, moreover.", true, 2},
			{"Just my two cents on the matter.", false, 0},
			{"I hope this helps! Let me know if you have questions.", true, 1},
		}

		for _, tc := range phrases {
			result := analyzer.Analyze(tc.text)

			if tc.expectAI && result.Signals.AIPhraseScore < 0.3 {
				t.Errorf("expected AI phrases in '%s', got score %f", tc.text, result.Signals.AIPhraseScore)
			}

			if !tc.expectAI && result.Signals.AIPhraseScore > 0.5 {
				t.Errorf("unexpected AI phrases in '%s', got score %f", tc.text, result.Signals.AIPhraseScore)
			}

			if len(result.DetectedAIPhrases) < tc.minPhrases {
				t.Errorf("expected at least %d AI phrases in '%s', got %d: %v",
					tc.minPhrases, tc.text, len(result.DetectedAIPhrases), result.DetectedAIPhrases)
			}
		}
	})
}

// TestTextStats verifies basic statistics calculation.
func TestTextStats(t *testing.T) {
	analyzer := NewTextAnalyzer()

	text := "Hello world. This is a test. One two three."
	result := analyzer.Analyze(text)

	if result.Stats.WordCount != 10 {
		t.Errorf("expected 10 words, got %d", result.Stats.WordCount)
	}

	if result.Stats.SentenceCount != 3 {
		t.Errorf("expected 3 sentences, got %d", result.Stats.SentenceCount)
	}

	if result.Stats.UniqueWords != 10 { // All words are unique
		t.Errorf("expected 10 unique words, got %d", result.Stats.UniqueWords)
	}

	t.Logf("Stats: %+v", result.Stats)
}

// TestTokenize tests word tokenization.
func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Hello world", 2},
		{"Hello, world!", 2},
		{"one-two-three", 3},
		{"it's a test", 3}, // it's -> it, s, a, test... actually "it's" as one
		{"", 0},
	}

	for _, tc := range tests {
		tokens := tokenize(tc.input)
		if len(tokens) != tc.expected {
			t.Errorf("tokenize(%q) = %d tokens, want %d: %v", tc.input, len(tokens), tc.expected, tokens)
		}
	}
}

// TestSplitSentences tests sentence splitting.
func TestSplitSentences(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Hello. World.", 2},
		{"Hello! World?", 2},
		{"One sentence", 1},
		{"First. Second! Third?", 3},
		{"", 0},
	}

	for _, tc := range tests {
		sentences := splitSentences(tc.input)
		if len(sentences) != tc.expected {
			t.Errorf("splitSentences(%q) = %d, want %d: %v", tc.input, len(sentences), tc.expected, sentences)
		}
	}
}

// TestIsCommonWord tests common word detection.
func TestIsCommonWord(t *testing.T) {
	common := []string{"the", "a", "is", "are", "and", "but", "it", "for"}
	uncommon := []string{"algorithm", "quantum", "serendipity", "xylophone", "ephemeral"}

	for _, w := range common {
		if !isCommonWord(w) {
			t.Errorf("expected %q to be common", w)
		}
	}

	for _, w := range uncommon {
		if isCommonWord(w) {
			t.Errorf("expected %q to be uncommon", w)
		}
	}
}

// TestVocabularyRichness tests lexical diversity analysis.
func TestVocabularyRichness(t *testing.T) {
	analyzer := NewTextAnalyzer()

	// Rich vocabulary (many unique words)
	richText := "The ephemeral serendipity of discovering quantum phenomena through algorithmic analysis reveals fascinating epistemological implications for our understanding of consciousness."

	// Poor vocabulary (repeated words)
	poorText := "The thing is a thing and the thing does thing things. It is a thing that things do. Things are things."

	richResult := analyzer.Analyze(richText)
	poorResult := analyzer.Analyze(poorText)

	// Rich vocabulary should have lower AI score
	if richResult.Signals.VocabularyRichness > poorResult.Signals.VocabularyRichness {
		t.Errorf("rich vocabulary should have lower AI score: rich=%f, poor=%f",
			richResult.Signals.VocabularyRichness, poorResult.Signals.VocabularyRichness)
	}

	t.Logf("Vocabulary Richness - Rich: %f, Poor: %f",
		richResult.Signals.VocabularyRichness, poorResult.Signals.VocabularyRichness)
}

// TestContractionsAnalysis tests contraction detection.
func TestContractionsAnalysis(t *testing.T) {
	analyzer := NewTextAnalyzer()

	// Text with contractions (human-like)
	contractionText := "I've been thinking, and I don't know if it's worth it. We're not sure what we'll do. They've said it won't work, but I can't believe that."

	// Formal text without contractions (AI-like)
	formalText := "I have been thinking, and I do not know if it is worth it. We are not sure what we will do. They have said it will not work, but I cannot believe that."

	contractionResult := analyzer.Analyze(contractionText)
	formalResult := analyzer.Analyze(formalText)

	// Contractions should have lower AI score
	if contractionResult.Signals.ContractionsUsage > formalResult.Signals.ContractionsUsage {
		t.Errorf("contraction text should have lower AI score: with=%f, without=%f",
			contractionResult.Signals.ContractionsUsage, formalResult.Signals.ContractionsUsage)
	}

	t.Logf("Contractions - With: %f, Without: %f",
		contractionResult.Signals.ContractionsUsage, formalResult.Signals.ContractionsUsage)
}

// BenchmarkTextAnalyzer benchmarks the analysis speed.
func BenchmarkTextAnalyzer(b *testing.B) {
	analyzer := NewTextAnalyzer()

	// Medium-length text
	text := `This is a sample text that will be used for benchmarking the HumanMark text analyzer. 
	It contains multiple sentences with various lengths and structures. Some are short. Others are quite 
	a bit longer and contain more complex vocabulary and sentence structures that require more analysis.
	The goal is to measure how quickly we can analyze text for AI detection.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(text)
	}
}

// TestRealWorldExamples tests with realistic examples.
func TestRealWorldExamples(t *testing.T) {
	analyzer := NewTextAnalyzer()

	// Example 1: ChatGPT-style response
	gptResponse := `Certainly! I'd be happy to help you with that. Here's a comprehensive overview of the topic:

Machine learning is a subset of artificial intelligence that enables computers to learn from data. It's important to note that there are several key approaches:

1. Supervised Learning
2. Unsupervised Learning
3. Reinforcement Learning

In conclusion, machine learning offers tremendous potential for solving complex problems. I hope this helps! Feel free to ask if you have any more questions.`

	// Example 2: Human blog post
	humanBlog := `So I finally tried that new coffee shop everyone's been talking about. Honestly? Kinda overrated.

Don't get me wrong - the lattes are decent. But $8 for a medium?? Come on. My kitchen can do better lol

The vibe was nice tho. Exposed brick, lots of plants, you know the aesthetic. Would I go back? Maybe. If someone else is paying 😂`

	gptResult := analyzer.Analyze(gptResponse)
	humanResult := analyzer.Analyze(humanBlog)

	if gptResult.AIScore < 0.5 {
		t.Errorf("GPT response should have high AI score, got %f", gptResult.AIScore)
	}

	if humanResult.AIScore > 0.5 {
		t.Errorf("Human blog should have low AI score, got %f", humanResult.AIScore)
	}

	t.Logf("GPT Response: AI Score = %f, Phrases = %v", gptResult.AIScore, gptResult.DetectedAIPhrases)
	t.Logf("Human Blog: AI Score = %f, Signals = %+v", humanResult.AIScore, humanResult.Signals)
}
