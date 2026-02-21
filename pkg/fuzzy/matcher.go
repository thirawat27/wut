// Package fuzzy provides advanced fuzzy matching functionality for WUT
package fuzzy

import (
	"strings"
	"unicode"

	"github.com/agnivade/levenshtein"
)

// Algorithm represents the fuzzy matching algorithm type
type Algorithm string

const (
	// AlgorithmLevenshtein uses Levenshtein distance
	AlgorithmLevenshtein Algorithm = "levenshtein"
	// AlgorithmPrefix gives higher score to prefix matches
	AlgorithmPrefix Algorithm = "prefix"
	// AlgorithmContains gives higher score to substring matches
	AlgorithmContains Algorithm = "contains"
	// AlgorithmHybrid combines multiple algorithms
	AlgorithmHybrid Algorithm = "hybrid"
)

// Matcher provides fuzzy string matching with multiple algorithms
type Matcher struct {
	caseSensitive bool
	maxDistance   int
	threshold     float64
	algorithm     Algorithm
	
	// Weights for hybrid algorithm
	prefixWeight    float64
	containsWeight  float64
	levenshteinWeight float64
}

// Match represents a fuzzy match result
type Match struct {
	Confidence float64
	Distance   int
	Algorithm  Algorithm
	MatchedAt  int // Position where match starts (for highlighting)
	
	// Detailed scores for debugging/analysis
	PrefixScore    float64
	ContainsScore  float64
	LevenshteinScore float64
}

// MatchOptions represents options for matching
type MatchOptions struct {
	Algorithm     Algorithm
	CaseSensitive bool
	MaxDistance   int
	Threshold     float64
}

// NewMatcher creates a new fuzzy matcher with default settings
func NewMatcher(caseSensitive bool, maxDistance int, threshold float64) *Matcher {
	return &Matcher{
		caseSensitive:     caseSensitive,
		maxDistance:       maxDistance,
		threshold:         threshold,
		algorithm:         AlgorithmHybrid,
		prefixWeight:      0.4,
		containsWeight:    0.3,
		levenshteinWeight: 0.3,
	}
}

// NewMatcherWithOptions creates a new fuzzy matcher with options
func NewMatcherWithOptions(opts MatchOptions) *Matcher {
	m := NewMatcher(opts.CaseSensitive, opts.MaxDistance, opts.Threshold)
	m.algorithm = opts.Algorithm
	return m
}

// SetAlgorithm sets the matching algorithm
func (m *Matcher) SetAlgorithm(alg Algorithm) {
	m.algorithm = alg
}

// SetWeights sets the weights for hybrid algorithm
func (m *Matcher) SetWeights(prefix, contains, levenshtein float64) {
	m.prefixWeight = prefix
	m.containsWeight = contains
	m.levenshteinWeight = levenshtein
}

// Match performs fuzzy matching between query and target
func (m *Matcher) Match(query, target string) *Match {
	if query == "" {
		return &Match{Confidence: 0, Distance: 0, Algorithm: m.algorithm}
	}
	
	if query == target {
		return &Match{Confidence: 1.0, Distance: 0, Algorithm: m.algorithm, MatchedAt: 0}
	}

	originalTarget := target
	
	if !m.caseSensitive {
		query = toLower(query)
		target = toLower(target)
	}

	switch m.algorithm {
	case AlgorithmLevenshtein:
		return m.matchLevenshtein(query, target)
	case AlgorithmPrefix:
		return m.matchPrefix(query, target)
	case AlgorithmContains:
		return m.matchContains(query, target, originalTarget)
	case AlgorithmHybrid:
		return m.matchHybrid(query, target, originalTarget)
	default:
		return m.matchHybrid(query, target, originalTarget)
	}
}

// MatchMultiple matches query against multiple targets and returns sorted results
func (m *Matcher) MatchMultiple(query string, targets []string) []MatchResult {
	results := make([]MatchResult, 0, len(targets))
	
	for i, target := range targets {
		match := m.Match(query, target)
		if match.Confidence >= m.threshold {
			results = append(results, MatchResult{
				Target:    target,
				Index:     i,
				Match:     match,
			})
		}
	}
	
	// Sort by confidence (highest first)
	sortResults(results)
	return results
}

