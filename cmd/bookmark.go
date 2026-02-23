package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var bookmarkCmd = &cobra.Command{
	Use:     "bookmark",
	Aliases: []string{"b", "bm"},
	Short:   "Save and organize your favorite commands with labels",
	Long: `Bookmark system allows you to record, tag, and organize favorite WUT commands.
You can label them and add notes for quick recall.`,
	RunE: runBookmarkList,
}

var bookmarkAddCmd = &cobra.Command{
	Use:   "add [command]",
	Short: "Add a new mapped command",
	RunE:  runBookmarkAdd,
}

var bookmarkRemoveCmd = &cobra.Command{
	Use:     "remove [id/label]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a bookmark",
	RunE:    runBookmarkRemove,
}

var bookmarkSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search through your bookmarks",
	RunE:  runBookmarkSearch,
}

var bmLabel string
var bmNotes string

func init() {
	rootCmd.AddCommand(bookmarkCmd)
	bookmarkCmd.AddCommand(bookmarkAddCmd)
	bookmarkCmd.AddCommand(bookmarkRemoveCmd)
	bookmarkCmd.AddCommand(bookmarkSearchCmd)

	bookmarkAddCmd.Flags().StringVarP(&bmLabel, "label", "l", "default", "Label for the command")
	bookmarkAddCmd.Flags().StringVarP(&bmNotes, "notes", "n", "", "Optional notes")
}

func getDB() (*db.Storage, error) {
	cfg := config.Get()
	dbPath := cfg.Database.Path
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = home + "/.config/wut/wut.db"
	}
	return db.NewStorage(dbPath)
}

func runBookmarkAdd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide the command to bookmark. Ex: wut bookmark add 'docker ps' -l docker")
	}

	commandStr := strings.Join(args, " ")
	store, err := getDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	err = store.AddBookmark(context.Background(), commandStr, bmLabel, bmNotes)
	if err != nil {
		return fmt.Errorf("failed to add bookmark: %w", err)
	}

	fmt.Printf("%s Successfully bookmarked command: %s\n", ui.Green("✓"), ui.Cyan(commandStr))
	fmt.Printf("   Label: %s\n", ui.Accent(bmLabel))
	return nil
}

func printBookmarks(bookmarks []db.Bookmark) {
	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks found.")
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	fmt.Println(titleStyle.Render("📌 Your Bookmarks"))
	fmt.Println()

	for _, bm := range bookmarks {
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true) // Emerald
		cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))              // Blue

		fmt.Printf(" %s [%s] %s\n", ui.Muted(bm.ID[len(bm.ID)-6:]), labelStyle.Render(bm.Label), cmdStyle.Render(bm.Command))
		if bm.Notes != "" {
			fmt.Printf("      %s\n", ui.Muted("🗒️  "+bm.Notes))
		}
	}
	fmt.Println()
	fmt.Println(ui.Muted("Use 'wut bookmark remove <id>' to delete, or 'wut bookmark add' to save new ones."))
}

func runBookmarkList(cmd *cobra.Command, args []string) error {
	logger.Info("listing bookmarks")
	store, err := getDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	bms, err := store.GetBookmarks(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get bookmarks: %w", err)
	}

	printBookmarks(bms)
	return nil
}

func runBookmarkSearch(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a search query")
	}
	query := strings.Join(args, " ")

	store, err := getDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	bms, err := store.SearchBookmarks(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to search bookmarks: %w", err)
	}

	printBookmarks(bms)
	return nil
}

func runBookmarkRemove(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide the ID or label of the bookmark to delete")
	}
	idOrLabel := args[0]

	store, err := getDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	bms, err := store.GetBookmarks(context.Background())
	if err != nil {
		return err
	}

	var foundID string
	for _, bm := range bms {
		if strings.HasSuffix(bm.ID, idOrLabel) || bm.ID == idOrLabel || bm.Label == idOrLabel {
			foundID = bm.ID
			break
		}
	}

	if foundID == "" {
		return fmt.Errorf("bookmark not found with identifier: %s", idOrLabel)
	}

	if err := store.DeleteBookmark(context.Background(), foundID); err != nil {
		return err
	}

	fmt.Printf("%s Bookmark removed successfully.\n", ui.Green("✓"))
	return nil
}
