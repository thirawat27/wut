// Package shell provides shell integration functionality for WUT
package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ShellType represents supported shell types
type ShellType string

const (
	Bash       ShellType = "bash"
	Zsh        ShellType = "zsh"
	Fish       ShellType = "fish"
	PowerShell ShellType = "powershell"
	NuShell    ShellType = "nushell"
	Elvish     ShellType = "elvish"
	Xonsh      ShellType = "xonsh"
	Tcsh       ShellType = "tcsh"
	Ksh        ShellType = "ksh"
	Cmd        ShellType = "cmd"
)

// Installer represents a shell installer
type Installer interface {
	Install() error
	Uninstall() error
	IsInstalled() bool
	GetShellType() ShellType
}

// BaseInstaller provides common functionality
type BaseInstaller struct {
	ShellType      ShellType
	ConfigFile     string
	IntegrationStr string
}

// GetShellType returns the shell type
func (b *BaseInstaller) GetShellType() ShellType {
	return b.ShellType
}

// DetectShell detects the current shell
func DetectShell() ShellType {
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Windows
		if runtime.GOOS == "windows" {
			// Check PowerShell
			if os.Getenv("PSModulePath") != "" || os.Getenv("PSVersionTable") != "" {
				return PowerShell
			}
			// Check cmd
			if os.Getenv("COMSPEC") != "" {
				return Cmd
			}
			return PowerShell
		}
		return Bash
	}
	
	shellName := filepath.Base(shell)
	
	switch shellName {
	case "bash":
		return Bash
	case "zsh":
		return Zsh
	case "fish":
		return Fish
	case "pwsh", "powershell":
		return PowerShell
	case "nu":
		return NuShell
	case "elvish":
		return Elvish
	case "xonsh":
		return Xonsh
	case "tcsh", "csh":
		return Tcsh
	case "ksh", "mksh", "oksh":
		return Ksh
	case "cmd", "command":
		return Cmd
	default:
		return Bash
	}
}

// NewInstaller creates an installer for the specified shell
func NewInstaller(shellType ShellType) Installer {
	switch shellType {
	case Bash:
		return NewBashInstaller()
	case Zsh:
		return NewZshInstaller()
	case Fish:
		return NewFishInstaller()
	case PowerShell:
		return NewPowerShellInstaller()
	case NuShell:
		return NewNuShellInstaller()
	case Elvish:
		return NewElvishInstaller()
	case Xonsh:
		return NewXonshInstaller()
	case Tcsh:
		return NewTcshInstaller()
	case Ksh:
		return NewKshInstaller()
	default:
		return NewBashInstaller()
	}
}

// InstallForAll installs shell integration for all detected shells
func InstallForAll() error {
	installed := []ShellType{}
	errors := []error{}
	
	// Try to install for all shells
	for _, shell := range []ShellType{Bash, Zsh, Fish, PowerShell, NuShell, Elvish, Xonsh, Tcsh, Ksh} {
		installer := NewInstaller(shell)
		if installer.IsInstalled() {
			continue
		}
		
		if err := installer.Install(); err == nil {
			installed = append(installed, shell)
		} else {
			errors = append(errors, fmt.Errorf("%s: %w", shell, err))
		}
	}
	
	if len(installed) == 0 && len(errors) > 0 {
		return fmt.Errorf("failed to install for any shell: %v", errors)
	}
	
	return nil
}

// UninstallForAll uninstalls shell integration for all shells
func UninstallForAll() error {
	errors := []error{}
	
	for _, shell := range []ShellType{Bash, Zsh, Fish, PowerShell, NuShell, Elvish, Xonsh, Tcsh, Ksh} {
		installer := NewInstaller(shell)
		if err := installer.Uninstall(); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", shell, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("some uninstalls failed: %v", errors)
	}
	
	return nil
}

