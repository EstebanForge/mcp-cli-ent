package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/client"
	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

// DaemonClient provides communication with the MCP daemon
type DaemonClient struct {
	manager    *DaemonManager
	httpClient *http.Client
	autoStart  bool
}

// NewDaemonClient creates a new daemon client
func NewDaemonClient() *DaemonClient {
	return &DaemonClient{
		manager:    NewDaemonManager(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		autoStart:  true,
	}
}

// IsDaemonRunning checks if the daemon is available
func (dc *DaemonClient) IsDaemonRunning() bool {
	running, _, err := isDaemonRunning()
	return err == nil && running
}

// StartDaemon starts the daemon if auto-start is enabled
func (dc *DaemonClient) StartDaemon() error {
	if !dc.autoStart {
		return fmt.Errorf("daemon auto-start is disabled")
	}

	if dc.IsDaemonRunning() {
		return nil // Already running
	}

	return dc.manager.Start(false)
}

// GetStatus gets the daemon status
func (dc *DaemonClient) GetStatus() (*DaemonStatus, error) {
	if !dc.IsDaemonRunning() {
		return &DaemonStatus{Running: false}, nil
	}

	// Try to get detailed status from daemon
	if isUnixSocket(dc.manager.endpoint) || isNamedPipe(dc.manager.endpoint) {
		// For non-HTTP endpoints, return basic status
		running, pid, _ := isDaemonRunning()
		return &DaemonStatus{
			Running:  running,
			PID:      pid,
			Platform: dc.manager.platform,
			Endpoint: dc.manager.endpoint,
		}, nil
	}

	// For HTTP endpoints, get detailed status
	resp, err := dc.httpClient.Get(dc.getHTTPURL())
	if err != nil {
		return &DaemonStatus{Running: false}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return &DaemonStatus{Running: false}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &DaemonStatus{Running: false}, nil
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return &DaemonStatus{Running: false}, nil
	}

	if !apiResp.Success {
		return &DaemonStatus{Running: false}, nil
	}

	data, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}
	var status DaemonStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return &status, nil
}

// StartSession starts a new persistent session
func (dc *DaemonClient) StartSession(serverName string, serverConfig config.ServerConfig) error {
	if !dc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running")
	}

	req := struct {
		Config config.ServerConfig `json:"config"`
	}{
		Config: serverConfig,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := dc.httpClient.Post(
		dc.getSessionURL(serverName, "start"),
		"application/json",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if !apiResp.Success {
		return fmt.Errorf("daemon error: %s", apiResp.Error)
	}

	return nil
}

// StopSession stops a persistent session
func (dc *DaemonClient) StopSession(serverName string) error {
	if !dc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running")
	}

	req, err := http.NewRequest("DELETE", dc.getSessionURL(serverName, ""), nil)
	if err != nil {
		return err
	}

	resp, err := dc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if !apiResp.Success {
		return fmt.Errorf("daemon error: %s", apiResp.Error)
	}

	return nil
}

// ListSessions lists all sessions
func (dc *DaemonClient) ListSessions() ([]SessionInfo, error) {
	if !dc.IsDaemonRunning() {
		return []SessionInfo{}, nil
	}

	resp, err := dc.httpClient.Get(dc.getSessionsURL())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("daemon error: %s", apiResp.Error)
	}

	data, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}
	var sessions []SessionInfo
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sessions: %w", err)
	}

	return sessions, nil
}

// CallTool executes a tool via the daemon
func (dc *DaemonClient) CallTool(serverName, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
	if !dc.IsDaemonRunning() {
		return nil, fmt.Errorf("daemon is not running")
	}

	req := struct {
		Args map[string]interface{} `json:"args"`
	}{
		Args: args,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := dc.httpClient.Post(
		dc.getToolURL(serverName, toolName),
		"application/json",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("daemon error: %s", apiResp.Error)
	}

	data, _ := json.Marshal(apiResp.Data)
	var result mcp.ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListTools lists tools for a session via the daemon
func (dc *DaemonClient) ListTools(serverName string) ([]mcp.Tool, error) {
	if !dc.IsDaemonRunning() {
		return nil, fmt.Errorf("daemon is not running")
	}

	resp, err := dc.httpClient.Post(
		dc.getSessionURL(serverName, "tools"),
		"application/json",
		bytes.NewBuffer([]byte("{}")),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("daemon error: %s", apiResp.Error)
	}

	data, _ := json.Marshal(apiResp.Data)
	var tools []mcp.Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, err
	}

	return tools, nil
}

// SmartClient provides automatic daemon usage with fallback
type SmartClient struct {
	daemonClient *DaemonClient
	directClient func(config.ServerConfig) (mcp.MCPClient, error)
}

// NewSmartClient creates a new smart client
func NewSmartClient() *SmartClient {
	return &SmartClient{
		daemonClient: NewDaemonClient(),
		directClient: client.NewMCPClient,
	}
}

// ShouldUseDaemon determines if a server should use the daemon
func (sc *SmartClient) ShouldUseDaemon(serverName string, serverConfig config.ServerConfig) bool {
	// Don't use daemon if explicitly disabled
	if !serverConfig.Persistent {
		return false
	}

	// Use daemon if it's running
	if sc.daemonClient.IsDaemonRunning() {
		return true
	}

	// Auto-start daemon for persistent servers
	if serverConfig.Persistent {
		if err := sc.daemonClient.StartDaemon(); err == nil {
			return true
		}
	}

	return false
}

// CreateClient creates an MCP client, using daemon when appropriate
func (sc *SmartClient) CreateClient(serverName string, serverConfig config.ServerConfig) (mcp.MCPClient, error) {
	if sc.ShouldUseDaemon(serverName, serverConfig) {
		return NewDaemonMCPClient(sc.daemonClient, serverName), nil
	}

	// Fall back to direct client
	return sc.directClient(serverConfig)
}

// DaemonMCPClient is an MCP client that communicates with the daemon
type DaemonMCPClient struct {
	daemonClient *DaemonClient
	serverName   string
}

// NewDaemonMCPClient creates a new daemon MCP client
func NewDaemonMCPClient(daemonClient *DaemonClient, serverName string) *DaemonMCPClient {
	return &DaemonMCPClient{
		daemonClient: daemonClient,
		serverName:   serverName,
	}
}

// Initialize implements the MCPClient interface
func (dm *DaemonMCPClient) Initialize(ctx context.Context, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
	// Daemon doesn't need explicit initialization - sessions are started on demand
	return &mcp.InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: mcp.ServerCapabilities{
			Tools: &mcp.ToolsCapability{},
		},
		ServerInfo: mcp.ServerInfo{
			Name:    "mcp-cli-ent-daemon",
			Version: version.Version,
		},
	}, nil
}

