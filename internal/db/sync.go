// Package db provides TLDR Pages sync functionality
package db

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"wut/internal/concurrency"
	"wut/internal/logger"
)

// SyncManager manages syncing TLDR pages to local storage
type SyncManager struct {
	client    *Client
	storage   *Storage
	log       *logger.Logger
	workerPool *concurrency.Pool
}

// SyncOptions contains options for syncing
type SyncOptions struct {
	Platforms   []string
	Commands    []string
	Concurrency int
	ForceUpdate bool
	OnProgress  func(current, total int, command string)
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	Downloaded int
	Failed     int
	Skipped    int
	Errors     []error
	Duration   time.Duration
}

// NewSyncManager creates a new sync manager
func NewSyncManager(storage *Storage) *SyncManager {
	pool := concurrency.NewPool(concurrency.WithWorkerCount(runtime.NumCPU() * 2))
	pool.Start()

	sm := &SyncManager{
		client:     NewClient(),
		storage:    storage,
		log:        logger.With("db-sync"),
		workerPool: pool,
	}
	return sm
}

// SetClient sets a custom client (useful for testing)
func (sm *SyncManager) SetClient(client *Client) {
	sm.client = client
}

// Stop stops the sync manager and its worker pool
func (sm *SyncManager) Stop() {
	if sm.workerPool != nil {
		sm.workerPool.Stop()
	}
}

// SyncAll syncs all common commands to local storage
func (sm *SyncManager) SyncAll(ctx context.Context) (*SyncResult, error) {
	return sm.SyncCommands(ctx, nil)
}

// SyncCommands syncs specific commands to local storage with high concurrency
func (sm *SyncManager) SyncCommands(ctx context.Context, commands []string) (*SyncResult, error) {
	return sm.SyncCommandsWithOptions(ctx, SyncOptions{Commands: commands})
}

// SyncCommandsWithOptions syncs commands with detailed options
func (sm *SyncManager) SyncCommandsWithOptions(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{}

	// If no commands specified, get popular ones
	if len(opts.Commands) == 0 {
		var err error
		opts.Commands, err = sm.client.GetAvailableCommands(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get command list: %w", err)
		}
	}

	sm.log.Info("starting sync", "commands", len(opts.Commands), "concurrency", opts.Concurrency)

	totalCommands := int64(len(opts.Commands))
	var currentCount int64

	// Create task function for each command
	taskFunc := func(command string) func(context.Context) error {
		return func(ctx context.Context) error {
			err := sm.syncCommand(ctx, command)

			// Update progress
			current := atomic.AddInt64(&currentCount, 1)
			if opts.OnProgress != nil {
				opts.OnProgress(int(current), int(totalCommands), command)
			}

			return err
		}
	}

	// Create tasks
	tasks := make([]func(context.Context) error, len(opts.Commands))
	for i, cmd := range opts.Commands {
		tasks[i] = taskFunc(cmd)
	}

	// Determine concurrency level
	workers := opts.Concurrency
	if workers <= 0 {
		workers = runtime.NumCPU() * 2 // Use 2x CPU cores for I/O bound operations
	}

	// Execute tasks concurrently using our Map function
	results, err := concurrency.Map(ctx, tasks, func(fn func(context.Context) error) (error, error) {
		return fn(ctx), nil
	}, workers)

	if err != nil {
		sm.log.Warn("some sync operations failed", "error", err)
	}

	// Process results
	for i, res := range results {
		if res != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", opts.Commands[i], res))
			sm.log.Warn("failed to sync command", "command", opts.Commands[i], "error", res)
		} else {
			result.Downloaded++
		}
	}

	result.Duration = time.Since(start)

	// Update metadata
	meta := &Metadata{
		LastSync:   time.Now(),
		TotalPages: result.Downloaded,
		Platforms:  []string{PlatformCommon, PlatformLinux, PlatformMacOS, PlatformWindows},
	}
	if err := sm.storage.SaveMetadata(meta); err != nil {
		sm.log.Warn("failed to save metadata", "error", err)
	}

	sm.log.Info("sync completed",
		"downloaded", result.Downloaded,
		"failed", result.Failed,
		"duration", result.Duration,
	)

	return result, nil
}

// SyncCommandsBatch syncs commands in batches for better memory efficiency
func (sm *SyncManager) SyncCommandsBatch(ctx context.Context, commands []string, batchSize int) (*SyncResult, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	totalResult := &SyncResult{}
	start := time.Now()

	for i := 0; i < len(commands); i += batchSize {
		end := i + batchSize
		if end > len(commands) {
			end = len(commands)
		}

		batch := commands[i:end]
		sm.log.Debug("processing batch", "batch", i/batchSize+1, "commands", len(batch))

		result, err := sm.SyncCommands(ctx, batch)
		if err != nil {
			sm.log.Warn("batch sync failed", "batch", i/batchSize+1, "error", err)
		}

		totalResult.Downloaded += result.Downloaded
		totalResult.Failed += result.Failed
		totalResult.Errors = append(totalResult.Errors, result.Errors...)
	}

	totalResult.Duration = time.Since(start)

	// Update metadata
	meta := &Metadata{
		LastSync:   time.Now(),
		TotalPages: totalResult.Downloaded,
		Platforms:  []string{PlatformCommon, PlatformLinux, PlatformMacOS, PlatformWindows},
	}
	if err := sm.storage.SaveMetadata(meta); err != nil {
		sm.log.Warn("failed to save metadata", "error", err)
	}

	return totalResult, nil
}

