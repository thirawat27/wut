package corrector

// ──────────────────────────────────────────────────────────────────────────────
// Short-flag Cluster Correction
//
// Handles flag clusters like -it, -aux, -rf where each character is an
// independent short flag. The corrector:
//   1. Parses clusters into individual characters
//   2. Validates each character against known short flags for the root command
//   3. Suggests long-option equivalents for unknown characters
//   4. Identifies likely typos vs intentional flag combinations
// ──────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"strings"
)

// shortFlagInfo holds the long-option equivalent and description for a short flag.
type shortFlagInfo struct {
	LongOption  string
	Description string
}

// shortFlagMap maps root command → (short flag char → info)
var shortFlagMap = map[string]map[string]shortFlagInfo{
	"docker": {
		"i": {LongOption: "--interactive", Description: "Keep STDIN open"},
		"t": {LongOption: "--tty", Description: "Allocate a pseudo-TTY"},
		"d": {LongOption: "--detach", Description: "Run container in background"},
		"p": {LongOption: "--publish", Description: "Publish a container's port"},
		"v": {LongOption: "--volume", Description: "Bind mount a volume"},
		"e": {LongOption: "--env", Description: "Set environment variable"},
		"u": {LongOption: "--user", Description: "Username or UID"},
		"w": {LongOption: "--workdir", Description: "Working directory inside container"},
		"h": {LongOption: "--hostname", Description: "Container host name"},
		"m": {LongOption: "--memory", Description: "Memory limit"},
		"n": {LongOption: "--name", Description: "Assign a name to the container"},
		"q": {LongOption: "--quiet", Description: "Suppress output"},
		"f": {LongOption: "--force", Description: "Force the operation"},
		"a": {LongOption: "--all", Description: "Show/act on all items"},
	},
	"git": {
		"a": {LongOption: "--all", Description: "Stage all changes"},
		"m": {LongOption: "--message", Description: "Commit message"},
		"u": {LongOption: "--set-upstream", Description: "Set upstream tracking branch"},
		"v": {LongOption: "--verbose", Description: "Verbose output"},
		"q": {LongOption: "--quiet", Description: "Suppress output"},
		"f": {LongOption: "--force", Description: "Force the operation"},
		"b": {LongOption: "--branch", Description: "Checkout new branch"},
		"p": {LongOption: "--patch", Description: "Interactive patch mode"},
		"n": {LongOption: "--no-ff", Description: "No fast-forward merge"},
		"r": {LongOption: "--rebase", Description: "Rebase instead of merge"},
		"s": {LongOption: "--squash", Description: "Squash commits"},
		"t": {LongOption: "--tags", Description: "Include tags"},
		"d": {LongOption: "--delete", Description: "Delete branch"},
		"D": {LongOption: "--delete --force", Description: "Force delete branch"},
	},
	"kubectl": {
		"n": {LongOption: "--namespace", Description: "Namespace"},
		"o": {LongOption: "--output", Description: "Output format (json/yaml/wide)"},
		"f": {LongOption: "--filename", Description: "Filename/directory to apply"},
		"A": {LongOption: "--all-namespaces", Description: "All namespaces"},
		"w": {LongOption: "--watch", Description: "Watch for changes"},
		"l": {LongOption: "--selector", Description: "Label selector"},
		"v": {LongOption: "--verbose", Description: "Verbosity level"},
	},
	"tar": {
		"x": {LongOption: "--extract", Description: "Extract files"},
		"c": {LongOption: "--create", Description: "Create archive"},
		"z": {LongOption: "--gzip", Description: "Filter through gzip"},
		"j": {LongOption: "--bzip2", Description: "Filter through bzip2"},
		"v": {LongOption: "--verbose", Description: "List processed files"},
		"f": {LongOption: "--file", Description: "Archive file"},
		"t": {LongOption: "--list", Description: "List contents"},
		"r": {LongOption: "--append", Description: "Append files"},
		"u": {LongOption: "--update", Description: "Update archive"},
		"C": {LongOption: "--directory", Description: "Change to directory"},
	},
	"ls": {
		"a": {LongOption: "--all", Description: "Show hidden files"},
		"l": {LongOption: "--format=long", Description: "Long listing format"},
		"h": {LongOption: "--human-readable", Description: "Human-readable sizes"},
		"r": {LongOption: "--reverse", Description: "Reverse sort order"},
		"t": {LongOption: "--sort=time", Description: "Sort by modification time"},
		"s": {LongOption: "--size", Description: "Print size of each file"},
		"R": {LongOption: "--recursive", Description: "List subdirectories recursively"},
		"S": {LongOption: "--sort=size", Description: "Sort by file size"},
		"1": {LongOption: "--format=single-column", Description: "One file per line"},
	},
	"grep": {
		"i": {LongOption: "--ignore-case", Description: "Case-insensitive search"},
		"r": {LongOption: "--recursive", Description: "Search recursively"},
		"n": {LongOption: "--line-number", Description: "Show line numbers"},
		"v": {LongOption: "--invert-match", Description: "Invert match"},
		"l": {LongOption: "--files-with-matches", Description: "Print matching filenames"},
		"c": {LongOption: "--count", Description: "Count matching lines"},
		"o": {LongOption: "--only-matching", Description: "Print only matching part"},
		"w": {LongOption: "--word-regexp", Description: "Match whole words"},
		"E": {LongOption: "--extended-regexp", Description: "Extended regex"},
		"A": {LongOption: "--after-context", Description: "Lines after match"},
		"B": {LongOption: "--before-context", Description: "Lines before match"},
		"q": {LongOption: "--quiet", Description: "Suppress output"},
	},
	"curl": {
		"X": {LongOption: "--request", Description: "HTTP method"},
		"H": {LongOption: "--header", Description: "Custom header"},
		"d": {LongOption: "--data", Description: "POST data"},
		"o": {LongOption: "--output", Description: "Write output to file"},
		"O": {LongOption: "--remote-name", Description: "Write to filename from URL"},
		"L": {LongOption: "--location", Description: "Follow redirects"},
		"I": {LongOption: "--head", Description: "Fetch headers only"},
		"v": {LongOption: "--verbose", Description: "Verbose mode"},
		"s": {LongOption: "--silent", Description: "Silent mode"},
		"k": {LongOption: "--insecure", Description: "Skip TLS verification"},
		"u": {LongOption: "--user", Description: "User:password"},
		"c": {LongOption: "--cookie", Description: "Send cookie"},
		"f": {LongOption: "--fail", Description: "Fail on HTTP errors"},
	},
	"ssh": {
		"i": {LongOption: "--identity", Description: "Identity file (private key)"},
		"p": {LongOption: "--port", Description: "Port number"},
		"L": {LongOption: "--local-forward", Description: "Local port forwarding"},
		"R": {LongOption: "--remote-forward", Description: "Remote port forwarding"},
		"D": {LongOption: "--dynamic-forward", Description: "Dynamic port forwarding"},
		"N": {LongOption: "--no-shell", Description: "No remote commands (forwarding only)"},
		"v": {LongOption: "--verbose", Description: "Verbose mode"},
		"q": {LongOption: "--quiet", Description: "Quiet mode"},
		"A": {LongOption: "--forward-agent", Description: "Forward agent connection"},
		"X": {LongOption: "--forward-x11", Description: "Forward X11"},
		"t": {LongOption: "--request-tty", Description: "Force TTY allocation"},
	},
	"find": {
		"L": {LongOption: "--follow", Description: "Follow symbolic links"},
		"P": {LongOption: "--no-follow", Description: "Never follow symbolic links"},
		"H": {LongOption: "--follow-args", Description: "Follow symlinks only for args"},
	},
	"npm": {
		"g": {LongOption: "--global", Description: "Install globally"},
		"D": {LongOption: "--save-dev", Description: "Save as devDependency"},
		"E": {LongOption: "--save-exact", Description: "Save exact version"},
		"S": {LongOption: "--save", Description: "Save to dependencies"},
		"y": {LongOption: "--yes", Description: "Automatic yes to prompts"},
	},
	"ps": {
		"a": {LongOption: "--all", Description: "All processes (same terminal)"},
		"u": {LongOption: "--user", Description: "User-oriented format"},
		"x": {LongOption: "--no-tty", Description: "Include processes without TTY"},
		"e": {LongOption: "--everyone", Description: "All processes"},
		"f": {LongOption: "--full", Description: "Full format listing"},
		"l": {LongOption: "--long", Description: "Long format"},
	},
	"chmod": {
		"R": {LongOption: "--recursive", Description: "Change recursively"},
		"v": {LongOption: "--verbose", Description: "Verbose output"},
		"c": {LongOption: "--changes", Description: "Report changes"},
		"f": {LongOption: "--silent", Description: "Suppress error messages"},
	},
	"rsync": {
		"a": {LongOption: "--archive", Description: "Archive mode (recursive + preserve)"},
		"v": {LongOption: "--verbose", Description: "Verbose"},
		"z": {LongOption: "--compress", Description: "Compress during transfer"},
		"r": {LongOption: "--recursive", Description: "Recursive"},
		"n": {LongOption: "--dry-run", Description: "Dry run"},
		"e": {LongOption: "--rsh", Description: "Remote shell to use"},
		"P": {LongOption: "--progress --partial", Description: "Progress + partial transfers"},
		"h": {LongOption: "--human-readable", Description: "Human-readable sizes"},
		"u": {LongOption: "--update", Description: "Skip files newer on receiver"},
		"x": {LongOption: "--one-file-system", Description: "Don't cross filesystem boundaries"},
	},
}

