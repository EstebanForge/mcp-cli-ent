package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// StdioClient implements MCPClient for stdio-based MCP servers
type StdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	reader *bufio.Reader
	writer *bufio.Writer
	closed bool
	mutex  sync.Mutex
}

// NewStdioClient creates a new stdio MCP client
func NewStdioClient(command string, args []string, env map[string]string) (*StdioClient, error) {
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
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &StdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		reader: bufio.NewReader(stdout),
		writer: bufio.NewWriter(stdin),
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return client, nil
}

// ListTools retrieves available tools from the MCP server
func (c *StdioClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	req := mcp.NewRequest(1, "tools/list", &mcp.ListToolsParams{})

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
	req := mcp.NewRequest(3, "resources/list", &mcp.ListResourcesParams{})

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
	defer c.mutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Close pipes
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.stderr != nil {
		c.stderr.Close()
	}

	// Terminate the process
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait() // Wait for process to actually terminate
	}

	return nil
}

// sendRequest sends a JSON-RPC request to the stdio server
func (c *StdioClient) sendRequest(ctx context.Context, req *mcp.JSONRPCRequest) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Create a context with timeout for the operation
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Marshal the request
	reqBytes, err := mcp.MarshalRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Add newline for JSON-RPC over stdio
	reqBytes = append(reqBytes, '\n')

	// Send request
	if _, err := c.writer.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	if err := c.writer.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush request: %w", err)
	}

	// Read response with context
	responseChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	go func() {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			errorChan <- fmt.Errorf("failed to read response: %w", err)
			return
		}
		responseChan <- line
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	case err := <-errorChan:
		return nil, err
	case line := <-responseChan:
		// Unmarshal JSON-RPC response
		rpcResp, err := mcp.UnmarshalResponse(line)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
		}

		// Check for JSON-RPC error
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
		}

		return rpcResp.Result, nil
	}
}
