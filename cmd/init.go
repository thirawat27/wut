package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"wut/internal/config"
	"wut/internal/logger"
	"wut/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize WUT for first-time use",
	Long: `Interactive setup wizard for WUT configuration.

This command will:
  • Create configuration directory structure
  • Detect your shell and recommend integrations
  • Set up default preferences
  • Optionally sync TLDR pages

Run this when you first install WUT or want to reconfigure.`,
	Example: `  wut init              # Interactive setup
  wut init --quick      # Quick setup with defaults
  wut init --shell zsh  # Setup for specific shell`,
	RunE: runInit,
}

var (
	initQuick     bool
	initShell     string
	initSkipTLDR  bool
	initSkipShell bool
	initNonTUI    bool
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&initQuick, "quick", "q", false, "quick setup with defaults (non-interactive)")
	initCmd.Flags().StringVarP(&initShell, "shell", "s", "", "shell type (bash, zsh, fish, powershell)")
	initCmd.Flags().BoolVar(&initSkipTLDR, "skip-tldr", false, "skip TLDR pages setup")
	initCmd.Flags().BoolVar(&initSkipShell, "skip-shell", false, "skip shell integration setup")
	initCmd.Flags().BoolVar(&initNonTUI, "no-tui", false, "use simple text interface (no fancy UI)")
}

// Global UI colors
var (
	cBlue     = lipgloss.Color("#8B5CF6") // Changed to Purple/Violet for UI
	cCyan     = lipgloss.Color("#C4B5FD") // Light Purple
	cGreen    = lipgloss.Color("#10B981")
	cAmber    = lipgloss.Color("#F59E0B")
	cPink     = lipgloss.Color("#EC4899")
	cGray     = lipgloss.Color("#6B7280")
	cDarkGray = lipgloss.Color("#374151")
	cWhite    = lipgloss.Color("#F8F9FA")
)

// Helper methods for prompts
func askYN(prompt string, defaultYes bool) bool {
	q := lipgloss.NewStyle().Foreground(cPink).Bold(true).Render("?")
	p := lipgloss.NewStyle().Foreground(cWhite).Render(prompt)
	fmt.Printf("    %s  %s ", q, p)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if answer == "" {
			return defaultYes
		}
		return answer == "y" || answer == "yes" // accept any positive string
	}
	return defaultYes // Exits gracefully on EOF / Windows bug
}

func askChoice(prompt string, defaultVal string) string {
	q := lipgloss.NewStyle().Foreground(cPink).Bold(true).Render("?")
	p := lipgloss.NewStyle().Foreground(cWhite).Render(prompt)
	fmt.Printf("    %s  %s ", q, p)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(scanner.Text())
		if answer == "" {
			return defaultVal
		}
		return answer
	}
	return defaultVal
}

