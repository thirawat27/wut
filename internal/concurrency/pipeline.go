// Package concurrency provides concurrent processing utilities for WUT
package concurrency

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// Stage represents a pipeline stage
type Stage[T any] interface {
	Process(ctx context.Context, input T) (T, error)
}

// StageFunc is a function that implements Stage
type StageFunc[T any] func(ctx context.Context, input T) (T, error)

// Process implements Stage interface
func (f StageFunc[T]) Process(ctx context.Context, input T) (T, error) {
	return f(ctx, input)
}

// Pipeline represents a processing pipeline
type Pipeline[T any] struct {
	stages  []Stage[T]
	workers int
	buffer  int
}

// PipelineOption is a functional option for Pipeline
type PipelineOption[T any] func(*Pipeline[T])

// WithPipelineWorkers sets the number of workers per stage
func WithPipelineWorkers[T any](n int) PipelineOption[T] {
	return func(p *Pipeline[T]) {
		if n > 0 {
			p.workers = n
		}
	}
}

// WithBufferSize sets the buffer size for channels
func WithBufferSize[T any](n int) PipelineOption[T] {
	return func(p *Pipeline[T]) {
		if n > 0 {
			p.buffer = n
		}
	}
}

// NewPipeline creates a new pipeline
func NewPipeline[T any](opts ...PipelineOption[T]) *Pipeline[T] {
	p := &Pipeline[T]{
		workers: runtime.NumCPU(),
		buffer:  100,
	}
	
	for _, opt := range opts {
		opt(p)
	}
	
	return p
}

// AddStage adds a stage to the pipeline
func (p *Pipeline[T]) AddStage(stage Stage[T]) *Pipeline[T] {
	p.stages = append(p.stages, stage)
	return p
}

// AddStageFunc adds a function as a stage
func (p *Pipeline[T]) AddStageFunc(fn func(ctx context.Context, input T) (T, error)) *Pipeline[T] {
	return p.AddStage(StageFunc[T](fn))
}

// Process processes items through the pipeline
func (p *Pipeline[T]) Process(ctx context.Context, items []T) ([]T, error) {
	if len(items) == 0 {
		return []T{}, nil
	}
	
	if len(p.stages) == 0 {
		return items, nil
	}
	
	// Create channels for each stage
	channels := make([]chan T, len(p.stages)+1)
	errChan := make(chan error, len(items))
	
	for i := range channels {
		channels[i] = make(chan T, p.buffer)
	}
	
	// Start each stage
	var wg sync.WaitGroup
	
	for i, stage := range p.stages {
		inputChan := channels[i]
		outputChan := channels[i+1]
		
		for w := 0; w < p.workers; w++ {
			wg.Add(1)
			go func(s Stage[T], in <-chan T, out chan<- T) {
				defer wg.Done()
				
				for {
					select {
					case <-ctx.Done():
						return
					case item, ok := <-in:
						if !ok {
							return
						}
						
						result, err := s.Process(ctx, item)
						if err != nil {
							select {
							case errChan <- fmt.Errorf("stage error: %w", err):
							case <-ctx.Done():
								return
							}
							continue
						}
						
						select {
						case out <- result:
						case <-ctx.Done():
							return
						}
					}
				}
			}(stage, inputChan, outputChan)
		}
	}
	
	// Send input items with sync.Once to ensure channel is closed only once
	var closeOnce sync.Once
	go func() {
		defer closeOnce.Do(func() { close(channels[0]) })
		for _, item := range items {
			select {
			case channels[0] <- item:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Collect results
	results := make([]T, 0, len(items))
	resultDone := make(chan struct{})
	
	go func() {
		defer close(resultDone)
		for result := range channels[len(p.stages)] {
			results = append(results, result)
		}
	}()
	
	// Wait for completion
	wg.Wait()
	close(channels[len(p.stages)])
	<-resultDone
	close(errChan)
	
	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return results, fmt.Errorf("pipeline completed with %d errors", len(errs))
	}
	
	return results, nil
}

// ParallelPipeline runs multiple pipelines in parallel and merges results
type ParallelPipeline[T any] struct {
	pipelines []*Pipeline[T]
	merger    func([]T) ([]T, error)
}

// NewParallelPipeline creates a new parallel pipeline
func NewParallelPipeline[T any](merger func([]T) ([]T, error)) *ParallelPipeline[T] {
	if merger == nil {
		merger = func(results []T) ([]T, error) {
			var merged []T
			merged = append(merged, results...)
			return merged, nil
		}
	}
	
	return &ParallelPipeline[T]{
		pipelines: make([]*Pipeline[T], 0),
		merger:    merger,
	}
}

// AddPipeline adds a pipeline to the parallel pipeline
func (pp *ParallelPipeline[T]) AddPipeline(p *Pipeline[T]) *ParallelPipeline[T] {
	pp.pipelines = append(pp.pipelines, p)
	return pp
}

// Process processes items through all pipelines in parallel
func (pp *ParallelPipeline[T]) Process(ctx context.Context, items []T) ([]T, error) {
	if len(pp.pipelines) == 0 {
		return items, nil
	}
	
	// Process through all pipelines concurrently
	results := make([][]T, len(pp.pipelines))
	errChan := make(chan error, len(pp.pipelines))
	
	var wg sync.WaitGroup
	for i, pipeline := range pp.pipelines {
		wg.Add(1)
		go func(idx int, p *Pipeline[T]) {
			defer wg.Done()
			
			result, err := p.Process(ctx, items)
			if err != nil {
				errChan <- err
				return
			}
			results[idx] = result
		}(i, pipeline)
	}
	
	wg.Wait()
	close(errChan)
	
	// Check for errors
	for err := range errChan {
		return nil, err
	}
	
	// Flatten results for merger
	var flattened []T
	for _, r := range results {
		flattened = append(flattened, r...)
	}
	
	return pp.merger(flattened)
}

// BatchProcessor processes items in batches
type BatchProcessor[T any] struct {
	batchSize int
	processor func(ctx context.Context, batch []T) error
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor[T any](batchSize int, processor func(ctx context.Context, batch []T) error) *BatchProcessor[T] {
	return &BatchProcessor[T]{
		batchSize: batchSize,
		processor: processor,
	}
}

// Process processes items in batches
func (bp *BatchProcessor[T]) Process(ctx context.Context, items []T) error {
	if len(items) == 0 {
		return nil
	}
	
	for i := 0; i < len(items); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(items) {
			end = len(items)
		}
		
		batch := items[i:end]
		if err := bp.processor(ctx, batch); err != nil {
			return fmt.Errorf("batch %d failed: %w", i/bp.batchSize, err)
		}
	}
	
	return nil
}

// ProcessConcurrent processes items in batches concurrently
func (bp *BatchProcessor[T]) ProcessConcurrent(ctx context.Context, items []T, workers int) error {
	if len(items) == 0 {
		return nil
	}
	
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	
	// Create batches
	var batches [][]T
	for i := 0; i < len(items); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}
	
	// Process batches concurrently
	return ForEach(ctx, batches, func(batch []T) error {
		return bp.processor(ctx, batch)
	}, workers)
}

// FanOut distributes items to multiple workers
type FanOut[T any] struct {
	workers int
	handler func(ctx context.Context, item T) error
}

// NewFanOut creates a new fan-out processor
func NewFanOut[T any](workers int, handler func(ctx context.Context, item T) error) *FanOut[T] {
	return &FanOut[T]{
		workers: workers,
		handler: handler,
	}
}

// Process distributes items to multiple workers
func (fo *FanOut[T]) Process(ctx context.Context, items []T) error {
	if len(items) == 0 {
		return nil
	}
	
	itemChan := make(chan T, fo.workers)
	errChan := make(chan error, len(items))
	
	var wg sync.WaitGroup
	
	// Start workers
	for i := 0; i < fo.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-itemChan:
					if !ok {
						return
					}
					
					if err := fo.handler(ctx, item); err != nil {
						errChan <- err
					}
				}
			}
		}()
	}
	
	// Send items
	go func() {
		defer close(itemChan)
		for _, item := range items {
			select {
			case itemChan <- item:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Wait for completion
	wg.Wait()
	close(errChan)
	
	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("fan-out completed with %d errors", len(errs))
	}
	
	return nil
}

// FanIn merges multiple input channels into one
type FanIn[T any] struct {
	inputs []<-chan T
}

// NewFanIn creates a new fan-in processor
func NewFanIn[T any](inputs ...<-chan T) *FanIn[T] {
	return &FanIn[T]{
		inputs: inputs,
	}
}

// AddInput adds an input channel
func (fi *FanIn[T]) AddInput(ch <-chan T) *FanIn[T] {
	fi.inputs = append(fi.inputs, ch)
	return fi
}

// Merge merges all input channels into one output channel
func (fi *FanIn[T]) Merge(ctx context.Context) <-chan T {
	output := make(chan T, len(fi.inputs))
	
	var wg sync.WaitGroup
	
	for _, input := range fi.inputs {
		wg.Add(1)
		go func(ch <-chan T) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-ch:
					if !ok {
						return
					}
					
					select {
					case output <- item:
					case <-ctx.Done():
						return
					}
				}
			}
		}(input)
	}
	
	go func() {
		wg.Wait()
		close(output)
	}()
	
	return output
}

