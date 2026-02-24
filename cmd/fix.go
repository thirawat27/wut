package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/corrector"
	"wut/internal/db"
	"wut/internal/ui"
)

// fixCmd corrects typos in commands
var fixCmd = &cobra.Command{
	Use:   "fix [command]",
	Short: "Fix typos in your commands",
	Long: `Correct common typos and suggest the right command.
WUT will detect typos, dangerous commands, and suggest alternatives.`,
	Example: `  wut fix "gti status"
  wut fix "doker ps"
  wut fix "rm -rf /"`,
	RunE: runFix,
}

var (
	fixCopy bool
	fixList bool
)

func init() {
	rootCmd.AddCommand(fixCmd)

	fixCmd.Flags().BoolVarP(&fixCopy, "copy", "c", false, "copy corrected command to clipboard")
	fixCmd.Flags().BoolVarP(&fixList, "list", "l", false, "list common typos")
}

func runFix(cmd *cobra.Command, args []string) error {
	// 1. Setup storage and corrector
	cfg := config.Get()
	dbPath := cfg.Database.Path
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = home + "/.config/wut/wut.db"
	}

	store, err := db.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	c := corrector.New()

	// Populate corrector with history for better fuzzy matching
	if history, err := store.GetHistory(context.Background(), 100); err == nil {
		var historyCmds []string
		for _, h := range history {
			historyCmds = append(historyCmds, h.Command)
		}
		c.SetHistoryCommands(historyCmds)
	}

	// 2. Handle --list flag
	if fixList {
		return listCommonTypos()
	}

	// 3. Get input: either from args or last history command
	input := ""
	if len(args) > 0 {
		input = strings.Join(args, " ")
	} else {
		// Fetch last command from history (skipping 'wut' commands)
		history, err := store.GetHistory(context.Background(), 10)
		if err == nil {
			for _, entry := range history {
				cmdStr := strings.TrimSpace(entry.Command)
				if cmdStr != "" && !strings.HasPrefix(cmdStr, "wut") {
					input = cmdStr
					break
				}
			}
		}
	}

	if input == "" {
		return fmt.Errorf("no command provided and no recent history found to fix")
	}

	// 4. Perform correction
	correction, err := c.Correct(input)
	if err != nil {
		return err
	}

	if correction == nil {
		// No correction needed
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Render("✓")
		fmt.Printf("%s %s\n", successStyle, "This command looks correct!")

		// Suggest alternatives
		alternatives := c.SuggestAlternative(input)
		if len(alternatives) > 0 {
			fmt.Println()
			fmt.Println("Modern alternatives:")
			for _, alt := range alternatives {
				fmt.Printf("  • %s\n", ui.Cyan(alt))
			}
		}

		return nil
	}

	// Display correction
	displayCorrection(correction)

	// Copy to clipboard if requested
	if fixCopy && correction.Corrected != "" {
		if err := clipboard.WriteAll(correction.Corrected); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Printf("%s Copied to clipboard\n", ui.Success("✓"))
	}

	return nil
}

func displayCorrection(c *corrector.Correction) {
	if c.IsDangerous {
		dangerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#DC2626")).
			Padding(0, 1)

		fmt.Println()
		fmt.Println(dangerStyle.Render(" ⚠️  DANGEROUS COMMAND DETECTED "))
		fmt.Println()
		fmt.Println(c.Explanation)
		fmt.Println()

		warningBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
			Padding(1).
			Render("Never run this command unless you absolutely know what you're doing!")
		fmt.Println(warningBox)
		fmt.Println()
		return
	}

	// Normal correction
	fmt.Println()
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))
	fmt.Println(headerStyle.Render("🤔 Did you mean:"))
	fmt.Println()

	// Show original
	fmt.Printf("  Original:  %s\n", ui.Red(c.Original))

	// Show corrected
	if c.Corrected != "" {
		fmt.Printf("  Corrected: %s\n", ui.Green(c.Corrected))
	}

	// Show explanation
	if c.Explanation != "" {
		fmt.Println()
		infoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
		fmt.Printf("  %s\n", infoStyle.Render(c.Explanation))
	}

	// Show confidence
	fmt.Println()
	confidenceStr := fmt.Sprintf("Confidence: %.0f%%", c.Confidence*100)
	var confidenceColor lipgloss.Color
	switch {
	case c.Confidence >= 0.9:
		confidenceColor = lipgloss.Color("#10B981")
	case c.Confidence >= 0.7:
		confidenceColor = lipgloss.Color("#F59E0B")
	default:
		confidenceColor = lipgloss.Color("#6B7280")
	}
	confidenceStyle := lipgloss.NewStyle().
		Foreground(confidenceColor)
	fmt.Printf("  %s\n", confidenceStyle.Render(confidenceStr))

	fmt.Println()
}

func listCommonTypos() error {
	// Use a slice of examples since the new corrector uses a dynamic corpus
	examples := []struct {
		Typo    string
		Correct string
	}{
		{"gti comit", "git commit"},
		{"dockr buld", "docker build"},
		{"kubctl dpoly", "kubectl deploy"},
		{"terrform applay", "terraform apply"},
		{"npn isntall", "npm install"},
		{"systemtcl strat", "systemctl start"},
		{"cd..", "cd .."},
		{"grpe", "grep"},
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	fmt.Println()
	fmt.Println(headerStyle.Render("📋 Core Typo Correction Patterns"))
	fmt.Println()

	for _, ex := range examples {
		fmt.Printf("  %s → %s\n",
			ui.Red(ex.Typo),
			ui.Green(ex.Correct))
	}

	fmt.Println()
	fmt.Println("WUT will automatically correct these when you use 'wut fix'")

	return nil
}