func runInit(cmd *cobra.Command, args []string) error {
	log := logger.With("init")
	log.Info("starting initialization wizard")

	// ─── Interrupt handling ────────────────────────────────────────────────────
	osSig := make(chan os.Signal, 1)
	signal.Notify(osSig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-osSig
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Foreground(cAmber).Bold(true).Render("\n  ⚠ Setup cancelled — you can re-run 'wut init' any time.\n"))
		os.Exit(1)
	}()

	// ─── Get dynamic terminal width ──────────────────────────────────────────
	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}

	heroWidth := 54
	if termWidth < 60 {
		heroWidth = termWidth - 4
	}
	if heroWidth < 30 {
		heroWidth = 30
	}

	// ─── Hero Banner ───────────────────────────────────────────────────────────
	if !initQuick {
		panelBorder := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBlue).
			Padding(1, 3)

		heroLogo := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(cBlue).
			Padding(0, 2).
			Render(" 🚀 WUT SETUP ")

		heroDesc := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(cCyan).Render("Supercharge your terminal workflow."),
			lipgloss.NewStyle().Foreground(cGray).Render("Press ")+
				lipgloss.NewStyle().Foreground(cPink).Render("Ctrl+C")+
				lipgloss.NewStyle().Foreground(cGray).Render(" anytime to abort."),
		)

		heroContent := lipgloss.JoinVertical(lipgloss.Left, heroLogo, "", heroDesc)
		fmt.Println()
		fmt.Println(panelBorder.Width(heroWidth).Render(heroContent))
	}

	totalSteps := 4
	if initSkipShell {
		totalSteps--
	}
	if initSkipTLDR {
		totalSteps--
	}
	stepNum := 0

	printStep := func(icon, title string) {
		stepNum++

		separatorLen := 50
		if termWidth < 60 {
			separatorLen = termWidth - 8
		}
		if separatorLen < 20 {
			separatorLen = 20
		}

		badge := lipgloss.NewStyle().Bold(true).Foreground(cBlue).Render(fmt.Sprintf("[%d/%d]", stepNum, totalSteps))
		heading := lipgloss.NewStyle().Bold(true).Foreground(cWhite).Render(icon + "  " + title)
		fmt.Printf("\n  %s  %s\n", badge, heading)
		fmt.Println(lipgloss.NewStyle().Foreground(cDarkGray).Render("  " + strings.Repeat("━", separatorLen)))
	}
	printOK := func(s string) {
		fmt.Printf("    %s  %s\n", lipgloss.NewStyle().Foreground(cGreen).Render("✓"), lipgloss.NewStyle().Foreground(cGray).Render(s))
		time.Sleep(300 * time.Millisecond) // Add slight premium delay
	}
	printWarn := func(s string) {
		fmt.Printf("    %s  %s\n", lipgloss.NewStyle().Foreground(cAmber).Render("⚠"), lipgloss.NewStyle().Foreground(cGray).Render(s))
	}
	valFmt := func(s string) string { return lipgloss.NewStyle().Foreground(cCyan).Render(s) }

	cfg := config.Get()

	// ─── Step 1: Directories ───────────────────────────────────────────────────
	if !initQuick {
		printStep("📁", "Directories Setup")
	}
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	if !initQuick {
		printOK("Configuration folders verified")
	}

	// ─── Step 2: Configuration ─────────────────────────────────────────────────
	if initQuick {
		cfg.UI.Theme = "auto"
		cfg.Fuzzy.Enabled = true
		cfg.History.Enabled = true
		cfg.Context.Enabled = true
		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	} else {
		printStep("⚙️ ", "Terminal Preferences")

		lbl := lipgloss.NewStyle().Foreground(cGray).Render
		opt := lipgloss.NewStyle().Foreground(cWhite).Render
		num := lipgloss.NewStyle().Foreground(cBlue).Bold(true).Render

		themeMenu := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(cDarkGray).
			PaddingLeft(2).
			MarginLeft(4).
			Render(
				lipgloss.JoinVertical(lipgloss.Left,
					lbl("Choose your preferred theme:"),
					fmt.Sprintf(" %s %s", num("1"), opt("Auto-detect (Recommended)")),
					fmt.Sprintf(" %s %s", num("2"), opt("Dark mode")),
					fmt.Sprintf(" %s %s", num("3"), opt("Light mode")),
				),
			)
		fmt.Println()
		fmt.Println(themeMenu)

		fmt.Println()
		choice := askChoice("Selection [1]:", "1")
		switch choice {
		case "2":
			cfg.UI.Theme = "dark"
		case "3":
			cfg.UI.Theme = "light"
		default:
			cfg.UI.Theme = "auto"
		}
		printOK("Theme profile set to " + valFmt(cfg.UI.Theme))
		fmt.Println()

		cfg.History.Enabled = askYN("Enable command history productivity tracking? [Y/n]:", true)
		printOK("History tracking " + boolToEnabled(cfg.History.Enabled))
		fmt.Println()

		cfg.Context.Enabled = askYN("Enable project context analysis to get smarter suggestions? [Y/n]:", true)
		printOK("Context analysis " + boolToEnabled(cfg.Context.Enabled))

		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// ─── Step 3: Shell Integration (auto-install, merged from 'wut install') ──
	if !initSkipShell {
		shellType := initShell
		if shellType == "" {
			shellType = detectShellForInit()
		}

		if !initQuick {
			printStep("🐚", "Shell Integration")
			fmt.Printf("    Detected active shell: %s\n\n", valFmt(shellType))
			fmt.Printf("    %s\n\n", lipgloss.NewStyle().Foreground(cGray).Render("Installing key bindings, command-not-found hooks, and pro-tips..."))
		}

		// Auto-install shell integration (replaces separate 'wut install' step)
		if err := installShellIntegration(shellType); err != nil {
			if !initQuick {
				if err.Error() == "already installed" {
					printOK("Shell hooks already installed")
				} else {
					printWarn("Shell integration: " + err.Error())
				}
			}
		} else {
			if !initQuick {
				printOK("Hooks installed successfully")
				reloadCmd := "source " + getShellRcFile(shellType)
				if shellType == "powershell" || shellType == "pwsh" {
					reloadCmd = ". " + getShellRcFile(shellType)
				}
				fmt.Printf("      %s Type %s to apply immediately.\n",
					lipgloss.NewStyle().Foreground(cPink).Render("→"),
					lipgloss.NewStyle().Foreground(cWhite).Render(reloadCmd),
				)
			}
		}
	}

	// ─── Step 4: TLDR Pages ────────────────────────────────────────────────────
	if !initSkipTLDR {
		if !initQuick {
			printStep("📚", "Offline Knowledge Base")

			descBox := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(cDarkGray).
				PaddingLeft(2).
				MarginLeft(4).
				Foreground(cGray).
				Render("TLDR pages provide instant offline cheat sheets\nfor almost any CLI tool on your system.")

			fmt.Println()
			fmt.Println(descBox)
			fmt.Println()

			if askYN("Download TLDR database now? (Highly Recommended) [Y/n]:", true) {
				fmt.Printf("    %s\n", lipgloss.NewStyle().Foreground(cGray).Render("Syncing... please wait a moment."))
				if err := runDBSync(dbSyncCmd, []string{}); err != nil {
					printWarn("Sync encountered an issue: " + err.Error())
				} else {
					printOK("Documentation is now offline")
				}
			} else {
				printOK("Skipped — run 'wut tldr sync' to execute later")
			}
		} else {
			fmt.Println("Download TLDR pages: wut tldr sync")
		}
	}

	// ─── Mark as initialized ──────────────────────────────────────────────────
	cfg.App.Initialized = true
	if err := config.Save(); err != nil {
		log.Error("failed to mark as initialized", "error", err)
	}

	// ─── Done Card ─────────────────────────────────────────────────────────────
	if !initQuick {
		fmt.Println()

		cmdCol := func(s string) string { return lipgloss.NewStyle().Foreground(cCyan).Bold(true).Render(s) }
		descCol := func(s string) string { return lipgloss.NewStyle().Foreground(cGray).Render(s) }

		doneBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cGreen).
			Padding(1, 3).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(cGreen).Bold(true).Render("🎉 Setup Complete!"),
				"",
				ui.Mascot(),
				"",
				lipgloss.NewStyle().Foreground(cWhite).Render("Pro tips to get started:"),
				fmt.Sprintf("  %s        %s", cmdCol("wut s <cmd>"), descCol("Search instant AI cheat sheets")),
				fmt.Sprintf("  %s               %s", cmdCol("wut h"), descCol("Interactive timeline history")),
				fmt.Sprintf("  %s           %s", cmdCol("wut stats"), descCol("Productivity metric dashboard")),
				fmt.Sprintf("  %s        %s", cmdCol("wut bookmark"), descCol("Pin your favorite commands")),
			))

		fmt.Println(doneBox)
		fmt.Println()
	} else {
		fmt.Println(lipgloss.NewStyle().Foreground(cGreen).Bold(true).Render("✅ Quick setup complete!"))
		fmt.Println(ui.Accent("wut s git") + " — try it!")
	}

	return nil
}

// OS / Shell helpers

func detectShellForInit() string {
	sh := os.Getenv("SHELL")
	if sh != "" {
		switch {
		case strings.Contains(sh, "bash"):
			return "bash"
		case strings.Contains(sh, "zsh"):
			return "zsh"
		case strings.Contains(sh, "fish"):
			return "fish"
		case strings.Contains(sh, "pwsh"):
			return "pwsh"
		}
	}
	if runtime.GOOS == "windows" {
		if os.Getenv("PSModulePath") != "" {
			return "powershell"
		}
		return "cmd"
	}
	return "bash"
}

func getShellRcFile(shellType string) string {
	home, _ := os.UserHomeDir()
	switch shellType {
	case "bash":
		return home + "/.bashrc"
	case "zsh":
		return home + "/.zshrc"
	case "fish":
		return home + "/.config/fish/config.fish"
	case "powershell", "pwsh":
		return "$PROFILE"
	default:
		return home + "/.bashrc"
	}
}

func boolToEnabled(b bool) string {
	if b {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#2DC653")).Render("enabled")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF70A6")).Render("disabled")
}
