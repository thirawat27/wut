// Package corrector provides intelligent, corpus-based command correction.
// It performs token-by-token, context-aware correction using Levenshtein
// distance against a large corpus of known valid commands and subcommands.
package corrector

import (
	"fmt"
	"regexp"
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

// tokenFix records a single token correction
type tokenFix struct {
	original  string
	corrected string
	distance  int
}

// Corrector provides command correction functionality
type Corrector struct {
	dangerousPatterns []string
	historyCommands   []string
}

// New creates a new Corrector.
func New() *Corrector {
	return &Corrector{
		dangerousPatterns: dangerousList,
	}
}

// SetHistoryCommands supplies past commands for additional fuzzy matching.
func (c *Corrector) SetHistoryCommands(cmds []string) {
	c.historyCommands = cmds
}

// ──────────────────────────────────────────────────────────────────────────────
// Public API
// ──────────────────────────────────────────────────────────────────────────────

// Correct analyzes the full command sentence and returns a Correction if any
// token is misspelled, or nil when no issues are detected.
func (c *Corrector) Correct(command string) (*Correction, error) {
	// 1. Safety check first
	if d := c.checkDangerous(command); d != nil {
		return d, nil
	}

	// 2. Full-sentence, context-aware typo scan
	if fix := c.correctSentence(command); fix != nil {
		return fix, nil
	}

	// 3. History-based full-sentence fuzzy match
	if h := c.checkHistory(command); h != nil {
		return h, nil
	}

	return nil, nil
}

// SuggestAlternative returns modern tool alternatives for a given command.
func (c *Corrector) SuggestAlternative(command string) []string {
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil
	}
	return modernAlternatives[strings.ToLower(words[0])]
}

// ──────────────────────────────────────────────────────────────────────────────
// Core correction logic
// ──────────────────────────────────────────────────────────────────────────────

// correctSentence performs per-token correction using the corpus.
// It is context-aware: the subcommand corpus is chosen based on the root command.
func (c *Corrector) correctSentence(command string) *Correction {
	tokens := strings.Fields(command)
	if len(tokens) == 0 {
		return nil
	}

	corrected := make([]string, len(tokens))
	copy(corrected, tokens)

	var fixes []tokenFix
	totalScore := 0.0

	// ── Token 0: root command ──────────────────────────────────────────────
	root := strings.ToLower(tokens[0])
	bestRoot, bestDist := bestMatch(root, rootCommandCorpus(), maxDistForLen(root))
	if bestRoot != "" && bestRoot != root {
		fixes = append(fixes, tokenFix{tokens[0], bestRoot, bestDist})
		corrected[0] = bestRoot
		totalScore += confidenceScore(root, bestDist)
	} else {
		bestRoot = root // root is fine; use it for subcommand context
	}

	// ── Tokens 1…n: subcommands + args ────────────────────────────────────
	subCorpus := subcommandCorpus(bestRoot)
	flagCorpora := flagCorpus(bestRoot)

	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]

		// ── Flags (starts with - or --) ─────────────────────────────────
		if strings.HasPrefix(tok, "-") {
			// Try to correct the flag using the per-command flag corpus
			if len(flagCorpora.long) > 0 {
				var cleanTok, prefix string
				var knownFlags []string
				if strings.HasPrefix(tok, "--") {
					// long flag: strip --, get name before =
					cleanTok = strings.TrimPrefix(tok, "--")
					if eqIdx := strings.Index(cleanTok, "="); eqIdx != -1 {
						cleanTok = cleanTok[:eqIdx]
					}
					prefix = "--"
					knownFlags = flagCorpora.long
				} else if len(tok) > 2 {
					// short flag cluster like -it, -rf; skip
					continue
				} else {
					// single short flag like -v; skip
					continue
				}

				cleanTokLower := strings.ToLower(cleanTok)
				bestFlag, flagDist := bestMatch(cleanTokLower, knownFlags, maxDistForLen(cleanTokLower))
				if bestFlag != "" && bestFlag != cleanTokLower {
					newTok := prefix + bestFlag
					fixes = append(fixes, tokenFix{tok, newTok, flagDist})
					corrected[i] = newTok
					totalScore += confidenceScore(cleanTokLower, flagDist)
				}
			}
			// Flag not correctable or no corpus: leave as-is
			continue
		}

		// Skip things that look like paths or URLs
		if looksLikePathOrURL(tok) {
			continue
		}
		// Skip numeric-only tokens (port numbers, counts, etc.)
		if isNumeric(tok) {
			continue
		}

		tokLower := strings.ToLower(tok)
		maxDist := maxDistForLen(tokLower)

		var best string
		var dist int

		if i == 1 && len(subCorpus) > 0 {
			// First argument: try subcommand corpus first
			best, dist = bestMatch(tokLower, subCorpus, maxDist)
		}

		if best == "" {
			// Fallback: check the global token corpus
			best, dist = bestMatch(tokLower, globalTokenCorpus(), maxDist)
		}

		if best != "" && best != tokLower {
			// Preserve original capitalisation style if possible
			out := best
			if isAllUpper(tok) {
				out = strings.ToUpper(best)
			}
			fixes = append(fixes, tokenFix{tok, out, dist})
			corrected[i] = out
			totalScore += confidenceScore(tokLower, dist)
		}
	}

	if len(fixes) == 0 {
		return nil
	}

	// Missing-prefix check (e.g. "status" → "git status")
	if misfix := c.checkMissingPrefix(command); misfix != nil && len(fixes) == 0 {
		return misfix
	}

	avgConf := totalScore / float64(len(fixes))
	var explParts []string
	for _, f := range fixes {
		explParts = append(explParts, fmt.Sprintf("'%s'→'%s'", f.original, f.corrected))
	}
	explanation := "Fixed: " + strings.Join(explParts, ", ")

	return &Correction{
		Original:    command,
		Corrected:   strings.Join(corrected, " "),
		Confidence:  avgConf,
		Explanation: explanation,
	}
}

