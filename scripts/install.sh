#!/usr/bin/env bash
# WUT - Unix Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash

set -euo pipefail

# ── Config ─────────────────────────────────────────────────────────────────────
REPO="https://github.com/thirawat27/wut"
API="https://api.github.com/repos/thirawat27/wut"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"
TMP="$(mktemp -d)"

# Flags (set via args or env)
NO_INIT=0
NO_SHELL=0
FORCE=0
VERBOSE=0

# ── Cleanup ────────────────────────────────────────────────────────────────────
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

# ── Colors ─────────────────────────────────────────────────────────────────────
if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
    C_CYAN="\033[0;36m" C_GREEN="\033[0;32m" C_YELLOW="\033[1;33m"
    C_RED="\033[0;31m"  C_BLUE="\033[0;34m"  C_BOLD="\033[1m" C_RESET="\033[0m"
else
    C_CYAN="" C_GREEN="" C_YELLOW="" C_RED="" C_BLUE="" C_BOLD="" C_RESET=""
fi

# ── Print helpers ───────────────────────────────────────────────────────────────
banner() {
    printf "\n${C_CYAN}${C_BOLD}"
    printf "  ╔══════════════════════════════════════════╗\n"
    printf "  ║   WUT - Command Helper                   ║\n"
    printf "  ║   Unix Installer                         ║\n"
    printf "  ╚══════════════════════════════════════════╝\n"
    printf "${C_RESET}\n"
}
ok()   { printf "  ${C_GREEN}[+]${C_RESET} %s\n" "$1"; }
warn() { printf "  ${C_YELLOW}[!]${C_RESET} %s\n" "$1"; }
err()  { printf "  ${C_RED}[x]${C_RESET} %s\n" "$1" >&2; }
step() { printf "\n  ${C_BLUE}-->  ${C_RESET}%s\n" "$1"; }
info() { printf "      %s\n" "$1"; }
verbose() { [ "$VERBOSE" -eq 1 ] && printf "  [v] %s\n" "$1" || true; }

show_help() {
    cat <<EOF
WUT Installer

USAGE
  curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash

  With options:
  curl -fsSL ... | bash -s -- [OPTIONS]

OPTIONS
  --version <tag>       Specific version to install  (default: latest)
  --install-dir <path>  Where to install wut         (default: ~/.local/bin)
  --no-init             Skip automatic 'wut init'
  --no-shell            Skip shell integration setup
  --force               Overwrite existing install
  --verbose             Show extra output
  --help                Show this help

ENVIRONMENT
  VERSION       Version override
  INSTALL_DIR   Install path override
EOF
}

# ── Platform detection ─────────────────────────────────────────────────────────
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Linux)   os="Linux"   ;;
        Darwin)  os="Darwin"  ;;
        FreeBSD) os="FreeBSD" ;;
        *)       err "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  arch="x86_64" ;;
        aarch64|arm64) arch="arm64"  ;;
        armv7l)        arch="arm"    ;;
        i386|i686)     arch="i386"   ;;
        riscv64)       arch="riscv64";;
        *)             err "Unsupported arch: $(uname -m)"; exit 1 ;;
    esac

    PLATFORM="${os}_${arch}"
    OS="$os"
}

# ── Downloader ──────────────────────────────────────────────────────────────────
fetch() {
    local url="$1" out="$2"
    verbose "GET $url"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --retry 3 --connect-timeout 15 "$url" -o "$out"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --tries=3 --timeout=15 "$url" -O "$out"
    else
        err "Neither curl nor wget found"
        exit 1
    fi
}

# ── Get latest GitHub tag ───────────────────────────────────────────────────────
get_latest_version() {
    local tag
    tag=$(fetch "$API/releases/latest" - 2>/dev/null \
        | grep -o '"tag_name":"[^"]*"' \
        | head -1 \
        | cut -d'"' -f4)
    [ -n "$tag" ] || { err "Cannot fetch latest version from GitHub"; exit 1; }
    echo "$tag"
}

# ── Download & extract binary ───────────────────────────────────────────────────
download_binary() {
    local tag="$1"
    local ver="${tag#v}"
    local archive="wut_${ver}_${PLATFORM}.tar.gz"
    local url="$REPO/releases/download/$tag/$archive"
    local dest="$TMP/$archive"

    step "Downloading wut $tag ($PLATFORM)..."
    info "$url"

    fetch "$url" "$dest"

    step "Extracting..."
    tar -xzf "$dest" -C "$TMP"

    local bin
    bin="$(find "$TMP" -type f -name "wut" ! -name "*.gz" ! -name "*.tar" | head -1)"
    [ -n "$bin" ] || { err "Binary not found in archive"; exit 1; }
    echo "$bin"
}

# ── Build from source ───────────────────────────────────────────────────────────
build_from_source() {
    step "Building from source..."
    command -v go >/dev/null 2>&1 || { err "Go not found. Install from https://golang.org/dl/"; exit 1; }
    info "Go: $(go version)"

    local src="$TMP/src"
    if command -v git >/dev/null 2>&1; then
        git clone --depth 1 "$REPO" "$src" 2>/dev/null
    else
        fetch "$REPO/archive/refs/heads/main.tar.gz" "$TMP/main.tar.gz"
        tar -xzf "$TMP/main.tar.gz" -C "$TMP"
        mv "$TMP/wut-main" "$src"
    fi

    local out="$TMP/wut"
    (cd "$src" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$out" .)
    echo "$out"
}

# ── Detect shell config file ────────────────────────────────────────────────────
detect_shell_config() {
    local sh="${SHELL##*/}"
    case "$sh" in
        bash) echo "${BASH_ENV:-$HOME/.bashrc}" ;;
        zsh)  echo "$HOME/.zshrc" ;;
        fish) echo "$HOME/.config/fish/config.fish" ;;
        nu|nushell) echo "$HOME/.config/nushell/config.nu" ;;
        *)    echo "$HOME/.profile" ;;
    esac
}

