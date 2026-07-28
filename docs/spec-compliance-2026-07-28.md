# Spec Compliance Audit: MCP 2026-07-28

| Field | Value |
|---|---|
| Spec version | **MCP 2026-07-28** (fifth release; previous 2025-11-25) |
| Audit date | 2026-07-28 |
| Audited code | `EstebanForge/mcp-cli-ent` (HEAD) |
| Pinned protocol version | `"2024-11-05"` (THREE revisions stale) |
| Scope | Collision inventory only. No code changes. |

> Read with `docs/migration-plan-2026-07-28.md` (to be produced) for the phased remediation track.

---

## Severity legend

| Severity | Meaning |
|---|---|
| **CRITICAL** | Fabricates protocol state, blocks interop with modern servers, or opens a security hole. |
| **HIGH** | Violates a spec **MUST**. Breaks against any conformant modern server. |
| **MEDIUM** | Violates a **SHOULD**, mishandles a deprecated feature, or carries forward dead surface. |
| **LOW** | Cosmetic / future-proofing. No runtime break. |

---

## Executive summary

The 2026-07-28 spec rewrites the protocol around three ideas mcp-cli-ent does not currently model: a **stateless request/response core**, **per-request capability negotiation**, and **OAuth 2.1 authorization**. The codebase is built on the opposite assumptions: one-time `initialize`, held connections, and static env-resolved header maps.

**Five headline collisions:**

1. **Protocol version is stale and duplicated.** Literal `"2024-11-05"` at `internal/cli/commands.go:846` and `internal/daemon/client.go:375`. No version constant exists. Two generations behind.
2. **`initialize` is vestigial or fabricated.** HTTP and stdio transports send it then **discard the result** (`http.go:128`, `stdio.go:170`). The daemon `DaemonMCPClient.Initialize` **fabricates** a negotiation outcome without contacting the server (`daemon/client.go:371-386`). Under per-request negotiation this is the single worst assumption: it silently lies.
3. **Zero authorization.** Auth is a one-shot env-resolved `Headers` map (`config/types.go:11-23`, `config/config.go:108-112`). No OAuth, no token store, no scopes, no refresh, no `Authorization: Bearer` injection. The full 2026-07-28 OAuth 2.1 flow is unimplemented.
4. **Daemon HTTP API is unauthenticated** (`daemon/server.go`, bound `127.0.0.1:8080` / unix socket). Any local process can start/stop sessions and invoke tools. Not MCP-protocol compliant either (custom REST, not JSON-RPC).
5. **Two parallel stateful subsystems** assume held clients + caches: `internal/session/` (file-backed) and `internal/daemon/` (in-memory). Only `internal/session/stateless.go` (`StatelessSession`, creates a fresh client per call) aligns with the stateless core.

**Count:** 29 findings. 7 CRITICAL, 10 HIGH, 9 MEDIUM, 3 LOW.

---

## What the 2026-07-28 spec changed (context)

