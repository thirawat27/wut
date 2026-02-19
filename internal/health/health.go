// Package health provides health check functionality for WUT
package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Status represents health status
type Status int

const (
	// StatusHealthy indicates healthy state
	StatusHealthy Status = iota
	// StatusDegraded indicates degraded state
	StatusDegraded
	// StatusUnhealthy indicates unhealthy state
	StatusUnhealthy
)

func (s Status) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// Check represents a health check
type Check struct {
	Name        string
	Description string
	Checker     func(ctx context.Context) error
	Critical    bool // If true, failure makes system unhealthy
	Timeout     time.Duration
}

// Result represents a health check result
type Result struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
	Response  time.Duration `json:"response_time"`
	Timestamp time.Time     `json:"timestamp"`
	Critical  bool          `json:"critical"`
}

// Health represents the overall health status
type Health struct {
	Status    string            `json:"status"`
	Checks    []Result          `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Checker manages health checks
type Checker struct {
	checks    []Check
	version   string
	startTime time.Time
	mu        sync.RWMutex
	results   map[string]Result
}

// NewChecker creates a new health checker
func NewChecker(version string) *Checker {
	return &Checker{
		checks:    make([]Check, 0),
		version:   version,
		startTime: time.Now(),
		results:   make(map[string]Result),
	}
}

// Register registers a new health check
func (c *Checker) Register(check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if check.Timeout == 0 {
		check.Timeout = 5 * time.Second
	}

	c.checks = append(c.checks, check)
}

// RegisterDefaultChecks registers default health checks
func (c *Checker) RegisterDefaultChecks() {
	// Memory check
	c.Register(Check{
		Name:        "memory",
		Description: "Memory usage check",
		Checker:     c.checkMemory,
		Critical:    false,
		Timeout:     2 * time.Second,
	})

	// Disk check
	c.Register(Check{
		Name:        "disk",
		Description: "Disk space check",
		Checker:     c.checkDisk,
		Critical:    true,
		Timeout:     2 * time.Second,
	})
}

// Check runs all health checks and returns overall health
func (c *Checker) Check(ctx context.Context) Health {
	c.mu.RLock()
	checks := make([]Check, len(c.checks))
	copy(checks, c.checks)
	c.mu.RUnlock()

	results := make([]Result, 0, len(checks))
	var overallStatus Status = StatusHealthy

	for _, check := range checks {
		result := c.runCheck(ctx, check)
		results = append(results, result)

		if result.Status == "unhealthy" {
			if check.Critical {
				overallStatus = StatusUnhealthy
			} else if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}
	}

	// Store results
	c.mu.Lock()
	for _, r := range results {
		c.results[r.Name] = r
	}
	c.mu.Unlock()

	return Health{
		Status:    overallStatus.String(),
		Checks:    results,
		Timestamp: time.Now(),
		Version:   c.version,
		Uptime:    time.Since(c.startTime).String(),
	}
}

// runCheck runs a single health check
func (c *Checker) runCheck(ctx context.Context, check Check) Result {
	result := Result{
		Name:      check.Name,
		Critical:  check.Critical,
		Timestamp: time.Now(),
	}

	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	start := time.Now()
	err := check.Checker(checkCtx)
	result.Response = time.Since(start)

	if err != nil {
		result.Status = "unhealthy"
		result.Error = err.Error()
	} else {
		result.Status = "healthy"
	}

	return result
}

// GetResult returns the last result for a specific check
func (c *Checker) GetResult(name string) (Result, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, ok := c.results[name]
	return result, ok
}

// IsHealthy returns true if the last check was healthy
func (c *Checker) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, result := range c.results {
		if result.Status != "healthy" && result.Critical {
			return false
		}
	}
	return true
}

// checkMemory checks memory usage
func (c *Checker) checkMemory(ctx context.Context) error {
	// Simplified memory check
	// In production, this would check actual memory usage
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// checkDisk checks disk space
func (c *Checker) checkDisk(ctx context.Context) error {
	// Simplified disk check
	// In production, this would check actual disk usage
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Predefined health checks

// DatabaseCheck creates a database health check
func DatabaseCheck(dbChecker func(ctx context.Context) error) Check {
	return Check{
		Name:        "database",
		Description: "Database connectivity check",
		Checker:     dbChecker,
		Critical:    true,
		Timeout:     5 * time.Second,
	}
}

// AICheck creates an AI model health check
func AICheck(aiChecker func(ctx context.Context) error) Check {
	return Check{
		Name:        "ai_model",
		Description: "AI model availability check",
		Checker:     aiChecker,
		Critical:    false,
		Timeout:     10 * time.Second,
	}
}

// StorageCheck creates a storage health check
func StorageCheck(storageChecker func(ctx context.Context) error) Check {
	return Check{
		Name:        "storage",
		Description: "Storage access check",
		Checker:     storageChecker,
		Critical:    true,
		Timeout:     5 * time.Second,
	}
}

// ExternalServiceCheck creates an external service health check
func ExternalServiceCheck(name string, checker func(ctx context.Context) error) Check {
	return Check{
		Name:        fmt.Sprintf("external_%s", name),
		Description: fmt.Sprintf("External service %s check", name),
		Checker:     checker,
		Critical:    false,
		Timeout:     10 * time.Second,
	}
}