// checkMissingPrefix detects git/docker subcommands used without their parent.
func (c *Corrector) checkMissingPrefix(command string) *Correction {
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil
	}
	first := strings.ToLower(words[0])

	type prefixRule struct {
		corpus []string
		prefix string
	}
	rules := []prefixRule{
		{gitSubcommands, "git"},
		{dockerSubcommands, "docker"},
		{kubectlSubcommands, "kubectl"},
	}
	for _, r := range rules {
		for _, sub := range r.corpus {
			if first == sub {
				return &Correction{
					Original:    command,
					Corrected:   r.prefix + " " + command,
					Confidence:  0.78,
					Explanation: fmt.Sprintf("Did you forget '%s'? Try: %s %s", r.prefix, r.prefix, command),
				}
			}
		}
	}
	return nil
}

// checkDangerous flags destructive commands with a high-confidence warning.
func (c *Corrector) checkDangerous(command string) *Correction {
	cmdLower := strings.ToLower(strings.TrimSpace(command))
	for _, pattern := range c.dangerousPatterns {
		p := strings.ToLower(pattern)
		if cmdLower == p || strings.HasPrefix(cmdLower, p) {
			return &Correction{
				Original:    command,
				Corrected:   "",
				Confidence:  1.0,
				Explanation: fmt.Sprintf("⚠️  DANGEROUS: '%s' can destroy your system!", pattern),
				IsDangerous: true,
			}
		}
	}
	if ok, _ := regexp.MatchString(`(?i)rm\s+-rf\s+/?$`, command); ok {
		return &Correction{Original: command, Corrected: "", Confidence: 0.95,
			Explanation: "⚠️  This deletes the root directory!", IsDangerous: true}
	}
	if ok, _ := regexp.MatchString(`>\s*/dev/sd[a-z]`, command); ok {
		return &Correction{Original: command, Corrected: "", Confidence: 0.95,
			Explanation: "⚠️  This overwrites a disk device!", IsDangerous: true}
	}
	return nil
}

