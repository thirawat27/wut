// Package fuzzy provides fuzzy matching functionality for WUT
package fuzzy

import (
	"strings"
	"unicode"

	"github.com/agnivade/levenshtein"
	"github.com/sahilm/fuzzy"
)

// Match represents a fuzzy match result
type Match struct {
	Text        string  // Matched text
	Score       int     // Match score (higher is better)
	Distance    int     // Levenshtein distance (lower is better)
	Confidence  float64 // Confidence score (0-1)
	MatchedIndices []int // Indices of matched characters
}

// Matcher provides fuzzy matching capabilities
type Matcher struct {
	caseSensitive bool
	maxDistance   int
	threshold     float64
}

// NewMatcher creates a new fuzzy matcher
func NewMatcher(caseSensitive bool, maxDistance int, threshold float64) *Matcher {
	return &Matcher{
		caseSensitive: caseSensitive,
		maxDistance:   maxDistance,
		threshold:     threshold,
	}
}

// FindMatches finds matches for a pattern in a list of candidates
func (m *Matcher) FindMatches(pattern string, candidates []string) []Match {
	if !m.caseSensitive {
		pattern = strings.ToLower(pattern)
	}

	var matches []Match
	for _, candidate := range candidates {
		match := m.Match(pattern, candidate)
		if match.Confidence >= m.threshold {
			matches = append(matches, match)
		}
	}

	// Sort by score (descending)
	sortMatches(matches)
	return matches
}

// Match performs a single fuzzy match
func (m *Matcher) Match(pattern, text string) Match {
	originalText := text
	if !m.caseSensitive {
		pattern = strings.ToLower(pattern)
		text = strings.ToLower(text)
	}

	// Calculate Levenshtein distance
	distance := levenshtein.ComputeDistance(pattern, text)

	// Use sahil/fuzzy for better matching
	fuzzyMatches := fuzzy.Find(pattern, []string{text})

	var score int
	var matchedIndices []int

	if len(fuzzyMatches) > 0 {
		score = fuzzyMatches[0].Score
		matchedIndices = fuzzyMatches[0].MatchedIndexes
	}

	// Calculate confidence
	confidence := calculateConfidence(pattern, text, distance, score)

	return Match{
		Text:           originalText,
		Score:          score,
		Distance:       distance,
		Confidence:     confidence,
		MatchedIndices: matchedIndices,
	}
}

// MatchWithSource performs matching with source information
type MatchWithSource struct {
	Match
	Source      string // Source of the match (e.g., "history", "alias", "builtin")
	Description string // Optional description
}

// FindMatchesMultiSource finds matches from multiple sources
func (m *Matcher) FindMatchesMultiSource(pattern string, sources map[string][]string) []MatchWithSource {
	var allMatches []MatchWithSource

	for source, candidates := range sources {
		matches := m.FindMatches(pattern, candidates)
		for _, match := range matches {
			allMatches = append(allMatches, MatchWithSource{
				Match:  match,
				Source: source,
			})
		}
	}

	// Sort by confidence (descending)
	sortMatchesWithSource(allMatches)
	return allMatches
}

// SuggestCorrections suggests corrections for a typo
func (m *Matcher) SuggestCorrections(typo string, dictionary []string, maxSuggestions int) []Match {
	matches := m.FindMatches(typo, dictionary)

	// Filter by max distance
	var filtered []Match
	for _, match := range matches {
		if match.Distance <= m.maxDistance {
			filtered = append(filtered, match)
		}
	}

	// Limit results
	if maxSuggestions > 0 && len(filtered) > maxSuggestions {
		filtered = filtered[:maxSuggestions]
	}

	return filtered
}

// IsTypo checks if a word is likely a typo of any word in the dictionary
func (m *Matcher) IsTypo(word string, dictionary []string) bool {
	// Exact match
	for _, dictWord := range dictionary {
		if !m.caseSensitive {
			if strings.EqualFold(word, dictWord) {
				return false
			}
		} else {
			if word == dictWord {
				return false
			}
		}
	}

	// Check for close matches
	matches := m.FindMatches(word, dictionary)
	for _, match := range matches {
		if match.Distance <= m.maxDistance && match.Confidence >= m.threshold {
			return true
		}
	}

	return false
}

