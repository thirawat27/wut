// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/db"
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
  wut db sync --force            # Force update existing pages`,
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

By default, updates pages older than 7 days.`,
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
	dbSyncCmd.Flags().BoolVar(&dbOffline, "offline", false, "work in offline mode")
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

	fmt.Println("🔄 Syncing command database...")
	fmt.Println()

	// Determine what to sync
	if dbSyncAll {
		// Sync all available commands
		fmt.Println("This may take a while as we're downloading all pages...")
		result, err = syncManager.SyncAll(ctx)
	} else if len(args) > 0 {
		// Sync specific commands
		result, err = syncManager.SyncCommands(ctx, args)
	} else {
		// Sync popular commands
		result, err = syncManager.SyncPopular(ctx)
	}

	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Display results
	fmt.Println()
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

	// Check if stale
	if !syncManager.IsStale(7 * 24 * time.Hour) {
		lastSync, _ := syncManager.GetLastSync()
		fmt.Printf("✅ Database is up to date (last sync: %s)\n", lastSync.Format("2006-01-02"))
		return nil
	}

	// Auto sync
	fmt.Println("🔄 Updating stale pages...")
	result, err := syncManager.AutoSync(ctx, 7*24*time.Hour)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println()
	fmt.Println(formatSyncResult(result))

	return nil
}

// getDBPath returns the path to the database
func getDBPath() string {
	cfg := config.Get()
	if cfg.Database.Path != "" {
		return filepath.Join(filepath.Dir(cfg.Database.Path), "tldr.db")
	}

	// Default path
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wut", "tldr.db")
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

	// Last sync
	if lastSync, ok := stats["last_sync"].(time.Time); ok {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6")).
			Render(fmt.Sprintf("  Last Sync: %s", lastSync.Format("2006-01-02 15:04"))))
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