// GetAllInstalled returns all shells that have WUT installed
func GetAllInstalled() []ShellType {
	installed := []ShellType{}
	
	for _, shell := range []ShellType{Bash, Zsh, Fish, PowerShell, NuShell, Elvish, Xonsh, Tcsh, Ksh} {
		installer := NewInstaller(shell)
		if installer.IsInstalled() {
			installed = append(installed, shell)
		}
	}
	
	return installed
}

// bashInstaller installs for Bash
type bashInstaller struct {
	BaseInstaller
}

// NewBashInstaller creates a new Bash installer
func NewBashInstaller() Installer {
	return &bashInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Bash,
			ConfigFile:     filepath.Join(getHomeDir(), ".bashrc"),
			IntegrationStr: getBashIntegration(),
		},
	}
}

func (b *bashInstaller) Install() error {
	return appendToFile(b.ConfigFile, b.IntegrationStr)
}

func (b *bashInstaller) Uninstall() error {
	return removeFromFile(b.ConfigFile, b.IntegrationStr)
}

func (b *bashInstaller) IsInstalled() bool {
	return fileContains(b.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// zshInstaller installs for Zsh
type zshInstaller struct {
	BaseInstaller
}

// NewZshInstaller creates a new Zsh installer
func NewZshInstaller() Installer {
	return &zshInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Zsh,
			ConfigFile:     filepath.Join(getHomeDir(), ".zshrc"),
			IntegrationStr: getZshIntegration(),
		},
	}
}

func (z *zshInstaller) Install() error {
	return appendToFile(z.ConfigFile, z.IntegrationStr)
}

func (z *zshInstaller) Uninstall() error {
	return removeFromFile(z.ConfigFile, z.IntegrationStr)
}

func (z *zshInstaller) IsInstalled() bool {
	return fileContains(z.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// fishInstaller installs for Fish
type fishInstaller struct {
	BaseInstaller
}

// NewFishInstaller creates a new Fish installer
func NewFishInstaller() Installer {
	return &fishInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Fish,
			ConfigFile:     filepath.Join(getHomeDir(), ".config", "fish", "config.fish"),
			IntegrationStr: getFishIntegration(),
		},
	}
}

