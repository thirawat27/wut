// Package performance provides high-performance concurrency primitives
package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work
type Task interface {
	Execute(ctx context.Context) error
}

// TaskFunc is a function that implements Task
type TaskFunc func(ctx context.Context) error

// Execute implements Task interface
func (f TaskFunc) Execute(ctx context.Context) error {
	return f(ctx)
}

// WorkerPool is a high-performance worker pool with lock-free operations
type WorkerPool struct {
	taskQueue    *RingBuffer[taskWrapper]
	workers      int
	active       atomic.Bool
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	panicHandler func(any)
}

type taskWrapper struct {
	task Task
	done chan error
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers, queueSize int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if queueSize <= 0 {
		queueSize = workers * 64
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		taskQueue: NewRingBuffer[taskWrapper](queueSize),
		workers:   workers,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SetPanicHandler sets a handler for panics in workers
func (p *WorkerPool) SetPanicHandler(handler func(any)) {
	p.panicHandler = handler
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	if !p.active.CompareAndSwap(false, true) {
		return
	}

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	if !p.active.CompareAndSwap(true, false) {
		return
	}

	p.cancel()
	p.wg.Wait()
}

// Submit submits a task to the pool
// Returns true if submitted, false if queue is full
func (p *WorkerPool) Submit(task Task) bool {
	if !p.active.Load() {
		return false
	}

	wrapper := taskWrapper{
		task: task,
	}
	return p.taskQueue.TryPush(wrapper)
}

// SubmitWait submits a task and waits for completion
func (p *WorkerPool) SubmitWait(task Task) error {
	if !p.active.Load() {
		return context.Canceled
	}

	done := make(chan error, 1)
	wrapper := taskWrapper{
		task: task,
		done: done,
	}

	if !p.taskQueue.TryPush(wrapper) {
		return context.DeadlineExceeded
	}

	select {
	case err := <-done:
		return err
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// worker is the worker goroutine
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	if p.panicHandler != nil {
		defer func() {
			if r := recover(); r != nil {
				p.panicHandler(r)
			}
		}()
	}

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// Try to get task
		wrapper, ok := p.taskQueue.TryPop()
		if !ok {
			runtime.Gosched()
			continue
		}

		// Execute task
		err := wrapper.task.Execute(p.ctx)

		// Send result if channel exists
		if wrapper.done != nil {
			select {
			case wrapper.done <- err:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// IsActive returns true if pool is active
func (p *WorkerPool) IsActive() bool {
	return p.active.Load()
}

// QueueSize returns the current queue size
func (p *WorkerPool) QueueSize() uint64 {
	return p.taskQueue.Len()
}

// Stats holds pool statistics
type PoolStats struct {
	Workers   int
	QueueSize uint64
	Active    bool
}

// Stats returns pool statistics
func (p *WorkerPool) Stats() PoolStats {
	return PoolStats{
		Workers:   p.workers,
		QueueSize: p.taskQueue.Len(),
		Active:    p.active.Load(),
	}
}

// AdaptivePool is a worker pool that adjusts size based on load
type AdaptivePool struct {
	minWorkers  int
	maxWorkers  int
	current     atomic.Int32
	taskQueue   chan Task
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	idleTimeout time.Duration
}

// NewAdaptivePool creates an adaptive worker pool
func NewAdaptivePool(minWorkers, maxWorkers int, queueSize int, idleTimeout time.Duration) *AdaptivePool {
	if minWorkers <= 0 {
		minWorkers = 1
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &AdaptivePool{
		minWorkers:  minWorkers,
		maxWorkers:  maxWorkers,
		taskQueue:   make(chan Task, queueSize),
		ctx:         ctx,
		cancel:      cancel,
		idleTimeout: idleTimeout,
	}

	// Start minimum workers
	for i := 0; i < minWorkers; i++ {
		pool.addWorker()
	}

	return pool
}

// Submit submits a task
func (p *AdaptivePool) Submit(task Task) bool {
	select {
	case p.taskQueue <- task:
		// Check if we need more workers
		p.scaleUp()
		return true
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
}

// scaleUp adds workers if needed
func (p *AdaptivePool) scaleUp() {
	current := int(p.current.Load())
	queueLen := len(p.taskQueue)

	// Add workers if queue is filling up
	if queueLen > cap(p.taskQueue)/2 && current < p.maxWorkers {
		p.addWorker()
	}
}

// addWorker adds a new worker
func (p *AdaptivePool) addWorker() {
	p.mu.Lock()
	if int(p.current.Load()) >= p.maxWorkers {
		p.mu.Unlock()
		return
	}
	p.current.Add(1)
	p.mu.Unlock()

	p.wg.Add(1)
	go p.worker()
}

// worker is the adaptive worker
func (p *AdaptivePool) worker() {
	defer p.wg.Done()
	defer p.current.Add(-1)

	idleTimer := time.NewTimer(p.idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return

		case task := <-p.taskQueue:
			idleTimer.Reset(p.idleTimeout)
			_ = task.Execute(p.ctx)

		case <-idleTimer.C:
			// Check if we can scale down
			if int(p.current.Load()) > p.minWorkers {
				return
			}
			idleTimer.Reset(p.idleTimeout)
		}
	}
}

// Stop stops the adaptive pool
func (p *AdaptivePool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// CurrentWorkers returns current worker count
func (p *AdaptivePool) CurrentWorkers() int {
	return int(p.current.Load())
}

// PriorityPool is a worker pool with task priorities
type PriorityPool struct {
	highPriority   chan Task
	mediumPriority chan Task
	lowPriority    chan Task
	workers        int
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewPriorityPool creates a priority-based worker pool
func NewPriorityPool(workers, queueSizePerPriority int) *PriorityPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &PriorityPool{
		highPriority:   make(chan Task, queueSizePerPriority),
		mediumPriority: make(chan Task, queueSizePerPriority),
		lowPriority:    make(chan Task, queueSizePerPriority),
		workers:        workers,
		ctx:            ctx,
		cancel:         cancel,
	}

	for range workers {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// SubmitHigh submits a high-priority task
func (p *PriorityPool) SubmitHigh(task Task) bool {
	select {
	case p.highPriority <- task:
		return true
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
}

// SubmitMedium submits a medium-priority task
func (p *PriorityPool) SubmitMedium(task Task) bool {
	select {
	case p.mediumPriority <- task:
		return true
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
}

// SubmitLow submits a low-priority task
func (p *PriorityPool) SubmitLow(task Task) bool {
	select {
	case p.lowPriority <- task:
		return true
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
}

// worker processes tasks by priority
func (p *PriorityPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return

		case task := <-p.highPriority:
			_ = task.Execute(p.ctx)

		default:
			select {
			case <-p.ctx.Done():
				return

			case task := <-p.highPriority:
				_ = task.Execute(p.ctx)

			case task := <-p.mediumPriority:
				_ = task.Execute(p.ctx)

			default:
				select {
				case <-p.ctx.Done():
					return

				case task := <-p.highPriority:
					_ = task.Execute(p.ctx)

				case task := <-p.mediumPriority:
					_ = task.Execute(p.ctx)

				case task := <-p.lowPriority:
					_ = task.Execute(p.ctx)
				}
			}
		}
	}
}

// Stop stops the priority pool
func (p *PriorityPool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// ParallelMap executes a function over a slice in parallel
func ParallelMap[T any, R any](items []T, fn func(T) (R, error), maxWorkers int) ([]R, error) {
	if len(items) == 0 {
		return []R{}, nil
	}

	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}
	if maxWorkers > len(items) {
		maxWorkers = len(items)
	}

	results := make([]R, len(items))
	errChan := make(chan error, len(items))

	// Work channel
	work := make(chan int, len(items))
	for i := range items {
		work <- i
	}
	close(work)

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Go(func() {
			for idx := range work {
				result, err := fn(items[idx])
				if err != nil {
					select {
					case errChan <- err:
					default:
					}
				} else {
					results[idx] = result
				}
			}
		})
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, errs[0]
	}
	return results, nil
}

// ParallelForEach executes a function for each item in parallel
func ParallelForEach[T any](items []T, fn func(T) error, maxWorkers int) error {
	_, err := ParallelMap(items, func(item T) (struct{}, error) {
		return struct{}{}, fn(item)
	}, maxWorkers)
	return err
}

// ParallelFilter filters items in parallel
func ParallelFilter[T any](items []T, predicate func(T) bool, maxWorkers int) []T {
	type result struct {
		item T
		keep bool
	}

	results, _ := ParallelMap(items, func(item T) (result, error) {
		return result{item: item, keep: predicate(item)}, nil
	}, maxWorkers)

	filtered := make([]T, 0, len(items))
	for _, r := range results {
		if r.keep {
			filtered = append(filtered, r.item)
		}
	}
	return filtered
}

// Pipeline represents a processing pipeline
type Pipeline[T any] struct {
	stages []PipelineStage[T]
}

// PipelineStage represents a pipeline stage
type PipelineStage[T any] func(ctx context.Context, input T) (T, error)

// NewPipeline creates a new pipeline
func NewPipeline[T any](stages ...PipelineStage[T]) *Pipeline[T] {
	return &Pipeline[T]{stages: stages}
}

// Process processes an item through all stages
func (p *Pipeline[T]) Process(ctx context.Context, input T) (T, error) {
	var err error
	result := input

	for _, stage := range p.stages {
		result, err = stage(ctx, result)
		if err != nil {
			var zero T
			return zero, err
		}
	}

	return result, nil
}

// ProcessBatch processes multiple items through the pipeline
func (p *Pipeline[T]) ProcessBatch(ctx context.Context, items []T, workers int) ([]T, error) {
	return ParallelMap(items, func(item T) (T, error) {
		return p.Process(ctx, item)
	}, workers)
}

// RateLimiter provides token bucket rate limiting
type RateLimiter struct {
	tokens     atomic.Int64
	maxTokens  int64
	interval   time.Duration
	lastRefill atomic.Int64
}

// NewRateLimiter creates a rate limiter
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		maxTokens: int64(rate),
		interval:  interval,
	}
}

// Allow checks if operation is allowed
func (rl *RateLimiter) Allow() bool {
	now := time.Now().UnixNano()
	last := rl.lastRefill.Load()
	elapsed := now - last

	if elapsed > rl.interval.Nanoseconds() {
		// Refill tokens
		if rl.lastRefill.CompareAndSwap(last, now) {
			rl.tokens.Store(rl.maxTokens)
		}
	}

	for {
		tokens := rl.tokens.Load()
		if tokens <= 0 {
			return false
		}
		if rl.tokens.CompareAndSwap(tokens, tokens-1) {
			return true
		}
	}
}

// Wait waits for permission
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rl.interval / 10):
		}
	}
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	failures    atomic.Int32
	threshold   int32
	timeout     time.Duration
	lastFailure atomic.Int64
	state       atomic.Int32 // 0: closed, 1: open, 2: half-open
}

// NewCircuitBreaker creates a circuit breaker
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: int32(threshold),
		timeout:   timeout,
	}
}

// State returns current state: 0=closed, 1=open, 2=half-open
func (cb *CircuitBreaker) State() int32 {
	state := cb.state.Load()
	if state == 1 {
		// Check if timeout has passed
		lastFailure := cb.lastFailure.Load()
		if time.Since(time.Unix(0, lastFailure)) > cb.timeout {
			if cb.state.CompareAndSwap(1, 2) {
				cb.failures.Store(0)
				return 2
			}
		}
	}
	return cb.state.Load()
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	state := cb.State()
	if state == 1 {
		return fmt.Errorf("circuit breaker open")
	}

	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) recordFailure() {
	failures := cb.failures.Add(1)
	cb.lastFailure.Store(time.Now().UnixNano())
	if failures >= cb.threshold {
		cb.state.Store(1)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.failures.Store(0)
	cb.state.Store(0)
}