// calculateConfidence calculates a confidence score
func calculateConfidence(pattern, text string, distance, score int) float64 {
	if len(pattern) == 0 {
		return 0
	}

	// Base confidence on Levenshtein distance
	maxLen := len(pattern)
	if len(text) > maxLen {
		maxLen = len(text)
	}
	if maxLen == 0 {
		return 1.0
	}

	distanceConfidence := 1.0 - float64(distance)/float64(maxLen)

	// Factor in fuzzy score
	scoreConfidence := float64(score) / float64(len(pattern)*10)
	if scoreConfidence > 1.0 {
		scoreConfidence = 1.0
	}

	// Weighted average
	confidence := (distanceConfidence*0.6 + scoreConfidence*0.4)

	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	return confidence
}

// sortMatches sorts matches by score (descending)
func sortMatches(matches []Match) {
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score ||
				(matches[j].Score == matches[i].Score && matches[j].Confidence > matches[i].Confidence) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

// sortMatchesWithSource sorts matches with source by confidence (descending)
func sortMatchesWithSource(matches []MatchWithSource) {
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Confidence > matches[i].Confidence ||
				(matches[j].Confidence == matches[i].Confidence && matches[j].Score > matches[i].Score) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

// NormalizeString normalizes a string for better matching
func NormalizeString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove extra whitespace
	s = strings.Join(strings.Fields(s), " ")

	// Remove special characters
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// PrefixMatcher provides prefix-based matching
type PrefixMatcher struct {
	caseSensitive bool
}

// NewPrefixMatcher creates a new prefix matcher
func NewPrefixMatcher(caseSensitive bool) *PrefixMatcher {
	return &PrefixMatcher{caseSensitive: caseSensitive}
}

// MatchPrefix finds all strings that start with the prefix
func (pm *PrefixMatcher) MatchPrefix(prefix string, candidates []string) []string {
	if !pm.caseSensitive {
		prefix = strings.ToLower(prefix)
	}

	var matches []string
	for _, candidate := range candidates {
		testCandidate := candidate
		if !pm.caseSensitive {
			testCandidate = strings.ToLower(candidate)
		}

		if strings.HasPrefix(testCandidate, prefix) {
			matches = append(matches, candidate)
		}
	}

	return matches
}

// TokenMatcher provides token-based matching
type TokenMatcher struct {
	caseSensitive bool
}

// NewTokenMatcher creates a new token matcher
func NewTokenMatcher(caseSensitive bool) *TokenMatcher {
	return &TokenMatcher{caseSensitive: caseSensitive}
}

// MatchTokens finds matches based on token containment
func (tm *TokenMatcher) MatchTokens(query string, candidates []string) []Match {
	if !tm.caseSensitive {
		query = strings.ToLower(query)
	}

	queryTokens := strings.Fields(query)
	if len(queryTokens) == 0 {
		return nil
	}

	var matches []Match
	for _, candidate := range candidates {
		testCandidate := candidate
		if !tm.caseSensitive {
			testCandidate = strings.ToLower(candidate)
		}

		candidateTokens := strings.Fields(testCandidate)

		// Count matching tokens
		matchCount := 0
		for _, qt := range queryTokens {
			for _, ct := range candidateTokens {
				if strings.Contains(ct, qt) || levenshtein.ComputeDistance(qt, ct) <= 1 {
					matchCount++
					break
				}
			}
		}

		if matchCount > 0 {
			confidence := float64(matchCount) / float64(len(queryTokens))
			if confidence >= 0.5 {
				matches = append(matches, Match{
					Text:       candidate,
					Score:      matchCount,
					Confidence: confidence,
				})
			}
		}
	}

	sortMatches(matches)
	return matches
}
