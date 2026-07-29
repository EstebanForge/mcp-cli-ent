package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// HTTPClient implements MCPClient for HTTP-based MCP servers
type HTTPClient struct {
	client       *http.Client
	baseURL      string
	headers      map[string]string
	timeout      time.Duration
	era          mcp.Era
	capabilities mcp.ClientCapabilities
	once         sync.Once
}

// NewHTTPClient creates a new HTTP MCP client
func NewHTTPClient(url string, config *mcp.ClientConfig) *HTTPClient {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:      url,
		headers:      config.Headers,
		timeout:      timeout,
		era:          mcp.ClassifyEra(config.ProtocolVersion), // Unknown => auto-detect
		capabilities: mcp.ClientCapabilities{},
	}
}

// ListTools retrieves available tools from the MCP server
func (c *HTTPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	req := mcp.NewRequest(1, "tools/list", nil)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var listResult mcp.ListToolsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools list result: %w", err)
	}

	return listResult.Tools, nil
}

// CallTool executes a specific tool on the MCP server
func (c *HTTPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.ToolResult, error) {
	params := &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	req := mcp.NewRequest(2, "tools/call", params)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var toolResult mcp.ToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return &toolResult, nil
}

// ListResources retrieves available resources from the MCP server
func (c *HTTPClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	req := mcp.NewRequest(3, "resources/list", nil)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var listResult mcp.ListResourcesResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources list result: %w", err)
	}

	return listResult.Resources, nil
}

// Initialize the MCP connection
func (c *HTTPClient) Initialize(ctx context.Context, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
	req := mcp.NewRequest(0, "initialize", params)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var initResult mcp.InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	return &initResult, nil
}

// CreateMessage handles sampling requests
func (c *HTTPClient) CreateMessage(ctx context.Context, request *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	req := mcp.NewRequest(0, "sampling/createMessage", request)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var messageResult mcp.CreateMessageResult
	if err := json.Unmarshal(resultBytes, &messageResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message result: %w", err)
	}

	return &messageResult, nil
}

// RequestInput handles elicitation requests
func (c *HTTPClient) RequestInput(ctx context.Context, params *mcp.RequestInputParams) (*mcp.RequestInputResult, error) {
	req := mcp.NewRequest(0, "elicitation/requestInput", params)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to request input: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var inputResult mcp.RequestInputResult
	if err := json.Unmarshal(resultBytes, &inputResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input result: %w", err)
	}

	return &inputResult, nil
}

// ListRoots retrieves filesystem roots
func (c *HTTPClient) ListRoots(ctx context.Context) ([]mcp.Root, error) {
	req := mcp.NewRequest(0, "roots/list", nil)

	result, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list roots: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no result received")
	}

	// Parse the result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var listResult struct {
		Roots []mcp.Root `json:"roots"`
	}
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal roots list result: %w", err)
	}

	return listResult.Roots, nil
}

// NotifyRootsListChanged sends notification about roots change
func (c *HTTPClient) NotifyRootsListChanged(roots []mcp.Root) error {
	params := map[string]interface{}{
		"roots": roots,
	}
	req := mcp.NewRequest(nil, "roots/list_changed", params)

	// For notifications, we send without expecting a response
	reqBytes, err := mcp.MarshalRequest(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	ctx := context.Background()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}

	// Send notification (fire and forget)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

// Close closes the HTTP client
func (c *HTTPClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// sendRequest sends a JSON-RPC request to the MCP server
func (c *HTTPClient) sendRequest(ctx context.Context, req *mcp.JSONRPCRequest) (interface{}, error) {
	// Detect era once for auto-pinned servers, then shape the request.
	c.once.Do(func() {
		if c.era == mcp.EraUnknown {
			c.detectEra(ctx)
		}
	})
	if c.era == mcp.EraModern {
		if err := mcp.InjectMeta(req, c.capabilities); err != nil {
			return nil, err
		}
	}
	return c.sendRequestWithURL(ctx, req, c.baseURL, false)
}

// detectEra probes the server once with a modern server/discover request and
// caches the verdict. Best-effort: on transport failure it defaults to legacy.
func (c *HTTPClient) detectEra(ctx context.Context) {
	probe := mcp.NewRequest("era-probe", "server/discover", nil)
	if err := mcp.InjectMeta(probe, c.capabilities); err != nil {
		c.fallbackLegacy("inject _meta: %v", err)
		return
	}
	reqBytes, err := mcp.MarshalRequest(probe)
	if err != nil {
		c.fallbackLegacy("marshal probe: %v", err)
		return
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		c.fallbackLegacy("build probe request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion)
	httpReq.Header.Set(mcp.HeaderMethod, "server/discover")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.fallbackLegacy("probe request: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// A server that answers a modern probe with an SSE stream is modern; do not
	// ReadAll an unbounded stream. Otherwise read the (small) JSON body.
	if isEventStream(resp.Header.Get("Content-Type")) {
		c.era = mcp.EraModern
		return
	}
	body, _ := io.ReadAll(resp.Body)
	c.era = mcp.ClassifyHTTPProbe(body)
}

// fallbackLegacy sets the era to legacy and logs the reason, so a transient
// detection failure against a modern server leaves a breadcrumb rather than a
// silent misroute to the legacy path.
func (c *HTTPClient) fallbackLegacy(format string, args ...interface{}) {
	c.era = mcp.EraLegacy
	if Verbose {
		log.Printf("mcp: http era detection failed (%s); assuming legacy", fmt.Sprintf(format, args...))
	}
}

// isEventStream reports whether a Content-Type header denotes an SSE response.
func isEventStream(contentType string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(contentType)), "text/event-stream")
}

func (c *HTTPClient) sendRequestWithURL(ctx context.Context, req *mcp.JSONRPCRequest, urlStr string, triedFallback bool) (interface{}, error) {
	// Marshal the request
	reqBytes, err := mcp.MarshalRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	// Modern (2026-07-28) routing headers required on Streamable HTTP.
	if c.era == mcp.EraModern {
		httpReq.Header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion)
		httpReq.Header.Set(mcp.HeaderMethod, req.Method)
		if name := mcp.NameForRequest(req); name != "" {
			httpReq.Header.Set(mcp.HeaderName, name)
		}
	}

	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}

	// Send request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound && !triedFallback {
			if fallbackURL, ok := httpFallbackURL(urlStr); ok {
				return c.sendRequestWithURL(ctx, req, fallbackURL, true)
			}
		}
		return nil, fmt.Errorf("HTTP error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	// Unmarshal JSON-RPC response
	rpcResp, err := mcp.UnmarshalResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func httpFallbackURL(urlStr string) (string, bool) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", false
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", false
	}
	parsed.Path = "/mcp"
	return parsed.String(), true
}
