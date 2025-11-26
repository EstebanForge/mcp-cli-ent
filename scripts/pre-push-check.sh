#!/bin/bash

# Pre-push validation script - Replicates GitHub Actions CI/CD checks locally
# Run this before pushing to catch issues early
set -e

echo "🚦 MCP CLI-ENT Pre-Push Validation"
echo "==================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Helper functions
step() { echo -e "${BLUE}▶ $*${NC}"; }
success() { echo -e "${GREEN}✅ $*${NC}"; PASSED=$((PASSED + 1)); }
warning() { echo -e "${YELLOW}⚠️  $*${NC}"; WARNINGS=$((WARNINGS + 1)); }
error() { echo -e "${RED}❌ $*${NC}"; FAILED=$((FAILED + 1)); }
fatal() { echo -e "${RED}❌ $*${NC}"; echo ""; echo "🛑 Pre-push validation FAILED"; exit 1; }

# Track if we should exit with error
SHOULD_FAIL=false

echo "This script replicates all GitHub Actions checks locally"
echo "to catch issues before pushing to remote."
echo ""

# =============================================================================
# 1. ENVIRONMENT CHECK
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1️⃣  Environment Check"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Checking Go installation..."
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}')
    success "Go installed: $GO_VERSION"
else
    fatal "Go is not installed"
fi

step "Checking golangci-lint installation..."
if command -v golangci-lint >/dev/null 2>&1; then
    LINT_VERSION=$(golangci-lint --version | head -n1)
    success "golangci-lint installed: $LINT_VERSION"
else
    error "golangci-lint not installed (install: https://golangci-lint.run/welcome/install/)"
    warning "  → brew install golangci-lint (macOS)"
    warning "  → go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    SHOULD_FAIL=true
fi

step "Checking git repository..."
if [[ -d .git ]]; then
    success "Git repository exists"
else
    fatal "Not a git repository"
fi

echo ""

# =============================================================================
# 2. DEPENDENCY VALIDATION (matches GitHub Actions)
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2️⃣  Dependency Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Downloading dependencies..."
if go mod download 2>&1; then
    success "Dependencies downloaded"
else
    error "Failed to download dependencies"
    SHOULD_FAIL=true
fi

step "Verifying dependencies (checks for tampering)..."
if go mod verify 2>&1; then
    success "Dependencies verified"
else
    error "Dependency verification failed - possible tampering detected!"
    SHOULD_FAIL=true
fi

echo ""

# =============================================================================
# 3. CONFIGURATION VALIDATION
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3️⃣  Configuration Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Checking config file sync..."
if make check-config >/dev/null 2>&1; then
    success "Config files are in sync"
else
    error "Config files out of sync (mcp_servers.example.json)"
    SHOULD_FAIL=true
fi

echo ""

# =============================================================================
# 4. CODE QUALITY CHECKS
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4️⃣  Code Quality Checks"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Running go fmt..."
UNFORMATTED=$(gofmt -l . 2>&1 | grep -v vendor || true)
if [[ -z "$UNFORMATTED" ]]; then
    success "All files are properly formatted"
else
    error "Unformatted files found:"
    echo "$UNFORMATTED" | sed 's/^/    /'
    warning "Run: make fmt"
    SHOULD_FAIL=true
fi

step "Running golangci-lint (timeout: 5m)..."
if command -v golangci-lint >/dev/null 2>&1; then
    if golangci-lint run --timeout=5m 2>&1; then
        success "Linter passed"
    else
        error "Linter found issues"
        SHOULD_FAIL=true
    fi
else
    warning "Skipping linter (not installed)"
fi

echo ""

# =============================================================================
# 5. TESTS
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5️⃣  Test Suite"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Running tests..."
if go test -v ./... 2>&1 | tee /tmp/test_output.txt; then
    if grep -q "no test files" /tmp/test_output.txt; then
        warning "No test files found"
    else
        success "All tests passed"
    fi
else
    error "Tests failed"
    SHOULD_FAIL=true
fi

echo ""

# =============================================================================
# 6. BUILD VALIDATION
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "6️⃣  Build Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Building current platform binary..."
if make build >/dev/null 2>&1; then
    success "Current platform build successful"
else
    error "Current platform build failed"
    SHOULD_FAIL=true
fi

