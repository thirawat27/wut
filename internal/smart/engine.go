// Package smart provides intelligent command suggestions
package smart

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	appctx "wut/internal/context"
	"wut/internal/db"
	"wut/internal/performance"
)

// Engine provides intelligent command suggestions
type Engine struct {
	storage      *db.Storage
	matcher      *performance.FastMatcher
	cache        *performance.LRUCache[string, []Suggestion]
	ctxCache     *performance.LRUCache[string, *appctx.Context]
	index        *performance.InvertedIndex
	autocomplete *performance.Autocomplete

	// Scoring weights
	weights ScoringWeights

	mu sync.RWMutex
}

// ScoringWeights holds scoring weights for ranking
type ScoringWeights struct {
	ExactMatch       float64
	PrefixMatch      float64
	ContainsMatch    float64
	FuzzyMatch       float64
	HistoryFreq      float64
	Recency          float64
	ContextRelevance float64
}

// DefaultScoringWeights returns default weights
func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		ExactMatch:       1.0,
		PrefixMatch:      0.9,
		ContainsMatch:    0.7,
		FuzzyMatch:       0.5,
		HistoryFreq:      0.3,
		Recency:          0.2,
		ContextRelevance: 0.4,
	}
}

// Suggestion represents a command suggestion
type Suggestion struct {
	Command        string
	Description    string
	Score          float64
	Source         string
	Icon           string
	UsageCount     int
	LastUsed       time.Time
	ContextMatch   float64
	IsPerfectMatch bool
}

// NewEngine creates a new smart engine
func NewEngine(storage *db.Storage) *Engine {
	return &Engine{
		storage:      storage,
		matcher:      performance.NewFastMatcher(false, 0.3, 3),
		cache:        performance.NewLRUCache[string, []Suggestion](1000, 32),
		ctxCache:     performance.NewLRUCache[string, *appctx.Context](100, 8),
		index:        performance.NewInvertedIndex(),
		autocomplete: performance.NewAutocomplete(100),
		weights:      DefaultScoringWeights(),
	}
}

// SetWeights sets custom scoring weights
func (e *Engine) SetWeights(weights ScoringWeights) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.weights = weights
}

// Suggest returns intelligent command suggestions
func (e *Engine) Suggest(ctx context.Context, query string, contextData *appctx.Context, limit int) ([]Suggestion, error) {
	if limit <= 0 {
		limit = 10
	}

	// Check cache for exact query
	cacheKey := query + ":" + contextData.ProjectType
	if cached, ok := e.cache.Get(cacheKey); ok {
		return e.limitSuggestions(cached, limit), nil
	}

	// Collect suggestions from all sources concurrently
	suggestionChan := make(chan []Suggestion, 4)
	var wg sync.WaitGroup

	// 1. History-based suggestions
	wg.Go(func() {
		select {
		case suggestionChan <- e.getHistorySuggestions(ctx, query, limit):
		case <-ctx.Done():
		}
	})

	// 2. Context-specific suggestions
	wg.Go(func() {
		select {
		case suggestionChan <- e.getContextSuggestions(contextData, query):
		case <-ctx.Done():
		}
	})

	// 3. Common workflow suggestions
	wg.Go(func() {
		select {
		case suggestionChan <- e.getWorkflowSuggestions(contextData, query):
		case <-ctx.Done():
		}
	})

	// 4. Fuzzy matched suggestions
	wg.Go(func() {
		select {
		case suggestionChan <- e.getFuzzySuggestions(query, limit):
		case <-ctx.Done():
		}
	})

	// Close channel when done
	go func() {
		wg.Wait()
		close(suggestionChan)
	}()

	// Collect and deduplicate with context check
	suggestionMap := make(map[string]Suggestion)
	for {
		select {
		case suggestions, ok := <-suggestionChan:
			if !ok {
				// Channel closed, all workers done
				goto done
			}
			for _, s := range suggestions {
				if existing, ok := suggestionMap[s.Command]; ok {
					// Merge scores
					if s.Score > existing.Score {
						existing.Score = s.Score
					}
					existing.UsageCount += s.UsageCount
					suggestionMap[s.Command] = existing
				} else {
					suggestionMap[s.Command] = s
				}
			}
		case <-ctx.Done():
			// Context cancelled/timed out, return what we have
			goto done
		}
	}
done:

	// Convert to slice and sort
	results := make([]Suggestion, 0, len(suggestionMap))
	for _, s := range suggestionMap {
		results = append(results, s)
	}

	// Score and sort
	results = e.scoreAndSort(results, query, contextData)

	// Cache results
	e.cache.Set(cacheKey, results, 30*time.Second)

	return e.limitSuggestions(results, limit), nil
}

