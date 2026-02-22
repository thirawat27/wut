// Package performance provides high-performance database operations
package performance

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/bbolt"
)

const (
	// DefaultMaxBatchSize is the maximum number of operations per batch
	DefaultMaxBatchSize = 1000
	// DefaultBatchTimeout is the maximum time to wait before flushing a batch
	DefaultBatchTimeout = 50 * time.Millisecond
	// DefaultWriteBuffer is the write buffer size
	DefaultWriteBuffer = 64 * 1024 // 64KB
)

// OptimizedStorage provides high-performance storage with batching and caching
type OptimizedStorage struct {
	db          *bbolt.DB
	path        string
	writeQueue  chan *writeOp
	readCache   *LRUCache[string, []byte]
	batchConfig BatchConfig
	stats       StorageStats
	closed      atomic.Bool
}

// BatchConfig holds batch processing configuration
type BatchConfig struct {
	MaxSize   int
	Timeout   time.Duration
	Enabled   bool
	WriteBuffer int
}

// writeOp represents a write operation
type writeOp struct {
	bucket  string
	key     []byte
	value   []byte
	done    chan error
	isDelete bool
}

// StorageStats holds storage statistics
type StorageStats struct {
	Writes      atomic.Uint64
	Reads       atomic.Uint64
	Batches     atomic.Uint64
	CacheHits   atomic.Uint64
	CacheMisses atomic.Uint64
	Errors      atomic.Uint64
}

// NewOptimizedStorage creates a new optimized storage
func NewOptimizedStorage(dbPath string) (*OptimizedStorage, error) {
	return NewOptimizedStorageWithConfig(dbPath, BatchConfig{
		MaxSize:     DefaultMaxBatchSize,
		Timeout:     DefaultBatchTimeout,
		Enabled:     true,
		WriteBuffer: DefaultWriteBuffer,
	})
}

// NewOptimizedStorageWithConfig creates storage with custom config
func NewOptimizedStorageWithConfig(dbPath string, config BatchConfig) (*OptimizedStorage, error) {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout:      1 * time.Second,
		NoGrowSync:   false,
		PageSize:     4096,
		NoSync:       false, // Keep data safety
		FreelistType: bbolt.FreelistMapType, // Better for high write throughput
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &OptimizedStorage{
		db:          db,
		path:        dbPath,
		writeQueue:  make(chan *writeOp, config.MaxSize*2),
		readCache:   NewLRUCache[string, []byte](10000, 32),
		batchConfig: config,
	}

	// Start batch processor
	if config.Enabled {
		go s.batchProcessor()
	}

	return s, nil
}