func (f *fishInstaller) Install() error {
	// Ensure directory exists
	dir := filepath.Dir(f.ConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendToFile(f.ConfigFile, f.IntegrationStr)
}

func (f *fishInstaller) Uninstall() error {
	return removeFromFile(f.ConfigFile, f.IntegrationStr)
}

func (f *fishInstaller) IsInstalled() bool {
	return fileContains(f.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// powerShellInstaller installs for PowerShell
type powerShellInstaller struct {
	BaseInstaller
}

// NewPowerShellInstaller creates a new PowerShell installer
func NewPowerShellInstaller() Installer {
	configFile := filepath.Join(getHomeDir(), "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if runtime.GOOS != "windows" {
		configFile = filepath.Join(getHomeDir(), ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
	}
	
	return &powerShellInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      PowerShell,
			ConfigFile:     configFile,
			IntegrationStr: getPowerShellIntegration(),
		},
	}
}

func (p *powerShellInstaller) Install() error {
	dir := filepath.Dir(p.ConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendToFile(p.ConfigFile, p.IntegrationStr)
}

func (p *powerShellInstaller) Uninstall() error {
	return removeFromFile(p.ConfigFile, p.IntegrationStr)
}

func (p *powerShellInstaller) IsInstalled() bool {
	return fileContains(p.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// nuShellInstaller installs for Nushell
type nuShellInstaller struct {
	BaseInstaller
}

// NewNuShellInstaller creates a new Nushell installer
func NewNuShellInstaller() Installer {
	return &nuShellInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      NuShell,
			ConfigFile:     filepath.Join(getHomeDir(), ".config", "nushell", "config.nu"),
			IntegrationStr: getNuShellIntegration(),
		},
	}
}

func (n *nuShellInstaller) Install() error {
	dir := filepath.Dir(n.ConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendToFile(n.ConfigFile, n.IntegrationStr)
}

func (n *nuShellInstaller) Uninstall() error {
	return removeFromFile(n.ConfigFile, n.IntegrationStr)
}

func (n *nuShellInstaller) IsInstalled() bool {
	return fileContains(n.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// elvishInstaller installs for Elvish
type elvishInstaller struct {
	BaseInstaller
}

// NewElvishInstaller creates a new Elvish installer
func NewElvishInstaller() Installer {
	return &elvishInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Elvish,
			ConfigFile:     filepath.Join(getHomeDir(), ".elvish", "rc.elv"),
			IntegrationStr: getElvishIntegration(),
		},
	}
}

func (e *elvishInstaller) Install() error {
	dir := filepath.Dir(e.ConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendToFile(e.ConfigFile, e.IntegrationStr)
}

func (e *elvishInstaller) Uninstall() error {
	return removeFromFile(e.ConfigFile, e.IntegrationStr)
}

func (e *elvishInstaller) IsInstalled() bool {
	return fileContains(e.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// xonshInstaller installs for Xonsh
type xonshInstaller struct {
	BaseInstaller
}

// NewXonshInstaller creates a new Xonsh installer
func NewXonshInstaller() Installer {
	return &xonshInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Xonsh,
			ConfigFile:     filepath.Join(getHomeDir(), ".xonshrc"),
			IntegrationStr: getXonshIntegration(),
		},
	}
}

func (x *xonshInstaller) Install() error {
	return appendToFile(x.ConfigFile, x.IntegrationStr)
}

func (x *xonshInstaller) Uninstall() error {
	return removeFromFile(x.ConfigFile, x.IntegrationStr)
}

func (x *xonshInstaller) IsInstalled() bool {
	return fileContains(x.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// tcshInstaller installs for Tcsh
type tcshInstaller struct {
	BaseInstaller
}

// NewTcshInstaller creates a new Tcsh installer
func NewTcshInstaller() Installer {
	return &tcshInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Tcsh,
			ConfigFile:     filepath.Join(getHomeDir(), ".tcshrc"),
			IntegrationStr: getTcshIntegration(),
		},
	}
}

func (t *tcshInstaller) Install() error {
	return appendToFile(t.ConfigFile, t.IntegrationStr)
}

func (t *tcshInstaller) Uninstall() error {
	return removeFromFile(t.ConfigFile, t.IntegrationStr)
}

func (t *tcshInstaller) IsInstalled() bool {
	return fileContains(t.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// kshInstaller installs for Ksh
type kshInstaller struct {
	BaseInstaller
}

// NewKshInstaller creates a new Ksh installer
func NewKshInstaller() Installer {
	return &kshInstaller{
		BaseInstaller: BaseInstaller{
			ShellType:      Ksh,
			ConfigFile:     filepath.Join(getHomeDir(), ".kshrc"),
			IntegrationStr: getKshIntegration(),
		},
	}
}

func (k *kshInstaller) Install() error {
	return appendToFile(k.ConfigFile, k.IntegrationStr)
}

func (k *kshInstaller) Uninstall() error {
	return removeFromFile(k.ConfigFile, k.IntegrationStr)
}

func (k *kshInstaller) IsInstalled() bool {
	return fileContains(k.ConfigFile, "# WUT - AI-Powered Command Helper")
}

// Helper functions

func getHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

func appendToFile(filename, content string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	
	// Check if already contains content
	data, _ := os.ReadFile(filename)
	if strings.Contains(string(data), "# WUT - AI-Powered Command Helper") {
		return nil // Already installed
	}
	
	_, err = f.WriteString("\n" + content + "\n")
	return err
}

func removeFromFile(filename, marker string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	lines := strings.Split(string(data), "\n")
	var filtered []string
	skip := false
	
	for _, line := range lines {
		if strings.Contains(line, "# WUT - AI-Powered Command Helper") {
			skip = true
			continue
		}
		if skip && strings.TrimSpace(line) == "" {
			skip = false
			continue
		}
		if !skip {
			filtered = append(filtered, line)
		}
	}
	
	return os.WriteFile(filename, []byte(strings.Join(filtered, "\n")), 0644)
}

func fileContains(filename, substr string) bool {
	data, err := os.ReadFile(filename)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
}

// Integration strings

func getBashIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Bash

# Aliases
alias w='wut suggest'
alias wh='wut history'
alias we='wut explain'
alias wt='wut train'

# Completion
if command -v wut &> /dev/null; then
    eval "$(wut completion bash)"
fi

# Key binding (Ctrl+Space for quick suggest)
if [[ $- == *i* ]]; then
    bind -x '"\C-@": "wut suggest"' 2>/dev/null || true
fi
`
}

func getZshIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Zsh

# Aliases
alias w='wut suggest'
alias wh='wut history'
alias we='wut explain'
alias wt='wut train'

# Completion
if command -v wut &> /dev/null; then
    eval "$(wut completion zsh)"
fi

# Key binding (Ctrl+Space for quick suggest)
if [[ -o interactive ]]; then
    bindkey '^@' wut-suggest 2>/dev/null || true
fi
`
}

func getFishIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Fish

# Aliases
alias w 'wut suggest'
alias wh 'wut history'
alias we 'wut explain'
alias wt 'wut train'

# Completion
if command -v wut &> /dev/null
    wut completion fish | source
end

# Key binding (Ctrl+Space for quick suggest)
bind \c@ 'wut suggest' 2>/dev/null
`
}

func getPowerShellIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for PowerShell

# Aliases
Set-Alias -Name w -Value 'wut suggest'
Set-Alias -Name wh -Value 'wut history'
Set-Alias -Name we -Value 'wut explain'
Set-Alias -Name wt -Value 'wut train'

# Completion
if (Get-Command wut -ErrorAction SilentlyContinue) {
    wut completion powershell | Out-String | Invoke-Expression
}

# Key binding (Ctrl+Space for quick suggest)
# Note: Requires PSReadLine
if (Get-Module PSReadLine -ErrorAction SilentlyContinue) {
    Set-PSReadLineKeyHandler -Chord Ctrl+Space -ScriptBlock {
        [Microsoft.PowerShell.PSConsoleReadLine]::RevertLine()
        [Microsoft.PowerShell.PSConsoleReadLine]::Insert('wut suggest ')
        [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
    }
}
`
}

func getNuShellIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Nushell

# Aliases
alias w = wut suggest
alias wh = wut history
alias we = wut explain
alias wt = wut train

# Completion
# Note: Nushell has its own completion system
# Add to your config.nu: use wut-completions.nu *
`
}

func getElvishIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Elvish

# Aliases
fn w { wut suggest $@ }
fn wh { wut history $@ }
fn we { wut explain $@ }
fn wt { wut train $@ }

# Completion
# Note: Add completions manually or use:
# eval (wut completion elvish | slurp)
`
}

func getXonshIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Xonsh

# Aliases
aliases['w'] = 'wut suggest'
aliases['wh'] = 'wut history'
aliases['we'] = 'wut explain'
aliases['wt'] = 'wut train'

# Completion
# Note: Xonsh uses Python-based completions
import subprocess
$COMPLETIONS['wut'] = lambda args, prefix: subprocess.check_output(['wut', 'completion', 'xonsh']).decode().split()
`
}

func getTcshIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Tcsh

# Aliases
alias w 'wut suggest'
alias wh 'wut history'
alias we 'wut explain'
alias wt 'wut train'

# Completion (limited support in tcsh)
complete wut 'p/*/"wut completion tcsh"/'`
}

func getKshIntegration() string {
	return `# WUT - AI-Powered Command Helper
# Integration for Ksh

# Aliases
alias w='wut suggest'
alias wh='wut history'
alias we='wut explain'
alias wt='wut train'

# Completion (if supported)
if whence wut >/dev/null 2>&1; then
    eval "$(wut completion ksh)"
fi
`
}