// getHistorySuggestions gets suggestions from command history
func (e *Engine) getHistorySuggestions(ctx context.Context, query string, limit int) []Suggestion {
	if e.storage == nil {
		return nil
	}

	// Check context before database call
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	// Use smaller limit for faster response
	historyLimit := 100
	if limit > 0 && limit < 100 {
		historyLimit = limit * 10
	}

	entries, err := e.storage.GetHistory(ctx, historyLimit)
	if err != nil {
		return nil
	}

	var suggestions []Suggestion
	now := time.Now()
	maxEntries := 50 // Limit processing

	for i, entry := range entries {
		if i >= maxEntries {
			break
		}
		// Check context periodically
		if i%10 == 0 {
			select {
			case <-ctx.Done():
				return suggestions
			default:
			}
		}

		score := e.calculateHistoryScore(entry, query, now)
		if score > 0 {
			suggestions = append(suggestions, Suggestion{
				Command:     entry.Command,
				Description: "Used " + formatCount(entry.UsageCount),
				Score:       score,
				Source:      "📜 History",
				UsageCount:  entry.UsageCount,
				LastUsed:    entry.LastUsed,
			})
		}
	}

	return suggestions
}

// calculateHistoryScore calculates score based on history
func (e *Engine) calculateHistoryScore(entry db.HistoryEntry, query string, now time.Time) float64 {
	score := 0.0

	// Check match quality
	if query != "" {
		result := e.matcher.Match(query, entry.Command)
		if !result.Matched {
			return 0
		}
		score = result.Score * e.weights.FuzzyMatch
	}

	// Frequency boost
	freqScore := float64(entry.UsageCount) / 100.0
	if freqScore > 1.0 {
		freqScore = 1.0
	}
	score += freqScore * e.weights.HistoryFreq

	// Recency boost
	daysSince := now.Sub(entry.LastUsed).Hours() / 24
	if daysSince < 1 {
		score += e.weights.Recency
	} else if daysSince < 7 {
		score += e.weights.Recency * 0.7
	} else if daysSince < 30 {
		score += e.weights.Recency * 0.4
	} else if daysSince < 90 {
		score += e.weights.Recency * 0.2
	}

	return score
}

