package session

import (
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// SessionType represents the type of session
type SessionType int

const (
	// Stateless sessions create new clients for each command (current behavior)
	Stateless SessionType = iota
	// Persistent sessions maintain connections across commands for browser servers
	Persistent
	// Hybrid sessions try persistent but fallback to stateless
	Hybrid
)

// SessionStatus represents the current status of a session
type SessionStatus int

const (
	// Inactive session has not been started
	Inactive SessionStatus = iota
	// Starting session is being initialized
	Starting
	// Active session is ready for use
	Active
	// Error session encountered an error
	Error
	// Stopping session is being shut down
	Stopping
	// Stopped session was cleanly shut down
	Stopped
)

// String returns the string representation of SessionType
func (st SessionType) String() string {
	switch st {
	case Stateless:
		return "stateless"
	case Persistent:
		return "persistent"
	case Hybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// String returns the string representation of SessionStatus
func (ss SessionStatus) String() string {
	switch ss {
	case Inactive:
		return "inactive"
	case Starting:
		return "starting"
	case Active:
		return "active"
	case Error:
		return "error"
	case Stopping:
		return "stopping"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Session represents a managed MCP client session
type Session interface {
	// Name returns the session name (same as server name)
	Name() string

	// Type returns the session type
	Type() SessionType

	// Status returns the current session status
	Status() SessionStatus

	// Client returns the MCP client for this session
	Client() mcp.MCPClient

	// Config returns the server configuration
	Config() config.ServerConfig

	// Start starts the session if it's persistent
	Start() error

	// Stop stops the session and cleans up resources
	Stop() error

	// Restart restarts the session
	Restart() error

	// HealthCheck performs a health check on the session
	HealthCheck() error

	// LastActivity returns the time of last activity
	LastActivity() time.Time

	// UpdateActivity updates the last activity time
	UpdateActivity()
}

// SessionInfo contains metadata about a session
type SessionInfo struct {
	SessionID   string        `json:"sessionId"`
	Name         string        `json:"name"`
	Type         SessionType   `json:"type"`
	Status       SessionStatus `json:"status"`
	PID          int           `json:"pid,omitempty"`
	ProcessPath  string        `json:"processPath,omitempty"`
	ProcessArgs  []string      `json:"processArgs,omitempty"`
	ConnectionInfo *ConnectionInfo `json:"connectionInfo,omitempty"`
	StartTime    time.Time     `json:"startTime"`
	LastActivity time.Time     `json:"lastActivity"`
	Endpoints    []string      `json:"endpoints,omitempty"`
	Error        string        `json:"error,omitempty"`
	Config       config.ServerConfig `json:"config"`
}

// ConnectionInfo contains connection details for session reattachment
type ConnectionInfo struct {
	Type  string                 `json:"type"`  // "stdio" or "http"
	Ports map[string]int         `json:"ports,omitempty"` // For stdio: stdin, stdout, stderr
	URL   string                 `json:"url,omitempty"`   // For HTTP: endpoint URL
	Extra map[string]interface{} `json:"extra,omitempty"` // Additional connection metadata
}

// SessionConfig contains session-specific configuration
type SessionConfig struct {
	Type       SessionType `json:"type"`
	AutoStart  bool        `json:"autoStart"`
	Timeout    int         `json:"timeout"`    // Timeout in seconds
	MaxIdle    int         `json:"maxIdle"`    // Max idle time in seconds before auto-stop
	HealthCheck bool       `json:"healthCheck"` // Enable periodic health checks
}