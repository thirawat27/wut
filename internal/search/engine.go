// Package search provides comprehensive search functionality for WUT
package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"wut/internal/ai"
	"wut/internal/config"
	"wut/internal/db"
	"wut/pkg/fuzzy"
)

// Source represents a search source type
type Source string

const (
	SourceHistory   Source = "history"
	SourceAI        Source = "ai"
	SourceBuiltin   Source = "builtin"
	SourceContext   Source = "context"
	SourceAlias     Source = "alias"
	SourcePath      Source = "path"
)

// Result represents a search result
type Result struct {
	Command     string
	Description string
	Score       float64
	Source      Source
	Relevance   float64
	
	// Additional metadata
	UsageCount    int
	LastUsed      time.Time
	IsDangerous   bool
	Category      string
	Tags          []string
	
	// Match information
	Match         *fuzzy.Match
	MatchedTokens []string
}

// Query represents a search query
type Query struct {
	Text          string
	Context       map[string]interface{}
	Limit         int
	Sources       []Source
	MinScore      float64
	SortBy        SortCriteria
	Filters       map[string]interface{}
}

// SortCriteria represents how to sort results
type SortCriteria int

const (
	SortByRelevance SortCriteria = iota
	SortByScore
	SortByUsage
	SortByRecent
	SortByName
)

// Engine provides comprehensive search functionality
type Engine struct {
	config      *config.Config
	storage     *db.Storage
	aiModel     *ai.Model
	matcher     *fuzzy.Matcher
	
	// Built-in command database
	builtinCmds map[string]BuiltinCommand
	
	// Cache for recent searches
	cache       map[string]*cachedResult
	cacheMu     sync.RWMutex
	cacheTTL    time.Duration
}

// BuiltinCommand represents a built-in shell command
type BuiltinCommand struct {
	Name        string
	Description string
	Category    string
	Examples    []string
	IsDangerous bool
	Tags        []string
}

// cachedResult represents a cached search result
type cachedResult struct {
	results   []Result
	timestamp time.Time
	query     string
}

// NewEngine creates a new search engine
func NewEngine(cfg *config.Config, storage *db.Storage) (*Engine, error) {
	matcher := fuzzy.NewMatcher(
		cfg.Fuzzy.CaseSensitive,
		cfg.Fuzzy.MaxDistance,
		cfg.Fuzzy.Threshold,
	)
	matcher.SetAlgorithm(fuzzy.AlgorithmHybrid)
	
	engine := &Engine{
		config:      cfg,
		storage:     storage,
		matcher:     matcher,
		builtinCmds: initializeBuiltinCommands(),
		cache:       make(map[string]*cachedResult),
		cacheTTL:    5 * time.Minute,
	}
	
	return engine, nil
}

// SetAIModel sets the AI model for AI-powered search
func (e *Engine) SetAIModel(model *ai.Model) {
	e.aiModel = model
}

// Search performs a comprehensive search across all sources
func (e *Engine) Search(ctx context.Context, query Query) ([]Result, error) {
	if query.Text == "" {
		return e.getDefaultSuggestions(query)
	}
	
	// Check cache first
	if cached := e.getCached(query.Text); cached != nil {
		return e.filterAndSort(cached, query), nil
	}
	
	// Determine which sources to search
	sources := query.Sources
	if len(sources) == 0 {
		sources = []Source{SourceHistory, SourceBuiltin, SourceAI}
	}
	
	// Search all sources concurrently
	var wg sync.WaitGroup
	resultChan := make(chan []Result, len(sources))
	errChan := make(chan error, len(sources))
	
	for _, source := range sources {
		wg.Add(1)
		go func(src Source) {
			defer wg.Done()
			results, err := e.searchSource(ctx, src, query)
			if err != nil {
				errChan <- err
				return
			}
			resultChan <- results
		}(source)
	}
	
	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errChan)
	}()
	
	// Collect results
	var allResults []Result
	done := make(chan struct{})
	
	go func() {
		for results := range resultChan {
			allResults = append(allResults, results...)
		}
		close(done)
	}()
	
	// Check for errors with timeout
	select {
	case <-done:
		// Results collected
	case err := <-errChan:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	
	// Deduplicate and merge results
	allResults = e.deduplicate(allResults)
	
	// Cache results
	e.cacheResults(query.Text, allResults)
	
	return e.filterAndSort(allResults, query), nil
}

// SearchInteractive performs search optimized for interactive mode
func (e *Engine) SearchInteractive(ctx context.Context, queryText string, sources []Source) ([]Result, error) {
	query := Query{
		Text:     queryText,
		Limit:    e.config.AI.Inference.MaxSuggestions,
		Sources:  sources,
		MinScore: e.config.Fuzzy.Threshold,
		SortBy:   SortByRelevance,
	}
	
	results, err := e.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// If no results and query is not empty, try AI
	if len(results) == 0 && queryText != "" && e.aiModel != nil {
		aiResults, err := e.searchAI(ctx, query)
		if err == nil && len(aiResults) > 0 {
			results = append(results, aiResults...)
		}
	}
	
	return results, nil
}

