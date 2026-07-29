package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// stdioDetectionTimeout bounds the one-time era probe over stdio.
const stdioDetectionTimeout = 10 * time.Second

// StdioClient implements MCPClient for stdio-based MCP servers
type StdioClient struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	reader        *bufio.Reader
	writer        *bufio.Writer
	closed        bool
	mutex         sync.Mutex
	pending       map[string]chan []byte // request id -> waiter, keyed by idKey
	readerDone    chan struct{}          // closed when readLoop exits
	readerStarted bool                   // true once readLoop is launched
	era           mcp.Era
	capabilities  mcp.ClientCapabilities
	detectOnce    sync.Once
}

// NewStdioClient creates a new stdio MCP client
func NewStdioClient(command string, args []string, env map[string]string, config *mcp.ClientConfig) (*StdioClient, error) {
	ctx := context.Background()

	// Create the command
	cmd := exec.CommandContext(ctx, command, args...)

	// Set up environment
	if len(env) > 0 {
		cmdEnv := os.Environ()
		for k, v := range env {
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = cmdEnv
	}

	// Create pipes for stdin/stdout/stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	era := mcp.EraUnknown
	if config != nil {
		era = mcp.ClassifyEra(config.ProtocolVersion)
	}

	client := &StdioClient{
		cmd:          cmd,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		reader:       bufio.NewReader(stdout),
		writer:       bufio.NewWriter(stdin),
		pending:      make(map[string]chan []byte),
		readerDone:   make(chan struct{}),
		era:          era,
		capabilities: mcp.ClientCapabilities{},
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// One goroutine owns the reader for the lifetime of the subprocess, so a
	// context cancellation never leaves a competing reader behind.
	client.readerStarted = true
	go client.readLoop()

	return client, nil
}

// ListTools retrieves available tools from the MCP server
func (c *StdioClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
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
func (c *StdioClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.ToolResult, error) {
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
func (c *StdioClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
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
func (c *StdioClient) Initialize(ctx context.Context, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
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
func (c *StdioClient) CreateMessage(ctx context.Context, request *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
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
func (c *StdioClient) RequestInput(ctx context.Context, params *mcp.RequestInputParams) (*mcp.RequestInputResult, error) {
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
func (c *StdioClient) ListRoots(ctx context.Context) ([]mcp.Root, error) {
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
func (c *StdioClient) NotifyRootsListChanged(roots []mcp.Root) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	params := map[string]interface{}{
		"roots": roots,
	}
	req := mcp.NewRequest(nil, "roots/list_changed", params)

	// For notifications, we send without expecting a response
	reqBytes, err := mcp.MarshalRequest(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Add newline for JSON-RPC over stdio
	reqBytes = append(reqBytes, '\n')

	// Write to stdin
	_, err = c.stdin.Write(reqBytes)
	if err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// Close closes the stdio client and terminates the subprocess
func (c *StdioClient) Close() error {
	c.mutex.Lock()
	if c.closed {
		c.mutex.Unlock()
		return nil
	}
	c.closed = true
	started := c.readerStarted
	c.mutex.Unlock()

	// Close stdin first so the server sees EOF, then stop the process; only then
	// close stdout, which unblocks the reader goroutine. Killing the process
	// first avoids closing stdout under an active reader in the common case.
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait() // Wait for process to actually terminate
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.stderr != nil {
		_ = c.stderr.Close()
	}

	// Wait for the reader goroutine to finish so none outlive Close.
	if started {
		<-c.readerDone
	}
	return nil
}

// sendRequest sends a JSON-RPC request to the stdio server
func (c *StdioClient) sendRequest(ctx context.Context, req *mcp.JSONRPCRequest) (interface{}, error) {
	// Detect era once for auto-pinned servers, then shape the request.
	c.detectOnce.Do(func() {
		if c.era == mcp.EraUnknown {
			c.detectEra(ctx)
		}
	})
	if c.era == mcp.EraModern {
		if err := mcp.InjectMeta(req, c.capabilities); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	line, err := c.roundTrip(ctx, req)
	if err != nil {
		return nil, err
	}

	rpcResp, err := mcp.UnmarshalResponse(line)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// detectEra probes the server once with a modern server/discover request and
// caches the verdict. Best-effort: any failure defaults to legacy and is logged.
func (c *StdioClient) detectEra(ctx context.Context) {
	probe := mcp.NewRequest("era-probe", "server/discover", nil)
	if err := mcp.InjectMeta(probe, c.capabilities); err != nil {
		c.fallbackLegacy("inject _meta: %v", err)
		return
	}
	pctx, cancel := context.WithTimeout(ctx, stdioDetectionTimeout)
	defer cancel()
	line, err := c.roundTrip(pctx, probe)
	if err != nil {
		c.fallbackLegacy("probe: %v", err)
		return
	}
	rpcResp, err := mcp.UnmarshalResponse(line)
	if err != nil {
		c.fallbackLegacy("parse probe response: %v", err)
		return
	}
	c.era = mcp.ClassifyProbeResponse(rpcResp)
}

// fallbackLegacy sets the era to legacy and logs the reason, so a transient
// detection failure against a modern server leaves a breadcrumb rather than a
// silent misroute to the legacy path.
func (c *StdioClient) fallbackLegacy(format string, args ...interface{}) {
	c.era = mcp.EraLegacy
	if Verbose {
		log.Printf("mcp: stdio era detection failed (%s); assuming legacy", fmt.Sprintf(format, args...))
	}
}

// roundTrip writes req, waits for its matching response line, and returns it.
// Writes are serialized via c.mutex; the read is handled by readLoop, so a
// context cancellation unregisters the waiter instead of leaking a goroutine
// that competes for the shared reader.
func (c *StdioClient) roundTrip(ctx context.Context, req *mcp.JSONRPCRequest) ([]byte, error) {
	reqBytes, err := mcp.MarshalRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	key := idKey(req.ID)
	ch := make(chan []byte, 1)

	c.mutex.Lock()
	if c.closed {
		c.mutex.Unlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.pending[key] = ch
	if _, err := c.writer.Write(reqBytes); err != nil {
		delete(c.pending, key)
		c.mutex.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}
	if err := c.writer.Flush(); err != nil {
		delete(c.pending, key)
		c.mutex.Unlock()
		return nil, fmt.Errorf("failed to flush request: %w", err)
	}
	c.mutex.Unlock()

	select {
	case <-ctx.Done():
		c.mutex.Lock()
		delete(c.pending, key)
		c.mutex.Unlock()
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	case line, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}
		return line, nil
	}
}

// readLoop owns c.reader exclusively for the lifetime of the subprocess. It
// reads newline-delimited JSON-RPC messages and dispatches responses to the
// waiting caller by request id. Notifications and unparseable lines are
// skipped. On a read error (including process exit) it fails all pending
// waiters and exits.
func (c *StdioClient) readLoop() {
	defer close(c.readerDone)
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			c.failPending()
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		rpcResp, perr := mcp.UnmarshalResponse(line)
		if perr != nil {
			continue
		}
		if rpcResp.ID == nil {
			continue // server-to-client notification; not a response
		}
		key := idKey(rpcResp.ID)
		c.mutex.Lock()
		ch, ok := c.pending[key]
		if ok {
			delete(c.pending, key)
		}
		c.mutex.Unlock()
		if ok {
			select {
			case ch <- line:
			default: // waiter already gone (timed out); drop
			}
		}
	}
}

// failPending wakes every waiting caller after the reader exits.
func (c *StdioClient) failPending() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for key, ch := range c.pending {
		close(ch)
		delete(c.pending, key)
	}
}

// idKey returns a canonical key for a JSON-RPC request id, so an int request id
// (e.g. 1) matches its float64 echo (1.0) in the response.
func idKey(id interface{}) string {
	b, err := json.Marshal(id)
	if err != nil {
		return ""
	}
	return string(b)
}
