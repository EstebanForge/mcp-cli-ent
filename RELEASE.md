# Release Guide

This guide covers the complete process for creating and publishing MCP CLI-ENT releases.

## Prerequisites

### Repository Setup
- [ ] Repository is pushed to `https://github.com/EstebanForge/mcp-cli-ent`
- [ ] GitHub Actions are enabled
- [ ] Release workflow (`.github/workflows/release.yml`) is configured
- [ ] VERSION file contains the correct version number

### Release Requirements
- [ ] All binaries build successfully: `make build-release`
- [ ] Installer scripts validated: `./scripts/test-install.sh`
- [ ] Documentation is updated (README.md, CHANGELOG.md)
- [ ] Version is updated in CHANGELOG.md

## Release Process

### 1. Pre-Release Checklist

```bash
# 1. Update version if needed
make set-version VERSION=0.1.1

# 2. Update version references in all files:
#    - VERSION file (primary source of truth)
#    - CHANGELOG.md (version header)
#    - RELEASE.md (example commands and version references)
#    - Makefile (fallback default version)
#    - scripts/test-installer.sh (test version mock)
#    - Any other documentation with hardcoded version numbers

# 3. Update CHANGELOG.md
# Add new version entry with changes

# 4. Update README.md if needed
# Update installation URLs, documentation, etc.

# 5. Test build system
make clean && make build-release

# 6. Test installers
./scripts/test-install.sh

# 7. Verify binary functionality
./dist/mcp-cli-ent-$(go env GOOS)-$(go env GOARCH) --version
```

### 2. Commit and Tag

```bash
# 1. Stage all changes
git add .

# 2. Commit with semantic message
git commit -m "Release v0.1.0"

# 3. Create and push tag
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

### 3. Automated Release

**Option A: GitHub Actions (Recommended)**
- The release workflow will trigger automatically when a tag is pushed
- GitHub Actions will build all binaries and create a draft release
- Review and publish the draft release on GitHub

**Option B: Manual Release**
```bash
# 1. Build release artifacts
make release

# 2. Create release notes file
cat > release_notes.md << 'EOF'
## Features
- Full MCP protocol implementation
- Cross-platform binaries for Linux, macOS, and Windows
- Professional installer scripts
- Configuration management with Claude Code compatibility

## Installation
### Quick Install (Linux/macOS/WSL)
```bash
curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh | bash
```

### Windows PowerShell
```powershell
iwr -useb https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.ps1 | iex
```

Full installation instructions: https://github.com/EstebanForge/mcp-cli-ent#installation
EOF

# 3. Create GitHub release
gh release create v0.1.0 \
  --title "v0.1.0" \
  --notes-file release_notes.md \
  dist/*

# 3. Upload binaries
gh release upload v0.1.0 dist/*
```

## Post-Release Verification

### 1. Test Installation

```bash
# Test bash installer
curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh | bash

# Test PowerShell installer (on Windows)
iwr -useb https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.ps1 | iex

# Verify installation
mcp-cli-ent --version
mcp-cli-ent create-config
mcp-cli-ent list-servers
```

### 2. Test Cross-Platform

- [ ] Linux (Ubuntu/Debian) installation works
- [ ] macOS (Intel/Apple Silicon) installation works
- [ ] Windows (PowerShell) installation works
- [ ] WSL installation works
- [ ] PATH updates correctly on all platforms
- [ ] Configuration directory creation works

### 3. Update Documentation

- [ ] Update website if applicable
- [ ] Announce release on social media
- [ ] Update package managers (Homebrew, Chocolatey, Scoop) if applicable

## Release Structure

### GitHub Release Assets

The release should include these binaries:

```
mcp-cli-ent-linux-amd64         # Linux Intel/AMD
mcp-cli-ent-linux-arm64         # Linux ARM64
mcp-cli-ent-darwin-amd64        # macOS Intel
mcp-cli-ent-darwin-arm64        # macOS Apple Silicon
mcp-cli-ent-windows-amd64.exe   # Windows Intel/AMD
mcp-cli-ent-windows-arm64.exe   # Windows ARM64
```

### Version Information

Each binary includes embedded version information:

```bash
mcp-cli-ent --version
# Output: mcp-cli-ent version 0.1.0 (commit: abc123, built: 2025-11-15T23:00:00Z, go: go1.21.0)
```

## Troubleshooting

### Common Issues

**1. Build Failures**
```bash
# Clean and rebuild
make clean
go mod tidy
make build-release
```

**2. Installer Not Finding Binary**
- Verify GitHub release URL format
- Check that binary names match installer expectations
- Ensure release is published (not draft)

**3. Version Mismatch**
```bash
# Check VERSION file
cat VERSION

# Verify binary version
./dist/mcp-cli-ent-$(go env GOOS)-$(go env GOARCH) --version
```

**4. GitHub Actions Failures**
- Check that secrets are configured
- Verify workflows have proper permissions
- Review action logs for specific errors

### Debug Commands

```bash
# Check current git status
git status
git tag -l

# Verify remote URLs
git remote -v

# Test GitHub CLI
gh auth status

# Check release assets
gh release view v0.1.0
```

## Automation

### GitHub Actions Workflow

The release automation (`.github/workflows/release.yml`) handles:

1. **Trigger**: On tag push (`v*.*.*`)
2. **Build**: Cross-platform compilation
3. **Test**: Basic functionality tests
4. **Upload**: Create draft GitHub release with binaries
5. **Notify**: Status updates and completion

### Manual Override

If automation fails, use the manual process in Option B above.

## Rollback Plan

If issues are discovered after release:

1. **Unpublish** the GitHub release
2. **Delete** the tag: `git tag -d v0.1.0 && git push origin :v0.1.0`
3. **Fix** the issues
4. **Recreate** release with bumped version: `v0.1.1`

## Future Improvements

- [ ] Automated package manager updates (Homebrew, Chocolatey)
- [ ] Integration testing with real MCP servers
- [ ] Automated cross-platform installation testing
- [ ] Security scanning of release artifacts
- [ ] Checksum verification for downloads