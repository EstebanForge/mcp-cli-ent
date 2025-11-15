.PHONY: build test clean install release fmt lint deps

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
build:
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
	@echo "  deps           - Download dependencies"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  release        - Release build (test + lint + build-release)"
	@echo "  set-version    - Set version: make set-version VERSION=1.2.3"
	@echo "  dev-setup      - Set up development environment"
	@echo "  run-example    - Build and run with example config"