.PHONY: build test clean install release fmt lint deps sync-config check-config

# Default target
all: build

# Variables
BINARY_NAME=mcp-cli-ent
RELEASE_VERSION=$(shell cat VERSION 2>/dev/null || echo "0.1.0")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo $(RELEASE_VERSION))
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X github.com/mcp-cli-ent/mcp-cli/pkg/version.Version=${VERSION} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Commit=${COMMIT} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Date=${DATE}"
RELEASE_LDFLAGS=-ldflags "-X github.com/mcp-cli-ent/mcp-cli/pkg/version.Version=${RELEASE_VERSION} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Commit=${COMMIT} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Date=${DATE}"

# Build for current platform
build: check-config
	@echo "Building ${BINARY_NAME}..."
	go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/mcp-cli-ent

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
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

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
	@echo "  build-all      - Build for all platforms (dev version)"
	@echo "  build-release  - Build for all platforms (release version from VERSION file)"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
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