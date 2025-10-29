package utils

import (
	"regexp"
	"strings"
)

// Language codes
const (
	LangEnglish  = "en"
	LangHebrew   = "he"
	LangArabic   = "ar"
	LangRussian  = "ru"
	LangChinese  = "zh"
	LangJapanese = "ja"
	LangKorean   = "ko"
)

// Language represents a detected language
type Language struct {
	Code       string
	Name       string
	Confidence float64
}

// ScriptRatio represents the ratio of characters in a specific script
type ScriptRatio struct {
	Code  string
	Name  string
	Ratio float64
}

// DetectLanguage detects the language of the input text
// Returns the most likely language based on character patterns
func DetectLanguage(text string) Language {
	text = strings.TrimSpace(text)
	if text == "" {
		return Language{Code: LangEnglish, Name: "English", Confidence: 0.0}
	}

	// Calculate script ratios for different languages
	ratios := calculateScriptRatios(text)

	// Determine the language with the highest ratio
	return determineLanguageFromRatios(ratios, text)
}

// calculateScriptRatios calculates the ratio of characters for each script
func calculateScriptRatios(text string) []ScriptRatio {
	textRunes := len([]rune(text))

	// Hebrew detection - look for Hebrew characters (U+0590-U+05FF)
	hebrewPattern := regexp.MustCompile(`[\x{0590}-\x{05FF}]`)
	hebrewMatches := hebrewPattern.FindAllString(text, -1)
	hebrewRatio := float64(len(hebrewMatches)) / float64(textRunes)

	// Arabic detection - look for Arabic characters (U+0600-U+06FF)
	arabicPattern := regexp.MustCompile(`[\x{0600}-\x{06FF}]`)
	arabicMatches := arabicPattern.FindAllString(text, -1)
	arabicRatio := float64(len(arabicMatches)) / float64(textRunes)

	// Russian/Cyrillic detection - look for Cyrillic characters (U+0400-U+04FF)
	cyrillicPattern := regexp.MustCompile(`[\x{0400}-\x{04FF}]`)
	cyrillicMatches := cyrillicPattern.FindAllString(text, -1)
	cyrillicRatio := float64(len(cyrillicMatches)) / float64(textRunes)

	// Chinese detection - look for Chinese characters (U+4E00-U+9FFF)
	chinesePattern := regexp.MustCompile(`[\x{4E00}-\x{9FFF}]`)
	chineseMatches := chinesePattern.FindAllString(text, -1)
	chineseRatio := float64(len(chineseMatches)) / float64(textRunes)

	// Japanese detection - look for Hiragana (U+3040-U+309F), Katakana (U+30A0-U+30FF), and Kanji
	japanesePattern := regexp.MustCompile(`[\x{3040}-\x{309F}\x{30A0}-\x{30FF}\x{4E00}-\x{9FFF}]`)
	japaneseMatches := japanesePattern.FindAllString(text, -1)
	japaneseRatio := float64(len(japaneseMatches)) / float64(textRunes)

	// Korean detection - look for Hangul characters (U+AC00-U+D7AF)
	koreanPattern := regexp.MustCompile(`[\x{AC00}-\x{D7AF}]`)
	koreanMatches := koreanPattern.FindAllString(text, -1)
	koreanRatio := float64(len(koreanMatches)) / float64(textRunes)

	return []ScriptRatio{
		{Code: LangHebrew, Name: "Hebrew", Ratio: hebrewRatio},
		{Code: LangArabic, Name: "Arabic", Ratio: arabicRatio},
		{Code: LangRussian, Name: "Russian", Ratio: cyrillicRatio},
		{Code: LangChinese, Name: "Chinese", Ratio: chineseRatio},
		{Code: LangJapanese, Name: "Japanese", Ratio: japaneseRatio},
		{Code: LangKorean, Name: "Korean", Ratio: koreanRatio},
	}
}

// determineLanguageFromRatios determines the language based on script ratios
func determineLanguageFromRatios(ratios []ScriptRatio, text string) Language {
	threshold := 0.1 // Minimum 10% of characters must be in the target script

	// Find the highest ratio above threshold
	var bestMatch ScriptRatio
	bestMatch.Code = LangEnglish
	bestMatch.Name = "English"
	bestMatch.Ratio = 0.0

	for _, ratio := range ratios {
		if ratio.Ratio > threshold && ratio.Ratio > bestMatch.Ratio {
			bestMatch = ratio
		}
	}

	// If no language meets the threshold, check for any non-English script
	if bestMatch.Code == LangEnglish {
		for _, ratio := range ratios {
			if ratio.Ratio > 0.01 && ratio.Ratio > bestMatch.Ratio { // Lower threshold for mixed text
				bestMatch = ratio
			}
		}
	}

	// Special handling for Chinese vs Japanese
	if bestMatch.Code == LangChinese || bestMatch.Code == LangJapanese {
		return handleChineseJapanese(ratios, bestMatch, text)
	}

	return Language{Code: bestMatch.Code, Name: bestMatch.Name, Confidence: bestMatch.Ratio}
}

// handleChineseJapanese handles the special case of distinguishing Chinese from Japanese
func handleChineseJapanese(ratios []ScriptRatio, bestMatch ScriptRatio, text string) Language {
	// Check for Hiragana/Katakana characters to distinguish Japanese from Chinese
	hiraganaKatakanaPattern := regexp.MustCompile(`[\x{3040}-\x{309F}\x{30A0}-\x{30FF}]`)
	hiraganaKatakanaMatches := hiraganaKatakanaPattern.FindAllString(text, -1)
	hiraganaKatakanaRatio := float64(len(hiraganaKatakanaMatches)) / float64(len([]rune(text)))

	// If there are significant Hiragana/Katakana characters, it's Japanese
	if hiraganaKatakanaRatio > 0.05 { // More than 5% Hiragana/Katakana
		return Language{Code: LangJapanese, Name: "Japanese", Confidence: bestMatch.Ratio}
	}

	// Otherwise, it's Chinese
	return Language{Code: LangChinese, Name: "Chinese", Confidence: bestMatch.Ratio}
}

// GetLanguageInstruction returns a language instruction for the AI based on detected language
func GetLanguageInstruction(lang Language) string {
	switch lang.Code {
	case LangHebrew:
		return "Please respond in Hebrew (עברית)."
	case LangArabic:
		return "Please respond in Arabic (العربية)."
	case LangRussian:
		return "Please respond in Russian (Русский)."
	case LangChinese:
		return "Please respond in Chinese (中文)."
	case LangJapanese:
		return "Please respond in Japanese (日本語)."
	case LangKorean:
		return "Please respond in Korean (한국어)."
	case "es":
		return "Please respond in Spanish (Español)."
	case "fr":
		return "Please respond in French (Français)."
	case "de":
		return "Please respond in German (Deutsch)."
	case "it":
		return "Please respond in Italian (Italiano)."
	case "pt":
		return "Please respond in Portuguese (Português)."
	default:
		return "Please respond in English."
	}
}
