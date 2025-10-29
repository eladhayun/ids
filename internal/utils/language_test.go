package utils

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Hebrew text",
			input:    "שלום, איך אני יכול לעזור לך?",
			expected: "he",
		},
		{
			name:     "English text",
			input:    "Hello, how can I help you?",
			expected: "en",
		},
		{
			name:     "Arabic text",
			input:    "مرحبا، كيف يمكنني مساعدتك؟",
			expected: "ar",
		},
		{
			name:     "Russian text",
			input:    "Привет, как я могу помочь?",
			expected: "ru",
		},
		{
			name:     "Chinese text",
			input:    "你好，我能怎么帮助你？",
			expected: "zh",
		},
		{
			name:     "Japanese text",
			input:    "こんにちは、どのようにお手伝いできますか？",
			expected: "ja",
		},
		{
			name:     "Korean text",
			input:    "안녕하세요, 어떻게 도와드릴까요?",
			expected: "ko",
		},
		{
			name:     "Empty text",
			input:    "",
			expected: "en",
		},
		{
			name:     "Mixed text with Hebrew",
			input:    "Hello שלום world",
			expected: "he",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectLanguage(tt.input)
			if result.Code != tt.expected {
				t.Errorf("DetectLanguage(%q) = %v, expected %s", tt.input, result.Code, tt.expected)
			}
		})
	}
}

func TestGetLanguageInstruction(t *testing.T) {
	tests := []struct {
		lang     Language
		expected string
	}{
		{
			lang:     Language{Code: "he", Name: "Hebrew"},
			expected: "Please respond in Hebrew (עברית).",
		},
		{
			lang:     Language{Code: "en", Name: "English"},
			expected: "Please respond in English.",
		},
		{
			lang:     Language{Code: "ar", Name: "Arabic"},
			expected: "Please respond in Arabic (العربية).",
		},
		{
			lang:     Language{Code: "unknown", Name: "Unknown"},
			expected: "Please respond in English.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang.Code, func(t *testing.T) {
			result := GetLanguageInstruction(tt.lang)
			if result != tt.expected {
				t.Errorf("GetLanguageInstruction(%v) = %q, expected %q", tt.lang, result, tt.expected)
			}
		})
	}
}
