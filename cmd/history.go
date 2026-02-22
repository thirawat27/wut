// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/metrics"
	"wut/internal/performance"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View command history",
	Long:  `View, search, and analyze your command history.`,
	Example: `  wut history
  wut history --limit 50
  wut history --search "docker"
  wut history --stats
  wut history --import-shell`,
	RunE: runHistory,
}

var (
	historyLimit       int
	historySearch      string
	historyStats       bool
	historyClear       bool
	historyExport      string
	historyImport      string
	historyImportShell bool
	historyWorkers     int
)

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.Flags().IntVarP(&historyLimit, "limit", "l", 20, "number of entries to show")
	historyCmd.Flags().StringVarP(&historySearch, "search", "s", "", "search term")
	historyCmd.Flags().BoolVar(&historyStats, "stats", false, "show statistics")
	historyCmd.Flags().BoolVar(&historyClear, "clear", false, "clear history")
	historyCmd.Flags().StringVarP(&historyExport, "export", "e", "", "export history to file")
	historyCmd.Flags().StringVarP(&historyImport, "import", "i", "", "import history from file")
	historyCmd.Flags().BoolVar(&historyImportShell, "import-shell", false, "import from shell history (bash, zsh, fish, powershell)")
	historyCmd.Flags().IntVar(&historyWorkers, "workers", 0, "number of concurrent workers (0 = auto)")
}

func runHistory(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	log := logger.With("history")

	cfg := config.Get()

	// Initialize storage with optimized settings
	storage, err := db.NewStorage(cfg.Database.Path)
	if err != nil {
		log.Error("failed to initialize storage", "error", err)
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	// Handle clear
	if historyClear {
		if err := storage.ClearHistory(ctx); err != nil {
			log.Error("failed to clear history", "error", err)
			return fmt.Errorf("failed to clear history: %w", err)
		}
		fmt.Println("✅ History cleared successfully")
		return nil
	}

	// Handle export
	if historyExport != "" {
		if err := storage.ExportHistory(ctx, historyExport); err != nil {
			log.Error("failed to export history", "error", err, "file", historyExport)
			return fmt.Errorf("failed to export history: %w", err)
		}
		fmt.Printf("✅ History exported to %s\n", historyExport)
		return nil
	}

	// Handle import
	if historyImport != "" {
		if err := storage.ImportHistory(ctx, historyImport); err != nil {
			log.Error("failed to import history", "error", err, "file", historyImport)
			return fmt.Errorf("failed to import history: %w", err)
		}
		fmt.Printf("✅ History imported from %s\n", historyImport)
		return nil
	}

	// Handle import from shell
	if historyImportShell {
		return importShellHistory(ctx, storage)
	}

	// Show statistics
	if historyStats {
		return showHistoryStats(ctx, storage)
	}

	// Show history
	return showHistory(ctx, storage)
}

func showHistory(ctx context.Context, storage *db.Storage) error {
	log := logger.With("history.show")

	var entries []db.HistoryEntry
	var err error

	if historySearch != "" {
		log.Debug("searching history", "term", historySearch)
		entries, err = searchHistoryOptimized(storage, historySearch, historyLimit)
	} else {
		log.Debug("getting history", "limit", historyLimit)
		entries, err = storage.GetHistory(ctx, historyLimit)
	}

	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No history entries found.")
		fmt.Println("\nTip: Use 'wut history --import-shell' to import your shell history")
		return nil
	}

	// Print header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	fmt.Printf("\n%s\n\n", headerStyle.Render("📜 Command History"))

	// Print entries
	for i, entry := range entries {
		printHistoryEntry(i+1, entry)
	}

	// Footer
	fmt.Printf("\nShowing %d of %d commands\n", len(entries), getTotalCount(ctx, storage))
	fmt.Println("\nTip: Use 'wut history --stats' for detailed statistics")

	// Record metrics
	metrics.RecordHistoryView()

	return nil
}