// batchProcessor processes write operations in batches
func (s *OptimizedStorage) batchProcessor() {
	ticker := time.NewTicker(s.batchConfig.Timeout)
	defer ticker.Stop()

	batch := make([]*writeOp, 0, s.batchConfig.MaxSize)

	for {
		select {
		case op, ok := <-s.writeQueue:
			if !ok {
				// Process remaining batch
				if len(batch) > 0 {
					s.flushBatch(batch)
				}
				return
			}
			batch = append(batch, op)

			if len(batch) >= s.batchConfig.MaxSize {
				s.flushBatch(batch)
				batch = batch[:0]
				ticker.Reset(s.batchConfig.Timeout)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch executes a batch of write operations
func (s *OptimizedStorage) flushBatch(batch []*writeOp) {
	if len(batch) == 0 {
		return
	}

	// Group operations by bucket
	buckets := make(map[string][]*writeOp)
	for _, op := range batch {
		buckets[op.bucket] = append(buckets[op.bucket], op)
	}

	// Execute batch
	err := s.db.Update(func(tx *bbolt.Tx) error {
		for bucketName, ops := range buckets {
			bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
			if err != nil {
				return err
			}

			for _, op := range ops {
				if op.isDelete {
					if err := bucket.Delete(op.key); err != nil {
						return err
					}
				} else {
					if err := bucket.Put(op.key, op.value); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})

	// Notify callers
	for _, op := range batch {
		if op.done != nil {
			op.done <- err
		}
	}

	s.stats.Batches.Add(1)
	s.stats.Writes.Add(uint64(len(batch)))

	if err != nil {
		s.stats.Errors.Add(1)
	}
}

// Put stores a value (async if batching enabled)
func (s *OptimizedStorage) Put(bucket string, key, value []byte) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	if !s.batchConfig.Enabled {
		return s.putSync(bucket, key, value)
	}

	op := &writeOp{
		bucket: bucket,
		key:    key,
		value:  value,
		done:   make(chan error, 1),
	}

	select {
	case s.writeQueue <- op:
		return <-op.done
	case <-time.After(5 * time.Second):
		return fmt.Errorf("write queue full")
	}
}

// putSync performs synchronous put
func (s *OptimizedStorage) putSync(bucket string, key, value []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		return b.Put(key, value)
	})
}

// Get retrieves a value with caching
func (s *OptimizedStorage) Get(bucket string, key []byte) ([]byte, error) {
	if s.closed.Load() {
		return nil, fmt.Errorf("storage closed")
	}

	s.stats.Reads.Add(1)

	// Build cache key
	cacheKey := bucket + ":" + string(key)

	// Check cache
	if value, ok := s.readCache.Get(cacheKey); ok {
		s.stats.CacheHits.Add(1)
		return value, nil
	}
	s.stats.CacheMisses.Add(1)

	// Read from database
	var value []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}
		v := b.Get(key)
		if v == nil {
			return fmt.Errorf("key not found")
		}
		// Copy value (bbolt bytes are only valid inside transaction)
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Update cache
	s.readCache.Set(cacheKey, value, 5*time.Minute)

	return value, nil
}

// Delete removes a value
func (s *OptimizedStorage) Delete(bucket string, key []byte) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	// Remove from cache
	cacheKey := bucket + ":" + string(key)
	s.readCache.Delete(cacheKey)

	if !s.batchConfig.Enabled {
		return s.deleteSync(bucket, key)
	}

	op := &writeOp{
		bucket:   bucket,
		key:      key,
		isDelete: true,
		done:     make(chan error, 1),
	}

	select {
	case s.writeQueue <- op:
		return <-op.done
	case <-time.After(5 * time.Second):
		return fmt.Errorf("write queue full")
	}
}

// deleteSync performs synchronous delete
func (s *OptimizedStorage) deleteSync(bucket string, key []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.Delete(key)
	})
}

// GetBatch retrieves multiple values in a single transaction
func (s *OptimizedStorage) GetBatch(bucket string, keys [][]byte) (map[string][]byte, error) {
	if s.closed.Load() {
		return nil, fmt.Errorf("storage closed")
	}

	results := make(map[string][]byte, len(keys))

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}

		for _, key := range keys {
			value := b.Get(key)
			if value != nil {
				v := make([]byte, len(value))
				copy(v, value)
				results[string(key)] = v
			}
		}
		return nil
	})

	return results, err
}

// PutBatch stores multiple values in a single transaction
func (s *OptimizedStorage) PutBatch(bucket string, items map[string][]byte) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}

		for key, value := range items {
			if err := b.Put([]byte(key), value); err != nil {
				return err
			}

			// Update cache
			cacheKey := bucket + ":" + key
			s.readCache.Set(cacheKey, value, 5*time.Minute)
		}
		return nil
	})
}

// Scan iterates over all keys in a bucket
func (s *OptimizedStorage) Scan(bucket string, fn func(key, value []byte) bool) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	return s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			if !fn(k, v) {
				return fmt.Errorf("scan stopped")
			}
			return nil
		})
	})
}

