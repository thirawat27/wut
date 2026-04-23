package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"wut/internal/config"
)

const (
	integrationStartMarker = "# WUT Shell Integration"
	integrationEndMarker   = "# End WUT Integration"
	legacyIntegrationEnd   = "# End WUT Shell Integration"
	cmdAutoRunKey          = `HKCU\Software\Microsoft\Command Processor`
	cmdAutoRunValue        = "AutoRun"
)

type Installer struct {
	shells []string
}

func NewInstaller() *Installer {
	return &Installer{
		shells: DetectInstallableShells(),
	}
}

func (i *Installer) Install(shellName string) error {
	shellName = CanonicalName(shellName)
	if shellName == "" {
		return fmt.Errorf("unsupported shell")
	}
	if !SupportsInstall(shellName) {
		return fmt.Errorf("unsupported shell for installation: %s", shellName)
	}

	if shellName == "cmd" {
		return installCmdIntegration()
	}

	configFile, err := GetConfigFile(shellName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create shell config directory: %w", err)
	}
	if IsInstalled(configFile) {
		return fmt.Errorf("already installed")
	}

	shellCode := strings.TrimSpace(GenerateShellCode(shellName))
	if shellCode == "" {
		return fmt.Errorf("unsupported shell for installation: %s", shellName)
	}

	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open shell config: %w", err)
	}
	defer f.Close()

	marker := fmt.Sprintf("\n%s\n%s\n%s\n", integrationStartMarker, shellCode, integrationEndMarker)
	if _, err := f.WriteString(marker); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}

	return nil
}

func (i *Installer) Uninstall(shellName string) error {
	shellName = CanonicalName(shellName)
	if shellName == "" {
		return fmt.Errorf("unsupported shell")
	}

	if shellName == "cmd" {
		return uninstallCmdIntegration()
	}

	configFile, err := GetConfigFile(shellName)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read shell config: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	newLines := make([]string, 0, len(lines))
	inWUTSection := false

	for _, line := range lines {
		if strings.Contains(line, integrationStartMarker) {
			inWUTSection = true
			continue
		}
		if strings.Contains(line, integrationEndMarker) || strings.Contains(line, legacyIntegrationEnd) {
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

func GetDetectedShells() []string {
	return DetectInstallableShells()
}

func (i *Installer) GetDetectedShells() []string {
	return i.shells
}

func GetConfigFile(shellName string) (string, error) {
	shellName = CanonicalName(shellName)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	_, xdgConfigHome := xdgDirs(home)
	appData := strings.TrimSpace(os.Getenv("APPDATA"))

	switch shellName {
	case "bash":
		defaultPath := filepath.Join(home, ".bashrc")
		if runtime.GOOS == "darwin" {
			defaultPath = filepath.Join(home, ".bash_profile")
		}
		return pickConfigPath(defaultPath,
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
			filepath.Join(home, ".profile"),
		), nil
	case "zsh":
		return pickConfigPath(filepath.Join(home, ".zshrc"),
			filepath.Join(home, ".zshrc"),
			filepath.Join(home, ".zprofile"),
		), nil
	case "fish":
		return filepath.Join(xdgConfigHome, "fish", "config.fish"), nil
	case "powershell", "pwsh":
		if profile, err := queryPowerShellProfile(shellName); err == nil && profile != "" {
			return profile, nil
		}

		if runtime.GOOS == "windows" {
			if shellName == "powershell" {
				return filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), nil
			}
			return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
		}
		return filepath.Join(xdgConfigHome, "powershell", "Microsoft.PowerShell_profile.ps1"), nil
	case "nushell":
		if configPath, err := queryNuConfigPath(); err == nil && configPath != "" {
			return configPath, nil
		}
		if runtime.GOOS == "windows" && appData != "" {
			return filepath.Join(appData, "nushell", "config.nu"), nil
		}
		return filepath.Join(xdgConfigHome, "nushell", "config.nu"), nil
	case "xonsh":
		defaultPath := filepath.Join(home, ".xonshrc")
		if runtime.GOOS == "windows" && appData != "" {
			return pickConfigPath(defaultPath,
				filepath.Join(home, ".xonshrc"),
				filepath.Join(appData, "xonsh", "rc.xsh"),
				filepath.Join(home, ".config", "xonsh", "rc.xsh"),
			), nil
		}
		return pickConfigPath(defaultPath,
			filepath.Join(home, ".xonshrc"),
			filepath.Join(xdgConfigHome, "xonsh", "rc.xsh"),
		), nil
	case "elvish":
		legacyPath := filepath.Join(home, ".elvish", "rc.elv")
		if _, err := os.Stat(legacyPath); err == nil {
			return legacyPath, nil
		}
		if runtime.GOOS == "windows" && appData != "" {
			return filepath.Join(appData, "elvish", "rc.elv"), nil
		}
		return filepath.Join(xdgConfigHome, "elvish", "rc.elv"), nil
	case "tcsh":
		return pickConfigPath(filepath.Join(home, ".tcshrc"),
			filepath.Join(home, ".tcshrc"),
			filepath.Join(home, ".cshrc"),
		), nil
	case "csh":
		return filepath.Join(home, ".cshrc"), nil
	case "ksh":
		return filepath.Join(home, ".kshrc"), nil
	case "mksh":
		return filepath.Join(home, ".mkshrc"), nil
	case "yash":
		return filepath.Join(home, ".yashrc"), nil
	case "dash", "ash", "sh":
		return filepath.Join(home, ".profile"), nil
	case "cmd":
		return cmdInitScriptPath(), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shellName)
	}
}

func IsInstalled(configFile string) bool {
	content, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), integrationStartMarker)
}

