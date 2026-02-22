// Package performance provides benchmarking utilities
package performance

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// BenchmarkResult holds benchmark results
type BenchmarkResult struct {
	Name       string
	Ops        uint64
	Duration   time.Duration
	Bytes      uint64
	Allocs     uint64
	Latency    time.Duration // Average latency
	Throughput float64       // Ops per second
}

// String returns formatted benchmark result
func (r BenchmarkResult) String() string {
	return fmt.Sprintf("%s: %d ops, %v, %.2f ops/sec, avg latency: %v, %d allocs",
		r.Name, r.Ops, r.Duration, r.Throughput, r.Latency, r.Allocs)
}

// Benchmark runs a benchmark function
func Benchmark(name string, duration time.Duration, fn func()) BenchmarkResult {
	var ops atomic.Uint64
	var allocs uint64

	// Warmup
	fn()
	runtime.GC()

	// Get initial allocations
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run benchmark
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				fn()
				ops.Add(1)
			}
		}
	}()

	time.Sleep(duration)
	close(done)

	// Get final allocations
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	allocs = m2.Mallocs - m1.Mallocs

	opCount := ops.Load()
	avgLatency := time.Duration(int64(duration) / int64(opCount))
	throughput := float64(opCount) / duration.Seconds()

	return BenchmarkResult{
		Name:       name,
		Ops:        opCount,
		Duration:   duration,
		Allocs:     allocs,
		Latency:    avgLatency,
		Throughput: throughput,
	}
}

// BenchmarkParallel runs parallel benchmark
func BenchmarkParallel(name string, duration time.Duration, workers int, fn func()) BenchmarkResult {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	var ops atomic.Uint64

	// Warmup
	fn()
	runtime.GC()

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
					fn()
					ops.Add(1)
				}
			}
		})
	}

	time.Sleep(duration)
	close(done)
	wg.Wait()

	opCount := ops.Load()
	avgLatency := time.Duration(int64(duration) / int64(opCount))
	throughput := float64(opCount) / duration.Seconds()

	return BenchmarkResult{
		Name:       name,
		Ops:        opCount,
		Duration:   duration,
		Latency:    avgLatency,
		Throughput: throughput,
	}
}

// LatencyDistribution measures latency distribution
type LatencyDistribution struct {
	Samples []time.Duration
	Count   int
	Min     time.Duration
	Max     time.Duration
	Mean    time.Duration
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
	P999    time.Duration
}

// MeasureLatency measures latency distribution
func MeasureLatency(samples int, fn func()) LatencyDistribution {
	if samples <= 0 {
		samples = 10000
	}

	d := LatencyDistribution{
		Samples: make([]time.Duration, samples),
		Count:   samples,
		Min:     time.Duration(1<<63 - 1),
		Max:     0,
	}

	var sum int64

	for i := 0; i < samples; i++ {
		start := time.Now()
		fn()
		latency := time.Since(start)

		d.Samples[i] = latency
		sum += int64(latency)

		if latency < d.Min {
			d.Min = latency
		}
		if latency > d.Max {
			d.Max = latency
		}
	}

	d.Mean = time.Duration(sum / int64(samples))

	// Sort for percentiles
	sortDurations(d.Samples)

	d.P50 = d.Samples[samples*50/100]
	d.P95 = d.Samples[samples*95/100]
	d.P99 = d.Samples[samples*99/100]
	d.P999 = d.Samples[samples*999/1000]

	return d
}

func sortDurations(d []time.Duration) {
	quickSortDuration(d, 0, len(d)-1)
}

func quickSortDuration(d []time.Duration, low, high int) {
	if low < high {
		pi := partitionDuration(d, low, high)
		quickSortDuration(d, low, pi-1)
		quickSortDuration(d, pi+1, high)
	}
}

func partitionDuration(d []time.Duration, low, high int) int {
	pivot := d[high]
	i := low - 1

	for j := low; j < high; j++ {
		if d[j] < pivot {
			i++
			d[i], d[j] = d[j], d[i]
		}
	}
	d[i+1], d[high] = d[high], d[i+1]
	return i + 1
}

// String returns formatted distribution
func (d LatencyDistribution) String() string {
	return fmt.Sprintf("Latency Distribution (n=%d):\n"+
		"  Min:  %v\n"+
		"  Mean: %v\n"+
		"  P50:  %v\n"+
		"  P95:  %v\n"+
		"  P99:  %v\n"+
		"  P99.9: %v\n"+
		"  Max:  %v",
		d.Count, d.Min, d.Mean, d.P50, d.P95, d.P99, d.P999, d.Max)
}

