# Migration Plan: MCP 2026-07-28 (Dual-Era Client)

| Field | Value |
|---|---|
| Spec target | MCP 2026-07-28 |
| Companion | `docs/spec-compliance-2026-07-28.md` (29-finding audit) |
| Strategy | **Dual-era**: add modern path, keep legacy as cached fallback |
| Default | `protocolVersion: "auto"` (probe + detect + cache) |

## Goal and non-goals

**Goal.** Make mcp-cli-ent a dual-era client: it speaks the modern stateless protocol with 2026-07-28 servers and falls back to the legacy `initialize` handshake with older servers. The path is chosen per server, automatically, and cached.

**Non-goals (this plan).**
- Removing the legacy path. It stays as the fallback tier until the ecosystem is predominantly modern.
- Implementing server-side 2026-07-28. This is a client-only tool.
- Adopting deprecated features (Roots/Sampling/Logging) on the modern path.

## Guiding decision: dual-era, default `auto`

Tested 2026-07-28: every reachable server in `mcp_servers.example.json` is **legacy** (deepwiki, context7, sequential-thinking all reject `server/discover` / the modern version). A modern-only client would break the entire installed base today. Therefore:

- The legacy path is **first-class**, not deprecated.
- Era is detected, not assumed. A new config field lets the user force a path when detection is wrong or unwanted.

