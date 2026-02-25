// Package performance provides high-performance fuzzy matching
package performance

import (
	"unicode"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// FastMatcher provides high-performance fuzzy matching
// Uses optimized algorithms with minimal allocations
type FastMatcher struct {
	caseSensitive bool
	threshold     float64
	maxDistance   int
}

// NewFastMatcher creates a new fast matcher
func NewFastMatcher(caseSensitive bool, threshold float64, maxDistance int) *FastMatcher {
	return &FastMatcher{
		caseSensitive: caseSensitive,
		threshold:     threshold,
		maxDistance:   maxDistance,
	}
}

// MatchResult represents a fuzzy match result
type MatchResult struct {
	Score      float64
	Distance   int
	Matched    bool
	MatchStart int
	MatchEnd   int
}

// Match performs fuzzy matching between query and target
// Optimized for speed with early exit on poor matches
func (m *FastMatcher) Match(query, target string) MatchResult {
	if query == "" {
		return MatchResult{Score: 0, Matched: true}
	}

	if query == target {
		return MatchResult{Score: 1.0, Distance: 0, Matched: true, MatchStart: 0, MatchEnd: len(target)}
	}

	// Preprocess
	if !m.caseSensitive {
		query = fastToLowerASCII(query)
		target = fastToLowerASCII(target)
	}

	// Quick checks for fast path
	if len(query) > len(target)*2 {
		return MatchResult{Score: 0, Matched: false}
	}

	// Try exact substring match first (fastest)
	if idx := fastIndexASCII(target, query); idx >= 0 {
		score := 0.8 + 0.2*(float64(len(query))/float64(len(target)))
		if idx == 0 {
			score += 0.1 // Bonus for prefix match
		}
		return MatchResult{
			Score:      minFloat64(score, 1.0),
			Distance:   0,
			Matched:    true,
			MatchStart: idx,
			MatchEnd:   idx + len(query),
		}
	}

	// Try prefix match
	if fastHasPrefixASCII(target, query) {
		return MatchResult{
			Score:      0.95,
			Distance:   0,
			Matched:    true,
			MatchStart: 0,
			MatchEnd:   len(query),
		}
	}

	// Fuzzy match
	matched, positions := fuzzyMatch(query, target)
	if !matched {
		// Try highly optimized Levenshtein distance from fuzzysearch
		dist := fuzzy.LevenshteinDistance(query, target)
		if dist > m.maxDistance {
			return MatchResult{Score: 0, Matched: false}
		}

		maxLen := maxInt(len(query), len(target))
		score := 1.0 - float64(dist)/float64(maxLen)
		if score < m.threshold {
			return MatchResult{Score: 0, Matched: false}
		}

		return MatchResult{
			Score:    score,
			Distance: dist,
			Matched:  true,
		}
	}

	// Calculate score based on match quality
	score := calculateFuzzyScore(query, target, positions)
	if score < m.threshold {
		return MatchResult{Score: 0, Matched: false}
	}

	return MatchResult{
		Score:      score,
		Distance:   len(target) - len(positions),
		Matched:    true,
		MatchStart: positions[0],
		MatchEnd:   positions[len(positions)-1] + 1,
	}
}

// MatchMultiple matches query against multiple targets
func (m *FastMatcher) MatchMultiple(query string, targets []string) []ScoredMatch {
	results := make([]ScoredMatch, 0, 32)

	for i, target := range targets {
		result := m.Match(query, target)
		if result.Matched {
			results = append(results, ScoredMatch{
				Target: target,
				Index:  i,
				Score:  result.Score,
			})
		}
	}

	// Sort by score (descending)
	quickSortScoredMatches(results, 0, len(results)-1)
	return results
}

// ScoredMatch represents a scored match
type ScoredMatch struct {
	Target string
	Index  int
	Score  float64
}

// fuzzyMatch checks if all characters of query appear in target in order
// Returns matched positions for scoring
func fuzzyMatch(query, target string) (bool, []int) {
	if len(query) == 0 {
		return true, []int{}
	}
	if len(target) == 0 {
		return false, nil
	}

	// Check if all query chars exist in target
	// Using pre-allocated stack array for small queries
	var positions [256]int
	posCount := 0

	targetIdx := 0
	for i := 0; i < len(query); i++ {
		qc := query[i]

		// Find in target
		found := false
		for targetIdx < len(target) {
			if target[targetIdx] == qc {
				if posCount < len(positions) {
					positions[posCount] = targetIdx
				}
				posCount++
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

	// Copy positions to slice
	result := make([]int, posCount)
	copy(result, positions[:posCount])
	return true, result
}

// calculateFuzzyScore calculates a score based on match quality
func calculateFuzzyScore(query, target string, positions []int) float64 {
	if len(positions) == 0 {
		return 0
	}

	// Base score from match ratio
	matchRatio := float64(len(query)) / float64(len(target))
	score := 0.5 * matchRatio

	// Bonus for consecutive matches
	consecutive := 0
	for i := 1; i < len(positions); i++ {
		if positions[i] == positions[i-1]+1 {
			consecutive++
		}
	}
	if len(query) > 1 {
		score += 0.3 * float64(consecutive) / float64(len(query)-1)
	}

	// Bonus for starting at position 0
	if positions[0] == 0 {
		score += 0.1
	}

	// Penalty for spread out matches
	spread := positions[len(positions)-1] - positions[0] + 1
	spreadRatio := float64(len(query)) / float64(spread)
	score += 0.1 * spreadRatio

	return minFloat64(score, 1.0)
}

// fastToLowerASCII converts ASCII string to lowercase
func fastToLowerASCII(s string) string {
	// Check if conversion needed
	needsLower := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			needsLower = true
			break
		}
	}
	if !needsLower {
		return s
	}

	// Create lowercase version
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// fastIndexASCII finds substring index (ASCII only)
func fastIndexASCII(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}

	// Simple substring search
	first := substr[0]
	max := len(s) - len(substr) + 1

	for i := range max {
		if s[i] == first {
			match := true
			for j := 1; j < len(substr); j++ {
				if s[i+j] != substr[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}

// fastHasPrefixASCII checks prefix (ASCII only)
func fastHasPrefixASCII(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// quickSortScoredMatches sorts matches by score (descending)
func quickSortScoredMatches(arr []ScoredMatch, low, high int) {
	if low < high {
		pi := partitionScoredMatches(arr, low, high)
		quickSortScoredMatches(arr, low, pi-1)
		quickSortScoredMatches(arr, pi+1, high)
	}
}

func partitionScoredMatches(arr []ScoredMatch, low, high int) int {
	pivot := arr[high].Score
	i := low - 1

	for j := low; j < high; j++ {
		if arr[j].Score > pivot { // Descending order
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

// Utility functions

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// UnicodeFuzzyMatcher handles Unicode strings
type UnicodeFuzzyMatcher struct {
	asciiMatcher *FastMatcher
}

// NewUnicodeFuzzyMatcher creates a new Unicode-aware matcher
func NewUnicodeFuzzyMatcher(caseSensitive bool, threshold float64, maxDistance int) *UnicodeFuzzyMatcher {
	return &UnicodeFuzzyMatcher{
		asciiMatcher: NewFastMatcher(caseSensitive, threshold, maxDistance),
	}
}

// Match performs fuzzy matching with Unicode support
func (m *UnicodeFuzzyMatcher) Match(query, target string) MatchResult {
	// Check if both strings are ASCII for fast path
	if isASCII(query) && isASCII(target) {
		return m.asciiMatcher.Match(query, target)
	}

	// Convert to rune slices for Unicode handling
	queryRunes := []rune(query)
	targetRunes := []rune(target)

	// Simple case folding for Unicode
	if !m.asciiMatcher.caseSensitive {
		queryRunes = toLowerRunes(queryRunes)
		targetRunes = toLowerRunes(targetRunes)
	}

	// Convert back to string for matching
	queryStr := string(queryRunes)
	targetStr := string(targetRunes)

	return m.asciiMatcher.Match(queryStr, targetStr)
}

// isASCII checks if string is ASCII
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// toLowerRunes converts rune slice to lowercase
func toLowerRunes(runes []rune) []rune {
	result := make([]rune, len(runes))
	for i, r := range runes {
		result[i] = unicode.ToLower(r)
	}
	return result
}

// PrefixMatcher provides fast prefix matching
type PrefixMatcher struct {
	prefixes []string
	index    map[byte][]int // First char index
}

// NewPrefixMatcher creates a new prefix matcher
func NewPrefixMatcher(prefixes []string) *PrefixMatcher {
	index := make(map[byte][]int)
	for i, p := range prefixes {
		if len(p) > 0 {
			first := p[0]
			index[first] = append(index[first], i)
		}
	}
	return &PrefixMatcher{
		prefixes: prefixes,
		index:    index,
	}
}

// FindPrefix finds the longest matching prefix
func (m *PrefixMatcher) FindPrefix(s string) (string, int) {
	if len(s) == 0 {
		return "", -1
	}

	first := s[0]
	indices, ok := m.index[first]
	if !ok {
		return "", -1
	}

	var longest string
	var longestIdx = -1

	for _, idx := range indices {
		prefix := m.prefixes[idx]
		if len(prefix) > len(s) {
			continue
		}
		if s[:len(prefix)] == prefix && len(prefix) > len(longest) {
			longest = prefix
			longestIdx = idx
		}
	}

	return longest, longestIdx
}

// HasPrefix checks if any prefix matches
func (m *PrefixMatcher) HasPrefix(s string) bool {
	_, idx := m.FindPrefix(s)
	return idx >= 0
}

// SuffixMatcher provides fast suffix matching
type SuffixMatcher struct {
	suffixes []string
}

// NewSuffixMatcher creates a new suffix matcher
func NewSuffixMatcher(suffixes []string) *SuffixMatcher {
	return &SuffixMatcher{suffixes: suffixes}
}

// HasSuffix checks if string has any of the suffixes
func (m *SuffixMatcher) HasSuffix(s string) bool {
	for _, suffix := range m.suffixes {
		if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// BoyerMoore provides Boyer-Moore string search
type BoyerMoore struct {
	pattern []byte
	badChar [256]int
}

// NewBoyerMoore creates a new Boyer-Moore searcher
func NewBoyerMoore(pattern string) *BoyerMoore {
	if len(pattern) == 0 {
		return nil
	}

	p := []byte(pattern)
	bm := &BoyerMoore{
		pattern: p,
	}

	// Initialize bad character table
	for i := range bm.badChar {
		bm.badChar[i] = -1
	}
	for i := range p {
		bm.badChar[p[i]] = i
	}

	return bm
}

// Search searches for pattern in text
func (bm *BoyerMoore) Search(text string) int {
	if len(bm.pattern) == 0 {
		return 0
	}
	if len(text) < len(bm.pattern) {
		return -1
	}

	t := []byte(text)
	n := len(t)
	m := len(bm.pattern)

	s := 0
	for s <= n-m {
		j := m - 1

		for j >= 0 && bm.pattern[j] == t[s+j] {
			j--
		}

		if j < 0 {
			return s
		}

		// Shift based on bad character
		badCharShift := max(j-bm.badChar[t[s+j]], 1)
		s += badCharShift
	}

	return -1
}

// Trie is a simple trie for fast prefix matching
type Trie struct {
	root *trieNode
}

type trieNode struct {
	children [256]*trieNode
	isEnd    bool
	value    any
}

// NewTrie creates a new trie
func NewTrie() *Trie {
	return &Trie{
		root: &trieNode{},
	}
}

// Insert adds a word to the trie
func (t *Trie) Insert(word string, value any) {
	node := t.root
	for i := 0; i < len(word); i++ {
		c := word[i]
		if node.children[c] == nil {
			node.children[c] = &trieNode{}
		}
		node = node.children[c]
	}
	node.isEnd = true
	node.value = value
}

// Search checks if word exists in trie
func (t *Trie) Search(word string) (any, bool) {
	node := t.root
	for i := 0; i < len(word); i++ {
		c := word[i]
		if node.children[c] == nil {
			return nil, false
		}
		node = node.children[c]
	}
	return node.value, node.isEnd
}

// StartsWith checks if any word starts with prefix
func (t *Trie) StartsWith(prefix string) bool {
	node := t.root
	for i := 0; i < len(prefix); i++ {
		c := prefix[i]
		if node.children[c] == nil {
			return false
		}
		node = node.children[c]
	}
	return true
}

// FindWithPrefix finds all words with given prefix
func (t *Trie) FindWithPrefix(prefix string) []string {
	node := t.root
	for i := 0; i < len(prefix); i++ {
		c := prefix[i]
		if node.children[c] == nil {
			return nil
		}
		node = node.children[c]
	}

	var results []string
	t.dfs(node, prefix, &results)
	return results
}

func (t *Trie) dfs(node *trieNode, prefix string, results *[]string) {
	if node.isEnd {
		*results = append(*results, prefix)
	}
	for c := range 256 {
		if node.children[c] != nil {
			t.dfs(node.children[c], prefix+string(byte(c)), results)
		}
	}
}
