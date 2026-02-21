// Package history provides shell history reading functionality
package history

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"wut/internal/db"
	"wut/internal/logger"
)

// ShellType represents the type of shell
type ShellType string

const (
	ShellBash       ShellType = "bash"
	ShellZsh        ShellType = "zsh"
	ShellFish       ShellType = "fish"
	ShellPowerShell ShellType = "powershell"
	ShellUnknown    ShellType = "unknown"
)

// Entry represents a single history entry
type Entry struct {
	Command   string
	Timestamp time.Time
	Shell     ShellType
	Raw       string
}

// Reader provides concurrent shell history reading
type Reader struct {
	workers int
	mu      sync.RWMutex
	cache   map[string][]Entry
}

// ReaderOption is a functional option for Reader
type ReaderOption func(*Reader)

// WithWorkers sets the number of concurrent workers
func WithWorkers(n int) ReaderOption {
	return func(r *Reader) {
		if n > 0 {
			r.workers = n
		}
	}
}

// NewReader creates a new history reader
func NewReader(opts ...ReaderOption) *Reader {
	r := &Reader{
		workers: runtime.NumCPU(),
		cache:   make(map[string][]Entry),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// DetectShells returns detected shells and their history file paths
func (r *Reader) DetectShells() map[ShellType]string {
	shells := make(map[ShellType]string)
	home, _ := os.UserHomeDir()

	// Detect bash
	if path := r.detectBashHistory(home); path != "" {
		shells[ShellBash] = path
	}

	// Detect zsh
	if path := r.detectZshHistory(home); path != "" {
		shells[ShellZsh] = path
	}

	// Detect fish
	if path := r.detectFishHistory(home); path != "" {
		shells[ShellFish] = path
	}

	// Detect PowerShell
	if path := r.detectPowerShellHistory(home); path != "" {
		shells[ShellPowerShell] = path
	}

	return shells
}

// ReadAll reads history from all detected shells concurrently
func (r *Reader) ReadAll(ctx context.Context) ([]Entry, error) {
	log := logger.With("history.reader")

	shells := r.DetectShells()
	if len(shells) == 0 {
		log.Warn("no shell history files detected")
		return nil, fmt.Errorf("no shell history files found")
	}

	log.Info("detected shells", "count", len(shells))

	// Create channels for concurrent processing
	entriesChan := make(chan []Entry, len(shells))
	errChan := make(chan error, len(shells))

	// Use WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Limit concurrent workers with semaphore
	sem := make(chan struct{}, r.workers)

	// Read from each shell concurrently
	for shellType, path := range shells {
		wg.Add(1)
		go func(st ShellType, p string) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			entries, err := r.ReadFromShell(ctx, st, p)
			if err != nil {
				log.Warn("failed to read shell history", "shell", st, "path", p, "error", err)
				errChan <- err
				return
			}

			log.Info("read shell history", "shell", st, "entries", len(entries))
			entriesChan <- entries
		}(shellType, path)
	}

	// Close channels when done
	go func() {
		wg.Wait()
		close(entriesChan)
		close(errChan)
	}()

	// Collect results
	var allEntries []Entry
	for entries := range entriesChan {
		allEntries = append(allEntries, entries...)
	}

	// Check for errors (non-blocking)
	select {
	case err := <-errChan:
		if err != nil {
			// Log but continue - partial results are still useful
			log.Debug("some shell histories failed to read", "error", err)
		}
	default:
	}

	return allEntries, nil
}

// ReadFromShell reads history from a specific shell
func (r *Reader) ReadFromShell(ctx context.Context, shell ShellType, path string) ([]Entry, error) {
	log := logger.With("history.reader")

	// Check cache first
	r.mu.RLock()
	if cached, ok := r.cache[path]; ok {
		r.mu.RUnlock()
		log.Debug("using cached history", "path", path, "entries", len(cached))
		return cached, nil
	}
	r.mu.RUnlock()

	// Read file
	entries, err := r.parseShellFile(ctx, shell, path)
	if err != nil {
		return nil, err
	}

	// Cache results
	r.mu.Lock()
	r.cache[path] = entries
	r.mu.Unlock()

	return entries, nil
}

// ImportToDB imports history entries to the database concurrently
func (r *Reader) ImportToDB(ctx context.Context, storage *db.Storage, entries []Entry) (int, error) {
	log := logger.With("history.importer")

	if len(entries) == 0 {
		return 0, nil
	}

	// Deduplicate entries
	deduped := r.deduplicate(entries)
	log.Info("importing history", "total", len(entries), "unique", len(deduped))

	// Use worker pool for concurrent database writes
	jobs := make(chan Entry, len(deduped))
	results := make(chan error, len(deduped))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				// Skip empty or very short commands
				if len(entry.Command) < 2 {
					results <- nil
					continue
				}

				// Skip sensitive commands
				if r.isSensitive(entry.Command) {
					results <- nil
					continue
				}

				if err := storage.AddHistory(ctx, entry.Command); err != nil {
					results <- err
				} else {
					results <- nil
				}
			}
		}()
	}

	// Send jobs
	go func() {
		for _, entry := range deduped {
			jobs <- entry
		}
		close(jobs)
	}()

	// Wait and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Count successes
	successCount := 0
	errors := 0
	for err := range results {
		if err == nil {
			successCount++
		} else {
			errors++
		}
	}

	if errors > 0 {
		log.Warn("some entries failed to import", "errors", errors)
	}

	return successCount, nil
}

