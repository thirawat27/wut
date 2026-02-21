#!/bin/bash
# WUT - Command Helper | Unix Installer
# Usage:  curl -fsSL https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.sh | bash
set -e

# --- config ---
REPO="thirawat27/wut"
API="https://api.github.com/repos/$REPO"
BASE="https://github.com/$REPO"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
TMP="$(mktemp -d)"

NO_INIT=0
NO_SHELL=0
FORCE=0

trap 'rm -rf "$TMP"' EXIT

# --- colors ---
if [ -t 1 ]; then
    R='\033[0;31m' G='\033[0;32m' Y='\033[1;33m' C='\033[0;36m' B='\033[1m' N='\033[0m'
else
    R='' G='' Y='' C='' B='' N=''
fi

ok()   { echo -e "${G}[OK]${N} $1"; }
err()  { echo -e "${R}[ERR]${N} $1"; }
info() { echo -e "${C}[>]${N} $1"; }
warn() { echo -e "${Y}[!]${N} $1"; }

banner() {
    echo ""
    echo -e "${C}${B}  WUT - Command Helper  |  Unix Installer${N}"
    echo ""
}

has() { command -v "$1" >/dev/null 2>&1; }

# --- download helper ---
dl() {
    local url="$1" out="$2"
    if has curl; then
        curl -fsSL "$url" -o "$out" 2>/dev/null
    elif has wget; then
        wget -qO "$out" "$url" 2>/dev/null
    else
        err "Need curl or wget"; exit 1
    fi
}

# --- detect platform ---
detect_platform() {
    local os arch
    os="$(uname -s)"
    arch="$(uname -m)"

    case "$os" in
        Linux*)   os="Linux" ;;
        Darwin*)  os="Darwin" ;;
        FreeBSD*) os="FreeBSD" ;;
        OpenBSD*) os="OpenBSD" ;;
        NetBSD*)  os="NetBSD" ;;
        MSYS*|CYGWIN*|MINGW*) os="Windows" ;;
        *) err "Unsupported OS: $os"; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="x86_64" ;;
        i386|i686)     arch="i386" ;;
        arm64|aarch64) arch="arm64" ;;
        armv7l|armv7)  arch="arm" ;;
        riscv64)       arch="riscv64" ;;
        *) err "Unsupported arch: $arch"; exit 1 ;;
    esac

    OS="$os"
    ARCH="$arch"
    PLATFORM="${os}_${arch}"
}

# --- resolve version ---
resolve_version() {
    if [ "$VERSION" != "latest" ]; then return; fi
    local v=""
    if has curl; then
        v="$(curl -fsSL "$API/releases/latest" 2>/dev/null | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)"
    elif has wget; then
        v="$(wget -qO- "$API/releases/latest" 2>/dev/null | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)"
    fi
    if [ -z "$v" ]; then err "Cannot fetch latest version"; exit 1; fi
    VERSION="$v"
}

# --- add to PATH ---
add_to_path() {
    # current session
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *) export PATH="$PATH:$INSTALL_DIR" ;;
    esac

    # permanent
    local rc=""
    local shell_name="${SHELL##*/}"
    case "$shell_name" in
        bash) rc="$HOME/.bashrc"; [ ! -f "$rc" ] && rc="$HOME/.bash_profile" ;;
        zsh)  rc="$HOME/.zshrc" ;;
        fish) rc="$HOME/.config/fish/config.fish" ;;
        *)    rc="$HOME/.profile" ;;
    esac

    if [ -n "$rc" ]; then
        if [ -f "$rc" ] && grep -q "# WUT PATH" "$rc" 2>/dev/null; then
            return
        fi
        mkdir -p "$(dirname "$rc")"
        echo '' >> "$rc"
        echo '# WUT PATH' >> "$rc"
        if [ "$shell_name" = "fish" ]; then
            echo "fish_add_path $INSTALL_DIR" >> "$rc"
        else
            echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$rc"
        fi
        ok "Added to PATH in $rc"
    fi
}

