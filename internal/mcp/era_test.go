package mcp

import "testing"

func TestClassifyEra(t *testing.T) {
	cases := []struct {
		v    string
		want Era
	}{
		{"2026-07-28", EraModern},
		{"2025-11-25", EraLegacy},
		{"2025-06-18", EraLegacy},
		{"2025-03-26", EraLegacy},
		{"2024-11-05", EraLegacy},
		{"", EraUnknown},
		{"auto", EraUnknown},
		{"2099-01-01", EraUnknown},
	}
	for _, c := range cases {
		if got := ClassifyEra(c.v); got != c.want {
			t.Errorf("ClassifyEra(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestIsKnownVersion(t *testing.T) {
	for _, v := range KnownVersions {
		if !IsKnownVersion(v) {
			t.Errorf("IsKnownVersion(%q) = false, want true", v)
		}
	}
	if IsKnownVersion("2099-01-01") {
		t.Error(`IsKnownVersion("2099-01-01") = true, want false`)
	}
}

func TestClassifyProbeResponse(t *testing.T) {
	if got := ClassifyProbeResponse(nil); got != EraLegacy {
		t.Errorf("nil response => %v, want EraLegacy", got)
	}
	// A success (DiscoverResult) identifies a modern server.
	if got := ClassifyProbeResponse(&JSONRPCResponse{ID: 1, Result: "x"}); got != EraModern {
		t.Errorf("success response => %v, want EraModern", got)
	}
	// Recognized modern errors identify a modern server.
	for _, code := range []int{UnsupportedProtocolVersion, HeaderMismatchError, MissingRequiredClientCapability} {
		got := ClassifyProbeResponse(&JSONRPCResponse{ID: 1, Error: &JSONRPCError{Code: code}})
		if got != EraModern {
			t.Errorf("modern error %d => %v, want EraModern", code, got)
		}
	}
	// Legacy-shaped errors identify a legacy server (codes observed live).
	for _, code := range []int{MethodNotFound, -32600, -32000} {
		got := ClassifyProbeResponse(&JSONRPCResponse{ID: 1, Error: &JSONRPCError{Code: code}})
		if got != EraLegacy {
			t.Errorf("legacy error %d => %v, want EraLegacy", code, got)
		}
	}
}

func TestClassifyHTTPProbe(t *testing.T) {
	if got := ClassifyHTTPProbe(nil); got != EraLegacy {
		t.Errorf("empty body => %v, want EraLegacy", got)
	}
	if got := ClassifyHTTPProbe([]byte("not json")); got != EraLegacy {
		t.Errorf("garbage body => %v, want EraLegacy", got)
	}
	// Modern error in a 400 body => modern (server rejected for a modern reason).
	body := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32022,"message":"unsupported"}}`)
	if got := ClassifyHTTPProbe(body); got != EraModern {
		t.Errorf("modern error body => %v, want EraModern", got)
	}
	// DiscoverResult success => modern.
	body = []byte(`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`)
	if got := ClassifyHTTPProbe(body); got != EraModern {
		t.Errorf("discover body => %v, want EraModern", got)
	}
	// context7's observed "No valid session ID" (-32000) => legacy.
	body = []byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"No valid session ID"},"id":null}`)
	if got := ClassifyHTTPProbe(body); got != EraLegacy {
		t.Errorf("context7 -32000 body => %v, want EraLegacy", got)
	}
}

func TestInjectMeta(t *testing.T) {
	req := NewRequest(1, "tools/call", &CallToolParams{Name: "get_weather", Arguments: map[string]interface{}{"loc": "SEA"}})
	if err := InjectMeta(req, ClientCapabilities{}); err != nil {
		t.Fatalf("InjectMeta: %v", err)
	}
	pm, ok := req.Params.(map[string]interface{})
	if !ok {
		t.Fatalf("params not converted to map after InjectMeta")
	}
	meta, ok := pm["_meta"].(map[string]interface{})
	if !ok {
		t.Fatal("_meta not set")
	}
	if meta[MetaKeyProtocolVersion] != ProtocolVersion {
		t.Errorf("protocolVersion = %v, want %s", meta[MetaKeyProtocolVersion], ProtocolVersion)
	}
	if _, ok := meta[MetaKeyClientInfo].(ClientInfo); !ok {
		t.Error("clientInfo not a ClientInfo struct")
	}
	if _, ok := meta[MetaKeyClientCapabilities]; !ok {
		t.Error("clientCapabilities missing")
	}
	if pm["name"] != "get_weather" {
		t.Errorf("name lost after inject: %v", pm["name"])
	}
	// NameForRequest reads params.name from the (now map) params.
	if got := NameForRequest(req); got != "get_weather" {
		t.Errorf("NameForRequest = %q, want get_weather", got)
	}
}