// parseShellFile parses a shell history file based on shell type
func (r *Reader) parseShellFile(ctx context.Context, shell ShellType, path string) ([]Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	switch shell {
	case ShellBash:
		return r.parseBashHistory(ctx, file)
	case ShellZsh:
		return r.parseZshHistory(ctx, file)
	case ShellFish:
		return r.parseFishHistory(ctx, file)
	case ShellPowerShell:
		return r.parsePowerShellHistory(ctx, file)
	default:
		return nil, fmt.Errorf("unsupported shell type: %s", shell)
	}
}

// parseBashHistory parses bash history file
func (r *Reader) parseBashHistory(ctx context.Context, file *os.File) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(file)

	// Bash history format: simple list of commands
	// With HISTTIMEFORMAT: #1234567890\ncommand
	var currentTime time.Time

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for timestamp line (starts with #)
		if strings.HasPrefix(line, "#") {
			if ts, err := parseTimestamp(line[1:]); err == nil {
				currentTime = ts
			}
			continue
		}

		// Skip common non-command lines
		if shouldSkipCommand(line) {
			continue
		}

		entries = append(entries, Entry{
			Command:   line,
			Timestamp: currentTime,
			Shell:     ShellBash,
			Raw:       line,
		})

		// Reset timestamp for next entry
		currentTime = time.Time{}
	}

	return entries, scanner.Err()
}

// parseZshHistory parses zsh history file (extended format)
func (r *Reader) parseZshHistory(ctx context.Context, file *os.File) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(file)

	// Zsh extended format: : timestamp:0;command
	zshRegex := regexp.MustCompile(`^: (\d+):\d+;(.*)$`)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		line := scanner.Text()

		// Try extended format first
		if matches := zshRegex.FindStringSubmatch(line); len(matches) == 3 {
			timestamp, _ := parseTimestamp(matches[1])
			command := matches[2]

			if !shouldSkipCommand(command) {
				entries = append(entries, Entry{
					Command:   command,
					Timestamp: timestamp,
					Shell:     ShellZsh,
					Raw:       line,
				})
			}
		} else if !strings.HasPrefix(line, ":") && !shouldSkipCommand(line) {
			// Simple format - just command
			entries = append(entries, Entry{
				Command:   line,
				Timestamp: time.Time{},
				Shell:     ShellZsh,
				Raw:       line,
			})
		}
	}

	return entries, scanner.Err()
}

// parseFishHistory parses fish history file (YAML-like format)
func (r *Reader) parseFishHistory(ctx context.Context, file *os.File) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(file)

	var currentEntry Entry
	currentEntry.Shell = ShellFish

	// Fish format is YAML-like:
	// - cmd: command
	//   when: timestamp

	cmdRegex := regexp.MustCompile(`^\s*- cmd:\s*(.+)$`)
	whenRegex := regexp.MustCompile(`^\s*when:\s*(\d+)$`)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		line := scanner.Text()

		if matches := cmdRegex.FindStringSubmatch(line); len(matches) == 2 {
			// Save previous entry if exists
			if currentEntry.Command != "" && !shouldSkipCommand(currentEntry.Command) {
				entries = append(entries, currentEntry)
			}
			// Start new entry
			currentEntry = Entry{
				Command: matches[1],
				Shell:   ShellFish,
				Raw:     line,
			}
		} else if matches := whenRegex.FindStringSubmatch(line); len(matches) == 2 {
			if ts, err := parseTimestamp(matches[1]); err == nil {
				currentEntry.Timestamp = ts
			}
		}
	}

	// Don't forget the last entry
	if currentEntry.Command != "" && !shouldSkipCommand(currentEntry.Command) {
		entries = append(entries, currentEntry)
	}

	return entries, scanner.Err()
}

// parsePowerShellHistory parses PowerShell history file (JSON format)
func (r *Reader) parsePowerShellHistory(ctx context.Context, file *os.File) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(file)

	// PowerShell (PSReadLine) format is JSON:
	// {"CommandLine": "command", "ExecutionStatus": "Completed", "StartExecutionTime": "..."}

	cmdRegex := regexp.MustCompile(`"CommandLine"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	timeRegex := regexp.MustCompile(`"StartExecutionTime"\s*:\s*"([^"]+)"`)

	var currentCmd string
	var currentTime time.Time

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		line := scanner.Text()

		if matches := cmdRegex.FindStringSubmatch(line); len(matches) == 2 {
			currentCmd = unescapeJSON(matches[1])
		}

		if matches := timeRegex.FindStringSubmatch(line); len(matches) == 2 {
			if t, err := time.Parse(time.RFC3339, matches[1]); err == nil {
				currentTime = t
			}
		}

		// End of JSON object
		if strings.TrimSpace(line) == "}" && currentCmd != "" {
			if !shouldSkipCommand(currentCmd) {
				entries = append(entries, Entry{
					Command:   currentCmd,
					Timestamp: currentTime,
					Shell:     ShellPowerShell,
					Raw:       currentCmd,
				})
			}
			currentCmd = ""
			currentTime = time.Time{}
		}
	}

	return entries, scanner.Err()
}

