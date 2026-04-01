package cmd

import (
	"context"
	"fmt"
	"strings"

	"wut/internal/config"
	"wut/internal/db"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tipCmd = &cobra.Command{
	Use:    "pro-tip [command]",
	Short:  "Check if a proactive tip should be shown for a given command",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}

		cfg := config.Get()
		if !cfg.History.Enabled {
			return nil
		}

		lastCmd := strings.TrimSpace(strings.Join(args, " "))
		if lastCmd == "" || strings.HasPrefix(lastCmd, "wut ") {
			return nil
		}

		storage, err := db.NewStorage(config.GetDatabasePath())
		if err != nil {
			return nil
		}
		defer storage.Close()

		ctx := context.Background()

		// Always save the executed command so history-backed search can learn from
		// real shell usage instead of only long commands.
		_ = storage.AddHistory(ctx, lastCmd)
		if cfg.History.MaxEntries > 0 {
			_ = storage.TrimHistory(ctx, cfg.History.MaxEntries)
		}

		if len(lastCmd) < 15 {
			return nil
		}

		count, err := storage.GetCommandUsageCount(ctx, lastCmd, 5)
		if err != nil {
			return nil
		}
		if count < 5 {
			return nil
		}

		tipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308")).Bold(true)
		cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

		fmt.Printf("\n  💡 %s\n  %s\n",
			tipStyle.Render("Tip: You run this long command frequently! Want a shortcut?"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(fmt.Sprintf("Run: wut a --add myalias \"%s\"", cmdStyle.Render(lastCmd))),
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(tipCmd)
}
