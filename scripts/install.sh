#!/bin/bash
set -euo pipefail

# MCP CLI Installer
# Installs MCP CLI (Model Context Protocol CLI tool)
# Compatible with Linux, macOS, and Windows (WSL)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh | bash
#   bash <(curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh)
#
# Or download and run locally:
#   curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh -o install.sh
#   chmod +x install.sh
#   ./install.sh

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Print functions
info() { echo -e "${BLUE}ℹ  $*${NC}"; }
success() { echo -e "${GREEN}✓ $*${NC}"; }
warn() { echo -e "${YELLOW}⚠  $*${NC}"; }
error() { echo -e "${RED}✗ $*${NC}"; exit 1; }

# Detect platform
detect_platform() {
    local kernel="$(uname -s | tr '[:upper:]' '[:lower:]')"
    local arch="$(uname -m)"

    case "$kernel" in
        linux)
            if [[ "$arch" == "x86_64" ]]; then
                echo "linux-amd64"
            elif [[ "$arch" == "aarch64" ]]; then
                echo "linux-arm64"
            else
                echo "linux-386"
            fi
            ;;
        darwin)
            if [[ "$arch" == "x86_64" ]]; then
                echo "darwin-amd64"
            elif [[ "$arch" == "arm64" ]]; then
                echo "darwin-arm64"
            else
                echo "darwin-386"
            fi
            ;;
        cygwin*|mingw*|msys*)
            echo "windows-amd64"
            ;;
        *)
            error "Unsupported platform: $kernel-$arch"
            ;;
    esac
}

# Detect if running in WSL
detect_wsl() {
    if [[ -f /proc/version ]] && grep -qi "microsoft\|wsl" /proc/version; then
        return 0
    else
        return 1
    fi
}

# Detect architecture more precisely
detect_arch() {
    local arch
    arch="$(uname -m)"

    case "$arch" in
        x86_64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        i386|i686) echo "386" ;;
        armv7l) echo "armv7" ;;
        *) echo "$arch" ;;
    esac
}