// MatchResult represents a match against a specific target
type MatchResult struct {
	Target string
	Index  int
	Match  *Match
}

// matchLevenshtein performs pure Levenshtein distance matching
func (m *Matcher) matchLevenshtein(query, target string) *Match {
	distance := levenshtein.ComputeDistance(query, target)
	
	// Apply max distance filter
	if m.maxDistance > 0 && distance > m.maxDistance {
		return &Match{Confidence: 0, Distance: distance, Algorithm: AlgorithmLevenshtein}
	}
	
	maxLen := max(len(query), len(target))
	if maxLen == 0 {
		return &Match{Confidence: 1.0, Distance: 0, Algorithm: AlgorithmLevenshtein}
	}

	confidence := 1.0 - float64(distance)/float64(maxLen)
	if confidence < 0 {
		confidence = 0
	}

	return &Match{
		Confidence:       confidence,
		Distance:         distance,
		Algorithm:        AlgorithmLevenshtein,
		LevenshteinScore: confidence,
	}
}

// matchPrefix performs prefix-based matching
func (m *Matcher) matchPrefix(query, target string) *Match {
	if strings.HasPrefix(target, query) {
		// Exact prefix match
		confidence := 0.9 + (0.1 * (1.0 - float64(len(query))/float64(len(target))))
		return &Match{
			Confidence: confidence,
			Distance:   0,
			Algorithm:  AlgorithmPrefix,
			MatchedAt:  0,
			PrefixScore: confidence,
		}
	}
	
	// Try to find if query is a prefix of any word in target
	words := strings.Fields(target)
	for _, word := range words {
		if strings.HasPrefix(word, query) {
			confidence := 0.7 + (0.2 * (1.0 - float64(len(query))/float64(len(word))))
			return &Match{
				Confidence:  confidence,
				Distance:    0,
				Algorithm:   AlgorithmPrefix,
				PrefixScore: confidence,
			}
		}
	}
	
	// Fallback to Levenshtein with reduced confidence
	result := m.matchLevenshtein(query, target)
	result.Confidence *= 0.5
	result.Algorithm = AlgorithmPrefix
	return result
}

// matchContains performs substring matching
func (m *Matcher) matchContains(query, target, originalTarget string) *Match {
	idx := strings.Index(target, query)
	if idx >= 0 {
		// Exact substring match
		confidence := 0.8 + (0.15 * (float64(len(query)) / float64(len(target))))
		if idx == 0 {
			confidence += 0.05 // Bonus for matching at start
		}
		return &Match{
			Confidence:    confidence,
			Distance:      0,
			Algorithm:     AlgorithmContains,
			MatchedAt:     idx,
			ContainsScore: confidence,
		}
	}
	
	// Check if all characters appear in order (fuzzy contains)
	if matched, positions := fuzzyContains(query, target); matched {
		confidence := 0.5 + (0.3 * (float64(len(positions)) / float64(len(target))))
		return &Match{
			Confidence:    confidence,
			Distance:      len(target) - len(positions),
			Algorithm:     AlgorithmContains,
			MatchedAt:     positions[0],
			ContainsScore: confidence,
		}
	}
	
	// Fallback to Levenshtein
	result := m.matchLevenshtein(query, target)
	result.Algorithm = AlgorithmContains
	return result
}