// syncCommand syncs a single command
func (sm *SyncManager) syncCommand(ctx context.Context, command string) error {
	page, err := sm.client.GetPageAnyPlatform(ctx, command)
	if err != nil {
		return err
	}

	return sm.storage.SavePage(page)
}

// SyncPlatforms syncs commands for specific platforms concurrently
func (sm *SyncManager) SyncPlatforms(ctx context.Context, platforms []string) (*SyncResult, error) {
	if len(platforms) == 0 {
		platforms = []string{PlatformCommon, PlatformLinux, PlatformMacOS, PlatformWindows}
	}

	sm.log.Info("syncing platforms", "platforms", platforms)

	// Use Parallel to sync all platforms concurrently
	syncFuncs := make([]func() error, len(platforms))
	results := make([]*SyncResult, len(platforms))
	var mu sync.Mutex

	for i, platform := range platforms {
		idx := i
		plat := platform
		syncFuncs[i] = func() error {
			commands, err := sm.getPlatformCommands(ctx, plat)
			if err != nil {
				return fmt.Errorf("failed to get commands for %s: %w", plat, err)
			}

			result, err := sm.SyncCommands(ctx, commands)
			if err != nil {
				return err
			}

			mu.Lock()
			results[idx] = result
			mu.Unlock()

			return nil
		}
	}

	errs := concurrency.Parallel(ctx, syncFuncs...)

	// Aggregate results
	totalResult := &SyncResult{}
	for _, result := range results {
		if result != nil {
			totalResult.Downloaded += result.Downloaded
			totalResult.Failed += result.Failed
			totalResult.Errors = append(totalResult.Errors, result.Errors...)
		}
	}

	if len(errs) > 0 {
		return totalResult, fmt.Errorf("platform sync completed with %d errors", len(errs))
	}

	return totalResult, nil
}

// getPlatformCommands gets available commands for a platform
func (sm *SyncManager) getPlatformCommands(ctx context.Context, platform string) ([]string, error) {
	// This is a simplified version - in reality, you'd fetch from TLDR API
	// For now, return common commands
	return sm.client.GetAvailableCommands(ctx)
}

// SyncPopular syncs popular/common commands
func (sm *SyncManager) SyncPopular(ctx context.Context) (*SyncResult, error) {
	popularCommands := []string{
		"git", "docker", "npm", "node", "python", "pip", "cargo", "rustc",
		"kubectl", "helm", "terraform", "ansible", "vagrant",
		"ls", "cd", "pwd", "cat", "less", "more", "head", "tail",
		"grep", "find", "sed", "awk", "sort", "uniq", "wc",
		"tar", "zip", "unzip", "gzip", "gunzip",
		"chmod", "chown", "chgrp", "ln", "mkdir", "rm", "cp", "mv",
		"ps", "top", "htop", "kill", "killall",
		"ssh", "scp", "rsync", "curl", "wget", "ping", "netstat",
		"systemctl", "service", "crontab", "at",
		"vim", "vi", "nano", "emacs", "code",
		"apt", "yum", "dnf", "pacman", "brew",
		"make", "cmake", "gcc", "g++", "clang",
		"jq", "yq", "xmllint",
	}

	return sm.SyncCommands(ctx, popularCommands)
}

// UpdateCommand updates a single command in local storage
func (sm *SyncManager) UpdateCommand(ctx context.Context, command string) error {
	return sm.syncCommand(ctx, command)
}

// IsStale checks if the local database is stale
func (sm *SyncManager) IsStale(maxAge time.Duration) bool {
	meta, err := sm.storage.GetMetadata()
	if err != nil {
		return true
	}
	return time.Since(meta.LastSync) > maxAge
}

// GetLastSync returns the last sync time
func (sm *SyncManager) GetLastSync() (time.Time, error) {
	meta, err := sm.storage.GetMetadata()
	if err != nil {
		return time.Time{}, err
	}
	return meta.LastSync, nil
}

// AutoSync syncs if the database is older than maxAge
func (sm *SyncManager) AutoSync(ctx context.Context, maxAge time.Duration) (*SyncResult, error) {
	if !sm.IsStale(maxAge) {
		sm.log.Info("local database is up to date")
		return &SyncResult{Skipped: 1}, nil
	}

	sm.log.Info("local database is stale, syncing...")
	return sm.SyncPopular(ctx)
}

// SyncWithProgress syncs with progress reporting
func (sm *SyncManager) SyncWithProgress(ctx context.Context, commands []string, onProgress func(current, total int, command string)) (*SyncResult, error) {
	opts := SyncOptions{
		Commands:   commands,
		OnProgress: onProgress,
	}
	return sm.SyncCommandsWithOptions(ctx, opts)
}
