package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMeaningfulTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple words",
			input:    "Glock 19 holster",
			expected: []string{"glock", "19", "holster"},
		},
		{
			name:     "with punctuation",
			input:    "Right-Hand, OWB!",
			expected: []string{"right", "hand", "owb"},
		},
		{
			name:     "mixed case",
			input:    "Fobus TACTICAL Holster",
			expected: []string{"fobus", "tactical", "holster"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single word",
			input:    "Glock",
			expected: []string{"glock"},
		},
		{
			name:     "numbers only",
			input:    "19 17 43",
			expected: []string{"19", "17", "43"},
		},
		{
			name:     "special characters",
			input:    "M&P Shield 9mm",
			expected: []string{"shield", "9mm"}, // Single-letter words are filtered out unless they're digits
		},
		{
			name:     "multiple spaces",
			input:    "Hello     World",
			expected: []string{"hello", "world"},
		},
		{
			name:     "unicode characters",
			input:    "Hellcat Pro™ Holster®",
			expected: []string{"hellcat", "pro", "holster"},
		},
		{
			name:     "alphanumeric combinations",
			input:    "P365XL Glock19 M&P9",
			expected: []string{"p365xl", "glock19", "p9"}, // Single 'm' is filtered out
		},
		{
			name:     "with stopwords",
			input:    "a the an holster for Glock",
			expected: []string{"holster", "glock"},
		},
		{
			name:     "hyphenated words",
			input:    "right-hand left-hand",
			expected: []string{"right", "hand", "left"}, // Duplicates are removed by deduplication
		},
		{
			name:     "model numbers",
			input:    "Glock-19 Gen5 MOS",
			expected: []string{"glock", "19", "gen5", "mos"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMeaningfulTokens(tt.input)
			assert.ElementsMatch(t, tt.expected, result, "Tokens should match")
		})
	}
}