// getContextSuggestions gets context-specific suggestions
func (e *Engine) getContextSuggestions(ctx *appctx.Context, query string) []Suggestion {
	var suggestions []Suggestion

	// Define project-type specific commands
	projectCommands := map[string][]Suggestion{
		"go": {
			{Command: "go mod tidy", Description: "Clean up module dependencies", Source: "🎯 Context", Icon: "📦"},
			{Command: "go test ./...", Description: "Run all tests", Source: "🎯 Context", Icon: "🧪"},
			{Command: "go build ./...", Description: "Build all packages", Source: "🎯 Context", Icon: "🔨"},
			{Command: "go run .", Description: "Run current package", Source: "🎯 Context", Icon: "▶️"},
			{Command: "go fmt ./...", Description: "Format all Go files", Source: "🎯 Context", Icon: "✨"},
			{Command: "go vet ./...", Description: "Run static analysis", Source: "🎯 Context", Icon: "🔍"},
			{Command: "go mod download", Description: "Download dependencies", Source: "🎯 Context", Icon: "⬇️"},
			{Command: "go generate ./...", Description: "Run code generation", Source: "🎯 Context", Icon: "⚙️"},
		},
		"nodejs": {
			{Command: "npm install", Description: "Install dependencies", Source: "🎯 Context", Icon: "📦"},
			{Command: "npm run dev", Description: "Start dev server", Source: "🎯 Context", Icon: "🚀"},
			{Command: "npm run build", Description: "Build for production", Source: "🎯 Context", Icon: "🔨"},
			{Command: "npm test", Description: "Run test suite", Source: "🎯 Context", Icon: "🧪"},
			{Command: "npm run lint", Description: "Run linter", Source: "🎯 Context", Icon: "✨"},
			{Command: "npm run start", Description: "Start production server", Source: "🎯 Context", Icon: "▶️"},
			{Command: "npm outdated", Description: "Check outdated packages", Source: "🎯 Context", Icon: "📋"},
			{Command: "npm audit fix", Description: "Fix security issues", Source: "🎯 Context", Icon: "🔒"},
		},
		"python": {
			{Command: "pip install -r requirements.txt", Description: "Install dependencies", Source: "🎯 Context", Icon: "📦"},
			{Command: "python -m pytest", Description: "Run tests", Source: "🎯 Context", Icon: "🧪"},
			{Command: "python -m venv venv", Description: "Create virtual environment", Source: "🎯 Context", Icon: "🐍"},
			{Command: "source venv/bin/activate", Description: "Activate virtual environment", Source: "🎯 Context", Icon: "⚡"},
			{Command: "pip freeze > requirements.txt", Description: "Save dependencies", Source: "🎯 Context", Icon: "💾"},
			{Command: "black .", Description: "Format Python code", Source: "🎯 Context", Icon: "✨"},
			{Command: "flake8 .", Description: "Lint Python code", Source: "🎯 Context", Icon: "🔍"},
		},
		"docker": {
			{Command: "docker-compose up -d", Description: "Start services", Source: "🎯 Context", Icon: "🐳"},
			{Command: "docker-compose down", Description: "Stop services", Source: "🎯 Context", Icon: "🛑"},
			{Command: "docker-compose logs -f", Description: "Follow logs", Source: "🎯 Context", Icon: "📋"},
			{Command: "docker build -t myapp .", Description: "Build image", Source: "🎯 Context", Icon: "🔨"},
			{Command: "docker ps", Description: "List running containers", Source: "🎯 Context", Icon: "📊"},
			{Command: "docker images", Description: "List images", Source: "🎯 Context", Icon: "🖼️"},
			{Command: "docker system prune", Description: "Clean up resources", Source: "🎯 Context", Icon: "🧹"},
		},
		"git": {
			{Command: "git status", Description: "Check repository status", Source: "🎯 Context", Icon: "📊"},
			{Command: "git add .", Description: "Stage all changes", Source: "🎯 Context", Icon: "➕"},
			{Command: "git commit -m \"message\"", Description: "Commit changes", Source: "🎯 Context", Icon: "💾"},
			{Command: "git push", Description: "Push to remote", Source: "🎯 Context", Icon: "🚀"},
			{Command: "git pull", Description: "Pull from remote", Source: "🎯 Context", Icon: "⬇️"},
			{Command: "git log --oneline -10", Description: "View recent commits", Source: "🎯 Context", Icon: "📜"},
			{Command: "git branch", Description: "List branches", Source: "🎯 Context", Icon: "🌿"},
			{Command: "git diff", Description: "Show changes", Source: "🎯 Context", Icon: "📝"},
		},
		"rust": {
			{Command: "cargo build", Description: "Build project", Source: "🎯 Context", Icon: "🔨"},
			{Command: "cargo test", Description: "Run tests", Source: "🎯 Context", Icon: "🧪"},
			{Command: "cargo run", Description: "Run project", Source: "🎯 Context", Icon: "▶️"},
			{Command: "cargo check", Description: "Check code", Source: "🎯 Context", Icon: "✅"},
			{Command: "cargo clippy", Description: "Run linter", Source: "🎯 Context", Icon: "🔍"},
			{Command: "cargo fmt", Description: "Format code", Source: "🎯 Context", Icon: "✨"},
			{Command: "cargo update", Description: "Update dependencies", Source: "🎯 Context", Icon: "🔄"},
		},
	}

	// Get commands for current project type
	if cmds, ok := projectCommands[ctx.ProjectType]; ok {
		suggestions = append(suggestions, cmds...)
	}

	// Git commands for git repos
	if ctx.IsGitRepo {
		if cmds, ok := projectCommands["git"]; ok {
			suggestions = append(suggestions, cmds...)
		}
	}

	// Filter by query
	if query == "" {
		return suggestions
	}

	return e.filterSuggestions(suggestions, query)
}

// getWorkflowSuggestions gets common workflow suggestions
func (e *Engine) getWorkflowSuggestions(ctx *appctx.Context, query string) []Suggestion {
	var suggestions []Suggestion

	// Quick actions based on context
	if ctx.IsGitRepo {
		if len(ctx.GitStatus.ModifiedFiles) > 0 || len(ctx.GitStatus.StagedFiles) > 0 {
			suggestions = append(suggestions, Suggestion{
				Command:     "git add . && git commit -m \"update\"",
				Description: "Quick commit all changes",
				Source:      "⚡ Quick",
				Icon:        "⚡",
			})
		}
		if ctx.GitStatus.Ahead > 0 {
			suggestions = append(suggestions, Suggestion{
				Command:     "git push",
				Description: "Push commits to remote",
				Source:      "⚡ Quick",
				Icon:        "🚀",
			})
		}
	}

	// Filter by query
	if query == "" {
		return suggestions
	}

	return e.filterSuggestions(suggestions, query)
}

// getFuzzySuggestions gets fuzzy-matched suggestions from common commands
func (e *Engine) getFuzzySuggestions(query string, limit int) []Suggestion {
	if query == "" {
		return nil
	}

	commonCommands := []string{
		"git", "docker", "kubectl", "npm", "yarn", "cargo", "go",
		"ls", "cd", "pwd", "cat", "grep", "find", "awk", "sed",
		"ssh", "scp", "curl", "wget", "ping", "netstat",
		"tar", "zip", "unzip", "gzip",
		"chmod", "chown", "mkdir", "rm", "cp", "mv",
		"ps", "top", "htop", "kill",
		"vim", "nvim", "code", "nano",
		"make", "cmake", "gcc", "g++",
	}

	results := e.matcher.MatchMultiple(query, commonCommands)

	suggestions := make([]Suggestion, 0, len(results))
	for _, r := range results {
		suggestions = append(suggestions, Suggestion{
			Command: r.Target,
			Score:   r.Score * e.weights.FuzzyMatch,
			Source:  "🔍 Fuzzy",
			Icon:    "🔍",
		})
	}

	return suggestions
}

