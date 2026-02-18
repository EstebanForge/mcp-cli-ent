# Project Overview

MCP CLI-Ent is a Go-based standalone CLI client for MCP (Model Context Protocol) servers. It enables interaction with MCP servers without loading them into Claude Code's context window, providing intelligent context management for AI agents.

**Key Characteristics:**
- **Language**: Go 1.21+
- **Purpose**: Standalone MCP server interaction with minimal context pollution and persistent daemon support
- **Architecture**: Transport-agnostic JSON-RPC 2.0 implementation with cross-platform daemon service
- **Compatibility**: Works with existing Claude Code/VSCode `mcp_servers.json` configurations
- **Distribution**: Single binary with zero runtime dependencies
- **Session Persistence**: Gemini CLI-like persistent browser sessions for automation workflows

## Repository Structure

```
mcp-cli-ent/
├── cmd/mcp-cli-ent/           # Main CLI entry point
│   └── main.go               # Application bootstrap
├── internal/
│   ├── client/               # MCP client transport implementations
│   │   ├── interface.go      # Client factory and interfaces
│   │   ├── http.go           # HTTP transport client
│   │   └── stdio.go          # Stdio transport client
│   ├── cli/                  # CLI command definitions
│   │   ├── root.go           # Root command and global flags
│   │   └── commands.go       # All subcommands
│   ├── config/               # Configuration management
│   │   ├── config.go         # Configuration loading and validation
│   │   └── types.go          # Configuration type definitions
│   ├── daemon/               # Persistent daemon service
│   │   ├── daemon.go         # Main daemon implementation
│   │   ├── types.go          # Daemon-specific types
│   │   ├── manager.go        # Cross-platform daemon lifecycle
│   │   ├── client.go         # Smart CLI-daemon bridge
│   │   ├── server.go         # HTTP API handlers
│   │   ├── endpoint.go       # Cross-platform endpoint management
│   │   └── platform.go       # Platform-specific process management
│   └── mcp/                  # MCP protocol implementation
│       ├── protocol.go       # Client interface and validation
│       └── types.go          # JSON-RPC 2.0 protocol types
├── pkg/version/              # Version information
│   └── version.go            # Build-time version variables
├── scripts/                  # Installation and utility scripts
│   ├── install.sh            # Unix/Linux/macOS installer
│   └── test-*.sh             # Installer testing scripts
├── .github/workflows/        # CI/CD pipelines
│   ├── build.yml             # Test and build workflow
│   └── release.yml           # Release automation
├── go.mod                    # Go module definition
├── go.sum                    # Go dependency checksums
├── Makefile                  # Build automation
├── VERSION                   # Release version file
├── mcp_servers.example.json  # Example MCP server configuration
├── README.md                 # Project documentation
├── CHANGELOG.md              # Version history
└── LICENSE                   # MIT License
```

## Development Environment Setup

### Prerequisites
- Go 1.21 or later
- Git
- Make (optional, for build automation)

### Initial Setup
```bash
# Clone repository
git clone https://github.com/EstebanForge/mcp-cli-ent.git
cd mcp-cli-ent

# Install development tools
make dev-setup

# Download dependencies
make deps

# Build for current platform
make build

# Run tests
make test
```

## Build System

### Make Targets

**Development:**
```bash
make build          # Build for current platform (dev version)
make build-all      # Build for all platforms (dev version)
make test           # Run unit tests
make test-coverage  # Run tests with coverage report
make fmt            # Format code (go fmt + goimports)
make lint           # Run golangci-lint
make clean          # Clean build artifacts
make deps           # Download and tidy dependencies
```

**Release:**
```bash
make build-release  # Build for all platforms (release version)
make release        # Full release build (test + lint + build-release)
make set-version VERSION=1.2.3  # Set release version
```

**Utility:**
```bash
make dev-setup      # Install development tools
make run-example    # Build and test with example config
make install        # Install to GOPATH/bin
make help           # Show all available targets
```

### Version Management
- **Development Version**: Generated from `git describe --tags --always --dirty`
- **Release Version**: Read from `VERSION` file
- **Policy**: Do not edit `VERSION` manually; release workflow sets it from the tag.
- **Build Metadata**: Commit hash, build date, and Go version embedded at build time

