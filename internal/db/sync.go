// Package db provides TLDR Pages sync functionality
package db

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wut/internal/concurrency"
	"wut/internal/logger"
)

// SyncManager manages syncing TLDR pages to local storage
type SyncManager struct {
	client     *Client
	storage    *Storage
	log        *logger.Logger
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
	// 1. Prefer local source if available
	localPaths := []string{
		"tldr-main",
		filepath.Join("tldr-main", "tldr-main"),
	}

	for _, p := range localPaths {
		if stat, err := os.Stat(filepath.Join(p, "pages")); err == nil && stat.IsDir() {
			sm.log.Info("found local tldr directory, syncing from disk ...", "path", p)
			return sm.SyncFromLocalDir(ctx, p)
		}
	}

	// 2. Fallback to downloading the full archive
	zipURL := "https://github.com/tldr-pages/tldr/releases/latest/download/tldr.zip"
	sm.log.Info("syncing from remote zip archive ...")
	return sm.SyncFromZip(ctx, zipURL)
}

// SyncFromLocalDir reads an extracted tldr-pages archive directory
func (sm *SyncManager) SyncFromLocalDir(ctx context.Context, pagesDir string) (*SyncResult, error) {
	start := time.Now()
	var pages []*Page

	sm.log.Info("reading local pages directory", "dir", pagesDir)

	err := filepath.Walk(pagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		relPath, err := filepath.Rel(pagesDir, path)
		if err != nil {
			return nil
		}

		// Expect langDir/platform/command.md
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		if len(parts) != 3 {
			return nil
		}

		langDir := parts[0]
		language := "en"
		if strings.HasPrefix(langDir, "pages.") {
			language = strings.TrimPrefix(langDir, "pages.")
		} else if langDir != "pages" {
			return nil
		}

		platform := parts[1]
		command := strings.TrimSuffix(parts[2], ".md")

		content, err := os.ReadFile(path)
		if err != nil {
			sm.log.Warn("failed to read local page", "file", path, "error", err)
			return nil
		}

		page := sm.client.parsePage(string(content), command, platform, language)
		pages = append(pages, page)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed walking local pages dir: %w", err)
	}

	return sm.saveBatchPages(pages, start)
}

// SyncFromZip downloads the full TLDR database archive and imports it
func (sm *SyncManager) SyncFromZip(ctx context.Context, zipURL string) (*SyncResult, error) {
	start := time.Now()
	sm.log.Info("downloading full tldr archive", "url", zipURL)

	req, err := http.NewRequestWithContext(ctx, "GET", zipURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := sm.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code downloading zip: %d", resp.StatusCode)
	}

	// Read full zip into memory
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip body: %w", err)
	}

	sm.log.Info("archive downloaded", "size", len(body))

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip file: %w", err)
	}

	var pages []*Page

	for _, f := range zipReader.File {
		// Only parse .md files
		if !strings.HasSuffix(f.Name, ".md") {
			continue
		}

		parts := strings.Split(f.Name, "/")
		if len(parts) < 3 {
			continue
		}

		fileName := parts[len(parts)-1]
		platform := parts[len(parts)-2]
		langDir := parts[len(parts)-3]

		// For github release tldr.zip, valid pages are right under `pages/` or `pages.xx/`
		language := "en"
		if strings.HasPrefix(langDir, "pages.") {
			language = strings.TrimPrefix(langDir, "pages.")
		} else if langDir != "pages" {
			continue
		}

		command := strings.TrimSuffix(fileName, ".md")

		rc, err := f.Open()
		if err != nil {
			sm.log.Warn("failed to open file in zip", "file", f.Name, "error", err)
			continue
		}

		contentBytes, err := io.ReadAll(rc)
		rc.Close()

		if err != nil {
			sm.log.Warn("failed to read file in zip", "file", f.Name, "error", err)
			continue
		}

		page := sm.client.parsePage(string(contentBytes), command, platform, language)
		pages = append(pages, page)
	}

	return sm.saveBatchPages(pages, start)
}

func (sm *SyncManager) saveBatchPages(pages []*Page, start time.Time) (*SyncResult, error) {
	sm.log.Info("parsed pages from source", "count", len(pages))

	batchSize := 500
	var savedCount int
	var errors []error

	for i := 0; i < len(pages); i += batchSize {
		end := i + batchSize
		if end > len(pages) {
			end = len(pages)
		}

		batch := pages[i:end]
		if err := sm.storage.SavePages(batch); err != nil {
			errors = append(errors, fmt.Errorf("failed to save batch [%d:%d]: %w", i, end, err))
			sm.log.Warn("batch save failed", "error", err)
		} else {
			savedCount += len(batch)
		}
	}

	result := &SyncResult{
		Downloaded: savedCount,
		Failed:     len(pages) - savedCount,
		Errors:     errors,
		Duration:   time.Since(start),
	}

	// Update metadata
	meta := &Metadata{
		LastSync:   time.Now(),
		TotalPages: savedCount,
		Platforms:  []string{PlatformCommon, PlatformLinux, PlatformMacOS, PlatformWindows, PlatformAndroid, PlatformFreeBSD, PlatformNetBSD, PlatformOpenBSD, PlatformSunOS},
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
			if strings.Contains(res.Error(), "page not found") {
				result.Skipped++
			} else {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Errorf("%s: %w", opts.Commands[i], res))
				sm.log.Warn("failed to sync command", "command", opts.Commands[i], "error", res)
			}
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
		end := min(i+batchSize, len(commands))

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
// We overwrite this to enforce complete download, solving "page not found" errors
func (sm *SyncManager) SyncPopular(ctx context.Context) (*SyncResult, error) {
	sm.log.Info("SyncPopular was requested, upgrading to full sync for better offline support")
	return sm.SyncAll(ctx)
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