func TestTokenHasDigit(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{"has digit at end", "glock19", true},
		{"has digit at start", "19glock", true},
		{"has digit in middle", "p365xl", true},
		{"no digits", "glock", false},
		{"only digits", "123", true},
		{"empty string", "", false},
		{"mixed", "m&p9", true},
		{"with spaces", "glock 19", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenHasDigit(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsAllTokens(t *testing.T) {
	tests := []struct {
		name            string
		productTokens   map[string]struct{}
		requiredTokens  []string
		expectedOk      bool
		expectedMissing []string
	}{
		{
			name: "all tokens present",
			productTokens: map[string]struct{}{
				"glock": {},
				"19":    {},
				"owb":   {},
			},
			requiredTokens:  []string{"glock", "19"},
			expectedOk:      true,
			expectedMissing: []string{},
		},
		{
			name: "missing one token",
			productTokens: map[string]struct{}{
				"glock": {},
				"owb":   {},
			},
			requiredTokens:  []string{"glock", "19"},
			expectedOk:      false,
			expectedMissing: []string{"19"},
		},
		{
			name: "missing all tokens",
			productTokens: map[string]struct{}{
				"fobus":  {},
				"paddle": {},
			},
			requiredTokens:  []string{"glock", "19"},
			expectedOk:      false,
			expectedMissing: []string{"glock", "19"},
		},
		{
			name: "empty required tokens",
			productTokens: map[string]struct{}{
				"glock": {},
			},
			requiredTokens:  []string{},
			expectedOk:      true,
			expectedMissing: []string{},
		},
		{
			name:            "empty product tokens",
			productTokens:   map[string]struct{}{},
			requiredTokens:  []string{"glock"},
			expectedOk:      false,
			expectedMissing: []string{"glock"},
		},
		{
			name: "extra product tokens",
			productTokens: map[string]struct{}{
				"glock":   {},
				"19":      {},
				"owb":     {},
				"polymer": {},
				"holster": {},
			},
			requiredTokens:  []string{"glock", "19"},
			expectedOk:      true,
			expectedMissing: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, missing := ContainsAllTokens(tt.productTokens, tt.requiredTokens)
			assert.Equal(t, tt.expectedOk, ok)
			assert.ElementsMatch(t, tt.expectedMissing, missing)
		})
	}
}

func TestBuildTokenSet(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected map[string]struct{}
	}{
		{
			name:   "single value",
			values: []string{"Glock 19 Holster"},
			expected: map[string]struct{}{
				"glock":   {},
				"19":      {},
				"holster": {},
			},
		},
		{
			name:   "multiple values",
			values: []string{"Glock 19", "OWB Holster", "Right Hand"},
			expected: map[string]struct{}{
				"glock":   {},
				"19":      {},
				"owb":     {},
				"holster": {},
				"right":   {},
				"hand":    {},
			},
		},
		{
			name:     "empty values",
			values:   []string{},
			expected: map[string]struct{}{},
		},
		{
			name:   "duplicate tokens",
			values: []string{"Glock 19", "Glock 17", "Glock Holster"},
			expected: map[string]struct{}{
				"glock":   {},
				"19":      {},
				"17":      {},
				"holster": {},
			},
		},
		{
			name:   "with empty strings",
			values: []string{"", "Glock", ""},
			expected: map[string]struct{}{
				"glock": {},
			},
		},
		{
			name:   "special characters",
			values: []string{"M&P Shield", "Sig-Sauer P365"},
			expected: map[string]struct{}{
				"shield": {},
				"sig":    {},
				"sauer":  {},
				"p365":   {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildTokenSet(tt.values...)
			assert.Equal(t, len(tt.expected), len(result))
			for key := range tt.expected {
				_, exists := result[key]
				assert.True(t, exists, "Token '%s' should exist", key)
			}
		})
	}
}

func TestExtractMeaningfulTokens_StopWords(t *testing.T) {
	// Test that common stop words are removed
	stopWords := []string{"a", "an", "the", "and", "or", "for", "with", "to", "of"} // "but" is not in the actual stopwords list

	for _, word := range stopWords {
		t.Run("stopword_"+word, func(t *testing.T) {
			input := word + " glock holster"
			result := ExtractMeaningfulTokens(input)

			// Stop word should not be in result
			for _, token := range result {
				assert.NotEqual(t, word, token, "Stop word '%s' should be filtered out", word)
			}

			// But meaningful words should be present
			assert.Contains(t, result, "glock")
			assert.Contains(t, result, "holster")
		})
	}
}

func TestExtractMeaningfulTokens_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "very long string",
			input:    "This is a very long product description with many words about a Glock 19 holster that should still extract meaningful tokens correctly",
			expected: []string{"very", "long", "product", "description", "many", "words", "about", "glock", "19", "holster", "should", "still", "extract", "meaningful", "tokens", "correctly"}, // "this", "is", "a", "with", "that" are stopwords
		},
		{
			name:     "all stop words",
			input:    "a an the and or for with to of",
			expected: []string{},
		},
		{
			name:     "repeated words",
			input:    "glock glock glock 19 19",
			expected: []string{"glock", "19"},
		},
		{
			name:     "only special characters",
			input:    "!@#$%^&*()",
			expected: []string{},
		},
		{
			name:     "tabs and newlines",
			input:    "Glock\t19\nHolster",
			expected: []string{"glock", "19", "holster"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMeaningfulTokens(tt.input)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestTokenHasDigit_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{"unicode digits", "test①②③", false}, // These are not ASCII digits
		{"negative number", "-123", true},
		{"decimal number", "12.34", true},
		{"scientific notation", "1e10", true},
		{"roman numerals", "XVII", false},
		{"very long with digit", "abcdefghijklmnopqrstuvwxyz123", true},
		{"digit at every position", "1a2b3c4d5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenHasDigit(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkExtractMeaningfulTokens(b *testing.B) {
	input := "Fobus Evolution Paddle Holster for Glock 19/23/32 - Right Hand, Black"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractMeaningfulTokens(input)
	}
}

func BenchmarkBuildTokenSet(b *testing.B) {
	values := []string{
		"Glock 19 Holster",
		"Right Hand OWB",
		"Black Polymer",
		"Tactical Duty Holster",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildTokenSet(values...)
	}
}

func BenchmarkContainsAllTokens(b *testing.B) {
	productTokens := map[string]struct{}{
		"glock":   {},
		"19":      {},
		"holster": {},
		"owb":     {},
		"black":   {},
	}
	requiredTokens := []string{"glock", "19"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ContainsAllTokens(productTokens, requiredTokens)
	}
}
