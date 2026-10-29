default:
    @just --list

# Build development binary to bin/mcp-cli-ent
build:
    make build

# Build optimized release binary
build-release:
    make build-release

# Format Go code
fmt:
    make fmt

# Run go vet
vet:
    make vet

# Run unit tests
test:
    make test

# Run full checks (config, fmt, vet, lint, test, build)
check:
    make check

# Local CI verification gate
ci: check