// ScanPrefix iterates over keys with given prefix
func (s *OptimizedStorage) ScanPrefix(bucket string, prefix []byte, fn func(key, value []byte) bool) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	return s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}

		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && fastHasPrefixASCII(string(k), string(prefix)); k, v = c.Next() {
			if !fn(k, v) {
				return fmt.Errorf("scan stopped")
			}
		}
		return nil
	})
}

// Stats returns storage statistics as individual values
func (s *OptimizedStorage) Stats() (writes, reads, batches, cacheHits, cacheMisses, errors uint64) {
	return s.stats.Writes.Load(),
		s.stats.Reads.Load(),
		s.stats.Batches.Load(),
		s.stats.CacheHits.Load(),
		s.stats.CacheMisses.Load(),
		s.stats.Errors.Load()
}

// CacheStats returns cache statistics
func (s *OptimizedStorage) CacheStats() (hits, misses uint64) {
	return s.stats.CacheHits.Load(), s.stats.CacheMisses.Load()
}

// Close closes the storage
func (s *OptimizedStorage) Close() error {
	s.closed.Store(true)

	// Close write queue
	close(s.writeQueue)

	// Wait for batch processor to finish
	time.Sleep(100 * time.Millisecond)

	return s.db.Close()
}

// Sync forces a sync to disk
func (s *OptimizedStorage) Sync() error {
	return s.db.Sync()
}

// Compact compacts the database
func (s *OptimizedStorage) Compact(dstPath string) error {
	return s.db.View(func(tx *bbolt.Tx) error {
		return tx.CopyFile(dstPath, 0600)
	})
}

// PrefetchCache preloads keys into cache
func (s *OptimizedStorage) PrefetchCache(bucket string, keys []string) {
	for _, key := range keys {
		if _, err := s.Get(bucket, []byte(key)); err == nil {
			// Successfully cached
		}
	}
}

// ClearCache clears the read cache
func (s *OptimizedStorage) ClearCache() {
	s.readCache.Clear()
}

// JSONStorage provides JSON-specific storage operations
type JSONStorage struct {
	*OptimizedStorage
}

// NewJSONStorage creates a new JSON storage
func NewJSONStorage(dbPath string) (*JSONStorage, error) {
	storage, err := NewOptimizedStorage(dbPath)
	if err != nil {
		return nil, err
	}
	return &JSONStorage{OptimizedStorage: storage}, nil
}

// GetJSON retrieves and unmarshals a JSON value
func (s *JSONStorage) GetJSON(bucket string, key []byte, v interface{}) error {
	data, err := s.Get(bucket, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// PutJSON marshals and stores a value
func (s *JSONStorage) PutJSON(bucket string, key []byte, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Put(bucket, key, data)
}

// DBConnectionPool manages database connections
type DBConnectionPool struct {
	mu       sync.RWMutex
	storages map[string]*OptimizedStorage
	maxConns int
}

// NewDBConnectionPool creates a new connection pool
func NewDBConnectionPool(maxConns int) *DBConnectionPool {
	return &DBConnectionPool{
		storages: make(map[string]*OptimizedStorage),
		maxConns: maxConns,
	}
}

// Get gets or creates a storage connection
func (p *DBConnectionPool) Get(path string) (*OptimizedStorage, error) {
	p.mu.RLock()
	if s, ok := p.storages[path]; ok {
		p.mu.RUnlock()
		return s, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check
	if s, ok := p.storages[path]; ok {
		return s, nil
	}

	// Create new connection
	s, err := NewOptimizedStorage(path)
	if err != nil {
		return nil, err
	}

	p.storages[path] = s
	return s, nil
}

// Close closes all connections
func (p *DBConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range p.storages {
		s.Close()
	}
	p.storages = make(map[string]*OptimizedStorage)
}

// Remove removes a connection from pool
func (p *DBConnectionPool) Remove(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if s, ok := p.storages[path]; ok {
		s.Close()
		delete(p.storages, path)
	}
}
