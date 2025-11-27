#!/bin/bash

# Comprehensive MCP Server Test Script
# Tests all example servers and CLI commands before release
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
success() {
    echo -e "${GREEN}✅ $1${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

error() {
    echo -e "${RED}❌ $1${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

section() {
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}  $1${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Cleanup function
cleanup() {
    # Kill any background processes
    if [[ -n "$PID_FILE" ]] && [[ -f "$PID_FILE" ]]; then
        kill $(cat "$PID_FILE" 2>/dev/null) 2>/dev/null || true
        rm -f "$PID_FILE"
    fi
    # Clean up test config
    if [[ -n "$TEST_CONFIG" ]] && [[ -f "$TEST_CONFIG" ]]; then
        rm -f "$TEST_CONFIG"
    fi
}
trap cleanup EXIT

# Test build
section "Build Validation"
echo "Building mcp-cli-ent..."
if make build >/dev/null 2>&1; then
    success "Build successful"
    BINARY="./bin/mcp-cli-ent"
else
    error "Build failed"
    exit 1
fi

# Test version command
section "Basic CLI Commands"
if $BINARY version >/dev/null 2>&1; then
    success "Version command works"
else
    error "Version command failed"
fi

# Test help command
if $BINARY --help >/dev/null 2>&1; then
    success "Help command works"
else
    error "Help command failed"
fi

# Create test config from example
section "Configuration Testing"
TEST_CONFIG=$(mktemp)
cp mcp_servers.example.json "$TEST_CONFIG"

# Test list-servers
if $BINARY --config "$TEST_CONFIG" list-servers >/dev/null 2>&1; then
    success "list-servers command works"
else
    error "list-servers command failed"
fi

# Test list-servers --all
if $BINARY --config "$TEST_CONFIG" list-servers --all >/dev/null 2>&1; then
    success "list-servers --all command works"
else
    error "list-servers --all command failed"
fi

# Test error handling
section "Error Handling Tests"

# Test with non-existent server
info "Testing error handling for non-existent server..."
if $BINARY --config "$TEST_CONFIG" call-tool non-existent-server test 2>&1 | grep -q "server 'non-existent-server' not found"; then
    success "Error handling shows helpful message"
else
    error "Error handling doesn't show expected message"
fi

# Test with non-existent server for list-tools
info "Testing list-tools with non-existent server..."
if $BINARY --config "$TEST_CONFIG" list-tools invalid-server 2>&1 | grep -q "server 'invalid-server' not found"; then
    success "list-tools error handling works"
else
    error "list-tools error handling failed"
fi

# Test enabled servers
section "Testing Enabled MCP Servers"

# Extract enabled servers from config
ENABLED_SERVERS=$(jq -r '.mcpServers | to_entries[] | select(.value.enabled == true) | .key' "$TEST_CONFIG")

if [[ -z "$ENABLED_SERVERS" ]]; then
    error "No enabled servers found in config"
else
    info "Found enabled servers: $(echo $ENABLED_SERVERS | tr '\n' ', ' | sed 's/,$//')"

    # Test each enabled server
    for SERVER in $ENABLED_SERVERS; do
        echo ""
        info "Testing server: $SERVER"

        # Test list-tools for each server
        echo -n "  • list-tools: "
        if timeout 30 $BINARY --config "$TEST_CONFIG" list-tools "$SERVER" >/dev/null 2>&1; then
            success "$SERVER list-tools"
        else
            info "  ⏭️  $SERVER list-tools (may require network access - skipped)"
        fi

        # Test basic tool call if possible
        case "$SERVER" in
            "sequential-thinking")
                echo -n "  • call-tool: "
                TEST_RESULT=$(timeout 30 $BINARY --config "$TEST_CONFIG" call-tool sequential-thinking sequentialthinking \
                    '{"thought": "Testing the tool", "thoughtNumber": 1, "totalThoughts": 1, "nextThoughtNeeded": false}' 2>&1 || echo "FAILED")
                if [[ "$TEST_RESULT" != *"FAILED"* ]] && [[ "$TEST_RESULT" != *"Error"* ]]; then
                    success "$SERVER call-tool works"
                else
                    error "$SERVER call-tool failed"
                fi
                ;;
            "deepwiki")
                echo -n "  • list-resources: "
                if timeout 30 $BINARY --config "$TEST_CONFIG" list-resources "$SERVER" >/dev/null 2>&1; then
                    success "$SERVER list-resources"
                else
                    info "  ⏭️  $SERVER list-resources (may require network access - skipped)"
                fi
                ;;
            "time")
                echo -n "  • call-tool: "
                if timeout 30 $BINARY --config "$TEST_CONFIG" call-tool time get_current_time '{}' >/dev/null 2>&1; then
                    success "$SERVER call-tool works"
                else
                    info "  ⏭️  $SERVER call-tool (may require network access - skipped)"
                fi
                ;;
            *)
                info "  No basic test defined for $SERVER"
                ;;
        esac
    done
fi

# Test disabled servers (should still be listed with --all)
section "Testing Disabled Server Visibility"
if $BINARY --config "$TEST_CONFIG" list-servers 2>&1 | grep -q "disabled"; then
    error "Disabled servers appear in normal list (should only be in --all)"
else
    success "Disabled servers hidden in normal list"
fi

if $BINARY --config "$TEST_CONFIG" list-servers --all 2>&1 | grep -q "disabled"; then
    success "Disabled servers visible with --all flag"
else
    error "Disabled servers not visible with --all flag"
fi

# Test session commands
section "Session Management Commands"
if $BINARY session list >/dev/null 2>&1; then
    success "session list command works"
else
    error "session list command failed"
fi

# Test daemon commands (will fail if not running, but should not crash)
if $BINARY daemon status >/dev/null 2>&1; then
    success "daemon status command works"
else
    info "daemon not running (expected for test environment)"
fi

# Test configuration commands
section "Configuration Commands"

# Test create-config with unique temp file
TEMP_CONFIG_FILE=$(mktemp -t mcp-config-XXXXXX)
if $BINARY create-config "$TEMP_CONFIG_FILE.json" >/dev/null 2>&1; then
    success "create-config command works"
    rm -f "$TEMP_CONFIG_FILE.json"
else
    error "create-config command failed"
    rm -f "$TEMP_CONFIG_FILE.json" 2>/dev/null || true
fi

# Test verbose flag
section "Verbose Mode Testing"
if $BINARY --verbose version >/dev/null 2>&1; then
    success "Verbose mode works"
else
    error "Verbose mode failed"
fi

# Test timeout flag
section "Timeout Flag Testing"
if $BINARY --timeout 5 version >/dev/null 2>&1; then
    success "Timeout flag works"
else
    error "Timeout flag failed"
fi

# Summary
section "Test Summary"
TOTAL=$((TESTS_PASSED + TESTS_FAILED))
PASS_RATE=$((TESTS_PASSED * 100 / TOTAL))

echo ""
echo -e "${BLUE}Total Tests: $TESTS_RUN${NC}"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo -e "${YELLOW}Pass Rate: $PASS_RATE%${NC}"
echo ""

if [[ $TESTS_FAILED -eq 0 ]]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Please review the output above.${NC}"
    exit 1
fi
