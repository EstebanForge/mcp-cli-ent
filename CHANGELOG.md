# Changelog

## [0.3.0] - 2025-11-26

### ✨ Agent Experience Improvements

This release focuses on **optimizing the CLI for AI agent usage** with cleaner output, better context management, and enhanced MCP server ecosystem.

### 🚀 Major Enhancements

#### Agent-Optimized Interface
- **Concise Help Output** - 60% reduction in help text for minimal context usage
- **Smart Tool Discovery** - `list-tools` now shows server summary instead of dumping all tools
- **Verbose-Controlled Warnings** - Session warnings only shown in verbose mode to reduce context pollution
- **Server Descriptions** - `list-servers` now displays helpful descriptions for each MCP server

#### Enhanced MCP Server Management
- **Single Source of Truth** - `mcp_servers.example.json` embedded in binary for consistency
- **Environment Variable Standardization** - All API keys now use `ENT_` prefix to prevent conflicts
- **Automated Config Sync** - Build process ensures embedded config matches source of truth
- **Comprehensive Server Ecosystem** - Added Cipher memory layer, Brave Search, Time server

### 🔧 Configuration Improvements

#### Environment Variable Consistency
```bash
# New standardized ENT_ prefixed variables
export ENT_CONTEXT7_API_KEY="your_key"
export ENT_BRAVE_API_KEY="your_key"
export ENT_OPENAI_API_KEY="your_key"
export ENT_ANTHROPIC_API_KEY="your_key"
```

#### Updated Server Configurations
- **Cipher** - Memory layer for coding agents (auto-generate AI memories, IDE switching, team sharing)
- **Brave Search** - Web search with AI summaries
- **Time MCP Server** - Current time and timezone conversions
- **Context7** - Updated to npx-based configuration
- **Removed Deprecated** - GitHub and GitLab MCP servers (deprecated upstream)

#### Build System Enhancements
- **Config Validation** - `make check-config` ensures embedded config sync
- **CI/CD Integration** - GitHub Actions now use Makefile targets for consistency
- **Automated Sync** - `make sync-config` copies example config to embedded location

### 🧠 Memory Layer Integration

#### Cipher MCP Server
- **Dual Memory Architecture** - System 1 (facts) and System 2 (reasoning) memory
- **Team Collaboration** - Workspace memory sharing across development teams
- **Persistent Context** - IDE-agnostic memory that persists across sessions
- **Cross-Platform Compatibility** - Works with Cursor, Windsurf, Claude Desktop, etc.

### 🎯 Developer Experience

#### Agent-Friendly Output
```bash
# Before: Verbose help polluting context
mcp-cli-ent is a standalone CLI tool for interacting with MCP (Model Context Protocol) servers...

# After: Concise and purposeful
MCP CLI-Ent: Call MCP tools without loading them into agent context.
```

#### Enhanced Tool Discovery
```bash
# Smart warning when no server specified
⚠️  Warning: Listing all tools from all enabled servers can be slow.
Found 7 enabled MCP servers:

  • time - Current time and timezone conversions
  • cipher - Memory layer for coding agents: auto-generate AI memories, IDE switching, team sharing
  • deepwiki - Repository documentation from public Git repos

💡 Please specify a server name to see its tools:
  mcp-cli-ent list-tools <server-name>
```

### 🔒 Security Improvements

#### Namespace Isolation
- **ENT_ Prefix** - Prevents conflicts with existing environment variables
- **Clean Configuration** - No hardcoded credentials or API keys
- **Safe Defaults** - Disabled by default for servers requiring API keys

### 🛠️ Build and CI/CD

#### GitHub Actions Updates
- **Makefile Integration** - CI now uses `make build-release` and `make build`
- **Config Sync Validation** - Builds fail if config files are out of sync
- **Consistent Environment** - Same build process locally and in CI/CD

### 📚 Documentation Updates

#### Configuration Guides
- **API Keys Section** - Comprehensive guide for setting up API keys
- **Environment Variables** - Complete documentation of ENT_ prefixed variables
- **Server Descriptions** - Token-efficient descriptions optimized for LLM context
- **Migration Guide** - Instructions for updating environment variables

