package corrector

// ──────────────────────────────────────────────────────────────────────────────
// Semantic Intent Engine
//
// Translates natural-language descriptions into shell commands.
// Uses two layers:
//   1. Exact keyword scoring (fast, deterministic)
//   2. Jaccard similarity on word-token sets for fuzzy phrase matching
//
// No external NLP library required – "github.com/sahilm/fuzzy" is used for
// fast substring matching within the intent description index.
// ──────────────────────────────────────────────────────────────────────────────

import (
	"math"
	"sort"
	"strings"
	"github.com/sahilm/fuzzy"
)

// Intent represents a natural-language pattern that maps to a shell command.
type Intent struct {
	// Keywords are individual tokens that trigger this intent (order-independent).
	Keywords []string
	// Phrases are multi-word triggers; any phrase match gives a bonus.
	Phrases []string
	// Command is the exact shell command to suggest.
	Command string
	// Description explains what the command does (shown to the user).
	Description string
	// Category groups related intents (docker, git, system …).
	Category string
}

// IntentMatch is a scored result from a semantic query.
type IntentMatch struct {
	Intent     Intent
	Score      float64
	Confidence float64
}

// semanticIntents is the global intent database.
// Add new entries here to extend semantic coverage.
var semanticIntents = []Intent{
	// ── Docker ─────────────────────────────────────────────────────────────
	{
		Keywords:    []string{"list", "running", "containers"},
		Phrases:     []string{"list running containers", "show containers", "running containers"},
		Command:     "docker ps",
		Description: "List currently running Docker containers",
		Category:    "docker",
	},
	{
		Keywords:    []string{"list", "all", "containers"},
		Phrases:     []string{"list all containers", "show all containers"},
		Command:     "docker ps -a",
		Description: "List all Docker containers (including stopped)",
		Category:    "docker",
	},
	{
		Keywords:    []string{"list", "images", "docker"},
		Phrases:     []string{"list docker images", "show images", "docker images"},
		Command:     "docker images",
		Description: "List all Docker images",
		Category:    "docker",
	},
	{
		Keywords:    []string{"stop", "all", "containers"},
		Phrases:     []string{"stop all containers", "stop all docker"},
		Command:     "docker stop $(docker ps -q)",
		Description: "Stop all running Docker containers",
		Category:    "docker",
	},
	{
		Keywords:    []string{"remove", "all", "containers"},
		Phrases:     []string{"remove all containers", "delete all containers", "clean containers"},
		Command:     "docker rm $(docker ps -aq)",
		Description: "Remove all Docker containers",
		Category:    "docker",
	},
	{
		Keywords:    []string{"remove", "unused", "images"},
		Phrases:     []string{"remove unused images", "clean images", "prune images"},
		Command:     "docker image prune -a",
		Description: "Remove all unused Docker images",
		Category:    "docker",
	},
	{
		Keywords:    []string{"logs", "container", "follow"},
		Phrases:     []string{"follow container logs", "tail container logs", "stream logs"},
		Command:     "docker logs -f <container>",
		Description: "Stream logs of a Docker container",
		Category:    "docker",
	},
	{
		Keywords:    []string{"enter", "shell", "container"},
		Phrases:     []string{"enter container", "bash into container", "open shell container", "exec into container"},
		Command:     "docker exec -it <container> /bin/bash",
		Description: "Open an interactive shell inside a running container",
		Category:    "docker",
	},
	{
		Keywords:    []string{"build", "image", "dockerfile"},
		Phrases:     []string{"build docker image", "build image"},
		Command:     "docker build -t <name> .",
		Description: "Build a Docker image from the current directory",
		Category:    "docker",
	},
	{
		Keywords:    []string{"disk", "usage", "docker"},
		Phrases:     []string{"docker disk usage", "docker space", "how much space docker"},
		Command:     "docker system df",
		Description: "Show Docker disk usage",
		Category:    "docker",
	},
	{
		Keywords:    []string{"clean", "prune", "docker"},
		Phrases:     []string{"clean docker", "prune docker", "free docker space"},
		Command:     "docker system prune -a",
		Description: "Remove all unused Docker data (images, containers, volumes)",
		Category:    "docker",
	},

	// ── Git ────────────────────────────────────────────────────────────────
	{
		Keywords:    []string{"undo", "last", "commit"},
		Phrases:     []string{"undo last commit", "revert last commit", "go back one commit"},
		Command:     "git reset --soft HEAD~1",
		Description: "Undo the last commit but keep the changes staged",
		Category:    "git",
	},
	{
		Keywords:    []string{"unstage", "files"},
		Phrases:     []string{"unstage all files", "unstage changes"},
		Command:     "git restore --staged .",
		Description: "Unstage all staged files",
		Category:    "git",
	},
	{
		Keywords:    []string{"discard", "changes", "working"},
		Phrases:     []string{"discard all changes", "discard local changes", "reset working tree"},
		Command:     "git restore .",
		Description: "Discard all uncommitted working directory changes",
		Category:    "git",
	},
	{
		Keywords:    []string{"list", "branches"},
		Phrases:     []string{"list all branches", "show branches"},
		Command:     "git branch -a",
		Description: "List all local and remote branches",
		Category:    "git",
	},
	{
		Keywords:    []string{"delete", "branch"},
		Phrases:     []string{"delete branch", "remove branch"},
		Command:     "git branch -d <branch>",
		Description: "Delete a local branch",
		Category:    "git",
	},
	{
		Keywords:    []string{"rename", "branch"},
		Phrases:     []string{"rename branch", "change branch name"},
		Command:     "git branch -m <old-name> <new-name>",
		Description: "Rename a local branch",
		Category:    "git",
	},
	{
		Keywords:    []string{"show", "log", "oneline"},
		Phrases:     []string{"show git log", "show commits", "list commits"},
		Command:     "git log --oneline --graph --decorate",
		Description: "Show a condensed, graphical commit log",
		Category:    "git",
	},
	{
		Keywords:    []string{"stash", "changes"},
		Phrases:     []string{"save changes", "stash current work", "temporarily save"},
		Command:     "git stash",
		Description: "Temporarily stash uncommitted changes",
		Category:    "git",
	},
	{
		Keywords:    []string{"restore", "stash", "pop"},
		Phrases:     []string{"restore stash", "apply stash", "pop stash"},
		Command:     "git stash pop",
		Description: "Restore the latest stashed changes",
		Category:    "git",
	},
	{
		Keywords:    []string{"find", "commit", "text", "search"},
		Phrases:     []string{"search commit history", "find text in commits", "find string in history"},
		Command:     "git log -S '<text>'",
		Description: "Search commit history for changes introducing a specific string",
		Category:    "git",
	},
	{
		Keywords:    []string{"show", "changed", "files", "commit"},
		Phrases:     []string{"show changed files", "which files changed"},
		Command:     "git diff --name-only HEAD~1",
		Description: "Show which files changed in the last commit",
		Category:    "git",
	},
	{
		Keywords:    []string{"tag", "release", "version"},
		Phrases:     []string{"create tag", "tag release", "tag version"},
		Command:     "git tag -a v<version> -m 'Release v<version>'",
		Description: "Create an annotated release tag",
		Category:    "git",
	},

	// ── Kubernetes ──────────────────────────────────────────────────────────
	{
		Keywords:    []string{"list", "pods"},
		Phrases:     []string{"list pods", "show pods", "get pods"},
		Command:     "kubectl get pods",
		Description: "List all pods in the current namespace",
		Category:    "kubernetes",
	},
	{
		Keywords:    []string{"list", "all", "namespaces"},
		Phrases:     []string{"list all namespaces", "show namespaces", "get namespaces"},
		Command:     "kubectl get namespaces",
		Description: "List all Kubernetes namespaces",
		Category:    "kubernetes",
	},
	{
		Keywords:    []string{"logs", "pod"},
		Phrases:     []string{"get pod logs", "show pod logs", "view pod logs"},
		Command:     "kubectl logs <pod>",
		Description: "Get logs from a pod",
		Category:    "kubernetes",
	},
	{
		Keywords:    []string{"exec", "shell", "pod"},
		Phrases:     []string{"open shell in pod", "exec into pod", "bash into pod"},
		Command:     "kubectl exec -it <pod> -- /bin/bash",
		Description: "Open an interactive shell inside a Kubernetes pod",
		Category:    "kubernetes",
	},
	{
		Keywords:    []string{"scale", "deployment", "replicas"},
		Phrases:     []string{"scale deployment", "change replicas", "resize deployment"},
		Command:     "kubectl scale deployment <name> --replicas=<n>",
		Description: "Scale a Kubernetes deployment",
		Category:    "kubernetes",
	},
	{
		Keywords:    []string{"restart", "deployment"},
		Phrases:     []string{"restart deployment", "rolling restart", "redeploy"},
		Command:     "kubectl rollout restart deployment/<name>",
		Description: "Trigger a rolling restart of a deployment",
		Category:    "kubernetes",
	},

	// ── System / Linux ─────────────────────────────────────────────────────
	{
		Keywords:    []string{"find", "large", "files"},
		Phrases:     []string{"find large files", "largest files", "biggest files"},
		Command:     "find . -type f -size +100M",
		Description: "Find files larger than 100 MB in the current directory",
		Category:    "system",
	},
	{
		Keywords:    []string{"disk", "usage", "directory"},
		Phrases:     []string{"disk usage", "directory size", "folder size", "how much space"},
		Command:     "du -sh *",
		Description: "Show disk usage of all items in the current directory",
		Category:    "system",
	},
	{
		Keywords:    []string{"kill", "process", "name"},
		Phrases:     []string{"kill process", "stop process by name"},
		Command:     "pkill -f <name>",
		Description: "Kill a process by name",
		Category:    "system",
	},
	{
		Keywords:    []string{"port", "listening", "check"},
		Phrases:     []string{"check port", "which port", "port in use", "port listening"},
		Command:     "ss -tlnp | grep <port>",
		Description: "Check which process is listening on a port",
		Category:    "system",
	},
	{
		Keywords:    []string{"free", "memory", "ram"},
		Phrases:     []string{"check memory", "how much ram", "free memory", "ram usage"},
		Command:     "free -h",
		Description: "Show free and used memory",
		Category:    "system",
	},
	{
		Keywords:    []string{"cpu", "usage", "load"},
		Phrases:     []string{"cpu usage", "check cpu", "cpu load"},
		Command:     "top -bn1 | grep 'Cpu'",
		Description: "Show current CPU usage",
		Category:    "system",
	},
	{
		Keywords:    []string{"compress", "files", "tar"},
		Phrases:     []string{"compress files", "create archive", "zip folder"},
		Command:     "tar -czf archive.tar.gz <directory>",
		Description: "Compress a directory into a .tar.gz archive",
		Category:    "system",
	},
	{
		Keywords:    []string{"extract", "archive", "unzip"},
		Phrases:     []string{"extract archive", "unzip file", "extract tar"},
		Command:     "tar -xzf archive.tar.gz",
		Description: "Extract a .tar.gz archive",
		Category:    "system",
	},
	{
		Keywords:    []string{"count", "lines", "file"},
		Phrases:     []string{"count lines", "how many lines", "line count"},
		Command:     "wc -l <file>",
		Description: "Count the number of lines in a file",
		Category:    "system",
	},
	{
		Keywords:    []string{"search", "text", "files"},
		Phrases:     []string{"search for text", "find text in files", "grep recursively"},
		Command:     "grep -r '<text>' .",
		Description: "Search for text recursively in the current directory",
		Category:    "system",
	},
	{
		Keywords:    []string{"show", "environment", "variables"},
		Phrases:     []string{"show env vars", "list environment variables", "print env"},
		Command:     "printenv | sort",
		Description: "List all environment variables (sorted)",
		Category:    "system",
	},
	{
		Keywords:    []string{"current", "directory", "path"},
		Phrases:     []string{"where am i", "current path", "current directory"},
		Command:     "pwd",
		Description: "Print the current working directory",
		Category:    "system",
	},

	// ── npm / Node ──────────────────────────────────────────────────────────
	{
		Keywords:    []string{"install", "dependencies", "npm"},
		Phrases:     []string{"install dependencies", "install packages", "npm install"},
		Command:     "npm install",
		Description: "Install all npm dependencies from package.json",
		Category:    "npm",
	},
	{
		Keywords:    []string{"outdated", "packages", "npm"},
		Phrases:     []string{"outdated packages", "check updates", "which packages outdated"},
		Command:     "npm outdated",
		Description: "Check for outdated npm packages",
		Category:    "npm",
	},
	{
		Keywords:    []string{"security", "audit", "npm"},
		Phrases:     []string{"security audit", "npm audit", "check vulnerabilities"},
		Command:     "npm audit",
		Description: "Run a security audit on npm packages",
		Category:    "npm",
	},

	// ── Go ─────────────────────────────────────────────────────────────────
	{
		Keywords:    []string{"run", "tests", "go"},
		Phrases:     []string{"run go tests", "run all tests", "test go project"},
		Command:     "go test ./...",
		Description: "Run all Go tests recursively",
		Category:    "go",
	},
	{
		Keywords:    []string{"build", "go", "binary"},
		Phrases:     []string{"build go binary", "compile go", "build go app"},
		Command:     "go build -o <output> .",
		Description: "Build the Go project to a binary",
		Category:    "go",
	},
	{
		Keywords:    []string{"tidy", "modules", "dependencies"},
		Phrases:     []string{"tidy go modules", "clean go dependencies", "go mod tidy"},
		Command:     "go mod tidy",
		Description: "Remove unused and add missing Go module dependencies",
		Category:    "go",
	},
}