// detectBashHistory detects bash history file path
func (r *Reader) detectBashHistory(home string) string {
	// Check HISTFILE environment variable
	if histfile := os.Getenv("HISTFILE"); histfile != "" {
		if _, err := os.Stat(histfile); err == nil {
			return histfile
		}
	}

	// Default location
	path := filepath.Join(home, ".bash_history")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}

// detectZshHistory detects zsh history file path
func (r *Reader) detectZshHistory(home string) string {
	// Check HISTFILE environment variable
	if histfile := os.Getenv("HISTFILE"); histfile != "" && strings.Contains(histfile, "zsh") {
		if _, err := os.Stat(histfile); err == nil {
			return histfile
		}
	}

	// Default location
	path := filepath.Join(home, ".zsh_history")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}

// detectFishHistory detects fish history file path
func (r *Reader) detectFishHistory(home string) string {
	path := filepath.Join(home, ".local", "share", "fish", "fish_history")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Alternative location on macOS
	if runtime.GOOS == "darwin" {
		path = filepath.Join(home, "Library", "Application Support", "fish", "fish_history")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// detectPowerShellHistory detects PowerShell history file path
func (r *Reader) detectPowerShellHistory(home string) string {
	// PowerShell 7+ (cross-platform)
	path := filepath.Join(home, ".local", "share", "powershell", "PSReadLine", "ConsoleHost_history.txt")
	if runtime.GOOS == "windows" {
		path = filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
	}

	if _, err := os.Stat(path); err == nil {
		return path
	}

	// PowerShell 5.1 (Windows only)
	if runtime.GOOS == "windows" {
		path = filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// deduplicate removes duplicate commands, keeping the most recent
func (r *Reader) deduplicate(entries []Entry) []Entry {
	seen := make(map[string]Entry)

	for _, entry := range entries {
		if existing, ok := seen[entry.Command]; ok {
			// Keep the one with newer timestamp
			if entry.Timestamp.After(existing.Timestamp) {
				seen[entry.Command] = entry
			}
		} else {
			seen[entry.Command] = entry
		}
	}

	// Convert back to slice
	result := make([]Entry, 0, len(seen))
	for _, entry := range seen {
		result = append(result, entry)
	}

	return result
}

// isSensitive checks if a command contains sensitive information
func (r *Reader) isSensitive(command string) bool {
	sensitivePatterns := []string{
		"password", "passwd", "secret", "token", "key",
		"api_key", "apikey", "private_key", "credential",
	}

	lower := strings.ToLower(command)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// shouldSkipCommand returns true if the command should be skipped
func shouldSkipCommand(cmd string) bool {
	if cmd == "" {
		return true
	}

	// Skip short commands
	if len(cmd) < 2 {
		return true
	}

	// Skip common non-useful commands
	skipPrefixes := []string{
		"ls", "cd", "pwd", "exit", "clear", "history",
		"fg", "bg", "jobs", "echo", "cat", "man",
	}

	lower := strings.ToLower(cmd)
	for _, prefix := range skipPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+" ") {
			return true
		}
	}

	return false
}

// parseTimestamp parses a Unix timestamp string
func parseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	// Try Unix timestamp
	var unixTs int64
	if _, err := fmt.Sscanf(s, "%d", &unixTs); err == nil {
		if unixTs > 1e12 {
			// Milliseconds
			unixTs = unixTs / 1000
		}
		return time.Unix(unixTs, 0), nil
	}

	return time.Time{}, fmt.Errorf("invalid timestamp: %s", s)
}

// unescapeJSON unescapes JSON string
func unescapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

// GetStats returns statistics about the history
func (r *Reader) GetStats(entries []Entry) HistoryStats {
	stats := HistoryStats{
		TotalEntries: len(entries),
		ByShell:      make(map[ShellType]int),
	}

	commands := make(map[string]int)

	for _, entry := range entries {
		stats.ByShell[entry.Shell]++
		commands[entry.Command]++

		if entry.Timestamp.After(stats.NewestTime) {
			stats.NewestTime = entry.Timestamp
		}
		if stats.OldestTime.IsZero() || entry.Timestamp.Before(stats.OldestTime) {
			stats.OldestTime = entry.Timestamp
		}
	}

	stats.UniqueCommands = len(commands)

	// Find most used command
	for cmd, count := range commands {
		if count > stats.MostUsedCount {
			stats.MostUsedCount = count
			stats.MostUsedCommand = cmd
		}
	}

	return stats
}

// HistoryStats represents history statistics
type HistoryStats struct {
	TotalEntries    int
	UniqueCommands  int
	MostUsedCommand string
	MostUsedCount   int
	ByShell         map[ShellType]int
	NewestTime      time.Time
	OldestTime      time.Time
}