// ListTools implements the MCPClient interface
func (dm *DaemonMCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	tools, err := dm.daemonClient.ListTools(dm.serverName)
	if err != nil {
		// Try to start the session if it doesn't exist
		if config, loadErr := LoadMCPConfig(); loadErr == nil {
			if serverConfig, exists := config.MCPServers[dm.serverName]; exists {
				if startErr := dm.daemonClient.StartSession(dm.serverName, serverConfig); startErr == nil {
					// Give it a moment to start
					time.Sleep(1 * time.Second)
					return dm.daemonClient.ListTools(dm.serverName)
				}
			}
		}
		return nil, err
	}
	return tools, nil
}

// CallTool implements the MCPClient interface
func (dm *DaemonMCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.ToolResult, error) {
	result, err := dm.daemonClient.CallTool(dm.serverName, toolName, arguments)
	if err != nil {
		// Try to start the session if it doesn't exist
		if config, loadErr := LoadMCPConfig(); loadErr == nil {
			if serverConfig, exists := config.MCPServers[dm.serverName]; exists {
				if startErr := dm.daemonClient.StartSession(dm.serverName, serverConfig); startErr == nil {
					// Give it a moment to start
					time.Sleep(1 * time.Second)
					return dm.daemonClient.CallTool(dm.serverName, toolName, arguments)
				}
			}
		}
	}
	return result, err
}

// ListResources implements the MCPClient interface
func (dm *DaemonMCPClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	// Daemon doesn't support resources yet, fall back to empty list
	return []mcp.Resource{}, nil
}

// CreateMessage implements the MCPClient interface (sampling)
func (dm *DaemonMCPClient) CreateMessage(ctx context.Context, request *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	// Not supported by daemon yet
	return nil, fmt.Errorf("CreateMessage not supported by daemon client")
}

// RequestInput implements the MCPClient interface (elicitation)
func (dm *DaemonMCPClient) RequestInput(ctx context.Context, params *mcp.RequestInputParams) (*mcp.RequestInputResult, error) {
	// Not supported by daemon yet
	return nil, fmt.Errorf("RequestInput not supported by daemon client")
}

// ListRoots implements the MCPClient interface
func (dm *DaemonMCPClient) ListRoots(ctx context.Context) ([]mcp.Root, error) {
	// Not supported by daemon yet
	return []mcp.Root{}, nil
}

// NotifyRootsListChanged implements the MCPClient interface
func (dm *DaemonMCPClient) NotifyRootsListChanged(roots []mcp.Root) error {
	// Not supported by daemon yet
	return nil
}

// Close implements the MCPClient interface
func (dm *DaemonMCPClient) Close() error {
	// Daemon manages session lifecycle, nothing to close here
	return nil
}

// Helper methods for URL construction

func (dc *DaemonClient) getHTTPURL() string {
	if isUnixSocket(dc.manager.endpoint) || isNamedPipe(dc.manager.endpoint) {
		return "http://localhost:8080" // Fallback for non-HTTP endpoints
	}
	return "http://" + dc.manager.endpoint
}

func (dc *DaemonClient) getSessionsURL() string {
	if isUnixSocket(dc.manager.endpoint) || isNamedPipe(dc.manager.endpoint) {
		return "http://localhost:8080/sessions"
	}
	return "http://" + dc.manager.endpoint + "/sessions"
}

func (dc *DaemonClient) getSessionURL(serverName, action string) string {
	base := dc.getSessionsURL()
	if action == "" {
		return base + "/" + serverName
	}
	return base + "/" + serverName + "/" + action
}

func (dc *DaemonClient) getToolURL(serverName, toolName string) string {
	if isUnixSocket(dc.manager.endpoint) || isNamedPipe(dc.manager.endpoint) {
		return fmt.Sprintf("http://localhost:8080/sessions/%s/call-tool/%s", serverName, toolName)
	}
	return fmt.Sprintf("http://%s/sessions/%s/call-tool/%s", dc.manager.endpoint, serverName, toolName)
}
