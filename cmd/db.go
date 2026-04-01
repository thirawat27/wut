// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/ui"
)

// dbCmd represents the db command
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage command database",
	Long: `Manage command database for offline access.

The database contains command cheat sheets from TLDR Pages.
This command allows you to sync and manage the local database.`,
}

var (
	dbSyncAll bool
	dbForce   bool
	dbOffline bool

	dbUpdateDays    int
	dbUpdateOffline bool
)

// dbSyncCmd represents the sync subcommand
var dbSyncCmd = &cobra.Command{
	Use:   "sync [commands...]",
	Short: "Sync command pages to local database",
	Long: `Download and cache command pages to local database for offline access.

If no commands are specified, syncs popular commands.
Use --all to sync all available commands.`,
	Example: `  wut db sync                    # Sync popular commands
  wut db sync git docker npm     # Sync specific commands
  wut db sync --all              # Sync all commands (may take a while)
  wut db sync --force            # Force update existing pages
  wut db sync --offline git      # Import from local tldr-main checkout only`,
	RunE: runDBSync,
}

// dbStatusCmd represents the status subcommand
var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database status",
	Long:  `Display information about the local database.`,
	RunE:  runDBStatus,
}

// dbClearCmd represents the clear subcommand
var dbClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear local database",
	Long:  `Remove all cached command pages from local database.`,
	RunE:  runDBClear,
}

// dbUpdateCmd represents the update subcommand
var dbUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update stale command pages",
	Long: `Check for and update stale command pages in the local database.

By default, uses tldr.auto_sync_interval (7 days unless configured).`,
	Example: `  wut db update
  wut db update --days 3
  wut db update --offline`,
	RunE: runDBUpdate,
}

func init() {
	rootCmd.AddCommand(dbCmd)

	dbCmd.AddCommand(dbSyncCmd)
	dbCmd.AddCommand(dbStatusCmd)
	dbCmd.AddCommand(dbClearCmd)
	dbCmd.AddCommand(dbUpdateCmd)

	// Sync flags
	dbSyncCmd.Flags().BoolVarP(&dbSyncAll, "all", "a", false, "sync all commands (may take a while)")
	dbSyncCmd.Flags().BoolVarP(&dbForce, "force", "f", false, "force update existing pages")
	dbSyncCmd.Flags().BoolVar(&dbOffline, "offline", false, "sync from local TLDR source only (no network)")

	// Update flags
	dbUpdateCmd.Flags().IntVar(&dbUpdateDays, "days", 7, "update pages older than this many days")
	dbUpdateCmd.Flags().BoolVar(&dbUpdateOffline, "offline", false, "update from local TLDR source only (no network)")
}

func runDBSync(cmd *cobra.Command, args []string) error {
	// Get database path
	dbPath := getDBPath()

	// Create storage
	storage, err := db.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer storage.Close()

	// Create sync manager
	syncManager := db.NewSyncManager(storage)

	ctx := context.Background()
	var result *db.SyncResult

	err = ui.RunWithSpinner("Syncing command database...", func() error {
		var syncErr error
		opts := db.SyncOptions{
			Commands:    args,
			ForceUpdate: dbForce,
			Offline:     dbOffline,
		}

		if dbSyncAll {
			result, syncErr = syncManager.SyncAllWithOptions(ctx, opts)
		} else if len(args) > 0 {
			result, syncErr = syncManager.SyncCommandsWithOptions(ctx, opts)
		} else {
			result, syncErr = syncManager.SyncPopularWithOptions(ctx, opts)
		}
		return syncErr
	})

	fmt.Println()

	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Display results
	fmt.Println(formatSyncResult(result))

	return nil
}

func runDBStatus(cmd *cobra.Command, args []string) error {
	dbPath := getDBPath()

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("❌ Local database not found")
		fmt.Println()
		fmt.Println("Run 'wut db sync' to create the database")
		return nil
	}

	// Open storage
	storage, err := db.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer storage.Close()

	// Get stats
	stats, err := storage.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	autoSyncDays := config.Get().TLDR.AutoSyncInterval
	if autoSyncDays <= 0 {
		autoSyncDays = 7
	}
	stalePages, err := storage.ListStalePages(time.Duration(autoSyncDays)*24*time.Hour, 0)
	if err != nil {
		return fmt.Errorf("failed to inspect stale pages: %w", err)
	}
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("failed to stat database: %w", err)
	}
	stats["db_path"] = dbPath
	stats["db_size_bytes"] = fileInfo.Size()
	stats["stale_pages"] = len(stalePages)
	stats["stale_threshold_days"] = autoSyncDays

	// Display status
	fmt.Println(formatStatus(stats))

	return nil
}

