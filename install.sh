#!/usr/bin/env bash
#
# Install claude-squad from the rakesh97 fork by building from source.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rakesh97/claude-squad/main/install.sh | bash
#
# Environment overrides:
#   REPO          default: rakesh97/claude-squad
#   BRANCH        default: main
#   BIN_DIR       default: $HOME/.local/bin
#   SRC_DIR       default: $HOME/.cache/claude-squad-src
#   INSTALL_NAME  default: claude-squad        (the binary name)
#   NO_ALIAS      if set, skip creating the `cs` shell alias

set -euo pipefail

REPO="${REPO:-rakesh97/claude-squad}"
BRANCH="${BRANCH:-main}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
SRC_DIR="${SRC_DIR:-$HOME/.cache/claude-squad-src}"
INSTALL_NAME="${INSTALL_NAME:-claude-squad}"

log()  { printf '==> %s\n' "$*"; }
warn() { printf 'warn: %s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

detect_profile() {
    case "${SHELL:-}" in
        */zsh)  echo "$HOME/.zshrc" ;;
        */bash) echo "$HOME/.bashrc" ;;
        */fish) echo "$HOME/.config/fish/config.fish" ;;
        *)      echo "" ;;
    esac
}

install_pkg() {
    local pkg="$1"
    local platform
    platform="$(uname | tr '[:upper:]' '[:lower:]')"

    if [[ "$platform" == "darwin" ]]; then
        command -v brew >/dev/null || die "Homebrew is required to install $pkg on macOS. See https://brew.sh"
        brew install "$pkg"
    elif [[ "$platform" == "linux" ]]; then
        if command -v apt-get >/dev/null; then sudo apt-get update && sudo apt-get install -y "$pkg"
        elif command -v dnf     >/dev/null; then sudo dnf install -y "$pkg"
        elif command -v yum     >/dev/null; then sudo yum install -y "$pkg"
        elif command -v pacman  >/dev/null; then sudo pacman -S --noconfirm "$pkg"
        else die "No supported package manager found. Install $pkg manually."
        fi
    else
        die "Unsupported platform: $platform. Install $pkg manually."
    fi
}

ensure_dep() {
    local cmd="$1" pkg="${2:-$1}"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        log "$cmd not found, installing..."
        install_pkg "$pkg"
    fi
}

ensure_path() {
    if [[ ":$PATH:" == *":$BIN_DIR:"* ]]; then return; fi
    local profile
    profile="$(detect_profile)"
    if [ -z "$profile" ]; then
        warn "could not detect shell; add $BIN_DIR to your PATH manually."
        return
    fi
    if [ -f "$profile" ] && grep -qF "$BIN_DIR" "$profile"; then return; fi
    log "Adding $BIN_DIR to PATH in $profile"
    {
        echo ""
        echo "export PATH=\"\$PATH:$BIN_DIR\""
    } >> "$profile"
}

ensure_cs_alias() {
    [ -n "${NO_ALIAS:-}" ] && return
    local profile alias_line
    profile="$(detect_profile)"
    [ -z "$profile" ] && return
    alias_line="alias cs='TERM=xterm-256color $BIN_DIR/$INSTALL_NAME'"

    if [ -f "$profile" ] && grep -qE "^alias cs=" "$profile"; then
        log "Existing \`cs\` alias found in $profile — leaving it alone."
        log "If it points at an old location, replace it with:"
        log "  $alias_line"
        return
    fi
    log "Adding \`cs\` alias to $profile"
    {
        echo ""
        echo "$alias_line"
    } >> "$profile"
}

main() {
    ensure_dep git
    ensure_dep go
    ensure_dep tmux
    ensure_dep gh

    if [ -d "$SRC_DIR/.git" ]; then
        log "Updating source in $SRC_DIR"
        git -C "$SRC_DIR" fetch --depth=1 origin "$BRANCH"
        git -C "$SRC_DIR" reset --hard "origin/$BRANCH"
    else
        log "Cloning $REPO into $SRC_DIR"
        mkdir -p "$(dirname "$SRC_DIR")"
        git clone --depth=1 -b "$BRANCH" "https://github.com/${REPO}.git" "$SRC_DIR"
    fi

    mkdir -p "$BIN_DIR"
    log "Building $INSTALL_NAME -> $BIN_DIR/$INSTALL_NAME"
    ( cd "$SRC_DIR" && go build -o "$BIN_DIR/$INSTALL_NAME" . )

    ensure_path
    ensure_cs_alias

    echo ""
    log "Installed: $("$BIN_DIR/$INSTALL_NAME" version)"
    echo ""
    echo "Restart your shell (or \`source $(detect_profile)\`) to pick up PATH/alias changes."
}

main "$@"
