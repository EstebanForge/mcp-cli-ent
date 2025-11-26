package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
	"github.com/mcp-cli-ent/mcp-cli/internal/session"
)

// SessionAwareClientFactory creates MCP clients with session awareness
type SessionAwareClientFactory struct {
	sessionManager *session.Manager
}

// NewSessionAwareClientFactory creates a new session-aware client factory
func NewSessionAwareClientFactory(sessionManager *session.Manager) *SessionAwareClientFactory {
	return &SessionAwareClientFactory{
		sessionManager: sessionManager,
	}
}

// NewSessionManager creates a new session manager with client factory
func NewSessionManager(configDir string) (*session.Manager, error) {
	clientFactory := func(config config.ServerConfig) (mcp.MCPClient, error) {
		return NewMCPClient(config)
	}
	return session.NewManager(configDir, clientFactory)
}

// CreateClient creates an MCP client with appropriate session management
func (f *SessionAwareClientFactory) CreateClient(serverName string, serverConfig config.ServerConfig) (mcp.MCPClient, error) {
	// Get or create session for the server
	sess, err := f.sessionManager.GetSession(serverName, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// For stateless sessions, we need to handle client creation differently
	if sess.Type() == session.Stateless {
		return f.createStatelessClient(serverConfig)
	}

	// For persistent sessions, return the session's client
	client := sess.Client()
	if client == nil {
		// Try to start the session if it's not active
		if err := sess.Start(); err != nil {
			// Check for browser profile conflicts and provide helpful message
			if strings.Contains(err.Error(), "browser is already running") ||
				strings.Contains(err.Error(), "chrome-profile") {
				return nil, fmt.Errorf("browser profile conflict: %s\n\nSuggestion: Run './mcp-cli-ent session cleanup' to clean up old sessions, or try again in a few moments", err.Error())
			}
			return nil, fmt.Errorf("failed to start session: %w", err)
		}
		client = sess.Client()
		if client == nil {
			return nil, fmt.Errorf("session client is nil after starting")
		}
	}

	return &SessionAwareClient{
		client:  client,
		session: sess,
	}, nil
}

// createStatelessClient creates a traditional stateless client
func (f *SessionAwareClientFactory) createStatelessClient(serverConfig config.ServerConfig) (mcp.MCPClient, error) {
	return NewMCPClient(serverConfig)
}

// SessionAwareClient wraps an MCP client with session awareness
type SessionAwareClient struct {
	client  mcp.MCPClient
	session session.Session
}

// ListTools implements mcp.MCPClient
func (c *SessionAwareClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	return c.client.ListTools(ctx)
}

// CallTool implements mcp.MCPClient
func (c *SessionAwareClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.ToolResult, error) {
	// Update session activity
	if c.session != nil {
		c.session.UpdateActivity()
	}

	result, err := c.client.CallTool(ctx, name, arguments)
	if err != nil {
		// Check if this is a session-related error and handle it
		if c.session != nil && c.session.Type() == session.Persistent {
			// Try health check and restart if needed
			if healthErr := c.session.HealthCheck(); healthErr != nil {
				// Session is unhealthy, try to restart
				if restartErr := c.session.Restart(); restartErr != nil {
					return nil, fmt.Errorf("client error: %w, session restart failed: %v", err, restartErr)
				}
				// Try the operation again with the restarted session
				newClient := c.session.Client()
				if newClient != nil {
					return newClient.CallTool(ctx, name, arguments)
				}
			}
		}
		return nil, err
	}

	return result, nil
}

// ListResources implements mcp.MCPClient
func (c *SessionAwareClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	return c.client.ListResources(ctx)
}

// Initialize implements mcp.MCPClient
func (c *SessionAwareClient) Initialize(ctx context.Context, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
	// Update session activity
	if c.session != nil {
		c.session.UpdateActivity()
	}

	return c.client.Initialize(ctx, params)
}

// CreateMessage implements mcp.MCPClient
func (c *SessionAwareClient) CreateMessage(ctx context.Context, request *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	// Update session activity
	if c.session != nil {
		c.session.UpdateActivity()
	}

	return c.client.CreateMessage(ctx, request)
}

// RequestInput implements mcp.MCPClient
func (c *SessionAwareClient) RequestInput(ctx context.Context, params *mcp.RequestInputParams) (*mcp.RequestInputResult, error) {
	// Update session activity
	if c.session != nil {
		c.session.UpdateActivity()
	}

	return c.client.RequestInput(ctx, params)
}

// ListRoots implements mcp.MCPClient
func (c *SessionAwareClient) ListRoots(ctx context.Context) ([]mcp.Root, error) {
	// Update session activity
	if c.session != nil {
		c.session.UpdateActivity()
	}

	return c.client.ListRoots(ctx)
}

// NotifyRootsListChanged implements mcp.MCPClient
func (c *SessionAwareClient) NotifyRootsListChanged(roots []mcp.Root) error {
	// Update session activity
	if c.session != nil {
		c.session.UpdateActivity()
	}

	return c.client.NotifyRootsListChanged(roots)
}

// Close implements mcp.MCPClient
func (c *SessionAwareClient) Close() error {
	// For persistent sessions, we don't close the client here
	// The session manager handles session lifecycle
	if c.session != nil && c.session.Type() == session.Persistent {
		return nil // Don't close persistent clients
	}

	// For stateless sessions, close the client
	return c.client.Close()
}

// GetSession returns the underlying session (if any)
func (c *SessionAwareClient) GetSession() session.Session {
	return c.session
}

// IsPersistent returns true if this client uses a persistent session
func (c *SessionAwareClient) IsPersistent() bool {
	return c.session != nil && c.session.Type() == session.Persistent
}