// searchSource searches a specific source
func (e *Engine) searchSource(ctx context.Context, source Source, query Query) ([]Result, error) {
	switch source {
	case SourceHistory:
		return e.searchHistory(ctx, query)
	case SourceBuiltin:
		return e.searchBuiltin(query)
	case SourceAI:
		if e.aiModel != nil {
			return e.searchAI(ctx, query)
		}
		return nil, nil
	case SourceAlias:
		return e.searchAliases(query)
	case SourcePath:
		return e.searchPath(query)
	default:
		return nil, fmt.Errorf("unknown search source: %s", source)
	}
}

// searchHistory searches command history
func (e *Engine) searchHistory(ctx context.Context, query Query) ([]Result, error) {
	history, err := e.storage.GetHistory(ctx, 1000)
	if err != nil {
		return nil, err
	}
	
	var results []Result
	seen := make(map[string]bool)
	
	for _, cmd := range history {
		// Skip duplicates
		if seen[cmd.Command] {
			continue
		}
		seen[cmd.Command] = true
		
		match := e.matcher.Match(query.Text, cmd.Command)
		if match.Confidence >= e.config.Fuzzy.Threshold {
			results = append(results, Result{
				Command:     cmd.Command,
				Description: cmd.Description,
				Score:       match.Confidence,
				Source:      SourceHistory,
				Relevance:   fuzzy.CalculateRelevance(query.Text, cmd.Command, match),
				UsageCount:  cmd.UsageCount,
				LastUsed:    cmd.LastUsed,
				Match:       match,
			})
		}
	}
	
	return results, nil
}

// searchBuiltin searches built-in commands
func (e *Engine) searchBuiltin(query Query) ([]Result, error) {
	var results []Result
	
	for name, cmd := range e.builtinCmds {
		// Search in name
		nameMatch := e.matcher.Match(query.Text, name)
		
		// Search in description
		descMatch := e.matcher.Match(query.Text, cmd.Description)
		
		// Search in tags
		var tagScore float64
		for _, tag := range cmd.Tags {
			if m := e.matcher.Match(query.Text, tag); m.Confidence > tagScore {
				tagScore = m.Confidence
			}
		}
		
		// Use best match
		bestScore := nameMatch.Confidence
		bestMatch := nameMatch
		
		if descMatch.Confidence > bestScore {
			bestScore = descMatch.Confidence
			bestMatch = descMatch
		}
		
		if tagScore > bestScore {
			bestScore = tagScore
		}
		
		if bestScore >= e.config.Fuzzy.Threshold {
			results = append(results, Result{
				Command:     name,
				Description: cmd.Description,
				Score:       bestScore,
				Source:      SourceBuiltin,
				Relevance:   fuzzy.CalculateRelevance(query.Text, name, bestMatch),
				Category:    cmd.Category,
				IsDangerous: cmd.IsDangerous,
				Tags:        cmd.Tags,
				Match:       bestMatch,
			})
		}
	}
	
	return results, nil
}

// searchAI performs AI-powered search
func (e *Engine) searchAI(ctx context.Context, query Query) ([]Result, error) {
	if e.aiModel == nil {
		return nil, nil
	}
	
	aiReq := ai.SuggestRequest{
		Query:      query.Text,
		MaxResults: query.Limit,
		Confidence: e.config.AI.Inference.ConfidenceThreshold,
	}
	
	aiResp, err := e.aiModel.Suggest(ctx, aiReq)
	if err != nil {
		return nil, err
	}
	
	var results []Result
	for _, s := range aiResp.Suggestions {
		results = append(results, Result{
			Command:     s.Command,
			Description: s.Description,
			Score:       s.Confidence,
			Source:      SourceAI,
			Relevance:   s.Confidence,
		})
	}
	
	return results, nil
}

// searchAliases searches shell aliases
func (e *Engine) searchAliases(query Query) ([]Result, error) {
	// TODO: Implement alias search from shell configuration
	return nil, nil
}

// searchPath searches executables in PATH
func (e *Engine) searchPath(query Query) ([]Result, error) {
	// TODO: Implement PATH search
	return nil, nil
}

