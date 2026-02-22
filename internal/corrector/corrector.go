// Package corrector provides intelligent command correction
package corrector

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
)

// Correction represents a suggested correction
type Correction struct {
	Original    string
	Corrected   string
	Confidence  float64
	Explanation string
	IsDangerous bool
}

// Corrector provides command correction functionality
type Corrector struct {
	// Common typos and their corrections
	commonTypos map[string]string

	// Command patterns that are often confused
	confusablePatterns map[string][]string

	// Dangerous patterns
	dangerousPatterns []string

	// Known commands from history
	historyCommands []string
}

// New creates a new command corrector
func New() *Corrector {
	c := &Corrector{
		commonTypos:        initializeCommonTypos(),
		confusablePatterns: initializeConfusablePatterns(),
		dangerousPatterns: []string{
			"rm -rf /",
			"rm -rf /*",
			"> /dev/sda",
			"mkfs.ext3 /dev/sda",
			"dd if=/dev/zero of=/dev/sda",
			":(){ :|:& };:",
			"chmod -R 777 /",
		},
	}
	return c
}

// SetHistoryCommands sets the known commands from history for better matching
func (c *Corrector) SetHistoryCommands(commands []string) {
	c.historyCommands = commands
}

// Correct analyzes a command and suggests corrections if needed
func (c *Corrector) Correct(command string) (*Correction, error) {
	// Check for dangerous commands first
	if dangerous := c.checkDangerous(command); dangerous != nil {
		return dangerous, nil
	}

	// Check common typos
	if typo := c.checkCommonTypos(command); typo != nil {
		return typo, nil
	}

	// Check for confusable patterns
	if confusable := c.checkConfusablePatterns(command); confusable != nil {
		return confusable, nil
	}

	// Check against history using fuzzy matching
	if history := c.checkHistory(command); history != nil {
		return history, nil
	}

	// No correction needed
	return nil, nil
}

// checkDangerous checks for dangerous commands
func (c *Corrector) checkDangerous(command string) *Correction {
	cmdLower := strings.ToLower(strings.TrimSpace(command))

	// Exact dangerous matches
	for _, pattern := range c.dangerousPatterns {
		if cmdLower == strings.ToLower(pattern) ||
			strings.HasPrefix(cmdLower, strings.ToLower(pattern)) {
			return &Correction{
				Original:    command,
				Corrected:   "",
				Confidence:  1.0,
				Explanation: fmt.Sprintf("⚠️  DANGEROUS COMMAND DETECTED: This command '%s' can destroy your system!", pattern),
				IsDangerous: true,
			}
		}
	}

	// Check for rm -rf patterns
	if matched, _ := regexp.MatchString(`(?i)rm\s+-rf\s+/?$`, command); matched {
		return &Correction{
			Original:    command,
			Corrected:   "",
			Confidence:  0.95,
			Explanation: "⚠️  WARNING: You are about to delete the root directory. Are you sure?",
			IsDangerous: true,
		}
	}

	// Check for destructive redirects
	if matched, _ := regexp.MatchString(`>\s*/dev/sd[a-z]`, command); matched {
		return &Correction{
			Original:    command,
			Corrected:   "",
			Confidence:  0.95,
			Explanation: "⚠️  WARNING: This will overwrite a disk device!",
			IsDangerous: true,
		}
	}

	return nil
}

// checkCommonTypos checks for common typos
func (c *Corrector) checkCommonTypos(command string) *Correction {
	// Check exact typo matches
	cmdLower := strings.ToLower(strings.TrimSpace(command))
	if correction, ok := c.commonTypos[cmdLower]; ok {
		return &Correction{
			Original:    command,
			Corrected:   correction,
			Confidence:  0.9,
			Explanation: fmt.Sprintf("Did you mean '%s'?", correction),
		}
	}

	// Check for swapped characters
	words := strings.Fields(command)
	if len(words) > 0 {
		firstWord := strings.ToLower(words[0])
		for typo, correct := range c.commonTypos {
			// Check Levenshtein distance for typos
			if distance := levenshtein.ComputeDistance(firstWord, typo); distance <= 1 {
				words[0] = correct
				return &Correction{
					Original:    command,
					Corrected:   strings.Join(words, " "),
					Confidence:  0.85 - float64(distance)*0.1,
					Explanation: fmt.Sprintf("'%s' looks like a typo. Did you mean '%s'?", firstWord, correct),
				}
			}
		}
	}

	return nil
}

