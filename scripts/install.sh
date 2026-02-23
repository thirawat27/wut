#!/usr/bin/env bash
#
# WUT Installer Script
# Downloads wut-setup.exe from GitHub Releases and launches the installer.
#
# One-line install:
#   curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
#
# NOTE: wut-setup.exe is a Windows installer. This script is provided for
#       environments that can run Windows EXE files (e.g. WSL2 with interop,
#       Wine on Linux/macOS). On a native Unix system you may want to invoke
#       it via Wine:  WINE_RUNNER=wine  bash install.sh
#

set -euo pipefail

# ── Colors ───────────────────────────────────────────────────────────────────
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly BOLD='\033[1m'
readonly NC='\033[0m'

# ── Configuration ─────────────────────────────────────────────────────────────
REPO="thirawat27/wut"
INSTALLER_NAME="wut-setup.exe"
VERSION="latest"
FORCE=false
UNINSTALL=false

# ── Helpers ───────────────────────────────────────────────────────────────────
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

info()    { echo -e "${BLUE}[INFO]${NC}  $1"; }
success() { echo -e "${GREEN}[OK]${NC}    $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1" >&2; }
die()     { error "$1"; exit 1; }

usage() {
    cat <<EOF
Usage: install.sh [OPTIONS]

Options:
    -v, --version TAG   Install a specific release tag (default: latest)
    -f, --force         Skip confirmation if WUT is already installed
    --uninstall         Run the installer in uninstall/remove mode
    -h, --help          Show this message

Examples:
    # Latest version
    curl -fsSL .../install.sh | bash

    # Specific version
    curl -fsSL .../install.sh | bash -s -- --version v0.1.0

    # Uninstall
    curl -fsSL .../install.sh | bash -s -- --uninstall
EOF
}

# ── Detect download tool ───────────────────────────────────────────────────────
http_get() {
    local url="$1"
    local out="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --progress-bar "$url" -o "$out"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --show-progress "$url" -O "$out"
    else
        die "Neither curl nor wget found. Please install one of them."
    fi
}

http_get_json() {
    local url="$1"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -H "User-Agent: WUT-Installer" -H "Accept: application/vnd.github+json" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- --header="User-Agent: WUT-Installer" --header="Accept: application/vnd.github+json" "$url"
    else
        die "Neither curl nor wget found. Please install one of them."
    fi
}

# ── Detect runner for .exe ─────────────────────────────────────────────────────
detect_exe_runner() {
    # 1. WSL2 interop – run Windows EXE directly
    if grep -qi microsoft /proc/version 2>/dev/null; then
        echo "native"   # WSL – just exec the .exe, Windows handles it
        return
    fi

    # 2. Wine
    if command -v "${WINE_RUNNER:-wine}" >/dev/null 2>&1; then
        echo "${WINE_RUNNER:-wine}"
        return
    fi

    # 3. Give up
    echo ""
}

