#!/usr/bin/env bash

set -e

setup_shell_and_path() {
    BIN_DIR=${BIN_DIR:-$HOME/.local/bin}

    case $SHELL in
        */zsh)
            PROFILE=$HOME/.zshrc
            ;;
        */bash)
            PROFILE=$HOME/.bashrc
            ;;
        */fish)
            PROFILE=$HOME/.config/fish/config.fish
            ;;
        */ash)
            PROFILE=$HOME/.profile
            ;;
        *)
            echo "could not detect shell, manually add ${BIN_DIR} to your PATH."
            exit 1
    esac

    if [[ ":$PATH:" != *":${BIN_DIR}:"* ]]; then
        echo >> "$PROFILE" && echo "export PATH=\"\$PATH:$BIN_DIR\"" >> "$PROFILE"
    fi
}

detect_platform_and_arch() {
    PLATFORM="$(uname | tr '[:upper:]' '[:lower:]')"
    if [[ "$PLATFORM" == mingw*_nt* ]]; then
        PLATFORM="windows"
    fi

    ARCHITECTURE="$(uname -m)"
    if [ "${ARCHITECTURE}" = "x86_64" ]; then
        # Redirect stderr to /dev/null to avoid printing errors if non Rosetta.
        if [ "$(sysctl -n sysctl.proc_translated 2>/dev/null)" = "1" ]; then
            ARCHITECTURE="arm64" # Rosetta.
        else
            ARCHITECTURE="amd64" # Intel.
        fi
    elif [ "${ARCHITECTURE}" = "arm64" ] || [ "${ARCHITECTURE}" = "aarch64" ]; then
        ARCHITECTURE="arm64" # Arm.
    else
        ARCHITECTURE="amd64" # Amd.
    fi

    if [[ "$PLATFORM" == "windows" ]]; then
        ARCHIVE_EXT=".zip"
        EXTENSION=".exe"
    else
        ARCHIVE_EXT=".tar.gz"
        EXTENSION=""
    fi
}

