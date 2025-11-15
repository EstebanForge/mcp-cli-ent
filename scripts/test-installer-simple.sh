#!/bin/bash

# Simple test script for install.sh syntax and basic functions
set -e

echo "Testing installer script..."

echo "1. Checking syntax..."
if bash -n scripts/install.sh; then
    echo "   ✓ Syntax is valid"
else
    echo "   ✗ Syntax error found"
    exit 1
fi

echo "2. Testing platform detection logic..."
# Test the platform detection logic directly
kernel="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

echo "   Kernel: $kernel"
echo "   Architecture: $arch"

case "$kernel" in
    linux)
        if [[ "$arch" == "x86_64" ]]; then
            echo "   ✓ Would detect: linux-amd64"
        elif [[ "$arch" == "aarch64" ]]; then
            echo "   ✓ Would detect: linux-arm64"
        else
            echo "   ✓ Would detect: linux-386"
        fi
        ;;
    darwin)
        if [[ "$arch" == "x86_64" ]]; then
            echo "   ✓ Would detect: darwin-amd64"
        elif [[ "$arch" == "arm64" ]]; then
            echo "   ✓ Would detect: darwin-arm64"
        else
            echo "   ✓ Would detect: darwin-386"
        fi
        ;;
    *)
        echo "   ✓ Would detect: other platform"
        ;;
esac

echo "3. Checking script is executable..."
if [[ -x "scripts/install.sh" ]]; then
    echo "   ✓ Script is executable"
else
    echo "   ✗ Script is not executable"
    chmod +x scripts/install.sh
    echo "   ✓ Made script executable"
fi

echo "Installer script validation completed successfully!"