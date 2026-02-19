#!/bin/bash
# WUT Installation Script for Unix-like systems
# Supports: Linux, macOS, FreeBSD, OpenBSD, NetBSD
# Shells: bash, zsh, fish, nushell, elvish, xonsh, tcsh, ksh

set -e

# Colors (fallback for terminals without color support)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' CYAN='' BOLD='' NC=''
fi

# Configuration
REPO_URL="https://github.com/thirawat27/wut"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/wut}"
DATA_DIR="${DATA_DIR:-$HOME/.wut}"

# Detect OS and Architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$OS" in
        linux*)     OS="linux" ;;
        darwin*)    OS="darwin" ;;
        freebsd*)   OS="freebsd" ;;
        openbsd*)   OS="openbsd" ;;
        netbsd*)    OS="netbsd" ;;
        msys*|cygwin*|mingw*)
                    OS="windows" ;;
        *)          OS="unknown" ;;
    esac
    
    case "$ARCH" in
        x86_64|amd64)   ARCH="amd64" ;;
        i386|i686)      ARCH="386" ;;
        arm64|aarch64)  ARCH="arm64" ;;
        armv7l|armv7)   ARCH="arm" ;;
        riscv64)        ARCH="riscv64" ;;
        *)              ARCH="unknown" ;;
    esac
    
    PLATFORM="${OS}-${ARCH}"
}

# Print functions
print_header() {
    echo -e "${BLUE}${BOLD}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                                                            ║"
    echo "║   WUT - AI-Powered Command Helper                          ║"
    echo "║   Universal Installation Script                            ║"
    echo "║                                                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }
print_info() { echo -e "${BLUE}ℹ${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
print_step() { echo -e "${CYAN}→${NC} $1"; }

# Check dependencies
check_dependencies() {
    print_step "Checking dependencies..."
    
    local missing=()
    
    if ! command -v curl &> /dev/null && ! command -v wget &> /dev/null; then
        missing+=("curl or wget")
    fi
    
    if ! command -v uname &> /dev/null; then
        missing+=("uname")
    fi
    
    if [ ${#missing[@]} -ne 0 ]; then
        print_error "Missing required tools: ${missing[*]}"
        exit 1
    fi
    
    print_success "All dependencies satisfied"
}

# Create directories
create_directories() {
    print_step "Creating directories..."
    
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"/{data,models,logs}
    
    print_success "Directories created"
}

# Download binary
download_binary() {
    print_step "Downloading WUT binary for $PLATFORM..."
    
    local binary_name="wut-${PLATFORM}"
    local download_url
    
    if [ "$VERSION" = "latest" ]; then
        download_url="${REPO_URL}/releases/latest/download/${binary_name}"
    else
        download_url="${REPO_URL}/releases/download/${VERSION}/${binary_name}"
    fi
    
    local output_path="${INSTALL_DIR}/wut"
    
    # Download with progress
    if command -v curl &> /dev/null; then
        if ! curl -fsSL --progress-bar "$download_url" -o "$output_path"; then
            return 1
        fi
    else
        if ! wget -q --show-progress "$download_url" -O "$output_path"; then
            return 1
        fi
    fi
    
    chmod +x "$output_path"
    print_success "Binary downloaded: ${output_path}"
}

# Build from source
build_from_source() {
    print_warning "Binary download failed, attempting to build from source..."
    
    if ! command -v go &> /dev/null; then
        print_error "Go is required to build from source"
        print_info "Please install Go from https://golang.org/dl/"
        exit 1
    fi
    
    print_step "Building from source..."
    
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" EXIT
    
    cd "$temp_dir"
    
    if command -v git &> /dev/null; then
        git clone --depth 1 "$REPO_URL" wut 2>/dev/null
    else
        # Fallback: download tarball
        local tarball_url="${REPO_URL}/archive/refs/heads/main.tar.gz"
        if command -v curl &> /dev/null; then
            curl -fsSL "$tarball_url" | tar -xz
        else
            wget -qO- "$tarball_url" | tar -xz
        fi
        mv wut-main wut
    fi
    
    cd wut
    
    # Build with optimizations
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${INSTALL_DIR}/wut" .
    
    print_success "Built from source"
}

# Detect shell configuration file
detect_shell_config() {
    local shell_name="${SHELL##*/}"
    
    case "$shell_name" in
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                echo "$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                echo "$HOME/.bash_profile"
            else
                echo "$HOME/.bashrc"
            fi
            ;;
        zsh)
            if [ -f "$HOME/.zshrc" ]; then
                echo "$HOME/.zshrc"
            elif [ -f "$HOME/.zprofile" ]; then
                echo "$HOME/.zprofile"
            else
                echo "$HOME/.zshrc"
            fi
            ;;
        fish)
            echo "$HOME/.config/fish/config.fish"
            ;;
        nu|nush)
            echo "$HOME/.config/nushell/config.nu"
            ;;
        elvish)
            echo "$HOME/.elvish/rc.elv"
            ;;
        xonsh)
            echo "$HOME/.xonshrc"
            ;;
        tcsh|csh)
            if [ -f "$HOME/.tcshrc" ]; then
                echo "$HOME/.tcshrc"
            elif [ -f "$HOME/.cshrc" ]; then
                echo "$HOME/.cshrc"
            else
                echo "$HOME/.tcshrc"
            fi
            ;;
        ksh|mksh|oksh)
            echo "$HOME/.kshrc"
            ;;
        *)
            echo "$HOME/.profile"
            ;;
    esac
}

