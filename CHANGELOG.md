# Changelog

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