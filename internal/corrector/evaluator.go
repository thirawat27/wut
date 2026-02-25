package corrector

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Rule represents a condition and a correction logic based on command execution error output.
type Rule struct {
	Name        string
	Match       func(command string, output string) bool
	GetNewCmd   func(command string, output string) []string
	Explanation string
}

var coreRules = []Rule{
	{
		Name: "git_push_set_upstream",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "git push") && strings.Contains(output, "git push --set-upstream")
		},
		GetNewCmd: func(command string, output string) []string {
			re := regexp.MustCompile(`git push --set-upstream origin[^\n]*`)
			match := re.FindString(output)
			if match != "" {
				return []string{strings.TrimSpace(match)}
			}
			return nil
		},
		Explanation: "Set upstream branch for git push",
	},
	{
		Name: "sudo_permission_denied",
		Match: func(command string, output string) bool {
			return !strings.HasPrefix(command, "sudo ") && (strings.Contains(strings.ToLower(output), "permission denied") || strings.Contains(strings.ToLower(output), "operation not permitted") || strings.Contains(strings.ToLower(output), "are you root"))
		},
		GetNewCmd: func(command string, output string) []string {
			return []string{"sudo " + command}
		},
		Explanation: "Command requires elevated privileges (sudo)",
	},
	{
		Name: "git_did_you_mean",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "git ") && strings.Contains(output, "Did you mean this?")
		},
		GetNewCmd: func(command string, output string) []string {
			re := regexp.MustCompile(`Did you mean this\?\n\s+([A-Za-z0-9_-]+)`)
			match := re.FindStringSubmatch(output)
			if len(match) > 1 {
				parts := strings.Fields(command)
				if len(parts) > 1 {
					parts[1] = match[1]
					return []string{strings.Join(parts, " ")}
				}
			}
			return nil
		},
		Explanation: "Git suggested a correct command",
	},
	{
		Name: "apt_get_search",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "apt-get search") || strings.HasPrefix(command, "apt search")
		},
		GetNewCmd: func(command string, output string) []string {
			return []string{strings.Replace(command, "apt-get", "apt-cache", 1), strings.Replace(command, "apt search", "apt-cache search", 1)}
		},
		Explanation: "Use apt-cache to search for packages instead",
	},
	{
		Name: "brew_install_update",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "brew install") && strings.Contains(output, "No available formula")
		},
		GetNewCmd: func(command string, output string) []string {
			return []string{"brew update && " + command}
		},
		Explanation: "Formula not found, updating brew might help",
	},
	{
		Name: "cd_parent",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "cd..")
		},
		GetNewCmd: func(command string, output string) []string {
			parts := strings.Replace(command, "cd..", "cd ..", 1)
			return []string{parts}
		},
		Explanation: "Missing space in cd command",
	},
	{
		Name: "docker_not_running",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "docker ") && strings.Contains(output, "Cannot connect to the Docker daemon")
		},
		GetNewCmd: func(command string, output string) []string {
			return []string{"sudo systemctl start docker && " + command, "sudo service docker start && " + command}
		},
		Explanation: "Docker daemon is not running, starting it first",
	},
	{
		Name: "port_in_use",
		Match: func(command string, output string) bool {
			return strings.Contains(output, "address already in use") || strings.Contains(output, "port is already allocated")
		},
		GetNewCmd: func(command string, output string) []string {
			re := regexp.MustCompile(`(?i):(\d{2,5})`)
			match := re.FindStringSubmatch(output)
			if len(match) > 1 {
				port := match[1]
				return []string{fmt.Sprintf("kill -9 $(lsof -t -i:%s) && %s", port, command)}
			}
			return nil
		},
		Explanation: "Port is in use, attempt to kill the blocking process",
	},
	{
		Name: "go_run_directory",
		Match: func(command string, output string) bool {
			return command == "go run" || (strings.HasPrefix(command, "go run") && strings.Contains(output, "go run: no go files listed"))
		},
		GetNewCmd: func(command string, output string) []string {
			return []string{"go run ."}
		},
		Explanation: "Run all go files in the current directory",
	},
	{
		Name: "npm_missing_script",
		Match: func(command string, output string) bool {
			return strings.HasPrefix(command, "npm run") && strings.Contains(output, "Missing script:")
		},
		GetNewCmd: func(command string, output string) []string {
			re := regexp.MustCompile(`Did you mean one of these\?\n\s+([A-Za-z0-9_-]+)`)
			match := re.FindStringSubmatch(output)
			if len(match) > 1 {
				parts := strings.Fields(command)
				if len(parts) >= 3 {
					parts[2] = match[1]
					return []string{strings.Join(parts, " ")}
				}
			}
			return nil
		},
		Explanation: "Likely a typo in the npm script name",
	},
}

// evaluateErrorRules runs the command safely and uses the output to determine a 100% match correction based on known error patterns.
func (c *Corrector) evaluateErrorRules(command string) *Correction {
	// Skip for interactive commands or ones that might hang
	if looksLikeInteractive(command) {
		return nil
	}

	// We'll execute the given command with a short timeout to grab its error output.
	// This is the core engine to "kill" `thefuck` without reading history dynamically.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}

	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	// We only need the combined output (stdout and stderr)
	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	// If the command failed because the executable wasn't found at all,
	// rely on the typo corrector instead of rules.
	if err != nil && strings.Contains(err.Error(), "executable file not found") {
		return nil
	}

	// If the command actually succeeds, we don't need error evaluation correction unless it was completely silent
	if err == nil && outputStr == "" {
		return nil
	}

	// Iterate through all our defined rules to find a match
	for _, rule := range coreRules {
		if rule.Match(command, outputStr) {
			newCmds := rule.GetNewCmd(command, outputStr)
			if len(newCmds) > 0 {
				return &Correction{
					Original:    command,
					Corrected:   newCmds[0], // Present the highest confidence option
					Confidence:  1.0,        // 100% match based on concrete error output
					Explanation: "💡 Output Context: " + rule.Explanation,
					IsDangerous: false,
				}
			}
		}
	}

	return nil
}

// looksLikeInteractive skips commands like vim, nano, less, top that wait for user input
func looksLikeInteractive(command string) bool {
	interactiveCmds := []string{"vim", "nvim", "nano", "less", "more", "top", "htop", "btop", "ssh", "psql", "mysql", "irb", "python", "node"}
	root := strings.Fields(command)
	if len(root) == 0 {
		return true
	}
	r := strings.ToLower(root[0])
	for _, ic := range interactiveCmds {
		if r == ic {
			return true
		}
	}
	return false
}
