# MCP CLI-ENT

*"Do not be hasty."*

A wise context-guardian for your AI agent's MCP (Model Context Protocol) servers.

## 🌲 The Problem: Context Window Deforestation

The challenge arises when an AI model's limited context window is overwhelmed by loading the entire set of tool definitions and metadata from every connected MCP server. This mass-loading occurs regardless of the tools' relevance to the current task, consuming valuable context better reserved for reasoning and task completion.

## 🛡️ The Solution: The CLI-Ent

`mcp-cli-ent` acts as a deliberate guardian by fundamentally changing the interaction model.

Instead of forcing an agent to load all tool definitions from all servers *into* its limited context, the agent is given a **single, lightweight tool**: the `mcp-cli-ent` CLI itself.

The agent can now access any MCP server *on-demand* by executing a simple CLI command. The definitions and metadata live *outside* the context window, invoked only when needed.

This provides a complete, context-safe interaction model in several layers:

- **Agent-Friendly Discoverability**: `mcp-cli-ent` exposes simple, agent-usable commands (`list-servers`, `list-tools [server]`) that allow the agent to dynamically discover available servers, their tools, and the required arguments. The agent learns *what* it can do, *when* it needs to, without any upfront context cost.
- **On-Demand Tool Execution**: Based on that discovery, the agent calls specific tools via the CLI (`mcp-cli-ent call-tool ...`). This eliminates the primary cause of context deforestation, as no server definitions are ever loaded into the agent's prompt.
- **Structured Responses**: As a final layer of defense, `mcp-cli-ent` intercepts the verbose, raw JSON *output* from the tool and returns a clean, parsed summary, preventing the context from being flooded by a successful call.
- **True Context Preservation**: This multi-layer "Discover, Execute, Summarize" approach maintains focus on high-signal information, allowing the agent to use its context for complex reasoning, not for storing definitions or parsing output.

The name **CLI-Ent** embodies this dual purpose:
- **CLI**: A **C**ommand **L**ine **I**nterface for accessing MCP servers.
- **Ent**: Inspired by the wise, deliberate guardians from *Lord of the Rings*, who protect their environment (the context window) from "hasty" and wasteful-loading.

## Features

- **Cross-Platform Compatibility**: Works seamlessly across Claude Code, VSCode, and other MCP-compatible environments.
- **Cross-platform support** - Single binary for Windows, macOS, and Linux
- **Zero runtime dependencies** - No external dependencies required
- **Gemini, OpenCode, Claude Code, VSCode (and forks) compatible** - Uses the same usual `mcp_servers.json` configuration format
- **HTTP and stdio transport support** - Works with both HTTP-based and stdio-based MCP servers
- **Multi-server management** - Configure and interact with multiple MCP servers
- **Environment variable substitution** - Use `${VAR_NAME}` in headers and environment
- **Comprehensive error handling** - Clear error messages and proper exit codes

## Screenshots

### MCP CLI-ENT in Action

Tool Execution - Getting Documentation via Context7:
![MCP CLI-ENT Server List](screenshot-01.png)
![MCP CLI-ENT Tool Call](screenshot-02.png)

The CLI-ENT provides a clean, command-line interface for interacting with MCP servers while preserving your AI agent's context window.

## Installation

### Quick Install (Recommended)

**Linux, macOS, and Windows (WSL):**
```bash
# Option 1: One-line installation
curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh | bash

# Option 2: Download and run locally
curl -fsSL https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.sh -o install.sh
chmod +x install.sh
./install.sh
```

**Windows (PowerShell):**
```powershell
# Option 1: One-line installation
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.ps1" -OutFile "install.ps1"
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process -Force
.\install.ps1

# Option 2: Direct execution
iwr -useb https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.ps1 | iex
```

