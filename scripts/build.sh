#!/bin/bash
set -e

# Build script for MCP CLI
# Usage: ./scripts/build.sh [target]

BINARY_NAME=mcp-cli-ent
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-ldflags \"-X github.com/mcp-cli-ent/mcp-cli/pkg/version.Version=${VERSION} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Commit=${COMMIT} -X github.com/mcp-cli-ent/mcp-cli/pkg/version.Date=${DATE}\""

TARGET=${1:-"local"}

echo "Building ${BINARY_NAME} version ${VERSION} (commit ${COMMIT})"

case $TARGET in
    "local")
        echo "Building for local platform..."
        mkdir -p bin
        eval "go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/${BINARY_NAME}"
        echo "Built: bin/${BINARY_NAME}"
        ;;
    "all")
        echo "Building for all platforms..."
        mkdir -p dist

        # Linux AMD64
        echo "Building for Linux AMD64..."
        GOOS=linux GOARCH=amd64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/${BINARY_NAME}"

        # Linux ARM64
        echo "Building for Linux ARM64..."
        GOOS=linux GOARCH=arm64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/${BINARY_NAME}"

        # macOS AMD64
        echo "Building for macOS AMD64..."
        GOOS=darwin GOARCH=amd64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/${BINARY_NAME}"

        # macOS ARM64
        echo "Building for macOS ARM64..."
        GOOS=darwin GOARCH=arm64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/${BINARY_NAME}"

        # Windows AMD64
        echo "Building for Windows AMD64..."
        GOOS=windows GOARCH=amd64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/${BINARY_NAME}"

        # Windows ARM64
        echo "Building for Windows ARM64..."
        GOOS=windows GOARCH=arm64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-arm64.exe ./cmd/${BINARY_NAME}"

        echo "Build artifacts created in dist/"
        ls -la dist/
        ;;
    "linux-amd64")
        echo "Building for Linux AMD64..."
        mkdir -p dist
        GOOS=linux GOARCH=amd64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/${BINARY_NAME}"
        echo "Built: dist/${BINARY_NAME}-linux-amd64"
        ;;
    "darwin-amd64")
        echo "Building for macOS AMD64..."
        mkdir -p dist
        GOOS=darwin GOARCH=amd64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/${BINARY_NAME}"
        echo "Built: dist/${BINARY_NAME}-darwin-amd64"
        ;;
    "darwin-arm64")
        echo "Building for macOS ARM64..."
        mkdir -p dist
        GOOS=darwin GOARCH=arm64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/${BINARY_NAME}"
        echo "Built: dist/${BINARY_NAME}-darwin-arm64"
        ;;
    "windows-amd64")
        echo "Building for Windows AMD64..."
        mkdir -p dist
        GOOS=windows GOARCH=amd64 eval "go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/${BINARY_NAME}"
        echo "Built: dist/${BINARY_NAME}-windows-amd64.exe"
        ;;
    *)
        echo "Unknown target: $TARGET"
        echo "Usage: $0 [local|all|linux-amd64|darwin-amd64|darwin-arm64|windows-amd64]"
        exit 1
        ;;
esac

echo "Build completed successfully!"