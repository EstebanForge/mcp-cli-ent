.PHONY: build sign build-signed release-sign notarize-release test test-mcp-servers clean install release fmt vet lint deps sync-config check-config check

# Default target
all: build

# Variables
BINARY_NAME=mcp-cli-ent
BINARY_PATH=bin/${BINARY_NAME}
RELEASE_VERSION=$(shell cat VERSION 2>/dev/null || echo "0.1.0")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo $(RELEASE_VERSION))
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X github.com/mcp-cli-ent/mcp-cli/pkg/version.Version=${VERSION} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Commit=${COMMIT} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Date=${DATE}"
RELEASE_LDFLAGS=-ldflags "-X github.com/mcp-cli-ent/mcp-cli/pkg/version.Version=${RELEASE_VERSION} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Commit=${COMMIT} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Date=${DATE}"
SIGN_IDENTITY?=-

# Build for current platform
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p $(dir ${BINARY_PATH})
	go build -o ${BINARY_PATH} ./cmd/mcp-cli-ent

sign: build
	@echo "Signing ${BINARY_PATH} (macOS only)..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign -s "$(SIGN_IDENTITY)" -f "${BINARY_PATH}"; \
		echo "✓ Signed: ${BINARY_PATH}"; \
	else \
		echo "ℹ️  Skipping sign (non-macOS)"; \
	fi

build-signed: build sign

release-sign:
	@if [ "$${RELEASE_SIGN:-0}" != "1" ]; then \
		echo "ℹ️  RELEASE_SIGN!=1; skipping release signing"; \
	elif [ "$$(uname)" != "Darwin" ]; then \
		echo "ℹ️  Release signing requires a macOS runner; skipping"; \
	else \
		for bin in dist/${BINARY_NAME}-darwin-amd64 dist/${BINARY_NAME}-darwin-arm64; do \
			if [ -f "$$bin" ]; then \
				codesign -s "$(SIGN_IDENTITY)" -f "$$bin"; \
				echo "✓ Signed $$bin"; \
			else \
				echo "ℹ️  Missing $$bin (skip)"; \
			fi; \
		done; \
	fi

notarize-release:
	@if [ "$${RELEASE_NOTARIZE:-0}" != "1" ]; then \
		echo "ℹ️  RELEASE_NOTARIZE!=1; skipping notarization"; \
	elif [ "$$(uname)" != "Darwin" ]; then \
		echo "✗ Notarization requires macOS runner"; \
		exit 1; \
	elif [ -x "./scripts/notarize-release.sh" ]; then \
		./scripts/notarize-release.sh; \
	else \
		echo "✗ scripts/notarize-release.sh not found/executable"; \
		exit 1; \
	fi

# Build for all platforms
build-all:
	@echo "Building ${BINARY_NAME} for all platforms..."
	@mkdir -p dist

	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/${BINARY_NAME}

	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/${BINARY_NAME}

	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/${BINARY_NAME}

	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/${BINARY_NAME}

	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/${BINARY_NAME}

	# Windows ARM64
	GOOS=windows GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-arm64.exe ./cmd/${BINARY_NAME}

	@echo "Build artifacts created in dist/"

# Build for all platforms (release version)
build-release:
	@echo "Building ${BINARY_NAME} v${RELEASE_VERSION} for all platforms..."
	@mkdir -p dist

	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/${BINARY_NAME}

	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/${BINARY_NAME}

	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/${BINARY_NAME}

	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/${BINARY_NAME}

	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/${BINARY_NAME}

	# Windows ARM64
	GOOS=windows GOARCH=arm64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-windows-arm64.exe ./cmd/${BINARY_NAME}

	@echo "Release artifacts created in dist/"
	@echo "Version: ${RELEASE_VERSION}"

# Set version (for releases)
set-version:
	@echo "Setting version to $(VERSION)"
	@echo "$(VERSION)" > VERSION

# Run tests
test:
	@echo "Downloading dependencies..."
	go mod download
	@echo "Verifying dependencies..."
	go mod verify
	@echo "Running tests..."
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		go test -v -race ./...; \
	else \
		echo "CGO disabled; running tests without -race"; \
		go test -v ./...; \
	fi

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Test all MCP servers and CLI commands
test-mcp-servers:
	@echo "Testing all MCP servers and CLI commands..."
	@./scripts/test-mcp-servers.sh

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "⚠️  goimports not found; skipping import formatting"; \
		echo "   Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Lint code
