// Package cmd provides shortcut commands for WUT
// These commands provide shorter alternatives to common commands
package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func init() {
	// Add shortcut commands
	rootCmd.AddCommand(suggestShortcutCmd())
	rootCmd.AddCommand(historyShortcutCmd())
	rootCmd.AddCommand(explainShortcutCmd())
	rootCmd.AddCommand(aliasShortcutCmd())
	rootCmd.AddCommand(configShortcutCmd())
	rootCmd.AddCommand(dbShortcutCmd())
	rootCmd.AddCommand(fixShortcutCmd())
	rootCmd.AddCommand(smartShortcutCmd())
}

// executeMainCommand executes the main command using os.Exec
func executeMainCommand(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// suggestShortcutCmd creates 's' as a shortcut for 'suggest'
func suggestShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "s [command]",
		Short: "Shortcut for 'suggest'",
		Long:  `Quick shortcut for the suggest command. Same as 'wut suggest'.`,
		Example: `  wut s git
  wut s docker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allArgs := buildArgs(cmd, args, "suggest", []string{"raw", "quiet", "offline", "exec"})
			return executeMainCommand(allArgs...)
		},
	}

	// Add common flags
	cmd.Flags().BoolP("raw", "r", false, "output raw text")
	cmd.Flags().BoolP("quiet", "q", false, "quiet mode")
	cmd.Flags().BoolP("offline", "o", false, "offline mode")
	cmd.Flags().BoolP("exec", "e", false, "execute command")

	return cmd
}

// historyShortcutCmd creates 'h' as a shortcut for 'history'
func historyShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "h",
		Short: "Shortcut for 'history'",
		Long:  `Quick shortcut for the history command. Same as 'wut history'.`,
		Example: `  wut h
  wut h --stats`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allArgs := buildArgs(cmd, args, "history", []string{"stats", "clear", "import-shell"})
			return executeMainCommand(allArgs...)
		},
	}

	cmd.Flags().Bool("stats", false, "show statistics")
	cmd.Flags().Bool("clear", false, "clear history")
	cmd.Flags().Bool("import-shell", false, "import from shell history")
	cmd.Flags().IntP("limit", "l", 20, "number of entries")
	cmd.Flags().StringP("search", "s", "", "search term")

	return cmd
}

// explainShortcutCmd creates 'x' (for explain) as a shortcut for 'explain'
func explainShortcutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "x [command]",
		Short: "Shortcut for 'explain'",
		Long:  `Quick shortcut for the explain command. Same as 'wut explain'.`,
		Example: `  wut x "git rebase"
  wut x "rm -rf /"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMainCommand(append([]string{"explain"}, args...)...)
		},
	}
}

