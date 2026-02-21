package search

import (
	"context"
	"strings"

	appcontext "wut/internal/context"
)

// ContextAwareSearch performs search with context awareness
func (e *Engine) ContextAwareSearch(ctx context.Context, query Query) ([]Result, error) {
	// Analyze current context
	analyzer := appcontext.NewAnalyzer()
	ctxInfo, err := analyzer.Analyze()
	if err != nil {
		// Fall back to regular search if context analysis fails
		return e.Search(ctx, query)
	}

	// Get context-relevant commands
	relevantCmds := analyzer.GetRelevantCommands()

	// Perform regular search
	results, err := e.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// Boost scores for context-relevant commands
	boostedResults := e.boostContextRelevant(results, relevantCmds, ctxInfo)

	// Add context-specific suggestions if query is empty or results are few
	if query.Text == "" || len(boostedResults) < 5 {
		contextSuggestions := e.generateContextSuggestions(ctxInfo, relevantCmds)
		boostedResults = append(contextSuggestions, boostedResults...)
	}

	return e.filterAndSort(boostedResults, query), nil
}

// boostContextRelevant boosts scores for commands relevant to current context
func (e *Engine) boostContextRelevant(results []Result, relevantCmds []string, ctxInfo *appcontext.Context) []Result {
	relevantMap := make(map[string]bool)
	for _, cmd := range relevantCmds {
		relevantMap[cmd] = true
	}

	boosted := make([]Result, len(results))
	for i, r := range results {
		boosted[i] = r

		// Check if command is context-relevant
		if relevantMap[r.Command] {
			// Boost score by 20%
			boosted[i].Score = minFloat(r.Score*1.2, 1.0)
			boosted[i].Relevance = minFloat(r.Relevance*1.2, 1.0)
		}

		// Additional boost for git commands in git repos
		if ctxInfo.IsGitRepo && strings.HasPrefix(r.Command, "git ") {
			boosted[i].Score = minFloat(boosted[i].Score*1.1, 1.0)
			boosted[i].Relevance = minFloat(boosted[i].Relevance*1.1, 1.0)
		}

		// Boost project-specific commands
		if r.Category == ctxInfo.ProjectType {
			boosted[i].Score = minFloat(boosted[i].Score*1.15, 1.0)
			boosted[i].Relevance = minFloat(boosted[i].Relevance*1.15, 1.0)
		}
	}

	return boosted
}

// generateContextSuggestions generates suggestions based on current context
func (e *Engine) generateContextSuggestions(ctxInfo *appcontext.Context, relevantCmds []string) []Result {
	var suggestions []Result

	// Add relevant commands as high-priority suggestions
	for i, cmd := range relevantCmds {
		if i >= 5 { // Limit to top 5
			break
		}

		suggestions = append(suggestions, Result{
			Command:     cmd,
			Description: e.getCommandDescription(cmd, ctxInfo),
			Score:       0.95 - float64(i)*0.05, // Decreasing scores
			Source:      SourceContext,
			Relevance:   0.95 - float64(i)*0.05,
			Category:    ctxInfo.ProjectType,
		})
	}

	return suggestions
}

// getCommandDescription generates a context-aware description
func (e *Engine) getCommandDescription(cmd string, ctxInfo *appcontext.Context) string {
	// Git-specific descriptions
	if ctxInfo.IsGitRepo {
		switch cmd {
		case "git status":
			if !ctxInfo.GitStatus.IsClean {
				return "Check current changes (you have uncommitted changes)"
			}
			return "Check repository status"
		case "git add .":
			if len(ctxInfo.GitStatus.ModifiedFiles) > 0 {
				return "Stage all modified files for commit"
			}
			return "Stage all changes"
		case "git commit -m 'message'":
			if len(ctxInfo.GitStatus.StagedFiles) > 0 {
				return "Commit staged changes"
			}
			return "Commit changes with message"
		case "git push":
			if ctxInfo.GitStatus.Ahead > 0 {
				return "Push local commits to remote"
			}
			return "Push changes to remote repository"
		case "git pull":
			if ctxInfo.GitStatus.Behind > 0 {
				return "Pull latest changes from remote"
			}
			return "Fetch and merge remote changes"
		}
	}

	// Project-specific descriptions
	switch ctxInfo.ProjectType {
	case "nodejs":
		switch cmd {
		case "npm install":
			return "Install Node.js dependencies"
		case "npm run dev":
			return "Start development server"
		case "npm run build":
			return "Build for production"
		case "npm test":
			return "Run test suite"
		}
	case "go":
		switch cmd {
		case "go mod tidy":
			return "Clean up Go module dependencies"
		case "go build":
			return "Compile Go application"
		case "go test ./...":
			return "Run all tests"
		case "go run .":
			return "Run Go application"
		}
	case "python":
		switch cmd {
		case "pip install -r requirements.txt":
			return "Install Python dependencies"
		case "python -m venv venv":
			return "Create virtual environment"
		case "pytest":
			return "Run Python tests"
		}
	case "docker":
		switch cmd {
		case "docker-compose up -d":
			return "Start Docker containers in background"
		case "docker-compose down":
			return "Stop and remove containers"
		case "docker ps":
			return "List running containers"
		}
	}

	// Default description
	return "Context-relevant command"
}

// GetContextInfo returns current context information
func (e *Engine) GetContextInfo() (*appcontext.Context, error) {
	analyzer := appcontext.NewAnalyzer()
	return analyzer.Analyze()
}

// IsContextRelevant checks if a command is relevant to current context
func (e *Engine) IsContextRelevant(command string, ctxInfo *appcontext.Context) bool {
	// Git commands in git repos
	if ctxInfo.IsGitRepo && strings.HasPrefix(command, "git ") {
		return true
	}

	// Project-specific commands
	analyzer := appcontext.NewAnalyzer()
	relevantCmds := analyzer.GetRelevantCommands()

	for _, cmd := range relevantCmds {
		if strings.Contains(command, cmd) || strings.Contains(cmd, command) {
			return true
		}
	}

	return false
}