// filterSuggestions filters suggestions by query
func (e *Engine) filterSuggestions(suggestions []Suggestion, query string) []Suggestion {
	if query == "" {
		return suggestions
	}

	queryLower := strings.ToLower(query)
	var filtered []Suggestion

	for _, s := range suggestions {
		cmdLower := strings.ToLower(s.Command)
		descLower := strings.ToLower(s.Description)

		if strings.Contains(cmdLower, queryLower) || strings.Contains(descLower, queryLower) {
			// Boost score for exact matches
			if strings.HasPrefix(cmdLower, queryLower) {
				s.Score += e.weights.PrefixMatch
			}
			filtered = append(filtered, s)
		}
	}

	return filtered
}

// scoreAndSort scores and sorts suggestions
func (e *Engine) scoreAndSort(suggestions []Suggestion, query string, ctx *appctx.Context) []Suggestion {
	// Score each suggestion
	for i := range suggestions {
		suggestions[i].Score = e.calculateFinalScore(suggestions[i], query, ctx)
	}

	// Sort by score (descending)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	return suggestions
}

// calculateFinalScore calculates the final score for a suggestion
func (e *Engine) calculateFinalScore(s Suggestion, query string, ctx *appctx.Context) float64 {
	score := s.Score

	// Boost perfect matches
	if query != "" && strings.EqualFold(s.Command, query) {
		score += e.weights.ExactMatch
		s.IsPerfectMatch = true
	}

	// Context relevance boost
	score += s.ContextMatch * e.weights.ContextRelevance

	return score
}

// limitSuggestions limits the number of suggestions
func (e *Engine) limitSuggestions(suggestions []Suggestion, limit int) []Suggestion {
	if len(suggestions) <= limit {
		return suggestions
	}
	return suggestions[:limit]
}

// formatCount formats a count for display
func formatCount(n int) string {
	if n == 1 {
		return "1 time"
	}
	return fmt.Sprintf("%d times", n)
}

// GetFallbackSuggestions returns fallback suggestions when normal flow fails
func (e *Engine) GetFallbackSuggestions(ctx *appctx.Context, limit int) []Suggestion {
	if limit <= 0 {
		limit = 10
	}

	// Always provide context-based suggestions as fallback
	suggestions := e.getContextSuggestions(ctx, "")

	// If still empty, provide generic suggestions
	if len(suggestions) == 0 {
		suggestions = []Suggestion{
			{Command: "ls", Description: "List directory contents", Source: "📌 Common", Icon: "📄", Score: 1.0},
			{Command: "pwd", Description: "Print working directory", Source: "📌 Common", Icon: "📁", Score: 1.0},
			{Command: "cd ..", Description: "Go to parent directory", Source: "📌 Common", Icon: "🔙", Score: 1.0},
			{Command: "clear", Description: "Clear the screen", Source: "📌 Common", Icon: "🧹", Score: 0.9},
		}
	}

	// Add git commands if in git repo
	if ctx.IsGitRepo {
		suggestions = append([]Suggestion{
			{Command: "git status", Description: "Check repository status", Source: "🎯 Context", Icon: "📊", Score: 1.5},
			{Command: "git add .", Description: "Stage all changes", Source: "🎯 Context", Icon: "➕", Score: 1.4},
			{Command: "git commit -m \"message\"", Description: "Commit changes", Source: "🎯 Context", Icon: "💾", Score: 1.3},
		}, suggestions...)
	}

	return e.limitSuggestions(suggestions, limit)
}

// Preload preloads suggestions into cache
func (e *Engine) Preload(ctx context.Context, ctxData *appctx.Context) {
	// Preload empty query suggestions
	go func() {
		_, _ = e.Suggest(ctx, "", ctxData, 20)
	}()
}

// ClearCache clears the suggestion cache
func (e *Engine) ClearCache() {
	e.cache.Clear()
	e.ctxCache.Clear()
}

// GetAutocomplete returns autocomplete suggestions
func (e *Engine) GetAutocomplete(prefix string) []string {
	return e.autocomplete.Suggest(prefix)
}

// AddToAutocomplete adds a command to autocomplete
func (e *Engine) AddToAutocomplete(command string) {
	e.autocomplete.Add(command)
}
