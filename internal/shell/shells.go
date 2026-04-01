package shell

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type HistorySourceKind string

const (
	HistorySourceFile    HistorySourceKind = "file"
	HistorySourceCommand HistorySourceKind = "command"
)

type HistorySource struct {
	Shell   string
	Path    string
	Kind    HistorySourceKind
	Command string
	Args    []string
}

func (s HistorySource) DisplayPath() string {
	if s.Kind == HistorySourceCommand {
		if s.Path != "" {
			return s.Path
		}
		return strings.TrimSpace(strings.Join(append([]string{s.Command}, s.Args...), " "))
	}
	return s.Path
}

func SupportedShells() []string {
	return []string{
		"bash",
		"zsh",
		"fish",
		"powershell",
		"pwsh",
		"cmd",
		"nushell",
		"xonsh",
		"elvish",
		"tcsh",
		"csh",
		"ksh",
		"mksh",
		"yash",
		"dash",
		"ash",
		"sh",
	}
}

func IntegrationShells() []string {
	return []string{
		"bash",
		"zsh",
		"fish",
		"powershell",
		"pwsh",
		"nushell",
		"xonsh",
		"elvish",
		"cmd",
	}
}

func SupportsInstall(shellName string) bool {
	switch CanonicalName(shellName) {
	case "bash", "zsh", "fish", "powershell", "pwsh", "nushell", "xonsh", "elvish", "cmd":
		return true
	default:
		return false
	}
}

func CanonicalName(shellName string) string {
	shellName = strings.ToLower(strings.TrimSpace(shellName))
	if shellName == "" {
		return ""
	}
	shellName = filepath.Base(shellName)
	shellName = strings.TrimSuffix(shellName, ".exe")

	switch {
	case shellName == "nu", shellName == "nushell":
		return "nushell"
	case shellName == "pwsh":
		return "pwsh"
	case shellName == "powershell", shellName == "powershell_ise", strings.Contains(shellName, "windowspowershell"):
		return "powershell"
	case strings.Contains(shellName, "bash"):
		return "bash"
	case strings.Contains(shellName, "zsh"):
		return "zsh"
	case strings.Contains(shellName, "fish"):
		return "fish"
	case strings.Contains(shellName, "xonsh"):
		return "xonsh"
	case strings.Contains(shellName, "elvish"):
		return "elvish"
	case shellName == "tcsh":
		return "tcsh"
	case shellName == "csh":
		return "csh"
	case shellName == "ksh", shellName == "ksh93":
		return "ksh"
	case shellName == "mksh":
		return "mksh"
	case shellName == "yash":
		return "yash"
	case shellName == "dash":
		return "dash"
	case shellName == "ash":
		return "ash"
	case shellName == "sh", shellName == "posh":
		return "sh"
	case shellName == "cmd", strings.Contains(shellName, "cmd"):
		return "cmd"
	default:
		return shellName
	}
}

func DetectCurrentShell() string {
	if sourceShell := CanonicalName(os.Getenv("WUT_SOURCE_SHELL")); sourceShell != "" {
		return sourceShell
	}

	switch {
	case os.Getenv("NU_VERSION") != "":
		return "nushell"
	case os.Getenv("XONSH_VERSION") != "":
		return "xonsh"
	case os.Getenv("ELVISH_VERSION") != "":
		return "elvish"
	case os.Getenv("FISH_VERSION") != "":
		return "fish"
	case os.Getenv("ZSH_VERSION") != "":
		return "zsh"
	case os.Getenv("BASH_VERSION") != "":
		return "bash"
	}

	if shellPath := os.Getenv("SHELL"); shellPath != "" {
		if shellName := CanonicalName(shellPath); shellName != "" {
			return shellName
		}
	}

	if runtime.GOOS == "windows" {
		if os.Getenv("PSModulePath") != "" {
			if exe := strings.ToLower(filepath.Base(os.Args[0])); strings.Contains(exe, "pwsh") {
				return "pwsh"
			}
			return "powershell"
		}
		if comspec := CanonicalName(os.Getenv("COMSPEC")); comspec != "" {
			return comspec
		}
		return "cmd"
	}

	return ""
}

