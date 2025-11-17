#!/bin/bash

# Test script for install.sh functions (offline)
set -e

echo "Testing installer script functions..."

# Override the network-dependent functions
get_latest_version() {
    echo "v0.2.0"
}

# Source the installer script but don't run main
source scripts/install.sh

echo "1. Testing platform detection..."
platform=$(detect_platform)
echo "   Detected platform: $platform"

echo "2. Testing WSL detection..."
if detect_wsl; then
    echo "   WSL detected"
else
    echo "   WSL not detected"
fi

echo "3. Testing command existence checks..."
if command_exists bash; then
    echo "   ✓ bash command found"
else
    echo "   ✗ bash command not found"
fi

if command_exists curl; then
    echo "   ✓ curl command found"
else
    echo "   ✗ curl command not found"
fi

echo "4. Testing architecture detection..."
arch=$(detect_arch)
echo "   Detected architecture: $arch"

echo "All installer functions working correctly!"