get_latest_version() {
    # Get latest version from GitHub API, including prereleases
    API_RESPONSE=$(curl -sS "https://api.github.com/repos/howinator/agent-farmer/releases")
    if [ $? -ne 0 ]; then
        echo "Failed to connect to GitHub API"
        exit 1
    fi
    
    if echo "$API_RESPONSE" | grep -q "Not Found"; then
        echo "No releases found in the repository"
        exit 1
    fi
    
    # Get the first release (latest) from the array
    LATEST_VERSION=$(echo "$API_RESPONSE" | grep -m1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//')
    if [ -z "$LATEST_VERSION" ]; then
        echo "Failed to parse version from GitHub API response:"
        echo "$API_RESPONSE" | grep -v "upload_url" # Filter out long upload_url line
        exit 1
    fi
    echo "$LATEST_VERSION"
}

download_release() {
    local version=$1
    local binary_url=$2
    local archive_name=$3
    local tmp_dir=$4

    echo "Downloading binary from $binary_url"
    DOWNLOAD_OUTPUT=$(curl -sS -L -f -w '%{http_code}' "$binary_url" -o "${tmp_dir}/${archive_name}" 2>&1)
    HTTP_CODE=$?
    
    if [ $HTTP_CODE -ne 0 ]; then
        echo "Error: Failed to download release asset"
        echo "This could be because:"
        echo "1. The release ${version} doesn't have assets uploaded yet"
        echo "2. The asset for ${PLATFORM}_${ARCHITECTURE} wasn't built"
        echo "3. The asset name format has changed"
        echo ""
        echo "Expected asset name: ${archive_name}"
        echo "URL attempted: ${binary_url}"
        if [ "$version" == "latest" ]; then
            echo ""
            echo "Tip: Try installing a specific version instead of 'latest'"
            echo "Available versions:"
            echo "$API_RESPONSE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//'
        fi
        rm -rf "$tmp_dir"
        exit 1
    fi
}

extract_and_install() {
    local tmp_dir=$1
    local archive_name=$2
    local bin_dir=$3
    local extension=$4

    if [[ "$PLATFORM" == "windows" ]]; then
        if ! unzip -t "${tmp_dir}/${archive_name}" > /dev/null 2>&1; then
            echo "Error: Downloaded file is not a valid zip archive"
            rm -rf "$tmp_dir"
            exit 1
        fi
        ensure unzip "${tmp_dir}/${archive_name}" -d "$tmp_dir"
    else
        if ! tar tzf "${tmp_dir}/${archive_name}" > /dev/null 2>&1; then
            echo "Error: Downloaded file is not a valid tar.gz archive"
            rm -rf "$tmp_dir"
            exit 1
        fi
        ensure tar xzf "${tmp_dir}/${archive_name}" -C "$tmp_dir"
    fi

    if [ ! -d "$bin_dir" ]; then
        mkdir -p "$bin_dir"
    fi

    # Remove existing binary if upgrading
    if [ "$UPGRADE_MODE" = true ] && [ -f "$bin_dir/$INSTALL_NAME${extension}" ]; then
        echo "Removing previous installation from $bin_dir/$INSTALL_NAME${extension}"
        rm -f "$bin_dir/$INSTALL_NAME${extension}"
    fi

    # Install binary with desired name
    mv "${tmp_dir}/agent-farmer${extension}" "$bin_dir/$INSTALL_NAME${extension}"
    rm -rf "$tmp_dir"

    if [ ! -f "$bin_dir/$INSTALL_NAME${extension}" ]; then
        echo "Installation failed, could not find $bin_dir/$INSTALL_NAME${extension}"
        exit 1
    fi

    chmod +x "$bin_dir/$INSTALL_NAME${extension}"
    
    echo ""
    if [ "$UPGRADE_MODE" = true ]; then
        echo "Successfully upgraded '$INSTALL_NAME' to:"
    else
        echo "Installed as '$INSTALL_NAME':"
    fi
    echo "$("$bin_dir/$INSTALL_NAME${extension}" version)"
}

check_command_exists() {
    if command -v "$INSTALL_NAME" &> /dev/null; then
        EXISTING_PATH=$(which "$INSTALL_NAME")
        echo "Found existing installation of '$INSTALL_NAME' at $EXISTING_PATH"
        echo "Will upgrade to the latest version"
        UPGRADE_MODE=true
    else
        UPGRADE_MODE=false
    fi
}

check_and_install_dependencies() {
    echo "Checking for required dependencies..."
    
    # Check for tmux
    if ! command -v tmux &> /dev/null; then
        echo "tmux is not installed. Installing tmux..."
        
        if [[ "$PLATFORM" == "darwin" ]]; then
            # macOS
            if command -v brew &> /dev/null; then
                ensure brew install tmux
            else
                echo "Homebrew is not installed. Please install Homebrew first to install tmux."
                echo "Visit https://brew.sh for installation instructions."
                exit 1
            fi
        elif [[ "$PLATFORM" == "linux" ]]; then
            # Linux
            if command -v apt-get &> /dev/null; then
                ensure sudo apt-get update
                ensure sudo apt-get install -y tmux
            elif command -v dnf &> /dev/null; then
                ensure sudo dnf install -y tmux
            elif command -v yum &> /dev/null; then
                ensure sudo yum install -y tmux
            elif command -v pacman &> /dev/null; then
                ensure sudo pacman -S --noconfirm tmux
            else
                echo "Could not determine package manager. Please install tmux manually."
                exit 1
            fi
        elif [[ "$PLATFORM" == "windows" ]]; then
            echo "For Windows, please install tmux via WSL or another method."
            exit 1
        fi
        
        echo "tmux installed successfully."
    else
        echo "tmux is already installed."
    fi
    
    # Check for GitHub CLI (gh)
    if ! command -v gh &> /dev/null; then
        echo "GitHub CLI (gh) is not installed. Installing GitHub CLI..."
        
        if [[ "$PLATFORM" == "darwin" ]]; then
            # macOS
            if command -v brew &> /dev/null; then
                ensure brew install gh
            else
                echo "Homebrew is not installed. Please install Homebrew first to install GitHub CLI."
                echo "Visit https://brew.sh for installation instructions."
                exit 1
            fi
        elif [[ "$PLATFORM" == "linux" ]]; then
            # Linux
            if command -v apt-get &> /dev/null; then
                echo "Installing GitHub CLI on Debian/Ubuntu..."
                ensure curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
                ensure sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
                ensure echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
                ensure sudo apt-get update
                ensure sudo apt-get install -y gh
            elif command -v dnf &> /dev/null; then
                ensure sudo dnf install -y 'dnf-command(config-manager)'
                ensure sudo dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
                ensure sudo dnf install -y gh
            elif command -v yum &> /dev/null; then
                ensure sudo yum install -y yum-utils
                ensure sudo yum-config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
                ensure sudo yum install -y gh
            elif command -v pacman &> /dev/null; then
                ensure sudo pacman -S --noconfirm github-cli
            else
                echo "Could not determine package manager. Please install GitHub CLI manually."
                echo "Visit https://github.com/cli/cli#installation for installation instructions."
                exit 1
            fi
        elif [[ "$PLATFORM" == "windows" ]]; then
            echo "For Windows, please install GitHub CLI manually."
            echo "Visit https://github.com/cli/cli#installation for installation instructions."
            exit 1
        fi
        
        echo "GitHub CLI (gh) installed successfully."
    else
        echo "GitHub CLI (gh) is already installed."
    fi
    
    echo "All dependencies are installed."
}

main() {
    # Parse command line arguments
    INSTALL_NAME="af"
    UPGRADE_MODE=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --name)
                INSTALL_NAME="$2"
                shift 2
                ;;
            *)
                echo "Unknown option: $1"
                echo "Usage: install.sh [--name <n>]"
                exit 1
                ;;
        esac
    done

    check_command_exists
    detect_platform_and_arch
    
    check_and_install_dependencies
    
    setup_shell_and_path

    VERSION=${VERSION:-"latest"}
    if [[ "$VERSION" == "latest" ]]; then
        VERSION=$(get_latest_version)
    fi

    RELEASE_URL="https://github.com/howinator/agent-farmer/releases/download/v${VERSION}"
    ARCHIVE_NAME="agent-farmer_${VERSION}_${PLATFORM}_${ARCHITECTURE}${ARCHIVE_EXT}"
    BINARY_URL="${RELEASE_URL}/${ARCHIVE_NAME}"
    TMP_DIR=$(mktemp -d)
    
    download_release "$VERSION" "$BINARY_URL" "$ARCHIVE_NAME" "$TMP_DIR"
    extract_and_install "$TMP_DIR" "$ARCHIVE_NAME" "$BIN_DIR" "$EXTENSION"
}

# Run a command that should never fail. If the command fails execution
# will immediately terminate with an error showing the failing
# command.
ensure() {
    if ! "$@"; then err "command failed: $*"; fi
}

err() {
    echo "$1" >&2
    exit 1
}

main "$@" || exit 1