# --- build from source ---
build_from_source() {
    if ! has go; then
        err "Go is required to build from source (https://golang.org/dl/)"
        return 1
    fi
    info "Building from source..."
    local srcdir="$TMP/src"
    if has git; then
        git clone --depth 1 "$BASE.git" "$srcdir" 2>/dev/null
    else
        dl "$BASE/archive/refs/heads/main.tar.gz" "$TMP/src.tar.gz"
        tar -xzf "$TMP/src.tar.gz" -C "$TMP"
        mv "$TMP/wut-main" "$srcdir"
    fi
    local out="$INSTALL_DIR/wut"
    (cd "$srcdir" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$out" .)
    chmod +x "$out"
    ok "Built from source"
}

# --- main ---
main() {
    banner
    detect_platform
    info "Platform: $PLATFORM"

    # deps check
    if ! has curl && ! has wget; then err "Need curl or wget"; exit 1; fi
    if ! has tar; then err "Need tar"; exit 1; fi

    resolve_version
    info "Version:  $VERSION"

    # create dirs
    mkdir -p "$INSTALL_DIR" "$HOME/.config/wut" "$HOME/.wut/data" "$HOME/.wut/logs"

    local dest="$INSTALL_DIR/wut"
    [ "$OS" = "Windows" ] && dest="$INSTALL_DIR/wut.exe"

    # check existing
    if [ -f "$dest" ] && [ $FORCE -eq 0 ]; then
        local ev
        ev="$("$dest" --version 2>/dev/null | head -1)" || ev="unknown"
        warn "Already installed: $ev  (use --force to overwrite)"
    fi

    # download
    local vn="${VERSION#v}"
    local ext="tar.gz"
    [ "$OS" = "Windows" ] && ext="zip"
    local archive="wut_${vn}_${PLATFORM}.${ext}"
    local url="$BASE/releases/download/$VERSION/$archive"
    local dl_path="$TMP/$archive"

    info "Downloading $archive ..."
    if dl "$url" "$dl_path"; then
        info "Extracting..."
        if [ "$ext" = "zip" ]; then
            unzip -qo "$dl_path" -d "$TMP"
        else
            tar -xzf "$dl_path" -C "$TMP"
        fi

        local bin="$TMP/wut"
        [ "$OS" = "Windows" ] && bin="$TMP/wut.exe"
        if [ ! -f "$bin" ]; then
            bin="$(find "$TMP" -name "wut" -o -name "wut.exe" 2>/dev/null | head -1)"
        fi

        if [ -z "$bin" ] || [ ! -f "$bin" ]; then
            err "Binary not found in archive"
            exit 1
        fi

        [ -f "$dest" ] && mv "$dest" "${dest}.bak" 2>/dev/null || true
        cp "$bin" "$dest"
        chmod +x "$dest"
        ok "Installed: $dest"
    else
        warn "Download failed, trying build from source..."
        build_from_source || { err "Installation failed"; exit 1; }
    fi

    # PATH
    add_to_path

    # verify
    local v
    v="$("$dest" --version 2>/dev/null | head -1)" || v="installed"
    ok "Verified: $v"

    # shell integration
    if [ $NO_SHELL -eq 0 ] && [ -x "$dest" ]; then
        info "Setting up shell integration..."
        "$dest" install --all 2>/dev/null && ok "Shell integration done" || warn "Shell integration skipped"
    fi

    # auto init
    if [ $NO_INIT -eq 0 ] && [ -x "$dest" ]; then
        info "Initializing..."
        "$dest" init --quick 2>/dev/null && ok "Initialization complete" || warn "Init skipped (run 'wut init' manually)"
    fi

    # done
    echo ""
    echo -e "${G}${B}  WUT is ready!${N}"
    echo ""
    echo "  Quick start:"
    echo "    wut suggest 'git push'     # Get suggestions"
    echo "    wut explain 'git rebase'   # Explain a command"
    echo "    wut fix 'gti status'       # Fix typos"
    echo "    wut --help                 # All commands"
    echo ""

    if ! has wut; then
        warn "Restart your terminal or run:  source $(detect_shell_config)"
    fi
}

detect_shell_config() {
    local s="${SHELL##*/}"
    case "$s" in
        bash) [ -f "$HOME/.bashrc" ] && echo "$HOME/.bashrc" || echo "$HOME/.bash_profile" ;;
        zsh)  echo "$HOME/.zshrc" ;;
        fish) echo "$HOME/.config/fish/config.fish" ;;
        *)    echo "$HOME/.profile" ;;
    esac
}

# --- args ---
while [ $# -gt 0 ]; do
    case "$1" in
        --version)   VERSION="$2"; shift 2 ;;
        --install-dir) INSTALL_DIR="$2"; shift 2 ;;
        --no-init)   NO_INIT=1; shift ;;
        --no-shell)  NO_SHELL=1; shift ;;
        --force)     FORCE=1; shift ;;
        --help|-h)
            echo "Usage: install.sh [OPTIONS]"
            echo ""
            echo "  --version VER     Install specific version (default: latest)"
            echo "  --install-dir DIR Install directory (default: ~/.local/bin)"
            echo "  --no-init         Skip auto-initialization"
            echo "  --no-shell        Skip shell integration"
            echo "  --force           Force overwrite"
            echo "  --help            Show this help"
            exit 0 ;;
        *) err "Unknown option: $1"; exit 1 ;;
    esac
done

main