// getDefaultSuggestions returns default suggestions when query is empty
func (e *Engine) getDefaultSuggestions(query Query) ([]Result, error) {
	ctx := context.Background()
	
	// Get recent and frequent commands from history
	history, err := e.storage.GetHistory(ctx, 100)
	if err != nil {
		return nil, err
	}
	
	var results []Result
	seen := make(map[string]bool)
	
	// Add frequent commands
	for _, cmd := range history {
		if seen[cmd.Command] {
			continue
		}
		seen[cmd.Command] = true
		
		// Calculate score based on frequency and recency
		score := 0.5
		if cmd.UsageCount > 0 {
			score += 0.3 * minFloat(float64(cmd.UsageCount)/10.0, 1.0)
		}
		if time.Since(cmd.LastUsed) < 24*time.Hour {
			score += 0.2
		}
		
		results = append(results, Result{
			Command:     cmd.Command,
			Description: cmd.Description,
			Score:       score,
			Source:      SourceHistory,
			Relevance:   score,
			UsageCount:  cmd.UsageCount,
			LastUsed:    cmd.LastUsed,
		})
		
		if len(results) >= query.Limit && query.Limit > 0 {
			break
		}
	}
	
	return results, nil
}

// deduplicate removes duplicate results, keeping the one with highest score
func (e *Engine) deduplicate(results []Result) []Result {
	seen := make(map[string]*Result)
	
	for i := range results {
		cmd := results[i].Command
		if existing, ok := seen[cmd]; ok {
			// Merge scores and keep highest
			if results[i].Score > existing.Score {
				results[i].Score = existing.Score
				seen[cmd] = &results[i]
			} else {
				// Boost existing score slightly for multiple sources
				existing.Score = minFloat(existing.Score+0.05, 1.0)
				existing.Relevance = minFloat(existing.Relevance+0.05, 1.0)
			}
		} else {
			seen[cmd] = &results[i]
		}
	}
	
	// Convert back to slice
	deduplicated := make([]Result, 0, len(seen))
	for _, r := range seen {
		deduplicated = append(deduplicated, *r)
	}
	
	return deduplicated
}

// filterAndSort filters and sorts results according to query criteria
func (e *Engine) filterAndSort(results []Result, query Query) []Result {
	// Filter by minimum score
	filtered := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Score >= query.MinScore {
			filtered = append(filtered, r)
		}
	}
	
	// Sort results
	switch query.SortBy {
	case SortByScore:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Score > filtered[j].Score
		})
	case SortByUsage:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].UsageCount > filtered[j].UsageCount
		})
	case SortByRecent:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].LastUsed.After(filtered[j].LastUsed)
		})
	case SortByName:
		sort.Slice(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Command) < strings.ToLower(filtered[j].Command)
		})
	default: // SortByRelevance
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Relevance > filtered[j].Relevance
		})
	}
	
	// Apply limit
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}
	
	return filtered
}

// cache methods
func (e *Engine) getCached(query string) []Result {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()
	
	if cached, ok := e.cache[query]; ok {
		if time.Since(cached.timestamp) < e.cacheTTL {
			return cached.results
		}
	}
	return nil
}

func (e *Engine) cacheResults(query string, results []Result) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	
	e.cache[query] = &cachedResult{
		results:   results,
		timestamp: time.Now(),
		query:     query,
	}
}

// ClearCache clears the search cache
func (e *Engine) ClearCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	
	e.cache = make(map[string]*cachedResult)
}

// GetBuiltinCommands returns all built-in commands
func (e *Engine) GetBuiltinCommands() map[string]BuiltinCommand {
	return e.builtinCmds
}

