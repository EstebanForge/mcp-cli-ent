# Changelog

## [0.1.0] - 2025-11-15

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