// Stream represents a stream of data
type Stream[T any] struct {
	ch     <-chan T
	ctx    context.Context
	cancel context.CancelFunc
}

// NewStream creates a new stream
func NewStream[T any](ch <-chan T) *Stream[T] {
	ctx, cancel := context.WithCancel(context.Background())
	return &Stream[T]{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Map applies a function to each item in the stream
func (s *Stream[T]) Map(fn func(T) (T, error)) *Stream[T] {
	output := make(chan T)
	
	go func() {
		defer close(output)
		
		for {
			select {
			case <-s.ctx.Done():
				return
			case item, ok := <-s.ch:
				if !ok {
					return
				}
				
				result, err := fn(item)
				if err != nil {
					continue // Skip failed items
				}
				
				select {
				case output <- result:
				case <-s.ctx.Done():
					return
				}
			}
		}
	}()
	
	return NewStream(output)
}

// Filter filters items in the stream
func (s *Stream[T]) Filter(predicate func(T) bool) *Stream[T] {
	output := make(chan T)
	
	go func() {
		defer close(output)
		
		for {
			select {
			case <-s.ctx.Done():
				return
			case item, ok := <-s.ch:
				if !ok {
					return
				}
				
				if predicate(item) {
					select {
					case output <- item:
					case <-s.ctx.Done():
						return
					}
				}
			}
		}
	}()
	
	return NewStream(output)
}

// Collect collects all items from the stream
func (s *Stream[T]) Collect() []T {
	var results []T
	for item := range s.ch {
		results = append(results, item)
	}
	return results
}

// CollectWithContext collects items from the stream with context
func (s *Stream[T]) CollectWithContext(ctx context.Context) []T {
	var results []T
	
	for {
		select {
		case <-ctx.Done():
			return results
		case item, ok := <-s.ch:
			if !ok {
				return results
			}
			results = append(results, item)
		}
	}
}

// Cancel cancels the stream
func (s *Stream[T]) Cancel() {
	s.cancel()
}