// matchHybrid combines multiple matching strategies
func (m *Matcher) matchHybrid(query, target, originalTarget string) *Match {
	prefixMatch := m.matchPrefix(query, target)
	containsMatch := m.matchContains(query, target, originalTarget)
	levenshteinMatch := m.matchLevenshtein(query, target)
	
	// Calculate weighted confidence
	confidence := (prefixMatch.Confidence * m.prefixWeight) +
		(containsMatch.Confidence * m.containsWeight) +
		(levenshteinMatch.Confidence * m.levenshteinWeight)
	
	// Boost confidence for exact or near-exact matches
	if levenshteinMatch.Distance == 0 && prefixMatch.Confidence > 0.9 {
		confidence = 1.0
	}
	
	// Determine best algorithm and matched position
	matchedAt := -1
	
	if prefixMatch.Confidence > containsMatch.Confidence && prefixMatch.Confidence > levenshteinMatch.Confidence {
		matchedAt = 0
	} else if containsMatch.Confidence > levenshteinMatch.Confidence {
		matchedAt = containsMatch.MatchedAt
	}
	
	return &Match{
		Confidence:       confidence,
		Distance:         levenshteinMatch.Distance,
		Algorithm:        AlgorithmHybrid,
		MatchedAt:        matchedAt,
		PrefixScore:      prefixMatch.Confidence,
		ContainsScore:    containsMatch.Confidence,
		LevenshteinScore: levenshteinMatch.Confidence,
	}
}

// fuzzyContains checks if all characters of query appear in target in order
func fuzzyContains(query, target string) (bool, []int) {
	if len(query) == 0 {
		return true, []int{}
	}
	if len(target) == 0 {
		return false, nil
	}
	
	positions := make([]int, 0, len(query))
	targetIdx := 0
	
	for _, queryRune := range query {
		found := false
		for targetIdx < len(target) {
			if rune(target[targetIdx]) == queryRune {
				positions = append(positions, targetIdx)
				targetIdx++
				found = true
				break
			}
			targetIdx++
		}
		if !found {
			return false, nil
		}
	}
	
	return true, positions
}

// HighlightMatch returns the target with the matched portion highlighted
func (m *Matcher) HighlightMatch(query, target string, highlightFunc func(string) string) string {
	if query == "" || highlightFunc == nil {
		return target
	}
	
	match := m.Match(query, target)
	if match.MatchedAt < 0 || match.MatchedAt >= len(target) {
		return target
	}
	
	// Determine match length
	matchLen := len(query)
	if match.Algorithm == AlgorithmLevenshtein {
		// For Levenshtein, try to find best matching substring
		return target // Skip highlighting for pure Levenshtein
	}
	
	endPos := match.MatchedAt + matchLen
	if endPos > len(target) {
		endPos = len(target)
	}
	
	before := target[:match.MatchedAt]
	matched := target[match.MatchedAt:endPos]
after := ""
	if endPos < len(target) {
		after = target[endPos:]
	}
	
	return before + highlightFunc(matched) + after
}

// sortResults sorts match results by confidence (descending)
func sortResults(results []MatchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Match.Confidence > results[i].Match.Confidence {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// toLower converts string to lowercase (Unicode-aware)
func toLower(s string) string {
	return strings.Map(func(r rune) rune {
		return unicode.ToLower(r)
	}, s)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// NormalizeString normalizes a string for better matching
func NormalizeString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)
	
	// Remove extra whitespace
	s = strings.Join(strings.Fields(s), " ")
	
	// Remove common punctuation that might interfere with matching
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	
	return strings.TrimSpace(s)
}

// Tokenize splits a string into searchable tokens
func Tokenize(s string) []string {
	s = NormalizeString(s)
	
	// Split by whitespace and common delimiters
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '/'
	})
	
	// Remove duplicates and empty strings
	seen := make(map[string]bool)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" && !seen[f] {
			seen[f] = true
			tokens = append(tokens, f)
		}
	}
	
	return tokens
}

// CalculateRelevance calculates a relevance score for ranking
func CalculateRelevance(query, target string, match *Match) float64 {
	baseScore := match.Confidence
	
	// Boost exact matches
	if strings.EqualFold(query, target) {
		baseScore += 0.5
	}
	
	// Boost prefix matches
	if strings.HasPrefix(strings.ToLower(target), strings.ToLower(query)) {
		baseScore += 0.2
	}
	
	// Boost shorter targets (more specific matches)
	if len(target) < len(query)*2 {
		baseScore += 0.1 * (1.0 - float64(len(target))/float64(len(query)*2))
	}
	
	// Cap at 1.0
	if baseScore > 1.0 {
		baseScore = 1.0
	}
	
	return baseScore
}
