#!/usr/bin/env bash
#
# WUT Installer Script
# One-line install: curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
#

set -euo pipefail

# Colors
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m' # No Color
readonly BOLD='\033[1m'

# Configuration
REPO="thirawat27/wut"
BINARY="wut"
VERSION="latest"
INSTALL_DIR=""
NO_INIT=false
NO_SHELL=false
FORCE=false
UNINSTALL=false

# Print helpers
print_header() {
    echo -e "${CYAN}${BOLD}"
    echo ' _    _ _____ _____'
    echo '| |  | |_   _|  __ \'
    echo '| |  | | | | | |  | |'
    echo '| |  | | | | | |  | |'
    echo '| |__| |_| |_| |__| |'
    echo ' \____/|_____|_____/'
    echo -e "${NC}"
    echo -e "${BLUE}AI-Powered Command Helper${NC}"
    echo ""
}

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
die() { error "$1"; exit 1; }

# Detect OS
detect_os() {
    local os
    case "$(uname -s)" in
        Linux*)     os="Linux" ;;
        Darwin*)    os="Darwin" ;;
        FreeBSD*)   os="FreeBSD" ;;
        OpenBSD*)   os="OpenBSD" ;;
        NetBSD*)    os="NetBSD" ;;
        *)          die "Unsupported operating system: $(uname -s)" ;;
    esac
    echo "$os"
}

# Detect architecture
detect_arch() {
    local arch
    case "$(uname -m)" in
        x86_64|amd64)   arch="x86_64" ;;
        arm64|aarch64)  arch="arm64" ;;
        armv7l|armv7)   arch="armv7" ;;
        i386|i686)      arch="i386" ;;
        riscv64)        arch="riscv64" ;;
        *)              die "Unsupported architecture: $(uname -m)" ;;
    esac
    echo "$arch"
}

# Get latest version from GitHub
get_latest_version() {
    local api_url="https://api.github.com/repos/${REPO}/releases/latest"
    local version
    
    if command -v curl >/dev/null 2>&1; then
        version=$(curl -sL "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        version=$(wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        die "Neither curl nor wget found. Please install one of them."
    fi
    
    echo "${version:-latest}"
}

# Determine install directory
get_install_dir() {
    local dir
    
    # Priority: /usr/local/bin > ~/.local/bin > ~/bin
    if [ -w "/usr/local/bin" ] || [ "$EUID" -eq 0 ]; then
        dir="/usr/local/bin"
    elif [ -d "$HOME/.local/bin" ] && [ -w "$HOME/.local/bin" ]; then
        dir="$HOME/.local/bin"
    elif [ -d "$HOME/bin" ] && [ -w "$HOME/bin" ]; then
        dir="$HOME/bin"
    else
        dir="$HOME/.local/bin"
        mkdir -p "$dir"
    fi
    
    echo "$dir"
}

# Check if binary exists
check_existing() {
    local existing
    existing=$(command -v "$BINARY" 2>/dev/null || true)
    
    if [ -n "$existing" ] && [ "$FORCE" != true ]; then
        warn "$BINARY is already installed at: $existing"
        read -p "Overwrite? [y/N] " -n 1 -r
        echo
        [[ $REPLY =~ ^[Yy]$ ]] || die "Installation cancelled"
    fi
}

# Download and extract binary
download_binary() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local install_dir="$4"
    
    # Normalize version (remove 'v' prefix if present for URL construction)
    local version_tag="$version"
    if [[ "$version" == v* ]]; then
        version_tag="${version#v}"
    fi
    
    # Archive name format: wut_<VERSION>_<OS>_<ARCH>.tar.gz
    local archive_name="${BINARY}_${version_tag}_${os}_${arch}.tar.gz"
    local download_url
    
    if [ "$version" = "latest" ]; then
        download_url="https://github.com/${REPO}/releases/latest/download/${archive_name}"
    else
        download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
    fi
    
    # Create temp directory
    local temp_dir
    temp_dir=$(mktemp -d)
    
    info "Downloading from: $download_url"
    
    local archive_file="${temp_dir}/${archive_name}"
    
    # Download
    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL --progress-bar "$download_url" -o "$archive_file"; then
            rm -rf "$temp_dir"
            die "Failed to download archive. Please check the version and try again."
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -q --show-progress "$download_url" -O "$archive_file"; then
            rm -rf "$temp_dir"
            die "Failed to download archive. Please check the version and try again."
        fi
    else
        rm -rf "$temp_dir"
        die "Neither curl nor wget found. Please install one of them."
    fi
    
    success "Download complete"
    
    # Extract archive
    info "Extracting archive..."
    if ! tar -xzf "$archive_file" -C "$temp_dir"; then
        rm -rf "$temp_dir"
        die "Failed to extract archive"
    fi
    
    # Find the binary in extracted files
    local extracted_binary
    extracted_binary=$(find "$temp_dir" -type f -name "$BINARY" | head -n 1)
    
    if [ -z "$extracted_binary" ]; then
        rm -rf "$temp_dir"
        die "Binary not found in archive"
    fi
    
    # Install
    local target="${install_dir}/${BINARY}"
    mv "$extracted_binary" "$target"
    chmod +x "$target"
    
    # Cleanup
    rm -rf "$temp_dir"
    
    echo "$target"
}

