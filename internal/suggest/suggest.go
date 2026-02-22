// Package suggest provides command suggestions based on history and fuzzy matching
// This is a lightweight replacement for the AI-based suggestion system
package suggest

import (
	"context"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"wut/internal/db"
)

// Suggester provides command suggestions
type Suggester struct {
	storage *db.Storage
}

// Result represents a suggestion result
type Result struct {
	Command     string
	Score       float64
	Description string
	Source      string // "history", "fuzzy", "common"
}

// New creates a new suggester
func New(storage *db.Storage) *Suggester {
	return &Suggester{
		storage: storage,
	}
}

// Suggest returns command suggestions based on query
func (s *Suggester) Suggest(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}

	// Get all history entries
	entries, err := s.storage.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}

	// Score and rank suggestions
	results := s.scoreSuggestions(query, entries)

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// scoreSuggestions scores history entries based on query match
func (s *Suggester) scoreSuggestions(query string, entries []db.HistoryEntry) []Result {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]Result, 0, len(entries))
	seen := make(map[string]bool)

	for _, entry := range entries {
		cmd := entry.Command
		cmdLower := strings.ToLower(cmd)

		// Skip duplicates
		if seen[cmd] {
			continue
		}
		seen[cmd] = true

		score := 0.0
		source := "history"

		if query == "" {
			// No query - rank by usage frequency
			score = float64(entry.UsageCount) * 10.0
			source = "history"
		} else if cmdLower == query {
			// Exact match - highest score
			score = 1000.0
			source = "exact"
		} else if strings.HasPrefix(cmdLower, query) {
			// Prefix match - high score
			score = 500.0 + float64(entry.UsageCount)*5.0
			source = "prefix"
		} else if strings.Contains(cmdLower, query) {
			// Substring match - medium score
			score = 300.0 + float64(entry.UsageCount)*3.0
			source = "substring"
		} else {
			// Fuzzy match - calculate Levenshtein distance
			distance := levenshtein.ComputeDistance(query, cmdLower)
			maxLen := max(len(cmdLower), len(query))
			if maxLen > 0 && distance <= maxLen/2 {
				// Similar enough - give it a score
				similarity := 1.0 - float64(distance)/float64(maxLen)
				score = similarity * 100.0 * float64(entry.UsageCount)
				source = "fuzzy"
			}
		}

		if score > 0 {
			results = append(results, Result{
				Command: cmd,
				Score:   score,
				Source:  source,
			})
		}
	}

	// Add common commands if query is provided and we have few results
	if query != "" && len(results) < 3 {
		commonCmds := getCommonCommands(query)
		for _, cmd := range commonCmds {
			if !seen[cmd] {
				results = append(results, Result{
					Command: cmd,
					Score:   50.0,
					Source:  "common",
				})
				seen[cmd] = true
			}
		}
	}

	return results
}

// getCommonCommands returns common commands that match the query
func getCommonCommands(query string) []string {
	query = strings.ToLower(query)
	common := []string{
		"git status", "git log", "git add", "git commit", "git push", "git pull",
		"ls -la", "ls -lh", "cd ~", "pwd", "cat", "less", "more",
		"grep -r", "find .", "rm -rf", "cp -r", "mv", "mkdir -p",
		"docker ps", "docker build", "docker run", "docker-compose up",
		"npm install", "npm run", "npm test", "npm start",
		"go build", "go test", "go run", "go mod tidy",
		"python", "python3", "pip install", "pip list",
		"kubectl get", "kubectl apply", "kubectl delete",
		"ssh", "scp", "rsync", "curl", "wget",
		"tar -xzf", "tar -czf", "zip", "unzip",
		"chmod +x", "chmod 755", "chown",
		"ps aux", "top", "htop", "df -h", "du -sh",
	}

	var matches []string
	for _, cmd := range common {
		cmdLower := strings.ToLower(cmd)
		if strings.Contains(cmdLower, query) || levenshtein.ComputeDistance(query, cmdLower) <= 3 {
			matches = append(matches, cmd)
		}
	}

	return matches
}

// GetMostUsed returns the most frequently used commands
func (s *Suggester) GetMostUsed(ctx context.Context, limit int) ([]Result, error) {
	entries, err := s.storage.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}

	// Sort by usage count
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UsageCount > entries[j].UsageCount
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	results := make([]Result, len(entries))
	for i, entry := range entries {
		results[i] = Result{
			Command: entry.Command,
			Score:   float64(entry.UsageCount),
			Source:  "history",
		}
	}

	return results, nil
}

// Close closes the suggester
func (s *Suggester) Close() error {
	return nil
}