func printHistoryEntry(index int, entry db.HistoryEntry) {
	// Index style
	indexStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Width(4).
		Align(lipgloss.Right)

	// Command style
	cmdStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#10B981"))

	// Meta style
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	// Print command
	fmt.Printf("%s  %s\n",
		indexStyle.Render(fmt.Sprintf("%d.", index)),
		cmdStyle.Render(entry.Command))

	// Print metadata
	meta := fmt.Sprintf("Used: %d times | Last: %s",
		entry.UsageCount,
		entry.LastUsed.Format("2006-01-02 15:04"))
	fmt.Printf("     %s\n", metaStyle.Render(meta))

	if entry.Description != "" {
		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
		fmt.Printf("     %s\n", descStyle.Render(entry.Description))
	}
	fmt.Println()
}

func searchHistoryOptimized(storage *db.Storage, query string, limit int) ([]db.HistoryEntry, error) {
	// Use fast fuzzy matching for search
	matcher := performance.NewFastMatcher(false, 0.3, 3)

	// Get all entries
	entries, err := storage.GetHistory(context.Background(), 10000)
	if err != nil {
		return nil, err
	}

	// Score and filter
	type scoredEntry struct {
		entry db.HistoryEntry
		score float64
	}

	var scored []scoredEntry
	for _, entry := range entries {
		result := matcher.Match(query, entry.Command)
		if result.Matched {
			scored = append(scored, scoredEntry{
				entry: entry,
				score: result.Score,
			})
		}
	}

	// Sort by score
	for i := range scored {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}

	// Extract entries
	results := make([]db.HistoryEntry, len(scored))
	for i, s := range scored {
		results[i] = s.entry
	}

	return results, nil
}

func getTotalCount(ctx context.Context, storage *db.Storage) int {
	entries, err := storage.GetHistory(ctx, 0)
	if err != nil {
		return 0
	}
	return len(entries)
}

func showHistoryStats(ctx context.Context, storage *db.Storage) error {
	log := logger.With("history.stats")
	log.Debug("getting history statistics")

	stats, err := storage.GetHistoryStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get history statistics: %w", err)
	}

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	fmt.Printf("\n%s\n\n", headerStyle.Render("📊 History Statistics"))

	// Main stats
	statStyle := lipgloss.NewStyle().Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	fmt.Printf("  %s %s\n", statStyle.Render("Total Commands:"), valueStyle.Render(fmt.Sprintf("%d", stats.TotalCommands)))
	fmt.Printf("  %s %s\n", statStyle.Render("Unique Commands:"), valueStyle.Render(fmt.Sprintf("%d", stats.UniqueCommands)))
	fmt.Printf("  %s %s\n", statStyle.Render("Most Used Command:"), valueStyle.Render(stats.MostUsedCommand))
	fmt.Printf("  %s %s\n", statStyle.Render("Most Used Count:"), valueStyle.Render(fmt.Sprintf("%d", stats.MostUsedCount)))
	fmt.Printf("  %s %.2f\n", statStyle.Render("Average Usage:"), stats.AverageUsage)
	fmt.Println()

	// Top commands
	if len(stats.TopCommands) > 0 {
		topStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
		fmt.Printf("%s\n", topStyle.Render("🏆 Top Commands:"))
		for i, cmd := range stats.TopCommands {
			fmt.Printf("  %d. %s (%d times)\n", i+1, cmd.Command, cmd.Count)
		}
		fmt.Println()
	}

	// Top categories
	if len(stats.TopCategories) > 0 {
		catStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3B82F6"))
		fmt.Printf("%s\n", catStyle.Render("📁 Top Categories:"))
		for _, cat := range stats.TopCategories {
			fmt.Printf("  • %s: %d commands\n", cat.Name, cat.Count)
		}
		fmt.Println()
	}

	// Tips
	fmt.Println("💡 Tips:")
	fmt.Println("  • Use 'wut smart' for AI-powered suggestions based on your history")
	fmt.Println("  • Use 'wut history --search <query>' to find specific commands")
	fmt.Println("  • Use 'wut history --export backup.json' to backup your history")

	// Record metrics
	metrics.RecordHistoryView()

	return nil
}