// ShortFlagClusterResult describes the analysis of a short flag cluster.
type ShortFlagClusterResult struct {
	Original     string            // e.g. "-it"
	Expansion    string            // "--interactive --tty"
	UnknownFLags []string          // flag chars with no known mapping
	Annotations  map[string]string // char → description
}

// AnalyseShortFlagCluster parses a flag cluster (e.g. "-auxf") and returns
// the long-form equivalents and any unknown flags for the given root command.
func AnalyseShortFlagCluster(root, flagCluster string) (*ShortFlagClusterResult, bool) {
	// Must be a cluster: starts with '-' but NOT '--', length > 2 (e.g. "-it")
	if !strings.HasPrefix(flagCluster, "-") || strings.HasPrefix(flagCluster, "--") {
		return nil, false
	}
	chars := []rune(strings.TrimPrefix(flagCluster, "-"))
	if len(chars) < 2 {
		return nil, false // single short flag; not a cluster
	}

	knownMap := shortFlagMap[root]
	if knownMap == nil {
		return nil, false // no data for this command
	}

	result := &ShortFlagClusterResult{
		Original:    flagCluster,
		Annotations: map[string]string{},
	}

	var longParts []string
	for _, ch := range chars {
		key := string(ch)
		if info, ok := knownMap[key]; ok {
			longParts = append(longParts, info.LongOption)
			result.Annotations[key] = info.Description
		} else {
			// Flag char is not in the corpus → potential typo
			result.UnknownFLags = append(result.UnknownFLags, key)
		}
	}

	if len(longParts) == 0 {
		return nil, false
	}

	result.Expansion = strings.Join(longParts, " ")
	return result, true
}

