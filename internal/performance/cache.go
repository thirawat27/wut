// Package performance provides high-performance caching solutions
package performance

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
)

// LRUCache is a high-performance thread-safe LRU cache
// Uses sharding to reduce lock contention
type LRUCache[K comparable, V any] struct {
	shards     []*cacheShard[K, V]
	shardCount uint64
	shardMask  uint64
}

// cacheShard is a single shard of the LRU cache
type cacheShard[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]*cacheEntry[V]
	head     *cacheEntry[V]
	tail     *cacheEntry[V]
	capacity int
	size     int
}

// cacheEntry is a single cache entry with LRU list pointers
type cacheEntry[V any] struct {
	key        any
	value      V
	expiresAt  int64
	prev       *cacheEntry[V]
	next       *cacheEntry[V]
	accessFreq atomic.Uint32
}

// NewLRUCache creates a new sharded LRU cache
func NewLRUCache[K comparable, V any](capacity, shards int) *LRUCache[K, V] {
	if shards <= 0 {
		shards = 32
	}
	// Round up to power of 2
	shardCount := 1
	for shardCount < shards {
		shardCount <<= 1
	}

	c := &LRUCache[K, V]{
		shards:     make([]*cacheShard[K, V], shardCount),
		shardCount: uint64(shardCount),
		shardMask:  uint64(shardCount - 1),
	}

	perShard := max(capacity/shardCount, 16)

	for i := 0; i < shardCount; i++ {
		c.shards[i] = newCacheShard[K, V](perShard)
	}

	return c
}

// newCacheShard creates a new cache shard
func newCacheShard[K comparable, V any](capacity int) *cacheShard[K, V] {
	return &cacheShard[K, V]{
		items:    make(map[K]*cacheEntry[V], capacity),
		capacity: capacity,
	}
}

// getShard returns the shard for a given key
func (c *LRUCache[K, V]) getShard(key K) *cacheShard[K, V] {
	h := fastHash64(key)
	return c.shards[h&c.shardMask]
}

// fastHash64 computes an ultra-fast xxhash for a key
func fastHash64[K comparable](key K) uint64 {
	switch k := any(key).(type) {
	case string:
		return xxhash.Sum64String(k)
	case []byte:
		return xxhash.Sum64(k)
	case int:
		return xxhash.Sum64([]byte{byte(k), byte(k >> 8), byte(k >> 16), byte(k >> 24),
			byte(k >> 32), byte(k >> 40), byte(k >> 48), byte(k >> 56)})
	case uint64:
		return xxhash.Sum64([]byte{byte(k), byte(k >> 8), byte(k >> 16), byte(k >> 24),
			byte(k >> 32), byte(k >> 40), byte(k >> 48), byte(k >> 56)})
	default:
		// For other types, use stringification
		return xxhash.Sum64String(fmt.Sprintf("%v", key))
	}
}

// Get retrieves a value from the cache
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	shard := c.getShard(key)
	return shard.get(key)
}

// Set adds or updates a value in the cache
func (c *LRUCache[K, V]) Set(key K, value V, ttl time.Duration) {
	shard := c.getShard(key)
	shard.set(key, value, ttl)
}

// Delete removes a value from the cache
func (c *LRUCache[K, V]) Delete(key K) {
	shard := c.getShard(key)
	shard.delete(key)
}

// Len returns the total number of items in the cache
func (c *LRUCache[K, V]) Len() int {
	total := 0
	for _, shard := range c.shards {
		shard.mu.RLock()
		total += shard.size
		shard.mu.RUnlock()
	}
	return total
}

// Clear removes all items from the cache
func (c *LRUCache[K, V]) Clear() {
	for _, shard := range c.shards {
		shard.clear()
	}
}

// get retrieves a value from the shard
func (s *cacheShard[K, V]) get(key K) (V, bool) {
	s.mu.RLock()
	entry, exists := s.items[key]
	s.mu.RUnlock()

	if !exists {
		var zero V
		return zero, false
	}

	// Check expiration
	if entry.expiresAt > 0 && time.Now().UnixNano() > entry.expiresAt {
		s.mu.Lock()
		// Double-check after acquiring write lock
		if e, ok := s.items[key]; ok && e == entry {
			s.removeEntry(entry)
			delete(s.items, key)
			s.size--
		}
		s.mu.Unlock()
		var zero V
		return zero, false
	}

	// Update access frequency
	entry.accessFreq.Add(1)

	// Move to front (MRU position) - need full lock
	s.mu.Lock()
	// Double-check entry still exists after acquiring lock
	if _, stillExists := s.items[key]; stillExists && entry == s.items[key] {
		s.moveToFront(entry)
	}
	s.mu.Unlock()

	return entry.value, true
}

// set adds or updates a value in the shard
func (s *cacheShard[K, V]) set(key K, value V, ttl time.Duration) {
	expiresAt := int64(0)
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).UnixNano()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.items[key]; exists {
		// Update existing entry
		entry.value = value
		entry.expiresAt = expiresAt
		entry.accessFreq.Store(0)
		s.moveToFront(entry)
		return
	}

	// Create new entry
	entry := &cacheEntry[V]{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}

	// Evict oldest if at capacity
	if s.size >= s.capacity {
		s.evictOldest()
	}

	// Add to front
	s.addToFront(entry)
	s.items[key] = entry
	s.size++
}

