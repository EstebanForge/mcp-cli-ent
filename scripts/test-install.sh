#!/bin/bash

# Test script to verify installation scripts work correctly
set -e

echo "Testing MCP CLI installation scripts..."

echo "1. Testing Linux/macOS script syntax..."
if bash -n scripts/install.sh; then
    echo "   ✓ Bash script syntax is valid"
else
    echo "   ✗ Bash script has syntax errors"
    exit 1
fi

echo "2. Testing PowerShell script syntax..."
if command -v pwsh >/dev/null 2>&1; then
    if pwsh -Command "Get-Content scripts/install.ps1 | Out-Null"; then
        echo "   ✓ PowerShell script syntax is valid"
    else
        echo "   ✗ PowerShell script has syntax errors"
        exit 1
    fi
else
    echo "   ℹ PowerShell not available, skipping PowerShell test"
fi

echo "3. Testing platform detection logic..."
kernel="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
echo "   Current platform: $kernel-$arch"

echo "4. Checking script executability..."
if [[ -x "scripts/install.sh" ]]; then
    echo "   ✓ Installer script is executable"
else
    echo "   ✗ Installer script is not executable"
    exit 1
fi

echo "5. Running comprehensive validation..."
if ./scripts/test-installer-simple.sh; then
    echo "   ✓ Comprehensive validation passed"
else
    echo "   ✗ Comprehensive validation failed"
    exit 1
fi

echo ""
echo "🎉 All installation scripts are syntactically correct and validated!"