#### Enhanced Examples
- **Real-World Configurations** - All examples use ENT_ prefixed variables
- **Server Descriptions** - Each server includes concise, useful descriptions
- **Best Practices** - Guidelines for production deployment

### 🔧 Technical Improvements

#### Session Management
- **Verbose-Controlled Logging** - Session cleanup warnings only in verbose mode
- **Clean Output** - Reduced noise for agent interactions
- **Better Error Messages** - Clear, actionable feedback

#### Configuration Management
- **Embedded Configuration** - Single source of truth embedded in binary
- **Environment Substitution** - Support for `${VAR_NAME}` in args, headers, and env
- **Validation** - Automatic config validation and helpful error messages

### 🐛 Bug Fixes

#### Installer Improvements
- **Non-Interactive Upgrade** - Fixed installer to auto-upgrade when run via `curl | bash`
- **Interactive Detection** - Properly detects TTY and skips prompts in non-interactive mode
- **Version Comparison** - Improved version detection and upgrade logic

---

## [0.2.0] - 2025-11-17

### 🚀 Major Enhancement: Persistent Daemon Architecture

This release introduces a **complete persistent daemon architecture** that provides Gemini CLI-like session persistence for browser automation MCP servers, enabling seamless multi-command workflows.

### ✨ New Features

#### Persistent Browser Sessions
- **Daemon Background Service** - Cross-platform daemon process for managing persistent MCP connections
- **Automatic Session Persistence** - Browser state maintained across CLI command invocations
- **Zero Configuration Setup** - Works out of the box with existing `mcp_servers.json` configurations
- **Smart Client Bridge** - Automatic daemon usage with fallback to direct connections
- **Multi-MCP Server Support** - Simultaneous sessions for Chrome DevTools, Playwright, and other persistent servers

#### New Daemon CLI Commands
- `daemon start` - Start the background daemon service
- `daemon stop` - Stop the running daemon
- `daemon status` - Display daemon status and active sessions
- `daemon logs` - View daemon service logs

#### Enhanced Browser Automation
- **Chrome DevTools Integration** - Persistent sessions with console access, navigation, and screenshots
- **Playwright Integration** - Advanced browser automation with element interaction and page snapshots
- **Session Auto-Creation** - Browser sessions automatically created when tools are called
- **Cross-Platform Compatibility** - Windows, Linux, macOS, and WSL support

### 🔧 Configuration Enhancements

