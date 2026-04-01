// Package cmd provides smart command suggestions
package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	appctx "wut/internal/context"
	"wut/internal/corrector"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/smart"
	"wut/internal/ui"
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
	// Use shorter timeout to ensure responsiveness
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	log := logger.With("smart")

	// Get query from args
	query := ""
	if len(args) > 0 {
		query = strings.Join(args, " ")
	}

	// Detect context with timeout
	analyzer := appctx.NewAnalyzer()
	appCtx, err := analyzer.Analyze(ctx)
	if err != nil {
		log.Warn("failed to detect context", "error", err)
		appCtx = &appctx.Context{
			WorkingDir:  ".",
			ProjectType: "unknown",
		}
	}

	// Show context header
	printContextInfo(appCtx)

	// Initialize storage once and reuse it for correction, ranking and history.
	storage := openSmartStorage(log)
	if storage != nil {
		defer storage.Close()
	}

	// Check for typos if enabled
	if smartCorrect && query != "" {
		c := corrector.New()

		// Optional: supply history to corrector for better matching
		if storage != nil {
			if history, err := storage.GetHistory(context.Background(), 100); err == nil {
				historyCmds := make([]string, 0, len(history))
				for _, h := range history {
					historyCmds = append(historyCmds, h.Command)
				}
				c.SetHistoryCommands(historyCmds)
			}
		}

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

	// Create smart engine
	engine := smart.NewEngine(storage)

	// Get intelligent suggestions with timeout
	suggestionsCh := make(chan []smart.Suggestion, 1)
	var suggestErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic in suggest", "recover", r)
			}
		}()
		sugs, err := engine.Suggest(ctx, query, appCtx, smartLimit)
		if err != nil {
			suggestErr = err
			return
		}
		// Use select so the goroutine exits even if the caller already timed out.
		select {
		case suggestionsCh <- sugs:
		case <-ctx.Done():
		}
	}()

	var suggestions []smart.Suggestion
	select {
	case suggestions = <-suggestionsCh:
		// Got suggestions
	case <-ctx.Done():
		log.Warn("suggestion timeout, using fallback")
		suggestions = engine.GetFallbackSuggestions(appCtx, smartLimit)
	}

	if suggestErr != nil {
		log.Error("failed to get suggestions", "error", suggestErr)
		// Try fallback
		suggestions = engine.GetFallbackSuggestions(appCtx, smartLimit)
	}

	// Display suggestions
	if len(suggestions) == 0 {
		// Always show fallback suggestions instead of empty
		suggestions = engine.GetFallbackSuggestions(appCtx, smartLimit)
	}

	printSmartSuggestions(suggestions)

	// Record this query in history (async, don't block)
	if storage != nil {
		recordCmd := "wut smart"
		if query != "" {
			recordCmd += " " + query
		}
		go func() {
			recordCtx, recordCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer recordCancel()
			if err := storage.AddHistory(recordCtx, recordCmd); err != nil {
				log.Debug("failed to record history", "error", err)
			}
		}()
	}

	return nil
}

func openSmartStorage(log *logger.Logger) *db.Storage {
	storageCh := make(chan *db.Storage, 1)
	storageErrCh := make(chan error, 1)

	go func() {
		s, err := db.NewStorage(config.GetDatabasePath())
		if err != nil {
			storageErrCh <- err
			return
		}
		storageCh <- s
	}()

	select {
	case storage := <-storageCh:
		return storage
	case err := <-storageErrCh:
		log.Warn("failed to initialize storage, continuing without history", "error", err)
	case <-time.After(500 * time.Millisecond):
		log.Warn("storage initialization timeout, continuing without history")
	}

	return nil
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

	// Get folder name from working directory
	folderName := filepath.Base(ctx.WorkingDir)
	if folderName == "." || folderName == "/" || folderName == "\\" {
		folderName = ctx.WorkingDir
	}

	info := []string{
		fmt.Sprintf("Project: %s", folderName),
	}

	// Also show project type if detected
	if ctx.ProjectType != "unknown" && ctx.ProjectType != "git" {
		info = append(info, fmt.Sprintf("Type: %s", ctx.ProjectType))
	}

	if ctx.IsGitRepo {
		info = append(info, fmt.Sprintf("Branch: %s", ctx.GitBranch))

		if ctx.GitStatus.Ahead > 0 {
			info = append(info, fmt.Sprintf("↑ %d commits ahead", ctx.GitStatus.Ahead))
		}
		if ctx.GitStatus.Behind > 0 {
			info = append(info, fmt.Sprintf("↓ %d commits behind", ctx.GitStatus.Behind))
		}
		if !ctx.GitStatus.IsClean {
			info = append(info, "📝 Has uncommitted changes")
		}
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

func printSmartSuggestions(suggestions []smart.Suggestion) {
	// Group suggestions by source
	bySource := make(map[string][]smart.Suggestion)
	for _, s := range suggestions {
		bySource[s.Source] = append(bySource[s.Source], s)
	}

	// Print header
	fmt.Println(lipgloss.NewStyle().Bold(true).Render("💡 Smart Suggestions:"))
	fmt.Println()

	// Print suggestions with source grouping
	printed := 0
	for _, s := range suggestions {
		icon := s.Icon
		if icon == "" {
			icon = "•"
		}

		// Color based on score
		var cmdColor lipgloss.Style
		if s.Score >= 1.5 {
			cmdColor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")) // High score - green
		} else if s.Score >= 1.0 {
			cmdColor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3B82F6")) // Medium score - blue
		} else {
			cmdColor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6B7280")) // Low score - gray
		}

		sourceColor := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Render(s.Source)

		fmt.Printf("%s %s %s\n", icon, cmdColor.Render(s.Command), sourceColor)

		if s.Description != "" {
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280"))
			fmt.Printf("   %s\n", descStyle.Render(s.Description))
		}

		printed++
		if printed < len(suggestions) {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("Tip: Use 'wut smart <query>' to search for specific commands"))
}
