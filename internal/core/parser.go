// Package core provides core functionality for WUT
package core

import (
	"fmt"
	"regexp"
	"strings"
)

// ParsedCommand represents a parsed command
type ParsedCommand struct {
	Raw         string   // Original raw command
	Command     string   // Base command
	Args        []string // Arguments
	Flags       []Flag   // Flags
	Subcommand  string   // Subcommand if any
	IsPipe      bool     // Whether command contains pipe
	IsRedirect  bool     // Whether command contains redirection
	PipedCommands []ParsedCommand // Commands in the pipe
}

// Flag represents a command flag
type Flag struct {
	Name    string // Flag name
	Value   string // Flag value (if any)
	IsShort bool   // Whether it's a short flag (-f vs --flag)
}

// Parser handles command parsing
type Parser struct {
	// Regex patterns
	flagPattern    *regexp.Regexp
	pipePattern    *regexp.Regexp
	redirectPattern *regexp.Regexp
}

// NewParser creates a new command parser
func NewParser() *Parser {
	return &Parser{
		flagPattern:     regexp.MustCompile(`^--?([^=\s]+)(?:=(\S+))?`),
		pipePattern:     regexp.MustCompile(`\s*\|\s*`),
		redirectPattern: regexp.MustCompile(`[<>]|>>|2>|&>`),
	}
}

// Parse parses a command string
func (p *Parser) Parse(input string) (*ParsedCommand, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("empty command")
	}

	// Check for pipes
	if p.pipePattern.MatchString(input) {
		return p.parsePipedCommand(input)
	}

	// Check for redirections
	isRedirect := p.redirectPattern.MatchString(input)

	// Tokenize the input
	tokens := p.tokenize(input)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens found")
	}

	parsed := &ParsedCommand{
		Raw:        strings.TrimSpace(input),
		Command:    tokens[0],
		IsRedirect: isRedirect,
	}

	// Parse flags and arguments
	i := 1
	for i < len(tokens) {
		token := tokens[i]

		// Check if it's a flag
		if flag := p.parseFlag(token); flag != nil {
			parsed.Flags = append(parsed.Flags, *flag)
			// Check for flag value in next token (if not already captured)
			if flag.Value == "" && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++
				if i < len(tokens) {
					// Update the flag value
					parsed.Flags[len(parsed.Flags)-1].Value = tokens[i]
				}
			}
		} else if parsed.Subcommand == "" && !strings.HasPrefix(token, "-") && len(parsed.Args) == 0 {
			// First non-flag argument could be a subcommand
			parsed.Subcommand = token
		} else {
			parsed.Args = append(parsed.Args, token)
		}
		i++
	}

	return parsed, nil
}

// parsePipedCommand parses a piped command
func (p *Parser) parsePipedCommand(input string) (*ParsedCommand, error) {
	parts := p.pipePattern.Split(input, -1)
	if len(parts) < 2 {
		return p.Parse(input)
	}

	parsed := &ParsedCommand{
		Raw:       strings.TrimSpace(input),
		Command:   strings.TrimSpace(parts[0]),
		IsPipe:    true,
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		subCmd, err := p.Parse(part)
		if err != nil {
			// If parsing fails, create a simple parsed command
			subCmd = &ParsedCommand{
				Raw:     part,
				Command: part,
			}
		}
		parsed.PipedCommands = append(parsed.PipedCommands, *subCmd)
	}

	// Set the main command from the first piped command
	if len(parsed.PipedCommands) > 0 {
		parsed.Command = parsed.PipedCommands[0].Command
	}

	return parsed, nil
}