func DetectAvailableShells() []string {
	ordered := []struct {
		name        string
		executables []string
	}{
		{name: "bash", executables: []string{"bash"}},
		{name: "zsh", executables: []string{"zsh"}},
		{name: "fish", executables: []string{"fish"}},
		{name: "powershell", executables: []string{"powershell"}},
		{name: "pwsh", executables: []string{"pwsh"}},
		{name: "nushell", executables: []string{"nu"}},
		{name: "xonsh", executables: []string{"xonsh"}},
		{name: "elvish", executables: []string{"elvish"}},
		{name: "tcsh", executables: []string{"tcsh"}},
		{name: "csh", executables: []string{"csh"}},
		{name: "ksh", executables: []string{"ksh", "ksh93"}},
		{name: "mksh", executables: []string{"mksh"}},
		{name: "yash", executables: []string{"yash"}},
		{name: "dash", executables: []string{"dash"}},
		{name: "ash", executables: []string{"ash"}},
		{name: "sh", executables: []string{"sh"}},
	}

	seen := make(map[string]struct{}, len(ordered)+2)
	shells := make([]string, 0, len(ordered)+2)
	add := func(shellName string) {
		shellName = CanonicalName(shellName)
		if shellName == "" {
			return
		}
		if _, ok := seen[shellName]; ok {
			return
		}
		seen[shellName] = struct{}{}
		shells = append(shells, shellName)
	}

	add(DetectCurrentShell())
	if runtime.GOOS == "windows" {
		add("cmd")
	}

	for _, candidate := range ordered {
		for _, executable := range candidate.executables {
			if _, err := exec.LookPath(executable); err == nil {
				add(candidate.name)
				break
			}
		}
	}

	return shells
}

func DetectInstallableShells() []string {
	all := DetectAvailableShells()
	installable := make([]string, 0, len(all))
	for _, shellName := range all {
		if SupportsInstall(shellName) {
			installable = append(installable, shellName)
		}
	}
	return installable
}

func DetectPreferredInstallShell() string {
	current := DetectCurrentShell()
	if SupportsInstall(current) {
		return current
	}

	if installable := DetectInstallableShells(); len(installable) > 0 {
		return installable[0]
	}

	return ""
}

func DetectHistorySources() []HistorySource {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	xdgDataHome, xdgConfigHome := xdgDirs(home)
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))

	sources := make([]HistorySource, 0, 24)
	seen := make(map[string]struct{}, 32)
	addSource := func(source HistorySource) {
		source.Shell = CanonicalName(source.Shell)
		if source.Shell == "" {
			return
		}
		key := strings.Join([]string{
			source.Shell,
			string(source.Kind),
			filepath.Clean(source.Path),
			source.Command,
			strings.Join(source.Args, "\x00"),
		}, "\x01")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		sources = append(sources, source)
	}

	addFileSources := func(shellName string, candidates ...string) {
		for _, candidate := range uniqueExistingPaths(candidates...) {
			addSource(HistorySource{
				Shell: shellName,
				Path:  candidate,
				Kind:  HistorySourceFile,
			})
		}
	}

	addFileSources("bash",
		filepath.Join(home, ".bash_history"),
		filepath.Join(xdgDataHome, "bash", "history"),
	)
	addFileSources("zsh",
		filepath.Join(home, ".zsh_history"),
		filepath.Join(xdgDataHome, "zsh", "history"),
	)
	addFileSources("fish",
		filepath.Join(xdgDataHome, "fish", "fish_history"),
		filepath.Join(xdgConfigHome, "fish", "fish_history"),
	)
	addFileSources("tcsh",
		filepath.Join(home, ".tcsh_history"),
	)
	addFileSources("csh",
		filepath.Join(home, ".history"),
	)
	addFileSources("ksh",
		filepath.Join(home, ".ksh_history"),
	)
	addFileSources("mksh",
		filepath.Join(home, ".mksh_history"),
	)
	addFileSources("yash",
		filepath.Join(home, ".yash_history"),
	)
	addFileSources("ash",
		filepath.Join(home, ".ash_history"),
	)
	addFileSources("sh",
		filepath.Join(home, ".sh_history"),
	)

	for _, source := range detectPowerShellHistorySources(home, xdgDataHome, xdgConfigHome) {
		addSource(source)
	}
	for _, source := range detectClinkHistorySources(localAppData, appData) {
		addSource(source)
	}

	if _, err := exec.LookPath("nu"); err == nil {
		addSource(HistorySource{
			Shell:   "nushell",
			Path:    "nu history export",
			Kind:    HistorySourceCommand,
			Command: "nu",
			Args:    []string{"-c", "history --long | get command | to json"},
		})
	} else {
		addFileSources("nushell",
			filepath.Join(xdgDataHome, "nushell", "history.txt"),
			filepath.Join(xdgConfigHome, "nushell", "history.txt"),
			filepath.Join(home, ".local", "share", "nushell", "history.txt"),
			filepath.Join(home, ".config", "nushell", "history.txt"),
			filepath.Join(appData, "nushell", "history.txt"),
		)
	}

	if _, err := exec.LookPath("xonsh"); err == nil {
		addSource(HistorySource{
			Shell:   "xonsh",
			Path:    "xonsh history export",
			Kind:    HistorySourceCommand,
			Command: "xonsh",
			Args: []string{
				"-c",
				"import json; from xonsh.built_ins import XSH; hist = XSH.history; items = [] if hist is None else [item.get('inp', '').rstrip() for item in hist.all_items(newest_first=False)]; print(json.dumps(items))",
			},
		})
	}

	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].Shell == sources[j].Shell {
			return sources[i].DisplayPath() < sources[j].DisplayPath()
		}
		return sources[i].Shell < sources[j].Shell
	})

	return sources
}

