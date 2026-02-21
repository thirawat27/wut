#!/bin/bash
# WUT Installation Script for Unix-like systems
# Supports: Linux, macOS, FreeBSD, OpenBSD, NetBSD
# Shells: bash, zsh, fish, nushell, elvish, xonsh, tcsh, ksh
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
#   
#   # With options:
#   curl -fsSL ... | bash -s -- --version v1.0.0 --init

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
API_URL="https://api.github.com/repos/thirawat27/wut"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/wut}"
DATA_DIR="${DATA_DIR:-$HOME/.wut}"
TEMP_DIR="${TEMP_DIR:-$(mktemp -d)}"

# Flags
NO_SHELL_INTEGRATION=0
RUN_INIT=0
FORCE=0
VERBOSE=0

# Cleanup on exit
cleanup() {
    if [ -d "$TEMP_DIR" ] && [[ "$TEMP_DIR" == /tmp/* ]]; then
        rm -rf "$TEMP_DIR"
    fi
}
trap cleanup EXIT

# Detect OS and Architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$OS" in
        linux*)     OS="Linux" ;;
        darwin*)    OS="Darwin" ;;
        freebsd*)   OS="FreeBSD" ;;
        openbsd*)   OS="OpenBSD" ;;
        netbsd*)    OS="NetBSD" ;;
        msys*|cygwin*|mingw*)
                    OS="Windows" ;;
        *)          OS="unknown" ;;
    esac
    
    case "$ARCH" in
        x86_64|amd64)   ARCH="x86_64" ;;
        i386|i686)      ARCH="i386" ;;
        arm64|aarch64)  ARCH="arm64" ;;
        armv7l|armv7)   ARCH="arm" ;;
        riscv64)        ARCH="riscv64" ;;
        *)              ARCH="unknown" ;;
    esac
    
    PLATFORM="${OS}_${ARCH}"
}

# Print functions
print_header() {
    echo -e "${BLUE}${BOLD}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                                                            ║"
    echo "║   WUT - Command Helper                                     ║"
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
print_verbose() {
    if [ $VERBOSE -eq 1 ]; then
        echo -e "${CYAN}[verbose]${NC} $1"
    fi
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check dependencies
check_dependencies() {
    print_step "Checking dependencies..."
    
    local missing=()
    
    if ! command_exists curl && ! command_exists wget; then
        missing+=("curl or wget")
    fi
    
    if ! command_exists uname; then
        missing+=("uname")
    fi
    
    # Check for tar (needed for extracting archives)
    if ! command_exists tar; then
        missing+=("tar")
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

# Get latest version from GitHub API
get_latest_version() {
    local version
    
    if command_exists curl; then
        version=$(curl -fsSL "${API_URL}/releases/latest" 2>/dev/null | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)
    else
        version=$(wget -qO- "${API_URL}/releases/latest" 2>/dev/null | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)
    fi
    
    echo "${version:-latest}"
}

# Download file with progress
download_file() {
    local url="$1"
    local output="$2"
    
    print_verbose "Downloading: $url"
    
    if command_exists curl; then
        if [ $VERBOSE -eq 1 ]; then
            curl -fsSL --progress-bar "$url" -o "$output"
        else
            curl -fsSL "$url" -o "$output" 2>/dev/null
        fi
    else
        if [ $VERBOSE -eq 1 ]; then
            wget --show-progress -q "$url" -O "$output"
        else
            wget -q "$url" -O "$output" 2>/dev/null
        fi
    fi
}

# Verify checksum
verify_checksum() {
    local file="$1"
    local checksum_file="$2"
    
    if [ ! -f "$checksum_file" ]; then
        print_warning "Checksum file not found, skipping verification"
        return 0
    fi
    
    print_step "Verifying checksum..."
    
    local expected_checksum
    expected_checksum=$(grep "$(basename "$file")" "$checksum_file" 2>/dev/null | awk '{print $1}')
    
    if [ -z "$expected_checksum" ]; then
        print_warning "Checksum for $(basename "$file") not found in checksums file"
        return 0
    fi
    
    local actual_checksum
    if command_exists sha256sum; then
        actual_checksum=$(sha256sum "$file" | awk '{print $1}')
    elif command_exists shasum; then
        actual_checksum=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        print_warning "Neither sha256sum nor shasum found, skipping verification"
        return 0
    fi
    
    if [ "$expected_checksum" != "$actual_checksum" ]; then
        print_error "Checksum verification failed!"
        print_error "Expected: $expected_checksum"
        print_error "Actual:   $actual_checksum"
        return 1
    fi
    
    print_success "Checksum verified"
    return 0
}

# Download and install binary
download_and_install() {
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(get_latest_version)
        print_info "Latest version: $VERSION"
    fi
    
    # Remove 'v' prefix if present for URL construction
    local version_no_v="${VERSION#v}"
    
    local archive_name="wut_${version_no_v}_${PLATFORM}.tar.gz"
    local checksum_name="checksums.txt"
    
    local download_url="${REPO_URL}/releases/download/${VERSION}/${archive_name}"
    local checksum_url="${REPO_URL}/releases/download/${VERSION}/${checksum_name}"
    
    print_step "Downloading WUT ${VERSION} for ${PLATFORM}..."
    print_verbose "Archive: $archive_name"
    print_verbose "URL: $download_url"
    
    local archive_path="${TEMP_DIR}/${archive_name}"
    local checksum_path="${TEMP_DIR}/${checksum_name}"
    
    # Download archive
    if ! download_file "$download_url" "$archive_path"; then
        print_error "Failed to download archive"
        return 1
    fi
    
    print_verbose "Downloaded to: $archive_path"
    
    # Try to download checksums (optional)
    download_file "$checksum_url" "$checksum_path" 2>/dev/null || true
    
    # Verify checksum if available
    if [ -f "$checksum_path" ]; then
        verify_checksum "$archive_path" "$checksum_path" || return 1
    fi
    
    # Extract archive
    print_step "Extracting archive..."
    if ! tar -xzf "$archive_path" -C "$TEMP_DIR"; then
        print_error "Failed to extract archive"
        return 1
    fi
    
    # Find the binary
    local binary_name="wut"
    if [ "$OS" = "Windows" ]; then
        binary_name="wut.exe"
    fi
    
    local extracted_binary="${TEMP_DIR}/${binary_name}"
    if [ ! -f "$extracted_binary" ]; then
        # Try to find in subdirectory
        extracted_binary=$(find "$TEMP_DIR" -name "$binary_name" -type f 2>/dev/null | head -1)
        if [ -z "$extracted_binary" ]; then
            print_error "Binary not found in archive"
            return 1
        fi
    fi
    
    print_verbose "Found binary: $extracted_binary"
    
    # Install binary
    local output_path="${INSTALL_DIR}/wut"
    if [ "$OS" = "Windows" ]; then
        output_path="${INSTALL_DIR}/wut.exe"
    fi
    
    # Backup existing binary if exists
    if [ -f "$output_path" ] && [ $FORCE -eq 0 ]; then
        print_warning "WUT is already installed. Use --force to overwrite."
        return 0
    fi
    
    if [ -f "$output_path" ]; then
        mv "$output_path" "${output_path}.backup"
        print_verbose "Backed up existing binary"
    fi
    
    # Copy and make executable
    cp "$extracted_binary" "$output_path"
    chmod +x "$output_path"
    
    print_success "Binary installed: ${output_path}"
}

# Build from source
build_from_source() {
    print_warning "Binary download failed, attempting to build from source..."
    
    if ! command_exists go; then
        print_error "Go is required to build from source"
        print_info "Please install Go from https://golang.org/dl/"
        exit 1
    fi
    
    # Check Go version (requires 1.21+)
    local go_version
    go_version=$(go version | grep -o 'go[0-9.]*' | head -1)
    print_info "Found Go: $go_version"
    
    print_step "Building from source..."
    
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" EXIT
    
    cd "$temp_dir"
    
    if command_exists git; then
        git clone --depth 1 "$REPO_URL" wut 2>/dev/null
    else
        # Fallback: download tarball
        local tarball_url="${REPO_URL}/archive/refs/heads/main.tar.gz"
        if command_exists curl; then
            curl -fsSL "$tarball_url" | tar -xz
        else
            wget -qO- "$tarball_url" | tar -xz
        fi
        mv wut-main wut
    fi
    
    cd wut
    
    # Build with optimizations
    local output_path="${INSTALL_DIR}/wut"
    if [ "$OS" = "Windows" ]; then
        output_path="${output_path}.exe"
    fi
    
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "$output_path" .
    
    print_success "Built from source: $output_path"
    
    cd - >/dev/null
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
    
    # Check if already added
    if [ -f "$config_file" ] && grep -q "WUT PATH" "$config_file" 2>/dev/null; then
        print_success "PATH entry already exists in $config_file"
        return 0
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
    if [ $NO_SHELL_INTEGRATION -eq 1 ]; then
        print_verbose "Skipping shell integration (--no-shell-integration)"
        return 0
    fi
    
    print_step "Installing shell integration..."
    
    local wut_path="${INSTALL_DIR}/wut"
    if [ "$OS" = "Windows" ]; then
        wut_path="${INSTALL_DIR}/wut.exe"
    fi
    
    if [ -x "$wut_path" ]; then
        if "$wut_path" install --all 2>/dev/null; then
            print_success "Shell integration installed"
        else
            print_warning "Shell integration may need manual configuration"
            print_info "Run: wut install --all"
        fi
    else
        print_warning "WUT binary not found, skipping shell integration"
    fi
}

# Run wut init
run_init() {
    if [ $RUN_INIT -eq 0 ]; then
        return 0
    fi
    
    print_step "Running initialization..."
    
    local wut_path="${INSTALL_DIR}/wut"
    if [ "$OS" = "Windows" ]; then
        wut_path="${INSTALL_DIR}/wut.exe"
    fi
    
    if [ -x "$wut_path" ]; then
        if "$wut_path" init --quick 2>/dev/null; then
            print_success "Initialization complete"
        else
            print_warning "Initialization failed or not available"
            print_info "Run manually: wut init"
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
    
    local wut_path="${INSTALL_DIR}/wut"
    if [ "$OS" = "Windows" ]; then
        wut_path="${INSTALL_DIR}/wut.exe"
    fi
    
    if [ -x "$wut_path" ]; then
        local version
        version=$("$wut_path" --version 2>/dev/null || echo "unknown")
        print_success "Installation verified: $version"
    else
        print_error "Installation verification failed"
        return 1
    fi
}

# Print usage
print_usage() {
    cat << EOF
Usage: install.sh [OPTIONS]

Options:
  --version VERSION       Install specific version (default: latest)
  --install-dir DIR       Install to specific directory (default: ~/.local/bin)
  --no-shell-integration  Skip shell integration
  --init                  Run 'wut init' after installation
  --force                 Force overwrite existing installation
  --verbose               Enable verbose output
  --help                  Show this help message

Environment Variables:
  VERSION                 Version to install (overrides --version)
  INSTALL_DIR             Installation directory
  CONFIG_DIR              Configuration directory
  DATA_DIR                Data directory

Examples:
  # Install latest version
  ./install.sh

  # Install specific version
  ./install.sh --version v1.0.0

  # Install with initialization
  ./install.sh --init

  # Install to custom directory
  ./install.sh --install-dir /usr/local/bin
EOF
}

# Main installation
main() {
    print_header
    
    detect_platform
    print_system_info
    
    # Check for existing installation
    local existing_version=""
    local wut_path="${INSTALL_DIR}/wut"
    if [ "$OS" = "Windows" ]; then
        wut_path="${wut_path}.exe"
    fi
    
    if [ -f "$wut_path" ] && [ $FORCE -eq 0 ]; then
        existing_version=$("$wut_path" --version 2>/dev/null | head -1 || echo "unknown")
        print_info "Existing installation found: $existing_version"
        print_info "Use --force to overwrite or install anyway"
        
        read -p "Continue with installation? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Installation cancelled"
            exit 0
        fi
    fi
    
    check_dependencies
    create_directories
    
    # Try to download binary, fallback to building from source
    if ! download_and_install; then
        print_warning "Download failed, trying to build from source..."
        build_from_source
    fi
    
    add_to_path
    install_shell_integration
    run_init
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
        --init)
            RUN_INIT=1
            shift
            ;;
        --force)
            FORCE=1
            shift
            ;;
        --verbose)
            VERBOSE=1
            shift
            ;;
        --help|-h)
            print_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            print_usage
            exit 1
            ;;
    esac
done

main
