// Package context provides context analysis functionality for WUT
package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Context holds information about the current environment
type Context struct {
	WorkingDir     string
	HomeDir        string
	IsGitRepo      bool
	GitBranch      string
	GitStatus      GitStatus
	ProjectType    string
	ProjectFiles   []string
	Environment    map[string]string
	Shell          string
	OS             string
}

// GitStatus represents git repository status
type GitStatus struct {
	IsClean       bool
	ModifiedFiles []string
	StagedFiles   []string
	UntrackedFiles []string
	Ahead         int
	Behind        int
}

// Analyzer analyzes the current context
type Analyzer struct {
	context *Context
}

// NewAnalyzer creates a new context analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		context: &Context{
			Environment: make(map[string]string),
		},
	}
}

// Analyze analyzes the current context
func (a *Analyzer) Analyze() (*Context, error) {
	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	a.context.WorkingDir = wd
	
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	a.context.HomeDir = home
	
	// Detect OS
	a.context.OS = detectOS()
	
	// Detect shell
	a.context.Shell = detectShell()
	
	// Analyze git context
	a.analyzeGit()
	
	// Detect project type
	a.detectProjectType()
	
	// Get environment variables
	a.getEnvironment()
	
	return a.context, nil
}

// analyzeGit analyzes git repository context
func (a *Analyzer) analyzeGit() {
	// Check if in a git repository
	gitDir := findGitDir(a.context.WorkingDir)
	if gitDir == "" {
		a.context.IsGitRepo = false
		return
	}
	
	a.context.IsGitRepo = true
	
	// Get current branch
	if branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		a.context.GitBranch = strings.TrimSpace(string(branch))
	}
	
	// Get git status
	a.context.GitStatus = a.getGitStatus()
}

// getGitStatus gets detailed git status
func (a *Analyzer) getGitStatus() GitStatus {
	status := GitStatus{}
	
	// Check if clean
	if output, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		status.IsClean = len(lines) == 0 || (len(lines) == 1 && lines[0] == "")
		
		for _, line := range lines {
			if len(line) < 2 {
				continue
			}
			
			// Parse porcelain output
			indexStatus := line[0]
			workTreeStatus := ' '
			if len(line) > 1 {
				workTreeStatus = rune(line[1])
			}
			
			// Get filename (starts at position 3)
			filename := ""
			if len(line) > 3 {
				filename = strings.TrimSpace(line[3:])
			}
			
			// Check for renamed files
			if strings.Contains(filename, " -> ") {
				parts := strings.Split(filename, " -> ")
				if len(parts) == 2 {
					filename = parts[1]
				}
			}
			
			switch indexStatus {
			case 'M', 'A', 'D', 'R', 'C':
				status.StagedFiles = append(status.StagedFiles, filename)
			}
			
			switch workTreeStatus {
			case 'M', 'D':
				status.ModifiedFiles = append(status.ModifiedFiles, filename)
			case '?':
				status.UntrackedFiles = append(status.UntrackedFiles, filename)
			}
		}
	}
	
	// Get ahead/behind
	if output, err := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}").Output(); err == nil {
		var ahead, behind int
		if _, err := fmt.Sscanf(string(output), "%d\t%d", &ahead, &behind); err == nil {
			status.Ahead = ahead
			status.Behind = behind
		}
	}
	
	return status
}

// detectProjectType detects the project type based on files
func (a *Analyzer) detectProjectType() {
	files, err := os.ReadDir(a.context.WorkingDir)
	if err != nil {
		return
	}
	
	var projectFiles []string
	for _, file := range files {
		if !file.IsDir() {
			projectFiles = append(projectFiles, file.Name())
		}
	}
	a.context.ProjectFiles = projectFiles
	
	// Detect project type based on files
	typePatterns := map[string][]string{
		"nodejs":    {"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml"},
		"python":    {"requirements.txt", "setup.py", "pyproject.toml", "Pipfile"},
		"go":        {"go.mod", "go.sum"},
		"rust":      {"Cargo.toml", "Cargo.lock"},
		"ruby":      {"Gemfile", "Gemfile.lock"},
		"java":      {"pom.xml", "build.gradle"},
		"dotnet":    {"*.csproj", "*.sln"},
		"docker":    {"Dockerfile", "docker-compose.yml", "docker-compose.yaml"},
		"terraform": {"*.tf", "*.tfvars"},
		"ansible":   {"ansible.cfg", "inventory", "playbook.yml"},
		"kubernetes": {"*.yaml", "*.yml"},
	}
	
	// Check for specific files
	for projectType, patterns := range typePatterns {
		for _, pattern := range patterns {
			if matchPattern(projectFiles, pattern) {
				a.context.ProjectType = projectType
				return
			}
		}
	}
	
	// Check for git repo
	if a.context.IsGitRepo {
		a.context.ProjectType = "git"
		return
	}
	
	a.context.ProjectType = "unknown"
}

