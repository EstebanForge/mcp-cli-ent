package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/client"
	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

// Daemon represents the main daemon process
type Daemon struct {
	httpServer    *http.Server
	sessions      map[string]*PersistentSession
	sessionMutex  sync.RWMutex
	config        *DaemonConfig
	clientFactory func(config.ServerConfig) (mcp.MCPClient, error)
	startTime     time.Time
	pid           int
	platform      string
	endpoint      string
	shutdownChan  chan struct{}
}

// NewDaemon creates a new daemon instance
func NewDaemon(config *DaemonConfig) (*Daemon, error) {
	if config == nil {
		config = DefaultDaemonConfig()
	}

	platform := detectPlatform()
	endpoint := getDaemonEndpoint(platform)

	daemon := &Daemon{
		sessions:      make(map[string]*PersistentSession),
		config:        config,
		clientFactory: client.NewMCPClient,
		startTime:     time.Now(),
		pid:           os.Getpid(),
		platform:      platform,
		endpoint:      endpoint,
		shutdownChan:  make(chan struct{}),
	}

	return daemon, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	log.Printf("Starting MCP CLI daemon on %s", d.endpoint)

	// Create HTTP server
	mux := http.NewServeMux()
	d.setupRoutes(mux)

	d.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start background cleanup routine
	go d.cleanupRoutine()

	// Start server based on endpoint type
	var err error
	if isUnixSocket(d.endpoint) {
		err = d.startUnixSocket()
	} else if isNamedPipe(d.endpoint) {
		err = d.startNamedPipe()
	} else {
		err = d.startHTTPServer()
	}

	if err != nil {
		return fmt.Errorf("failed to start daemon server: %w", err)
	}

	log.Printf("Daemon started successfully on %s (PID: %d)", d.endpoint, d.pid)
	return nil
}

// Stop stops the daemon gracefully
func (d *Daemon) Stop() error {
	log.Printf("Stopping MCP CLI daemon...")

	// Stop all sessions
	d.sessionMutex.Lock()
	for serverName, session := range d.sessions {
		if session.Client != nil {
			log.Printf("Stopping session: %s", serverName)
			session.Client.Close()
		}
	}
	d.sessions = make(map[string]*PersistentSession)
	d.sessionMutex.Unlock()

	// Shutdown HTTP server
	if d.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
		}
	}

	// Signal shutdown
	close(d.shutdownChan)

	log.Printf("Daemon stopped")
	return nil
}

// StartSession starts a new persistent session for a server
func (d *Daemon) StartSession(serverName string, serverConfig config.ServerConfig) error {
	d.sessionMutex.Lock()
	defer d.sessionMutex.Unlock()

	// Check if session already exists
	if existing, exists := d.sessions[serverName]; exists {
		if existing.Status == SessionStatusActive {
			return fmt.Errorf("session %s already active", serverName)
		}
		if existing.Status == SessionStatusStarting {
			return fmt.Errorf("session %s is already starting", serverName)
		}
	}

	// Create new session
	session := &PersistentSession{
		ServerName: serverName,
		Status:     SessionStatusStarting,
		Config:     serverConfig,
		StartTime:  time.Now(),
		LastUsed:   time.Now(),
		ToolCache:  make(map[string][]mcp.Tool),
	}

	d.sessions[serverName] = session

	// Start session in background to avoid blocking
	go d.startSessionBackground(session)

	return nil
}

// startSessionBackground starts a session in the background
func (d *Daemon) startSessionBackground(session *PersistentSession) {
	log.Printf("Starting session: %s", session.ServerName)

	// Create MCP client
	client, err := d.clientFactory(session.Config)
	if err != nil {
		d.setSessionError(session.ServerName, fmt.Sprintf("failed to create client: %v", err))
		return
	}

	// Test connection with a simple health check
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = client.ListTools(ctx)
	if err != nil {
		client.Close()
		d.setSessionError(session.ServerName, fmt.Sprintf("health check failed: %v", err))
		return
	}

	// Session started successfully
	d.sessionMutex.Lock()
	if existingSession, exists := d.sessions[session.ServerName]; exists {
		existingSession.Client = client
		existingSession.Status = SessionStatusActive
		existingSession.LastUsed = time.Now()
		existingSession.Error = ""

		// Try to get PID if it's a stdio session
		if session.Config.Command != "" {
			existingSession.PID = d.tryGetSessionPID(session.Config)
		}
	}
	d.sessionMutex.Unlock()

	log.Printf("Session started successfully: %s", session.ServerName)
}

// StopSession stops a session
func (d *Daemon) StopSession(serverName string) error {
	d.sessionMutex.Lock()
	defer d.sessionMutex.Unlock()

	session, exists := d.sessions[serverName]
	if !exists {
		return fmt.Errorf("session %s not found", serverName)
	}

	if session.Status != SessionStatusActive {
		return fmt.Errorf("session %s is not active", serverName)
	}

	session.Status = SessionStatusStopping

	if session.Client != nil {
		session.Client.Close()
		session.Client = nil
	}

	delete(d.sessions, serverName)
	log.Printf("Session stopped: %s", serverName)

	return nil
}