**What the installer does:**
- ✅ Detects your platform and architecture automatically
- ✅ Downloads the latest release binary
- ✅ Installs to standard locations:
  - **Linux/macOS/WSL**: `~/.local/bin/`
  - **Windows**: `%USERPROFILE%\AppData\Roaming\mcp-cli-ent\`
- ✅ Adds to PATH (you may need to restart your shell)
- ✅ Installs dependencies on Linux (curl)

### Manual Installation

1. Download pre-built binaries from [Releases](https://github.com/EstebanForge/mcp-cli-ent/releases)
2. Extract and move to a directory in your PATH
3. Create your config file for MCP Servers. See below.

### Build from Source

```bash
git clone https://github.com/EstebanForge/mcp-cli-ent.git
cd mcp-cli
make build
```

**Requirements:** Go 1.21+

## Verification

After installation, verify it works:

```bash
mcp-cli-ent --version
mcp-cli-ent create-config
mcp-cli-ent list-servers
```

## Configuration

### MCP Server Configuration

The MCP CLI stores MCP server configurations in `mcp_servers.json` in standard platform-specific locations:

**Linux, macOS, and WSL:**
```
~/.config/mcp-cli-ent/mcp_servers.json
```

**Windows:**
```
%USERPROFILE%\AppData\Roaming\mcp-cli-ent/mcp_servers.json
```

**Note**: `mcp_servers.json` contains MCP server definitions only. A separate `config.json` file will be used for tool configuration in the future (not yet implemented).

### Creating Configuration

The easiest way to create a configuration file is:

```bash
mcp-cli-ent create-config
```

This creates `mcp_servers.json` in the standard location with example server configurations.

For reference, see `mcp_servers.example.json` in the project directory for the configuration format.

The configuration file format is compatible with Claude Code and VSCode:

```json
{
  "mcpServers": {
    "chrome-devtools": {
      "command": "npx",
      "args": ["chrome-devtools-mcp@latest"]
    },
    "context7": {
      "type": "http",
      "url": "https://mcp.context7.com/mcp",
      "headers": {
        "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
      },
      "timeout": 30
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"],
      "timeout": 30
    },
    "deepwiki": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://mcp.deepwiki.com/sse"],
      "timeout": 30
    }
  }
}
```

### Configuration Options

- **HTTP servers**: Use `"type": "http"` or provide `"url"`
- **Stdio servers**: Use `"command"` and optional `"args"`
- **Headers**: Add `"headers"` object for HTTP authentication
- **Environment variables**: Use `"env"` object for stdio servers
- **Environment substitution**: Use `${VAR_NAME}` in header values
- **Disable servers**: Set `"disabled": true` to temporarily disable a server
- **Timeout**: Set `"timeout"` in seconds (default: 30)

### Creating an Example Configuration

```bash
mcp-cli-ent create-config
```

## Usage

### List all configured servers

```bash
mcp-cli-ent list-servers
```

### List tools from all enabled servers

```bash
mcp-cli-ent list-tools
```

### List tools from a specific server

```bash
mcp-cli-ent list-tools context7
```

### Call a tool with arguments

```bash
# Get HyperPress block building documentation via Context7
mcp-cli-ent --timeout 60 call-tool context7 get-library-docs '{"context7CompatibleLibraryID": "/estebanforge/hyperpress", "query": "how to build a block"}'

# Use sequential thinking
mcp-cli-ent call-tool sequential-thinking sequentialthinking '{
  "thought": "I need to solve this complex problem step by step...",
  "nextThoughtNeeded": true,
  "thoughtNumber": 1,
  "totalThoughts": 5
}'
```

### List resources from a server

```bash
mcp-cli-ent list-resources deepwiki
```

### Using a custom configuration file

For custom configurations or testing, you can specify a different file:

```bash
mcp-cli-ent --config /path/to/custom.json list-servers
```

### Configuration File Discovery

The MCP CLI automatically discovers configuration files in this order:

1. **Custom file**: Specified with `--config` flag
2. **Standard location**: Platform-specific config directory
3. **Current directory**: `mcp_servers.json` (for backward compatibility)

This means existing users with configuration files in their current directory will continue to work without changes.

### Verbose output

```bash
mcp-cli-ent --verbose list-tools
```

## Available Commands

- `list-servers` - List all configured MCP servers
- `list-tools [server]` - List tools from servers
- `call-tool <server> <tool> [args]` - Call a specific tool
- `list-resources <server>` - List resources from a server
- `create-config [filename]` - Create an example configuration
- `version` - Show version information

## Pre configured MCP Servers

- **Context7** - Library documentation and code snippets
- **DeepWiki** - GitHub repository documentation
- **Sequential Thinking** - Problem-solving and planning tool
- **Chrome DevTools** - Let agents navigate and use Chrome and its dev tools

## Examples

### Library Documentation via Context7

```bash
# Resolve a library
mcp-cli-ent call-tool context7 resolve-library-id '{"libraryName": "react"}'

# Get library documentation
mcp-cli-ent call-tool context7 get-library-docs '{
  "context7CompatibleLibraryID": "/reactjs/react.dev",
  "tokens": 2000
}'
```

### Repository Documentation via DeepWiki

```bash
# List GitHub repository structure
mcp-cli-ent call-tool deepwiki read_wiki_structure '{
  "repoName": "facebook/react"
}'

# Get repository contents
mcp-cli-ent call-tool deepwiki read_wiki_contents '{
  "repoName": "facebook/react"
}'
```

### Problem Solving with Sequential Thinking

```bash
# Start a thinking process
mcp-cli-ent call-tool sequential-thinking sequentialthinking '{
  "thought": "I need to analyze this complex system design problem...",
  "nextThoughtNeeded": true,
  "thoughtNumber": 1,
  "totalThoughts": 8
}'
```

## Development

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Development setup
make dev-setup
```

### Testing

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage
```

### Linting

```bash
# Format code
make fmt

# Lint code
make lint
```

## Error Handling

The tool includes comprehensive error handling for:

- Network connectivity issues
- Invalid JSON arguments and configuration
- Server errors and HTTP status codes
- Missing dependencies and configuration files
- Disabled servers and invalid server names
- Timeouts and connection failures

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/EstebanForge/mcp-cli-ent/issues)
- **Discussions**: [GitHub Discussions](https://github.com/EstebanForge/mcp-cli-ent/discussions)
