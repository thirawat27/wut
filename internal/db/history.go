package db

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"go.etcd.io/bbolt"
)

const historyBucketName = "command_execution_log"

// CommandExecution represents a single execution of a command
type CommandExecution struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	Dir       string    `json:"dir"`
	SessionID string    `json:"session_id"`
}

// HistoryStats represents statistics computed from the raw execution log
type HistoryStats struct {
	TotalExecutions  int
	UniqueCommands   int
	MostUsedCommand  string
	MostUsedCount    int
	TopCommands      []CommandStat
	TimeDistribution map[string]int
}

// CommandStat represents a command and its occurrence count
type CommandStat struct {
	Command string
	Count   int
}

// AddHistory adds a strictly logged command execution to the DB
func (s *Storage) AddHistory(ctx context.Context, command string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage not initialized")
	}

	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	now := time.Now()
	// Use UnixNano as ID to ensure strictly increasing sequential keys
	id := fmt.Sprintf("%020d", now.UnixNano())

	dir, _ := os.Getwd()
	sessionID := os.Getenv("WUT_SESSION_ID") // optional grouping

	exec := CommandExecution{
		ID:        id,
		Command:   command,
		Timestamp: now,
		Dir:       dir,
		SessionID: sessionID,
	}

	data, err := json.Marshal(exec)
	if err != nil {
		return fmt.Errorf("failed to marshal command execution: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(historyBucketName))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(id), data)
	})
}

// GetHistory retrieves command execution logs, newest first
func (s *Storage) GetHistory(ctx context.Context, limit int) ([]CommandExecution, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	var entries []CommandExecution

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(historyBucketName))
		if bucket == nil {
			return nil
		}

		// Cursor to iterate over keys, since ID is padded timestamp we can iterate in reverse
		c := bucket.Cursor()
		count := 0
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var entry CommandExecution
			if err := json.Unmarshal(v, &entry); err == nil {
				entries = append(entries, entry)
				count++
				if limit > 0 && count >= limit {
					break
				}
			}
		}
		return nil
	})

	return entries, err
}

// GetAllHistory retrieves all command executions
func (s *Storage) GetAllHistory(ctx context.Context) ([]CommandExecution, error) {
	return s.GetHistory(ctx, 0)
}

// SearchHistory searches history logs by command text
func (s *Storage) SearchHistory(ctx context.Context, query string, limit int) ([]CommandExecution, error) {
	allEntries, err := s.GetAllHistory(ctx)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []CommandExecution

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

// ClearHistory clears all recorded command execution logs
func (s *Storage) ClearHistory(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage not initialized")
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket([]byte(historyBucketName))
		// Support removing the legacy history bucket too
		_ = tx.DeleteBucket([]byte("command_history"))
		_, err := tx.CreateBucket([]byte(historyBucketName))
		return err
	})
}

// ExportHistory exports raw execution history to a JSON file
func (s *Storage) ExportHistory(ctx context.Context, filepath string) error {
	entries, err := s.GetAllHistory(ctx)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(filepath, data, 0644)
}

// ImportHistory imports execution log history from a JSON file
func (s *Storage) ImportHistory(ctx context.Context, filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var entries []CommandExecution
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse history: %w", err)
	}

	// Just add them sequentially
	for _, entry := range entries {
		if err := s.AddHistory(ctx, entry.Command); err != nil {
			return err
		}
	}

	return nil
}

// GetHistoryStats returns aggregated statistics about command history
func (s *Storage) GetHistoryStats(ctx context.Context) (*HistoryStats, error) {
	entries, err := s.GetAllHistory(ctx)
	if err != nil {
		return nil, err
	}

	stats := &HistoryStats{
		TotalExecutions:  len(entries),
		TopCommands:      []CommandStat{},
		TimeDistribution: make(map[string]int),
	}

	if len(entries) == 0 {
		return stats, nil
	}

	counts := make(map[string]int)
	for _, entry := range entries {
		counts[entry.Command]++

		hour := entry.Timestamp.Hour()
		if hour >= 6 && hour < 12 {
			stats.TimeDistribution["Morning (06:00-12:00)"]++
		} else if hour >= 12 && hour < 18 {
			stats.TimeDistribution["Afternoon (12:00-18:00)"]++
		} else if hour >= 18 && hour < 24 {
			stats.TimeDistribution["Evening (18:00-24:00)"]++
		} else {
			stats.TimeDistribution["Night (00:00-06:00)"]++
		}
	}

	stats.UniqueCommands = len(counts)

	var cmds []CommandStat
	for c, count := range counts {
		cmds = append(cmds, CommandStat{Command: c, Count: count})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Count > cmds[j].Count
	})

	if len(cmds) > 0 {
		stats.MostUsedCommand = cmds[0].Command
		stats.MostUsedCount = cmds[0].Count
	}

	limit := len(cmds)
	if limit > 10 {
		limit = 10
	}
	stats.TopCommands = cmds[:limit]

	return stats, nil
}
