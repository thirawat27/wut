// Package cmd provides smart command suggestions
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/ui"
	appctx "wut/internal/context"
	"wut/internal/corrector"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/suggest"
	"wut/internal/workflow"
)

// smartCmd provides intelligent, context-aware command suggestions
var smartCmd = &cobra.Command{
	Use:   "smart [query]",
	Short: "Smart command suggestions based on context",
	Long: `Get intelligent command suggestions based on your project context,
command history, and current directory. WUT will detect your project type
and suggest the most relevant commands.`,
	Example: `  wut smart
  wut smart git
  wut smart "docker build"`,
	RunE: runSmart,
}

var (
	smartLimit   int
	smartExec    bool
	smartCorrect bool
)

func init() {
	rootCmd.AddCommand(smartCmd)

	smartCmd.Flags().IntVarP(&smartLimit, "limit", "l", 10, "maximum suggestions to show")
	smartCmd.Flags().BoolVarP(&smartExec, "exec", "e", false, "execute selected command")
	smartCmd.Flags().BoolVarP(&smartCorrect, "correct", "c", true, "auto-correct typos")
}

func runSmart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	log := logger.With("smart")

	// Get query from args
	query := ""
	if len(args) > 0 {
		query = strings.Join(args, " ")
	}

	// Detect context
	analyzer := appctx.NewAnalyzer()
	appCtx, err := analyzer.Analyze()
	if err != nil {
		log.Warn("failed to detect context", "error", err)
	}

	// Show context header
	printContextInfo(appCtx)

	// Check for typos if enabled
	if smartCorrect && query != "" {
		c := corrector.New()
		if correction, err := c.Correct(query); err == nil && correction != nil {
			printCorrection(correction)
			if correction.IsDangerous {
				return nil // Don't proceed with dangerous commands
			}
			if correction.Corrected != "" {
				query = correction.Corrected
			}
		}
	}

	// Get suggestions from multiple sources
	var suggestions []SmartSuggestion

	// 1. Workflow suggestions
	wfEngine := workflow.NewEngine()
	if query == "" {
		// Show quick actions and workflows
		quickActions := wfEngine.GetQuickActions(appCtx)
		for _, action := range quickActions {
			suggestions = append(suggestions, SmartSuggestion{
				Command:     action.Command,
				Description: action.Description,
				Source:      "⚡ Quick",
				Icon:        action.Icon,
			})
		}
	}

	// 2. History-based suggestions
	cfg := config.Get()
	storage, err := db.NewStorage(cfg.Database.Path)
	if err == nil {
		defer storage.Close()

		suggester := suggest.New(storage)
		historyResults, err := suggester.Suggest(ctx, query, smartLimit)
		if err == nil {
			for _, r := range historyResults {
				suggestions = append(suggestions, SmartSuggestion{
					Command:     r.Command,
					Description: fmt.Sprintf("Used %d times", int(r.Score/10)),
					Source:      "📜 History",
					Score:       r.Score,
				})
			}
		}
	}

	// 3. Context-specific suggestions
	contextSuggestions := getContextSuggestions(appCtx, query)
	suggestions = append(suggestions, contextSuggestions...)

	// Display suggestions
	if len(suggestions) == 0 {
		fmt.Println("No suggestions found. Try a different query.")
		return nil
	}

	printSuggestions(suggestions[:min(len(suggestions), smartLimit)])

	return nil
}

// SmartSuggestion represents a smart suggestion
type SmartSuggestion struct {
	Command     string
	Description string
	Source      string
	Score       float64
	Icon        string
}

func printContextInfo(ctx *appctx.Context) {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	// Print context
	fmt.Println()
	fmt.Println(headerStyle.Render("📍 Context"))

	info := []string{
		fmt.Sprintf("Project: %s", ctx.ProjectType),
	}

	if ctx.IsGitRepo {
		info = append(info, fmt.Sprintf("Branch: %s", ctx.GitBranch))
	}

	if ctx.GitStatus.Ahead > 0 {
		info = append(info, fmt.Sprintf("↑ %d commits ahead", ctx.GitStatus.Ahead))
	}
	if ctx.GitStatus.Behind > 0 {
		info = append(info, fmt.Sprintf("↓ %d commits behind", ctx.GitStatus.Behind))
	}

	for _, line := range info {
		fmt.Println(infoStyle.Render("  " + line))
	}
	fmt.Println()
}

func printCorrection(c *corrector.Correction) {
	if c.IsDangerous {
		warningStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444")).
			Background(lipgloss.Color("#FEE2E2"))
		fmt.Println(warningStyle.Render(" " + c.Explanation + " "))
		fmt.Println()
		return
	}

	if c.Corrected != "" && c.Corrected != c.Original {
		correctionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))
		fmt.Printf("%s %s → %s\n\n",
			correctionStyle.Render("🤔 Did you mean:"),
			c.Original,
			ui.Green(c.Corrected))
	}
}

func printSuggestions(suggestions []SmartSuggestion) {
	for i, s := range suggestions {
		icon := s.Icon
		if icon == "" {
			icon = "•"
		}

		sourceColor := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render(s.Source)

		cmdColor := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")).
			Render(s.Command)

		fmt.Printf("%s %s %s\n", icon, cmdColor, sourceColor)
		if s.Description != "" {
			fmt.Printf("   %s\n", s.Description)
		}

		if i < len(suggestions)-1 {
			fmt.Println()
		}
	}
}

func getContextSuggestions(ctx *appctx.Context, query string) []SmartSuggestion {
	var suggestions []SmartSuggestion

	// Add context-specific commands based on project type
	switch ctx.ProjectType {
	case "go":
		suggestions = append(suggestions, []SmartSuggestion{
			{Command: "go mod tidy", Description: "Clean up module dependencies", Source: "🎯 Go"},
			{Command: "go test ./...", Description: "Run all tests", Source: "🎯 Go"},
			{Command: "go build ./...", Description: "Build all packages", Source: "🎯 Go"},
			{Command: "go run .", Description: "Run current package", Source: "🎯 Go"},
			{Command: "go fmt ./...", Description: "Format all Go files", Source: "🎯 Go"},
			{Command: "go vet ./...", Description: "Run static analysis", Source: "🎯 Go"},
		}...)

	case "nodejs":
		suggestions = append(suggestions, []SmartSuggestion{
			{Command: "npm install", Description: "Install dependencies", Source: "🎯 Node"},
			{Command: "npm run dev", Description: "Start dev server", Source: "🎯 Node"},
			{Command: "npm run build", Description: "Build for production", Source: "🎯 Node"},
			{Command: "npm test", Description: "Run test suite", Source: "🎯 Node"},
			{Command: "npm run lint", Description: "Run linter", Source: "🎯 Node"},
		}...)

	case "docker":
		suggestions = append(suggestions, []SmartSuggestion{
			{Command: "docker-compose up -d", Description: "Start services", Source: "🎯 Docker"},
			{Command: "docker-compose down", Description: "Stop services", Source: "🎯 Docker"},
			{Command: "docker-compose logs -f", Description: "Follow logs", Source: "🎯 Docker"},
			{Command: "docker build -t myapp .", Description: "Build image", Source: "🎯 Docker"},
		}...)
	}

	// Filter by query if provided
	if query != "" {
		var filtered []SmartSuggestion
		q := strings.ToLower(query)
		for _, s := range suggestions {
			if strings.Contains(strings.ToLower(s.Command), q) ||
			   strings.Contains(strings.ToLower(s.Description), q) {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}

	return suggestions
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