# Setup shell integration
setup_shell() {
    if [ "$NO_SHELL" = true ]; then
        info "Skipping shell integration"
        return
    fi
    
    local shell_name="${SHELL##*/}"
    info "Detected shell: $shell_name"
    
    case "$shell_name" in
        bash)
            setup_bash ;;
        zsh)
            setup_zsh ;;
        fish)
            setup_fish ;;
        *)
            warn "Unknown shell: $shell_name. Skipping shell integration."
            ;;
    esac
}

setup_bash() {
    local rc_file
    if [ -f "$HOME/.bashrc" ]; then
        rc_file="$HOME/.bashrc"
    elif [ -f "$HOME/.bash_profile" ]; then
        rc_file="$HOME/.bash_profile"
    else
        rc_file="$HOME/.bashrc"
        touch "$rc_file"
    fi
    
    # Add key bindings if not present
    if ! grep -q "wut key-binding" "$rc_file" 2>/dev/null; then
        cat >> "$rc_file" << 'EOF'

# WUT key bindings
if command -v wut >/dev/null 2>&1; then
    # Ctrl+Space to open WUT
    bind '"\C-@":"\C-uwut\C-m"' 2>/dev/null || true
fi
EOF
        success "Added Bash integration to $rc_file"
    fi
}

setup_zsh() {
    local rc_file="$HOME/.zshrc"
    [ -f "$rc_file" ] || touch "$rc_file"
    
    if ! grep -q "wut key-binding" "$rc_file" 2>/dev/null; then
        cat >> "$rc_file" << 'EOF'

# WUT key bindings
if command -v wut >/dev/null 2>&1; then
    # Ctrl+Space to open WUT
    bindkey '^@' wut-widget 2>/dev/null || true
fi
EOF
        success "Added Zsh integration to $rc_file"
    fi
}

setup_fish() {
    local config_dir="$HOME/.config/fish"
    mkdir -p "$config_dir/functions"
    
    # Create fish function for wut
    cat > "$config_dir/functions/wut-widget.fish" << 'EOF'
function wut-widget
    commandline -r "wut "
    commandline -f execute
end
EOF
    
    # Add key binding
    local fish_config="$config_dir/config.fish"
    if ! grep -q "wut key-binding" "$fish_config" 2>/dev/null; then
        cat >> "$fish_config" << 'EOF'

# WUT key bindings
if command -v wut >/dev/null
    bind \c@ wut-widget
end
EOF
        success "Added Fish integration"
    fi
}