# Get latest release version
get_latest_version() {
    local api_url="https://api.github.com/repos/EstebanForge/mcp-cli-ent/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$api_url" | grep '"tag_name":' | cut -d'"' -f4
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$api_url" | grep '"tag_name":' | cut -d'"' -f4
    else
        warn "Neither curl nor wget found. Please install one of them."
        echo "latest"
    fi
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install dependencies if needed
install_dependencies() {
    if [[ "$OSTYPE" == "linux-gnu" ]]; then
        if command_exists apt-get; then
            DEBIAN_FRONTEND=noninteractive sudo apt-get update -qq
            DEBIAN_FRONTEND=noninteractive sudo apt-get install -y curl
        elif command_exists yum; then
            sudo yum install -y curl
        elif command_exists dnf; then
            sudo dnf install -y curl
        elif command_exists pacman; then
            sudo pacman -Sy --needed curl
        elif command_exists zypper; then
            sudo zypper install -y curl
        else
            warn "Package manager not detected. Please ensure curl is installed."
        fi
    fi
}


# Main installation function
install_mcp_cli() {
    local platform
    platform="$(detect_platform)"

    # Special handling for WSL
    if detect_wsl; then
        platform="wsl-$platform"
    fi

    local version
    version="$(get_latest_version)"

    local filename="mcp-cli-ent-${platform}"
    if [[ "$platform" == *"windows"* ]]; then
        filename="${filename}.exe"
    fi

    local download_url="https://github.com/EstebanForge/mcp-cli-ent/releases/download/${version}/${filename}"

    info "MCP CLI Installer"
    info "=================="
    info "Platform: $platform"
    info "Version: $version"

    # Set installation directory
    local install_dir
    if [[ "$platform" == *"windows"* ]]; then
        # Windows
        install_dir="${USERPROFILE:-$USERPROFILE}\\AppData\\Roaming\\mcp-cli-ent"
        install_dir_unix="${USERPROFILE:-$HOME}/AppData/Roaming/mcp-cli-ent"
    else
        # Unix-like systems
        install_dir="${HOME:-$HOME}/.local/bin"
        install_dir_unix="$install_dir"
    fi

    info "Installation directory: $install_dir"

    # Create installation directory
    if [[ ! -d "$install_dir_unix" ]]; then
        info "Creating installation directory..."
        mkdir -p "$install_dir_unix"
    fi

    # Check if already installed
    local binary_path="$install_dir_unix/mcp-cli-ent"
    if [[ "$platform" != *"windows"* ]]; then
        binary_path="$install_dir/mcp-cli-ent"
    else
        binary_path="$install_dir\\mcp-cli-ent.exe"
    fi

    if [[ -f "$binary_path" ]] && [[ "${SKIP_EXISTING:-}" != "1" ]]; then
        warn "MCP CLI is already installed at: $binary_path"
        read -p "Do you want to reinstall? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            info "Installation cancelled."
            exit 0
        fi
    fi

    # Download the binary
    info "Downloading MCP CLI-ENT..."
    local temp_file="${TMPDIR:-/tmp}/${filename}"

    if command_exists curl; then
        if ! curl -fsSL "$download_url" -o "$temp_file"; then
            error "Failed to download MCP CLI"
        fi
    elif command_exists wget; then
        if ! wget -q "$download_url" -O "$temp_file"; then
            error "Failed to download MCP CLI"
        fi
    else
        error "Neither curl nor wget found. Please install one of them."
    fi

    # Make binary executable (Unix systems)
    if [[ "$platform" != *"windows"* ]]; then
        chmod +x "$temp_file"
    fi

    # Install the binary
    info "Installing MCP CLI-ENT..."
    if [[ "$platform" != *"windows"* ]]; then
        mv "$temp_file" "$binary_path"
    else
        mv "$temp_file" "$binary_path"
    fi

    # Test installation
    if ! "$binary_path" --version >/dev/null 2>&1; then
        error "Installation verification failed"
    fi

    success "Installation completed successfully!"
    info "Binary installed at: $binary_path"

    # Add to PATH if needed
    check_path "$install_dir_unix"

    # Show first run instructions
    show_first_run_instructions
}

# Check if installation directory is in PATH
check_path() {
    local install_dir="$1"

    # Add to PATH if not already there
    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        warn "Installation directory is not in your PATH"

        # Detect shell profile file
        local profile=""
        if [[ -f "$HOME/.bash_profile" ]]; then
            profile="$HOME/.bash_profile"
        elif [[ -f "$HOME/.profile" ]]; then
            profile="$HOME/.profile"
        elif [[ -f "$HOME/.bashrc" ]]; then
            profile="$HOME/.bashrc"
        elif [[ -f "$HOME/.zshrc" ]]; then
            profile="$HOME/.zshrc"
        fi

        if [[ -n "$profile" ]]; then
            echo "Adding to PATH in $profile"
            echo "" >> "$profile"
            echo "# Added by MCP CLI installer" >> "$profile"
            echo "export PATH=\"\$PATH:$install_dir\"" >> "$profile"
            success "Added to PATH in $profile"
            info "Run 'source $profile' or restart your shell to use MCP CLI"
        else
            warn "Could not detect shell profile file"
            info "Add '$install_dir' to your PATH manually"
        fi
    else
        success "Installation directory is already in your PATH"
    fi
}

# Show first run instructions
show_first_run_instructions() {
    echo
    info "First time setup:"
    echo "  mcp-cli-ent --help          # Show help"
    echo "  mcp-cli-ent create-config    # Create example config"
    echo "  mcp-cli-ent list-servers     # List configured servers"
    echo
    echo "Configuration directory:"
    if [[ "$OSTYPE" == "darwin" ]] || [[ "$OSTYPE" == "linux-gnu" ]]; then
        echo "  ~/.config/mcp-cli-ent/      # Unix-like systems"
    elif detect_wsl; then
        echo "  ~/.config/mcp-cli-ent/      # WSL"
    else
        echo "  %APPDATA%\\mcp-cli-ent\\   # Windows"
    fi
    echo
}

# Main execution
main() {
    # Check if running as root
    if [[ $EUID -eq 0 ]]; then
        warn "Running as root is not recommended"
        warn "Consider installing for the current user instead"
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 0
        fi
    fi

    # Install dependencies if needed (Linux)
    if [[ "$OSTYPE" == "linux-gnu" ]]; then
        if ! command_exists curl; then
            info "Installing required dependencies..."
            install_dependencies
        fi
    fi

    # Perform installation
    install_mcp_cli

    success "MCP CLI installation completed successfully!"
}

# Run main function
main "$@"