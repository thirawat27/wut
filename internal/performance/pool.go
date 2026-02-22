// Package performance provides high-performance utilities for WUT
// Focus on zero-allocation patterns and memory efficiency
package performance

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"
)

// BufferPool provides a pool of reusable bytes.Buffer objects
// Reduces GC pressure for string building operations
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (p *BufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf *bytes.Buffer) {
	// Limit max buffer size to prevent memory bloat
	if buf.Cap() > 1024*1024 { // 1MB
		return // Let GC collect large buffers
	}
	p.pool.Put(buf)
}

// StringPool provides a pool for string building operations
type StringPool struct {
	pool sync.Pool
}

// NewStringPool creates a new string pool
func NewStringPool() *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 256)
			},
		},
	}
}

// Get retrieves a byte slice from the pool
func (p *StringPool) Get() []byte {
	return p.pool.Get().([]byte)[:0]
}

// Put returns a byte slice to the pool
func (p *StringPool) Put(b []byte) {
	if cap(b) > 4096 { // 4KB
		return // Let GC collect large slices
	}
	p.pool.Put(b)
}

// ObjectPool is a generic object pool using atomic operations
// for lock-free acquire/release in hot paths
type ObjectPool[T any] struct {
	pool sync.Pool
	new  func() T
}

// NewObjectPool creates a new generic object pool
func NewObjectPool[T any](newFunc func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return newFunc()
			},
		},
		new: newFunc,
	}
}

// Get retrieves an object from the pool
func (p *ObjectPool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool
func (p *ObjectPool[T]) Put(obj T) {
	p.pool.Put(obj)
}

// RingBuffer is a lock-free circular buffer for high-throughput scenarios
type RingBuffer[T any] struct {
	buffer   []T
	head     atomic.Uint64
	tail     atomic.Uint64
	mask     uint64
	capacity uint64
}

// NewRingBuffer creates a new ring buffer with capacity rounded to power of 2
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	// Round up to power of 2
	cap := uint64(1)
	for cap < uint64(capacity) {
		cap <<= 1
	}

	return &RingBuffer[T]{
		buffer:   make([]T, cap),
		mask:     cap - 1,
		capacity: cap,
	}
}

// TryPush attempts to add an item without blocking
// Returns true if successful, false if buffer is full
func (r *RingBuffer[T]) TryPush(item T) bool {
	tail := r.tail.Load()
	head := r.head.Load()

	if tail-head >= r.capacity {
		return false // Buffer full
	}

	idx := tail & r.mask
	r.buffer[idx] = item
	r.tail.Store(tail + 1)
	return true
}

// TryPop attempts to remove an item without blocking
// Returns (item, true) if successful, (zero, false) if empty
func (r *RingBuffer[T]) TryPop() (T, bool) {
	tail := r.tail.Load()
	head := r.head.Load()

	if head >= tail {
		var zero T
		return zero, false // Buffer empty
	}

	idx := head & r.mask
	item := r.buffer[idx]
	var zero T
	r.buffer[idx] = zero // Help GC
	r.head.Store(head + 1)
	return item, true
}

// Len returns the number of items in the buffer
func (r *RingBuffer[T]) Len() uint64 {
	return r.tail.Load() - r.head.Load()
}

// IsEmpty returns true if buffer is empty
func (r *RingBuffer[T]) IsEmpty() bool {
	return r.head.Load() == r.tail.Load()
}

// IsFull returns true if buffer is full
func (r *RingBuffer[T]) IsFull() bool {
	return r.tail.Load()-r.head.Load() >= r.capacity
}

// SlicePool provides a pool for reusable slices with pre-allocated capacity
type SlicePool[T any] struct {
	pool sync.Pool
}

// NewSlicePool creates a new slice pool
func NewSlicePool[T any](capacity int) *SlicePool[T] {
	return &SlicePool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]T, 0, capacity)
			},
		},
	}
}

// Get retrieves a slice from the pool
func (p *SlicePool[T]) Get() []T {
	return p.pool.Get().([]T)[:0]
}

// Put returns a slice to the pool
func (p *SlicePool[T]) Put(s []T) {
	// Only keep reasonably sized slices
	if cap(s) > 10000 {
		return
	}
	p.pool.Put(s[:0])
}

// CacheEntry represents a cached value with expiration
type CacheEntry[T any] struct {
	Value     T
	ExpiresAt atomic.Int64
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry[T]) IsExpired() bool {
	return time.Now().UnixNano() > e.ExpiresAt.Load()
}

// FastStringBuilder provides a high-performance string builder
// using stack-allocated buffers for small strings
type FastStringBuilder struct {
	buf []byte
}

// NewFastStringBuilder creates a new fast string builder
func NewFastStringBuilder() *FastStringBuilder {
	return &FastStringBuilder{
		buf: make([]byte, 0, 256),
	}
}

// WriteString appends a string
func (b *FastStringBuilder) WriteString(s string) {
	b.buf = append(b.buf, s...)
}

// WriteByte appends a byte
func (b *FastStringBuilder) WriteByte(c byte) {
	b.buf = append(b.buf, c)
}

// Write appends bytes
func (b *FastStringBuilder) Write(p []byte) {
	b.buf = append(b.buf, p...)
}

// String returns the built string
func (b *FastStringBuilder) String() string {
	return string(b.buf)
}

// Bytes returns the built bytes
func (b *FastStringBuilder) Bytes() []byte {
	return b.buf
}

// Reset clears the builder
func (b *FastStringBuilder) Reset() {
	b.buf = b.buf[:0]
}

// Len returns the length
func (b *FastStringBuilder) Len() int {
	return len(b.buf)
}

// Cap returns the capacity
func (b *FastStringBuilder) Cap() int {
	return cap(b.buf)
}

// PreallocStringBuilder pre-allocates capacity for known sizes
type PreallocStringBuilder struct {
	buf []byte
}

// NewPreallocStringBuilder creates a builder with pre-allocated capacity
func NewPreallocStringBuilder(capacity int) *PreallocStringBuilder {
	return &PreallocStringBuilder{
		buf: make([]byte, 0, capacity),
	}
}

// WriteString appends a string
func (b *PreallocStringBuilder) WriteString(s string) {
	b.buf = append(b.buf, s...)
}

// String returns the built string
func (b *PreallocStringBuilder) String() string {
	return string(b.buf)
}

// Bytes returns the built bytes
func (b *PreallocStringBuilder) Bytes() []byte {
	return b.buf
}

// Global pools for common use cases
var (
	// GlobalBufferPool is the global buffer pool
	GlobalBufferPool = NewBufferPool()

	// GlobalStringPool is the global string pool
	GlobalStringPool = NewStringPool()

	// GlobalByteSlicePool is the global byte slice pool (4KB)
	GlobalByteSlicePool = NewSlicePool[byte](4096)

	// GlobalStringSlicePool is the global string slice pool
	GlobalStringSlicePool = NewSlicePool[string](64)
)

// AcquireBuffer gets a buffer from the global pool
func AcquireBuffer() *bytes.Buffer {
	return GlobalBufferPool.Get()
}

// ReleaseBuffer returns a buffer to the global pool
func ReleaseBuffer(buf *bytes.Buffer) {
	GlobalBufferPool.Put(buf)
}

// AcquireByteSlice gets a byte slice from the global pool
func AcquireByteSlice() []byte {
	return GlobalByteSlicePool.Get()
}

// ReleaseByteSlice returns a byte slice to the global pool
func ReleaseByteSlice(b []byte) {
	GlobalByteSlicePool.Put(b)
}