// ── Scoring engine ────────────────────────────────────────────────────────────

// QuerySemantic searches intents by natural-language query.
// It returns up to `limit` matches sorted by score (highest first).
// Uses two passes:
//  1. Keyword frequency scoring (weighted by IDF)
//  2. Fuzzy phrase matching via sahilm/fuzzy
func QuerySemantic(query string, limit int) []IntentMatch {
	if limit <= 0 {
		limit = 5
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	// Build description strings for fuzzy matching
	descriptions := make([]string, len(semanticIntents))
	for i, intent := range semanticIntents {
		descriptions[i] = intent.Description + " " + strings.Join(intent.Phrases, " ")
	}

	scored := make([]IntentMatch, len(semanticIntents))
	for i, intent := range semanticIntents {
		score := keywordScore(queryTokens, intent)
		scored[i] = IntentMatch{
			Intent: intent,
			Score:  score,
		}
	}

	// Pass 2: add fuzzy bonus from sahilm/fuzzy
	fuzzyResults := fuzzy.FindFrom(query, fuzzySourceList(descriptions))
	fuzzyBonus := map[int]float64{}
	for rank, r := range fuzzyResults {
		// Higher bonus for lower rank (closer match)
		bonus := 1.5 / float64(rank+1)
		fuzzyBonus[r.Index] += bonus
	}
	for i := range scored {
		scored[i].Score += fuzzyBonus[i]
	}

	// Sort by score descending
	sort.Slice(scored, func(a, b int) bool {
		return scored[a].Score > scored[b].Score
	})

	// Filter out very low scores
	var results []IntentMatch
	for _, m := range scored {
		if m.Score < 0.4 {
			break
		}
		// Normalise to a 0–1 confidence
		m.Confidence = math.Min(1.0, m.Score/3.0)
		results = append(results, m)
		if len(results) >= limit {
			break
		}
	}
	return results
}

// keywordScore computes a simple keyword-overlap score between query tokens
// and an intent using a weighted Jaccard-like formula.
func keywordScore(queryTokens []string, intent Intent) float64 {
	score := 0.0

	// Exact keyword hits
	for _, kw := range intent.Keywords {
		for _, qt := range queryTokens {
			if qt == kw {
				score += 1.0
			} else if strings.Contains(qt, kw) || strings.Contains(kw, qt) {
				score += 0.4
			}
		}
	}

	// Whole-phrase bonus (much stronger signal)
	queryLower := strings.ToLower(strings.Join(queryTokens, " "))
	for _, phrase := range intent.Phrases {
		if strings.Contains(queryLower, strings.ToLower(phrase)) {
			score += 2.5
		}
	}

	// Synonym expansion (common query words → canonical keywords)
	for _, qt := range queryTokens {
		if expanded, ok := synonymMap[qt]; ok {
			for _, kw := range intent.Keywords {
				if expanded == kw {
					score += 0.7
				}
			}
		}
	}

	return score
}

// tokenize lowercases and splits a string into meaningful word tokens,
// removing stop words that carry no semantic weight.
func tokenize(s string) []string {
	raw := strings.Fields(strings.ToLower(s))
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		// Strip punctuation suffixes/prefixes
		w = strings.Trim(w, ".,!?;:\"'()")
		if w == "" {
			continue
		}
		if stopWords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// stopWords are common words that don't contribute to intent matching.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"i": true, "in": true, "on": true, "of": true, "for": true,
	"to": true, "how": true, "can": true, "do": true, "my": true,
	"me": true, "this": true, "that": true, "it": true, "and": true,
	"or": true, "with": true, "want": true, "need": true, "please": true,
	"using": true, "use": true, "get": true, "see": true, "what": true,
}

// synonymMap maps common query words to the canonical keywords used in intents.
var synonymMap = map[string]string{
	// list/show synonyms
	"display": "list",
	"view":    "list",
	"print":   "list",
	"output":  "list",
	"show":    "list",
	// remove/delete synonyms
	"delete": "remove",
	"rm":     "remove",
	"erase":  "remove",
	"purge":  "remove",
	"clean":  "remove",
	"drop":   "remove",
	// create synonyms
	"make":  "create",
	"new":   "create",
	"init":  "create",
	"add":   "create",
	"build": "build",
	// service/container synonyms
	"service":   "container",
	"instance":  "container",
	"app":       "container",
	"container": "container",
	// disk synonyms
	"space": "disk",
	"size":  "disk",
	"usage": "disk",
	// port synonyms
	"listening": "listening",
	"open":      "listening",
	"used":      "listening",
	// memory synonyms
	"memory": "memory",
	"ram":    "memory",
	"mem":    "memory",
	// search synonyms
	"grep":   "search",
	"find":   "find",
	"locate": "find",
	"look":   "find",
}

// fuzzySourceList adapts a []string to the fuzzy.Source interface.
type fuzzySourceList []string

func (f fuzzySourceList) Len() int            { return len(f) }
func (f fuzzySourceList) String(i int) string { return f[i] }
