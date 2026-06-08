---
name: mcp-cli-ent
description: Interact with Model Context Protocol (MCP) servers using the mcp-cli-ent command-line client. Use when listing servers, checking tool schemas, calling tools, or managing persistent browser sessions.
---

# MCP CLI-Ent

A Go-based standalone CLI client for Model Context Protocol (MCP) servers. Agent-first: JSON output by default, structured for programmatic consumption.

## Quick Start

```bash
# Discover all available tools (compact index: name + description per tool)
mcp-cli-ent

# List tools from a specific server (full JSON with params, call examples)
mcp-cli-ent list-tools context7

# Search for a tool across all servers
mcp-cli-ent --search "wiki"

# Call a tool
mcp-cli-ent call brave-search brave_web_search '{"query":"gemini models"}'

# Human-readable terminal output (terse, 1 line per tool)
mcp-cli-ent list-tools context7 --human

# Expanded human output (desc, params, call example)
mcp-cli-ent list-tools context7 --human --verbose
```

## Output Modes

### Bare invocation: Compact Discovery Index

Running `mcp-cli-ent` with no args returns a compact index — one `name` + `description` per tool, grouped by server. No params, no call examples, no schema. Use `list-tools <server>` when ready to call.

```json
{
  "context7": [
    { "name": "resolve-library-id", "description": "Resolves a package name to a Context7 library ID" },
    { "name": "query-docs", "description": "Retrieves up-to-date documentation and code examples" }
  ],
  "deepwiki": [
    { "name": "read_wiki_structure", "description": "Get documentation topics for a GitHub repository" }
  ]
}
```

### `list-tools <server>`: Full Tool Details

Returns full JSON with `name`, `description`, `params`, `call` per tool.

```json
[
  {
    "name": "resolve-library-id",
    "description": "Resolves a package/product name...",
    "params": ["libraryName", "query"],
    "call": "mcp-cli-ent call context7 resolve-library-id '{\"libraryName\":\"...\", \"query\":\"...\"}'"
  }
]
```

### `--human`: Terminal Output

Terse by default (1 line per tool). `--human --verbose` expands to 4-line format (name, desc, params, call).

### `--verbose` (JSON mode): Adds full `schema` field with complete `inputSchema`.

### Error responses

Empty results or no matches return structured JSON errors:

```json
{ "error": true, "error_code": "no_match", "error_description": "No tools matching 'wiki' found" }
{ "error": true, "error_code": "no_tools", "error_description": "No tools found on any server" }
```

## Flags

| Flag | Description |
|------|-------------|
| `--search <query>` | Filter tools by name or description (case-insensitive). Works in both JSON and human modes. |
| `--human` | Switch from JSON to human-readable terminal output. |
| `-v, --verbose` | JSON: include full `schema`. Human: expanded 4-line format. |
| `--refresh` | Force refresh tools cache. |
| `--clear-cache` | Clear tools cache. |
| `--config <path>` | Custom config file path. |
| `--timeout <sec>` | Request timeout (default 30). |

## Workflows

### 1. Tool Discovery & Execution
- [ ] Run `mcp-cli-ent` to get compact discovery index (all servers, all tools).
- [ ] Filter with `--search "keyword"` to find specific capabilities.
- [ ] Run `mcp-cli-ent list-tools <server>` for full params and call examples.
- [ ] Use `--verbose` to get full parameter schemas when needed.
- [ ] Execute: `mcp-cli-ent call <server> <tool> '<json-args>'`.
- [ ] Ensure integers, booleans, arrays are unquoted in JSON payload (`0`, `true`, `[]`).

### 2. Session Management
For browser-based servers requiring persistent state (Chrome DevTools, Playwright):
- [ ] Check sessions: `mcp-cli-ent session list`
- [ ] Start persistent session: `mcp-cli-ent session start <server-name>`
- [ ] Call tool (auto-routed through session): `mcp-cli-ent call <server> <tool> '<args>'`
- [ ] Stop session: `mcp-cli-ent session stop <server-name>`

### 3. Daemon Management
Background daemon persists sessions across CLI invocations:
- [ ] Check state: `mcp-cli-ent daemon status`
- [ ] Start: `mcp-cli-ent daemon start`
- [ ] Stop: `mcp-cli-ent daemon stop`
- [ ] Logs: `mcp-cli-ent daemon logs`
