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
		if len(lastCmd) < 15 {
			return nil
		}

		storage, err := db.NewStorage(cfg.Database.Path)
		if err != nil {
			return nil
		}
		defer storage.Close()

		ctx := context.Background()

		// Save the executed command to the sequential history log
		_ = storage.AddHistory(ctx, lastCmd)

		// Check history stats to infer frequency
		stats, err := storage.GetHistoryStats(ctx)
		if err != nil {
			return nil
		}

		for _, top := range stats.TopCommands {
			if top.Command == lastCmd && top.Count >= 5 {
				// Suggest creating an alias
				tipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308")).Bold(true)
				cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

				fmt.Printf("\n  💡 %s\n  %s\n",
					tipStyle.Render("Tip: You run this long command frequently! Want a shortcut?"),
					lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(fmt.Sprintf("Run: wut a --add myalias \"%s\"", cmdStyle.Render(lastCmd))),
				)
				break
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(tipCmd)
}