// tokenize splits input into tokens respecting quotes
func (p *Parser) tokenize(input string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	var quoteChar rune

	for _, r := range input {
		switch r {
		case '"', '\'':
			if !inQuotes {
				inQuotes = true
				quoteChar = r
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else if r == quoteChar {
				inQuotes = false
				tokens = append(tokens, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		case ' ', '\t':
			if inQuotes {
				current.WriteRune(r)
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseFlag parses a flag from a token
func (p *Parser) parseFlag(token string) *Flag {
	if !strings.HasPrefix(token, "-") {
		return nil
	}

	matches := p.flagPattern.FindStringSubmatch(token)
	if matches == nil {
		return nil
	}

	flag := &Flag{
		Name:    matches[1],
		IsShort: !strings.HasPrefix(token, "--"),
	}

	if len(matches) > 2 && matches[2] != "" {
		flag.Value = matches[2]
	}

	return flag
}

// IsFlag checks if a string is a flag
func (p *Parser) IsFlag(s string) bool {
	return strings.HasPrefix(s, "-")
}

// ExtractCommand extracts the base command from a string
func (p *Parser) ExtractCommand(input string) string {
	tokens := p.tokenize(input)
	if len(tokens) > 0 {
		return tokens[0]
	}
	return ""
}

// ExtractArgs extracts arguments from a command string (excluding flags)
func (p *Parser) ExtractArgs(input string) []string {
	parsed, err := p.Parse(input)
	if err != nil {
		return nil
	}
	return parsed.Args
}

// ExtractFlags extracts all flags from a command string
func (p *Parser) ExtractFlags(input string) []Flag {
	parsed, err := p.Parse(input)
	if err != nil {
		return nil
	}
	return parsed.Flags
}

// ValidateCommand checks if a command string is valid
func (p *Parser) ValidateCommand(input string) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("empty command")
	}

	// Check for unclosed quotes
	inQuotes := false
	var quoteChar rune
	for _, r := range input {
		if r == '"' || r == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = r
			} else if r == quoteChar {
				inQuotes = false
			}
		}
	}

	if inQuotes {
		return fmt.Errorf("unclosed quote in command")
	}

	return nil
}

// CommandInfo holds information about a command
type CommandInfo struct {
	Name        string
	Description string
	Category    string
	Usage       string
	Examples    []string
}

// CommonCommands returns a list of common shell commands
func CommonCommands() []CommandInfo {
	return []CommandInfo{
		// Git commands
		{Name: "git", Description: "Version control system", Category: "vcs", Usage: "git [command]", Examples: []string{"git status", "git add .", "git commit -m 'msg'"}},
		{Name: "git status", Description: "Show working tree status", Category: "vcs", Usage: "git status"},
		{Name: "git add", Description: "Add file contents to index", Category: "vcs", Usage: "git add [file]", Examples: []string{"git add .", "git add filename.txt"}},
		{Name: "git commit", Description: "Record changes to repository", Category: "vcs", Usage: "git commit -m [message]"},
		{Name: "git push", Description: "Update remote refs", Category: "vcs", Usage: "git push [remote] [branch]"},
		{Name: "git pull", Description: "Fetch and merge", Category: "vcs", Usage: "git pull [remote] [branch]"},
		{Name: "git clone", Description: "Clone a repository", Category: "vcs", Usage: "git clone [url]"},
		{Name: "git checkout", Description: "Switch branches", Category: "vcs", Usage: "git checkout [branch]"},
		{Name: "git branch", Description: "List or manage branches", Category: "vcs", Usage: "git branch"},
		{Name: "git log", Description: "Show commit logs", Category: "vcs", Usage: "git log"},
		{Name: "git diff", Description: "Show changes", Category: "vcs", Usage: "git diff"},
		{Name: "git merge", Description: "Join development histories", Category: "vcs", Usage: "git merge [branch]"},
		{Name: "git rebase", Description: "Reapply commits", Category: "vcs", Usage: "git rebase [branch]"},
		{Name: "git stash", Description: "Stash changes", Category: "vcs", Usage: "git stash"},
		{Name: "git reset", Description: "Reset current HEAD", Category: "vcs", Usage: "git reset [mode] [commit]"},
		
		// File operations
		{Name: "ls", Description: "List directory contents", Category: "file", Usage: "ls [options] [path]"},
		{Name: "cd", Description: "Change directory", Category: "file", Usage: "cd [directory]"},
		{Name: "pwd", Description: "Print working directory", Category: "file", Usage: "pwd"},
		{Name: "cat", Description: "Concatenate and print files", Category: "file", Usage: "cat [file]"},
		{Name: "touch", Description: "Change file timestamps", Category: "file", Usage: "touch [file]"},
		{Name: "mkdir", Description: "Make directories", Category: "file", Usage: "mkdir [directory]"},
		{Name: "rm", Description: "Remove files or directories", Category: "file", Usage: "rm [options] [file]"},
		{Name: "cp", Description: "Copy files", Category: "file", Usage: "cp [source] [dest]"},
		{Name: "mv", Description: "Move files", Category: "file", Usage: "mv [source] [dest]"},
		{Name: "find", Description: "Search for files", Category: "file", Usage: "find [path] [expression]"},
		{Name: "grep", Description: "Search text patterns", Category: "file", Usage: "grep [pattern] [file]"},
		{Name: "chmod", Description: "Change file permissions", Category: "file", Usage: "chmod [mode] [file]"},
		{Name: "chown", Description: "Change file owner", Category: "file", Usage: "chown [owner] [file]"},
		{Name: "tar", Description: "Archive files", Category: "file", Usage: "tar [options] [archive] [files]"},
		{Name: "zip", Description: "Package and compress files", Category: "file", Usage: "zip [archive] [files]"},
		{Name: "unzip", Description: "Extract compressed files", Category: "file", Usage: "unzip [archive]"},
		
		// Process management
		{Name: "ps", Description: "Report process status", Category: "process", Usage: "ps [options]"},
		{Name: "top", Description: "Display processes", Category: "process", Usage: "top"},
		{Name: "kill", Description: "Send signal to process", Category: "process", Usage: "kill [pid]"},
		{Name: "killall", Description: "Kill processes by name", Category: "process", Usage: "killall [name]"},
		{Name: "bg", Description: "Resume job in background", Category: "process", Usage: "bg [job]"},
		{Name: "fg", Description: "Resume job in foreground", Category: "process", Usage: "fg [job]"},
		{Name: "jobs", Description: "List active jobs", Category: "process", Usage: "jobs"},
		{Name: "nohup", Description: "Run command immune to hangups", Category: "process", Usage: "nohup [command]"},
		{Name: "nice", Description: "Run with modified scheduling", Category: "process", Usage: "nice [command]"},
		{Name: "renice", Description: "Alter process priority", Category: "process", Usage: "renice [priority] [pid]"},
		
		// Network
		{Name: "curl", Description: "Transfer data from/to server", Category: "network", Usage: "curl [url]"},
		{Name: "wget", Description: "Network downloader", Category: "network", Usage: "wget [url]"},
		{Name: "ping", Description: "Send ICMP echo requests", Category: "network", Usage: "ping [host]"},
		{Name: "netstat", Description: "Network statistics", Category: "network", Usage: "netstat [options]"},
		{Name: "ss", Description: "Investigate sockets", Category: "network", Usage: "ss [options]"},
		{Name: "scp", Description: "Secure copy", Category: "network", Usage: "scp [source] [dest]"},
		{Name: "ssh", Description: "OpenSSH client", Category: "network", Usage: "ssh [user@]hostname"},
		{Name: "telnet", Description: "User interface to TELNET", Category: "network", Usage: "telnet [host] [port]"},
		{Name: "nc", Description: "Netcat - networking utility", Category: "network", Usage: "nc [options] [host] [port]"},
		{Name: "dig", Description: "DNS lookup", Category: "network", Usage: "dig [domain]"},
		{Name: "nslookup", Description: "Query DNS servers", Category: "network", Usage: "nslookup [domain]"},
		{Name: "traceroute", Description: "Trace route to host", Category: "network", Usage: "traceroute [host]"},
		{Name: "iftop", Description: "Display bandwidth usage", Category: "network", Usage: "iftop"},
		
		// Docker
		{Name: "docker", Description: "Container platform", Category: "container", Usage: "docker [command]"},
		{Name: "docker ps", Description: "List containers", Category: "container", Usage: "docker ps [options]"},
		{Name: "docker build", Description: "Build image from Dockerfile", Category: "container", Usage: "docker build [path]"},
		{Name: "docker run", Description: "Run a container", Category: "container", Usage: "docker run [image]"},
		{Name: "docker exec", Description: "Execute command in container", Category: "container", Usage: "docker exec [container] [command]"},
		{Name: "docker stop", Description: "Stop a container", Category: "container", Usage: "docker stop [container]"},
		{Name: "docker rm", Description: "Remove containers", Category: "container", Usage: "docker rm [container]"},
		{Name: "docker rmi", Description: "Remove images", Category: "container", Usage: "docker rmi [image]"},
		{Name: "docker-compose", Description: "Multi-container tool", Category: "container", Usage: "docker-compose [command]"},
		{Name: "docker-compose up", Description: "Create and start containers", Category: "container", Usage: "docker-compose up [options]"},
		{Name: "docker-compose down", Description: "Stop and remove containers", Category: "container", Usage: "docker-compose down"},
		{Name: "docker-compose build", Description: "Build services", Category: "container", Usage: "docker-compose build"},
		{Name: "docker-compose logs", Description: "View output from containers", Category: "container", Usage: "docker-compose logs [service]"},
		
		// Kubernetes
		{Name: "kubectl", Description: "Kubernetes CLI", Category: "k8s", Usage: "kubectl [command]"},
		{Name: "kubectl get", Description: "Display resources", Category: "k8s", Usage: "kubectl get [resource]"},
		{Name: "kubectl apply", Description: "Apply configuration", Category: "k8s", Usage: "kubectl apply -f [file]"},
		{Name: "kubectl describe", Description: "Show resource details", Category: "k8s", Usage: "kubectl describe [resource] [name]"},
		{Name: "kubectl logs", Description: "Print pod logs", Category: "k8s", Usage: "kubectl logs [pod]"},
		{Name: "kubectl exec", Description: "Execute command in container", Category: "k8s", Usage: "kubectl exec [pod] -- [command]"},
		{Name: "kubectl port-forward", Description: "Forward ports", Category: "k8s", Usage: "kubectl port-forward [pod] [ports]"},
		{Name: "kubectl delete", Description: "Delete resources", Category: "k8s", Usage: "kubectl delete [resource] [name]"},
		{Name: "kubectl create", Description: "Create resources", Category: "k8s", Usage: "kubectl create [resource]"},
		{Name: "kubectl config", Description: "Manage kubeconfig", Category: "k8s", Usage: "kubectl config [subcommand]"},
		{Name: "kubectl cluster-info", Description: "Display cluster info", Category: "k8s", Usage: "kubectl cluster-info"},
		{Name: "kubectl rollout", Description: "Manage rollout", Category: "k8s", Usage: "kubectl rollout [subcommand] [resource]"},
		{Name: "kubectl scale", Description: "Set deployment size", Category: "k8s", Usage: "kubectl scale [resource] --replicas=[n]"},
		
		// Node.js / NPM
		{Name: "npm", Description: "Node package manager", Category: "nodejs", Usage: "npm [command]"},
		{Name: "npm install", Description: "Install packages", Category: "nodejs", Usage: "npm install [package]"},
		{Name: "npm run", Description: "Run package scripts", Category: "nodejs", Usage: "npm run [script]"},
		{Name: "npm start", Description: "Start package", Category: "nodejs", Usage: "npm start"},
		{Name: "npm test", Description: "Run tests", Category: "nodejs", Usage: "npm test"},
		{Name: "npm build", Description: "Build package", Category: "nodejs", Usage: "npm run build"},
		{Name: "npx", Description: "Execute Node packages", Category: "nodejs", Usage: "npx [package]"},
		{Name: "node", Description: "Node.js runtime", Category: "nodejs", Usage: "node [script]"},
		{Name: "yarn", Description: "Yarn package manager", Category: "nodejs", Usage: "yarn [command]"},
		{Name: "pnpm", Description: "Fast package manager", Category: "nodejs", Usage: "pnpm [command]"},
		
		// Python
		{Name: "python", Description: "Python interpreter", Category: "python", Usage: "python [script]"},
		{Name: "python3", Description: "Python 3 interpreter", Category: "python", Usage: "python3 [script]"},
		{Name: "pip", Description: "Python package manager", Category: "python", Usage: "pip [command]"},
		{Name: "pip install", Description: "Install Python packages", Category: "python", Usage: "pip install [package]"},
		{Name: "pip freeze", Description: "List installed packages", Category: "python", Usage: "pip freeze"},
		{Name: "pytest", Description: "Python testing framework", Category: "python", Usage: "pytest [options]"},
		{Name: "virtualenv", Description: "Create virtual environment", Category: "python", Usage: "virtualenv [dir]"},
		{Name: "python -m venv", Description: "Create virtual environment", Category: "python", Usage: "python -m venv [dir]"},
		{Name: "jupyter", Description: "Jupyter notebook", Category: "python", Usage: "jupyter [subcommand]"},
		{Name: "jupyter notebook", Description: "Start Jupyter notebook", Category: "python", Usage: "jupyter notebook"},
		{Name: "conda", Description: "Conda package manager", Category: "python", Usage: "conda [command]"},
		{Name: "poetry", Description: "Python dependency manager", Category: "python", Usage: "poetry [command]"},
		
		// Go
		{Name: "go", Description: "Go toolchain", Category: "go", Usage: "go [command]"},
		{Name: "go build", Description: "Compile packages", Category: "go", Usage: "go build [packages]"},
		{Name: "go run", Description: "Compile and run", Category: "go", Usage: "go run [files]"},
		{Name: "go test", Description: "Run tests", Category: "go", Usage: "go test [packages]"},
		{Name: "go get", Description: "Download packages", Category: "go", Usage: "go get [packages]"},
		{Name: "go mod", Description: "Module maintenance", Category: "go", Usage: "go mod [command]"},
		{Name: "go mod init", Description: "Initialize module", Category: "go", Usage: "go mod init [module]"},
		{Name: "go mod tidy", Description: "Add/remove dependencies", Category: "go", Usage: "go mod tidy"},
		{Name: "go mod download", Description: "Download modules", Category: "go", Usage: "go mod download"},
		{Name: "go fmt", Description: "Format Go source", Category: "go", Usage: "go fmt [packages]"},
		{Name: "go vet", Description: "Report likely errors", Category: "go", Usage: "go vet [packages]"},
		{Name: "go install", Description: "Compile and install", Category: "go", Usage: "go install [packages]"},
		
		// System
		{Name: "sudo", Description: "Execute as superuser", Category: "system", Usage: "sudo [command]"},
		{Name: "su", Description: "Substitute user", Category: "system", Usage: "su [user]"},
		{Name: "passwd", Description: "Change password", Category: "system", Usage: "passwd [user]"},
		{Name: "whoami", Description: "Print current user", Category: "system", Usage: "whoami"},
		{Name: "id", Description: "Print user identity", Category: "system", Usage: "id [user]"},
		{Name: "uname", Description: "Print system info", Category: "system", Usage: "uname [options]"},
		{Name: "df", Description: "Report disk space", Category: "system", Usage: "df [options]"},
		{Name: "du", Description: "Estimate file space", Category: "system", Usage: "du [options] [file]"},
		{Name: "free", Description: "Display memory", Category: "system", Usage: "free [options]"},
		{Name: "uptime", Description: "Show uptime", Category: "system", Usage: "uptime"},
		{Name: "crontab", Description: "Schedule commands", Category: "system", Usage: "crontab [file]"},
		{Name: "systemctl", Description: "Control systemd", Category: "system", Usage: "systemctl [command] [unit]"},
		{Name: "service", Description: "Run system V init scripts", Category: "system", Usage: "service [name] [command]"},
		{Name: "journalctl", Description: "Query systemd journal", Category: "system", Usage: "journalctl [options]"},
		{Name: "env", Description: "Run in modified environment", Category: "system", Usage: "env [command]"},
		{Name: "export", Description: "Set environment variable", Category: "system", Usage: "export VAR=value"},
		{Name: "source", Description: "Execute commands from file", Category: "system", Usage: "source [file]"},
		{Name: "alias", Description: "Create command alias", Category: "system", Usage: "alias [name]=[command]"},
		{Name: "history", Description: "Show command history", Category: "system", Usage: "history"},
		{Name: "clear", Description: "Clear terminal screen", Category: "system", Usage: "clear"},
		{Name: "exit", Description: "Exit shell", Category: "system", Usage: "exit [n]"},
	}
}

// GetCommandNames returns a list of all command names
func GetCommandNames() []string {
	commands := CommonCommands()
	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Name
	}
	return names
}

// FindCommand finds a command by name
func FindCommand(name string) *CommandInfo {
	for _, cmd := range CommonCommands() {
		if cmd.Name == name {
			return &cmd
		}
	}
	return nil
}