Per the [compatibility matrix](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning#compatibility-matrix), only a dual-era client works against both modern and legacy servers.

## The `protocolVersion` field

New optional field on `config.ServerConfig`:

```go
// internal/config/types.go
type ServerConfig struct {
    // ...existing fields...
    ProtocolVersion string `json:"protocolVersion,omitempty"`
}
```

**Values:**

| Value | Meaning |
|---|---|
| `""` or `"auto"` (default) | Probe `server/discover`, detect era, cache verdict. Skip probe on subsequent calls. |
| `"2026-07-28"` | Force modern path (stateless, per-request `_meta`, new headers). No probe. |
| `"2025-11-25"` and earlier dates | Force legacy path (`initialize` handshake). No probe. |

Accepted version dates: `2024-11-05`, `2025-03-26`, `2025-06-18`, `2025-11-25`, `2026-07-28`. Unknown values: warn at config load, fall back to `auto` (do not brick startup on a typo).

**Era mapping** (internal): `2026-07-28` → modern; all earlier dates → legacy.

**Semantics.** The field selects the *protocol path*, not the transport. Both eras run over stdio and HTTP. Do not conflate with the existing `Type` (http/stdio) or `Session.Type`.

**Config example:**

```json
{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "protocolVersion": "2025-11-25",
      "headers": { "Context7-API-Key": "${ENT_CONTEXT7_API_KEY}" }
    },
    "deepwiki": {
      "protocolVersion": "2025-11-25",
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://mcp.deepwiki.com/mcp"]
    },
    "newModernServer": {
      "url": "https://example.com/mcp"
    }
  }
}
```

Servers tested legacy on 2026-07-28 are pinned for documentation and to skip the probe round-trip. Unpinned servers default to `auto` and resolve themselves.

## Era detection algorithm (the hinge)

Implements the spec's [version-negotiation](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning) and transport fallback rules. Only runs when `protocolVersion` is `auto`.

**Recognized modern errors** (any of these in a response body = modern server): `UnsupportedProtocolVersionError` (`-32022`), `HeaderMismatch` (`-32020`), `MissingRequiredClientCapability` (`-32021`).

### HTTP transport

1. Send a modern request: `_meta` (protocolVersion, clientInfo, clientCapabilities) + `MCP-Protocol-Version` header.
2. On `DiscoverResult` or any recognized modern error → **modern**. On `UnsupportedProtocolVersionError`, pick a mutually supported version from `data.supported` and retry.
3. On `400 Bad Request` with empty body or a non-modern error → **legacy**. Fall back to `initialize`.
4. Cache verdict per origin.

### stdio transport

1. Send `server/discover` with modern `_meta`.
2. `DiscoverResult` or `UnsupportedProtocolVersionError` → **modern** (retry on the latter with a supported version).
3. Any other error (`-32601 Method not found`, timeout, etc.) or no response → **legacy**. Fall back to `initialize`.
4. Cache verdict per server process.

**Cache lifecycle.** Verdict is a property of the server, not the request. Cache for the lifetime of the stdio process or HTTP origin. May persist across restarts of the same server config; re-probe if the cached assumption later fails. If `protocolVersion` is an explicit pin, skip detection entirely.

### Verification against tested servers

| Server | Probe response | Algorithm verdict | Correct? |
|---|---|---|---|
| deepwiki (HTTP) | `-32600` "Unsupported protocol version", lists legacy versions | legacy (not a modern error code) | yes |
| context7 (HTTP) | `-32000` "No valid session ID provided" | legacy | yes |
| sequential-thinking (stdio) | `-32601` "Method not found" | legacy | yes |

The algorithm classifies the current installed base correctly with no config edits.

## Phased plan

Each phase is independently verifiable. Findings reference `docs/spec-compliance-2026-07-28.md`.

### Phase 0 — Foundation (unblocks all others)
- Add `ProtocolVersion` constant in `internal/mcp/types.go`. (F1)
- Inject `_meta` (protocolVersion, clientInfo, clientCapabilities) on every outbound request from a single transport-level helper. (F2)
- Set `MCP-Protocol-Version` header on HTTP POST. (F3)
- Capture and expose `serverInfo` from results. (F4)
- **Verify:** a modern request carries correct `_meta` + header against a mock modern server.

### Phase 1 — `protocolVersion` field + era detection
- Add field to `ServerConfig`; config validation; default `auto`. (new)
- Implement era detection for HTTP and stdio per the algorithm above. (F10, F11)
- Route to modern or legacy path based on verdict/pin.
- Keep existing `initialize`/handshake code as the legacy path (do not delete). (F5, F7, F8 — the daemon fabrication stub becomes the legacy-path behavior, clearly labeled, not a lie about modern negotiation)
- Cache verdict per server. (new)
- **Verify:** auto resolves each example server to the correct era; explicit pins skip the probe.

### Phase 2 — HTTP transport completion (modern path)
- Required routing headers `Mcp-Method`, `Mcp-Name` with Base64 sentinel encoding. (F13, F14)
- SSE response-stream consumer (`text/event-stream`); handle progress notifications before final response. (F15)
- Unique per-request JSON-RPC IDs. (F16)
- Stop blind `/mcp` path retry on 404; honor `-32601`. (F17)
- `resultType` on all results; treat omitted as `"complete"`. (F25, partial)
- **Verify:** full modern request/response cycle against a conformant modern server, including SSE.

### Phase 3 — Authorization (OAuth 2.1, modern path)
- Protected Resource Metadata discovery (RFC 9728). (F19)
- Authorization server discovery (RFC 8414 / OIDC). (F19)
- Client registration: Client ID Metadata Documents preferred, DCR as fallback. (F20)
- PKCE, `resource` param (RFC 8707), `iss` validation (RFC 9207). (F21)
- Token store keyed by issuer; no cross-issuer reuse; re-register on AS change. (F22)
- `Authorization: Bearer` injection on every HTTP request.
- Step-up authorization / `insufficient_scope` (403) handling. (F23)
- stdio stays on environment credentials, not OAuth. (F24)
- Legacy path keeps the existing `Headers` map (works for API-key servers like context7).
- **Verify:** OAuth flow completes against a protected modern server; legacy API-key servers still work.

### Phase 4 — Daemon re-scoping and security
- Add an auth boundary to the daemon's local HTTP API, or demote the daemon to a pure host concern with no externally callable surface. (F29)
- Remove the fabricated `DaemonMCPClient.Initialize`; era detection replaces it. (F8)
- Decide `internal/session` vs `internal/daemon` consolidation: one stateful subsystem, not two. (F9)
- **Verify:** unauthenticated local processes cannot drive sessions/tools.

### Phase 5 — Deprecation cleanup (legacy-only features)
- Keep Roots/Sampling/Logging available on the legacy path; remove from the modern path. (F27)
- Do NOT port `CreateMessage`/`RequestInput`/`ListRoots` to MRTR; they are deprecated. (F25, remainder)
- Track the 2027-07-28 earliest-removal date.
- **Verify:** legacy servers using these features still work; modern path does not advertise them.

### Phase 6 — Caching and extensions (opportunities)
- `ttlMs` + `cacheScope` (CacheableResult) on list/read results; respect for client-side caching. (F26)
- `extensions` capability map. (F12)
- Optional opt-in to Tasks (`tasks/get`, `tasks/update`) and MCP Apps as client features. (F28)
- **Verify:** cache honors `ttlMs`; an extension round-trips its capability.

## Risks and open questions

- **`Session.Type` vs era conflict.** Legacy era requires `initialize` + a held connection, so it cannot be `stateless`. If config pins legacy (`protocolVersion` < `2026-07-28`) and `Session.Type: "stateless"`, **error at config load** (fail closed, no silent coercion). `ValidateConfig` already exists in `config/config.go` as the insertion point. **DECIDED 2026-07-28.**
- **Detection latency.** `auto` adds one probe round-trip on cold start. Mitigated by caching and explicit pins for known servers. Acceptable; revisit if measured as a problem.
- **Mixed-era auth.** context7 ships OAuth Protected Resource Metadata on a legacy (session-based) transport. Expect servers that mix modern auth with legacy transport during the transition. The era field drives transport/handshake; auth detection is independent (driven by 401 + `WWW-Authenticate`). Keep them decoupled.
- **`-32600` from deepwiki.** Not a recognized modern error code, so it routes to legacy (correct outcome) but we cannot extract its advertised supported versions. Acceptable; document the edge.
- **Stdio era cache invalidation.** If a server upgrades between runs and the cache persists, the stale verdict will fail on first use. Re-probe on any unexpected failure; do not trust the cache blindly.

## Sequencing rationale

Foundation (0) before detection (1), because detection sends a modern request and needs `_meta`/header correct first. HTTP completion (2) and auth (3) are the bulk of modern-path correctness and can proceed in parallel once routing exists. Daemon re-scoping (4) is security-critical and independent. Deprecation (5) and extensions (6) are lowest risk and last.

## References

- Spec: [Versioning & compatibility matrix](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning), [Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http), [stdio](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio), [`server/discover`](https://modelcontextprotocol.io/specification/2026-07-28/server/discover), [Authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/index)
- Internal: `docs/spec-compliance-2026-07-28.md` (findings F1-F29), [announcement](https://claude.com/blog/bringing-mcp-2026-07-28-to-claude)
