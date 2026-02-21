# WUT - Command Helper

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://golang.org)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux%20%7C%20BSD-blue)](https://github.com/thirawat27/wut/releases)

WUT is a production-ready, cross-platform, intelligent command line assistant that helps you find the right commands, correct typos, and learn new shell commands through natural language queries.

## ✨ Features

- 🚀 **Smart Suggestions** - Get command suggestions with TUI interface
- 🔧 **Command Fixing** - Auto-correct typos in your commands
- 📚 **TLDR Integration** - Access TLDR pages for quick command help
- 📜 **History Tracking** - Track and analyze your command usage
- 🐚 **Shell Integration** - Key bindings for quick access (Ctrl+Space)
- ⚙️ **Flexible Config** - Easy configuration management with dot notation
- 🎨 **Adaptive UI** - Auto-detect terminal capabilities
- 🔒 **Privacy First** - All processing happens locally

## 🌍 Cross-Platform Support

### Operating Systems
- **Windows**: Windows 10/11, Windows Server (amd64, arm64, 386)
- **macOS**: 10.15+ Intel & Apple Silicon (amd64, arm64)
- **Linux**: All distributions (amd64, arm64, arm, 386, riscv64)
- **BSD**: FreeBSD, OpenBSD, NetBSD (amd64, arm64)

### Shells
- Bash, Zsh, Fish, PowerShell, Nushell, Elvish, Xonsh, Tcsh/Csh, Ksh

## 🚀 Quick Start

### Installation

**One-Line Install (Recommended):**

```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash

# Windows (PowerShell):
irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
```

**Install with Auto-Setup:**

```bash
# Linux/macOS - Install and run initialization
curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash -s -- --init

# Windows - Install with initialization
irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex -Init
```

**Package Managers:**

```bash
# Chocolatey (Windows)
choco install wut

# WinGet (Windows)
winget install thirawat27.wut

# Go Install
go install github.com/thirawat27/wut@latest

# Docker
docker pull ghcr.io/thirawat27/wut:latest
```

### First-Time Setup

After installation, run the initialization wizard:

```bash
wut init              # Interactive setup
wut init --quick      # Quick setup with defaults
```

This will:
- Create configuration directories
- Set up your preferred theme
- Detect and configure shell integration
- Optionally download TLDR pages

## 📖 Usage

### Command Shortcuts

WUT provides convenient shortcuts for common commands:

| Shortcut | Command | Description |
|----------|---------|-------------|
| `wut s` | `wut suggest` | Get command suggestions |
| `wut h` | `wut history` | View command history |
| `wut x` | `wut explain` | Explain a command |
| `wut a` | `wut alias` | Manage aliases |
| `wut c` | `wut config` | Manage configuration |
| `wut t` | `wut tldr` | TLDR pages management |
| `wut f` | `wut fix` | Fix command typos |
| `wut ?` | `wut smart` | Smart command suggestions |

### Get Command Suggestions

```bash
# Interactive TUI mode
wut suggest
wut s

# Get specific command help
wut suggest git
wut s docker

# Quiet mode (pipe-friendly)
wut suggest git --quiet

# Raw text output
wut suggest git --raw
```

### Fix Typos

```bash
# Correct typos in commands
wut fix "gti status"
wut f "doker ps"

# List common typos
wut fix --list
```

### View Command History

```bash
# View recent commands
wut history
wut h

# Show statistics
wut h --stats

# Search history
wut h --search "docker"

# Import from shell history
wut h --import-shell
```

### Explain Commands

```bash
# Explain a command
wut explain "git rebase -i"
wut x "kubectl apply -f deployment.yaml"

# Check for dangerous commands
wut x "rm -rf /"
```

### Smart Suggestions

```bash
# Get context-aware suggestions
wut smart
wut ?

# Query-based suggestions
wut ? "how to find large files"
wut ? "compress folder to tar.gz"
```

### Manage Aliases

```bash
# List aliases
wut alias
wut a

# Generate smart aliases for current project
wut a --generate

# Add custom alias
wut a --add --name gs --command "git status"
```

## ⚙️ Configuration

WUT provides a powerful configuration system with dot notation support.

