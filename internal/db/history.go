// Package db provides database functionality for WUT
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

const historyBucketName = "command_history"

// HistoryEntry represents a command history entry
type HistoryEntry struct {
	Command     string    `json:"command"`
	Description string    `json:"description"`
	UsageCount  int       `json:"usage_count"`
	LastUsed    time.Time `json:"last_used"`
}

// HistoryStats represents history statistics
type HistoryStats struct {
	TotalCommands   int
	UniqueCommands  int
	MostUsedCommand string
	MostUsedCount   int
	AverageUsage    float64
	TopCommands     []CommandStat
	TopCategories   []CategoryStat
}

// CommandStat represents a command statistic
type CommandStat struct {
	Command string
	Count   int
}

// CategoryStat represents a category statistic
type CategoryStat struct {
	Name  string
	Count int
}

// AddHistory adds a command to the history
func (s *Storage) AddHistory(ctx context.Context, command string) error {
	if s == nil {
		return fmt.Errorf("storage is nil")
	}
	if s.db == nil {
		return fmt.Errorf("storage database not initialized")
	}

	// Check if command already exists
	key := fmt.Sprintf("history/%s", command)

	var entry HistoryEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(historyBucketName))
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}
		data := bucket.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(data, &entry)
	})

	if err == nil {
		// Update existing entry
		entry.UsageCount++
		entry.LastUsed = time.Now()
	} else {
		// Create new entry
		entry = HistoryEntry{
			Command:     command,
			Description: "",
			UsageCount:  1,
			LastUsed:    time.Now(),
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal history entry: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(historyBucketName))
		if err != nil {
			return fmt.Errorf("create history bucket: %w", err)
		}
		return bucket.Put([]byte(key), data)
	})
}

// GetHistory retrieves command history entries
func (s *Storage) GetHistory(ctx context.Context, limit int) ([]HistoryEntry, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	var entries []HistoryEntry

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(historyBucketName))
		if bucket == nil {
			return nil // No history yet
		}

		return bucket.ForEach(func(k, v []byte) error {
			var entry HistoryEntry
			if err := json.Unmarshal(v, &entry); err == nil {
				entries = append(entries, entry)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// Sort by last used (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})

	// Apply limit if specified
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// SearchHistory searches history entries by command text
func (s *Storage) SearchHistory(ctx context.Context, query string, limit int) ([]HistoryEntry, error) {
	allEntries, err := s.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []HistoryEntry

	for _, entry := range allEntries {
		if strings.Contains(strings.ToLower(entry.Command), queryLower) {
			results = append(results, entry)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// ClearHistory clears all command history
func (s *Storage) ClearHistory(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage not initialized")
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		// Delete and recreate the bucket
		if err := tx.DeleteBucket([]byte(historyBucketName)); err != nil {
			// Ignore error if bucket doesn't exist
			if !strings.Contains(err.Error(), "bucket not found") {
				return err
			}
		}
		_, err := tx.CreateBucket([]byte(historyBucketName))
		return err
	})
}

// ExportHistory exports history to a JSON file
func (s *Storage) ExportHistory(ctx context.Context, filepath string) error {
	entries, err := s.GetHistory(ctx, 0)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(filepath, data, 0644)
}

// ImportHistory imports history from a JSON file
func (s *Storage) ImportHistory(ctx context.Context, filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse history: %w", err)
	}

	for _, entry := range entries {
		if err := s.AddHistory(ctx, entry.Command); err != nil {
			return err
		}
	}

	return nil
}

// GetHistoryStats returns statistics about command history
func (s *Storage) GetHistoryStats(ctx context.Context) (*HistoryStats, error) {
	entries, err := s.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}

	stats := &HistoryStats{
		UniqueCommands: len(entries),
		TopCommands:    []CommandStat{},
		TopCategories:  []CategoryStat{},
	}

	if len(entries) == 0 {
		return stats, nil
	}

	// Calculate totals and find most used
	totalUsage := 0
	maxCount := 0
	for _, entry := range entries {
		stats.TotalCommands += entry.UsageCount
		totalUsage += entry.UsageCount

		if entry.UsageCount > maxCount {
			maxCount = entry.UsageCount
			stats.MostUsedCommand = entry.Command
			stats.MostUsedCount = entry.UsageCount
		}
	}

	if stats.UniqueCommands > 0 {
		stats.AverageUsage = float64(totalUsage) / float64(stats.UniqueCommands)
	}

	// Sort by usage count for top commands
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UsageCount > entries[j].UsageCount
	})

	// Get top 10 commands
	limit := min(len(entries), 10)
	for i := 0; i < limit; i++ {
		stats.TopCommands = append(stats.TopCommands, CommandStat{
			Command: entries[i].Command,
			Count:   entries[i].UsageCount,
		})
	}

	return stats, nil
}