func runDBClear(cmd *cobra.Command, args []string) error {
	dbPath := getDBPath()

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("ℹ️  Database already empty")
		return nil
	}

	// Confirm
	fmt.Print("⚠️  Are you sure you want to clear the database? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Println("Cancelled")
		return nil
	}

	// Open storage
	storage, err := db.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer storage.Close()

	// Clear all
	if err := storage.ClearAll(); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	fmt.Println("✅ Database cleared")

	return nil
}

func runDBUpdate(cmd *cobra.Command, args []string) error {
	dbPath := getDBPath()

	// Open storage
	storage, err := db.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer storage.Close()

	// Create sync manager
	syncManager := db.NewSyncManager(storage)

	ctx := context.Background()
	updateDays := dbUpdateDays
	if !cmd.Flags().Changed("days") {
		if configuredDays := config.Get().TLDR.AutoSyncInterval; configuredDays > 0 {
			updateDays = configuredDays
		}
	}
	if updateDays <= 0 {
		return fmt.Errorf("--days must be greater than 0")
	}
	maxAge := time.Duration(updateDays) * 24 * time.Hour

	totalPages, err := storage.CountPages()
	if err != nil {
		return fmt.Errorf("failed to inspect database: %w", err)
	}
	if totalPages == 0 {
		fmt.Println("ℹ️  Database is empty")
		fmt.Println()
		fmt.Println("Run 'wut db sync' to download command pages first")
		return nil
	}

	var result *db.SyncResult

	err = ui.RunWithSpinner("Updating stale pages...", func() error {
		var syncErr error
		result, syncErr = syncManager.UpdateStalePages(ctx, maxAge, db.SyncOptions{
			Offline: dbUpdateOffline,
		})
		return syncErr
	})

	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if result.Downloaded == 0 && result.Failed == 0 && result.Skipped == 0 {
		fmt.Printf("✅ No stale pages older than %d days found\n", updateDays)
		return nil
	}

	fmt.Println()
	fmt.Println(formatSyncResult(result))

	return nil
}

// getDBPath returns the path to the database
func getDBPath() string {
	return config.GetTLDRDatabasePath()
}

// formatSyncResult formats the sync result for display
func formatSyncResult(result *db.SyncResult) string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#10B981")).
		Render("✅ Sync Complete")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Stats
	stats := []struct {
		label string
		value int
		color string
	}{
		{"Downloaded", result.Downloaded, "#10B981"},
		{"Skipped", result.Skipped, "#F59E0B"},
		{"Failed", result.Failed, "#EF4444"},
	}

	for _, s := range stats {
		if s.value > 0 {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(s.color)).
				Render(fmt.Sprintf("  • %s: %d", s.label, s.value)))
			b.WriteString("\n")
		}
	}

	// Duration
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render(fmt.Sprintf("  • Duration: %s", result.Duration)))
	b.WriteString("\n")

	// Errors
	if len(result.Errors) > 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Render("Errors:"))
		b.WriteString("\n")
		for _, err := range result.Errors[:min(len(result.Errors), 5)] {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Render(fmt.Sprintf("  • %v", err)))
			b.WriteString("\n")
		}
		if len(result.Errors) > 5 {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Render(fmt.Sprintf("  ... and %d more errors", len(result.Errors)-5)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func formatStatus(stats map[string]any) string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Render("📊 Database Status")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Total pages
	totalPages := 0
	if v, ok := stats["total_pages"].(int); ok {
		totalPages = v
	}
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Render(fmt.Sprintf("  Total Pages: %d", totalPages)))
	b.WriteString("\n")

	if stalePages, ok := stats["stale_pages"].(int); ok {
		days := 7
		if v, ok := stats["stale_threshold_days"].(int); ok && v > 0 {
			days = v
		}
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Render(fmt.Sprintf("  Stale Pages (> %d days): %d", days, stalePages)))
		b.WriteString("\n")
	}

	// Last sync
	if lastSync, ok := stats["last_sync"].(time.Time); ok {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6")).
			Render(fmt.Sprintf("  Last Sync: %s", lastSync.Format("2006-01-02 15:04"))))
		b.WriteString("\n")
	}

	if sizeBytes, ok := stats["db_size_bytes"].(int64); ok {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Render(fmt.Sprintf("  Database Size: %s", formatBytes(sizeBytes))))
		b.WriteString("\n")
	}

	if dbPath, ok := stats["db_path"].(string); ok && dbPath != "" {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render(fmt.Sprintf("  Path: %s", dbPath)))
		b.WriteString("\n")
	}

	// Platforms
	if platforms, ok := stats["platforms"].(map[string]int); ok && len(platforms) > 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F59E0B")).
			Render("Platforms:"))
		b.WriteString("\n")
		for platform, count := range platforms {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Render(fmt.Sprintf("  • %s: %d", platform, count)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func formatBytes(size int64) string {
	const unit = int64(1024)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := unit, 0
	for n := size / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}

	suffixes := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), suffixes[exp])
}