// GetSession returns a session by name
func (d *Daemon) GetSession(serverName string) (*PersistentSession, error) {
	d.sessionMutex.RLock()
	defer d.sessionMutex.RUnlock()

	session, exists := d.sessions[serverName]
	if !exists {
		return nil, fmt.Errorf("session %s not found", serverName)
	}

	if session.Status != SessionStatusActive {
		return nil, fmt.Errorf("session %s is not active (status: %s)", serverName, session.Status)
	}

	return session, nil
}

// ListSessions returns information about all sessions
func (d *Daemon) ListSessions() []SessionInfo {
	d.sessionMutex.RLock()
	defer d.sessionMutex.RUnlock()

	var sessions []SessionInfo
	for _, session := range d.sessions {
		info := SessionInfo{
			ServerName: session.ServerName,
			Status:     session.Status.String(),
			StartTime:  session.StartTime,
			LastUsed:   session.LastUsed,
			Duration:   time.Since(session.StartTime),
			Error:      session.Error,
			PID:        session.PID,
		}
		sessions = append(sessions, info)
	}

	return sessions
}

// CallTool executes a tool in a persistent session
func (d *Daemon) CallTool(serverName, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
	session, err := d.GetSession(serverName)
	if err != nil {
		return nil, err
	}

	// Update last used time
	session.LastUsed = time.Now()

	// Execute tool
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := session.Client.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	return result, nil
}

// ListTools lists tools for a persistent session
func (d *Daemon) ListTools(serverName string) ([]mcp.Tool, error) {
	session, err := d.GetSession(serverName)
	if err != nil {
		return nil, err
	}

	// Check cache first
	d.sessionMutex.RLock()
	if tools, cached := session.ToolCache["list"]; cached {
		d.sessionMutex.RUnlock()
		return tools, nil
	}
	d.sessionMutex.RUnlock()

	// Fetch tools
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools, err := session.Client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Cache the result
	d.sessionMutex.Lock()
	session.ToolCache["list"] = tools
	session.LastUsed = time.Now()
	d.sessionMutex.Unlock()

	return tools, nil
}

// GetStatus returns the overall daemon status
func (d *Daemon) GetStatus() *DaemonStatus {
	d.sessionMutex.RLock()
	defer d.sessionMutex.RUnlock()

	var activeSessions []SessionInfo
	for _, session := range d.sessions {
		info := SessionInfo{
			ServerName: session.ServerName,
			Status:     session.Status.String(),
			StartTime:  session.StartTime,
			LastUsed:   session.LastUsed,
			Duration:   time.Since(session.StartTime),
			Error:      session.Error,
			PID:        session.PID,
		}
		activeSessions = append(activeSessions, info)
	}

	return &DaemonStatus{
		Running:        true,
		StartTime:      d.startTime,
		Version:        version.Version,
		SessionCount:   len(d.sessions),
		ActiveSessions: activeSessions,
		PID:            d.pid,
		Endpoint:       d.endpoint,
		Platform:       d.platform,
	}
}

// Helper methods

func (d *Daemon) setSessionError(serverName, errorMsg string) {
	d.sessionMutex.Lock()
	defer d.sessionMutex.Unlock()

	if session, exists := d.sessions[serverName]; exists {
		session.Status = SessionStatusError
		session.Error = errorMsg
	}
}

func (d *Daemon) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.cleanupIdleSessions()
		case <-d.shutdownChan:
			return
		}
	}
}

func (d *Daemon) cleanupIdleSessions() {
	d.sessionMutex.Lock()
	defer d.sessionMutex.Unlock()

	now := time.Now()
	maxIdle := time.Duration(d.config.MaxIdleTime) * time.Second

	for serverName, session := range d.sessions {
		if session.Status != SessionStatusActive {
			continue
		}

		if now.Sub(session.LastUsed) > maxIdle {
			log.Printf("Cleaning up idle session: %s", serverName)
			if session.Client != nil {
				session.Client.Close()
			}
			delete(d.sessions, serverName)
		}
	}
}

func (d *Daemon) tryGetSessionPID(serverConfig config.ServerConfig) int {
	// This is a simplified implementation
	// In a real implementation, we'd need to track the actual process
	// that gets spawned by the stdio client
	return 0
}

func (d *Daemon) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		// If encoding fails, we can't write a JSON response, so we'll log and write a plain text error
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// Platform detection helpers
func detectPlatform() string {
	if isWSL() {
		return "wsl"
	}
	return runtime.GOOS
}

func isWSL() bool {
	// Check for WSL by reading /proc/version
	if data, err := os.ReadFile("/proc/version"); err == nil {
		return strings.Contains(string(data), "Microsoft") || strings.Contains(string(data), "WSL")
	}
	return false
}
