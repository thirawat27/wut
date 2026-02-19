// Package cmd provides CLI commands for WUT
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"wut/internal/logger"
	"wut/internal/shell"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell integration",
	Long: `Install WUT shell integration for enhanced functionality.
This adds aliases, completions, and hooks to your shell configuration.`,
	Example: `  wut install
  wut install --shell bash
  wut install --shell zsh
  wut install --all
  wut install --uninstall`,
	RunE: runInstall,
}

var (
	installShell     string
	installAll       bool
	installUninstall bool
	installForce     bool
)

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().StringVarP(&installShell, "shell", "s", "", "shell to install for (bash, zsh, fish, powershell)")
	installCmd.Flags().BoolVarP(&installAll, "all", "a", false, "install for all shells")
	installCmd.Flags().BoolVarP(&installUninstall, "uninstall", "u", false, "uninstall shell integration")
	installCmd.Flags().BoolVarP(&installForce, "force", "f", false, "force overwrite existing configuration")
}

func runInstall(cmd *cobra.Command, args []string) error {
	log := logger.With("install")

	// Auto-detect shell if not specified
	if installShell == "" && !installAll {
		installShell = detectShell()
		log.Info("auto-detected shell", "shell", installShell)
	}

	if installUninstall {
		return runUninstall(log)
	}

	if installAll {
		return installAllShells(log)
	}

	return installShellIntegration(installShell, log)
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Windows
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return "bash"
	}

	// Extract shell name from path
	return filepath.Base(shell)
}

func installShellIntegration(sh string, log *logger.Logger) error {
	log.Info("installing shell integration", "shell", sh)

	var installer shell.Installer

	switch sh {
	case "bash":
		installer = shell.NewBashInstaller()
	case "zsh":
		installer = shell.NewZshInstaller()
	case "fish":
		installer = shell.NewFishInstaller()
	case "powershell", "pwsh":
		installer = shell.NewPowerShellInstaller()
	default:
		return fmt.Errorf("unsupported shell: %s", sh)
	}

	// Check if already installed
	if installer.IsInstalled() && !installForce {
		return fmt.Errorf("shell integration already installed (use --force to overwrite)")
	}

	// Install
	if err := installer.Install(); err != nil {
		return fmt.Errorf("failed to install: %w", err)
	}

	fmt.Printf("Shell integration installed for %s\n", sh)
	fmt.Println("Please restart your terminal or run:")
	
	switch sh {
	case "bash":
		fmt.Println("  source ~/.bashrc")
	case "zsh":
		fmt.Println("  source ~/.zshrc")
	case "fish":
		fmt.Println("  source ~/.config/fish/config.fish")
	case "powershell":
		fmt.Println("  . $PROFILE")
	}

	return nil
}

func installAllShells(log *logger.Logger) error {
	log.Info("installing for all shells")

	shells := []string{"bash", "zsh", "fish"}
	if runtime.GOOS == "windows" {
		shells = []string{"powershell"}
	}

	var errors []error
	for _, sh := range shells {
		if err := installShellIntegration(sh, log); err != nil {
			log.Warn("failed to install for shell", "shell", sh, "error", err)
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to install for %d shell(s)", len(errors))
	}

	return nil
}

func runUninstall(log *logger.Logger) error {
	log.Info("uninstalling shell integration")

	if installShell == "" && !installAll {
		installShell = detectShell()
	}

	if installAll {
		shells := []string{"bash", "zsh", "fish", "powershell"}
		for _, sh := range shells {
			uninstallShell(sh, log)
		}
		return nil
	}

	return uninstallShell(installShell, log)
}

func uninstallShell(sh string, log *logger.Logger) error {
	log.Info("uninstalling shell integration", "shell", sh)

	var installer shell.Installer

	switch sh {
	case "bash":
		installer = shell.NewBashInstaller()
	case "zsh":
		installer = shell.NewZshInstaller()
	case "fish":
		installer = shell.NewFishInstaller()
	case "powershell", "pwsh":
		installer = shell.NewPowerShellInstaller()
	default:
		return fmt.Errorf("unsupported shell: %s", sh)
	}

	if !installer.IsInstalled() {
		fmt.Printf("Shell integration not installed for %s\n", sh)
		return nil
	}

	if err := installer.Uninstall(); err != nil {
		return fmt.Errorf("failed to uninstall: %w", err)
	}

	fmt.Printf("Shell integration uninstalled for %s\n", sh)
	return nil
}
