#!/bin/bash

# Pre-release validation script
set -e

echo "🚀 MCP CLI-ENT Pre-Release Validation"
echo "===================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Helper functions
success() { echo -e "${GREEN}✅ $*${NC}"; }
warning() { echo -e "${YELLOW}⚠️  $*${NC}"; }
error() { echo -e "${RED}❌ $*${NC}"; exit 1; }

echo ""
echo "1. 📋 Version Validation"
echo "-----------------------"

if [[ -f VERSION ]]; then
    VERSION=$(cat VERSION)
    success "VERSION file exists: $VERSION"
else
    error "VERSION file not found"
fi

if [[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    success "Version format is valid (semantic versioning)"
else
    error "Version format is invalid: $VERSION (expected x.y.z)"
fi

echo ""
echo "2. 🔨 Build System Validation"
echo "-----------------------------"

if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}')
    success "Go installed: $GO_VERSION"
else
    error "Go is not installed"
fi

if [[ -f Makefile ]]; then
    success "Makefile exists"
else
    error "Makefile not found"
fi

echo ""
echo "3. 🏗️  Build Test"
echo "---------------"

echo "Cleaning previous builds..."
make clean >/dev/null 2>&1 || true

echo "Building release binaries..."
if make build-release >/dev/null 2>&1; then
    success "Release build successful"
else
    error "Release build failed"
fi

echo ""
echo "4. 📦 Binary Validation"
echo "----------------------"

REQUIRED_BINARIES=(
    "dist/mcp-cli-ent-linux-amd64"
    "dist/mcp-cli-ent-linux-arm64"
    "dist/mcp-cli-ent-darwin-amd64"
    "dist/mcp-cli-ent-darwin-arm64"
    "dist/mcp-cli-ent-windows-amd64.exe"
    "dist/mcp-cli-ent-windows-arm64.exe"
)

for binary in "${REQUIRED_BINARIES[@]}"; do
    if [[ -f $binary ]]; then
        success "Binary exists: $binary"
    else
        error "Binary missing: $binary"
    fi
done

# Test current platform binary
CURRENT_BINARY="dist/mcp-cli-ent-$(go env GOOS)-$(go env GOARCH)"
if [[ $CURRENT_BINARY == *.exe ]]; then
    CURRENT_BINARY="dist/mcp-cli-ent-windows-$(go env GOARCH).exe"
fi

if [[ -f $CURRENT_BINARY ]]; then
    BINARY_VERSION=$($CURRENT_BINARY --version 2>/dev/null | grep -o "$VERSION" || echo "version mismatch")
    if [[ $BINARY_VERSION == "$VERSION" ]]; then
        success "Binary version correct: $BINARY_VERSION"
    else
        error "Binary version mismatch: expected $VERSION, got $BINARY_VERSION"
    fi
else
    error "Current platform binary not found: $CURRENT_BINARY"
fi

echo ""
echo "5. 📜 Documentation Validation"
echo "-----------------------------"

if [[ -f README.md ]]; then
    if grep -q "curl.*EstebanForge/mcp-cli-ent" README.md; then
        success "README.md contains correct repository URLs"
    else
        error "README.md missing correct repository URLs"
    fi
else
    error "README.md not found"
fi

if [[ -f CHANGELOG.md ]]; then
    if grep -q "\[.*$VERSION.*\]" CHANGELOG.md; then
        success "CHANGELOG.md contains version $VERSION"
    else
        warning "CHANGELOG.md doesn't contain version $VERSION entry"
    fi
else
    error "CHANGELOG.md not found"
fi

if [[ -f RELEASE.md ]]; then
    success "RELEASE.md exists"
else
    error "RELEASE.md not found"
fi

echo ""
echo "6. 🔧 Installer Validation"
echo "-------------------------"

if [[ -f scripts/install.sh ]]; then
    if bash -n scripts/install.sh 2>/dev/null; then
        success "install.sh syntax is valid"
    else
        error "install.sh has syntax errors"
    fi

    if grep -q "EstebanForge/mcp-cli-ent" scripts/install.sh; then
        success "install.sh contains correct repository URLs"
    else
        error "install.sh missing correct repository URLs"
    fi
else
    error "install.sh not found"
fi

if [[ -f scripts/install.ps1 ]]; then
    if command -v pwsh >/dev/null 2>&1; then
        if pwsh -Command "Get-Content scripts/install.ps1 | Out-Null" 2>/dev/null; then
            success "install.ps1 syntax is valid"
        else
            error "install.ps1 has syntax errors"
        fi
    else
        warning "PowerShell not available, skipping install.ps1 validation"
    fi

    if grep -q "EstebanForge/mcp-cli-ent" scripts/install.ps1; then
        success "install.ps1 contains correct repository URLs"
    else
        error "install.ps1 missing correct repository URLs"
    fi
else
    error "install.ps1 not found"
fi

echo ""
echo "7. 📁 Git Repository Validation"
echo "------------------------------"

if [[ -d .git ]]; then
    success "Git repository exists"

    if git remote get-url origin >/dev/null 2>&1; then
        REMOTE_URL=$(git remote get-url origin)
        if [[ $REMOTE_URL == *"EstebanForge/mcp-cli-ent"* ]]; then
            success "Remote URL is correct: $REMOTE_URL"
        else
            error "Remote URL incorrect: $REMOTE_URL (expected EstebanForge/mcp-cli-ent)"
        fi
    else
        error "No remote 'origin' found"
    fi
else
    error "Not a git repository"
fi

echo ""
echo "8. 🤖 GitHub Actions Validation"
echo "-------------------------------"

if [[ -f .github/workflows/release.yml ]]; then
    success "Release workflow exists"

    if grep -q "EstebanForge/mcp-cli-ent" .github/workflows/release.yml; then
        success "Release workflow contains correct repository URLs"
    else
        error "Release workflow missing correct repository URLs"
    fi
else
    error "Release workflow not found"
fi

echo ""
echo "9. 📋 Final Check"
echo "----------------"

# Check for any uncommitted changes
if [[ -n $(git status --porcelain 2>/dev/null) ]]; then
    warning "There are uncommitted changes:"
    git status --porcelain
    warning "Commit these changes before creating a release"
else
    success "No uncommitted changes"
fi

# Check if tag already exists
if git rev-parse "v$VERSION" >/dev/null 2>&1; then
    error "Tag v$VERSION already exists"
else
    success "Tag v$VERSION doesn't exist yet"
fi

echo ""
echo "🎉 Pre-Release Validation Complete!"
echo "=================================="
echo ""
echo "✅ Ready to create release v$VERSION"
echo ""
echo "Next steps:"
echo "1. Commit any pending changes: git add . && git commit -m 'Release v$VERSION'"
echo "2. Create and push tag: git tag v$VERSION && git push origin v$VERSION"
echo "3. GitHub Actions will automatically create the release"
echo ""
success "All validations passed! 🚀"