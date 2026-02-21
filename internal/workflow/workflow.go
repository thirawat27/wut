// Package workflow provides context-aware workflow suggestions
package workflow

import (
	"fmt"
	"strings"

	"wut/internal/context"
)

// Workflow represents a suggested workflow
type Workflow struct {
	Name        string
	Description string
	Category    string
	Steps       []Step
	Priority    int // Higher = more important
}

// Step represents a workflow step
type Step struct {
	Command     string
	Description string
	Optional    bool
	Condition   string // Condition for when this step applies
}

// Engine provides workflow suggestions
type Engine struct {
	workflows map[string][]Workflow
}

// NewEngine creates a new workflow engine
func NewEngine() *Engine {
	e := &Engine{
		workflows: make(map[string][]Workflow),
	}
	e.initializeWorkflows()
	return e
}

// GetWorkflows returns workflows for the given context
func (e *Engine) GetWorkflows(ctx *context.Context) []Workflow {
	var workflows []Workflow

	// Get workflows for the specific project type
	if typeWorkflows, ok := e.workflows[ctx.ProjectType]; ok {
		workflows = append(workflows, typeWorkflows...)
	}

	// Get generic workflows for all projects
	if genericWorkflows, ok := e.workflows["generic"]; ok {
		workflows = append(workflows, genericWorkflows...)
	}

	// Get git workflows if in a git repo
	if ctx.IsGitRepo {
		if gitWorkflows, ok := e.workflows["git"]; ok {
			for _, w := range gitWorkflows {
				workflows = append(workflows, e.adaptGitWorkflow(w, ctx)...)
			}
		}
	}

	return e.sortByPriority(workflows)
}

// GetQuickActions returns quick actions based on context
func (e *Engine) GetQuickActions(ctx *context.Context) []QuickAction {
	var actions []QuickAction

	// Context-aware quick actions
	switch ctx.ProjectType {
	case "go":
		actions = append(actions, QuickAction{
			Name:        "Build & Test",
			Command:     "go build ./... && go test ./...",
			Description: "Build and test all packages",
			Icon:        "🔨",
		})
		actions = append(actions, QuickAction{
			Name:        "Tidy Modules",
			Command:     "go mod tidy",
			Description: "Clean up module dependencies",
			Icon:        "📦",
		})

	case "nodejs":
		actions = append(actions, QuickAction{
			Name:        "Install Dependencies",
			Command:     "npm install",
			Description: "Install npm packages",
			Icon:        "📦",
		})
		actions = append(actions, QuickAction{
			Name:        "Build Project",
			Command:     "npm run build",
			Description: "Build the project",
			Icon:        "🔨",
		})
		actions = append(actions, QuickAction{
			Name:        "Run Tests",
			Command:     "npm test",
			Description: "Run test suite",
			Icon:        "🧪",
		})
		actions = append(actions, QuickAction{
			Name:        "Start Dev Server",
			Command:     "npm run dev",
			Description: "Start development server",
			Icon:        "🚀",
		})

	case "docker":
		actions = append(actions, QuickAction{
			Name:        "Build Image",
			Command:     "docker build -t myapp .",
			Description: "Build Docker image",
			Icon:        "🐳",
		})
		actions = append(actions, QuickAction{
			Name:        "Compose Up",
			Command:     "docker-compose up -d",
			Description: "Start services",
			Icon:        "▶️",
		})
	}

	// Git actions if in git repo
	if ctx.IsGitRepo {
		actions = append(actions, QuickAction{
			Name:        "Git Status",
			Command:     "git status",
			Description: "Check repository status",
			Icon:        "📊",
		})
		actions = append(actions, QuickAction{
			Name:        "Git Pull",
			Command:     "git pull",
			Description: "Pull latest changes",
			Icon:        "⬇️",
		})
	}

	return actions
}

// GetNextLikelyCommand predicts the next command based on context and history
func (e *Engine) GetNextLikelyCommand(ctx *context.Context, lastCommand string) []string {
	var suggestions []string

	// Based on project type and last command
	switch ctx.ProjectType {
	case "go":
		suggestions = append(suggestions, e.predictGoCommand(lastCommand)...)
	case "nodejs":
		suggestions = append(suggestions, e.predictNodeCommand(lastCommand)...)
	case "git", "generic":
		if ctx.IsGitRepo {
			suggestions = append(suggestions, e.predictGitCommand(lastCommand)...)
		}
	}

	return suggestions
}