// checkHistory fuzzy-matches the full sentence against previously used commands.
func (c *Corrector) checkHistory(command string) *Correction {
	if len(c.historyCommands) == 0 {
		return nil
	}
	best := ""
	bestDist := 5
	for _, h := range c.historyCommands {
		d := levenshtein.ComputeDistance(command, h)
		if d > 0 && d < bestDist {
			bestDist = d
			best = h
		}
	}
	if best == "" {
		return nil
	}
	return &Correction{
		Original:    command,
		Corrected:   best,
		Confidence:  0.65 - float64(bestDist)*0.07,
		Explanation: fmt.Sprintf("Similar to a past command: '%s'", best),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Flag Corpus
// ──────────────────────────────────────────────────────────────────────────────

// flagSet holds the known long flags for a command.
type flagSet struct {
	long []string // without leading --
}

// flagCorpus returns known flags for the given root command.
// Returning an empty flagSet means "no flag corpus; don't attempt flag correction".
func flagCorpus(root string) flagSet {
	m := map[string]flagSet{
		"docker": {
			long: []string{
				"privileged", "interactive", "tty", "detach", "rm",
				"name", "hostname", "env", "volume", "mount", "network",
				"publish", "expose", "platform", "restart", "entrypoint",
				"workdir", "user", "memory", "cpus", "label", "link",
				"cap-add", "cap-drop", "device", "runtime", "no-cache",
				"build-arg", "tag", "file", "target", "squash", "quiet",
				"force", "all", "filter", "format", "follow", "tail",
			},
		},
		"docker-compose": {
			long: []string{
				"detach", "build", "no-build", "force-recreate", "no-recreate",
				"no-start", "no-deps", "scale", "timeout", "volumes",
				"remove-orphans", "quiet", "project-name", "file",
			},
		},
		"git": {
			long: []string{
				"all", "amend", "author", "branch", "cached", "color",
				"message", "no-ff", "no-rebase", "oneline", "patch",
				"prune", "quiet", "rebase", "recurse-submodules", "remote",
				"set-upstream", "soft", "hard", "mixed", "staged", "stat",
				"tags", "track", "upstream", "verbose", "word-diff", "force",
				"force-with-lease", "continue", "abort", "skip", "interactive",
				"dry-run", "no-edit", "signoff", "squash", "autostash",
			},
		},
		"kubectl": {
			long: []string{
				"all-namespaces", "namespace", "output", "selector", "filename",
				"recursive", "dry-run", "force", "grace-period", "cascade",
				"wait", "timeout", "watch", "context", "cluster", "user",
				"kubeconfig", "server", "token", "insecure-skip-tls-verify",
				"container", "stdin", "tty", "replicas", "image", "port",
				"labels", "annotations", "type", "from-literal", "from-file",
				"record", "overwrite", "show-labels", "sort-by", "field-selector",
			},
		},
		"npm": {
			long: []string{
				"save", "save-dev", "save-exact", "global", "legacy-peer-deps",
				"no-save", "prefer-offline", "no-package-lock", "dry-run",
				"verbose", "quiet", "audit", "fund", "production",
				"ignore-scripts", "force", "prefix", "workspace", "workspaces",
			},
		},
		"go": {
			long: []string{
				"verbose", "race", "count", "timeout", "run", "bench",
				"benchtime", "cover", "coverprofile", "output", "tags",
				"ldflags", "gcflags", "trimpath", "mod", "work",
				"json", "list", "short", "failfast", "parallel",
			},
		},
		"terraform": {
			long: []string{
				"auto-approve", "compact-warnings", "destroy", "detailed-exitcode",
				"input", "json", "lock", "lock-timeout", "no-color",
				"out", "parallelism", "plan", "refresh", "refresh-only",
				"replace", "state", "state-out", "target", "var", "var-file",
			},
		},
		"curl": {
			long: []string{
				"request", "header", "data", "data-raw", "data-binary",
				"output", "location", "silent", "verbose", "insecure",
				"user", "cookie", "cookie-jar", "upload-file", "form",
				"compressed", "max-time", "connect-timeout", "retry",
				"proxy", "user-agent", "referer", "include", "head",
				"basic", "digest", "oauth2-bearer", "ntlm",
			},
		},
		"ssh": {
			long: []string{
				"identity", "port", "jump", "local-forward", "remote-forward",
				"dynamic-forward", "no-shell", "verbose", "quiet",
				"option", "compression", "cipher", "mac",
				"proxy-command", "request-tty", "no-remote-command",
			},
		},
		"find": {
			long: []string{
				"name", "iname", "type", "size", "mtime", "atime", "ctime",
				"newer", "empty", "exec", "execdir", "print", "print0",
				"maxdepth", "mindepth", "not", "and", "or", "delete",
				"prune", "regex", "follow", "links",
			},
		},
		"grep": {
			long: []string{
				"extended-regexp", "fixed-strings", "perl-regexp", "line-number",
				"with-filename", "no-filename", "recursive", "include", "exclude",
				"ignore-case", "invert-match", "word-regexp", "line-regexp",
				"count", "only-matching", "quiet", "silent", "color",
				"before-context", "after-context", "context", "max-count",
				"binary-files", "text",
			},
		},
		"systemctl": {
			long: []string{
				"type", "state", "all", "recursive", "no-block", "quiet",
				"full", "runtime", "global", "no-pager", "plain",
				"no-legend", "failed",
			},
		},
		"apt": {
			long: []string{
				"yes", "assume-yes", "quiet", "verbose", "no-install-recommends",
				"install-suggests", "fix-broken", "fix-missing", "ignore-missing",
				"allow-unauthenticated", "allow-downgrades", "reinstall",
				"purge", "auto-remove", "dry-run", "simulate",
			},
		},
		"wut": {
			long: []string{
				"list", "copy", "exec", "raw", "quiet", "offline",
				"limit", "stats", "search", "import", "export",
				"clear", "all", "force", "get", "set", "value",
				"edit", "reset", "shell", "debug", "help", "version",
			},
		},
	}
	if fs, ok := m[root]; ok {
		return fs
	}
	return flagSet{}
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// bestMatch finds the closest string in corpus within maxDist.
// Returns ("", 0) when nothing is close enough.
func bestMatch(token string, corpus []string, maxDist int) (string, int) {
	best := ""
	bestDist := maxDist + 1
	for _, candidate := range corpus {
		d := levenshtein.ComputeDistance(token, candidate)
		if d == 0 {
			return "", 0 // exact match → no correction needed
		}
		if d < bestDist {
			bestDist = d
			best = candidate
		}
	}
	if bestDist > maxDist {
		return "", 0
	}
	return best, bestDist
}

// maxDistForLen returns the acceptable edit distance based on token length.
// Short tokens tolerate only 1 edit; longer tokens tolerate up to 3.
func maxDistForLen(s string) int {
	n := len(s)
	switch {
	case n <= 3:
		return 1
	case n <= 6:
		return 2
	default:
		return 3
	}
}

// confidenceScore converts edit distance to a [0,1] confidence value.
func confidenceScore(original string, dist int) float64 {
	ratio := float64(dist) / float64(len(original)+1)
	score := 1.0 - ratio*1.5
	if score < 0.3 {
		score = 0.3
	}
	return score
}

func isNumeric(s string) bool  { return regexp.MustCompile(`^\d+$`).MatchString(s) }
func isAllUpper(s string) bool { return s == strings.ToUpper(s) }
func looksLikePathOrURL(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") || strings.HasPrefix(s, "~") ||
		strings.Contains(s, "://") || strings.HasPrefix(s, "http")
}

// ──────────────────────────────────────────────────────────────────────────────
// Corpora
// ──────────────────────────────────────────────────────────────────────────────

var dangerousList = []string{
	"rm -rf /", "rm -rf /*", "> /dev/sda", "mkfs.ext3 /dev/sda",
	"dd if=/dev/zero of=/dev/sda", ":(){ :|:& };:", "chmod -R 777 /",
}

// rootCommandCorpus returns all known root-level commands.
func rootCommandCorpus() []string {
	return []string{
		// Version control
		"git", "svn", "hg", "fossil",
		// Containers / orchestration
		"docker", "podman", "kubectl", "helm", "k9s", "k3s",
		"docker-compose", "skaffold", "kustomize",
		// Cloud CLIs
		"aws", "az", "gcloud", "terraform", "pulumi", "ansible",
		// Package managers
		"npm", "yarn", "pnpm", "npx", "pip", "pip3", "conda",
		"gem", "cargo", "go", "mvn", "gradle", "composer",
		"apt", "apt-get", "yum", "dnf", "pacman", "brew", "choco",
		// Runtimes / interpreters
		"node", "python", "python3", "ruby", "java", "php",
		"perl", "lua", "dart", "swift", "rustc", "javac",
		// Shell / file operations
		"ls", "ll", "la", "cat", "echo", "head", "tail", "less",
		"more", "grep", "rg", "find", "fd", "sed", "awk",
		"cut", "sort", "uniq", "wc", "diff", "patch",
		"cp", "mv", "rm", "mkdir", "rmdir", "touch", "ln",
		"chmod", "chown", "chgrp", "stat", "file",
		"tar", "zip", "unzip", "gzip", "gunzip", "bzip2",
		// System
		"ps", "top", "htop", "kill", "killall", "systemctl",
		"service", "journalctl", "lsof", "netstat", "ss", "ip",
		"ifconfig", "ping", "curl", "wget", "ssh", "scp", "rsync",
		"mount", "umount", "df", "du", "free",
		// Editors / tools
		"vim", "nvim", "nano", "emacs", "code", "subl",
		"make", "cmake", "gcc", "g++", "clang", "ld",
		"gdb", "lldb", "strace", "ltrace", "valgrind",
		// Misc dev
		"jq", "yq", "fzf", "bat", "htop", "btop", "exa", "lsd",
		"tmux", "screen", "nohup", "cron", "crontab",
		"openssl", "gpg", "pass", "1password",
		// Database clients
		"mysql", "psql", "mongo", "redis-cli", "sqlite3",
		// WUT own commands
		"wut",
	}
}

// subcommandCorpus returns the valid subcommands for a given root command.
func subcommandCorpus(root string) []string {
	m := map[string][]string{
		"git":       gitSubcommands,
		"docker":    dockerSubcommands,
		"kubectl":   kubectlSubcommands,
		"helm":      {"install", "uninstall", "upgrade", "rollback", "list", "repo", "search", "pull", "push", "create", "package", "lint", "template", "dependency", "status", "history"},
		"npm":       {"install", "uninstall", "update", "run", "start", "test", "build", "publish", "link", "init", "list", "outdated", "audit", "ci", "pack", "login", "logout", "version"},
		"yarn":      {"install", "add", "remove", "upgrade", "run", "start", "test", "build", "publish", "link", "init", "list", "outdated", "audit", "version", "workspace"},
		"pip":       {"install", "uninstall", "list", "show", "freeze", "download", "wheel", "hash", "check", "config", "index", "inspect"},
		"pip3":      {"install", "uninstall", "list", "show", "freeze", "download", "check", "config"},
		"go":        {"build", "run", "test", "get", "install", "mod", "generate", "fmt", "vet", "lint", "clean", "env", "version", "doc", "tool", "work"},
		"cargo":     {"build", "run", "test", "check", "install", "uninstall", "publish", "update", "init", "new", "add", "remove", "doc", "bench", "fmt", "clippy"},
		"terraform": {"init", "plan", "apply", "destroy", "show", "state", "import", "output", "validate", "fmt", "workspace", "providers", "refresh"},
		"aws":       {"s3", "ec2", "iam", "lambda", "rds", "cloudformation", "ecs", "eks", "route53", "ssm", "sts", "configure", "logs"},
		"gcloud":    {"compute", "container", "iam", "storage", "run", "functions", "sql", "dns", "auth", "config", "projects", "logging"},
		"az":        {"vm", "aks", "storage", "network", "group", "login", "logout", "account", "devops", "webapp"},
		"systemctl": {"start", "stop", "restart", "reload", "enable", "disable", "status", "is-active", "is-enabled", "mask", "unmask", "list-units", "daemon-reload"},
		"apt":       {"install", "remove", "purge", "update", "upgrade", "autoremove", "search", "show", "list", "clean", "autoclean"},
		"apt-get":   {"install", "remove", "purge", "update", "upgrade", "autoremove", "clean", "autoclean", "dist-upgrade"},
		"brew":      {"install", "uninstall", "update", "upgrade", "list", "info", "search", "tap", "untap", "link", "unlink", "doctor", "cleanup"},
		"vim":       {"--version", "-p", "-o", "-O", "-n", "-R", "-b"},
		"ssh":       {"-i", "-p", "-L", "-R", "-D", "-N", "-v", "-vvv"},
		"tar":       {"-xf", "-xzf", "-xjf", "-cf", "-czf", "-cjf", "-tf", "-tzf"},
		"curl":      {"-X", "-H", "-d", "-o", "-O", "-L", "-I", "-v", "-k", "--data", "--header"},
		"find":      {"-name", "-type", "-size", "-mtime", "-exec", "-maxdepth", "-mindepth", "-not", "-iname"},
		"wut":       {"suggest", "fix", "explain", "smart", "history", "alias", "config", "db", "install", "bookmark", "stats", "undo", "init"},
	}
	if subs, ok := m[root]; ok {
		return subs
	}
	return nil
}

// globalTokenCorpus are common words that appear as arguments across commands.
func globalTokenCorpus() []string {
	return []string{
		// status/state words
		"status", "install", "uninstall", "update", "upgrade", "remove",
		"delete", "create", "deploy", "build", "run", "start", "stop",
		"restart", "reload", "enable", "disable", "list", "show", "get",
		"apply", "destroy", "plan", "init", "sync", "push", "pull",
		"clone", "fetch", "merge", "rebase", "checkout", "branch",
		"commit", "add", "diff", "log", "stash", "tag", "reset",
		"revert", "cherry-pick", "bisect", "blame", "archive",
		// docker-specific
		"images", "containers", "networks", "volumes", "services",
		"exec", "logs", "inspect", "pull", "push", "tag", "image",
		"container", "network", "volume", "system", "compose",
		// general verbs
		"search", "test", "format", "lint", "clean", "check",
		"generate", "package", "publish", "release", "version",
		"login", "logout", "config", "configure", "setup",
		"import", "export", "output", "input", "migrate",
		"backup", "restore", "dump", "load", "seed",
		// file ops
		"copy", "move", "rename", "mkdir", "touch", "link",
		"chmod", "chown", "compress", "extract", "archive",
		// networks
		"connect", "disconnect", "expose", "bind", "proxy", "forward",
		// system
		"daemon", "service", "process", "kill", "signal",
		"mount", "unmount", "encrypt", "decrypt",
	}
}

// ── Subcommand lists (used both in corpus and prefix-detection) ──────────────

var gitSubcommands = []string{
	"add", "bisect", "blame", "branch", "checkout", "cherry-pick",
	"clean", "clone", "commit", "config", "describe", "diff",
	"fetch", "format-patch", "gc", "grep", "init", "log",
	"merge", "mv", "notes", "pull", "push", "rebase", "reflog",
	"remote", "reset", "restore", "revert", "rm", "shortlog",
	"show", "stash", "status", "submodule", "switch", "tag",
	"worktree", "archive", "bundle",
}

var dockerSubcommands = []string{
	"build", "commit", "container", "cp", "create", "deploy",
	"diff", "events", "exec", "export", "history", "image",
	"images", "import", "info", "inspect", "kill", "load",
	"login", "logout", "logs", "network", "node", "pause",
	"plugin", "port", "ps", "pull", "push", "rename",
	"restart", "rm", "rmi", "run", "save", "search",
	"secret", "service", "stack", "start", "stats", "stop",
	"swarm", "system", "tag", "top", "trust", "unpause",
	"update", "version", "volume", "wait",
}

var kubectlSubcommands = []string{
	"apply", "attach", "autoscale", "cluster-info", "config",
	"cordon", "cp", "create", "debug", "delete", "describe",
	"diff", "drain", "edit", "exec", "explain", "expose",
	"get", "label", "logs", "patch", "port-forward", "proxy",
	"replace", "rollout", "run", "scale", "set", "taint",
	"top", "uncordon", "version", "wait", "auth", "certificate",
	"api-resources", "api-versions",
}

// ── Modern alternatives map ──────────────────────────────────────────────────

var modernAlternatives = map[string][]string{
	"ls":   {"exa", "lsd"},
	"cat":  {"bat", "batcat"},
	"find": {"fd"},
	"grep": {"ripgrep", "rg"},
	"ps":   {"procs"},
	"top":  {"htop", "btop"},
	"du":   {"dust"},
	"df":   {"duf"},
	"diff": {"delta"},
	"curl": {"httpie"},
	"ping": {"gping"},
}