func GenerateShellCode(shellName string) string {
	shellName = CanonicalName(shellName)
	switch shellName {
	case "bash", "zsh":
		return generateBashZshCode()
	case "fish":
		return generateFishCode()
	case "powershell", "pwsh":
		return generatePowerShellCode(shellName)
	case "nushell":
		return generateNushellCode()
	case "xonsh":
		return generateXonshCode()
	case "elvish":
		return generateElvishCode()
	case "cmd":
		return generateCmdCode()
	default:
		return ""
	}
}

func GetReloadCommand(shellName, configFile string) string {
	shellName = CanonicalName(shellName)
	switch shellName {
	case "bash", "zsh", "fish":
		return "source " + configFile
	case "powershell", "pwsh":
		return ". " + configFile
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

__wut_last_command_from_history() {
    if [[ $# -gt 0 ]]; then
        printf '%s\n' "$*"
        return
    fi

    local last_cmd=""
    if [[ -n "$BASH_VERSION" || -n "$ZSH_VERSION" ]]; then
        last_cmd="$(fc -ln -1 2>/dev/null | tail -n 1)"
        case "$last_cmd" in
            oops*|again*|wut\ *)
                last_cmd="$(fc -ln -2 2>/dev/null | head -n 1)"
                ;;
        esac
    fi
    printf '%s\n' "$last_cmd"
}

oops() {
    local cmd
    cmd="$(__wut_last_command_from_history "$@")"
    cmd="$(printf '%s' "$cmd" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
    if [[ -z "$cmd" || "$cmd" == wut\ * || "$cmd" == oops* || "$cmd" == again* ]]; then
        return 1
    fi

    local fixed
    fixed="$(WUT_SOURCE_SHELL="${WUT_SOURCE_SHELL:-${BASH_VERSION:+bash}${ZSH_VERSION:+zsh}}" wut fix --shell "$cmd")" || {
        wut fix "$cmd"
        return 1
    }

    if [[ -z "$fixed" || "$fixed" == "$cmd" ]]; then
        wut fix "$cmd"
        return 1
    fi

    printf '%s\n' "$fixed"
    eval "$fixed"
}

again() {
    oops "$@"
}

if [[ -n "$BASH_VERSION" ]] && declare -F command_not_found_handle >/dev/null 2>&1; then
    eval "$(declare -f command_not_found_handle | sed '1s/command_not_found_handle/__wut_original_command_not_found_handle/')"
fi
if [[ -n "$ZSH_VERSION" ]] && typeset -f command_not_found_handler >/dev/null 2>&1; then
    eval "$(functions command_not_found_handler | sed '1s/command_not_found_handler/__wut_original_command_not_found_handler/')"
fi

command_not_found_handle() {
    wut fix "$*"
    if declare -F __wut_original_command_not_found_handle >/dev/null 2>&1; then
        __wut_original_command_not_found_handle "$@"
        return $?
    fi
    return 127
}
command_not_found_handler() {
    wut fix "$*"
    if typeset -f __wut_original_command_not_found_handler >/dev/null 2>&1; then
        __wut_original_command_not_found_handler "$@"
        return $?
    fi
    return 127
}

__wut_last_hist_id=""

__wut_record_last_command() {
    local histnum=""
    local cmd=""

    if [[ -n "$BASH_VERSION" ]]; then
        local hist_entry
        hist_entry="$(history 1 2>/dev/null)"
        histnum="$(printf '%s' "$hist_entry" | sed -E 's/^[[:space:]]*([0-9]+).*/\1/')"
        cmd="$(printf '%s' "$hist_entry" | sed -E 's/^[[:space:]]*[0-9]+[[:space:]]*//')"
    elif [[ -n "$ZSH_VERSION" ]]; then
        histnum="${HISTCMD:-}"
        cmd="$(fc -ln -1 2>/dev/null)"
    fi

    if [[ -n "$cmd" && "$histnum" != "$__wut_last_hist_id" && "$cmd" != wut\ * ]]; then
        __wut_last_hist_id="$histnum"
        WUT_SOURCE_SHELL="${WUT_SOURCE_SHELL:-${BASH_VERSION:+bash}${ZSH_VERSION:+zsh}}" wut pro-tip "$cmd"
    fi
}

__wut_protip() {
    local exitStatus=$?
    __wut_record_last_command
    return $exitStatus
}

if [[ -n "$BASH_VERSION" ]]; then
    bind '"\C-@":"\C-uwut suggest\C-m"' 2>/dev/null || true
    bind '"\C-g":"\C-awut suggest \"\C-e\"\C-m"' 2>/dev/null || true
    PROMPT_COMMAND="__wut_protip; $PROMPT_COMMAND"
elif [[ -n "$ZSH_VERSION" ]]; then
    autoload -Uz add-zsh-hook 2>/dev/null
    add-zsh-hook precmd __wut_protip 2>/dev/null || true
    __wut_zle_tui() {
        BUFFER='wut suggest'
        zle accept-line
    }
    __wut_zle_current() {
        local cmd="$BUFFER"
        BUFFER="wut suggest ${(q)cmd}"
        zle accept-line
    }
    zle -N __wut_zle_tui
    zle -N __wut_zle_current
    bindkey '^@' __wut_zle_tui 2>/dev/null || true
    bindkey '^G' __wut_zle_current 2>/dev/null || true
fi
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

function oops
    set -l cmd (string join ' ' $argv)
    if test -z "$cmd"
        set cmd $history[1]
        if string match -qr '^(oops|again|wut)\b' -- $cmd
            set cmd $history[2]
        end
    end

    set cmd (string trim -- $cmd)
    if test -z "$cmd"
        return 1
    end
    if string match -qr '^(oops|again|wut)\b' -- $cmd
        return 1
    end

    set -l fixed (env WUT_SOURCE_SHELL=fish wut fix --shell "$cmd")
    if test $status -ne 0
        wut fix "$cmd"
        return 1
    end

    set fixed (string trim -- $fixed)
    if test -z "$fixed"
        wut fix "$cmd"
        return 1
    end

    echo $fixed
    eval $fixed
end

function again
    oops $argv
end

functions -q fish_command_not_found; and functions -c fish_command_not_found __wut_original_fish_command_not_found
function fish_command_not_found
    wut fix "$argv"
    if functions -q __wut_original_fish_command_not_found
        __wut_original_fish_command_not_found $argv
    end
end

set -g __wut_last_command ''

function __wut_protip --on-event fish_prompt
    set -l cmd $history[1]
    if test -n "$cmd"; and test "$cmd" != "$__wut_last_command"
        set -g __wut_last_command $cmd
        env WUT_SOURCE_SHELL=fish wut pro-tip "$cmd"
    end
end

bind \c@ __wut_tui 2>/dev/null; or true
bind \cg __wut_with_current 2>/dev/null; or true
`
}

func generatePowerShellCode(sourceShell string) string {
	return fmt.Sprintf(`# WUT Key Bindings - Quick Access
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

function Invoke-WUTOops {
    param(
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$CommandLine
    )

    $target = ($CommandLine -join ' ').Trim()
    if (-not $target) {
        $history = @(Get-History -Count 2 -ErrorAction SilentlyContinue)
        if ($history.Count -gt 0) {
            $target = $history[0].CommandLine
            if (($target -like 'oops*' -or $target -like 'again*' -or $target -like 'wut *') -and $history.Count -gt 1) {
                $target = $history[1].CommandLine
            }
        }
    }

    if (-not $target -or $target -like 'oops*' -or $target -like 'again*' -or $target -like 'wut *') {
        return
    }

    $env:WUT_SOURCE_SHELL = '%s'
    $fixed = & wut fix --shell $target
    $exitCode = $LASTEXITCODE
    Remove-Item Env:\WUT_SOURCE_SHELL -ErrorAction SilentlyContinue

    if ($exitCode -ne 0 -or [string]::IsNullOrWhiteSpace($fixed)) {
        & wut fix $target
        return
    }

    Write-Host $fixed -ForegroundColor Cyan
    Invoke-Expression $fixed
}

Set-Alias oops Invoke-WUTOops -ErrorAction SilentlyContinue
Set-Alias again Invoke-WUTOops -ErrorAction SilentlyContinue

if (-not $global:WUTOriginalCommandNotFoundAction) {
    $global:WUTOriginalCommandNotFoundAction = $ExecutionContext.InvokeCommand.CommandNotFoundAction
}

$ExecutionContext.InvokeCommand.CommandNotFoundAction = {
    param([string]$commandName, [System.Management.Automation.CommandLookupEventArgs]$commandLookupEventArgs)
    wut fix "$commandName"
    if ($global:WUTOriginalCommandNotFoundAction) {
        & $global:WUTOriginalCommandNotFoundAction $commandName $commandLookupEventArgs
    } else {
        $commandLookupEventArgs.CommandScriptBlock = { }
    }
}

if (-not $global:WUTOriginalPrompt) {
    if (Test-Path Function:\prompt) {
        $global:WUTOriginalPrompt = $function:prompt
    }
}

function global:prompt {
    $promptText = ""
    if ($global:WUTOriginalPrompt) {
        $promptText = & $global:WUTOriginalPrompt
    } else {
        $promptText = "PS $($executionContext.SessionState.Path.CurrentLocation)$('>' * ($nestedPromptLevel + 1)) "
    }

    try {
        $last = Get-History -Count 1 -ErrorAction SilentlyContinue
        if ($last -and $global:WUTLastHistoryId -ne $last.Id -and $last.CommandLine -notlike 'wut *') {
            $global:WUTLastHistoryId = $last.Id
            $env:WUT_SOURCE_SHELL = '%s'
            wut pro-tip "$($last.CommandLine)"
            Remove-Item Env:\WUT_SOURCE_SHELL -ErrorAction SilentlyContinue
        }
    } catch {
    }

    return $promptText
}

Set-PSReadLineKeyHandler -Chord 'Ctrl+SpaceBar' -ScriptBlock { Invoke-WUT-TUI } -ErrorAction SilentlyContinue
Set-PSReadLineKeyHandler -Chord 'Ctrl+g' -ScriptBlock { Invoke-WUT-WithCurrent } -ErrorAction SilentlyContinue
`, sourceShell, sourceShell)
}

func generateNushellCode() string {
	return `# WUT integration for Nushell
$env.WUT_LAST_COMMAND = ($env.WUT_LAST_COMMAND? | default "")
$env.WUT_LAST_RECORDED = ($env.WUT_LAST_RECORDED? | default "")

$env.config = ($env.config | default {})
$env.config.hooks = ($env.config.hooks? | default {})

$env.config.hooks.pre_execution = (
    $env.config.hooks.pre_execution?
    | default []
    | append {||
        $env.WUT_LAST_COMMAND = (commandline)
    }
)

$env.config.hooks.pre_prompt = (
    $env.config.hooks.pre_prompt?
    | default []
    | append {||
        let cmd = (($env.WUT_LAST_COMMAND? | default "") | str trim)
        let last = ($env.WUT_LAST_RECORDED? | default "")
        if ($cmd | str length) > 0 and $cmd != $last and not ($cmd | str starts-with "wut ") {
            $env.WUT_LAST_RECORDED = $cmd
            with-env { WUT_SOURCE_SHELL: "nushell" } { ^wut pro-tip $cmd }
        }
    }
)

$env.config.hooks.command_not_found = (
    $env.config.hooks.command_not_found?
    | default []
    | append {|command_name|
        ^wut fix $command_name | ignore
        null
    }
)

def --env wut-current-line [] {
    ^wut suggest (commandline)
}

def --env oops [...args] {
    if (($args | length) == 0) {
        ^wut fix --exec
    } else {
        ^wut fix --exec ...$args
    }
}

def --env again [...args] {
    oops ...$args
}
`
}

func generateXonshCode() string {
	return `# WUT integration for Xonsh
import os
import subprocess

from xonsh.events import events

aliases["wut-tui"] = ["wut", "suggest"]
aliases["oops"] = lambda args: subprocess.run(["wut", "fix", "--exec", *args], check=False)
aliases["again"] = aliases["oops"]

@events.on_postcommand
def _wut_postcommand(cmd, rtn=None, out=None, ts=None, **kwargs):
    line = (cmd or "").strip()
    if not line or line.startswith("wut "):
        return
    env = dict(os.environ)
    env["WUT_SOURCE_SHELL"] = "xonsh"
    subprocess.run(["wut", "pro-tip", line], env=env, check=False)

@events.on_command_not_found
def _wut_command_not_found(cmd, **kwargs):
    subprocess.run(["wut", "fix", cmd], check=False)

@events.on_ptk_create
def _wut_keybindings(bindings, **kwargs):
    try:
        from prompt_toolkit.keys import Keys
    except Exception:
        return

    @bindings.add(Keys.ControlG)
    def _wut_with_current(event):
        line = event.current_buffer.text
        subprocess.run(["wut", "suggest", line], check=False)
        event.app.renderer.erase()
`
}

func generateElvishCode() string {
	return `# WUT integration for Elvish
use edit
use str

var wut:last-command = ''

set edit:after-readline = [ $@edit:after-readline {|line|
    var cmd = (str:trim-space $line)
    if (and (!=s $cmd '') (!=s $cmd $wut:last-command) (not (str:has-prefix $cmd 'wut '))) {
        set wut:last-command = $cmd
        E:WUT_SOURCE_SHELL=elvish wut pro-tip $cmd > /dev/null 2> /dev/null
    }
} ]

set edit:insert:binding[Ctrl-G] = {
    wut suggest $edit:current-command
}

set edit:insert:binding[Ctrl-@] = {
    wut suggest
}

fn oops {|@args|
    wut fix --exec $@args
}

fn again {|@args|
    oops $@args
}
`
}

func generateCmdCode() string {
	return `@echo off
doskey wut-tui=wut suggest
doskey wut-current=wut suggest $*
doskey wut-fix=wut fix $*
doskey oops=wut fix --exec $*
doskey again=wut fix --exec $*
`
}

func queryPowerShellProfile(shellName string) (string, error) {
	out, err := exec.Command(shellName, "-NoProfile", "-Command", "Write-Output $PROFILE").Output()
	if err != nil {
		return "", err
	}

	profile := strings.TrimSpace(string(out))
	if profile == "" {
		return "", fmt.Errorf("empty profile path")
	}

	if err := os.MkdirAll(filepath.Dir(profile), 0755); err != nil {
		return "", fmt.Errorf("failed to create profile directory: %w", err)
	}
	return profile, nil
}

func queryNuConfigPath() (string, error) {
	out, err := exec.Command("nu", "-c", "$nu.config-path").Output()
	if err != nil {
		return "", err
	}

	configPath := strings.TrimSpace(string(out))
	if configPath == "" {
		return "", fmt.Errorf("empty config path")
	}
	return configPath, nil
}

func pickConfigPath(defaultPath string, candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return defaultPath
}

func installCmdIntegration() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("cmd integration is only supported on Windows")
	}

	scriptPath := cmdInitScriptPath()
	if isCmdInstalled(scriptPath) {
		return fmt.Errorf("already installed")
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		return fmt.Errorf("failed to create cmd integration directory: %w", err)
	}
	if err := os.WriteFile(scriptPath, []byte(generateCmdCode()), 0644); err != nil {
		return fmt.Errorf("failed to write cmd integration script: %w", err)
	}

	snippet := cmdAutoRunSnippet(scriptPath)
	currentValue, err := readRegistryString(cmdAutoRunKey, cmdAutoRunValue)
	if err != nil {
		return fmt.Errorf("failed to read cmd autorun: %w", err)
	}
	updatedValue := strings.TrimSpace(currentValue)
	if !strings.Contains(updatedValue, snippet) {
		if updatedValue == "" {
			updatedValue = snippet
		} else {
			updatedValue += " & " + snippet
		}
	}
	if err := writeRegistryString(cmdAutoRunKey, cmdAutoRunValue, updatedValue); err != nil {
		return fmt.Errorf("failed to configure cmd autorun: %w", err)
	}

	return nil
}

func uninstallCmdIntegration() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("cmd integration is only supported on Windows")
	}

	scriptPath := cmdInitScriptPath()
	snippet := cmdAutoRunSnippet(scriptPath)

	currentValue, err := readRegistryString(cmdAutoRunKey, cmdAutoRunValue)
	if err != nil {
		return fmt.Errorf("failed to read cmd autorun: %w", err)
	}

	updatedValue := strings.TrimSpace(currentValue)
	updatedValue = strings.Replace(updatedValue, " & "+snippet, "", 1)
	updatedValue = strings.Replace(updatedValue, snippet+" & ", "", 1)
	updatedValue = strings.Replace(updatedValue, snippet, "", 1)
	updatedValue = strings.Trim(strings.TrimSpace(updatedValue), "&")
	updatedValue = strings.TrimSpace(updatedValue)

	if updatedValue == "" {
		if err := deleteRegistryValue(cmdAutoRunKey, cmdAutoRunValue); err != nil {
			return fmt.Errorf("failed to remove cmd autorun: %w", err)
		}
	} else if updatedValue != currentValue {
		if err := writeRegistryString(cmdAutoRunKey, cmdAutoRunValue, updatedValue); err != nil {
			return fmt.Errorf("failed to update cmd autorun: %w", err)
		}
	}

	_ = os.Remove(scriptPath)
	return nil
}

func isCmdInstalled(scriptPath string) bool {
	currentValue, err := readRegistryString(cmdAutoRunKey, cmdAutoRunValue)
	if err != nil {
		return false
	}
	return strings.Contains(currentValue, cmdAutoRunSnippet(scriptPath))
}

func cmdInitScriptPath() string {
	return filepath.Join(config.GetDataDir(), "shell", "wut-cmd-init.cmd")
}

func cmdAutoRunSnippet(scriptPath string) string {
	scriptPath = strings.ReplaceAll(scriptPath, `"`, `\"`)
	return fmt.Sprintf(`if exist "%s" call "%s"`, scriptPath, scriptPath)
}

func readRegistryString(key, valueName string) (string, error) {
	cmd := exec.Command("reg", "query", key, "/v", valueName)
	output, err := cmd.Output()
	if err != nil {
		return "", nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if !strings.EqualFold(fields[0], valueName) {
			continue
		}
		return strings.Join(fields[2:], " "), nil
	}

	return "", nil
}

func writeRegistryString(key, valueName, value string) error {
	cmd := exec.Command("reg", "add", key, "/v", valueName, "/t", "REG_EXPAND_SZ", "/d", value, "/f")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func deleteRegistryValue(key, valueName string) error {
	cmd := exec.Command("reg", "delete", key, "/v", valueName, "/f")
	if output, err := cmd.CombinedOutput(); err != nil {
		lower := strings.ToLower(string(output))
		if strings.Contains(lower, "unable to find") || strings.Contains(lower, "cannot find") {
			return nil
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
