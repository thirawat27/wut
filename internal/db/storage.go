// Package db provides database storage for WUT
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

var (
	// Default bucket names
	commandsBucket  = []byte("commands")
	historyBucket   = []byte("history")
	configBucket    = []byte("config")
	modelsBucket    = []byte("models")
)

// Storage provides database operations
type Storage struct {
	db     *bbolt.DB
	path   string
	closed bool
}

// HistoryEntry represents a command history entry
type HistoryEntry struct {
	Command     string    `json:"command"`
	Description string    `json:"description,omitempty"`
	UsageCount  int       `json:"usage_count"`
	LastUsed    time.Time `json:"last_used"`
	FirstUsed   time.Time `json:"first_used"`
	Directory   string    `json:"directory,omitempty"`
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

// CommandStat represents command statistics
type CommandStat struct {
	Command string
	Count   int
}

// CategoryStat represents category statistics
type CategoryStat struct {
	Name  string
	Count int
}

// NewStorage creates a new storage instance
func NewStorage(path string) (*Storage, error) {
	// Expand path
	if path[:2] == "~/." || path[:2] == "~\\" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[2:])
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database
	db, err := bbolt.Open(path, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range [][]byte{commandsBucket, historyBucket, configBucket, modelsBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return &Storage{
		db:   db,
		path: path,
	}, nil
}

// Close closes the database
func (s *Storage) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// AddHistory adds a command to history
func (s *Storage) AddHistory(ctx context.Context, command string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(historyBucket)
		
		// Check if command already exists
		existing := bucket.Get([]byte(command))
		var entry HistoryEntry
		
		if existing != nil {
			if err := json.Unmarshal(existing, &entry); err != nil {
				return err
			}
			entry.UsageCount++
			entry.LastUsed = time.Now()
		} else {
			entry = HistoryEntry{
				Command:   command,
				UsageCount: 1,
				FirstUsed: time.Now(),
				LastUsed:  time.Now(),
			}
		}
		
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		
		return bucket.Put([]byte(command), data)
	})
}

// GetHistory retrieves command history
func (s *Storage) GetHistory(ctx context.Context, limit int) ([]HistoryEntry, error) {
	var entries []HistoryEntry
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(historyBucket)
		
		return bucket.ForEach(func(k, v []byte) error {
			var entry HistoryEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
			
			if limit > 0 && len(entries) >= limit {
				return nil
			}
			return nil
		})
	})
	
	return entries, err
}

// SearchHistory searches history
func (s *Storage) SearchHistory(ctx context.Context, query string, limit int) ([]HistoryEntry, error) {
	var entries []HistoryEntry
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(historyBucket)
		
		return bucket.ForEach(func(k, v []byte) error {
			if contains(string(k), query) {
				var entry HistoryEntry
				if err := json.Unmarshal(v, &entry); err != nil {
					return err
				}
				entries = append(entries, entry)
				
				if limit > 0 && len(entries) >= limit {
					return nil
				}
			}
			return nil
		})
	})
	
	return entries, err
}

// ClearHistory clears all history
func (s *Storage) ClearHistory(ctx context.Context) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(historyBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucket(historyBucket)
		return err
	})
}

// GetHistoryCount returns the number of history entries
func (s *Storage) GetHistoryCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(historyBucket)
		return bucket.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
	})
	return count, err
}

// GetHistoryStats returns history statistics
func (s *Storage) GetHistoryStats(ctx context.Context) (*HistoryStats, error) {
	entries, err := s.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}
	
	stats := &HistoryStats{
		UniqueCommands: len(entries),
		TopCommands:    make([]CommandStat, 0),
	}
	
	totalUsage := 0
	for _, entry := range entries {
		stats.TotalCommands += entry.UsageCount
		totalUsage += entry.UsageCount
		
		if entry.UsageCount > stats.MostUsedCount {
			stats.MostUsedCount = entry.UsageCount
			stats.MostUsedCommand = entry.Command
		}
		
		stats.TopCommands = append(stats.TopCommands, CommandStat{
			Command: entry.Command,
			Count:   entry.UsageCount,
		})
	}
	
	if len(entries) > 0 {
		stats.AverageUsage = float64(totalUsage) / float64(len(entries))
	}
	
	return stats, nil
}

// GetCommandHistory gets command history as string slice
func (s *Storage) GetCommandHistory(ctx context.Context, limit int) ([]string, error) {
	entries, err := s.GetHistory(ctx, limit)
	if err != nil {
		return nil, err
	}
	
	commands := make([]string, len(entries))
	for i, entry := range entries {
		commands[i] = entry.Command
	}
	return commands, nil
}

// ExportHistory exports history to a file
func (s *Storage) ExportHistory(ctx context.Context, path string) error {
	entries, err := s.GetHistory(ctx, 0)
	if err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}

// ImportHistory imports history from a file
func (s *Storage) ImportHistory(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	
	for _, entry := range entries {
		if err := s.AddHistory(ctx, entry.Command); err != nil {
			return err
		}
	}
	
	return nil
}

// GetConfig gets a configuration value
func (s *Storage) GetConfig(key string) ([]byte, error) {
	var value []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(configBucket)
		v := bucket.Get([]byte(key))
		if v != nil {
			value = append([]byte{}, v...)
		}
		return nil
	})
	return value, err
}

// SetConfig sets a configuration value
func (s *Storage) SetConfig(key string, value []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(configBucket)
		return bucket.Put([]byte(key), value)
	})
}

// SaveModel saves a model to the database
func (s *Storage) SaveModel(name string, data []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(modelsBucket)
		return bucket.Put([]byte(name), data)
	})
}

// LoadModel loads a model from the database
func (s *Storage) LoadModel(name string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(modelsBucket)
		v := bucket.Get([]byte(name))
		if v != nil {
			data = append([]byte{}, v...)
		}
		return nil
	})
	return data, err
}

// GetTrainingData returns training data from history
func (s *Storage) GetTrainingData(ctx context.Context) (*TrainingData, error) {
	entries, err := s.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}
	
	data := &TrainingData{
		Commands: make([]string, len(entries)),
		Counts:   make([]int, len(entries)),
	}
	
	for i, entry := range entries {
		data.Commands[i] = entry.Command
		data.Counts[i] = entry.UsageCount
	}
	
	return data, nil
}

// TrainingData represents training data
type TrainingData struct {
	Commands []string
	Counts   []int
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(s[:len(substr)] == substr) ||
		(s[len(s)-len(substr):] == substr) ||
		containsSubstring(s, substr))
}

// containsSubstring checks if s contains substr anywhere
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