### CI/CD Pipelines

**Build Workflow** (`.github/workflows/build.yml`):
- Triggers: Push to `main`/`develop`, PR to `main`
- Jobs: Test, Lint, Build (matrix of platforms)
- Coverage: Upload to Codecov
- Artifacts: Build binaries retained for 30 days

**Release Workflow** (`.github/workflows/release.yml`):
- Triggers: Git tags matching `v*`
- Actions: Build all platforms, generate changelog, create GitHub release
- Assets: Platform-specific archives with checksums
- Excludes: Windows ARM64 (not supported)

## Core Architecture

### MCP Protocol Implementation

**Location**: `internal/mcp/`

**Key Components:**
- **JSON-RPC 2.0**: Full protocol implementation with typed messages
- **Transport Agnostic**: Separate transport layers (HTTP/stdio)
- **Type Safety**: Strongly typed protocol messages and parameters
- **Error Handling**: Standardized JSON-RPC error codes and messages

**Protocol Types** (`types.go`):
```go
type JSONRPCRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      interface{} `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
}

type Tool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}
```

**Client Interface** (`protocol.go`):
```go
type MCPClient interface {
    ListTools(ctx context.Context) ([]Tool, error)
    CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)
    ListResources(ctx context.Context) ([]Resource, error)
    Close() error
}
```

### Daemon Architecture

**Location**: `internal/daemon/`

**Key Components:**
- **Background Service**: Cross-platform daemon process for persistent MCP connections
- **HTTP API Server**: RESTful interface for daemon communication
- **Session Management**: Lifecycle management for browser automation sessions
- **Smart Client Bridge**: Automatic daemon usage with graceful fallback

**Daemon Types** (`types.go`):
```go
type Daemon struct {
    httpServer   *http.Server
    sessions     map[string]*PersistentSession
    sessionMutex sync.RWMutex
    config       *DaemonConfig
    clientFactory func(config.ServerConfig) (mcp.MCPClient, error)
    startTime     time.Time
    pid           int
    platform      string
    endpoint      string
}

type PersistentSession struct {
    ServerName    string                    `json:"serverName"`
    Client        mcp.MCPClient             `json:"-"`
    Status        SessionStatus             `json:"status"`
    Config        config.ServerConfig       `json:"config"`
    LastUsed      time.Time                 `json:"lastUsed"`
    StartTime     time.Time                 `json:"startTime"`
    Error         string                    `json:"error,omitempty"`
    ToolCache     map[string][]mcp.Tool     `json:"-"`
    PID           int                       `json:"pid,omitempty"`
}
```

**Smart Client** (`client.go`):
```go
type SmartClient struct {
    daemonClient *DaemonClient
    directClient  func(config.ServerConfig) (mcp.MCPClient, error)
}

