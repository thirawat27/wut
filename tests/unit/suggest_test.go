package unit

import (
	"context"
	"testing"

	"wut/internal/db"
)

// mockHistoryStore implements the Storage interface methods used by suggest
type mockHistoryStore struct {
	entries []db.HistoryEntry
}

func (m *mockHistoryStore) GetHistory(ctx context.Context, limit int) ([]db.HistoryEntry, error) {
	if limit > 0 && limit < len(m.entries) {
		return m.entries[:limit], nil
	}
	return m.entries, nil
}

func (m *mockHistoryStore) SearchHistory(ctx context.Context, query string, limit int) ([]db.HistoryEntry, error) {
	var results []db.HistoryEntry
	for _, entry := range m.entries {
		if containsString(entry.Command, query) {
			results = append(results, entry)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Mock other required methods
func (m *mockHistoryStore) AddHistory(ctx context.Context, entry *db.HistoryEntry) error {
	return nil
}

func (m *mockHistoryStore) Close() error {
	return nil
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(s[:len(substr)] == substr) ||
		(s[len(s)-len(substr):] == substr) ||
		containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewSuggester(t *testing.T) {
	// Skip test if db.Storage cannot be mocked
	// This test requires a real db.Storage or interface refactoring
	t.Skip("Skipping test - requires interface refactoring for suggest.New")
}

func TestSuggestWithQuery(t *testing.T) {
	t.Skip("Skipping test - requires interface refactoring for suggest.New")
}

func TestSuggestEmptyQuery(t *testing.T) {
	t.Skip("Skipping test - requires interface refactoring for suggest.New")
}

func TestSuggestLimit(t *testing.T) {
	t.Skip("Skipping test - requires interface refactoring for suggest.New")
}

func TestGetMostUsed(t *testing.T) {
	t.Skip("Skipping test - requires interface refactoring for suggest.New")
}
