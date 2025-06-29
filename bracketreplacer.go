package main

import (
	"regexp"
	"strings"
	"unicode"
)

var allBracketPairs = map[rune]rune{
	'(': ')',
	'[': ']',
	'{': '}',
	'<': '>',
}

type BracketReplacer struct {
	activeBrackets map[rune]rune  // Only the bracket pairs we actually use
	contentRegex   *regexp.Regexp // Single compiled regex for all patterns
}

func NewBracketReplacer(configString string) *BracketReplacer {
	br := &BracketReplacer{
		activeBrackets: make(map[rune]rune),
	}

	br.parseConfig(configString)
	return br
}

func (br *BracketReplacer) parseConfig(config string) {
	var patterns []string
	var patternBuilder strings.Builder
	inPattern := false

	// First pass: identify active brackets and extract pattern content
	for _, char := range config {
		if br.isKnownOpenBracket(char) {
			br.activeBrackets[char] = allBracketPairs[char]
			inPattern = true
		} else if br.isKnownCloseBracket(char) {
			inPattern = false
		} else if inPattern {
			patternBuilder.WriteRune(char)
		}
	}

	// Parse patterns (split by comma, trim spaces)
	patternStr := strings.TrimSpace(patternBuilder.String())
	if patternStr != "" {
		rawPatterns := strings.SplitSeq(patternStr, ",")
		for pattern := range rawPatterns {
			trimmed := strings.TrimSpace(pattern)
			if trimmed != "" {
				patterns = append(patterns, trimmed)
			}
		}
	}

	// If no patterns specified, default to match everything
	if len(patterns) == 0 {
		patterns = []string{"*"}
	}

	br.buildContentRegex(patterns)
}

func (br *BracketReplacer) buildContentRegex(patterns []string) {
	var regexParts []string

	for _, pattern := range patterns {
		regexPart := br.convertPatternToRegex(pattern)
		regexParts = append(regexParts, regexPart)
	}

	// Combine all patterns with OR (|) and make case-insensitive
	fullPattern := "(?i)(" + strings.Join(regexParts, "|") + ")"

	br.contentRegex = regexp.MustCompile(fullPattern)
}

func (br *BracketReplacer) convertPatternToRegex(pattern string) string {
	var result strings.Builder

	runes := []rune(pattern)
	i := 0

	for i < len(runes) {
		switch runes[i] {
		case '*':
			result.WriteString(".*")
		default:
			if unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || unicode.IsSpace(runes[i]) {
				result.WriteRune(runes[i])
			} else {
				result.WriteString(regexp.QuoteMeta(string(runes[i])))
			}
		}
		i++
	}

	return result.String()
}

func (br *BracketReplacer) isKnownOpenBracket(r rune) bool {
	_, exists := allBracketPairs[r]
	return exists
}

func (br *BracketReplacer) isKnownCloseBracket(r rune) bool {
	for _, closeRune := range allBracketPairs {
		if r == closeRune {
			return true
		}
	}
	return false
}

func (br *BracketReplacer) isActiveOpenBracket(r rune) bool {
	_, exists := br.activeBrackets[r]
	return exists
}

func (br *BracketReplacer) Replace(text, replacement string) string {
	var result strings.Builder
	runes := []rune(text)
	i := 0

	for i < len(runes) {
		if br.isActiveOpenBracket(runes[i]) {
			start := i
			openBracket := runes[i]
			closeBracket := br.activeBrackets[openBracket]

			// Find matching closing bracket
			bracketCount := 1
			i++

			for i < len(runes) && bracketCount > 0 {
				if runes[i] == openBracket {
					bracketCount++
				} else if runes[i] == closeBracket {
					bracketCount--
				}
				i++
			}

			if bracketCount == 0 {
				// Extract content between brackets
				content := string(runes[start+1 : i-1])
				if br.contentRegex.MatchString(content) {
					result.WriteString(replacement)
				} else {
					result.WriteString(string(runes[start:i]))
				}
			} else {
				// No matching bracket, write as-is
				result.WriteString(string(runes[start:]))
				break
			}
		} else {
			result.WriteRune(runes[i])
			i++
		}
	}

	return result.String()
}

// Function to replace text using a pattern, for use in a template
func ReplaceInBrackets(configString, replacement, text string) string {

	replacer := NewBracketReplacer(configString)
	return replacer.Replace(text, replacement)
}

func RemoveBrackets(configString, text string) string {
	replacer := NewBracketReplacer(configString)
	return replacer.Replace(text, "")
}
