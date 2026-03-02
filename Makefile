.PHONY: all build build-release sign build-signed release-sign notarize-release \
	build-all build-linux build-linux-arm build-darwin build-darwin-arm build-windows build-windows-arm \
	release-all release-linux release-linux-arm release-darwin release-darwin-arm release-windows release-windows-arm \
	test test-coverage test-mcp-servers clean install release set-version \
	fmt vet lint lint-ci lint-fix deps sync-config check-config check ci \
	dev-setup run-example pre-push install-hooks uninstall-hooks dev-check run-dev release-preflight help

# Default target
.DEFAULT_GOAL := help

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

# Lint
GOLANGCI_LINT_BIN ?= golangci-lint
GOLANGCI_LINT_VERSION ?= 2.10.1
LINT_TIMEOUT ?= 5m

help: ## Show this help message
	@echo "MCP CLI-Ent - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-28s %s\n", $$1, $$2}'

build: ## Build for current platform (dev version)
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p $(dir ${BINARY_PATH})
	go build -o ${BINARY_PATH} ./cmd/mcp-cli-ent

build-release: ## Build optimized release binary for current platform
	@echo "Building release binary..."
	@mkdir -p $(dir ${BINARY_PATH})
	CGO_ENABLED=0 go build ${RELEASE_LDFLAGS} -trimpath -o ${BINARY_PATH} ./cmd/mcp-cli-ent
	@echo "✓ Release binary built: ${BINARY_PATH}"

sign: build ## Sign local binary (macOS only)
	@echo "Signing ${BINARY_PATH} (macOS only)..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign -s "$(SIGN_IDENTITY)" -f "${BINARY_PATH}"; \
		echo "✓ Signed: ${BINARY_PATH}"; \
	else \
		echo "ℹ️  Skipping sign (non-macOS)"; \
	fi

build-signed: build sign ## Build and sign local binary (macOS)

release-sign: ## Sign macOS release binaries in dist/ (optional; set RELEASE_SIGN=1)
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

notarize-release: ## Notarize release artifacts (optional; set RELEASE_NOTARIZE=1)
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

## Dev builds (git-describe version) ─────────────────────────────────────────

build-all: build-linux build-linux-arm build-darwin build-darwin-arm build-windows build-windows-arm ## Build for all platforms (dev version)
	@echo "✓ Build artifacts created in dist/"

build-linux: ## Build Linux AMD64 binary (dev version)
	@echo "Building for Linux AMD64..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-linux-amd64"

build-linux-arm: ## Build Linux ARM64 binary (dev version)
	@echo "Building for Linux ARM64..."
	@mkdir -p dist
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-linux-arm64"

build-darwin: ## Build macOS AMD64 binary (dev version)
	@echo "Building for macOS AMD64..."
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-darwin-amd64"

build-darwin-arm: ## Build macOS ARM64 binary (dev version)
	@echo "Building for macOS ARM64..."
	@mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-darwin-arm64"

build-windows: ## Build Windows AMD64 binary (dev version)
	@echo "Building for Windows AMD64..."
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-windows-amd64.exe"

build-windows-arm: ## Build Windows ARM64 binary (dev version)
	@echo "Building for Windows ARM64..."
	@mkdir -p dist
	GOOS=windows GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-arm64.exe ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-windows-arm64.exe"

## Release builds (release-all + per-platform, VERSION file version) ─────────

release-all: release-linux release-linux-arm release-darwin release-darwin-arm release-windows release-windows-arm ## Build for all platforms (release version from VERSION file)
	@echo "✓ Release artifacts created in dist/"
	@echo "Version: ${RELEASE_VERSION}"

release-linux: ## Build Linux AMD64 release binary
	@echo "Building release for Linux AMD64..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-linux-amd64"

release-linux-arm: ## Build Linux ARM64 release binary
	@echo "Building release for Linux ARM64..."
	@mkdir -p dist
	GOOS=linux GOARCH=arm64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-linux-arm64"

release-darwin: ## Build macOS AMD64 release binary
	@echo "Building release for macOS AMD64..."
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-darwin-amd64"

release-darwin-arm: ## Build macOS ARM64 release binary
	@echo "Building release for macOS ARM64..."
	@mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-darwin-arm64"

release-windows: ## Build Windows AMD64 release binary
	@echo "Building release for Windows AMD64..."
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-windows-amd64.exe"

release-windows-arm: ## Build Windows ARM64 release binary
	@echo "Building release for Windows ARM64..."
	@mkdir -p dist
	GOOS=windows GOARCH=arm64 go build ${RELEASE_LDFLAGS} -o dist/${BINARY_NAME}-windows-arm64.exe ./cmd/${BINARY_NAME}
	@echo "✓ Built: dist/${BINARY_NAME}-windows-arm64.exe"

set-version: ## Set version (usage: make set-version VERSION=1.2.3)
	@echo "Setting version to $(VERSION)"
	@echo "$(VERSION)" > VERSION

## Tests & Quality ────────────────────────────────────────────────────────────

test: ## Run tests
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

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