lint:
	@echo "Linting code..."
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

# Run full checks
check:
	@echo "==> make check-config"
	@$(MAKE) --no-print-directory check-config
	@echo "==> make fmt"
	@$(MAKE) --no-print-directory fmt
	@echo "==> make vet"
	@$(MAKE) --no-print-directory vet
	@echo "==> make lint"
	@$(MAKE) --no-print-directory lint
	@echo "==> make test"
	@$(MAKE) --no-print-directory test
	@echo "==> make build"
	@$(MAKE) --no-print-directory build
	@echo "✓ Full checks complete"

# Pre-push validation (replicates GitHub Actions checks)
pre-push:
	@echo "Running pre-push validation..."
	@./scripts/pre-push-check.sh

# Install git pre-push hook
install-hooks:
	@echo "Installing git hooks..."
	@if [ ! -f .git/hooks/pre-push ]; then \
		echo "#!/bin/bash" > .git/hooks/pre-push; \
		echo "exec ./scripts/pre-push-check.sh" >> .git/hooks/pre-push; \
		chmod +x .git/hooks/pre-push; \
		echo "✅ Pre-push hook installed"; \
		echo "   Hook will run automatically before each 'git push'"; \
		echo "   To skip: git push --no-verify"; \
	else \
		echo "⚠️  Pre-push hook already exists"; \
		echo "   Remove .git/hooks/pre-push and run 'make install-hooks' again"; \
	fi

# Uninstall git pre-push hook
uninstall-hooks:
	@echo "Removing git hooks..."
	@rm -f .git/hooks/pre-push
	@echo "✅ Pre-push hook removed"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/ dist/
	rm -f coverage.out coverage.html
	go clean -cache

# Install to local GOPATH/bin
install: build
	@echo "Installing ${BINARY_NAME}..."
	cp bin/${BINARY_NAME} ${GOPATH:-$(go env GOPATH)}/bin/

# Release build
release: test lint build-release
	@echo "Release build completed"
	@echo "Version: $(shell cat VERSION)"
	@ls -la dist/

# Development setup
dev-setup: deps
	@echo "Setting up development environment..."
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Run with example config
run-example: build
	@echo "Creating example config..."
	./bin/${BINARY_NAME} create-config
	@echo "Running with example config..."
	./bin/${BINARY_NAME} list-servers

# Sync example config files
sync-config:
	@echo "Syncing example config files..."
	cp mcp_servers.example.json internal/config/mcp_servers.example.json
	@echo "Example config synced"

# Check if example config files are in sync
check-config:
	@echo "Checking if example config files are in sync..."
	@diff -q mcp_servers.example.json internal/config/mcp_servers.example.json > /dev/null && echo "✓ Config files in sync" || (echo "✗ Config files out of sync! Run 'make sync-config'" && exit 1)

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build for current platform (dev version)"
	@echo "  sign           - Sign local binary (macOS only)"
	@echo "  build-signed   - Build and sign local binary (macOS)"
	@echo "  build-all      - Build for all platforms (dev version)"
	@echo "  build-release  - Build for all platforms (release version from VERSION file)"
	@echo "  release-sign   - Sign macOS release binaries (optional)"
	@echo "  notarize-release - Notarize release binaries (optional)"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  test-mcp-servers - Test all MCP servers and CLI commands"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  lint           - Lint code"
	@echo "  check          - Run full checks (fmt, vet, lint, test, build)"
	@echo "  pre-push       - Run pre-push validation (replicates GitHub Actions)"
	@echo "  install-hooks  - Install git pre-push hook"
	@echo "  uninstall-hooks- Remove git pre-push hook"
	@echo "  deps           - Download dependencies"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  release        - Release build (test + lint + build-release)"
	@echo "  set-version    - Set version: make set-version VERSION=1.2.3"
	@echo "  dev-setup      - Set up development environment"
	@echo "  run-example    - Build and run with example config"
	@echo "  sync-config    - Sync root example config to internal/config"
	@echo "  check-config   - Check if example configs are in sync"
