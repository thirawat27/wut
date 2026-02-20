# WUT - AI-Powered Command Helper

[![CI](https://github.com/thirawat27/wut/actions/workflows/ci.yml/badge.svg)](https://github.com/thirawat27/wut/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/thirawat27/wut)](https://github.com/thirawat27/wut/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://golang.org)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux%20%7C%20BSD-blue)](https://github.com/thirawat27/wut/releases)

WUT is a production-ready, cross-platform, intelligent command line assistant that helps you find the right commands, correct typos, and learn new shell commands through natural language queries.

## 🌍 Cross-Platform Support

WUT works on **all major platforms** and **all popular terminals**:

### Operating Systems
- **Windows**: Windows 10/11, Windows Server (amd64, arm64, 386)
- **macOS**: 10.15+ Intel & Apple Silicon (amd64, arm64)
- **Linux**: All distributions (amd64, arm64, arm, 386, riscv64)
- **BSD**: FreeBSD, OpenBSD, NetBSD (amd64, arm64)

### Shells
- **Bash** (Linux, macOS, WSL)
- **Zsh** (macOS default, popular on Linux)
- **Fish** (Modern, user-friendly)
- **PowerShell** (Windows, cross-platform)
- **Nushell** (Modern structured shell)
- **Elvish** (Expressive programming language shell)
- **Xonsh** (Python-powered shell)
- **Tcsh/Csh** (BSD, legacy systems)
- **Ksh** (AIX, commercial Unix)

### Terminals
- ✅ Windows Terminal (Recommended for Windows)
- ✅ iTerm2 (Recommended for macOS)
- ✅ Alacritty, WezTerm, Kitty (GPU-accelerated)
- ✅ GNOME Terminal, Konsole, Terminal.app
- ✅ Tmux, Screen (Terminal multiplexers)
- ✅ VS Code Terminal, JetBrains Terminal
- ✅ Any terminal with basic VT100 support

## 🚀 Quick Install

### One-Line Install (Recommended)

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
```

### Package Managers

**Chocolatey (Windows):** ⭐ Recommended
```powershell
choco install wut
```

**Snap (Linux):**
```bash
sudo snap install wut
```

**Go Install:**
```bash
go install github.com/thirawat27/wut@latest
```

**Docker:**
```bash
docker pull ghcr.io/thirawat27/wut:latest
```

**Nix (NixOS/Linux/macOS):**
```bash
nix profile install github:thirawat27/wut
```

**AUR (Arch Linux):**
```bash
yay -S wut
# or
paru -S wut
```

### Pre-built Binaries

Download from [Releases](https://github.com/thirawat27/wut/releases) for your platform:

| Platform | Architectures |
|----------|--------------|
| Windows | amd64, arm64, 386 |
| macOS | amd64, arm64 (Apple Silicon) |
| Linux | amd64, arm64, arm, 386, riscv64 |
| FreeBSD | amd64, arm64 |
| OpenBSD | amd64 |
| NetBSD | amd64 |

## 📖 Usage

### Get Command Suggestions

```bash
# Basic suggestion
wut suggest "git push"

# Interactive mode (TUI)
wut suggest

# Limit results
wut suggest "docker" --limit 3

# Quiet mode (pipe-friendly)
wut suggest "git psuh" --quiet
```

### View Command History

```bash
# View recent commands
wut history

# Show statistics
wut history --stats

# Search history
wut history --search "docker"

# Export/Import
wut history --export backup.json
wut history --import backup.json
```

### Explain Commands

```bash
# Explain a command
wut explain "git rebase -i"

# Detailed explanation
wut explain "kubectl apply -f deployment.yaml" --verbose

# Check for dangerous commands
wut explain "rm -rf /"
```

### Train AI Model

```bash
# Quick training
wut train

# Custom training
wut train --epochs 200 --learning-rate 0.005

# Force training with insufficient data
wut train --force
```

### Shell Integration

```bash
# Install for current shell
wut install

# Install for specific shell
wut install --shell zsh

# Install for all detected shells
wut install --all

# Uninstall
wut install --uninstall
```

## ⚙️ Terminal Adaptation

WUT automatically detects your terminal capabilities and adapts:

### Automatic Detection
- ✅ Color support (basic, 256, true color)
- ✅ Unicode and emoji support
- ✅ Terminal type and features
- ✅ Screen size and capabilities

### Adaptive Output

**Basic Terminals** (dumb, limited):
- ASCII-only output
- No colors or minimal colors
- Simple text layout

**Modern Terminals** (Windows Terminal, iTerm2, Alacritty):
- Full emoji support
- True color (24-bit)
- Beautiful TUI with Bubble Tea
- Nerd Font icons (optional)

**Configure manually if needed:**
```yaml
# ~/.config/wut/config.yaml
ui:
  theme: "auto"        # auto, none, basic, 256, truecolor
  force_unicode: false
  force_emoji: false
  use_nerd_fonts: false
```

## 🔧 Configuration

Configuration file locations:
- **Linux/macOS**: `~/.config/wut/config.yaml`
- **Windows**: `%USERPROFILE%\.config\wut\config.yaml`
- **XDG**: `$XDG_CONFIG_HOME/wut/config.yaml`

### Example Configuration

```yaml
app:
  name: "wut"
  version: "1.0.0"
  debug: false

ai:
  enabled: true
  model:
    type: "tiny_neural_network"
    embedding_dimensions: 64
    hidden_layers: 2
    hidden_units: 64
    quantized: true
  training:
    epochs: 100
    learning_rate: 0.01
    batch_size: 32
  inference:
    max_suggestions: 5
    confidence_threshold: 0.7

ui:
  theme: "auto"              # auto, none, basic, 256, truecolor
  show_confidence: true
  show_explanations: true
  syntax_highlighting: true
  # Terminal adaptation
  force_unicode: false       # Force Unicode even if not detected
  force_emoji: false         # Force emoji even if not detected
  use_nerd_fonts: auto       # auto, true, false

# Shell-specific settings
shell:
  bash:
    key_binding: "ctrl+space"
  zsh:
    key_binding: "ctrl+space"
  fish:
    key_binding: "ctrl+space"
  powershell:
    key_binding: "ctrl+space"
```

## 🐳 Docker

### Run with Docker
```bash
docker run --rm -it ghcr.io/thirawat27/wut:latest suggest
```

### Docker Compose
```yaml
version: '3.8'
services:
  wut:
    image: ghcr.io/thirawat27/wut:latest
    volumes:
      - ~/.wut:/home/wut/.wut
      - ~/.config/wut:/home/wut/.config/wut
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

# Build for specific platform
make build-windows
make build-linux
make build-macos
```

### Run Tests
```bash
make test
make test-coverage
```

### Development Mode
```bash
# With hot reload (requires air)
make dev

# Or manually
go run .
```

## 📊 Performance

- **Startup Time**: < 50ms
- **Suggestion Response**: < 20ms
- **Memory Usage**: < 20MB
- **Binary Size**: ~10-15MB
- **Model Size**: ~2-3MB

## 🔒 Security & Privacy

- All AI processing runs **locally**
- No data sent to external servers
- Command history stored locally
- Optional encryption for sensitive data
- Configurable anonymization

## 🛠️ Troubleshooting

### Terminal Not Detected Correctly
```bash
# Force specific terminal capabilities
export WUT_THEME=truecolor
export WUT_FORCE_UNICODE=1
export WUT_FORCE_EMOJI=1
```

### Shell Integration Not Working
```bash
# Manually add to your shell config
# Bash (~/.bashrc):
eval "$(wut completion bash)"

# Zsh (~/.zshrc):
eval "$(wut completion zsh)"

# Fish (~/.config/fish/config.fish):
wut completion fish | source
```

### Windows-Specific Issues
```powershell
# Execution policy
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# Windows Defender may flag - add exclusion
# Or use: Windows Security > Virus & threat protection > Exclusions
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
- [Charmbracelet Log](https://github.com/charmbracelet/log) - Structured logging
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [BBolt](https://github.com/etcd-io/bbolt) - Embedded database

## 📞 Support

- 🐛 [Report Bug](https://github.com/thirawat27/wut/issues)
- 💡 [Request Feature](https://github.com/thirawat27/wut/issues)
- 💬 [Discussions](https://github.com/thirawat27/wut/discussions)

---

<p align="center">
  <strong>Cross-Platform • Universal • Intelligent</strong><br>
  Made with ❤️ by <a href="https://github.com/thirawat27">@thirawat27</a>
</p>