// initializeBuiltinCommands initializes the built-in command database
func initializeBuiltinCommands() map[string]BuiltinCommand {
	return map[string]BuiltinCommand{
		"git": {
			Name:        "git",
			Description: "Distributed version control system",
			Category:    "vcs",
			Tags:        []string{"version control", "scm", "repository"},
			Examples:    []string{"git status", "git commit -m 'message'", "git push"},
		},
		"docker": {
			Name:        "docker",
			Description: "Container platform",
			Category:    "container",
			Tags:        []string{"container", "virtualization", "deployment"},
			Examples:    []string{"docker ps", "docker build -t myapp .", "docker run myapp"},
		},
		"kubectl": {
			Name:        "kubectl",
			Description: "Kubernetes command-line tool",
			Category:    "container",
			Tags:        []string{"kubernetes", "k8s", "orchestration"},
			Examples:    []string{"kubectl get pods", "kubectl apply -f deployment.yaml"},
		},
		"npm": {
			Name:        "npm",
			Description: "Node package manager",
			Category:    "package",
			Tags:        []string{"node", "javascript", "package manager"},
			Examples:    []string{"npm install", "npm run build", "npm test"},
		},
		"cargo": {
			Name:        "cargo",
			Description: "Rust package manager",
			Category:    "package",
			Tags:        []string{"rust", "build", "package manager"},
			Examples:    []string{"cargo build", "cargo run", "cargo test"},
		},
		"make": {
			Name:        "make",
			Description: "Build automation tool",
			Category:    "build",
			Tags:        []string{"build", "automation", "compile"},
			Examples:    []string{"make", "make build", "make clean"},
		},
		"ssh": {
			Name:        "ssh",
			Description: "Secure shell client",
			Category:    "network",
			Tags:        []string{"remote", "secure", "shell"},
			Examples:    []string{"ssh user@host", "ssh -i key.pem user@host"},
		},
		"curl": {
			Name:        "curl",
			Description: "Transfer data from/to server",
			Category:    "network",
			Tags:        []string{"http", "download", "api"},
			Examples:    []string{"curl https://api.example.com", "curl -O file.zip"},
		},
		"grep": {
			Name:        "grep",
			Description: "Search text using patterns",
			Category:    "text",
			Tags:        []string{"search", "filter", "pattern"},
			Examples:    []string{"grep 'pattern' file.txt", "grep -r 'pattern' dir/"},
		},
		"find": {
			Name:        "find",
			Description: "Search for files in directory hierarchy",
			Category:    "filesystem",
			Tags:        []string{"search", "files", "directory"},
			Examples:    []string{"find . -name '*.go'", "find / -type f -size +100M"},
		},
		"tar": {
			Name:        "tar",
			Description: "Archive files",
			Category:    "filesystem",
			Tags:        []string{"archive", "compress", "backup"},
			Examples:    []string{"tar -czvf archive.tar.gz dir/", "tar -xzvf archive.tar.gz"},
		},
		"chmod": {
			Name:        "chmod",
			Description: "Change file permissions",
			Category:    "filesystem",
			Tags:        []string{"permissions", "security", "files"},
			Examples:    []string{"chmod +x script.sh", "chmod 755 file"},
		},
		"chown": {
			Name:        "chown",
			Description: "Change file owner",
			Category:    "filesystem",
			Tags:        []string{"ownership", "permissions", "files"},
			Examples:    []string{"chown user:group file", "chown -R user: dir/"},
		},
		"ps": {
			Name:        "ps",
			Description: "Report process status",
			Category:    "process",
			Tags:        []string{"processes", "monitoring", "system"},
			Examples:    []string{"ps aux", "ps -ef | grep nginx"},
		},
		"top": {
			Name:        "top",
			Description: "Display processes",
			Category:    "process",
			Tags:        []string{"processes", "monitoring", "system"},
			Examples:    []string{"top", "top -u username"},
		},
		"kill": {
			Name:        "kill",
			Description: "Send signal to process",
			Category:    "process",
			Tags:        []string{"processes", "signal", "terminate"},
			Examples:    []string{"kill -9 PID", "killall processname"},
		},
		"rm": {
			Name:        "rm",
			Description: "Remove files or directories",
			Category:    "filesystem",
			IsDangerous: true,
			Tags:        []string{"delete", "remove", "files"},
			Examples:    []string{"rm file.txt", "rm -rf dir/"},
		},
		"cp": {
			Name:        "cp",
			Description: "Copy files and directories",
			Category:    "filesystem",
			Tags:        []string{"copy", "backup", "files"},
			Examples:    []string{"cp file.txt backup/", "cp -r dir/ backup/"},
		},
		"mv": {
			Name:        "mv",
			Description: "Move/rename files",
			Category:    "filesystem",
			Tags:        []string{"move", "rename", "files"},
			Examples:    []string{"mv old.txt new.txt", "mv file.txt dir/"},
		},
		"ls": {
			Name:        "ls",
			Description: "List directory contents",
			Category:    "filesystem",
			Tags:        []string{"list", "directory", "files"},
			Examples:    []string{"ls -la", "ls -ltr"},
		},
		"cd": {
			Name:        "cd",
			Description: "Change directory",
			Category:    "filesystem",
			Tags:        []string{"change", "directory", "navigate"},
			Examples:    []string{"cd /path/to/dir", "cd ~", "cd -"},
		},
		"pwd": {
			Name:        "pwd",
			Description: "Print working directory",
			Category:    "filesystem",
			Tags:        []string{"print", "directory", "path"},
		},
		"cat": {
			Name:        "cat",
			Description: "Concatenate and display files",
			Category:    "text",
			Tags:        []string{"display", "view", "files"},
			Examples:    []string{"cat file.txt", "cat file1 file2 > combined"},
		},
		"less": {
			Name:        "less",
			Description: "View file contents interactively",
			Category:    "text",
			Tags:        []string{"pager", "view", "scroll"},
			Examples:    []string{"less file.txt", "command | less"},
		},
		"vim": {
			Name:        "vim",
			Description: "Vi IMproved text editor",
			Category:    "editor",
			Tags:        []string{"editor", "text", "vi"},
			Examples:    []string{"vim file.txt", "vim +10 file.txt"},
		},
		"code": {
			Name:        "code",
			Description: "Visual Studio Code",
			Category:    "editor",
			Tags:        []string{"editor", "ide", "vscode"},
			Examples:    []string{"code .", "code file.txt"},
		},
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