// importShellHistory imports history from shell history files
func importShellHistory(ctx context.Context, storage *db.Storage) error {
	// Detect available shells and read history directly
	shellHistories := detectShellHistories()
	if len(shellHistories) == 0 {
		return fmt.Errorf("no shell history files detected. Make sure you have bash, zsh, fish, or PowerShell history")
	}

	fmt.Println("🔍 Detected shells:")
	for shellType, path := range shellHistories {
		fmt.Printf("  • %s: %s\n", shellType, path)
	}
	fmt.Println()

	// Read all histories
	fmt.Println("📖 Reading shell histories...")
	start := time.Now()

	var allCommands []string
	for shellType, path := range shellHistories {
		commands, err := readShellHistory(shellType, path)
		if err != nil {
			fmt.Printf("Warning: Failed to read %s history: %v\n", shellType, err)
			continue
		}
		allCommands = append(allCommands, commands...)
		fmt.Printf("  ✓ %s: %d commands\n", shellType, len(commands))
	}

	if len(allCommands) == 0 {
		fmt.Println("No history entries found in shell files")
		return nil
	}

	// Remove duplicates while preserving order
	uniqueCommands := removeDuplicates(allCommands)

	fmt.Printf("\n✅ Read %d entries (%d unique) in %v\n", len(allCommands), len(uniqueCommands), time.Since(start))
	fmt.Println()

	// Import to database
	fmt.Println("💾 Importing to WUT database...")
	importStart := time.Now()
	imported := 0

	for _, cmd := range uniqueCommands {
		if cmd = strings.TrimSpace(cmd); cmd != "" {
			if err := storage.AddHistory(ctx, cmd); err == nil {
				imported++
			}
		}
	}

	fmt.Printf("\n✅ Successfully imported %d commands in %v\n", imported, time.Since(importStart))
	fmt.Println()
	fmt.Println("💡 Next steps:")
	fmt.Println("  • 'wut history' - View your imported history")
	fmt.Println("  • 'wut smart' - Get AI-powered suggestions")
	fmt.Println("  • 'wut suggest' - Search command database")

	return nil
}

// detectShellHistories detects shell history files
func detectShellHistories() map[string]string {
	shells := make(map[string]string)
	home, err := os.UserHomeDir()
	if err != nil {
		return shells
	}

	// Bash
	bashHistory := filepath.Join(home, ".bash_history")
	if _, err := os.Stat(bashHistory); err == nil {
		shells["bash"] = bashHistory
	}

	// Zsh
	zshHistory := filepath.Join(home, ".zsh_history")
	if _, err := os.Stat(zshHistory); err == nil {
		shells["zsh"] = zshHistory
	}

	// Fish
	fishHistory := filepath.Join(home, ".local", "share", "fish", "fish_history")
	if runtime.GOOS == "darwin" {
		fishHistory = filepath.Join(home, ".config", "fish", "fish_history")
	}
	if _, err := os.Stat(fishHistory); err == nil {
		shells["fish"] = fishHistory
	}

	// PowerShell
	psHistory := filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
	if runtime.GOOS != "windows" {
		psHistory = filepath.Join(home, ".config", "powershell", "PSReadLine", "ConsoleHost_history.txt")
		if _, err := os.Stat(psHistory); err != nil {
			// Try alternative path
			psHistory = filepath.Join(home, ".local", "share", "powershell", "PSReadLine", "ConsoleHost_history.txt")
		}
	}
	if _, err := os.Stat(psHistory); err == nil {
		shells["powershell"] = psHistory
	}

	return shells
}

// readShellHistory reads history from a shell history file
func readShellHistory(shellType, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var commands []string
	lines := strings.Split(string(data), "\n")

	switch shellType {
	case "fish":
		// Fish history format: - cmd: <command>
		for _, line := range lines {
			if after, ok := strings.CutPrefix(line, "- cmd: "); ok {
				cmd := after
				commands = append(commands, cmd)
			}
		}
	case "zsh":
		// Zsh history may have timestamps: : timestamp:elapsed;command
		for _, line := range lines {
			if _, after, ok := strings.Cut(line, ";"); ok {
				commands = append(commands, after)
			} else if line != "" {
				commands = append(commands, line)
			}
		}
	default:
		// Bash, PowerShell - one command per line
		for _, line := range lines {
			if line = strings.TrimSpace(line); line != "" {
				commands = append(commands, line)
			}
		}
	}

	return commands, nil
}

// removeDuplicates removes duplicate commands while preserving order
func removeDuplicates(commands []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cmd := range commands {
		// Normalize command for deduplication
		normalized := strings.TrimSpace(cmd)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}
