package suggest

import (
	"context"
	"sort"
	"strings"

	"wut/internal/db"

	"github.com/agnivade/levenshtein"
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

	entries, err := s.storage.GetHistory(ctx, 0)
	if err != nil {
		return nil, err
	}

	results := s.scoreSuggestions(query, entries)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// scoreSuggestions scores history entries based on query match
func (s *Suggester) scoreSuggestions(query string, entries []db.CommandExecution) []Result {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]Result, 0)

	freqs := make(map[string]int)
	for _, e := range entries {
		freqs[e.Command]++
	}

	seen := make(map[string]bool)

	for _, entry := range entries {
		cmd := entry.Command
		cmdLower := strings.ToLower(cmd)

		if seen[cmd] {
			continue
		}
		seen[cmd] = true

		score := 0.0
		source := "history"

		usageCount := freqs[cmd]

		if query == "" {
			score = float64(usageCount) * 10.0
			source = "history"
		} else if cmdLower == query {
			score = 1000.0
			source = "exact"
		} else if strings.HasPrefix(cmdLower, query) {
			score = 500.0 + float64(usageCount)*5.0
			source = "prefix"
		} else if strings.Contains(cmdLower, query) {
			score = 300.0 + float64(usageCount)*3.0
			source = "substring"
		} else {
			distance := levenshtein.ComputeDistance(query, cmdLower)
			maxLen := max(len(cmdLower), len(query))
			if maxLen > 0 && distance <= maxLen/2 {
				similarity := 1.0 - float64(distance)/float64(maxLen)
				score = similarity * 100.0 * float64(usageCount)
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

	freqs := make(map[string]int)
	for _, e := range entries {
		freqs[e.Command]++
	}

	var results []Result
	for cmd, count := range freqs {
		results = append(results, Result{
			Command: cmd,
			Score:   float64(count),
			Source:  "history",
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Suggester) Close() error {
	return nil
}