// getEnvironment gets relevant environment variables
func (a *Analyzer) getEnvironment() {
	relevantVars := []string{
		"HOME", "USER", "SHELL", "EDITOR", "PAGER",
		"PATH", "LANG", "TERM",
		"SSH_CONNECTION", "SSH_CLIENT",
	}
	
	for _, v := range relevantVars {
		if val := os.Getenv(v); val != "" {
			a.context.Environment[v] = val
		}
	}
}

// GetContext returns the current context
func (a *Analyzer) GetContext() *Context {
	return a.context
}

// GetRelevantCommands returns commands relevant to the current context
func (a *Analyzer) GetRelevantCommands() []string {
	if a.context == nil {
		return nil
	}
	
	var commands []string
	
	// Git context commands
	if a.context.IsGitRepo {
		commands = append(commands, a.getGitCommands()...)
	}
	
	// Project type specific commands
	commands = append(commands, a.getProjectCommands()...)
	
	return commands
}

// getGitCommands returns git commands based on context
func (a *Analyzer) getGitCommands() []string {
	var commands []string
	
	// Based on git status
	if !a.context.GitStatus.IsClean {
		if len(a.context.GitStatus.StagedFiles) > 0 {
			commands = append(commands, "git commit -m 'message'")
		}
		if len(a.context.GitStatus.ModifiedFiles) > 0 {
			commands = append(commands, "git add .", "git stash")
		}
		if len(a.context.GitStatus.UntrackedFiles) > 0 {
			commands = append(commands, "git add .")
		}
	}
	
	// Based on branch status
	if a.context.GitStatus.Ahead > 0 {
		commands = append(commands, "git push")
	}
	if a.context.GitStatus.Behind > 0 {
		commands = append(commands, "git pull")
	}
	
	// Always relevant
	commands = append(commands, "git status", "git log --oneline -10")
	
	return commands
}

// getProjectCommands returns commands based on project type
func (a *Analyzer) getProjectCommands() []string {
	switch a.context.ProjectType {
	case "nodejs":
		return []string{
			"npm install",
			"npm run dev",
			"npm run build",
			"npm test",
			"npm start",
		}
	case "python":
		return []string{
			"pip install -r requirements.txt",
			"python -m venv venv",
			"pytest",
			"python main.py",
			"pip list",
		}
	case "go":
		return []string{
			"go mod tidy",
			"go build",
			"go test ./...",
			"go run .",
			"go fmt ./...",
		}
	case "docker":
		return []string{
			"docker-compose up -d",
			"docker-compose down",
			"docker-compose build",
			"docker ps",
			"docker build -t myapp .",
		}
	case "kubernetes":
		return []string{
			"kubectl apply -f .",
			"kubectl get pods",
			"kubectl get svc",
			"kubectl logs -f <pod>",
		}
	default:
		return nil
	}
}

// Helper functions

func findGitDir(startPath string) string {
	current := startPath
	// Check until we reach root or can't go further
	for {
		gitPath := filepath.Join(current, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return gitPath
		}
		parent := filepath.Dir(current)
		// Stop if we've reached root (parent == current) or empty path
		if parent == current || parent == "" {
			break
		}
		current = parent
	}
	return ""
}

func detectOS() string {
	// Use runtime.GOOS for reliable OS detection at compile time
	return strings.ToLower(runtime.GOOS)
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return filepath.Base(shell)
	}
	
	// Windows detection
	if runtime.GOOS == "windows" {
		// Check for PowerShell first (more common now)
		if os.Getenv("PSModulePath") != "" {
			// Could be PowerShell or PowerShell Core
			if os.Getenv("PSVersionTable") != "" {
				return "pwsh" // PowerShell Core
			}
			return "powershell"
		}
		// Fall back to COMSPEC (cmd.exe)
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return filepath.Base(comspec)
		}
		return "cmd"
	}
	
	return "sh" // Default fallback
}

func matchPattern(files []string, pattern string) bool {
	// Simple glob matching
	re := regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$")
	for _, file := range files {
		if re.MatchString(file) {
			return true
		}
	}
	return false
}
