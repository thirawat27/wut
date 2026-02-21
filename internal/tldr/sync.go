// Package tldr provides TLDR Pages sync functionality
package tldr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"wut/internal/logger"
)

// SyncManager manages syncing TLDR pages to local storage
type SyncManager struct {
	client  *Client
	storage *Storage
	log     *logger.Logger
}

// SyncOptions contains options for syncing
type SyncOptions struct {
	Platforms   []string
	Commands    []string
	Concurrency int
	ForceUpdate bool
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
	return &SyncManager{
		client:  NewClient(),
		storage: storage,
		log:     logger.With("tldr-sync"),
	}
}

// SetClient sets a custom client (useful for testing)
func (sm *SyncManager) SetClient(client *Client) {
	sm.client = client
}

// SyncAll syncs all common commands to local storage
func (sm *SyncManager) SyncAll(ctx context.Context) (*SyncResult, error) {
	return sm.SyncCommands(ctx, nil)
}

// SyncCommands syncs specific commands to local storage
func (sm *SyncManager) SyncCommands(ctx context.Context, commands []string) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{}

	// If no commands specified, get popular ones
	if len(commands) == 0 {
		var err error
		commands, err = sm.client.GetAvailableCommands(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get command list: %w", err)
		}
	}

	sm.log.Info("starting sync", "commands", len(commands))

	// Use worker pool for concurrent downloads
	concurrency := 5
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, cmd := range commands {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := sm.syncCommand(ctx, command)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Errorf("%s: %w", command, err))
				sm.log.Warn("failed to sync command", "command", command, "error", err)
			} else {
				result.Downloaded++
			}
		}(cmd)
	}

	wg.Wait()
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

// syncCommand syncs a single command
func (sm *SyncManager) syncCommand(ctx context.Context, command string) error {
	page, err := sm.client.GetPageAnyPlatform(ctx, command)
	if err != nil {
		return err
	}

	return sm.storage.SavePage(page)
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
