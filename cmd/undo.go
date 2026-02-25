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

var undoCmd = &cobra.Command{
	Use:   "undo [command]",
	Short: "Suggest how to undo a specific command or your last action",
	Long: `Accidentally ran a command and want to revert it? 
'wut undo' analyzes the command (or your recent history) and provides the exact command needed to undo the changes.`,
	Example: `  wut undo "git add ."
  wut undo "tar -xf archive.tar"
  wut undo  // Automatically finds and suggests undo for your last command`,
	RunE: runUndo,
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

// undoRule represents a pattern and its corresponding undo suggestion
type undoRule struct {
	Prefix      string
	Description string
	UndoCmd     func(args string) string
	Warning     string
}

// predefined undo rules
var undoRules = []undoRule{
	{
		Prefix:      "git add",
		Description: "Unstage files from the index",
		UndoCmd:     func(args string) string { return "git restore --staged " + args },
		Warning:     "",
	},
	{
		Prefix:      "git commit",
		Description: "Undo the last commit while keeping your working changes",
		UndoCmd:     func(args string) string { return "git reset --soft HEAD~1" },
		Warning:     "If you already pushed this commit, you'll need to force push (not recommended for shared branches).",
	},
	{
		Prefix:      "git push",
		Description: "Revert the pushed commits safely without rewriting history",
		UndoCmd:     func(args string) string { return "git revert HEAD" },
		Warning:     "To completely remove it from remote history, use 'git push -f' after 'git reset HEAD~1'.",
	},
	{
		Prefix:      "git merge",
		Description: "Abort a merge in progress, or revert a completed merge",
		UndoCmd: func(args string) string {
			return "git merge --abort    # (if in progress)\ngit reset --merge HEAD~1  # (if completed)"
		},
		Warning: "",
	},
	{
		Prefix:      "git rebase",
		Description: "Abort an ongoing rebase",
		UndoCmd:     func(args string) string { return "git rebase --abort" },
		Warning:     "",
	},
	{
		Prefix:      "git checkout",
		Description: "Go back to the previous branch you were on",
		UndoCmd:     func(args string) string { return "git checkout -" },
		Warning:     "",
	},
	{
		Prefix:      "tar -xf",
		Description: "Remove all files that were just extracted (requires knowing the contents)",
		UndoCmd:     func(args string) string { return "tar -tf " + args + " | xargs rm -rf" },
		Warning:     "Be extremely careful. This will forcefully delete all files listed in the archive.",
	},
	{
		Prefix:      "tar -xzf",
		Description: "Remove all files that were just extracted",
		UndoCmd:     func(args string) string { return "tar -tzf " + args + " | xargs rm -rf" },
		Warning:     "Be extremely careful. This will forcefully delete all files listed in the archive.",
	},
	{
		Prefix:      "mkdir",
		Description: "Remove the created directory",
		UndoCmd:     func(args string) string { return "rmdir " + args + "\n# or 'rm -rf " + args + "' if it isn't empty" },
		Warning:     "",
	},
	{
		Prefix:      "touch",
		Description: "Delete the created file",
		UndoCmd:     func(args string) string { return "rm " + args },
		Warning:     "",
	},
	{
		Prefix:      "systemctl start",
		Description: "Stop the started systemd service",
		UndoCmd:     func(args string) string { return "sudo systemctl stop " + args },
		Warning:     "",
	},
	{
		Prefix:      "systemctl stop",
		Description: "Start the stopped systemd service",
		UndoCmd:     func(args string) string { return "sudo systemctl start " + args },
		Warning:     "",
	},
	{
		Prefix:      "chown",
		Description: "Revert ownership recursively to the standard fallback (root) or previous owner",
		UndoCmd: func(args string) string {
			return "sudo chown root:root " + args + "   # (adjust to actual previous owner)"
		},
		Warning: "System cannot know the previous owner automatically.",
	},
	{
		Prefix:      "chmod",
		Description: "Restore permissions (No universal undo)",
		UndoCmd: func(args string) string {
			return "chmod 644 " + args + "   # for files\nchmod 755 " + args + "   # for directories"
		},
		Warning: "Actual previous permissions are lost.",
	},
	{
		Prefix:      "rm",
		Description: "Attempt to undelete files",
		UndoCmd: func(args string) string {
			return "# No direct CLI undo. You must rely on undelete utilities (e.g. extundelete/testdisk) or backups."
		},
		Warning: "The 'rm' command is usually permanent in Unix-like systems. Act quickly and unmount the drive if it is critical.",
	},
	{
		Prefix:      "npm install",
		Description: "Uninstall the added npm packages",
		UndoCmd:     func(args string) string { return "npm uninstall " + args },
		Warning:     "",
	},
	{
		Prefix:      "docker run",
		Description: "Stop and remove the container",
		UndoCmd:     func(args string) string { return "docker stop <container_id> && docker rm <container_id>" },
		Warning:     "You need the ID or Name returned by the run command.",
	},
}

func runUndo(cmd *cobra.Command, args []string) error {
	var targetCmd string

	// 1. If arguments are provided, use them as the target command
	if len(args) > 0 {
		targetCmd = strings.Join(args, " ")
	} else {
		// 2. Otherwise, fetch the last executed command from DB history
		cfg := config.Get()
		dbPath := cfg.Database.Path
		if dbPath == "" {
			home, _ := os.UserHomeDir()
			dbPath = home + "/.config/wut/wut.db"
		}

		store, err := db.NewStorage(dbPath)
		if err == nil {
			defer store.Close()
			// Fetch a bit more just in case the latest are 'wut' commands
			history, err := store.GetHistory(context.Background(), 10)
			if err == nil && len(history) > 0 {
				for _, entry := range history {
					c := strings.TrimSpace(entry.Command)
					// Skip any wut commands in the history
					if c != "" && !strings.HasPrefix(c, "wut") {
						targetCmd = c
						break
					}
				}
			}
		}
	}

	if targetCmd == "" {
		fmt.Println("No recent command found to undo. Please explicitly provide a command: wut undo \"git add .\"")
		return nil
	}

	targetCmd = strings.TrimSpace(targetCmd)
	logger.Info("Attempting to undo command", "command", targetCmd)

	// Display header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))
	fmt.Println()
	fmt.Println(headerStyle.Render("⏪ Undo Assistant"))
	fmt.Println()
	fmt.Printf("Command: %s\n\n", ui.Cyan(targetCmd))

	// Find matching rule
	for _, rule := range undoRules {
		if strings.HasPrefix(targetCmd, rule.Prefix) {

			// Extract arguments passed to the command (if any)
			cmdArgs := strings.TrimSpace(strings.TrimPrefix(targetCmd, rule.Prefix))

			actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
			fmt.Printf("Action: %s\n", actionStyle.Render(rule.Description))
			fmt.Println()
			fmt.Println(ui.Accent(rule.UndoCmd(cmdArgs)))

			if rule.Warning != "" {
				fmt.Println()
				warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")) // Red
				fmt.Printf("⚠️  %s\n", warningStyle.Render(rule.Warning))
			}
			fmt.Println()
			return nil
		}
	}

	// No rule matched
	fmt.Println(ui.Muted("🤷 I do not have a specific undo rule for this command."))
	fmt.Println(ui.Muted("Tip: Depending on the program, check its man page or undo feature."))
	fmt.Println("\n" + ui.Mascot())
	fmt.Println()

	return nil
}
