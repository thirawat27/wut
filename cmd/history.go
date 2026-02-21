// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/history"
	"wut/internal/logger"
	"wut/internal/metrics"
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

	// Initialize storage
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
		fmt.Println("History cleared successfully")
		return nil
	}

	// Handle export
	if historyExport != "" {
		if err := storage.ExportHistory(ctx, historyExport); err != nil {
			log.Error("failed to export history", "error", err, "file", historyExport)
			return fmt.Errorf("failed to export history: %w", err)
		}
		fmt.Printf("History exported to %s\n", historyExport)
		return nil
	}

	// Handle import
	if historyImport != "" {
		if err := storage.ImportHistory(ctx, historyImport); err != nil {
			log.Error("failed to import history", "error", err, "file", historyImport)
			return fmt.Errorf("failed to import history: %w", err)
		}
		fmt.Printf("History imported from %s\n", historyImport)
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
		entries, err = storage.SearchHistory(ctx, historySearch, historyLimit)
	} else {
		log.Debug("getting history", "limit", historyLimit)
		entries, err = storage.GetHistory(ctx, historyLimit)
	}

	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No history entries found")
		return nil
	}

	fmt.Printf("\n%s\n\n", "Command History:")
	for i, entry := range entries {
		fmt.Printf("%d. %s\n", i+1, entry.Command)
		if entry.Description != "" {
			fmt.Printf("   %s\n", entry.Description)
		}
		fmt.Printf("   Used: %d times | Last: %s\n", entry.UsageCount, entry.LastUsed.Format("2006-01-02 15:04"))
		fmt.Println()
	}

	// Record metrics
	metrics.RecordHistoryView()

	return nil
}

func showHistoryStats(ctx context.Context, storage *db.Storage) error {
	log := logger.With("history.stats")
	log.Debug("getting history statistics")

	stats, err := storage.GetHistoryStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get history statistics: %w", err)
	}

	fmt.Printf("\n%s\n\n", "History Statistics:")
	fmt.Printf("Total Commands: %d\n", stats.TotalCommands)
	fmt.Printf("Unique Commands: %d\n", stats.UniqueCommands)
	fmt.Printf("Most Used Command: %s (%d times)\n", stats.MostUsedCommand, stats.MostUsedCount)
	fmt.Printf("Average Usage: %.2f times per command\n", stats.AverageUsage)
	fmt.Println()

	if len(stats.TopCommands) > 0 {
		fmt.Println("Top Commands:")
		for _, cmd := range stats.TopCommands {
			fmt.Printf("  %s: %d times\n", cmd.Command, cmd.Count)
		}
		fmt.Println()
	}

	if len(stats.TopCategories) > 0 {
		fmt.Println("Top Categories:")
		for _, cat := range stats.TopCategories {
			fmt.Printf("  %s: %d commands\n", cat.Name, cat.Count)
		}
	}

	// Record metrics
	metrics.RecordHistoryView()

	return nil
}

// importShellHistory imports history from shell history files
func importShellHistory(ctx context.Context, storage *db.Storage) error {
	// Create history reader with concurrent workers
	workers := historyWorkers
	if workers <= 0 {
		workers = 0 // Auto-detect (uses runtime.NumCPU())
	}

	reader := history.NewReader(history.WithWorkers(workers))

	// Detect available shells
	shells := reader.DetectShells()
	if len(shells) == 0 {
		return fmt.Errorf("no shell history files detected. Make sure you have bash, zsh, fish, or PowerShell history")
	}

	fmt.Printf("Detected shells:\n")
	for shellType, path := range shells {
		fmt.Printf("  - %s: %s\n", shellType, path)
	}
	fmt.Println()

	// Read all histories concurrently
	fmt.Println("Reading shell histories...")
	entries, err := reader.ReadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to read shell histories: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No history entries found in shell files")
		return nil
	}

	// Show stats
	stats := reader.GetStats(entries)
	fmt.Printf("\nShell History Statistics:\n")
	fmt.Printf("Total Entries: %d\n", stats.TotalEntries)
	fmt.Printf("Unique Commands: %d\n", stats.UniqueCommands)
	for shellType, count := range stats.ByShell {
		fmt.Printf("  %s: %d entries\n", shellType, count)
	}
	if !stats.NewestTime.IsZero() {
		fmt.Printf("Time Range: %s to %s\n",
			stats.OldestTime.Format("2006-01-02"),
			stats.NewestTime.Format("2006-01-02"))
	}
	fmt.Println()

	// Import to database
	fmt.Println("Importing to WUT database...")
	imported, err := reader.ImportToDB(ctx, storage, entries)
	if err != nil {
		return fmt.Errorf("failed to import history: %w", err)
	}

	fmt.Printf("\n✅ Successfully imported %d commands from shell history\n", imported)
	fmt.Println("\nTip: Use 'wut history' to view your imported history")
	fmt.Println("     Use 'wut suggest' to get AI-powered suggestions based on your history")

	return nil
}

// parseCategory extracts category from command
func parseCategory(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}