# ── Resolve asset URL from GitHub API ─────────────────────────────────────────
get_setup_url() {
    local version="$1"
    local api_url

    if [ "$version" = "latest" ]; then
        api_url="https://api.github.com/repos/${REPO}/releases/latest"
    else
        # Normalise tag – ensure it starts with 'v'
        local tag="${version}"
        [[ "$tag" == v* ]] || tag="v${tag}"
        api_url="https://api.github.com/repos/${REPO}/releases/tags/${tag}"
    fi

    info "Querying GitHub API: $api_url"

    local json
    json=$(http_get_json "$api_url") || die "Failed to reach GitHub API"

    local tag_name
    tag_name=$(echo "$json" | grep '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
    info "Found release: $tag_name"

    # Extract browser_download_url for the installer
    local download_url
    download_url=$(echo "$json" \
        | grep -A2 "\"name\": \"${INSTALLER_NAME}\"" \
        | grep '"browser_download_url"' \
        | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' \
        | head -n1)

    if [ -z "$download_url" ]; then
        # Fallback: list all asset names for debugging
        local asset_names
        asset_names=$(echo "$json" | grep '"name"' | sed -E 's/.*"name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | tr '\n' ', ')
        die "Asset '${INSTALLER_NAME}' not found in release ${tag_name}. Available assets: ${asset_names}"
    fi

    echo "$download_url"
}

# ── Download ──────────────────────────────────────────────────────────────────
download_installer() {
    local url="$1"
    local dest="$2"

    info "Downloading from: $url"
    http_get "$url" "$dest" || die "Download failed. Check network and try again."
    success "Downloaded: $(basename "$dest")"
}

# ── Run installer ─────────────────────────────────────────────────────────────
run_installer() {
    local installer_path="$1"
    local runner="$2"
    local uninstall="$3"

    local extra_args=()
    if [ "$uninstall" = true ]; then
        extra_args=("/uninstall" "/S")
        info "Running uninstaller silently..."
    else
        extra_args=("/S")   # Inno Setup / NSIS silent flag
        info "Launching installer silently..."
    fi

    local exit_code=0

    if [ "$runner" = "native" ]; then
        # WSL interop: convert path to Windows path and exec directly
        local win_path
        win_path=$(wslpath -w "$installer_path" 2>/dev/null || echo "$installer_path")
        "$win_path" "${extra_args[@]}" || exit_code=$?
    else
        "$runner" "$installer_path" "${extra_args[@]}" || exit_code=$?
    fi

    if [ "$exit_code" -ne 0 ]; then
        die "Installer exited with code ${exit_code}"
    fi

    success "Installer finished (exit code 0)"
}

# ── Parse CLI args ─────────────────────────────────────────────────────────────
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -v|--version)
                VERSION="$2"; shift 2 ;;
            -f|--force)
                FORCE=true; shift ;;
            --uninstall)
                UNINSTALL=true; shift ;;
            -h|--help)
                usage; exit 0 ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
    parse_args "$@"

    print_header

    # Check for existing install
    if command -v wut >/dev/null 2>&1 && [ "$FORCE" != true ] && [ "$UNINSTALL" != true ]; then
        local current_ver
        current_ver=$(wut --version 2>/dev/null | head -1 || echo "unknown")
        warn "WUT is already installed (version: ${current_ver})"
        read -rp "Reinstall / upgrade? [y/N] " answer
        [[ "$answer" =~ ^[Yy]$ ]] || { info "Cancelled."; exit 0; }
    fi

    # Detect how to execute the .exe
    local runner
    runner=$(detect_exe_runner)
    if [ -z "$runner" ]; then
        warn "No Windows EXE runner found (WSL interop or Wine required)."
        warn "Downloading the installer for manual execution..."
    fi

    # Resolve URL via GitHub API
    local download_url
    download_url=$(get_setup_url "$VERSION")

    # Temp directory
    local temp_dir
    temp_dir=$(mktemp -d)
    trap 'rm -rf "$temp_dir"' EXIT

    local installer_path="${temp_dir}/${INSTALLER_NAME}"

    # Download
    download_installer "$download_url" "$installer_path"
    chmod +x "$installer_path"

    if [ -z "$runner" ]; then
        # No runner – just tell the user where the file is
        # Copy to current directory so it survives the trap cleanup
        cp "$installer_path" "./${INSTALLER_NAME}"
        success "Installer saved to: ./${INSTALLER_NAME}"
        info "Run it manually: ./${INSTALLER_NAME}"
        exit 0
    fi

    # Run installer
    run_installer "$installer_path" "$runner" "$UNINSTALL"

    echo ""
    if [ "$UNINSTALL" = true ]; then
        echo -e "${GREEN}${BOLD}Uninstall complete!${NC}"
    else
        echo -e "${GREEN}${BOLD}Installation complete!${NC}"
        echo ""
        echo "Quick start:"
        echo "  wut --help       Show help"
        echo "  wut suggest      Get command suggestions"
        echo "  wut fix 'gti'    Fix typos"
    fi
    echo ""
}

main "$@"