// predictGoCommand predicts next Go command
func (e *Engine) predictGoCommand(lastCmd string) []string {
	predictions := map[string][]string{
		"go mod init":     {"go get ./...", "go mod tidy"},
		"go get":          {"go build", "go test ./..."},
		"go build":        {"go test ./...", "go run ."},
		"go test":         {"go build", "go install"},
		"git add":         {"git commit", "git status"},
		"git commit":      {"git push", "git log"},
		"git pull":        {"git status", "go build"},
	}

	// Check for partial matches
	for pattern, cmds := range predictions {
		if strings.Contains(lastCmd, pattern) || strings.Contains(pattern, lastCmd) {
			return cmds
		}
	}

	// Default suggestions
	return []string{"go build", "go test ./...", "go run ."}
}

// predictNodeCommand predicts next Node.js command
func (e *Engine) predictNodeCommand(lastCmd string) []string {
	predictions := map[string][]string{
		"npm install":     {"npm run build", "npm test"},
		"npm ci":          {"npm run build", "npm start"},
		"npm run build":   {"npm test", "npm run deploy"},
		"git add":         {"git commit", "npm run lint"},
	}

	for pattern, cmds := range predictions {
		if strings.Contains(lastCmd, pattern) {
			return cmds
		}
	}

	// Default suggestions
	return []string{"npm run dev", "npm test", "npm run build"}
}

// predictGitCommand predicts next Git command
func (e *Engine) predictGitCommand(lastCmd string) []string {
	predictions := map[string][]string{
		"git status":      {"git add .", "git diff", "git log"},
		"git add":         {"git commit", "git diff --staged"},
		"git commit":      {"git push", "git log --oneline"},
		"git pull":        {"git status", "git log"},
		"git checkout -b": {"git push -u origin HEAD", "git status"},
	}

	for pattern, cmds := range predictions {
		if strings.Contains(lastCmd, pattern) || strings.Contains(pattern, lastCmd) {
			return cmds
		}
	}

	return []string{"git status", "git add .", "git commit"}
}

// adaptGitWorkflow adapts git workflows based on current state
func (e *Engine) adaptGitWorkflow(w Workflow, ctx *context.Context) []Workflow {
	var adapted []Workflow

	// Check if there are uncommitted changes
	if !ctx.GitStatus.IsClean {
		adapted = append(adapted, Workflow{
			Name:        "Commit Changes",
			Description: "Commit your uncommitted changes",
			Category:    "git",
			Priority:    100,
			Steps: []Step{
				{Command: "git status", Description: "Check what files have changed"},
				{Command: "git add .", Description: "Stage all changes", Optional: true},
				{Command: "git commit -m \"your message\"", Description: "Commit with a descriptive message"},
			},
		})
	}

	return adapted
}

// sortByPriority sorts workflows by priority (descending)
func (e *Engine) sortByPriority(workflows []Workflow) []Workflow {
	for i := 0; i < len(workflows)-1; i++ {
		for j := i + 1; j < len(workflows); j++ {
			if workflows[i].Priority < workflows[j].Priority {
				workflows[i], workflows[j] = workflows[j], workflows[i]
			}
		}
	}
	return workflows
}

// QuickAction represents a quick action button
type QuickAction struct {
	Name        string
	Command     string
	Description string
	Icon        string
}

