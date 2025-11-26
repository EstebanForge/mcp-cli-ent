# Development Guide

## Pre-Push Validation

To ensure code quality and catch issues before pushing to the repository, we provide a comprehensive pre-push validation script that replicates all GitHub Actions CI/CD checks locally.

### What It Checks

The pre-push validation script (`scripts/pre-push-check.sh`) performs the following checks:

#### 1. Environment Validation
- ✅ Go installation and version
- ✅ golangci-lint installation
- ✅ Git repository status

#### 2. Dependency Validation
- ✅ Download dependencies (`go mod download`)
- ✅ Verify dependencies for tampering (`go mod verify`)

#### 3. Configuration Validation
- ✅ Check if example config files are in sync

#### 4. Code Quality
- ✅ Go code formatting (`go fmt`)
- ✅ Linting with golangci-lint (timeout: 5m)

#### 5. Tests
- ✅ Run full test suite (`go test ./...`)

#### 6. Build Validation
- ✅ Build for current platform
- ✅ Test binary execution
- ✅ Build for all platforms (Linux, macOS, Windows × amd64, arm64)

#### 7. Binary Validation
- ✅ Verify all 6 binaries exist
- ✅ Check minimum size requirements (10MB+)
- ✅ Verify binary formats (ELF, Mach-O, PE)

#### 8. Git Status
- ⚠️  Warn about uncommitted changes
- ✅ Display current branch

### Usage

#### Manual Execution

Run the validation manually before pushing:

```bash
# Using make target (recommended)
make pre-push

# Or run script directly
./scripts/pre-push-check.sh
```

#### Automatic Execution (Git Hook)

Install the git pre-push hook to run validation automatically before each push:

```bash
# Install the hook
make install-hooks
```

The hook will:
- Run automatically before every `git push`
- Block the push if validation fails
- Can be skipped with `git push --no-verify` (not recommended)

**Uninstall the hook:**

```bash
make uninstall-hooks
```

### Example Output

```
🚦 MCP CLI-ENT Pre-Push Validation
===================================

This script replicates all GitHub Actions checks locally
to catch issues before pushing to remote.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
1️⃣  Environment Check
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

▶ Checking Go installation...
✅ Go installed: go1.22.0
▶ Checking golangci-lint installation...
✅ golangci-lint installed: v1.55.2
▶ Checking git repository...
✅ Git repository exists

[... more checks ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ✅ Passed:   24
  ⚠️  Warnings: 2
  ❌ Failed:   0

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Pre-push validation PASSED!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Your code is ready to push. GitHub Actions should pass.
```

### Benefits

1. **Catch Issues Early**: Find problems before pushing, not after
2. **Faster Feedback**: Get immediate results instead of waiting for CI/CD
3. **Confidence**: Push with confidence knowing CI/CD will likely pass
4. **Save Time**: Avoid push-fix-push cycles
5. **Matches CI/CD**: Identical checks to GitHub Actions workflows

### When Validation Fails

If the pre-push validation fails:

1. **Review the output** to see which checks failed
2. **Fix the issues** indicated by the error messages
3. **Re-run validation** with `make pre-push`
4. **Push when all checks pass**

Common fixes:
```bash
# Fix formatting issues
make fmt

# Fix linting issues
golangci-lint run --fix

# Update dependencies
go mod tidy

# Sync config files
make sync-config
```

### Skipping Validation

**Not recommended**, but you can skip validation:

```bash
# Skip manual check
git push --no-verify

# Disable hook temporarily
make uninstall-hooks
```

⚠️ **Warning**: Skipping validation may cause GitHub Actions to fail, requiring additional push-fix-push cycles.

### Integration with GitHub Actions

This script replicates the exact checks performed by our GitHub Actions workflows:

- `.github/workflows/build.yml` - Build and test on every push
- `.github/workflows/release.yml` - Release validation on tags

By running these checks locally, you ensure your code will pass CI/CD before pushing.

### Troubleshooting

#### golangci-lint not installed

```bash
# macOS
brew install golangci-lint

# Linux/Windows/WSL
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify installation
golangci-lint --version
```

#### Binary size validation fails

This usually indicates a build issue. Try:

```bash
make clean
make build-all
```

#### Permission denied error

Make sure the script is executable:

```bash
chmod +x scripts/pre-push-check.sh
```

## Development Workflow

Recommended workflow for contributing:

1. **Create a branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make changes**
   - Write code
   - Run tests: `make test`
   - Format code: `make fmt`

3. **Validate locally**
   ```bash
   make pre-push
   ```

4. **Commit and push**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   git push origin feature/my-feature
   ```

5. **Create pull request**
   - GitHub Actions will run the same checks
   - If local validation passed, CI/CD should pass too

## Additional Resources

- [GitHub Actions Workflows](../.github/workflows/)
- [Makefile Targets](../Makefile)
- [Contributing Guidelines](../CONTRIBUTING.md)
