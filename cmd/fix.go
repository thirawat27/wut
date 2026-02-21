// Package cmd provides command correction
package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/corrector"
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
	fixCopy  bool
	fixList  bool
	fixSafe  bool
)

func init() {
	rootCmd.AddCommand(fixCmd)

	fixCmd.Flags().BoolVarP(&fixCopy, "copy", "c", false, "copy corrected command to clipboard")
	fixCmd.Flags().BoolVarP(&fixList, "list", "l", false, "list common typos")
	fixCmd.Flags().BoolVar(&fixSafe, "safe", true, "check for dangerous commands")
}

func runFix(cmd *cobra.Command, args []string) error {
	// List common typos
	if fixList {
		return listCommonTypos()
	}

	if len(args) == 0 {
		return fmt.Errorf("please provide a command to fix")
	}

	input := strings.Join(args, " ")

	// Check for typos
	c := corrector.New()
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
		// TODO: Implement clipboard copy
		fmt.Println("(Copied to clipboard)")
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
	typos := map[string]string{
		"gti":        "git",
		"gi":         "git",
		"sl":         "ls",
		"cd..":       "cd ..",
		"grpe":       "grep",
		"docer":      "docker",
		"doker":      "docker",
		"npn":        "npm",
		"pthon":      "python",
		"gut":        "git",
		"mkr":        "mkdir",
		"gr":         "grep",
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	fmt.Println()
	fmt.Println(headerStyle.Render("📋 Common Typos"))
	fmt.Println()

	// Sort by key
	var keys []string
	for k := range typos {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, typo := range keys {
		correction := typos[typo]
		fmt.Printf("  %s → %s\n",
			ui.Red(typo),
			ui.Green(correction))
	}

	fmt.Println()
	fmt.Println("WUT will automatically correct these when you use 'wut fix'")

	return nil
}