// correctShortFlagClusters walks the token list looking for short flag clusters
// and returns any corrections or explanations found.
func correctShortFlagClusters(root string, tokens []string) []tokenFix {
	var fixes []tokenFix
	for _, tok := range tokens {
		if !strings.HasPrefix(tok, "-") || strings.HasPrefix(tok, "--") {
			continue
		}
		if len(tok) <= 2 {
			continue // single short flag; skip
		}

		result, ok := AnalyseShortFlagCluster(root, tok)
		if !ok {
			continue
		}

		if len(result.UnknownFLags) > 0 {
			// Cluster has unknown chars – suggest the expanded equivalents
			// so the user can identify the typo
			suggestion := result.Expansion
			if suggestion != "" {
				fixes = append(fixes, tokenFix{
					original:  tok,
					corrected: suggestion,
					distance:  1,
				})
			}
		}
		// If all flags are known and there are no unknowns, no correction needed
	}
	return fixes
}

// ExplainShortFlagCluster returns a human-readable expansion of a flag cluster.
// Example: docker, "-it" → "--interactive (Keep STDIN open) --tty (Allocate a pseudo-TTY)"
func ExplainShortFlagCluster(root, flagCluster string) string {
	_, ok := AnalyseShortFlagCluster(root, flagCluster)
	if !ok {
		return ""
	}

	stripped := strings.TrimPrefix(flagCluster, "-")
	var parts []string
	knownMap := shortFlagMap[root]
	for _, ch := range stripped {
		key := string(ch)
		if info, ok := knownMap[key]; ok {
			parts = append(parts, fmt.Sprintf("%s (%s)", info.LongOption, info.Description))
		} else {
			parts = append(parts, fmt.Sprintf("-%s (unknown)", key))
		}
	}
	return strings.Join(parts, "  ")
}