func (sc *SmartClient) ShouldUseDaemon(serverName string, serverConfig config.ServerConfig) bool
func (sc *SmartClient) CreateClient(serverName string, serverConfig config.ServerConfig) (mcp.MCPClient, error)
```

### Transport Layer

**HTTP Client** (`internal/client/http.go`):
- Supports HTTP-based MCP servers
- Configurable timeouts and custom headers
- Environment variable substitution in headers
- Proper context cancellation

**Stdio Client** (`internal/client/stdio.go`):
- Supports command-line MCP servers
- Process management with context cancellation
- Environment variable injection
- Graceful shutdown handling

**Client Factory** (`internal/client/interface.go`):
```go
func NewMCPClient(serverConfig config.ServerConfig) (mcp.MCPClient, error)
```

### Configuration Management

**Configuration Format** (`mcp_servers.json`):
```json
{
  "mcpServers": {
    "chrome-devtools": {
      "description": "Browser automation: console, navigation, screenshots",
      "command": "npx",
      "args": ["-y", "chrome-devtools-mcp@latest", "--isolated"],
      "persistent": true,
      "timeout": 60
    },
    "playwright": {
      "description": "Advanced browser automation: elements, snapshots, interactions",
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"],
      "persistent": true,
      "timeout": 60
    },
    "context7": {
      "description": "Code library docs and snippets",
      "type": "http",
      "url": "https://mcp.context7.com/mcp",
      "headers": {
        "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
      },
      "persistent": false,
      "timeout": 30
    },
    "sequential-thinking": {
      "description": "Problem-solving and planning",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"],
      "persistent": false,
      "timeout": 30
    }
  }
}
```

**Configuration Discovery Priority**:
1. Custom path specified with `--config` flag
2. Platform-specific config directory:
   - **Linux/macOS**: `~/.config/mcp-cli-ent/mcp_servers.json`
   - **Windows**: `%USERPROFILE%\AppData\Roaming\mcp-cli-ent\mcp_servers.json`
3. Current directory `mcp_servers.json` (backward compatibility)

**Features**:
- Environment variable substitution (`${VAR_NAME}` and `$VAR_NAME`)
- Server enable/disable functionality
- **Description Field**: Optional server descriptions displayed in `list-tools` for LLM context efficiency
- Configuration validation with helpful error messages
- Automatic first-run configuration initialization
- **Persistent Flag Support**: Simple boolean to enable daemon-managed sessions
- **Chrome Isolation**: Automatic `--isolated` flag for browser profile conflict prevention
- **Smart Defaults**: Browser automation servers configured for persistence by default

## CLI Interface

**Framework**: Cobra CLI with Viper configuration

**Global Flags**:
- `--config`: Custom configuration file path
- `--verbose, -v`: Verbose output for debugging
- `--timeout`: Request timeout in seconds (default: 30)

**Commands**:

### `list-servers`
List all configured MCP servers with status
```bash
mcp-cli-ent list-servers
```

### `list-tools [server-name]`
List tools from MCP servers
```bash
# List tools from all enabled servers
mcp-cli-ent list-tools

# List tools from specific server
mcp-cli-ent list-tools context7
```

### `call-tool <server-name> <tool-name> [arguments]`
Execute a specific tool with JSON arguments
```bash
mcp-cli-ent call-tool context7 resolve-library-id '{"libraryName": "react"}'
```

### `create-config [filename]`
Create example configuration file
```bash
# Create in standard location
mcp-cli-ent create-config

# Create in current directory
mcp-cli-ent create-config ./my-config.json
```

### `daemon start`
Start the background daemon service
```bash
mcp-cli-ent daemon start
```

### `daemon stop`
Stop the running daemon
```bash
mcp-cli-ent daemon stop
```

### `daemon status`
Display daemon status and active sessions
```bash
mcp-cli-ent daemon status
```

### `daemon logs`
View daemon service logs
```bash
mcp-cli-ent daemon logs
```

### `version`
Display version and build information
```bash
mcp-cli-ent version
```

## Installation Methods

### Homebrew (macOS & Linux)
```bash
brew install EstebanForge/tap/mcp-cli-ent
```

### One-line Installer (Unix/macOS/WSL)
```bash
curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh | bash
```

### PowerShell Installer (Windows)
```powershell
iwr -useb https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.ps1 | iex
```

### Manual Installation
1. Download pre-built binary from [Releases](https://github.com/EstebanForge/mcp-cli-ent/releases)
2. Extract and move to PATH directory
3. Run `mcp-cli-ent create-config` to initialize configuration

### Source Installation
```bash
git clone https://github.com/EstebanForge/mcp-cli-ent.git
cd mcp-cli-ent
make build
sudo cp bin/mcp-cli-ent /usr/local/bin/
```

## Known MCP Servers

**Tested with:**
- **Chrome DevTools**: Browser automation with console access, navigation, and screenshots (`chrome-devtools-mcp@latest`)
- **Playwright**: Advanced browser automation with element interaction and page snapshots (`@playwright/mcp@latest`)
- **Context7**: Library documentation and code snippets (`https://mcp.context7.com/mcp`)
- **Sequential Thinking**: Problem-solving and planning tool (`@modelcontextprotocol/server-sequential-thinking`)
- **DeepWiki**: GitHub repository documentation (`mcp-remote https://mcp.deepwiki.com/sse`)