// aliasShortcutCmd creates 'a' as a shortcut for 'alias'
func aliasShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "a",
		Short: "Shortcut for 'alias'",
		Long:  `Quick shortcut for the alias command. Same as 'wut alias'.`,
		Example: `  wut a --list
  wut a --generate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allArgs := buildArgs(cmd, args, "alias", []string{"list", "generate", "add", "apply"})
			return executeMainCommand(allArgs...)
		},
	}

	cmd.Flags().BoolP("list", "l", false, "list all aliases")
	cmd.Flags().BoolP("generate", "g", false, "generate smart aliases")
	cmd.Flags().Bool("add", false, "add a new alias")
	cmd.Flags().Bool("apply", false, "apply aliases to shell config")
	cmd.Flags().StringP("name", "n", "", "alias name")
	cmd.Flags().StringP("command", "c", "", "alias command")

	return cmd
}

// configShortcutCmd creates 'c' as a shortcut for 'config'
func configShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "c",
		Short: "Shortcut for 'config'",
		Long:  `Quick shortcut for the config command. Same as 'wut config'.`,
		Example: `  wut c --get ui.theme
  wut c --set ui.theme dark`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle flags specially
			var allArgs []string
			allArgs = append(allArgs, "config")

			if cmd.Flags().Changed("list") {
				allArgs = append(allArgs, "--list")
			}
			if cmd.Flags().Changed("get") {
				v, _ := cmd.Flags().GetString("get")
				allArgs = append(allArgs, "--get", v)
			}
			if cmd.Flags().Changed("set") {
				v, _ := cmd.Flags().GetString("set")
				allArgs = append(allArgs, "--set", v)
			}
			if cmd.Flags().Changed("value") {
				v, _ := cmd.Flags().GetString("value")
				allArgs = append(allArgs, "--value", v)
			}
			if cmd.Flags().Changed("edit") {
				allArgs = append(allArgs, "--edit")
			}
			if cmd.Flags().Changed("reset") {
				allArgs = append(allArgs, "--reset")
			}

			return executeMainCommand(allArgs...)
		},
	}

	cmd.Flags().BoolP("list", "l", false, "list all configuration keys")
	cmd.Flags().StringP("get", "g", "", "get configuration value")
	cmd.Flags().StringP("set", "s", "", "set configuration key")
	cmd.Flags().StringP("value", "v", "", "value to set")
	cmd.Flags().BoolP("edit", "e", false, "open in default editor")
	cmd.Flags().BoolP("reset", "r", false, "reset to defaults")

	return cmd
}

// dbShortcutCmd creates 'd' as a shortcut for 'db'
func dbShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "d",
		Short: "Shortcut for 'db'",
		Long: `Quick shortcut for the db command. Same as 'wut db'.

Available subcommands:
  d sync    - Sync command database
  d status  - Show database status
  d clear   - Clear local database
  d update  - Update stale pages`,
		Example: `  wut d sync
  wut d status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMainCommand("db", "status")
		},
	}

	// Add subcommands
	cmd.AddCommand(&cobra.Command{
		Use:   "sync [commands...]",
		Short: "Sync command database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMainCommand(append([]string{"db", "sync"}, args...)...)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show database status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMainCommand("db", "status")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Clear database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMainCommand("db", "clear")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Update stale pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMainCommand("db", "update")
		},
	})

	return cmd
}

// fixShortcutCmd creates 'f' as a shortcut for 'fix'
func fixShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "f [typo]",
		Short: "Shortcut for 'fix'",
		Long:  `Quick shortcut for the fix command. Same as 'wut fix'.`,
		Example: `  wut f "gti commit"
  wut f "docer ps"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allArgs := buildArgs(cmd, args, "fix", []string{"copy", "list"})
			return executeMainCommand(allArgs...)
		},
	}

	cmd.Flags().BoolP("copy", "c", false, "copy to clipboard")
	cmd.Flags().BoolP("list", "l", false, "list common typos")

	return cmd
}

// smartShortcutCmd creates '?' as a shortcut for 'smart'
func smartShortcutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "? [query]",
		Short: "Smart command suggestion",
		Long:  `Smart natural language command suggestion. Same as 'wut smart'.`,
		Example: `  wut ? "how to find large files"
  wut ? "compress folder to tar.gz"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allArgs := buildArgs(cmd, args, "smart", []string{"exec", "correct"})
			return executeMainCommand(allArgs...)
		},
	}

	cmd.Flags().BoolP("exec", "e", false, "execute selected command")
	cmd.Flags().BoolP("correct", "c", true, "auto-correct typos")

	return cmd
}

// buildArgs builds an argument list from the set flags
func buildArgs(cmd *cobra.Command, args []string, command string, boolFlags []string) []string {
	var allArgs []string
	allArgs = append(allArgs, command)

	for _, flag := range boolFlags {
		if cmd.Flags().Changed(flag) {
			val, _ := cmd.Flags().GetBool(flag)
			if val {
				allArgs = append(allArgs, "--"+flag)
			}
		}
	}

	allArgs = append(allArgs, args...)
	return allArgs
}
