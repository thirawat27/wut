// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"wut/internal/ai"
	"wut/internal/config"
	appcontext "wut/internal/context"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/metrics"
	"wut/internal/ui"
	"wut/pkg/fuzzy"
)

// suggestCmd represents the suggest command
var suggestCmd = &cobra.Command{
	Use:   "suggest [query]",
	Short: "Get command suggestions",
	Long: `Get intelligent command suggestions based on your query.
If no query is provided, enters interactive mode.`,
	Example: `  wut suggest "git push"
  wut suggest "docker build"
  wut suggest`,
	RunE: runSuggest,
}

var (
	suggestLimit    int
	suggestContext  bool
	suggestQuiet    bool
	suggestNoFuzzy  bool
)

func init() {
	rootCmd.AddCommand(suggestCmd)

	suggestCmd.Flags().IntVarP(&suggestLimit, "limit", "l", 5, "maximum number of suggestions")
	suggestCmd.Flags().BoolVarP(&suggestContext, "context", "c", true, "use context-aware suggestions")
	suggestCmd.Flags().BoolVarP(&suggestQuiet, "quiet", "q", false, "only output the command")
	suggestCmd.Flags().BoolVarP(&suggestNoFuzzy, "no-fuzzy", "f", false, "disable fuzzy matching")
}

func runSuggest(cmd *cobra.Command, args []string) error {
	log := logger.With("suggest")
	start := time.Now()

	defer func() {
		metrics.RecordAIInferenceTime(time.Since(start))
	}()

	// Load configuration
	cfg := config.Get()

	// Get query from args or enter interactive mode
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	log.Debug("processing suggest request",
		"query", query,
		"limit", suggestLimit,
		"context", suggestContext,
	)

	// Initialize database
	storage, err := db.NewStorage(cfg.Database.Path)
	if err != nil {
		log.Error("failed to initialize storage", "error", err)
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	// Get context if enabled
	var ctxInfo *appcontext.Context
	if suggestContext && cfg.Context.Enabled {
		analyzer := appcontext.NewAnalyzer()
		ctxInfo, err = analyzer.Analyze()
		if err != nil {
			log.Warn("context analysis failed", "error", err)
		} else {
			log.Debug("context analyzed",
				"directory", ctxInfo.WorkingDir,
				"is_git_repo", ctxInfo.IsGitRepo,
			)
		}
	}

	// Try fuzzy matching first
	if !suggestNoFuzzy && cfg.Fuzzy.Enabled && query != "" {
		suggestions, err := getFuzzySuggestions(query, cfg, storage)
		if err != nil {
			log.Warn("fuzzy matching failed", "error", err)
		} else if len(suggestions) > 0 {
			metrics.RecordAICacheHit()
			return displaySuggestions(suggestions, cfg, query)
		}
	}

	metrics.RecordAICacheMiss()

	// Use AI for suggestions
	suggestions, err := getAISuggestions(query, ctxInfo, cfg)
	if err != nil {
		log.Error("AI inference failed", "error", err)
		return fmt.Errorf("failed to get suggestions: %w", err)
	}

	// Record metrics
	metrics.RecordCommandSuggested()

	return displaySuggestions(suggestions, cfg, query)
}

// getFuzzySuggestions gets suggestions using fuzzy matching
func getFuzzySuggestions(query string, cfg *config.Config, storage *db.Storage) ([]Suggestion, error) {
	log := logger.With("fuzzy")
	log.Debug("trying fuzzy matching", "query", query)

	// Get command history
	history, err := storage.GetCommandHistory(context.Background(), 100)
	if err != nil {
		return nil, err
	}

	// Create fuzzy matcher
	matcher := fuzzy.NewMatcher(cfg.Fuzzy.CaseSensitive, cfg.Fuzzy.MaxDistance, cfg.Fuzzy.Threshold)

	var suggestions []Suggestion
	for _, cmd := range history {
		match := matcher.Match(query, cmd)
		if match.Confidence >= cfg.Fuzzy.Threshold {
			suggestions = append(suggestions, Suggestion{
				Command:     cmd,
				Score:       match.Confidence,
				Source:      "fuzzy",
				Description: getCommandDescription(cmd),
			})
		}
	}

	// Sort by score
	sortSuggestions(suggestions)

	if len(suggestions) > suggestLimit {
		suggestions = suggestions[:suggestLimit]
	}

	return suggestions, nil
}

// getAISuggestions gets suggestions using AI
func getAISuggestions(query string, ctxInfo *appcontext.Context, cfg *config.Config) ([]Suggestion, error) {
	log := logger.With("ai")
	log.Debug("using AI for suggestions", "query", query)

	// Initialize AI model
	model, err := ai.NewModel(cfg.AI)
	if err != nil {
		return nil, fmt.Errorf("failed to load AI model: %w", err)
	}
	defer model.Close()

	// Generate suggestions
	result, err := model.Suggest(context.Background(), ai.SuggestRequest{
		Query:       query,
		Context:     ctxInfo,
		MaxResults:  suggestLimit,
		Confidence:  cfg.AI.Inference.ConfidenceThreshold,
	})
	if err != nil {
		return nil, err
	}

	var suggestions []Suggestion
	for _, s := range result.Suggestions {
		suggestions = append(suggestions, Suggestion{
			Command:     s.Command,
			Score:       s.Confidence,
			Source:      "ai",
			Description: s.Description,
		})
	}

	return suggestions, nil
}

// displaySuggestions displays suggestions to the user
func displaySuggestions(suggestions []Suggestion, cfg *config.Config, query string) error {
	if len(suggestions) == 0 {
		fmt.Println("No suggestions found")
		return nil
	}

	if suggestQuiet {
		// Output only the top command
		fmt.Println(suggestions[0].Command)
		return nil
	}

	// Use TUI for interactive display
	if query == "" {
		// Interactive mode
		var uiSuggestions []ui.Suggestion
		for _, s := range suggestions {
			uiSuggestions = append(uiSuggestions, ui.Suggestion{
				Command:     s.Command,
				Score:       s.Score,
				Description: s.Description,
			})
		}
		program := tea.NewProgram(ui.NewSuggestModel(uiSuggestions, cfg.UI))
		if _, err := program.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
	} else {
		// Simple output mode
		fmt.Printf("\n%s\n\n", "Suggestions:")
		for i, s := range suggestions {
			fmt.Printf("%d. %s\n", i+1, s.Command)
			if cfg.UI.ShowExplanations && s.Description != "" {
				fmt.Printf("   %s\n", s.Description)
			}
			if cfg.UI.ShowConfidence {
				fmt.Printf("   Confidence: %.2f%%\n", s.Score*100)
			}
			fmt.Println()
		}
	}

	return nil
}

// Suggestion represents a command suggestion
type Suggestion struct {
	Command     string
	Score       float64
	Source      string
	Description string
}

// sortSuggestions sorts suggestions by score
func sortSuggestions(suggestions []Suggestion) {
	// Simple bubble sort for now
	for i := 0; i < len(suggestions); i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Score > suggestions[i].Score {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}
}

// getCommandDescription returns a description for a command
func getCommandDescription(cmd string) string {
	// This could be enhanced with a command database
	return ""
}