Source: [Key Changes](https://modelcontextprotocol.io/specification/2026-07-28/changelog), [Architecture](https://modelcontextprotocol.io/specification/2026-07-28/architecture/index), [Versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning).

1. **Stateless core.** Removes `initialize` / `notifications/initialized`. Every request carries protocol version + client capabilities in `_meta` (`io.modelcontextprotocol/protocolVersion`, `.../clientCapabilities`, `.../clientInfo`). Servers answer `.../serverInfo` in result `_meta`. (SEP-2575)
2. **No protocol sessions.** Removes `Mcp-Session-Id` from Streamable HTTP. List endpoints no longer vary per-connection. Cross-call state is explicit server-minted handles passed as ordinary tool args. (SEP-2567)
3. **`server/discover`** is now a **MUST** RPC: advertises supported versions, capabilities, identity. Clients MAY call it first or go inline and handle `UnsupportedProtocolVersionError`. (SEP-2575)
4. **Streamable HTTP reshaped.** POST-only (GET endpoint removed). Required headers `Mcp-Method`, `Mcp-Name`, plus `MCP-Protocol-Version`. SSE streams are per-request; no resumability (`Last-Event-ID` gone). Server-to-client calls are embedded as `InputRequiredResult`, never sent as standalone requests. (SEP-2243, SEP-2575)
5. **Multi Round-Trip Requests (MRTR).** Server-to-client interactions (sampling, elicitation, roots) return `resultType: "input_required"` with `inputRequests`; client retries the original request carrying `inputResponses`. (SEP-2322)
6. **All results carry `resultType`** (`"complete"` | `"input_required"`). Clients MUST treat omitted field as `"complete"`.
7. **Authorization hardened to OAuth 2.1.** Protected Resource Metadata (RFC 9728), AS discovery (RFC 8414 / OIDC), PKCE, `resource` param (RFC 8707), Bearer on every request, `iss` validation (RFC 9207), refresh tokens. (SEP-2468, SEP-2352)
8. **Caching.** `tools/list`, `prompts/list`, `resources/list`, `resources/read`, `resources/templates/list` results MUST carry `ttlMs` + `cacheScope` (CacheableResult). (SEP-2549)
9. **Removed methods.** `ping`, `logging/setLevel`, `notifications/roots/list_changed`. Log level is per-request via `_meta`.
10. **Deprecated (not removed):** Roots, Sampling, Logging (SEP-2577); HTTP+SSE transport; OAuth Dynamic Client Registration (RFC 7591, replaced by Client ID Metadata Documents).
11. **Extensions framework.** `extensions` map in capabilities, versioned identifiers (`io.modelcontextprotocol/tasks`, `io.modelcontextprotocol/ui`). Tasks graduated to a redesigned extension (`tasks/get`, `tasks/update`). (SEP-2663, SEP-1865, SEP-2133)
12. **Error code reallocation.** `-32020` to `-32099` reserved for spec. `HeaderMismatch` now `-32020`, `MissingRequiredClientCapability` `-32021`, `UnsupportedProtocolVersion` `-32022`. Resource-not-found moved to `-32602`.

---

## Collision inventory

### 4.1 Protocol versioning and per-request metadata

Every request MUST declare its protocol version in `_meta.io.modelcontextprotocol/protocolVersion`, mirrored to the `MCP-Protocol-Version` HTTP header. No constant exists; two sites hardcode the oldest version still in the tree.

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F1 | Pinned version is three revisions stale and duplicated | `cli/commands.go:846`, `daemon/client.go:375` (literal `"2024-11-05"`); no constant in `internal/mcp/types.go` | Version carried per request in `_meta`; single source of truth | **HIGH** |
| F2 | Requests carry no `_meta` block at all | `JSONRPCRequest` (`mcp/types.go:8-16`) has no `Params._meta`; HTTP request build at `client/http.go:282-340` sets only `Content-Type` + `Accept` + static headers; stdio build at `client/stdio.go:260-340` likewise | `_meta` MUST carry `protocolVersion`, `clientCapabilities`; SHOULD carry `clientInfo` (SEP-2575) | **CRITICAL** |
| F3 | No `MCP-Protocol-Version` header on HTTP POST | `client/http.go:300-305` (header set omits it) | Header REQUIRED on every Streamable HTTP POST (SEP-2575) | **CRITICAL** |
| F4 | No `serverInfo` capture from server results | `InitializeResult`/result types in `mcp/types.go:205-210` parse `ServerInfo` but it is never stored or used | Servers SHOULD return `serverInfo` in result `_meta`; clients SHOULD surface it | LOW |

**Migration direction.** Add a `ProtocolVersion` constant in `internal/mcp/types.go`; inject a `_meta` block on every outbound request from a single transport-level helper; set `MCP-Protocol-Version` on HTTP.

### 4.2 Stateless core (sessions removed)

The spec deletes the protocol session. mcp-cli-ent leans entirely on held-connection sessions.

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F5 | `MCPClient` interface forces a held-handle model | `Initialize(ctx, params)` + `Close() error` as peer methods (`mcp/protocol.go:13-14`); no per-request negotiation param on `ListTools`/`CallTool`/`ListResources` (`protocol.go:17-21`) | State is per-request, not per-connection | **HIGH** |
| F6 | Stdio transport is fully stateful | `StdioClient` holds `cmd`, pipes, `reader`, `writer`, `mutex` (`client/stdio.go:20-28`); assumes single in-flight request, skips notifications (`stdio.go:314-316`) | Stdio still allowed; each request still self-contained; era is a server property to cache | MEDIUM |
| F7 | `initialize` is vestigial on both transports | HTTP `Initialize` sends method then **discards result** (`client/http.go:128-150`, esp. `:146`); stdio same (`client/stdio.go:170-191`, esp. `:188`) | `initialize` removed from modern protocol | MEDIUM |
| F8 | Daemon fabricates the negotiation outcome | `DaemonMCPClient.Initialize` returns hardcoded `ProtocolVersion:"2024-11-05"`, fake `ServerCapabilities`, `ServerInfo{Name:"mcp-cli-ent-daemon"}` without contacting the server (`daemon/client.go:371-386`) | Must not fabricate; negotiate per request or via `server/discover` | **CRITICAL** |
| F9 | Two stateful session subsystems | `internal/session/*` (manager/persistent/filestore) and `internal/daemon/*` (`PersistentSession` + tool cache, `daemon/types.go:21-32`) both hold clients + cache | No protocol session; cross-call state = explicit handles | **HIGH** |

**Alignment point.** `internal/session/stateless.go` `StatelessSession` (creates a fresh client per `Client()` call, `stateless.go:57-78`) is the existing shape closest to the new core.

### 4.3 Capability negotiation and `server/discover`

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F10 | No `server/discover` support | Absent from `MCPClient` interface and all transports; daemon health-checks via `ListTools` (`daemon/daemon.go:142`) | `server/discover` is a **MUST** server RPC; clients SHOULD probe on stdio for era detection (SEP-2575) | **HIGH** |
| F11 | No era / back-compat fallback model | No modern-vs-legacy detection anywhere | Dual-era clients MUST detect era (stdio: probe + fall back on non-modern error; HTTP: inspect `400` body before falling back to `initialize`) | **HIGH** |
| F12 | No `extensions` field in capabilities | `ClientCapabilities` (`mcp/types.go:192-196`) and `ServerCapabilities` (`mcp/types.go:155-161`) have no `extensions map` | Extensions advertised via `capabilities.extensions` map (SEP-2133) | MEDIUM |

### 4.4 Streamable HTTP transport

The HTTP transport is *already* per-request POST (a positive), but misses every 2026-07-28 requirement.

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F13 | Missing required routing headers | `client/http.go:300-305` sets only `Content-Type`, `Accept`, static headers | `Mcp-Method` (all requests) and `Mcp-Name` (`tools/call`, `resources/read`, `prompts/get`) REQUIRED (SEP-2243) | **CRITICAL** |
| F14 | No `x-mcp-header` tool-param mirroring | Not implemented | Clients MUST mirror annotated tool params into `Mcp-Param-{Name}` headers, with Base64 sentinel encoding for non-ASCII (SEP-2243) | MEDIUM |
| F15 | No SSE response-stream handling | HTTP path assumes single JSON object; no `text/event-stream` consumer for progress/notifications | Server MAY answer with SSE; client MUST support both `application/json` and `text/event-stream` | **HIGH** |
| F16 | Hardcoded JSON-RPC request IDs | Fixed IDs 1/2/3/0 reused across methods (`client/http.go:32,73,99,129,158,179,205`) | IDs must be unique per in-flight request; id reuse risks misrouting under SSE | MEDIUM |
| F17 | `404` retry guesses `/mcp` path | `httpFallbackURL` (`client/http.go:343-353`) | Modern servers return `404` + JSON-RPC `-32601` for unknown methods; do not blindly retry paths | LOW |

### 4.5 Authorization (OAuth 2.1) — the largest gap

Auth is entirely absent as a concept. `Authorization: Bearer` is never constructed in Go (only mentioned in `README.md`); the `headers` map is the sole credential vector.

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F18 | No OAuth client / token model | `ServerConfig` (`config/types.go:11-23`) has only `Headers map[string]string`; no auth/token/scope/refresh field | OAuth 2.1 client role; `Authorization: Bearer` on every HTTP request | **CRITICAL** |
| F19 | No Protected Resource Metadata discovery | `config/config.go:108-112` resolves `${VAR}` headers once at load | MCP clients MUST use RFC 9728 metadata for AS discovery (RFC 8414 / OIDC) | **CRITICAL** |
| F20 | No client registration | None | MUST obtain client ID via Client ID Metadata Documents (preferred), pre-registration, or DCR (deprecated) | **HIGH** |
| F21 | No PKCE / `resource` param / `iss` validation | None | RFC 8707 `resource` param REQUIRED; PKCE; RFC 9207 `iss` MUST be validated (SEP-2468) | **HIGH** |
| F22 | Credentials not keyed by issuer | Static headers; no per-issuer store | Clients MUST key credentials by issuer; MUST NOT reuse across AS; MUST re-register on AS change (SEP-2352) | **HIGH** |
| F23 | No step-up authorization / scope handling | None | Clients SHOULD handle `insufficient_scope` (403) via step-up flow | MEDIUM |
| F24 | stdio auth path conflated with HTTP | `Headers` applied to all transports | stdio SHOULD use environment credentials, not OAuth | MEDIUM |

### 4.6 MRTR and `resultType`

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F25 | No `resultType` on results; server-to-client calls modeled as standalone requests | `ToolResult` (`mcp/types.go:40-43`) has no `resultType`; `CreateMessage`/`RequestInput`/`ListRoots` are separate client methods (`protocol.go:24,27,30`) invoked by the *client* | `InputRequiredResult` (`resultType:"input_required"`) returned by server; client retries original request with `inputResponses` (SEP-2322) | **HIGH** |

> Note: `CreateMessage`/`RequestInput`/`ListRoots` are Sampling/Elicitation/Roots, all of which are also **deprecated** (4.8). They should not be ported to MRTR; they should be removed on the deprecation timeline.

### 4.7 Caching

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F26 | No `ttlMs` / `cacheScope` on list/read results | `Tool`, `Resource` result types lack fields; daemon caches tools indefinitely in `session.ToolCache` (`daemon/daemon.go:230,244`) | CacheableResult MUST carry `ttlMs` + `cacheScope` on `tools/list`, `prompts/list`, `resources/list`, `resources/read`, `resources/templates/list` (SEP-2549) | MEDIUM |

### 4.8 Deprecated features

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F27 | Implements all three deprecated features | `CreateMessage`/Sampling (`protocol.go:24`, `cli/commands.go:68-77`), `RequestInput`/Elicitation (`protocol.go:27`, `cli/commands.go:57-66`), `ListRoots`/`NotifyRootsListChanged` (`protocol.go:30,33`, `client/http.go:222`, `client/stdio.go:213`) | Roots/Sampling/Logging deprecated (SEP-2577); new implementations SHOULD NOT adopt; earliest removal 2027-07-28 | MEDIUM |

### 4.9 Extensions framework

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F28 | No extensions support | No `extensions` capability map; no Tasks (`tasks/get`, `tasks/update`) or MCP Apps handling | Extensions opt-in via capabilities; additive, not blocking. Tasks/Apps are opportunities, not collisions | LOW |

### 4.10 Daemon HTTP API surface

Not a protocol requirement, but a security concern surfaced by the audit.

| # | Finding | Current code | Spec requirement | Sev |
|---|---|---|---|---|
| F29 | Daemon API is unauthenticated custom REST | `daemon/server.go` endpoints (`/sessions`, `/sessions/{server}/call-tool/{tool}`) take no auth; bound `127.0.0.1:8080` / unix socket (`daemon/endpoint.go:26`, `platform.go:22,53`) | n/a (not MCP protocol) but violates least-privilege; any local process can drive sessions/tools. Daemon should carry its own auth boundary or be re-scoped as a pure host concern | **CRITICAL** |

---

## Severity rollup

| Area | CRIT | HIGH | MED | LOW |
|---|---|---|---|---|
| 4.1 Versioning / `_meta` | 2 | 1 | 0 | 1 |
| 4.2 Stateless core | 1 | 2 | 2 | 0 |
| 4.3 Negotiation / discover | 0 | 2 | 1 | 0 |
| 4.4 Streamable HTTP | 1 | 1 | 2 | 1 |
| 4.5 Authorization | 2 | 3 | 2 | 0 |
| 4.6 MRTR / resultType | 0 | 1 | 0 | 0 |
| 4.7 Caching | 0 | 0 | 1 | 0 |
| 4.8 Deprecated features | 0 | 0 | 1 | 0 |
| 4.9 Extensions | 0 | 0 | 0 | 1 |
| 4.10 Daemon security | 1 | 0 | 0 | 0 |
| **Total** | **7** | **10** | **9** | **3** |

---

## What already aligns (do not regress)

- **HTTP transport is per-request POST** (`client/http.go:282-340`). The transport layer is connection-stateless even though the session layer is not. This is the correct foundation; the gaps are missing headers/auth/SSE, not the wrong model.
- **`StatelessSession`** (`internal/session/stateless.go`) constructs a fresh client per call. Closest existing analogue to the 2026-07-28 core.
- **No `Mcp-Session-Id` anywhere.** Nothing to remove on the session-header axis.
- **`Origin`/localhost binding present** on the daemon. The spec's DNS-rebinding `Origin` check applies to MCP *servers*; mcp-cli-ent is a client, so this is informational.

---

## Suggested migration ordering (preview)

Full detail belongs in `docs/migration-plan-2026-07-28.md`. Ordering by dependency and risk:

1. **Foundation** (unblocks everything): `ProtocolVersion` constant + per-request `_meta` injection + `MCP-Protocol-Version` header. (F1, F2, F3)
2. **Era detection**: `server/discover` + modern/legacy fallback matrix. (F10, F11)
3. **HTTP transport completion**: `Mcp-Method`/`Mcp-Name` headers, SSE response handling, unique request IDs, `x-mcp-header`. (F13, F14, F15, F16, F17)
4. **Authorization**: OAuth 2.1 client, RFC 9728 discovery, PKCE, `resource`, token store keyed by issuer, Bearer injection. (F18-F24) — largest work item; defer stdio to env-cred path.
5. **Daemon re-scoping**: auth boundary on the local API, or demote to pure host concern. (F29, F8)
6. **Deprecation cleanup**: drop Roots/Sampling/Logging on timeline; do NOT port to MRTR. (F25, F27)
7. **Caching + extensions**: `ttlMs`/`cacheScope`; opt into Tasks/Apps as opportunities. (F26, F28)

---

## References

- Spec: [2026-07-28 index](https://modelcontextprotocol.io/specification/2026-07-28), [Key Changes](https://modelcontextprotocol.io/specification/2026-07-28/changelog), [Deprecated registry](https://modelcontextprotocol.io/specification/2026-07-28/deprecated), [Versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning), [Architecture](https://modelcontextprotocol.io/specification/2026-07-28/architecture/index)
- Transports: [Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http), [stdio](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio)
- Auth: [Authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/index), [AS Discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery), [Client Registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration)
- RPC: [`server/discover`](https://modelcontextprotocol.io/specification/2026-07-28/server/discover), [MRTR](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr)
- Extensions: [Overview](https://modelcontextprotocol.io/extensions/overview), [Tasks](https://modelcontextprotocol.io/extensions/tasks/overview), [MCP Apps](https://modelcontextprotocol.io/extensions/apps/overview)
- Announcement: [Bringing MCP 2026-07-28 to Claude](https://claude.com/blog/bringing-mcp-2026-07-28-to-claude)
- Key SEPs: 2575 (stateless), 2567 (sessionless handles), 2243 (HTTP headers), 2322 (MRTR), 2663 (Tasks), 2133 (extensions), 2577 (deprecate Roots/Sampling/Logging), 2549 (TTL), 2468 / 2352 (auth)