# Run initialization
run_init() {
    if [ "$NO_INIT" = true ]; then
        info "Skipping initialization (--no-init)"
        return
    fi
    
    if ! command -v "$BINARY" >/dev/null 2>&1; then
        warn "$BINARY not found in PATH after installation"
        return
    fi
    
    info "Running quick initialization..."
    "$BINARY" init --quick 2>/dev/null || warn "Initialization failed, you can run 'wut init' later"
}

# Uninstall
uninstall() {
    info "Uninstalling $BINARY..."
    
    local found=false
    
    # Find and remove binary
    while IFS= read -r path; do
        if [ -f "$path/$BINARY" ]; then
            rm -f "$path/$BINARY"
            success "Removed: $path/$BINARY"
            found=true
        fi
    done < <(echo "$PATH" | tr ':' '\n')
    
    # Remove config
    local config_dir
    config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/wut"
    if [ -d "$config_dir" ]; then
        read -p "Remove configuration directory? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$config_dir"
            success "Removed config: $config_dir"
        fi
    fi
    
    if [ "$found" = true ]; then
        success "$BINARY has been uninstalled"
    else
        warn "$BINARY not found in PATH"
    fi
    
    exit 0
}

# Print usage
usage() {
    cat << EOF
Usage: install.sh [OPTIONS]

Options:
    -v, --version VERSION   Install specific version (default: latest)
    -d, --dir DIRECTORY     Install to specific directory
    --no-init              Skip running 'wut init --quick'
    --no-shell             Skip shell integration setup
    -f, --force            Force overwrite existing installation
    --uninstall            Uninstall wut
    -h, --help             Show this help message

Examples:
    # Default install (latest version)
    curl -fsSL .../install.sh | bash

    # Install specific version
    curl -fsSL .../install.sh | bash -s -- --version v1.0.0

    # Install without initialization
    curl -fsSL .../install.sh | bash -s -- --no-init
EOF
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--version)
                VERSION="$2"
                shift 2
                ;;
            -d|--dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --no-init)
                NO_INIT=true
                shift
                ;;
            --no-shell)
                NO_SHELL=true
                shift
                ;;
            -f|--force)
                FORCE=true
                shift
                ;;
            --uninstall)
                UNINSTALL=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Main installation
main() {
    parse_args "$@"
    
    print_header
    
    # Handle uninstall
    if [ "$UNINSTALL" = true ]; then
        uninstall
    fi
    
    # Detect system
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)
    info "Detected: $os/$arch"
    
    # Get version
    if [ "$VERSION" = "latest" ]; then
        info "Fetching latest version..."
        VERSION=$(get_latest_version)
    fi
    info "Version: $VERSION"
    
    # Determine install directory
    if [ -z "$INSTALL_DIR" ]; then
        INSTALL_DIR=$(get_install_dir)
    fi
    info "Install directory: $INSTALL_DIR"
    
    # Check existing
    check_existing
    
    # Download and install
    info "Downloading $BINARY..."
    local installed_path
    installed_path=$(download_binary "$VERSION" "$os" "$arch" "$INSTALL_DIR")
    success "Installed to: $installed_path"
    
    # Verify installation
    local installed_version
    installed_version=$("$installed_path" --version 2>/dev/null | head -1 || echo "unknown")
    success "Version: $installed_version"
    
    # Setup shell integration
    setup_shell
    
    # Run initialization
    run_init
    
    # Final message
    echo
    echo -e "${GREEN}${BOLD}✓ Installation complete!${NC}"
    echo
    echo "Quick start:"
    echo "  wut --help       Show help"
    echo "  wut suggest      Get command suggestions"
    echo "  wut fix 'gti'    Fix typos"
    echo
    
    # PATH warning if needed
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "$INSTALL_DIR is not in your PATH"
        echo "Add this to your shell profile:"
        echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
    
    # Reload reminder
    if [ "$NO_SHELL" != true ]; then
        echo
        echo "Run 'source ~/.$(basename "$SHELL")rc' or restart your terminal to apply changes."
    fi
}

# Run main
main "$@"
