package utils

import (
	"regexp"
	"strings"
)

var (
	tokenPattern = regexp.MustCompile(`[a-z0-9]+`)
	stopwords    = map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "can": {}, "for": {},
		"from": {}, "have": {}, "how": {}, "i": {}, "im": {}, "i'm": {}, "in": {}, "is": {}, "it": {},
		"looking": {}, "me": {}, "my": {}, "need": {}, "of": {}, "on": {}, "or": {}, "our": {},
		"please": {}, "recommendation": {}, "recommendations": {}, "searching": {}, "seeking": {},
		"show": {}, "some": {}, "someone": {}, "that": {}, "the": {}, "their": {}, "them": {},
		"there": {}, "these": {}, "they": {}, "this": {}, "those": {}, "to": {}, "want": {},
		"was": {}, "we": {}, "were": {}, "what": {}, "when": {}, "where": {}, "which": {},
		"who": {}, "with": {}, "you": {}, "your": {},
	}
)

// ExtractMeaningfulTokens tokenizes text, removes stopwords, and deduplicates tokens while preserving order.
func ExtractMeaningfulTokens(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	rawTokens := tokenize(text)
	filtered := filterTokens(rawTokens)
	return dedupeTokens(filtered)
}

// BuildTokenSet builds a unique token set from the provided values.
func BuildTokenSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		tokens := ExtractMeaningfulTokens(value)
		for _, token := range tokens {
			set[token] = struct{}{}
		}
	}
	return set
}

// ContainsAllTokens returns true when the token set contains every required token.
// The missing slice lists any tokens that were not found.
func ContainsAllTokens(tokenSet map[string]struct{}, required []string) (bool, []string) {
	if len(required) == 0 {
		return true, nil
	}

	var missing []string
	for _, token := range required {
		if _, ok := tokenSet[token]; !ok {
			missing = append(missing, token)
		}
	}

	return len(missing) == 0, missing
}

// TokenHasDigit reports whether the token contains at least one numeric digit.
func TokenHasDigit(token string) bool {
	for _, r := range token {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	return tokenPattern.FindAllString(lower, -1)
}

func filterTokens(tokens []string) []string {
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if len(token) == 0 {
			continue
		}
		if len(token) == 1 && (token[0] < '0' || token[0] > '9') {
			continue
		}
		if _, isStopword := stopwords[token]; isStopword {
			continue
		}
		result = append(result, token)
	}
	return result
}

func dedupeTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return tokens
	}

	seen := make(map[string]struct{}, len(tokens))
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}
