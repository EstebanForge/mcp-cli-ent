package client

// Tests for the stdio transport's initialize handshake, notification wire
// format, and per-request timeout honoring.
//
// The stub is a real subprocess speaking line-delimited JSON-RPC over stdio,
// built from source below. It mirrors servers (like the Automattic WordPress
// bridge) that queue MCP-protocol requests until the initialize handshake
// completes, while answering unknown methods with -32601 immediately.

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

const strictServerSrc = `package main

// Strict stdio MCP server for client tests. Queues MCP-protocol requests
// until notifications/initialized arrives (strict mode, default); loose mode
// (STUB_MODE=loose) answers every request immediately. Unknown methods get
// -32601 in both modes. STUB_DELAY_TOOLS=<seconds> delays tools/list answers.
// After every received message it rewrites a JSON summary to argv[1]:
// [{"method":"...","hasId":true|false}, ...]

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"time"
)

type incoming struct {
	JSONRPC string
	ID      *json.RawMessage
	Method  string
}

func main() {
	summaryPath := os.Args[1]
	mode := os.Getenv("STUB_MODE")
	delay := 0.0
	if d := os.Getenv("STUB_DELAY_TOOLS"); d != "" {
		delay, _ = strconv.ParseFloat(d, 64)
	}

	var received []map[string]interface{}
	var initialized bool
	var queued []incoming

	writeSummary := func() {
		b, _ := json.Marshal(received)
		_ = os.WriteFile(summaryPath, b, 0644)
	}

	answer := func(w *bufio.Writer, m incoming) {
		if delay > 0 {
			time.Sleep(time.Duration(delay * float64(time.Second)))
		}
		var resp map[string]interface{}
		if m.Method == "tools/list" {
			resp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      m.ID,
				"result":  map[string]interface{}{"tools": []map[string]string{{"name": "stub-tool", "description": "stub"}}},
			}
		} else {
			resp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      m.ID,
				"error":   map[string]interface{}{"code": -32601, "message": "Method not found"},
			}
		}
		b, _ := json.Marshal(resp)
		w.Write(append(b, '\n'))
		w.Flush()
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)

	for in.Scan() {
		line := bytes.TrimSpace(in.Bytes())
		if len(line) == 0 {
			continue
		}
		var m incoming
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		received = append(received, map[string]interface{}{
			"method": m.Method,
			"hasId":  m.ID != nil,
		})
		writeSummary()

		switch {
		case m.Method == "server/discover" && m.ID != nil:
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      m.ID,
				"error":   map[string]interface{}{"code": -32601, "message": "Method not found"},
			}
			b, _ := json.Marshal(resp)
			out.Write(append(b, '\n'))
			out.Flush()
		case m.Method == "initialize":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      m.ID,
				"result": map[string]interface{}{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
					"serverInfo":      map[string]interface{}{"name": "strict-stub", "version": "1.0.0"},
				},
			}
			b, _ := json.Marshal(resp)
			out.Write(append(b, '\n'))
			out.Flush()
		case m.Method == "notifications/initialized":
			// A spec-correct notification carries no id member. A message
			// with "id":null is neither request nor notification per spec;
			// strict servers drop it, leaving the handshake incomplete.
			if m.ID != nil {
				continue
			}
			initialized = true
			for _, q := range queued {
				answer(out, q)
			}
			queued = nil
		case m.ID != nil:
			if mode != "loose" && !initialized {
				queued = append(queued, m)
				continue
			}
			answer(out, m)
		}
	}
	writeSummary()
}
`

// buildStubServer compiles the stub source into a binary and returns its
// path plus the summary path to pass as argv[1].
func buildStubServer(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "strictserver.go")
	if err := os.WriteFile(src, []byte(strictServerSrc), 0644); err != nil {
		t.Fatalf("write stub source: %v", err)
	}
	bin := filepath.Join(dir, "strictserver-bin")
	summary := filepath.Join(dir, "summary.json")
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build stub server: %v\n%s", err, out)
	}
	return bin, summary
}

type stubEntry struct {
	Method string `json:"method"`
	HasID  bool   `json:"hasId"`
}

func readStubSummary(t *testing.T, summaryPath string) []stubEntry {
	t.Helper()
	b, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read stub summary: %v", err)
	}
	var entries []stubEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		t.Fatalf("parse stub summary: %v", err)
	}
	return entries
}

func methodsOf(entries []stubEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Method
	}
	return out
}

func joinMethods(entries []stubEntry) string {
	return strings.Join(methodsOf(entries), ", ")
}

func assertToolList(t *testing.T, tools []mcp.Tool, label string) {
	t.Helper()
	if len(tools) != 1 || tools[0].Name != "stub-tool" {
		t.Fatalf("%s: unexpected tools: %+v", label, tools)
	}
}

