package client

// Tests for the HTTP transport's bounded 429 retry. The stub is an
// httptest.Server, so the real HTTP client exercises real network calls.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

func legacyHTTPClient(t *testing.T, url string) *HTTPClient {
	t.Helper()
	// Pinned legacy era skips the server/discover probe, so tools/list is the
	// first wire request and the 429 counter is deterministic.
	return NewHTTPClient(url, &mcp.ClientConfig{Timeout: 15, ProtocolVersion: "2025-06-18"})
}

// TestListToolsRetriesOnceOn429: a 429 with Retry-After must be retried once
// and succeed. Break it guards: removing the retry turns any transient WAF
// rate limit into a hard failure for HTTP-direct servers.
func TestListToolsRetriesOnceOn429(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"tools": []map[string]string{{"name": "t1"}}},
		})
	}))
	defer srv.Close()

	c := legacyHTTPClient(t, srv.URL)
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools after 429 retry: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "t1" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server hit %d times, want exactly 2 (initial + one retry)", got)
	}
}

// TestListToolsStopsAfterOne429Retry: the retry is bounded; a persistently
// rate-limiting server gets exactly one retry, then the 429 surfaces as an
// error. Break it guards: an unbounded retry loop would hammer a rate limiter
// and prolong the WAF cooldown window.
func TestListToolsStopsAfterOne429Retry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := legacyHTTPClient(t, srv.URL)
	start := time.Now()
	_, err := c.ListTools(context.Background())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("ListTools succeeded against persistent 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("error = %v, want it to mention 429", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server hit %d times, want exactly 2 (no unbounded retry)", got)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("retry took %s; Retry-After was 1s", elapsed)
	}
}
