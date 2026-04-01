package suggest

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"wut/internal/db"
	"wut/internal/historyml"

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

	summaries, err := s.storage.GetHistoryCommandSummaries(ctx, 0)
	if err != nil {
		return nil, err
	}

	results := s.scoreSuggestions(query, summaries)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// scoreSuggestions scores history entries based on query match
func (s *Suggester) scoreSuggestions(query string, summaries []db.HistoryCommandSummary) []Result {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]Result, 0)
	summaryMap := make(map[string]db.HistoryCommandSummary, len(summaries))

	freqs := make(map[string]int, len(summaries))
	for _, summary := range summaries {
		freqs[summary.Command] = summary.UsageCount
		summaryMap[summary.Command] = summary
	}

	ranker := historyml.Train(toHistorySamples(summaries), time.Now())

	for cmd, usageCount := range freqs {
		cmdLower := strings.ToLower(cmd)

		score := 0.0
		source := "history"

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
			lenDiff := len(cmdLower) - len(query)
			if lenDiff < 0 {
				lenDiff = -lenDiff
			}
			maxLen := len(cmdLower)
			if len(query) > maxLen {
				maxLen = len(query)
			}

			if maxLen > 0 && lenDiff <= maxLen/2 {
				distance := levenshtein.ComputeDistance(query, cmdLower)
				if distance <= maxLen/2 {
					similarity := 1.0 - float64(distance)/float64(maxLen)
					score = similarity * 100.0 * float64(usageCount)
					source = "fuzzy"
				}
			}
		}

		if score > 0 {
			summary := summaryMap[cmd]
			score += historyBoost(summary, ranker)
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
			if freqs[cmd] == 0 {
				results = append(results, Result{
					Command: cmd,
					Score:   50.0,
					Source:  "common",
				})
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
		if strings.Contains(cmd, query) {
			matches = append(matches, cmd)
			continue
		}

		lenDiff := len(cmd) - len(query)
		if lenDiff < 0 {
			lenDiff = -lenDiff
		}
		if lenDiff <= 3 {
			if levenshtein.ComputeDistance(query, cmd) <= 3 {
				matches = append(matches, cmd)
			}
		}
	}

	return matches
}

// GetMostUsed returns the most frequently used commands
func (s *Suggester) GetMostUsed(ctx context.Context, limit int) ([]Result, error) {
	summaries, err := s.storage.GetHistoryCommandSummaries(ctx, 0)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(summaries))
	ranker := historyml.Train(toHistorySamples(summaries), time.Now())
	for _, summary := range summaries {
		results = append(results, Result{
			Command: summary.Command,
			Score:   float64(summary.UsageCount) + historyBoost(summary, ranker),
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

func toHistorySamples(summaries []db.HistoryCommandSummary) []historyml.CommandSample {
	samples := make([]historyml.CommandSample, 0, len(summaries))
	for _, summary := range summaries {
		samples = append(samples, historyml.CommandSample{
			Command:     summary.Command,
			UsageCount:  summary.UsageCount,
			LastUsed:    summary.LastUsed,
			SourceOS:    summary.SourceOS,
			SourceShell: summary.SourceShell,
		})
	}
	return samples
}

func historyBoost(summary db.HistoryCommandSummary, ranker *historyml.Ranker) float64 {
	boost := math.Log1p(float64(summary.UsageCount)) * 8
	if ranker == nil {
		return boost
	}
	return boost + ranker.Score(historyml.CommandSample{
		Command:     summary.Command,
		UsageCount:  summary.UsageCount,
		LastUsed:    summary.LastUsed,
		SourceOS:    summary.SourceOS,
		SourceShell: summary.SourceShell,
	})*40
}