# ── Add install dir to PATH ─────────────────────────────────────────────────────
add_to_path() {
    # Current session
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *) export PATH="$PATH:$INSTALL_DIR" ;;
    esac

    # Persistent
    local cfg
    cfg="$(detect_shell_config)"
    mkdir -p "$(dirname "$cfg")"

    if [ -f "$cfg" ] && grep -q "# wut" "$cfg" 2>/dev/null; then
        ok "PATH entry already in $cfg"
        return
    fi

    {
        printf '\n# wut\n'
        printf 'export PATH="$PATH:%s"\n' "$INSTALL_DIR"
    } >> "$cfg"
    ok "Added $INSTALL_DIR to PATH in $cfg"
}

# ── Shell integration ───────────────────────────────────────────────────────────
setup_shell() {
    local wut="$1"
    [ "$NO_SHELL" -eq 1 ] && return

    step "Setting up shell integration..."
    local sh="${SHELL##*/}"

    if "$wut" install --shell "$sh" 2>/dev/null; then
        ok "$sh integration installed"
    else
        warn "Shell integration failed — run 'wut install' later"
    fi
}

# ── wut init ───────────────────────────────────────────────────────────────────
run_init() {
    local wut="$1"
    [ "$NO_INIT" -eq 1 ] && return

    step "Running 'wut init --quick'..."
    if "$wut" init --quick; then
        ok "Initialization complete"
    else
        warn "Init failed — run 'wut init' manually"
    fi
}

# ── Main ───────────────────────────────────────────────────────────────────────
main() {
    banner
    detect_platform
    info "Platform : $PLATFORM"
    info "Shell    : ${SHELL##*/}"
    info "Install  : $INSTALL_DIR"

    # Resolve version
    if [ "$VERSION" = "latest" ]; then
        step "Fetching latest version..."
        VERSION="$(get_latest_version)"
        ok "Latest: $VERSION"
    fi

    # Create dirs
    mkdir -p "$INSTALL_DIR"

    # Check existing
    if [ -x "$INSTALL_DIR/wut" ] && [ "$FORCE" -eq 0 ]; then
        local ev
        ev="$("$INSTALL_DIR/wut" --version 2>/dev/null || echo unknown)"
        warn "WUT already installed: $ev"
        printf "      Reinstall? [y/N] "; read -r ans
        case "$ans" in
            [Yy]) ;;
            *) info "Cancelled."; return ;;
        esac
    fi

    # Get binary
    local bin
    if ! bin="$(download_binary "$VERSION")"; then
        warn "Binary download failed, trying to build from source..."
        bin="$(build_from_source)"
    fi

    # Install
    step "Installing to $INSTALL_DIR..."
    [ -f "$INSTALL_DIR/wut" ] && mv "$INSTALL_DIR/wut" "$INSTALL_DIR/wut.bak"
    cp "$bin" "$INSTALL_DIR/wut"
    chmod +x "$INSTALL_DIR/wut"
    ok "Installed: $INSTALL_DIR/wut"

    # PATH
    add_to_path

    # Verify
    step "Verifying..."
    if ! "$INSTALL_DIR/wut" --version >/dev/null 2>&1; then
        err "Binary does not run — check for libc issues or try building from source"
        exit 1
    fi
    ok "Verified: $("$INSTALL_DIR/wut" --version 2>&1 | head -1)"

    # Shell integration + init (automatic by default)
    setup_shell "$INSTALL_DIR/wut"
    run_init    "$INSTALL_DIR/wut"

    # Done
    printf "\n${C_CYAN}${C_BOLD}"
    printf "  ╔══════════════════════════════════════════╗\n"
    printf "  ║   Installation complete!                 ║\n"
    printf "  ╚══════════════════════════════════════════╝\n"
    printf "${C_RESET}\n"

    if command -v wut >/dev/null 2>&1; then
        ok "WUT is ready. Try:"
        info "  wut suggest git"
        info "  wut explain 'git rebase'"
        info "  wut --help"
    else
        warn "Open a new terminal or run: source $(detect_shell_config)"
        info "Then use: wut --help"
    fi
    printf "\n"
}

# ── Argument parsing ───────────────────────────────────────────────────────────
while [ $# -gt 0 ]; do
    case "$1" in
        --version)     VERSION="$2";      shift 2 ;;
        --install-dir) INSTALL_DIR="$2";  shift 2 ;;
        --no-init)     NO_INIT=1;         shift   ;;
        --no-shell)    NO_SHELL=1;        shift   ;;
        --force)       FORCE=1;           shift   ;;
        --verbose)     VERBOSE=1;         shift   ;;
        --help|-h)     show_help;         exit 0  ;;
        *) err "Unknown option: $1"; show_help; exit 1 ;;
    esac
done

main