step "Testing binary execution..."
CURRENT_BINARY="./bin/mcp-cli-ent"
if [[ -f "$CURRENT_BINARY" ]]; then
    if "$CURRENT_BINARY" --version >/dev/null 2>&1; then
        VERSION_OUTPUT=$("$CURRENT_BINARY" --version)
        success "Binary executes successfully: $VERSION_OUTPUT"
    else
        error "Binary execution failed"
        SHOULD_FAIL=true
    fi
else
    error "Binary not found at $CURRENT_BINARY"
    SHOULD_FAIL=true
fi

step "Building all platforms (like GitHub Actions)..."
if make build-all >/dev/null 2>&1; then
    success "Multi-platform build successful"
else
    error "Multi-platform build failed"
    SHOULD_FAIL=true
fi

echo ""

# =============================================================================
# 7. BINARY VALIDATION (Release-level checks)
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "7️⃣  Binary Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

REQUIRED_BINARIES=(
    "dist/mcp-cli-ent-linux-amd64"
    "dist/mcp-cli-ent-linux-arm64"
    "dist/mcp-cli-ent-darwin-amd64"
    "dist/mcp-cli-ent-darwin-arm64"
    "dist/mcp-cli-ent-windows-amd64.exe"
    "dist/mcp-cli-ent-windows-arm64.exe"
)

MIN_SIZE=10000000  # 10MB minimum (GitHub Actions requirement)

for binary in "${REQUIRED_BINARIES[@]}"; do
    if [[ -f "$binary" ]]; then
        SIZE=$(stat -f%z "$binary" 2>/dev/null || stat -c%s "$binary" 2>/dev/null)
        SIZE_MB=$((SIZE / 1024 / 1024))

        if [[ $SIZE -ge $MIN_SIZE ]]; then
            success "$(basename "$binary"): ${SIZE_MB}MB (valid)"
        else
            error "$(basename "$binary"): ${SIZE_MB}MB (too small, min 10MB)"
            SHOULD_FAIL=true
        fi

        # Binary format verification
        if command -v file >/dev/null 2>&1; then
            FILE_TYPE=$(file "$binary" 2>/dev/null || echo "unknown")
            case "$binary" in
                *darwin*)
                    if echo "$FILE_TYPE" | grep -q "Mach-O\|executable"; then
                        success "  → Valid macOS binary format"
                    else
                        warning "  → Could not verify macOS binary format"
                    fi
                    ;;
                *windows*)
                    if echo "$FILE_TYPE" | grep -q "PE\|executable"; then
                        success "  → Valid Windows binary format"
                    else
                        warning "  → Could not verify Windows binary format"
                    fi
                    ;;
                *linux*)
                    if echo "$FILE_TYPE" | grep -q "ELF\|executable"; then
                        success "  → Valid Linux binary format"
                    else
                        warning "  → Could not verify Linux binary format"
                    fi
                    ;;
            esac
        fi
    else
        error "$(basename "$binary"): Missing"
        SHOULD_FAIL=true
    fi
done

echo ""

# =============================================================================
# 8. GIT STATUS CHECK
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "8️⃣  Git Status"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

step "Checking for uncommitted changes..."
if [[ -n $(git status --porcelain 2>/dev/null) ]]; then
    warning "Uncommitted changes detected:"
    git status --short | sed 's/^/    /'
    warning "Consider committing before pushing"
else
    success "No uncommitted changes"
fi

step "Checking current branch..."
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$CURRENT_BRANCH" == "main" ]] || [[ "$CURRENT_BRANCH" == "develop" ]]; then
    success "On branch: $CURRENT_BRANCH"
else
    warning "On branch: $CURRENT_BRANCH (not main/develop)"
fi

echo ""

# =============================================================================
# 9. SUMMARY
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "  ✅ Passed:   $PASSED"
echo "  ⚠️  Warnings: $WARNINGS"
echo "  ❌ Failed:   $FAILED"
echo ""

if [[ "$SHOULD_FAIL" == "true" ]] || [[ $FAILED -gt 0 ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🛑 Pre-push validation FAILED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Please fix the issues above before pushing."
    echo "These same checks will run in GitHub Actions."
    echo ""
    exit 1
else
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "✅ Pre-push validation PASSED!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Your code is ready to push. GitHub Actions should pass."
    echo ""

    if [[ $WARNINGS -gt 0 ]]; then
        echo "⚠️  Note: $WARNINGS warning(s) detected (non-blocking)"
        echo ""
    fi

    exit 0
fi