// delete removes a value from the shard
func (s *cacheShard[K, V]) delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.items[key]; exists {
		s.removeEntry(entry)
		delete(s.items, key)
		s.size--
	}
}

// clear removes all items from the shard
func (s *cacheShard[K, V]) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[K]*cacheEntry[V], s.capacity)
	s.head = nil
	s.tail = nil
	s.size = 0
}

// addToFront adds an entry to the front of the list
func (s *cacheShard[K, V]) addToFront(entry *cacheEntry[V]) {
	entry.next = s.head
	entry.prev = nil
	if s.head != nil {
		s.head.prev = entry
	}
	s.head = entry
	if s.tail == nil {
		s.tail = entry
	}
}

// removeEntry removes an entry from the list
func (s *cacheShard[K, V]) removeEntry(entry *cacheEntry[V]) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		s.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		s.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

// moveToFront moves an entry to the front of the list
func (s *cacheShard[K, V]) moveToFront(entry *cacheEntry[V]) {
	if entry == s.head {
		return
	}
	s.removeEntry(entry)
	s.addToFront(entry)
}

// evictOldest removes the oldest entry
func (s *cacheShard[K, V]) evictOldest() {
	if s.tail == nil {
		return
	}
	oldest := s.tail

	// Check for expired entries first
	now := time.Now().UnixNano()
	for oldest != nil && oldest.expiresAt > 0 && now > oldest.expiresAt {
		key := oldest.key.(K)
		delete(s.items, key)
		s.removeEntry(oldest)
		s.size--
		oldest = s.tail
	}

	// If no expired entries, remove LRU
	if s.size >= s.capacity && s.tail != nil {
		key := s.tail.key.(K)
		delete(s.items, key)
		s.removeEntry(s.tail)
		s.size--
	}
}

// StringCache is a specialized cache for string keys with high performance
type StringCache[V any] struct {
	*LRUCache[string, V]
}

// NewStringCache creates a new string cache
func NewStringCache[V any](capacity, shards int) *StringCache[V] {
	return &StringCache[V]{
		LRUCache: NewLRUCache[string, V](capacity, shards),
	}
}

// ComputedCache is a cache with automatic value computation
type ComputedCache[K comparable, V any] struct {
	cache *LRUCache[K, V]
	mu    sync.RWMutex
	fn    func(K) (V, error)
}

// NewComputedCache creates a new computed cache
func NewComputedCache[K comparable, V any](capacity int, computeFn func(K) (V, error)) *ComputedCache[K, V] {
	return &ComputedCache[K, V]{
		cache: NewLRUCache[K, V](capacity, 32),
		fn:    computeFn,
	}
}

// GetOrCompute retrieves a value, computing it if necessary
func (c *ComputedCache[K, V]) GetOrCompute(key K) (V, error) {
	// Try cache first
	if value, ok := c.cache.Get(key); ok {
		return value, nil
	}

	// Compute value
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring lock
	if value, ok := c.cache.Get(key); ok {
		return value, nil
	}

	value, err := c.fn(key)
	if err != nil {
		var zero V
		return zero, err
	}

	c.cache.Set(key, value, 0)
	return value, nil
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits      atomic.Uint64
	Misses    atomic.Uint64
	Evictions atomic.Uint64
	Size      int
	Capacity  int
}

// HitRate returns the cache hit rate
func (s *CacheStats) HitRate() float64 {
	hits := s.Hits.Load()
	misses := s.Misses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// StatsCache is a cache with statistics tracking
type StatsCache[K comparable, V any] struct {
	*LRUCache[K, V]
	stats CacheStats
}

// NewStatsCache creates a new cache with statistics
func NewStatsCache[K comparable, V any](capacity, shards int) *StatsCache[K, V] {
	return &StatsCache[K, V]{
		LRUCache: NewLRUCache[K, V](capacity, shards),
	}
}

// Get retrieves a value and updates statistics
func (c *StatsCache[K, V]) Get(key K) (V, bool) {
	value, ok := c.LRUCache.Get(key)
	if ok {
		c.stats.Hits.Add(1)
	} else {
		c.stats.Misses.Add(1)
	}
	return value, ok
}

// Stats returns cache statistics
func (c *StatsCache[K, V]) Stats() *CacheStats {
	return &CacheStats{
		Hits:      atomic.Uint64{}, // Use zero value, caller should use HitRate()
		Misses:    atomic.Uint64{},
		Evictions: atomic.Uint64{},
		Size:      c.Len(),
	}
}

// GetHits returns the number of cache hits
func (c *StatsCache[K, V]) GetHits() uint64 {
	return c.stats.Hits.Load()
}

// GetMisses returns the number of cache misses
func (c *StatsCache[K, V]) GetMisses() uint64 {
	return c.stats.Misses.Load()
}

// GetEvictions returns the number of cache evictions
func (c *StatsCache[K, V]) GetEvictions() uint64 {
	return c.stats.Evictions.Load()
}