# Add to PATH
add_to_path() {
    print_step "Checking PATH..."
    
    if [[ ":$PATH:" == *":${INSTALL_DIR}:"* ]]; then
        print_success "Already in PATH"
        return 0
    fi
    
    local config_file
    config_file=$(detect_shell_config)
    
    # Create directory if it doesn't exist
    local config_dir
    config_dir=$(dirname "$config_file")
    if [ ! -d "$config_dir" ]; then
        mkdir -p "$config_dir"
    fi
    
    # Add to PATH
    echo "" >> "$config_file"
    echo "# WUT PATH" >> "$config_file"
    echo 'export PATH="$PATH:'"$INSTALL_DIR"'"' >> "$config_file"
    
    print_success "Added to PATH in $config_file"
    print_info "Please run: source $config_file"
}

# Install shell integration
install_shell_integration() {
    print_step "Installing shell integration..."
    
    if [ -x "${INSTALL_DIR}/wut" ]; then
        if "${INSTALL_DIR}/wut" install --all 2>/dev/null; then
            print_success "Shell integration installed"
        else
            print_warning "Shell integration may need manual configuration"
        fi
    fi
}

# Print system information
print_system_info() {
    print_info "Platform: $PLATFORM"
    print_info "Shell: ${SHELL##*/}"
    print_info "Install directory: $INSTALL_DIR"
    print_info "Config directory: $CONFIG_DIR"
}

# Verify installation
verify_installation() {
    print_step "Verifying installation..."
    
    if [ -x "${INSTALL_DIR}/wut" ]; then
        local version
        version=$("${INSTALL_DIR}/wut" --version 2>/dev/null || echo "unknown")
        print_success "Installation verified: $version"
    else
        print_error "Installation verification failed"
        return 1
    fi
}

# Main installation
main() {
    print_header
    
    detect_platform
    print_system_info
    
    check_dependencies
    create_directories
    
    # Try to download binary, fallback to building from source
    if ! download_binary; then
        build_from_source
    fi
    
    add_to_path
    install_shell_integration
    verify_installation
    
    echo
    print_header
    print_success "WUT installation complete!"
    echo
    print_info "Quick Start:"
    echo "  wut suggest 'git push'    # Get command suggestions"
    echo "  wut history               # View command history"
    echo "  wut explain 'git rebase'  # Explain a command"
    echo "  wut --help                # Show all commands"
    echo
    print_info "Please restart your terminal or run:"
    echo "  source $(detect_shell_config)"
    echo
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --no-shell-integration)
            NO_SHELL_INTEGRATION=1
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version VERSION       Install specific version (default: latest)"
            echo "  --install-dir DIR       Install to specific directory (default: ~/.local/bin)"
            echo "  --no-shell-integration  Skip shell integration"
            echo "  --help                  Show this help message"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

main