// TestListToolsAutoHandshakesStrictServer: a server that queues requests
// until initialize must be answered after era auto-detection, with a
// spec-correct handshake on the wire (notifications/initialized carries no
// id member). Break it guards: removing the handshake re-hangs list-tools
// for 30s (pre-fix behavior); sending "id":null notifications drops the
// initialized signal on TS-SDK servers and re-hangs the first call.
func TestListToolsAutoHandshakesStrictServer(t *testing.T) {
	bin, summary := buildStubServer(t)
	c, err := NewStdioClient(bin, []string{summary}, nil, &mcp.ClientConfig{ProtocolVersion: "auto"})
	if err != nil {
		t.Fatalf("NewStdioClient: %v", err)
	}
	defer func() { _ = c.Close() }()

	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools on strict server: %v", err)
	}
	assertToolList(t, tools, "strict server")

	entries := readStubSummary(t, summary)
	want := []string{"server/discover", "initialize", "notifications/initialized", "tools/list"}
	got := methodsOf(entries)
	if len(got) != len(want) {
		t.Fatalf("wire sequence = [%s], want [%s]", joinMethods(entries), strings.Join(want, ", "))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wire sequence = [%s], want [%s]", joinMethods(entries), strings.Join(want, ", "))
		}
	}
	if entries[2].HasID {
		t.Errorf("notifications/initialized carried an id member; spec-correct notifications omit it")
	}
}

// TestListToolsPinnedLegacyHandshakesStrictServer: with protocolVersion pinned
// to a legacy version, no era probe may be sent and the handshake must still
// run before tools/list.
func TestListToolsPinnedLegacyHandshakesStrictServer(t *testing.T) {
	bin, summary := buildStubServer(t)
	c, err := NewStdioClient(bin, []string{summary}, nil, &mcp.ClientConfig{ProtocolVersion: "2025-06-18"})
	if err != nil {
		t.Fatalf("NewStdioClient: %v", err)
	}
	defer func() { _ = c.Close() }()

	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools on strict server (pinned legacy): %v", err)
	}
	assertToolList(t, tools, "pinned legacy")

	entries := readStubSummary(t, summary)
	got := methodsOf(entries)
	if len(got) == 0 || got[0] != "initialize" {
		t.Fatalf("first wire message = %v, want initialize (no probe expected when era is pinned)", got)
	}
	for _, m := range got {
		if m == "server/discover" {
			t.Fatalf("era probe sent despite pinned legacy version: [%s]", joinMethods(entries))
		}
	}
}

// TestListToolsHandshakeOncePerClient: repeated calls on one client must not
// re-initialize; servers error on a second initialize. Break it guards: a
// per-request handshake (instead of once per client) breaks the second call
// and hammers strict servers.
func TestListToolsHandshakeOncePerClient(t *testing.T) {
	bin, summary := buildStubServer(t)
	c, err := NewStdioClient(bin, []string{summary}, nil, &mcp.ClientConfig{ProtocolVersion: "auto"})
	if err != nil {
		t.Fatalf("NewStdioClient: %v", err)
	}
	defer func() { _ = c.Close() }()

	for i := 0; i < 2; i++ {
		tools, err := c.ListTools(context.Background())
		if err != nil {
			t.Fatalf("ListTools call %d: %v", i+1, err)
		}
		assertToolList(t, tools, "repeat call")
	}

	entries := readStubSummary(t, summary)
	inits, lists := 0, 0
	for _, e := range entries {
		switch e.Method {
		case "initialize":
			inits++
		case "tools/list":
			lists++
		}
	}
	if inits != 1 {
		t.Fatalf("initialize sent %d times on one client, want exactly 1: [%s]", inits, joinMethods(entries))
	}
	if lists != 2 {
		t.Fatalf("tools/list sent %d times, want 2: [%s]", lists, joinMethods(entries))
	}
}

// TestStdioRequestHonorsConfigTimeout: a client configured with timeout=1 must
// give up on a slow tools/list after ~1s, not after the hardcoded 30s. Break
// it guards: dropping config.Timeout from the stdio client restores the fixed
// 30s request window and makes the configured value a lie.
func TestStdioRequestHonorsConfigTimeout(t *testing.T) {
	bin, summary := buildStubServer(t)
	t.Setenv("STUB_MODE", "loose")
	t.Setenv("STUB_DELAY_TOOLS", "3")

	c, err := NewStdioClient(bin, []string{summary}, nil, &mcp.ClientConfig{Timeout: 1, ProtocolVersion: "auto"})
	if err != nil {
		t.Fatalf("NewStdioClient: %v", err)
	}
	defer func() { _ = c.Close() }()

	start := time.Now()
	_, err = c.ListTools(context.Background())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("ListTools succeeded despite timeout=1 and 3s server delay (took %s)", elapsed)
	}
	if !strings.Contains(err.Error(), "request timeout") {
		t.Fatalf("error = %v, want request timeout", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("timeout took %s; configured 1s timeout was not honored", elapsed)
	}
}
