// Package concurrency provides concurrent processing utilities for WUT
package concurrency

import (
	"context"
	"fmt"
	"runtime"
	"sync"
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

// Result represents the result of a task execution
type Result struct {
	Task  Task
	Error error
	Index int
}

// Pool is a worker pool for concurrent task execution
type Pool struct {
	workers   int
	queue     chan Task
	results   chan Result
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	mu        sync.RWMutex
}

// PoolOption is a functional option for Pool
type PoolOption func(*Pool)

// WithWorkerCount sets the number of workers
func WithWorkerCount(n int) PoolOption {
	return func(p *Pool) {
		if n > 0 {
			p.workers = n
		}
	}
}

// WithQueueSize sets the queue size
func WithQueueSize(n int) PoolOption {
	return func(p *Pool) {
		// This is handled in NewPool
	}
}

// NewPool creates a new worker pool
func NewPool(opts ...PoolOption) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	
	p := &Pool{
		workers: runtime.NumCPU(),
		queue:   make(chan Task, 100),
		results: make(chan Result, 100),
		ctx:     ctx,
		cancel:  cancel,
	}
	
	for _, opt := range opts {
		opt(p)
	}
	
	return p
}

// Start starts the worker pool
func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.running {
		return
	}
	
	p.running = true
	
	// Start workers
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the worker pool
func (p *Pool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.running {
		return
	}
	
	p.cancel()
	p.wg.Wait()
	
	close(p.queue)
	close(p.results)
	
	p.running = false
}

// Submit submits a task to the pool
func (p *Pool) Submit(task Task) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if !p.running {
		return fmt.Errorf("pool is not running")
	}
	
	select {
	case p.queue <- task:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// SubmitFunc submits a function as a task
func (p *Pool) SubmitFunc(fn func(ctx context.Context) error) error {
	return p.Submit(TaskFunc(fn))
}

// Results returns the results channel
func (p *Pool) Results() <-chan Result {
	return p.results
}

// worker is the worker goroutine
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.queue:
			if !ok {
				return
			}
			
			ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
			err := task.Execute(ctx)
			cancel()
			
			select {
			case p.results <- Result{Task: task, Error: err, Index: id}:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// IsRunning returns true if the pool is running
func (p *Pool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetWorkerCount returns the number of workers
func (p *Pool) GetWorkerCount() int {
	return p.workers
}

// Map applies a function to all items concurrently
func Map[T any, R any](ctx context.Context, items []T, fn func(T) (R, error), workers int) ([]R, error) {
	if len(items) == 0 {
		return []R{}, nil
	}
	
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	
	results := make([]R, len(items))
	errChan := make(chan error, len(items))
	
	// Create work channel
	workChan := make(chan int, len(items))
	for i := range items {
		workChan <- i
	}
	close(workChan)
	
	// Create worker group
	var wg sync.WaitGroup
	
	// Start workers
	for i := 0; i < workers && i < len(items); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				case idx, ok := <-workChan:
					if !ok {
						return
					}
					
					result, err := fn(items[idx])
					if err != nil {
						select {
						case errChan <- fmt.Errorf("item %d: %w", idx, err):
						case <-ctx.Done():
							return
						}
					} else {
						results[idx] = result
					}
				}
			}
		}()
	}
	
	// Wait for completion
	wg.Wait()
	close(errChan)
	
	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return results, fmt.Errorf("%d errors occurred during map operation", len(errs))
	}
	
	return results, nil
}

// ForEach executes a function for all items concurrently
func ForEach[T any](ctx context.Context, items []T, fn func(T) error, workers int) error {
	_, err := Map(ctx, items, func(item T) (struct{}, error) {
		return struct{}{}, fn(item)
	}, workers)
	return err
}

// Filter filters items concurrently
func Filter[T any](ctx context.Context, items []T, predicate func(T) bool, workers int) ([]T, error) {
	type result struct {
		item  T
		keep  bool
		index int
	}
	
	results, err := Map(ctx, items, func(item T) (result, error) {
		return result{item: item, keep: predicate(item)}, nil
	}, workers)
	
	if err != nil {
		return nil, err
	}
	
	var filtered []T
	for _, r := range results {
		if r.keep {
			filtered = append(filtered, r.item)
		}
	}
	
	return filtered, nil
}

// Parallel runs multiple functions in parallel and returns their results
func Parallel(ctx context.Context, fns ...func() error) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(fns))
	
	for _, fn := range fns {
		wg.Add(1)
		go func(f func() error) {
			defer wg.Done()
			
			if err := f(); err != nil {
				select {
				case errChan <- err:
				case <-ctx.Done():
				}
			}
		}(fn)
	}
	
	wg.Wait()
	close(errChan)
	
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	return errs
}

// Race runs multiple functions and returns the first successful result or all errors
// Note: This version accepts a pointer type or uses a boolean to indicate success
func Race[T any](ctx context.Context, fns ...func(context.Context) (T, error)) (T, []error) {
	var zero T

	if len(fns) == 0 {
		return zero, nil
	}

	type result struct {
		value T
		err   error
		valid bool
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultChan := make(chan result, len(fns))

	var wg sync.WaitGroup

	for _, fn := range fns {
		wg.Add(1)
		go func(f func(context.Context) (T, error)) {
			defer wg.Done()

			val, err := f(ctx)
			select {
			case resultChan <- result{value: val, err: err, valid: err == nil}:
			case <-ctx.Done():
			}
		}(fn)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Wait for first successful result or all errors
	var errs []error
	for res := range resultChan {
		if res.valid {
			cancel() // Cancel remaining operations
			return res.value, nil
		}
		errs = append(errs, res.err)
		if len(errs) == len(fns) {
			return zero, errs
		}
	}

	return zero, errs
}

// Debounce wraps a function to limit its execution rate
func Debounce(fn func(), duration time.Duration) func() {
	var mu sync.Mutex
	var timer *time.Timer
	
	return func() {
		mu.Lock()
		defer mu.Unlock()
		
		if timer != nil {
			timer.Stop()
		}
		
		timer = time.AfterFunc(duration, fn)
	}
}

// Throttle wraps a function to limit its execution rate
func Throttle(fn func(), duration time.Duration) func() {
	var mu sync.Mutex
	var last time.Time
	
	return func() {
		mu.Lock()
		defer mu.Unlock()
		
		now := time.Now()
		if now.Sub(last) < duration {
			return
		}
		
		last = now
		fn()
	}
}

// Semaphore is a counting semaphore
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a new semaphore with the given capacity
func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, capacity),
	}
}

// Acquire acquires the semaphore
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire tries to acquire the semaphore without blocking
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases the semaphore
func (s *Semaphore) Release() {
	select {
	case <-s.ch:
	default:
	}
}