// ProfileOptions holds profiling options
type ProfileOptions struct {
	CPUProfile   bool
	MemProfile   bool
	BlockProfile bool
	MutexProfile bool
	Duration     time.Duration
}

// Profiler provides profiling utilities
type Profiler struct {
	options   ProfileOptions
	startTime time.Time
}

// NewProfiler creates a new profiler
func NewProfiler(opts ProfileOptions) *Profiler {
	return &Profiler{
		options: opts,
	}
}

// Start starts profiling
func (p *Profiler) Start() {
	p.startTime = time.Now()

	//if p.options.CPUProfile {
	//	// CPU profiling would be implemented here
	//	// Requires runtime/pprof
	//}
}

// Stop stops profiling and returns results
func (p *Profiler) Stop() ProfileResult {
	duration := time.Since(p.startTime)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return ProfileResult{
		Duration:     duration,
		AllocBytes:   m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		SysBytes:     m.Sys,
		NumGC:        m.NumGC,
		NumGoroutine: runtime.NumGoroutine(),
	}
}

// ProfileResult holds profiling results
type ProfileResult struct {
	Duration     time.Duration
	AllocBytes   uint64
	TotalAlloc   uint64
	SysBytes     uint64
	NumGC        uint32
	NumGoroutine int
}

// String returns formatted profile result
func (r ProfileResult) String() string {
	return fmt.Sprintf("Profile Results (%v):\n"+
		"  Alloc:       %d bytes\n"+
		"  Total Alloc: %d bytes\n"+
		"  Sys:         %d bytes\n"+
		"  Num GC:      %d\n"+
		"  Goroutines:  %d",
		r.Duration, r.AllocBytes, r.TotalAlloc, r.SysBytes, r.NumGC, r.NumGoroutine)
}

// ThroughputTester tests throughput over time
type ThroughputTester struct {
	interval time.Duration
	samples  []ThroughputSample
}

// ThroughputSample represents a throughput measurement
type ThroughputSample struct {
	Time       time.Time
	Ops        uint64
	Throughput float64 // ops/sec
}

// NewThroughputTester creates a new throughput tester
func NewThroughputTester(interval time.Duration) *ThroughputTester {
	return &ThroughputTester{
		interval: interval,
		samples:  make([]ThroughputSample, 0),
	}
}

// Run runs throughput test
func (t *ThroughputTester) Run(duration time.Duration, fn func()) []ThroughputSample {
	var ops atomic.Uint64
	stop := make(chan struct{})

	// Start operation counter
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				fn()
				ops.Add(1)
			}
		}
	}()

	// Sample throughput
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	timeout := time.After(duration)
	lastOps := uint64(0)

	for {
		select {
		case <-ticker.C:
			currentOps := ops.Load()
			deltaOps := currentOps - lastOps
			lastOps = currentOps

			throughput := float64(deltaOps) / t.interval.Seconds()
			t.samples = append(t.samples, ThroughputSample{
				Time:       time.Now(),
				Ops:        currentOps,
				Throughput: throughput,
			})

		case <-timeout:
			close(stop)
			return t.samples
		}
	}
}

// CompareBenchmarks compares two benchmark results
func CompareBenchmarks(baseline, current BenchmarkResult) ComparisonResult {
	return ComparisonResult{
		Baseline:     baseline,
		Current:      current,
		OpsDelta:     int64(current.Ops) - int64(baseline.Ops),
		OpsPercent:   (float64(current.Ops) - float64(baseline.Ops)) / float64(baseline.Ops) * 100,
		LatencyDelta: current.Latency - baseline.Latency,
	}
}

// ComparisonResult holds comparison results
type ComparisonResult struct {
	Baseline     BenchmarkResult
	Current      BenchmarkResult
	OpsDelta     int64
	OpsPercent   float64
	LatencyDelta time.Duration
}

// String returns formatted comparison
func (c ComparisonResult) String() string {
	return fmt.Sprintf("Comparison:\n"+
		"  Ops:    %d (%.2f%%)\n"+
		"  Latency: %v (delta: %v)",
		c.OpsDelta, c.OpsPercent, c.Current.Latency, c.LatencyDelta)
}
