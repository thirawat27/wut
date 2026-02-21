<div align="center">

# ⚡ WUT (What ?)

### The Smart Command Line Assistant That Actually Understands You

*Stop memorizing commands. Start getting things done.*

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://golang.org)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux%20%7C%20BSD-blue)]
[![Release](https://img.shields.io/github/v/release/thirawat27/wut)](https://github.com/thirawat27/wut/releases)

[Features](#key-features) • [Install](#installation) • [Quick Start](#getting-started) • [Commands](#command-reference) • [Docs](#configuration)

</div>

---

**WUT** is an intelligent command-line assistant that transforms how you work in the terminal. It suggests commands based on context, fixes typos instantly, explains complex operations, and learns from your workflow—all while keeping your data private and local.

## Table of Contents

- [Key Features](#key-features)
- [Installation](#installation)
- [Getting Started](#getting-started)
- [Command Reference](#command-reference)
- [Configuration](#configuration)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)

## Key Features

- **Smart Command Suggestions**: Context-aware command recommendations based on your project type and history
- **Typo Correction**: Automatically detect and fix common command typos
- **Command Explanations**: Get detailed breakdowns of what commands do and their potential risks
- **TLDR Integration**: Quick access to practical command examples from the TLDR pages database
- **History Tracking**: Learn from your command usage patterns
- **Shell Integration**: Quick access via keyboard shortcuts (Ctrl+Space)
- **Cross-Platform**: Works on Windows, macOS, Linux, and BSD systems
- **Privacy-Focused**: All processing happens locally on your machine

## Installation

### Windows

#### Option 1: GUI Installer (Recommended for Beginners)

1. Download `wut-setup.exe` from the [latest release](https://github.com/thirawat27/wut/releases/latest)
2. Double-click the installer and follow the setup wizard
3. Open a new PowerShell or Command Prompt window
4. Verify installation:
   ```powershell
   wut --version
   ```

#### Option 2: PowerShell Script

Open PowerShell and run:

```powershell
irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
```

This will automatically download, install, and configure WUT for your system.

#### Option 3: WinGet Package Manager

```powershell
winget install thirawat27.wut
```

### macOS

#### Option 1: Installation Script (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
```

#### Option 2: Homebrew (Coming Soon)

```bash
brew install wut
```

### Linux

#### Installation Script

```bash
curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
```

The script will:
- Detect your system architecture
- Download the appropriate binary
- Install it to `/usr/local/bin`
- Set up shell integration
- Initialize configuration

#### Manual Installation

1. Download the binary for your architecture from [releases](https://github.com/thirawat27/wut/releases/latest)
2. Extract and move to your PATH:
   ```bash
   tar -xzf wut-linux-amd64.tar.gz
   sudo mv wut /usr/local/bin/
   sudo chmod +x /usr/local/bin/wut
   ```

### Installation Options

All installation scripts support these options:

| Option | Description | Example |
|--------|-------------|---------|
| `--version` / `-Version` | Install specific version | `--version v1.0.0` |
| `--no-init` / `-NoInit` | Skip automatic initialization | `--no-init` |
| `--no-shell` / `-NoShell` | Skip shell integration | `--no-shell` |
| `--force` / `-Force` | Overwrite existing installation | `--force` |

Example with options:
```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash -s -- --version v1.0.0 --no-init

# Windows
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1))) -Version v1.0.0 -NoInit
```

### Docker

```bash
# Pull the image
docker pull ghcr.io/thirawat27/wut:latest

# Run WUT
docker run --rm -it ghcr.io/thirawat27/wut:latest suggest

# With persistent configuration
docker run --rm -it \
  -v ~/.wut:/home/wut/.wut \
  -v ~/.config/wut:/home/wut/.config/wut \
  ghcr.io/thirawat27/wut:latest
```

### Build from Source

Requirements:
- Go 1.21 or higher
- Git
- Make (optional)

```bash
# Clone the repository
git clone https://github.com/thirawat27/wut.git
cd wut

# Build using Make
make build

# Or build directly with Go
go build -o wut .

# Install to system
sudo mv wut /usr/local/bin/
```

## Getting Started

### Initial Setup

After installation, run the initialization command:

```bash
# Interactive setup (recommended for first-time users)
wut init

# Quick setup with defaults
wut init --quick
```

The initialization process will:
1. Create configuration directories
2. Set up your preferred theme
3. Detect and configure shell integration
4. Optionally download TLDR pages for offline use

### Shell Integration

Enable keyboard shortcuts and enhanced features:

```bash
# Auto-detect your shell and install integration
wut install

# Install for a specific shell
wut install --shell bash
wut install --shell zsh
wut install --shell fish

# Install for all detected shells
wut install --all
```

After installation, these keyboard shortcuts will be available:
- **Ctrl+Space**: Open WUT interactive mode
- **Ctrl+G**: Open WUT with the current command line pre-filled

To remove shell integration:
```bash
wut install --uninstall
```

### First Commands

Try these commands to get familiar with WUT:

```bash
# Get command suggestions interactively
wut suggest

# Search for a specific command
wut suggest git

# Fix a typo
wut fix "gti status"

# Explain what a command does
wut explain "docker-compose up -d"

# Get smart suggestions based on your project
wut smart
```

## Command Reference

### Command Shortcuts

WUT provides convenient shortcuts for faster typing:

| Shortcut | Full Command | Description |
|----------|--------------|-------------|
| `wut s` | `wut suggest` | Get command suggestions |
| `wut h` | `wut history` | View command history |
| `wut x` | `wut explain` | Explain a command |
| `wut a` | `wut alias` | Manage aliases |
| `wut c` | `wut config` | Manage configuration |
| `wut t` | `wut tldr` | TLDR pages management |
| `wut f` | `wut fix` | Fix command typos |
| `wut ?` | `wut smart` | Smart suggestions |

### 1. Suggest Command

Get command suggestions and examples from the TLDR pages database.

```bash
# Interactive mode with live search
wut suggest
wut s

# Get help for a specific command
wut suggest git
wut s docker

# Output in plain text format
wut suggest npm --raw

# Show only command examples (no descriptions)
wut suggest git --quiet

# Force offline mode (use local database only)
wut suggest docker --offline

# Limit number of examples shown
wut suggest git --limit 5
```

**Interactive Mode Features:**
- Type to search through thousands of commands
- Arrow keys to navigate
- Enter to view detailed examples
- Esc to exit

### 2. Fix Command

Automatically detect and correct typos in commands.

```bash
# Fix a typo
wut fix "gti status"
wut f "doker ps"

# Check for dangerous commands
wut fix "rm -rf /"

# List common typos that WUT can fix
wut fix --list

# Copy corrected command to clipboard
wut fix "gti push" --copy
```

**Common Typos Detected:**
- `gti` → `git`
- `doker` → `docker`
- `cd..` → `cd ..`
- `grpe` → `grep`
- `npn` → `npm`
- And many more...

### 3. Explain Command

Get detailed explanations of what commands do, including warnings for dangerous operations.

```bash
# Explain a command
wut explain "git rebase -i HEAD~3"
wut x "kubectl apply -f deployment.yaml"

# Get verbose explanation with more details
wut explain "docker build -t myapp ." --verbose

# Check specifically for dangerous commands
wut explain "rm -rf /" --dangerous
```

**Explanation Includes:**
- Command summary and description
- Argument and flag explanations
- Usage examples
- Safety warnings for dangerous operations
- Alternative commands
- Helpful tips

### 4. Smart Command

Get intelligent, context-aware suggestions based on your project type and command history.

```bash
# Get suggestions for current project
wut smart
wut ?

# Search with a query
wut ? "how to find large files"
wut ? "compress folder"

# Limit number of suggestions
wut smart --limit 5

# Execute selected command immediately
wut smart --exec

# Disable typo correction
wut smart --correct=false
```

**Context Detection:**
WUT automatically detects your project type and provides relevant suggestions:
- **Go projects**: `go mod tidy`, `go test ./...`, `go build`
- **Node.js projects**: `npm install`, `npm run dev`, `npm test`
- **Docker projects**: `docker-compose up`, `docker build`
- **Git repositories**: Branch info, commit status, push/pull suggestions

### 5. History Command

Track and analyze your command usage patterns.

```bash
# View recent commands
wut history
wut h

# Show usage statistics
wut h --stats

# Search command history
wut h --search "docker"
wut h --search "git commit"

# Import commands from shell history
wut h --import-shell

# Clear history
wut h --clear

# Export history to file
wut h --export history.json
```

### 6. Alias Command

Manage command aliases for frequently used commands.

```bash
# List all aliases
wut alias
wut a

# Add a new alias
wut a --add --name gs --command "git status"
wut a --add --name dc --command "docker-compose"

# Remove an alias
wut a --remove gs

# Generate smart aliases based on your project
wut a --generate

# Export aliases to shell format
wut a --export bash > ~/.bash_aliases
wut a --export zsh > ~/.zsh_aliases
```

### 7. Config Command

Manage WUT configuration settings.

```bash
# Show all configuration
wut config
wut c

# Get a specific value
wut config --get ui.theme
wut c -g fuzzy.threshold

# Set a configuration value
wut config --set ui.theme dark
wut c -s history.enabled --value true

# Edit configuration file in default editor
wut config --edit

# Reset to default configuration
wut config --reset

# Export configuration
wut config --export backup.yaml

# Import configuration
wut config --import backup.yaml
```

### 8. TLDR Command

Manage the TLDR pages database for offline use.

```bash
# Download popular commands
wut tldr sync
wut t sync

# Download specific commands
wut tldr sync git docker npm kubectl

# Download all available commands
wut tldr sync --all

# Check database status
wut tldr status

# Update existing database
wut tldr update

# Clear local database
wut tldr clear
```

### 9. Install Command

Manage shell integration.

```bash
# Auto-detect and install for current shell
wut install

# Install for specific shell
wut install --shell bash
wut install --shell zsh
wut install --shell fish
wut install --shell powershell

# Install for all detected shells
wut install --all

# Uninstall shell integration
wut install --uninstall

# Show installation status
wut install --status
```

## Configuration

### Configuration File Location

WUT stores its configuration in:
- **Linux/macOS**: `~/.config/wut/config.yaml`
- **Windows**: `%USERPROFILE%\.config\wut\config.yaml`
- **XDG**: `$XDG_CONFIG_HOME/wut/config.yaml`

### Available Configuration Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `ui.theme` | string | `auto` | Theme: `auto`, `dark`, `light` |
| `ui.show_confidence` | bool | `true` | Show confidence scores |
| `ui.show_explanations` | bool | `true` | Show detailed explanations |
| `fuzzy.enabled` | bool | `true` | Enable fuzzy matching |
| `fuzzy.threshold` | float | `0.7` | Fuzzy match threshold (0-1) |
| `history.enabled` | bool | `true` | Track command history |
| `history.max_entries` | int | `1000` | Maximum history entries |
| `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `tldr.auto_sync` | bool | `false` | Auto-sync TLDR pages |
| `tldr.cache_duration` | string | `24h` | Cache duration for TLDR pages |
| `context.enabled` | bool | `true` | Enable context analysis |

### Example Configuration

```yaml
app:
  debug: false

ui:
  theme: dark
  show_confidence: true
  show_explanations: true

fuzzy:
  enabled: true
  threshold: 0.7

history:
  enabled: true
  max_entries: 1000

logging:
  level: info
  file: ~/.config/wut/wut.log

tldr:
  auto_sync: false
  cache_duration: 24h

context:
  enabled: true

database:
  path: ~/.config/wut/wut.db
```

### Environment Variables

Override configuration with environment variables:

```bash
# Set theme
export WUT_THEME=dark

# Enable debug mode
export WUT_DEBUG=true

# Force Unicode support
export WUT_FORCE_UNICODE=1

# Force emoji support
export WUT_FORCE_EMOJI=1

# Set log level
export WUT_LOG_LEVEL=debug
```

## Advanced Usage

### Piping and Scripting

WUT can be used in scripts and pipelines:

```bash
# Get command and pipe to execution
wut suggest git --quiet | head -1 | bash

# Fix typo and execute
FIXED=$(wut fix "gti status" --quiet)
eval $FIXED

# Export history for analysis
wut history --export history.json
cat history.json | jq '.commands[] | select(.count > 10)'
```

### Custom Workflows

Create custom workflows by combining WUT commands:

```bash
# Create a deployment script
#!/bin/bash
echo "Checking project context..."
wut smart

echo "Running tests..."
wut ? "run tests" --exec

echo "Building..."
wut ? "build" --exec

echo "Deploying..."
wut ? "deploy" --exec
```

### Integration with Other Tools

WUT works well with other command-line tools:

```bash
# Use with fzf for enhanced search
wut history | fzf

# Combine with ripgrep
wut suggest | rg "docker"

# Use with watch for monitoring
watch -n 5 'wut smart --limit 3'
```

## Troubleshooting

### Common Issues

#### Command Not Found After Installation

**Windows:**
1. Close and reopen your terminal
2. Check if WUT is in PATH:
   ```powershell
   $env:PATH -split ';' | Select-String 'WUT'
   ```
3. If not found, add manually:
   ```powershell
   [Environment]::SetEnvironmentVariable("PATH", "$env:PATH;C:\Program Files\WUT", "User")
   ```

**Linux/macOS:**
1. Check if binary exists:
   ```bash
   which wut
   ```
2. If not found, ensure `/usr/local/bin` is in PATH:
   ```bash
   echo $PATH
   export PATH="/usr/local/bin:$PATH"
   ```

#### Windows SmartScreen Warning

When running the installer, Windows may show a protection warning:
1. Click "More info"
2. Click "Run anyway"

This is a false positive common with new executables. The software is safe.

#### Permission Denied (Linux/macOS)

```bash
# Make binary executable
chmod +x /usr/local/bin/wut

# Or install with sudo
sudo mv wut /usr/local/bin/
```

#### Shell Integration Not Working

```bash
# Reinstall shell integration
wut install --uninstall
wut install

# Reload shell configuration
source ~/.bashrc  # Bash
source ~/.zshrc   # Zsh
```

#### TLDR Pages Not Found

```bash
# Download TLDR database
wut tldr sync

# Check status
wut tldr status

# Force re-download
wut tldr clear
wut tldr sync --all
```

#### Configuration Reset

If WUT behaves unexpectedly, reset configuration:

```bash
# Reset to defaults
wut config --reset

# Or manually delete config
rm -rf ~/.config/wut
wut init --quick
```

### Debug Mode

Enable debug mode for detailed logging:

```bash
# Via flag
wut --debug suggest

# Via environment variable
export WUT_DEBUG=true
wut suggest

# Via configuration
wut config --set logging.level debug
```

### Getting Help

- **Bug Reports**: [GitHub Issues](https://github.com/thirawat27/wut/issues)
- **Feature Requests**: [GitHub Issues](https://github.com/thirawat27/wut/issues)
- **Discussions**: [GitHub Discussions](https://github.com/thirawat27/wut/discussions)
- **Documentation**: [GitHub Wiki](https://github.com/thirawat27/wut/wiki)

## Performance

WUT is designed to be fast and lightweight:

- **Startup Time**: < 50ms
- **Suggestion Response**: < 20ms  
- **Memory Usage**: < 20MB
- **Binary Size**: ~10-15MB
- **Database Size**: ~5MB (with TLDR pages)

## Security and Privacy

- All processing runs locally on your machine
- No data is sent to external servers
- Command history stored locally in SQLite database
- Optional encryption for sensitive data
- Open source - audit the code yourself

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Run tests: `make test`
5. Format code: `make fmt`
6. Commit changes: `git commit -m 'Add amazing feature'`
7. Push to branch: `git push origin feature/amazing-feature`
8. Open a Pull Request

Please ensure:
- Code follows Go best practices
- All tests pass
- Code is properly formatted
- Documentation is updated

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

WUT is built with these excellent open-source projects:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for terminal
- [TLDR Pages](https://tldr.sh/) - Community-driven command examples

## Support the Project

If you find WUT useful, please consider:
- ⭐ Starring the repository
- 🐛 Reporting bugs
- 💡 Suggesting features
- 📖 Improving documentation
- 🔀 Contributing code

---

Made with ❤️ by [@thirawat27](https://github.com/thirawat27)