test-mcp-servers: ## Test all MCP servers and CLI commands
	@echo "Testing all MCP servers and CLI commands..."
	@./scripts/test-mcp-servers.sh

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not found; skipping import formatting"; \
		echo "Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi
	@echo "✓ Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...
	@echo "✓ Vet complete"

lint: ## Run linter
	@echo "Running linter..."
	@which $(GOLANGCI_LINT_BIN) > /dev/null || (echo "$(GOLANGCI_LINT_BIN) not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@actual_version="$$($(GOLANGCI_LINT_BIN) version | awk '{print $$4}' | sed 's/^v//')"; \
	if [ "$$actual_version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint version mismatch: required $(GOLANGCI_LINT_VERSION), found $$actual_version"; \
		exit 1; \
	fi
	@echo "Using golangci-lint $(GOLANGCI_LINT_VERSION)"
	$(GOLANGCI_LINT_BIN) run --timeout=$(LINT_TIMEOUT)

lint-ci: ## Run linter in CI parity mode (clears cache first)
	@echo "Running CI-parity linter..."
	$(GOLANGCI_LINT_BIN) cache clean
	@$(MAKE) --no-print-directory lint

lint-fix: ## Run linter with auto-fix
	@echo "Running linter with auto-fix..."
	@which $(GOLANGCI_LINT_BIN) > /dev/null || (echo "$(GOLANGCI_LINT_BIN) not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@actual_version="$$($(GOLANGCI_LINT_BIN) version | awk '{print $$4}' | sed 's/^v//')"; \
	if [ "$$actual_version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint version mismatch: required $(GOLANGCI_LINT_VERSION), found $$actual_version"; \
		exit 1; \
	fi
	@echo "Using golangci-lint $(GOLANGCI_LINT_VERSION)"
	$(GOLANGCI_LINT_BIN) run --timeout=$(LINT_TIMEOUT) --fix

check: ## Run full checks (fmt, vet, lint, test, build)
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

ci: ## Run CI checks (full pipeline)
	@$(MAKE) --no-print-directory check
	@echo "✓ CI checks passed"

dev-check: fmt vet test ## Quick development check (fmt, vet, test)
	@echo "✓ Development checks passed"

run-dev: ## Run with hot reload (requires air)
	@which air > /dev/null || (echo "Air not installed. Install: go install github.com/cosmtrek/air@latest" && exit 1)
	@echo "Starting hot reload..."
	air

## Git & Hooks ────────────────────────────────────────────────────────────────

pre-push: ## Run pre-push validation (replicates GitHub Actions checks)
	@echo "Running pre-push validation..."
	@./scripts/pre-push-check.sh

install-hooks: ## Install git pre-push hook
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

uninstall-hooks: ## Remove git pre-push hook
	@echo "Removing git hooks..."
	@rm -f .git/hooks/pre-push
	@echo "✅ Pre-push hook removed"

## Config ─────────────────────────────────────────────────────────────────────

sync-config: ## Sync root example config to internal/config
	@echo "Syncing example config files..."
	cp mcp_servers.example.json internal/config/mcp_servers.example.json
	@echo "✓ Example config synced"

check-config: ## Check if example config files are in sync
	@echo "Checking if example config files are in sync..."
	@diff -q mcp_servers.example.json internal/config/mcp_servers.example.json > /dev/null && echo "✓ Config files in sync" || (echo "✗ Config files out of sync! Run 'make sync-config'" && exit 1)

## Misc ───────────────────────────────────────────────────────────────────────

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/ dist/
	rm -f coverage.out coverage.html
	go clean -cache

install: build ## Install to GOPATH/bin
	@echo "Installing ${BINARY_NAME}..."
	cp bin/${BINARY_NAME} ${GOPATH:-$(go env GOPATH)}/bin/

release: test lint release-all ## Release build (test + lint + all platform binaries)
	@echo "✓ Release build completed"
	@echo "Version: $(shell cat VERSION)"
	@ls -la dist/

dev-setup: deps ## Set up development environment
	@echo "Setting up development environment..."
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

run-example: build ## Build and run with example config
	@echo "Creating example config..."
	./bin/${BINARY_NAME} create-config
	@echo "Running with example config..."
	./bin/${BINARY_NAME} list-servers

release-preflight: ## Run release checks against a local tag (usage: make release-preflight TAG=1.2.3)
	@[ -n "$(TAG)" ] || (echo "TAG is required (example: make release-preflight TAG=1.2.3)" && exit 1)
	@git rev-parse --verify --quiet "refs/tags/$(TAG)" >/dev/null || (echo "Tag not found: $(TAG)" && exit 1)
	@git diff --quiet && git diff --cached --quiet || (echo "Working tree must be clean for release-preflight" && exit 1)
	@tmp_dir="$$(mktemp -d)"; \
	echo "Using temporary worktree: $$tmp_dir"; \
	trap 'git worktree remove --force "$$tmp_dir" >/dev/null 2>&1 || true' EXIT; \
	git worktree add --detach "$$tmp_dir" "refs/tags/$(TAG)" >/dev/null; \
	cd "$$tmp_dir"; \
	$(MAKE) --no-print-directory test; \
	$(MAKE) --no-print-directory lint-ci