// initializeWorkflows initializes built-in workflows
func (e *Engine) initializeWorkflows() {
	// Go workflows
	e.workflows["go"] = []Workflow{
		{
			Name:        "Start New Feature",
			Description: "Create a new branch and start development",
			Category:    "development",
			Priority:    90,
			Steps: []Step{
				{Command: "git checkout -b feature/name", Description: "Create and switch to new branch"},
				{Command: "go mod tidy", Description: "Ensure dependencies are up to date"},
				{Command: "go test ./...", Description: "Run existing tests to ensure baseline"},
			},
		},
		{
			Name:        "Prepare for PR",
			Description: "Get your code ready for pull request",
			Category:    "git",
			Priority:    85,
			Steps: []Step{
				{Command: "go fmt ./...", Description: "Format all Go code"},
				{Command: "go vet ./...", Description: "Run static analysis"},
				{Command: "go test ./...", Description: "Run all tests"},
				{Command: "go build ./...", Description: "Ensure everything builds"},
			},
		},
		{
			Name:        "Release",
			Description: "Build and prepare release binaries",
			Category:    "deployment",
			Priority:    70,
			Steps: []Step{
				{Command: "go test ./...", Description: "Final test run"},
				{Command: "go build -o bin/app", Description: "Build binary"},
				{Command: "git tag v1.0.0", Description: "Create version tag", Optional: true},
			},
		},
	}

	// Node.js workflows
	e.workflows["nodejs"] = []Workflow{
		{
			Name:        "Setup Project",
			Description: "Initial project setup",
			Category:    "setup",
			Priority:    95,
			Steps: []Step{
				{Command: "npm install", Description: "Install all dependencies"},
				{Command: "npm run build", Description: "Initial build", Optional: true},
				{Command: "npm test", Description: "Run tests to verify setup"},
			},
		},
		{
			Name:        "Development Loop",
			Description: "Common development workflow",
			Category:    "development",
			Priority:    90,
			Steps: []Step{
				{Command: "npm run dev", Description: "Start development server"},
				{Command: "npm run test:watch", Description: "Run tests in watch mode", Optional: true},
			},
		},
		{
			Name:        "Deploy",
			Description: "Build and deploy",
			Category:    "deployment",
			Priority:    75,
			Steps: []Step{
				{Command: "npm ci", Description: "Clean install for production"},
				{Command: "npm run build", Description: "Production build"},
				{Command: "npm run deploy", Description: "Deploy to production", Optional: true},
			},
		},
	}

	// Docker workflows
	e.workflows["docker"] = []Workflow{
		{
			Name:        "Build & Run",
			Description: "Build and run container locally",
			Category:    "development",
			Priority:    90,
			Steps: []Step{
				{Command: "docker build -t myapp .", Description: "Build Docker image"},
				{Command: "docker run -p 8080:80 myapp", Description: "Run container locally"},
			},
		},
		{
			Name:        "Compose Development",
			Description: "Full development environment with compose",
			Category:    "development",
			Priority:    85,
			Steps: []Step{
				{Command: "docker-compose build", Description: "Build all services"},
				{Command: "docker-compose up -d", Description: "Start all services"},
				{Command: "docker-compose logs -f", Description: "Follow logs"},
			},
		},
	}

	// Git workflows
	e.workflows["git"] = []Workflow{
		{
			Name:        "Safe Update",
			Description: "Update local branch safely",
			Category:    "git",
			Priority:    80,
			Steps: []Step{
				{Command: "git stash", Description: "Stash local changes", Optional: true},
				{Command: "git pull --rebase", Description: "Pull with rebase"},
				{Command: "git stash pop", Description: "Restore stashed changes", Optional: true},
			},
		},
		{
			Name:        "Feature Branch",
			Description: "Create a feature branch workflow",
			Category:    "git",
			Priority:    75,
			Steps: []Step{
				{Command: "git checkout main", Description: "Switch to main branch"},
				{Command: "git pull", Description: "Update main"},
				{Command: "git checkout -b feature/name", Description: "Create feature branch"},
			},
		},
	}

	// Generic workflows
	e.workflows["generic"] = []Workflow{
		{
			Name:        "Daily Start",
			Description: "Start your day with these commands",
			Category:    "daily",
			Priority:    70,
			Steps: []Step{
				{Command: "git pull", Description: "Get latest changes"},
				{Command: "ls -la", Description: "Check current directory"},
			},
		},
	}
}

// FormatWorkflow formats a workflow for display
func (e *Engine) FormatWorkflow(w Workflow) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 %s\n", w.Name))
	sb.WriteString(fmt.Sprintf("   %s\n", w.Description))
	sb.WriteString("   Steps:\n")
	for i, step := range w.Steps {
		optional := ""
		if step.Optional {
			optional = " (optional)"
		}
		sb.WriteString(fmt.Sprintf("   %d. %s%s\n", i+1, step.Description, optional))
		sb.WriteString(fmt.Sprintf("      → %s\n", step.Command))
	}
	return sb.String()
}
