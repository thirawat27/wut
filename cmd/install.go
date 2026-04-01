// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wut/internal/shell"

	"github.com/spf13/cobra"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell integration",
	Long: `Install WUT shell integration with key bindings.

This command sets up key bindings for your shell to quickly access WUT:
- Ctrl+Space: Open WUT TUI
- Ctrl+G: Open WUT with current command line

Supports live integration for: bash, zsh, fish, powershell, pwsh, nushell, xonsh, elvish, cmd`,
	Example: `  wut install           # Install for all detected shells (default)
  wut install --all     # Install for all detected shells
  wut install --uninstall # Remove shell integration`,
	RunE: runInstall,
}

var (
	installAll       bool
	installUninstall bool
	installShell     string
)

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().BoolVarP(&installAll, "all", "a", false, "install for all detected shells")
	installCmd.Flags().BoolVarP(&installUninstall, "uninstall", "u", false, "uninstall shell integration")
	installCmd.Flags().StringVarP(&installShell, "shell", "s", "", "target shell")
}

func runInstall(cmd *cobra.Command, args []string) error {
	if installUninstall {
		return runUninstall()
	}

	if installShell == "" && !installAll {
		installAll = true
	}

	if installAll {
		return installAllShells()
	}

	if err := installShellIntegration(installShell); err != nil {
		return err
	}
	return runPostInstallHistoryImport()
}

func runUninstall() error {
	if installShell == "" && !installAll {
		installShell = detectShell()
		if installShell == "" {
			return fmt.Errorf("could not detect shell, please specify with --shell")
		}
	}

	if installAll {
		return uninstallAllShells()
	}

	return uninstallShellIntegration(installShell)
}

func installShellIntegration(sh string) error {
	installer := shell.NewInstaller()
	sh = normalizeInstallShell(sh)
	if !shell.SupportsInstall(sh) {
		return fmt.Errorf("live integration is not implemented for %s yet; installable shells: %s", sh, strings.Join(shell.IntegrationShells(), ", "))
	}

	fmt.Printf("Installing WUT integration for %s...\n", sh)
	if err := installer.Install(sh); err != nil {
		if err.Error() == "already installed" {
			fmt.Println("✅ WUT integration is already installed")
			return nil
		}
		return err
	}

	fmt.Println("✅ Successfully installed!")
	fmt.Println()
	fmt.Println("Key bindings:")
	fmt.Println("  • Ctrl+Space - Open WUT TUI")
	fmt.Println("  • Ctrl+G     - Open WUT with current command")
	fmt.Println()
	if configFile, err := shell.GetConfigFile(sh); err == nil {
		if reloadCmd := shell.GetReloadCommand(sh, configFile); reloadCmd != "" {
			fmt.Printf("Please restart your shell or run: %s\n", reloadCmd)
			return nil
		}
	}
	fmt.Println("Please restart your shell to load the integration.")
	return nil
}

func uninstallShellIntegration(sh string) error {
	sh = normalizeInstallShell(sh)
	installer := shell.NewInstaller()

	fmt.Printf("Removing WUT integration from %s...\n", sh)
	if err := installer.Uninstall(sh); err != nil {
		return err
	}

	fmt.Println("✅ Successfully uninstalled!")
	if configFile, err := shell.GetConfigFile(sh); err == nil {
		if reloadCmd := shell.GetReloadCommand(sh, configFile); reloadCmd != "" {
			fmt.Printf("Please restart your shell or run: %s\n", reloadCmd)
			return nil
		}
	}
	fmt.Println("Please restart your shell to unload the integration.")
	return nil
}

func installAllShells() error {
	shells := detectAllShells()
	if len(shells) == 0 {
		return fmt.Errorf("no shells detected")
	}

	for _, sh := range shells {
		if err := installShellIntegration(sh); err != nil {
			fmt.Printf("⚠️  Failed to install for %s: %v\n", sh, err)
		}
		fmt.Println()
	}

	return runPostInstallHistoryImport()
}

func uninstallAllShells() error {
	shells := detectAllShells()
	for _, sh := range shells {
		if err := uninstallShellIntegration(sh); err != nil {
			fmt.Printf("⚠️  Failed to uninstall for %s: %v\n", sh, err)
		}
	}

	return nil
}

func detectShell() string {
	return normalizeInstallShell(shell.DetectPreferredInstallShell())
}

func detectAllShells() []string {
	return shell.DetectInstallableShells()
}

func normalizeInstallShell(sh string) string {
	return shell.CanonicalName(sh)
}

func runPostInstallHistoryImport() error {
	importCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	summary, err := bootstrapShellHistoryImport(importCtx)
	if err != nil {
		fmt.Printf("⚠️  Shell history import skipped: %v\n", err)
		return nil
	}

	switch {
	case summary.imported > 0:
		fmt.Printf("✅ Imported %d history entries from %d shell sources\n", summary.imported, len(summary.sources))
	case len(summary.sources) > 0:
		fmt.Printf("✓ Scanned %d shell history sources; no new commands to import\n", len(summary.sources))
	default:
		fmt.Println("✓ No shell history sources detected")
	}

	return nil
}