### View Configuration

```bash
wut config              # Show all config
wut c                   # Shortcut
```

### Get/Set Configuration

```bash
# Get a value
wut config --get ui.theme
wut c -g ui.theme

# Set a value
wut config --set ui.theme dark
wut c -s ui.theme --value dark

# Enable/disable features
wut c -s fuzzy.enabled --value true
wut c -s history.enabled --value false
```

### Available Config Keys

| Key | Type | Description |
|-----|------|-------------|
| `ui.theme` | string | Theme: auto, dark, light |
| `ui.show_confidence` | bool | Show confidence scores |
| `fuzzy.enabled` | bool | Enable fuzzy matching |
| `fuzzy.threshold` | float | Fuzzy match threshold (0-1) |
| `history.enabled` | bool | Track command history |
| `history.max_entries` | int | Max history entries |
| `logging.level` | string | Log level: debug, info, warn, error |
| `tldr.auto_sync` | bool | Auto-sync TLDR pages |
| `context.enabled` | bool | Enable context analysis |

### Edit Configuration File

```bash
# Open in default editor
wut config --edit

# Import/Export
wut config --import backup.yaml
wut config --export backup.yaml
```

### Configuration File Locations

- **Linux/macOS**: `~/.config/wut/config.yaml`
- **Windows**: `%USERPROFILE%\.config\wut\config.yaml`
- **XDG**: `$XDG_CONFIG_HOME/wut/config.yaml`

## 🐚 Shell Integration

### Install Shell Integration

```bash
# Auto-detect and install
wut install

# Install for specific shell
wut install --shell zsh

# Install for all detected shells
wut install --all
```

### Key Bindings

After installation, these key bindings are available:

| Key | Action |
|-----|--------|
| `Ctrl+Space` | Open WUT TUI |
| `Ctrl+G` | Open WUT with current command line |

### Uninstall

```bash
wut install --uninstall
```

## 📚 TLDR Pages

### Sync TLDR Pages

```bash
# Download popular commands
wut tldr sync
wut t sync

# Download specific commands
wut tldr sync git docker npm

# Download all commands
wut tldr sync --all
```

### Check Status

```bash
wut tldr status
```

## 🐳 Docker

```bash
# Run with Docker
docker run --rm -it ghcr.io/thirawat27/wut:latest suggest

# With persistent config
docker run --rm -it \
  -v ~/.wut:/home/wut/.wut \
  -v ~/.config/wut:/home/wut/.config/wut \
  ghcr.io/thirawat27/wut:latest
```

## 🏗️ Development

### Prerequisites
- Go 1.21+
- Make
- Git

### Build from Source

```bash
# Clone repository
git clone https://github.com/thirawat27/wut
cd wut

# Build for current platform
make build

# Build for all platforms
make build-all
```

### Run Tests

```bash
make test
make test-coverage
```

## 📊 Performance

- **Startup Time**: < 50ms
- **Suggestion Response**: < 20ms
- **Memory Usage**: < 20MB
- **Binary Size**: ~10-15MB

## 🔒 Security & Privacy

- All processing runs **locally**
- No data sent to external servers
- Command history stored locally
- Optional encryption for sensitive data

## 🛠️ Troubleshooting

### Terminal Not Detected Correctly

```bash
export WUT_THEME=truecolor
export WUT_FORCE_UNICODE=1
export WUT_FORCE_EMOJI=1
```

### Reset Configuration

```bash
wut config --reset
```

### Windows-Specific Issues

```powershell
# Execution policy
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure:
- Code follows Go best practices
- Tests pass (`make test`)
- Code is formatted (`make fmt`)
- Linting passes (`make lint`)

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [TLDR Pages](https://tldr.sh/) - Command line help database

## 📞 Support

- 🐛 [Report Bug](https://github.com/thirawat27/wut/issues)
- 💡 [Request Feature](https://github.com/thirawat27/wut/issues)
- 💬 [Discussions](https://github.com/thirawat27/wut/discussions)

---

<p align="center">
  <strong>Cross-Platform • Universal • Intelligent</strong><br>
  Made with ❤️ by <a href="https://github.com/thirawat27">@thirawat27</a>
</p>