func ReadHistory(source HistorySource) ([]string, error) {
	switch source.Kind {
	case HistorySourceCommand:
		return readHistoryCommand(source)
	case HistorySourceFile:
		return readHistoryFile(source.Shell, source.Path)
	default:
		return nil, fmt.Errorf("unsupported history source kind: %s", source.Kind)
	}
}

func readHistoryCommand(source HistorySource) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, source.Command, source.Args...)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal(output, &values); err == nil {
		return compactCommands(values), nil
	}

	lines := strings.Split(string(output), "\n")
	return compactCommands(lines), nil
}

func readHistoryFile(shellName, path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	commands := make([]string, 0, 1024)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	switch CanonicalName(shellName) {
	case "fish":
		for scanner.Scan() {
			line := scanner.Text()
			if after, ok := strings.CutPrefix(line, "- cmd: "); ok {
				commands = append(commands, after)
			}
		}
	case "zsh":
		for scanner.Scan() {
			line := scanner.Text()
			if _, after, ok := strings.Cut(line, ";"); ok {
				commands = append(commands, after)
				continue
			}
			commands = append(commands, line)
		}
	default:
		for scanner.Scan() {
			commands = append(commands, scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return compactCommands(commands), nil
}

func compactCommands(commands []string) []string {
	result := make([]string, 0, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		result = append(result, command)
	}
	return result
}

func uniqueExistingPaths(candidates ...string) []string {
	seen := make(map[string]struct{}, len(candidates))
	results := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		seen[candidate] = struct{}{}
		results = append(results, candidate)
	}
	return results
}

func xdgDirs(home string) (string, string) {
	xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(home, ".local", "share")
	}

	xdgConfigHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(home, ".config")
	}

	return xdgDataHome, xdgConfigHome
}

func detectPowerShellHistorySources(home, xdgDataHome, xdgConfigHome string) []HistorySource {
	directories := make([]string, 0, 8)
	if runtime.GOOS == "windows" {
		appData := strings.TrimSpace(os.Getenv("APPDATA"))
		directories = append(directories,
			filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine"),
			filepath.Join(appData, "Microsoft", "PowerShell", "PSReadLine"),
			filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine"),
			filepath.Join(home, "AppData", "Roaming", "Microsoft", "PowerShell", "PSReadLine"),
		)
	} else {
		directories = append(directories,
			filepath.Join(xdgDataHome, "powershell", "PSReadLine"),
			filepath.Join(xdgConfigHome, "powershell", "PSReadLine"),
		)
	}

	seen := make(map[string]struct{}, len(directories))
	results := make([]HistorySource, 0, len(directories))
	for _, dir := range directories {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		pattern := filepath.Join(dir, "*_history.txt")
		matches, _ := filepath.Glob(pattern)
		if len(matches) == 0 {
			matches = append(matches, filepath.Join(dir, "ConsoleHost_history.txt"))
		}
		for _, match := range matches {
			match = filepath.Clean(match)
			if _, ok := seen[match]; ok {
				continue
			}
			if _, err := os.Stat(match); err != nil {
				continue
			}
			seen[match] = struct{}{}
			results = append(results, HistorySource{
				Shell: detectPowerShellShell(match),
				Path:  match,
				Kind:  HistorySourceFile,
			})
		}
	}

	return results
}

func detectPowerShellShell(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(lower, "/windowspowershell/"):
		return "powershell"
	case strings.Contains(lower, "/powershell/"):
		return "pwsh"
	default:
		return "powershell"
	}
}

func detectClinkHistorySources(localAppData, appData string) []HistorySource {
	candidates := make([]string, 0, 6)
	if clinkProfile := strings.TrimSpace(os.Getenv("CLINK_PROFILE")); clinkProfile != "" {
		candidates = append(candidates,
			filepath.Join(clinkProfile, "clink_history"),
			filepath.Join(clinkProfile, "history"),
		)
	}
	candidates = append(candidates,
		filepath.Join(localAppData, "clink", "clink_history"),
		filepath.Join(localAppData, "clink", "history"),
		filepath.Join(appData, "clink", "clink_history"),
		filepath.Join(appData, "clink", "history"),
	)

	paths := uniqueExistingPaths(candidates...)
	results := make([]HistorySource, 0, len(paths))
	for _, path := range paths {
		results = append(results, HistorySource{
			Shell: "cmd",
			Path:  path,
			Kind:  HistorySourceFile,
		})
	}
	return results
}
