// Package performance provides high-performance search utilities
package performance

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SearchResult represents a search result
type SearchResult struct {
	ID       string
	Score    float64
	Data     interface{}
	Matched  bool
}

// Searcher defines the search interface
type Searcher interface {
	Search(query string, limit int) []SearchResult
}

// InvertedIndex provides fast full-text search
type InvertedIndex struct {
	mu       sync.RWMutex
	index    map[string][]int64 // term -> document IDs
	docs     map[int64]*IndexedDoc
	nextID   atomic.Int64
	tokenizer Tokenizer
}

// IndexedDoc represents an indexed document
type IndexedDoc struct {
	ID       int64
	Content  string
	Tokens   []string
	Data     interface{}
	AddedAt  time.Time
}

// Tokenizer tokenizes text
type Tokenizer interface {
	Tokenize(text string) []string
}

// SimpleTokenizer is a basic tokenizer
type SimpleTokenizer struct{}

// Tokenize splits text into tokens
func (t SimpleTokenizer) Tokenize(text string) []string {
	return FastFields(FastToLower(text))
}

// NewInvertedIndex creates a new inverted index
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		index:     make(map[string][]int64),
		docs:      make(map[int64]*IndexedDoc),
		tokenizer: SimpleTokenizer{},
	}
}

// AddDocument adds a document to the index
func (idx *InvertedIndex) AddDocument(content string, data interface{}) int64 {
	id := idx.nextID.Add(1)
	
	tokens := idx.tokenizer.Tokenize(content)
	doc := &IndexedDoc{
		ID:      id,
		Content: content,
		Tokens:  tokens,
		Data:    data,
		AddedAt: time.Now(),
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.docs[id] = doc

	// Update index
	for _, token := range tokens {
		idx.index[token] = append(idx.index[token], id)
	}

	return id
}

// RemoveDocument removes a document from the index
func (idx *InvertedIndex) RemoveDocument(id int64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	doc, exists := idx.docs[id]
	if !exists {
		return
	}

	delete(idx.docs, id)

	// Remove from index
	for _, token := range doc.Tokens {
		ids := idx.index[token]
		for i, docID := range ids {
			if docID == id {
				idx.index[token] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}
}

// Search searches for documents matching the query
func (idx *InvertedIndex) Search(query string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 10
	}

	// Tokenize query - ensure lowercase tokens
	queryLower := FastToLower(query)
	queryTokens := idx.tokenizer.Tokenize(queryLower)
	if len(queryTokens) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Score documents
	docScores := make(map[int64]float64)

	for _, token := range queryTokens {
		docIDs := idx.index[token]
		for _, id := range docIDs {
			docScores[id]++
		}
	}

	// Convert to results
	results := make([]SearchResult, 0, len(docScores))
	for id, score := range docScores {
		if doc, ok := idx.docs[id]; ok {
			// Normalize score by document length
			normalizedScore := score / float64(len(doc.Tokens)) * float64(len(queryTokens))
			results = append(results, SearchResult{
				ID:      string(rune(id)),
				Score:   normalizedScore,
				Data:    doc.Data,
				Matched: true,
			})
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// SearchCache provides cached search results
type SearchCache struct {
	cache *LRUCache[string, CachedSearch]
}

// CachedSearch represents cached search results
type CachedSearch struct {
	Query     string
	Results   []SearchResult
	CachedAt  time.Time
	TTL       time.Duration
}

// IsExpired checks if cache entry is expired
func (c CachedSearch) IsExpired() bool {
	return time.Since(c.CachedAt) > c.TTL
}

// NewSearchCache creates a new search cache
func NewSearchCache(size int) *SearchCache {
	return &SearchCache{
		cache: NewLRUCache[string, CachedSearch](size, 32),
	}
}

// Get retrieves cached search results
func (sc *SearchCache) Get(query string) ([]SearchResult, bool) {
	cached, ok := sc.cache.Get(query)
	if !ok {
		return nil, false
	}

	if cached.IsExpired() {
		sc.cache.Delete(query)
		return nil, false
	}

	return cached.Results, true
}

// Set caches search results
func (sc *SearchCache) Set(query string, results []SearchResult, ttl time.Duration) {
	sc.cache.Set(query, CachedSearch{
		Query:    query,
		Results:  results,
		CachedAt: time.Now(),
		TTL:      ttl,
	}, ttl)
}

// Clear clears the cache
func (sc *SearchCache) Clear() {
	sc.cache.Clear()
}

// ConcurrentSearcher performs concurrent searches across multiple sources
type ConcurrentSearcher struct {
	searchers []Searcher
	workers   int
}

// NewConcurrentSearcher creates a new concurrent searcher
func NewConcurrentSearcher(searchers []Searcher, workers int) *ConcurrentSearcher {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return &ConcurrentSearcher{
		searchers: searchers,
		workers:   workers,
	}
}

// Search searches across all sources concurrently
func (cs *ConcurrentSearcher) Search(ctx context.Context, query string, limitPerSource int) ([]SearchResult, error) {
	if len(cs.searchers) == 0 {
		return nil, nil
	}

	resultsChan := make(chan []SearchResult, len(cs.searchers))
	errChan := make(chan error, len(cs.searchers))

	var wg sync.WaitGroup
	
	// Limit concurrent searches
	semaphore := make(chan struct{}, cs.workers)

	for _, searcher := range cs.searchers {
		wg.Add(1)
		go func(s Searcher) {
			defer wg.Done()
			
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			results := s.Search(query, limitPerSource)
			resultsChan <- results
		}(searcher)
	}

	// Close channels when done
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errChan)
	}()

	// Collect results
	var allResults []SearchResult
	done := make(chan struct{})
	
	go func() {
		for results := range resultsChan {
			allResults = append(allResults, results...)
		}
		close(done)
	}()

	select {
	case <-done:
		// Results collected
	case err := <-errChan:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return allResults, nil
}

// Autocomplete provides fast autocomplete functionality
type Autocomplete struct {
	mu       sync.RWMutex
	trie     *Trie
	scores   map[string]int // usage scores
	maxResults int
}

// NewAutocomplete creates a new autocomplete
func NewAutocomplete(maxResults int) *Autocomplete {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &Autocomplete{
		trie:       NewTrie(),
		scores:     make(map[string]int),
		maxResults: maxResults,
	}
}

// Add adds a term to autocomplete
func (ac *Autocomplete) Add(term string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.trie.Insert(term, nil)
	ac.scores[term]++
}

// AddWithScore adds a term with a specific score
func (ac *Autocomplete) AddWithScore(term string, score int) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.trie.Insert(term, nil)
	ac.scores[term] = score
}

// Suggest returns suggestions for a prefix
func (ac *Autocomplete) Suggest(prefix string) []string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	terms := ac.trie.FindWithPrefix(prefix)
	
	// Sort by score
	sort.Slice(terms, func(i, j int) bool {
		return ac.scores[terms[i]] > ac.scores[terms[j]]
	})

	if len(terms) > ac.maxResults {
		terms = terms[:ac.maxResults]
	}

	return terms
}

// SearchMetrics holds search performance metrics
type SearchMetrics struct {
	Queries      atomic.Uint64
	CacheHits    atomic.Uint64
	CacheMisses  atomic.Uint64
	AvgLatency   atomic.Int64 // nanoseconds
	TotalLatency atomic.Int64 // nanoseconds
}

// RecordQuery records a query
func (m *SearchMetrics) RecordQuery(latency time.Duration, cacheHit bool) {
	m.Queries.Add(1)
	m.TotalLatency.Add(int64(latency))
	
	// Update average
	queries := m.Queries.Load()
	total := m.TotalLatency.Load()
	m.AvgLatency.Store(total / int64(queries))

	if cacheHit {
		m.CacheHits.Add(1)
	} else {
		m.CacheMisses.Add(1)
	}
}

// CacheHitRate returns cache hit rate
func (m *SearchMetrics) CacheHitRate() float64 {
	hits := m.CacheHits.Load()
	misses := m.CacheMisses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// MeasuredSearcher wraps a searcher with metrics
type MeasuredSearcher struct {
	searcher Searcher
	cache    *SearchCache
	metrics  *SearchMetrics
}

// NewMeasuredSearcher creates a measured searcher
func NewMeasuredSearcher(searcher Searcher, cache *SearchCache) *MeasuredSearcher {
	return &MeasuredSearcher{
		searcher: searcher,
		cache:    cache,
		metrics:  &SearchMetrics{},
	}
}

// Search performs a measured search
func (ms *MeasuredSearcher) Search(query string, limit int) []SearchResult {
	start := time.Now()

	// Check cache
	if ms.cache != nil {
		if results, ok := ms.cache.Get(query); ok {
			ms.metrics.RecordQuery(time.Since(start), true)
			return results
		}
	}

	// Perform search
	results := ms.searcher.Search(query, limit)

	// Cache results
	if ms.cache != nil {
		ms.cache.Set(query, results, 5*time.Minute)
	}

	ms.metrics.RecordQuery(time.Since(start), false)
	return results
}

// Metrics returns search metrics
func (ms *MeasuredSearcher) Metrics() *SearchMetrics {
	return ms.metrics
}

// DocumentStore provides document storage and retrieval
type DocumentStore struct {
	mu    sync.RWMutex
	docs  map[string]*StoredDocument
	index *InvertedIndex
}

// StoredDocument represents a stored document
type StoredDocument struct {
	ID        string
	Title     string
	Content   string
	Tags      []string
	Data      interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewDocumentStore creates a new document store
func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		docs:  make(map[string]*StoredDocument),
		index: NewInvertedIndex(),
	}
}

// Add adds a document
func (ds *DocumentStore) Add(doc *StoredDocument) string {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if doc.ID == "" {
		doc.ID = generateID()
	}
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt

	ds.docs[doc.ID] = doc

	// Index content
	content := doc.Title + " " + doc.Content + " " + FastJoin(doc.Tags, " ")
	ds.index.AddDocument(content, doc.ID)

	return doc.ID
}

// Get retrieves a document
func (ds *DocumentStore) Get(id string) (*StoredDocument, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	doc, ok := ds.docs[id]
	return doc, ok
}

// Search searches documents
func (ds *DocumentStore) Search(query string, limit int) []SearchResult {
	results := ds.index.Search(query, limit)

	// Enrich results with document data
	for i := range results {
		if docID, ok := results[i].Data.(string); ok {
			if doc, ok := ds.Get(docID); ok {
				results[i].Data = doc
			}
		}
	}

	return results
}

// generateID generates a unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