// checkConfusablePatterns checks for commonly confused commands
func (c *Corrector) checkConfusablePatterns(command string) *Correction {
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil
	}

	firstWord := strings.ToLower(words[0])

	// Check if command is often confused with others
	if alternatives, ok := c.confusablePatterns[firstWord]; ok {
		// If this is the less common version, suggest the common one
		if len(alternatives) > 0 {
			return &Correction{
				Original:    command,
				Corrected:   strings.Replace(command, words[0], alternatives[0], 1),
				Confidence:  0.6,
				Explanation: fmt.Sprintf("'%s' is often confused with '%s'. Did you mean that?", firstWord, alternatives[0]),
			}
		}
	}

	// Check for missing 'git' prefix
	gitSubcommands := []string{"status", "add", "commit", "push", "pull", "branch", "log", "checkout", "merge", "rebase", "clone", "init"}
	if slices.Contains(gitSubcommands, firstWord) {
		return &Correction{
			Original:    command,
			Corrected:   "git " + command,
			Confidence:  0.75,
			Explanation: fmt.Sprintf("Did you forget 'git'? Try: git %s", command),
		}
	}

	// Check for docker/docker-compose confusion
	if firstWord == "compose" {
		return &Correction{
			Original:    command,
			Corrected:   "docker-compose " + strings.Join(words[1:], " "),
			Confidence:  0.8,
			Explanation: "Did you mean 'docker-compose'?",
		}
	}

	// Check for common missing flags
	if firstWord == "rm" && len(words) > 1 {
		// Check if trying to delete directory without -r
		for _, arg := range words[1:] {
			if !strings.HasPrefix(arg, "-") {
				// Check if it's a directory (simple heuristic)
				if strings.HasSuffix(arg, "/") {
					return &Correction{
						Original:    command,
						Corrected:   "rm -r " + strings.Join(words[1:], " "),
						Confidence:  0.7,
						Explanation: fmt.Sprintf("'%s' looks like a directory. Use 'rm -r' for directories.", arg),
					}
				}
			}
		}
	}

	return nil
}

// checkHistory checks command against history for similar commands
func (c *Corrector) checkHistory(command string) *Correction {
	if len(c.historyCommands) == 0 {
		return nil
	}

	bestMatch := ""
	bestDistance := 3 // Maximum acceptable distance

	for _, histCmd := range c.historyCommands {
		distance := levenshtein.ComputeDistance(command, histCmd)
		if distance < bestDistance && distance > 0 {
			bestDistance = distance
			bestMatch = histCmd
		}
	}

	if bestMatch != "" {
		confidence := 0.7 - float64(bestDistance)*0.1
		return &Correction{
			Original:    command,
			Corrected:   bestMatch,
			Confidence:  confidence,
			Explanation: fmt.Sprintf("Similar to a command in your history: '%s'", bestMatch),
		}
	}

	return nil
}

// FixTypo attempts to fix common typing errors in real-time
func (c *Corrector) FixTypo(input string) string {
	// Fix common adjacent key typos
	adjacentKeys := map[rune][]rune{
		'g': {'f', 'h', 't', 'v'},
		'i': {'u', 'o', 'k', 'j'},
		't': {'r', 'y', 'f', 'g'},
		// Add more as needed
	}

	// This is a simplified version - could be expanded
	_ = adjacentKeys
	return input
}

// SuggestAlternative suggests alternative commands based on intent
func (c *Corrector) SuggestAlternative(command string) []string {
	var suggestions []string
	words := strings.Fields(command)
	if len(words) == 0 {
		return suggestions
	}

	firstWord := strings.ToLower(words[0])

	// Suggest modern alternatives
	alternatives := map[string][]string{
		"ls":   {"exa", "lsd"},
		"cat":  {"bat", "batcat"},
		"find": {"fd"},
		"grep": {"ripgrep", "rg"},
		"ps":   {"procs"},
		"top":  {"htop", "btop"},
		"du":   {"dust"},
		"df":   {"duf"},
	}

	if alts, ok := alternatives[firstWord]; ok {
		suggestions = append(suggestions, alts...)
	}

	return suggestions
}

// initializeCommonTypos initializes common typo mappings
func initializeCommonTypos() map[string]string {
	return map[string]string{
		// Git typos
		"gti":          "git",
		"tit":          "git",
		"gi":           "git",
		"gt":           "git",
		"gut":          "git",
		"gitp ull":     "git pull",
		"gitp uhs":     "git push",
		"git satus":    "git status",
		"git stauts":   "git status",
		"git commti":   "git commit",
		"git chekcout": "git checkout",

		// Docker typos
		"docer":   "docker",
		"doccker": "docker",
		"doker":   "docker",
		"dcoekr":  "docker",

		// Common command typos
		"sl":      "ls",
		"ks":      "ls",
		"lss":     "ls",
		"cd..":    "cd ..",
		"cd-":     "cd -",
		"grpe":    "grep",
		"grp":     "grep",
		"gr":      "grep",
		"tial":    "tail",
		"taill":   "tail",
		"cAT":     "cat",
		"mkr":     "mkdir",
		"makedir": "mkdir",
		"mkidr":   "mkdir",

		// Package manager typos
		"npn":      "npm",
		"nom":      "npm",
		"pni":      "npm install",
		" isntall": " install",

		// Go typos
		"go bulid":   "go build",
		"go buld":    "go build",
		"go tset":    "go test",
		"go isntall": "go install",

		// Python typos
		"pthon": "python",
		"pyton": "python",
		"pyp":   "pip",
		"pp":    "pip",
	}
}

// initializeConfusablePatterns initializes commonly confused patterns
func initializeConfusablePatterns() map[string][]string {
	return map[string][]string{
		// Commands that are often confused with each other
		"docker":  {"docker-compose", "docker compose"},
		"compose": {"docker-compose"},
		"kubectl": {"kubectx", "kubens"},
		"pip":     {"pip3"},
		"python":  {"python3"},
		"node":    {"nodejs"},
		"code":    {"code ."},
	}
}
