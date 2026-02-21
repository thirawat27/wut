// Package shell provides shell integration for WUT
package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Installer manages shell integration
type Installer struct {
	shells []string
}

// NewInstaller creates a new shell installer
func NewInstaller() *Installer {
	return &Installer{
		shells: detectShells(),
	}
}

// Install installs WUT integration for the given shell
func (i *Installer) Install(shell string) error {
	configFile, err := GetConfigFile(shell)
	if err != nil {
		return err
	}

	// Check if already installed
	if IsInstalled(configFile) {
		return fmt.Errorf("already installed")
	}

	// Generate shell code
	shellCode := GenerateShellCode(shell)

	// Append to config file
	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open shell config: %w", err)
	}
	defer f.Close()

	marker := fmt.Sprintf("\n# WUT Shell Integration\n%s\n# End WUT Integration\n", shellCode)
	
	if _, err := f.WriteString(marker); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}

	return nil
}

// Uninstall removes WUT integration from the given shell
func (i *Installer) Uninstall(shell string) error {
	configFile, err := GetConfigFile(shell)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read shell config: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inWUTSection := false

	for _, line := range lines {
		if strings.Contains(line, "# WUT Shell Integration") {
			inWUTSection = true
			continue
		}
		if strings.Contains(line, "# End WUT Integration") {
			inWUTSection = false
			continue
		}
		if !inWUTSection {
			newLines = append(newLines, line)
		}
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(configFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}

	return nil
}

// GetConfigFile returns the config file path for the given shell
func GetConfigFile(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch shell {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	case "powershell", "pwsh":
		if runtime.GOOS == "windows" {
			return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
		}
		return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

// IsInstalled checks if WUT is already installed in the config file
func IsInstalled(configFile string) bool {
	content, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "# WUT Shell Integration")
}

// GenerateShellCode generates shell-specific code for WUT integration
func GenerateShellCode(shell string) string {
	switch shell {
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
	return `# WUT Key Bindings - Quick Access
__wut_tui() {
    wut suggest
}

__wut_with_current() {
    local cmd="${READLINE_LINE}"
    READLINE_LINE=""
    READLINE_POINT=0
    wut suggest "$cmd"
}

# Ctrl+Space - Open WUT TUI
bind '"\C-@":"\C-uwut suggest\C-m"' 2>/dev/null || true

# Ctrl+G - Open WUT with current command
bind '"\C-g":"\C-awut suggest \"\C-e\"\C-m"' 2>/dev/null || true
`
}

func generateFishCode() string {
	return `# WUT Key Bindings - Quick Access
function __wut_tui
    wut suggest
    commandline -f repaint
end

function __wut_with_current
    set -l cmd (commandline)
    wut suggest $cmd
    commandline -f repaint
end

# Ctrl+Space - Open WUT TUI
bind \c@ __wut_tui 2>/dev/null; or true

# Ctrl+G - Open WUT with current command
bind \cg __wut_with_current 2>/dev/null; or true
`
}

func generatePowerShellCode() string {
	// Note: In PowerShell, backtick is the escape character
	code := `# WUT Key Bindings - Quick Access
function Invoke-WUT-TUI {
    wut suggest
}

function Invoke-WUT-WithCurrent {
    $line = $null
    $cursor = $null
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
    [Microsoft.PowerShell.PSConsoleReadLine]::RevertLine()
    $cmdLine = 'wut suggest "' + $line + '"'
    [Microsoft.PowerShell.PSConsoleReadLine]::Insert($cmdLine)
    [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
}

# Set up key handlers
Set-PSReadLineKeyHandler -Chord 'Ctrl+SpaceBar' -ScriptBlock { Invoke-WUT-TUI } -ErrorAction SilentlyContinue
Set-PSReadLineKeyHandler -Chord 'Ctrl+g' -ScriptBlock { Invoke-WUT-WithCurrent } -ErrorAction SilentlyContinue
`
	return code
}

// detectShells detects available shells
func detectShells() []string {
	var shells []string
	
	candidates := []string{"bash", "zsh", "fish"}
	if runtime.GOOS == "windows" {
		candidates = []string{"powershell", "pwsh", "cmd"}
	}

	for _, sh := range candidates {
		if _, err := exec.LookPath(sh); err == nil {
			shells = append(shells, sh)
		}
	}

	return shells
}

// GetDetectedShells returns detected shells
func (i *Installer) GetDetectedShells() []string {
	return i.shells
}

// DetectCurrentShell detects the current shell
func DetectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return ""
	}
	return filepath.Base(shell)
}