#### Simplified Server Configuration
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
  }
}
```

- **Persistent Flag** - Simple boolean to enable daemon-managed sessions
- **Chrome Isolation** - Automatic `--isolated` flag for browser profile conflict prevention
- **Smart Defaults** - Browser automation servers configured for persistence by default
- **HTTP API Communication** - RESTful daemon interface for reliable session management

### 🏗️ Architecture Improvements

#### Daemon System
- **Cross-Platform Process Management** - Native process lifecycle management for all platforms
- **HTTP API Server** - RESTful interface for daemon communication
- **Session Health Monitoring** - Automatic session cleanup and recovery
- **Resource Management** - Proper cleanup of browser processes and connections
- **PID File Management** - Daemon process tracking and lifecycle coordination

#### Smart Client Architecture
- **Automatic Daemon Detection** - Checks for running daemon before creating connections
- **Graceful Fallback** - Falls back to direct connections if daemon unavailable
- **Session Auto-Start** - Automatically starts daemon for persistent servers
- **Transport-Agnostic** - Works with both HTTP and stdio MCP servers

### 🛠️ Developer Experience

#### Enhanced Error Handling
- **Daemon Status Indicators** - Clear feedback on daemon connectivity
- **Session State Reporting** - Detailed session health and activity information
- **Graceful Degradation** - Continues working even if daemon unavailable
- **Comprehensive Logging** - Detailed daemon logs for troubleshooting

#### Improved CLI Interface
- **Verbose Mode** - Enhanced debugging with daemon communication details
- **Progress Indicators** - Real-time feedback on daemon operations
- **Clear Error Messages** - Actionable error messages with suggestions
- **Status Reporting** - Rich status information for all daemon-managed sessions

### 🔍 Testing and Validation

#### Browser Automation Workflows
- **Chrome DevTools Testing** - Complete workflow validation: navigate → console access → screenshots
- **Playwright Testing** - Advanced automation testing: navigation → interaction → screenshots
- **Session Persistence** - Verified browser state maintained across multiple CLI invocations
- **Cross-Platform Verification** - Daemon functionality validated on macOS, Linux, and Windows

#### Multi-Server Support
- **Simultaneous Sessions** - Multiple MCP servers running concurrently
- **Resource Isolation** - Separate browser instances for each MCP server
- **Memory Management** - Efficient resource usage with proper cleanup
- **Process Recovery** - Automatic recovery from browser process crashes

### 📚 Documentation Updates

#### Configuration Guide
- **Daemon Setup** - Complete guide for enabling persistent sessions
- **Browser Automation** - Updated examples for Chrome DevTools and Playwright
- **Troubleshooting** - Common issues and solutions for daemon usage
- **Migration Guide** - Instructions for upgrading from v0.1.0

#### Updated Examples
- **Persistent Workflows** - Real-world examples of multi-command browser automation
- **Daemon Management** - Complete lifecycle management examples
- **Chrome DevTools Console** - Browser console access and debugging examples
- **Cross-Platform Usage** - Platform-specific configuration and usage

---

## [0.1.0] - 2025-11-15

### 🎉 Initial Release

### Added
- **MCP Protocol Implementation**: Full JSON-RPC 2.0 support for Model Context Protocol
- **Multi-Transport Support**: HTTP and stdio transport layers for MCP servers
- **Cross-Platform Binaries**: Native support for Linux, macOS, and Windows (AMD64/ARM64)
- **Configuration Management**:
  - Compatible with common MCP Servers configurations format: `mcp_servers.json`
  - Platform-specific configuration directories (XDG compliant on Unix)
  - Environment variable substitution in headers (`${VAR_NAME}`)
  - First-run automatic configuration initialization
- **CLI Interface**:
  - `list-servers` - List all configured MCP servers
  - `list-tools` - List tools from servers (all or specific)
  - `call-tool` - Execute MCP tools with arguments
  - `list-resources` - List resources from servers
  - `create-config` - Generate example configuration files
  - `version` - Display version and build information
- **Security Features**:
  - No hardcoded credentials or API keys
  - Environment variable based authentication
  - Comprehensive input validation and error handling
- **Developer Experience**:
  - Zero runtime dependencies
  - Single binary distribution
  - Verbose output mode for debugging
  - Comprehensive error messages with proper exit codes
- **Build System**:
  - Multi-platform cross-compilation
  - Automated testing and linting
  - Version management with build metadata
  - GitHub Actions CI/CD pipelines
- **Installation Methods**:
  - One-line installer scripts (bash and PowerShell)
  - Manual binary installation
  - Source code compilation
  - Professional installer with platform detection

### Features
- **Context-Aware Design**: Intelligent MCP server interaction without context window pollution
- **Professional Installer**: Homebrew-style installation with dependency management
- **Platform Detection**: Automatic detection of OS, architecture, and WSL environment
- **Configuration Discovery**: Automatic config file discovery with fallback support
- **Timeout Management**: Configurable request timeouts for all MCP operations
- **Server Management**: Enable/disable servers, multiple server support
- **Error Recovery**: Graceful handling of network failures and server errors

### Security
- Input sanitization for all user-provided data
- Safe environment variable expansion
- No credential storage in configuration files
- Proper handling of sensitive information in error messages

### Documentation
- Comprehensive README with installation and usage guides
- Example configuration files with real MCP server setups
- Platform-specific installation instructions

### Validation
- Installation script syntax validation and testing
- Platform detection and compatibility verification
- Cross-platform binary compilation testing
- Configuration file format validation

### Tooling
- Go-based implementation for performance and portability
- Makefile with comprehensive build targets
- Automated dependency management
- Development environment setup scripts