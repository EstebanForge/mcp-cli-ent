package daemon

import (
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// SessionStatus represents the current status of a daemon session
type SessionStatus int

const (
	SessionStatusInactive SessionStatus = iota
	SessionStatusStarting
	SessionStatusActive
	SessionStatusStopping
	SessionStatusError
)

func (s SessionStatus) String() string {
	switch s {
	case SessionStatusInactive:
		return "inactive"
	case SessionStatusStarting:
		return "starting"
	case SessionStatusActive:
		return "active"
	case SessionStatusStopping:
		return "stopping"
	case SessionStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// PersistentSession represents a session managed by the daemon
type PersistentSession struct {
	ServerName string                `json:"serverName"`
	Client     mcp.MCPClient         `json:"-"`
	Status     SessionStatus         `json:"status"`
	Config     config.ServerConfig   `json:"config"`
	LastUsed   time.Time             `json:"lastUsed"`
	StartTime  time.Time             `json:"startTime"`
	Error      string                `json:"error,omitempty"`
	ToolCache  map[string][]mcp.Tool `json:"-"`
	PID        int                   `json:"pid,omitempty"`
}

// SessionInfo represents session information for API responses
type SessionInfo struct {
	ServerName string        `json:"serverName"`
	Status     string        `json:"status"`
	StartTime  time.Time     `json:"startTime"`
	LastUsed   time.Time     `json:"lastUsed"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	PID        int           `json:"pid,omitempty"`
}

// DaemonStatus represents the overall daemon status
type DaemonStatus struct {
	Running        bool          `json:"running"`
	StartTime      time.Time     `json:"startTime"`
	Version        string        `json:"version"`
	SessionCount   int           `json:"sessionCount"`
	ActiveSessions []SessionInfo `json:"activeSessions"`
	PID            int           `json:"pid"`
	Endpoint       string        `json:"endpoint"`
	Platform       string        `json:"platform"`
	Error          string        `json:"error,omitempty"`
}

// APIRequest represents a daemon API request
type APIRequest struct {
	Command string                 `json:"command"`
	Server  string                 `json:"server,omitempty"`
	Tool    string                 `json:"tool,omitempty"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// APIResponse represents a daemon API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// DaemonConfig represents daemon configuration
type DaemonConfig struct {
	Enabled     bool   `json:"enabled"`
	AutoStart   bool   `json:"autoStart"`
	LogLevel    string `json:"logLevel"`
	MaxIdleTime int    `json:"maxIdleTime"`
	MaxSessions int    `json:"maxSessions"`
}

// DefaultDaemonConfig returns default daemon configuration
func DefaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		Enabled:     true,
		AutoStart:   true,
		LogLevel:    "info",
		MaxIdleTime: 3600, // 1 hour
		MaxSessions: 10,
	}
}