**Server Configuration Examples**:
```json
{
  "chrome-devtools": {
    "command": "npx",
    "args": ["-y", "chrome-devtools-mcp@latest", "--isolated"],
    "persistent": true,
    "timeout": 60
  },
  "playwright": {
    "command": "npx",
    "args": ["-y", "@playwright/mcp@latest"],
    "persistent": true,
    "timeout": 60
  },
  "context7": {
    "type": "http",
    "url": "https://mcp.context7.com/mcp",
    "headers": {
      "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
    },
    "persistent": false,
    "timeout": 30
  }
}
```

## Code Style and Standards

### Go Standards
- Follow Go standard formatting (`go fmt`)
- Use `goimports` for import organization
- Run `golangci-lint` for comprehensive linting
- Maintain public API documentation with godoc comments

### Commit Messages
- Use conventional commit format: `type(scope): description`
- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
- Examples: `feat(cli): add resource listing command`, `fix(http): handle timeout errors`

### Testing
- No test files currently exist in the codebase
- CI/CD pipeline expects `go test -v ./...` to pass
- Coverage reports generated with `make test-coverage`

## Troubleshooting

### Common Issues

**Build Failures**:
```bash
# Check Go version
go version

# Clean and rebuild
make clean && make build

# Update dependencies
make deps
```

**Configuration Issues**:
```bash
# Create default config
mcp-cli-ent create-config

# Check config location
mcp-cli-ent --verbose list-servers

# Validate config syntax
cat ~/.config/mcp-cli-ent/mcp_servers.json | jq .
```

**Runtime Issues**:
```bash
# Enable verbose output
mcp-cli-ent --verbose list-tools

# Check server connectivity
curl -H "CONTEXT7_API_KEY: $CONTEXT7_API_KEY" https://mcp.context7.com/mcp

# Test specific server
mcp-cli-ent list-tools context7
```

### Debug Commands

**Version Information**:
```bash
mcp-cli-ent version
# Output: mcp-cli-ent 0.1.0 (commit: abc1234, built: 2025-11-15T20:00:00Z, go: go1.21.0)
```

**Configuration Debugging**:
```bash
# Show config file location
mcp-cli-ent --verbose list-servers

# Test with custom config
mcp-cli-ent --config ./test.json list-servers
```

## Security Considerations

- **No Hardcoded Credentials**: All authentication via environment variables
- **Input Validation**: Comprehensive validation of tool arguments
- **Timeout Protection**: Configurable timeouts prevent hanging operations
- **Process Isolation**: Stdio servers run in isolated processes
- **Error Handling**: No sensitive information leaked in error messages
- **Daemon Security**: HTTP API bound to localhost with proper process isolation
- **Session Isolation**: Separate browser instances prevent cross-contamination
- **Resource Cleanup**: Automatic cleanup of browser processes and network connections

## Development Guidelines

### When Adding Features
1. Update relevant CLI commands in `internal/cli/commands.go`
2. Add configuration options if needed in `internal/config/types.go`
3. Implement protocol changes in `internal/mcp/types.go`
4. Update client interfaces if transport changes needed
5. Add daemon functionality in `internal/daemon/` if persistent sessions needed
6. Add build targets to Makefile if required
7. Update documentation (README, CHANGELOG)

### When Fixing Bugs
1. Write regression tests if applicable
2. Update error messages for clarity
3. Consider edge cases and error conditions
4. Update CHANGELOG with fix description

### When Releasing
1. Update VERSION file: `make set-version VERSION=1.2.3`
2. Update CHANGELOG.md with new features and fixes
3. Run full test suite: `make release`
4. Create git tag: `git tag v1.2.3`
5. Push tag to trigger release workflow

## Project Philosophy

This tool follows the principle of **intelligent context management** for AI agents. Rather than dumping raw MCP server responses into the context window, it provides:

- **Filtered Interaction**: Execute MCP tools and receive processed results
- **Context Preservation**: Maintain focus on high-signal information
- **Persistent Workflows**: Gemini CLI-like session persistence for browser automation
- **Deliberate Design**: Each interaction is purposeful and minimal
- **Professional Implementation**: Enterprise-grade build system and distribution

The name "CLI-Ent" reflects this philosophy: a CLI tool that acts as a wise, deliberate guardian of the agent's context environment, now with enhanced capabilities for persistent browser automation workflows.
