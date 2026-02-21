// Package cmd provides CLI commands for WUT
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"wut/internal/shell"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell integration",
	Long: `Install WUT shell integration with key bindings.

This command sets up key bindings for your shell to quickly access WUT:
- Ctrl+Space: Open WUT TUI
- Ctrl+G: Open WUT with current command line

Supports: bash, zsh, fish, powershell`,
	Example: `  wut install           # Install for current shell
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
	installCmd.Flags().StringVarP(&installShell, "shell", "s", "", "target shell (bash, zsh, fish, powershell)")
}

func runInstall(cmd *cobra.Command, args []string) error {
	if installUninstall {
		return runUninstall()
	}

	// Detect current shell if not specified
	if installShell == "" && !installAll {
		installShell = detectShell()
		if installShell == "" {
			return fmt.Errorf("could not detect shell, please specify with --shell")
		}
	}

	if installAll {
		return installAllShells()
	}

	return installShellIntegration(installShell)
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
	
	fmt.Printf("Installing WUT integration for %s...\n", sh)
	
	configFile, err := getShellConfigFile(sh)
	if err != nil {
		return err
	}

	// Check if already installed
	installed, err := isAlreadyInstalled(configFile)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("✅ WUT integration is already installed")
		return nil
	}

	// Generate shell code
	shellCode := generateShellCode(sh)

	// Append to config file
	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open shell config: %w", err)
	}
	defer f.Close()

	// Add marker comment
	marker := fmt.Sprintf("\n# WUT Shell Integration - Added by 'wut install'\n%s\n# End WUT Shell Integration\n", shellCode)
	
	if _, err := f.WriteString(marker); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}

	fmt.Println("✅ Successfully installed!")
	fmt.Println()
	fmt.Println("Key bindings:")
	fmt.Println("  • Ctrl+Space - Open WUT TUI")
	fmt.Println("  • Ctrl+G     - Open WUT with current command")
	fmt.Println()
	fmt.Printf("Please restart your shell or run: source %s\n", configFile)
	
	_ = installer // Use the installer
	return nil
}

func uninstallShellIntegration(sh string) error {
	configFile, err := getShellConfigFile(sh)
	if err != nil {
		return err
	}

	fmt.Printf("Removing WUT integration from %s...\n", sh)

	// Read config file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read shell config: %w", err)
	}

	// Remove WUT section
	lines := strings.Split(string(content), "\n")
	var newLines []string
	inWUTSection := false

	for _, line := range lines {
		if strings.Contains(line, "# WUT Shell Integration") {
			inWUTSection = true
			continue
		}
		if strings.Contains(line, "# End WUT Shell Integration") {
			inWUTSection = false
			continue
		}
		if !inWUTSection {
			newLines = append(newLines, line)
		}
	}

	// Write back
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(configFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}

	fmt.Println("✅ Successfully uninstalled!")
	fmt.Printf("Please restart your shell or run: source %s\n", configFile)

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

	return nil
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
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Windows
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return ""
	}

	// Extract shell name from path
	return filepath.Base(shell)
}

func detectAllShells() []string {
	var shells []string
	
	// Check for common shells
	candidates := []string{"bash", "zsh", "fish"}
	if runtime.GOOS == "windows" {
		candidates = []string{"powershell"}
	}

	for _, sh := range candidates {
		if _, err := exec.LookPath(sh); err == nil {
			shells = append(shells, sh)
		}
	}

	return shells
}

func getShellConfigFile(sh string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch sh {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	case "powershell", "pwsh":
		if runtime.GOOS == "windows" {
			// Try PowerShell Core first, then Windows PowerShell
			psCorePath := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
			psWinPath := filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
			
			// Check which one exists, prefer PowerShell Core
			if _, err := os.Stat(psCorePath); err == nil {
				return psCorePath, nil
			}
			// Check Windows PowerShell
			if _, err := os.Stat(psWinPath); err == nil {
				return psWinPath, nil
			}
			// Default to PowerShell Core path (newer)
			return psCorePath, nil
		}
		// Linux/macOS PowerShell
		return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", sh)
	}
}

func isAlreadyInstalled(configFile string) (bool, error) {
	content, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, so not installed
			return false, nil
		}
		// Other error (permission denied, etc.)
		return false, fmt.Errorf("cannot read shell config: %w", err)
	}
	return strings.Contains(string(content), "# WUT Shell Integration"), nil
}

func generateShellCode(sh string) string {
	switch sh {
	case "bash", "zsh":
		return generateBashZshCode()
	case "fish":
		return generateFishCode()
	case "powershell", "pwsh":
		return generatePowerShellCode()
	default:
		return ""
	}
}

func generateBashZshCode() string {
	return `# WUT Key Bindings
__wut_open_tui() {
    wut suggest
}

__wut_open_with_current() {
    local cmd="${READLINE_LINE}"
    wut suggest "$cmd"
}

# Bind Ctrl+Space to open WUT TUI
bind '"\C-@":"\C-uwut suggest\C-m"' 2>/dev/null || true

# Bind Ctrl+G to open WUT with current command
bind '"\C-g":"\C-awut suggest \"\C-e\"\C-m"' 2>/dev/null || true
`
}

func generateFishCode() string {
	return `# WUT Key Bindings
function __wut_open_tui
    wut suggest
    commandline -f repaint
end

function __wut_open_with_current
    set -l cmd (commandline)
    wut suggest $cmd
    commandline -f repaint
end

# Bind Ctrl+Space to open WUT TUI
bind \c@ __wut_open_tui 2>/dev/null; or true

# Bind Ctrl+G to open WUT with current command
bind \cg __wut_open_with_current 2>/dev/null; or true
`
}

func generatePowerShellCode() string {
	code := `# WUT Key Bindings
$null = Register-EngineEvent -SourceIdentifier PowerShell.OnIdle -Action {
    if (-not $global:WUTKeyHandlersAdded) {
        # Ctrl+Space - Open WUT TUI
        Set-PSReadLineKeyHandler -Chord 'Ctrl+SpaceBar' -ScriptBlock {
            [Microsoft.PowerShell.PSConsoleReadLine]::RevertLine()
            [Microsoft.PowerShell.PSConsoleReadLine]::Insert('wut suggest')
            [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
        } -ErrorAction SilentlyContinue

        # Ctrl+G - Open WUT with current command
        Set-PSReadLineKeyHandler -Chord 'Ctrl+g' -ScriptBlock {
            $line = $null
            $cursor = $null
            [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
            [Microsoft.PowerShell.PSConsoleReadLine]::RevertLine()
            $cmdLine = 'wut suggest "' + $line + '"'
            [Microsoft.PowerShell.PSConsoleReadLine]::Insert($cmdLine)
            [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
        } -ErrorAction SilentlyContinue

        $global:WUTKeyHandlersAdded = $true
    }
}
`
	return code
}
