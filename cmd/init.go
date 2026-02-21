// Package cmd provides CLI commands for WUT
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/logger"
	"wut/internal/shell"
	"wut/internal/ui"
)

// initCmd initializes WUT for first-time use
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
	initQuick      bool
	initShell      string
	initSkipTLDR   bool
	initSkipShell  bool
	initNonTUI     bool
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&initQuick, "quick", "q", false, "quick setup with defaults (non-interactive)")
	initCmd.Flags().StringVarP(&initShell, "shell", "s", "", "shell type (bash, zsh, fish, powershell)")
	initCmd.Flags().BoolVar(&initSkipTLDR, "skip-tldr", false, "skip TLDR pages setup")
	initCmd.Flags().BoolVar(&initSkipShell, "skip-shell", false, "skip shell integration setup")
	initCmd.Flags().BoolVar(&initNonTUI, "no-tui", false, "use simple text interface (no fancy UI)")
}

func runInit(cmd *cobra.Command, args []string) error {
	log := logger.With("init")
	log.Info("starting initialization wizard")

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	if !initQuick {
		fmt.Println()
		fmt.Println(titleStyle.Render("⚡ WUT Initialization Wizard"))
		fmt.Println(subtitleStyle.Render("Let's set up WUT for your system"))
		fmt.Println()
	}

	// Step 1: Ensure directories
	if !initQuick {
		fmt.Println("📁 Creating directories...")
	}
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	if !initQuick {
		fmt.Println("   ✓ Directories created")
		fmt.Println()
	}

	// Step 2: Load or create config
	if !initQuick {
		fmt.Println("⚙️  Setting up configuration...")
	}
	cfg := config.Get()

	// Quick mode: use defaults
	if initQuick {
		cfg.UI.Theme = "auto"
		cfg.Fuzzy.Enabled = true
		cfg.History.Enabled = true
		cfg.Context.Enabled = true
		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	} else {
		// Interactive mode
		reader := bufio.NewReader(os.Stdin)

		// Theme preference
		fmt.Println(subtitleStyle.Render("Choose your preferred theme:"))
		fmt.Println("  1. Auto-detect (recommended)")
		fmt.Println("  2. Dark mode")
		fmt.Println("  3. Light mode")
		fmt.Print("\nSelection [1]: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		if choice == "" {
			choice = "1"
		}
		switch choice {
		case "2":
			cfg.UI.Theme = "dark"
		case "3":
			cfg.UI.Theme = "light"
		default:
			cfg.UI.Theme = "auto"
		}
		fmt.Printf("   ✓ Theme set to: %s\n\n", cfg.UI.Theme)

		// History tracking
		fmt.Print(subtitleStyle.Render("Enable command history tracking? [Y/n]: "))
		choice, _ = reader.ReadString('\n')
		choice = strings.ToLower(strings.TrimSpace(choice))
		cfg.History.Enabled = choice == "" || choice == "y" || choice == "yes"
		fmt.Printf("   ✓ History tracking: %s\n\n", boolToEnabled(cfg.History.Enabled))

		// Context analysis
		fmt.Print(subtitleStyle.Render("Enable context analysis (detects project types)? [Y/n]: "))
		choice, _ = reader.ReadString('\n')
		choice = strings.ToLower(strings.TrimSpace(choice))
		cfg.Context.Enabled = choice == "" || choice == "y" || choice == "yes"
		fmt.Printf("   ✓ Context analysis: %s\n\n", boolToEnabled(cfg.Context.Enabled))

		// Save config
		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Step 3: Shell integration
	if !initSkipShell {
		if !initQuick {
			fmt.Println("🐚 Shell Integration")
		}

		shellType := initShell
		if shellType == "" {
			shellType = detectShellForInit()
		}

		if !initQuick {
			fmt.Printf("   Detected shell: %s\n", ui.Cyan(shellType))
			fmt.Println()

			fmt.Print(subtitleStyle.Render("Would you like to set up shell integration? [Y/n]: "))
			reader := bufio.NewReader(os.Stdin)
			choice, _ := reader.ReadString('\n')
			choice = strings.ToLower(strings.TrimSpace(choice))
			if choice == "" || choice == "y" || choice == "yes" {
				if err := setupShellIntegration(shellType); err != nil {
					fmt.Printf("   ⚠️  Shell integration setup failed: %v\n", err)
				} else {
					fmt.Println("   ✓ Shell integration configured")
					fmt.Println("      Please restart your shell or run: source " + getShellRcFile(shellType))
				}
			}
		} else {
			// Quick mode: just print instructions
			fmt.Printf("Detected shell: %s\n", ui.Cyan(shellType))
			fmt.Println("To enable shell integration, run:")
			fmt.Printf("  wut install --shell %s\n", shellType)
		}
		fmt.Println()
	}

	// Step 4: TLDR setup
	if !initSkipTLDR {
		if !initQuick {
			fmt.Println("📚 TLDR Pages Setup")
			fmt.Println(subtitleStyle.Render("TLDR pages provide quick command references"))
			fmt.Println()

			fmt.Print("Would you like to download TLDR pages? [Y/n]: ")
			reader := bufio.NewReader(os.Stdin)
			choice, _ := reader.ReadString('\n')
			choice = strings.ToLower(strings.TrimSpace(choice))

			if choice == "" || choice == "y" || choice == "yes" {
				fmt.Println("   Downloading popular TLDR pages...")
					// Run db sync
				dbCmd.SetArgs([]string{"sync"})
				if err := dbCmd.Execute(); err != nil {
					fmt.Printf("   ⚠️  TLDR sync failed: %v\n", err)
				} else {
					fmt.Println("   ✓ TLDR pages downloaded")
				}
			}
		} else {
			fmt.Println("To download TLDR pages later, run:")
			fmt.Println("  wut tldr sync")
		}
		fmt.Println()
	}

	// Final summary
	if !initQuick {
		fmt.Println()
		fmt.Println(titleStyle.Render("✅ Setup Complete!"))
		fmt.Println()
		fmt.Println("Quick start:")
		fmt.Println("  wut s <command>     # Get command help")
		fmt.Println("  wut h               # View history")
		fmt.Println("  wut ? <query>       # Smart suggestions")
		fmt.Println("  wut config          # Edit configuration")
		fmt.Println()
		fmt.Println("For more help: wut --help")
	} else {
		fmt.Println("✅ Quick setup complete!")
		fmt.Println()
		fmt.Println("Try: wut s git")
	}

	return nil
}

func detectShellForInit() string {
	// Check SHELL environment variable
	shell := os.Getenv("SHELL")
	if shell != "" {
		switch {
		case strings.Contains(shell, "bash"):
			return "bash"
		case strings.Contains(shell, "zsh"):
			return "zsh"
		case strings.Contains(shell, "fish"):
			return "fish"
		}
	}

	// Check platform
	if runtime.GOOS == "windows" {
		// Check for PowerShell
		if os.Getenv("PSModulePath") != "" {
			return "powershell"
		}
		return "cmd"
	}

	// Default
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
	case "powershell":
		return "$PROFILE"
	default:
		return home + "/.bashrc"
	}
}

func setupShellIntegration(shellType string) error {
	installer := shell.NewInstaller()
	return installer.Install(shellType)
}

func boolToEnabled(b bool) string {
	if b {
		return ui.Green("enabled")
	}
	return ui.Red("disabled")
}
