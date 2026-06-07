---
name: mcp-cli-ent
description: Interact with Model Context Protocol (MCP) servers using the mcp-cli-ent command-line client. Use when listing servers, checking tool schemas, calling tools, or managing persistent browser sessions.
---

# MCP CLI-Ent

A Go-based standalone CLI client for Model Context Protocol (MCP) servers. Allows direct interaction and persistent daemon-managed browser sessions.

## Quick Start

Find and call tools directly:

```bash
# List all tools and copy-pasteable call commands
mcp-cli-ent

# Call a tool with parameters
mcp-cli-ent call-tool brave-search brave_web_search '{"query":"gemini models"}'
```

## Workflows

### 1. Tool Discovery & Execution
- [ ] Run `mcp-cli-ent` to see available servers, tools, and copy-pasteable invocation examples.
- [ ] Use `--verbose` for detailed descriptions: `mcp-cli-ent list-tools <server-name> --verbose`.
- [ ] Execute the tool: `mcp-cli-ent call-tool <server-name> <tool-name> '<json-arguments>'`.
- [ ] Check parameters and types: ensure integers, booleans, and arrays are unquoted in the JSON payload (e.g. `0`, `true`, `[]`).

### 2. Session Management
For browser-based servers requiring persistent state (e.g. Chrome DevTools, Playwright):
- [ ] Check session status: `mcp-cli-ent session list`
- [ ] Start persistent session: `mcp-cli-ent session start <server-name>`
- [ ] Call tool using stateful session (automatically routed): `mcp-cli-ent call-tool <server-name> <tool-name> '<args>'`
- [ ] Stop session: `mcp-cli-ent session stop <server-name>`

### 3. Daemon Management
The background daemon persists sessions across CLI command invocations:
- [ ] Check daemon state: `mcp-cli-ent daemon status`
- [ ] Start daemon: `mcp-cli-ent daemon start`
- [ ] Stop daemon: `mcp-cli-ent daemon stop`
- [ ] View daemon logs: `mcp-cli-ent daemon logs`
