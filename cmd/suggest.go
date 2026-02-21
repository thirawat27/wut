// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/logger"
	"wut/internal/tldr"
	"wut/internal/util"
)

// suggestCmd represents the suggest command
var suggestCmd = &cobra.Command{
	Use:   "suggest [command]",
	Short: "Get TLDR command cheat sheets",
	Long: `Get command cheat sheets from TLDR Pages - a community-driven 
command line help database. Provides concise, practical examples 
for thousands of commands.

If no command is provided, enters interactive mode with live search.

Uses local database if available, otherwise fetches from online.
Auto-detects offline mode when no internet connection.`,
	Example: `  wut suggest git
  wut suggest docker
  wut suggest              # Interactive mode
  wut suggest npm --raw    # Plain text output
  wut suggest git --offline # Force offline mode
  wut suggest git --exec   # Execute selected command`,
	RunE: runSuggest,
}

var (
	suggestRaw     bool
	suggestQuiet   bool
	suggestLimit   int
	suggestOffline bool
	suggestExec    bool
)

func init() {
	rootCmd.AddCommand(suggestCmd)

	suggestCmd.Flags().BoolVarP(&suggestRaw, "raw", "r", false, "output raw text instead of TUI")
	suggestCmd.Flags().BoolVarP(&suggestQuiet, "quiet", "q", false, "output only the command examples")
	suggestCmd.Flags().IntVarP(&suggestLimit, "limit", "l", 10, "maximum number of examples to show")
	suggestCmd.Flags().BoolVarP(&suggestOffline, "offline", "o", false, "force offline mode (use local database only)")
	suggestCmd.Flags().BoolVarP(&suggestExec, "exec", "e", false, "execute the selected command after TUI closes")
}

func runSuggest(cmd *cobra.Command, args []string) error {
	log := logger.With("suggest")
	start := time.Now()

	defer func() {
		log.Debug("suggest completed", "duration", time.Since(start))
	}()

	// Load configuration
	cfg := config.Get()

	// Get query from args or enter interactive mode
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	log.Debug("processing suggest request", "query", query, "raw", suggestRaw, "offline", suggestOffline)

	// Get database path
	dbPath := getTLDRDBPathForSuggest()

	// Open storage
	var storage *tldr.Storage
	var err error
	if _, statErr := os.Stat(dbPath); statErr == nil {
		storage, err = tldr.NewStorage(dbPath)
		if err != nil {
			log.Warn("failed to open local storage", "error", err)
		}
	}
	if storage != nil {
		defer storage.Close()
	}

	// Create client with storage and options
	clientOpts := []tldr.ClientOption{
		tldr.WithAutoDetect(true), // Auto-detect online/offline
	}
	if storage != nil {
		clientOpts = append(clientOpts, tldr.WithStorage(storage))
	}
	if suggestOffline {
		clientOpts = append(clientOpts, tldr.WithOfflineMode(true))
	}

	client := tldr.NewClient(clientOpts...)

	// Interactive mode - launch TUI
	if query == "" && !suggestRaw {
		return runInteractiveMode(client, storage)
	}

	// If raw mode or quiet mode with query
	if suggestRaw || suggestQuiet {
		return runRawMode(client, query)
	}

	// Normal mode with TUI for specific command
	return runCommandMode(client, query, cfg)
}

// runInteractiveMode runs the interactive TUI mode
func runInteractiveMode(client *tldr.Client, storage *tldr.Storage) error {
	log := logger.With("suggest")
	log.Debug("entering interactive mode")

	// Check if online
	ctx := context.Background()
	online := client.IsOnline(ctx)
	if !online && !client.IsOfflineMode() {
		fmt.Println("📴 Offline mode - using local database")
		fmt.Println("   Run 'wut tldr sync' to download more commands")
		fmt.Println()
	}

	// Create and run TUI
	model := tldr.NewModel()

	// Set storage if available
	if storage != nil {
		model.SetStorage(storage)
	}

	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Get selected command or executed command
	if m, ok := finalModel.(*tldr.Model); ok {
		// Check if a command should be executed
		if cmd := m.GetExecutedCommand(); cmd != "" {
			fmt.Printf("\n⚡ Executing: %s\n\n", cmd)
			if err := tldr.ExecuteCommand(cmd); err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}
			return nil
		}

		selected := m.Selected()
		if selected != "" {
			fmt.Println(selected)
		}
	}

	return nil
}

// runRawMode outputs command in plain text format
func runRawMode(client *tldr.Client, query string) error {
	ctx := context.Background()

	page, err := client.GetPageAnyPlatform(ctx, query)
	if err != nil {
		// Try to find similar commands
		commands, _ := client.GetAvailableCommands(ctx)
		var suggestions []string
		queryLower := strings.ToLower(query)

		for _, cmd := range commands {
			if strings.Contains(strings.ToLower(cmd), queryLower) {
				suggestions = append(suggestions, cmd)
			}
		}

		if len(suggestions) > 0 {
			fmt.Printf("Command '%s' not found. Did you mean:\n", query)
			for _, s := range suggestions[:util.Min(len(suggestions), 5)] {
				fmt.Printf("  - %s\n", s)
			}
		} else {
			fmt.Printf("Command not found: %s\n", query)
			if client.IsOfflineMode() || !client.IsOnline(ctx) {
				fmt.Println("📴 Run 'wut tldr sync' to download the database")
			}
		}
		return nil
	}

	// Output raw format
	if suggestQuiet {
		// Only output commands
		for _, ex := range page.Examples {
			fmt.Println(ex.Command)
		}
	} else {
		// Full raw output
		fmt.Printf("# %s\n\n", page.Name)
		fmt.Printf("> %s\n\n", page.Description)

		limit := suggestLimit
		if limit > len(page.Examples) {
			limit = len(page.Examples)
		}

		for i, ex := range page.Examples[:limit] {
			fmt.Printf("- %s\n", ex.Description)
			fmt.Printf("  `%s`\n", ex.Command)
			if i < limit-1 {
				fmt.Println()
			}
		}
	}

	return nil
}

// runCommandMode runs with TUI for a specific command
func runCommandMode(client *tldr.Client, query string, cfg *config.Config) error {
	ctx := context.Background()

	page, err := client.GetPageAnyPlatform(ctx, query)
	if err != nil {
		fmt.Printf("Command not found: %s\n", query)
		if client.IsOfflineMode() || !client.IsOnline(ctx) {
			fmt.Println("📴 Run 'wut tldr sync' to download the database")
		}
		return nil
	}

	// Render with lipgloss
	output := tldr.FormatPage(page)
	fmt.Println(output)

	return nil
}

// formatSuggestions formats suggestions for terminal output (fallback)
func formatSuggestions(suggestions []string, query string) string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Render(fmt.Sprintf("Command '%s' not found. Did you mean:", query))

	b.WriteString(title)
	b.WriteString("\n\n")

	for _, s := range suggestions {
		item := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Render(fmt.Sprintf("  • %s", s))
		b.WriteString(item)
		b.WriteString("\n")
	}

	return b.String()
}

// getTLDRDBPathForSuggest returns the path to the TLDR database
func getTLDRDBPathForSuggest() string {
	cfg := config.Get()
	if cfg.Database.Path != "" {
		return filepath.Join(filepath.Dir(cfg.Database.Path), "tldr.db")
	}

	// Default path
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wut", "tldr.db")
}
