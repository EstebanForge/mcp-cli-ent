package mcp

// Era detection helpers for the dual-era client (MCP 2026-07-28).
//
// The client sends a modern server/discover probe carrying per-request _meta.
// A response that is a successful result or a recognized modern error identifies
// a modern (stateless) server; any other response identifies a legacy
// (initialize-handshake) server. See docs/migration-plan-2026-07-28.md.

// ClassifyProbeResponse classifies the JSON-RPC response to a modern probe.
// A success (e.g. a DiscoverResult) or a recognized modern error => EraModern;
// any other error, or a nil response => EraLegacy.
func ClassifyProbeResponse(resp *JSONRPCResponse) Era {
	if resp == nil {
		return EraLegacy
	}
	if resp.Error == nil {
		return EraModern // success: the server answered a modern request
	}
	switch resp.Error.Code {
	case UnsupportedProtocolVersion, HeaderMismatchError, MissingRequiredClientCapability:
		return EraModern // modern server rejected the request for a modern reason
	}
	return EraLegacy // -32601 method-not-found, -32600, -32000 session-required, etc.
}

// ClassifyHTTPProbe classifies an HTTP probe response body into an Era. A body
// that parses as a modern JSON-RPC result/error => EraModern; an empty or
// unparseable body (a typical legacy rejection) => EraLegacy. The HTTP status
// code is not authoritative (400 with a modern error body is still modern).
func ClassifyHTTPProbe(body []byte) Era {
	if len(body) > 0 {
		if resp, err := UnmarshalResponse(body); err == nil {
			return ClassifyProbeResponse(resp)
		}
	}
	return EraLegacy
}
