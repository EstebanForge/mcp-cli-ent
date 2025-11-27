package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// ClientFactory creates MCP clients
type ClientFactory func(config.ServerConfig) (mcp.MCPClient, error)

// PersistentSession represents a persistent MCP client session
type PersistentSession struct {
	name           string
	config         config.ServerConfig
	sessionType    SessionType
	status         SessionStatus
	client         mcp.MCPClient
	clientFactory  ClientFactory
	fileStore      *FileStore
	processManager *ProcessManager
	mutex          sync.RWMutex
	startTime      time.Time
	lastActivity   time.Time
	pid            int
	sessionID      string
	processPath    string
	processArgs    []string
	connectionInfo *ConnectionInfo
	endpoints      []string
	error          string
}

// NewPersistentSession creates a new persistent session
func NewPersistentSession(name string, serverConfig config.ServerConfig, clientFactory ClientFactory) (*PersistentSession, error) {
	return NewPersistentSessionWithFileStore(name, serverConfig, clientFactory, nil)
}

// NewPersistentSessionWithFileStore creates a new persistent session with file store
func NewPersistentSessionWithFileStore(name string, serverConfig config.ServerConfig, clientFactory ClientFactory, fileStore *FileStore) (*PersistentSession, error) {
	sessionType := DetectSessionType(serverConfig)

	// Initialize file store if not provided
	if fileStore == nil {
		// Use default config directory
		configDir, _ := os.UserConfigDir()
		sessionsDir := filepath.Join(configDir, "mcp-cli-ent", "sessions")
		fileStore = NewFileStore(sessionsDir)
	}

	sessionID := fileStore.GenerateSessionID(name)

	session := &PersistentSession{
		name:           name,
		config:         serverConfig,
		sessionType:    sessionType,
		status:         Inactive,
		clientFactory:  clientFactory,
		fileStore:      fileStore,
		processManager: NewProcessManager(),
		startTime:      time.Time{},
		lastActivity:   time.Now(),
		sessionID:      sessionID,
	}

	return session, nil
}

// LoadPersistentSession loads an existing persistent session from file store
func LoadPersistentSession(sessionInfo *SessionInfo, clientFactory ClientFactory, fileStore *FileStore) (*PersistentSession, error) {
	// Initialize file store if not provided
	if fileStore == nil {
		configDir, _ := os.UserConfigDir()
		sessionsDir := filepath.Join(configDir, "mcp-cli-ent", "sessions")
		fileStore = NewFileStore(sessionsDir)
	}

	session := &PersistentSession{
		name:           sessionInfo.Name,
		config:         sessionInfo.Config,
		sessionType:    sessionInfo.Type,
		status:         sessionInfo.Status,
		clientFactory:  clientFactory,
		fileStore:      fileStore,
		processManager: NewProcessManager(),
		startTime:      sessionInfo.StartTime,
		lastActivity:   sessionInfo.LastActivity,
		pid:            sessionInfo.PID,
		sessionID:      sessionInfo.SessionID,
		processPath:    sessionInfo.ProcessPath,
		processArgs:    sessionInfo.ProcessArgs,
		connectionInfo: sessionInfo.ConnectionInfo,
		endpoints:      sessionInfo.Endpoints,
		error:          sessionInfo.Error,
	}

	return session, nil
}

// Name returns the session name
func (s *PersistentSession) Name() string {
	return s.name
}

// Type returns the session type
func (s *PersistentSession) Type() SessionType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.sessionType
}

// Status returns the current session status
func (s *PersistentSession) Status() SessionStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.status
}

// Client returns the MCP client for this session
func (s *PersistentSession) Client() mcp.MCPClient {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.client
}

// Config returns the server configuration
func (s *PersistentSession) Config() config.ServerConfig {
	return s.config
}

// Start starts the persistent session
func (s *PersistentSession) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.status == Active {
		return nil // Already started
	}

	if s.status == Starting {
		return fmt.Errorf("session is already starting")
	}

	if s.clientFactory == nil {
		return fmt.Errorf("client factory not initialized")
	}

	s.status = Starting
	s.error = ""

	// Try to reattach to existing session if we have session metadata
	if s.sessionID != "" && s.pid > 0 {
		reattachErr := s.tryReattach()
		if reattachErr == nil {
			// Successfully reattached
			return nil
		}
		// Reattachment failed, continue with creating new session
		fmt.Printf("Warning: Failed to reattach to existing session: %v\n", reattachErr)
	}

	// Create new session
	return s.createNewSession()
}

// tryReattach attempts to reattach to an existing session
func (s *PersistentSession) tryReattach() error {
	// Check if process is still alive
	if !s.processManager.IsProcessAlive(s.pid) {
		return fmt.Errorf("process %d is no longer alive", s.pid)
	}

	// Validate process matches our expectations
	_, err := s.processManager.FindProcess(s.pid)
	if err != nil {
		return fmt.Errorf("failed to get process info: %w", err)
	}

	// For HTTP-based sessions, we can try to reconnect directly
	if s.config.Type == "http" && s.config.URL != "" {
		return s.reattachToHTTPSession()
	}

	// For stdio-based sessions, reattachment is more complex and may not be possible
	// For now, we'll create a new session
	return fmt.Errorf("reattachment to stdio sessions not yet supported")
}

// reattachToHTTPSession attempts to reattach to an HTTP-based session
func (s *PersistentSession) reattachToHTTPSession() error {
	// Create new HTTP client that connects to existing endpoint
	client, err := s.clientFactory(s.config)
	if err != nil {
		return fmt.Errorf("failed to create client for reattachment: %w", err)
	}

	// Test the connection with a simple health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.ListTools(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("health check failed during reattachment: %w", err)
	}

	// Successfully reattached
	s.client = client
	s.status = Active
	s.lastActivity = time.Now()
	s.error = ""

	return nil
}

// createNewSession creates a brand new session
func (s *PersistentSession) createNewSession() error {
	// Create the MCP client using the factory
	client, err := s.clientFactory(s.config)
	if err != nil {
		s.status = Error
		s.error = fmt.Sprintf("failed to create client: %v", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Store process information
	s.pid = os.Getpid()

	// Try to get process information for metadata storage
	if processInfo, err := s.processManager.FindProcess(s.pid); err == nil {
		s.processPath = processInfo.Executable
		s.processArgs = processInfo.Args
	}

	// Set up connection info based on server type
	if s.config.Type == "http" {
		s.connectionInfo = &ConnectionInfo{
			Type: "http",
			URL:  s.config.URL,
			Extra: map[string]interface{}{
				"timeout": s.config.Timeout,
			},
		}
		s.endpoints = []string{s.config.URL}
	} else {
		s.connectionInfo = &ConnectionInfo{
			Type: "stdio",
			Extra: map[string]interface{}{
				"command": s.config.Command,
				"args":    s.config.Args,
				"timeout": s.config.Timeout,
			},
		}
	}

	s.client = client
	s.status = Active
	s.startTime = time.Now()
	s.lastActivity = time.Now()
	s.error = ""

	// Capture session info before releasing the lock to avoid deadlock
	sessionInfo := s.buildSessionInfo()

	// Save session metadata to file asynchronously
	s.saveToStoreAsyncWithInfo(&sessionInfo)

	return nil
}

// buildSessionInfo builds the session info structure (must be called with lock held)
func (s *PersistentSession) buildSessionInfo() SessionInfo {
	return SessionInfo{
		SessionID:      s.sessionID,
		Name:           s.name,
		Type:           s.sessionType,
		Status:         s.status,
		PID:            s.pid,
		ProcessPath:    s.processPath,
		ProcessArgs:    s.processArgs,
		ConnectionInfo: s.connectionInfo,
		StartTime:      s.startTime,
		LastActivity:   s.lastActivity,
		Endpoints:      s.endpoints,
		Error:          s.error,
		Config:         s.config,
	}
}

// saveToStoreAsyncWithInfo saves session metadata with pre-captured info (for use with lock held)
func (s *PersistentSession) saveToStoreAsyncWithInfo(info *SessionInfo) {
	if s.fileStore == nil {
		return
	}
	go func() {
		if err := s.fileStore.SaveSession(info); err != nil {
			if os.Getenv("MCP_VERBOSE") == "true" {
				fmt.Printf("Warning: Failed to save session metadata: %v\n", err)
			}
		}
	}()
}

// Stop stops the session and cleans up resources
func (s *PersistentSession) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.status == Inactive || s.status == Stopped {
		return nil // Already stopped
	}

	s.status = Stopping

	if s.client != nil {
		if err := s.client.Close(); err != nil {
			s.status = Error
			s.error = fmt.Sprintf("failed to close client: %v", err)
			return fmt.Errorf("failed to close client: %w", err)
		}
		s.client = nil
	}

	s.status = Stopped
	s.pid = 0
	s.endpoints = nil
	s.error = ""

	// Capture session info before releasing the lock to avoid deadlock
	sessionInfo := s.buildSessionInfo()

	// Update session metadata in file store asynchronously
	s.saveToStoreAsyncWithInfo(&sessionInfo)

	return nil
}

// Restart restarts the session
func (s *PersistentSession) Restart() error {
	if err := s.Stop(); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	// Wait a moment for cleanup
	time.Sleep(100 * time.Millisecond)

	return s.Start()
}

// HealthCheck performs a health check on the session
func (s *PersistentSession) HealthCheck() error {
	s.mutex.RLock()

	if s.status != Active {
		s.mutex.RUnlock()
		return fmt.Errorf("session is not active (status: %s)", s.status.String())
	}

	if s.client == nil {
		s.mutex.RUnlock()
		return fmt.Errorf("session has no active client")
	}

	// Check if process is still alive first
	if s.pid > 0 && !s.processManager.IsProcessAlive(s.pid) {
		s.mutex.RUnlock()
		// Mark session as stopped since process is dead
		s.mutex.Lock()
		s.status = Stopped
		s.pid = 0
		s.error = "process terminated"

		// Capture session info before releasing the lock
		sessionInfo := s.buildSessionInfo()
		s.mutex.Unlock()

		// Update session metadata asynchronously
		s.saveToStoreAsyncWithInfo(&sessionInfo)

		return fmt.Errorf("session process (PID %d) is no longer alive", s.pid)
	}

	client := s.client
	s.mutex.RUnlock()

	// Perform a simple health check by listing tools
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.ListTools(ctx)
	if err != nil {
		s.mutex.Lock()
		s.status = Error
		s.error = fmt.Sprintf("health check failed: %v", err)
		s.mutex.Unlock()
		return fmt.Errorf("health check failed: %w", err)
	}

	// Update last activity time on successful health check
	s.UpdateActivity()

	return nil
}

// LastActivity returns the time of last activity
func (s *PersistentSession) LastActivity() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastActivity
}

// UpdateActivity updates the last activity time
func (s *PersistentSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.lastActivity = time.Now()

	// Update activity in file store asynchronously
	go func() {
		if s.fileStore != nil {
			if err := s.fileStore.UpdateSessionActivity(s.sessionID); err != nil {
				// Only show warning if MCP_VERBOSE environment variable is set
				if os.Getenv("MCP_VERBOSE") == "true" {
					fmt.Printf("Warning: Failed to update session activity: %v\n", err)
				}
			}
		}
	}()
}

// GetInfo returns session information
func (s *PersistentSession) GetInfo() SessionInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return SessionInfo{
		SessionID:      s.sessionID,
		Name:           s.name,
		Type:           s.sessionType,
		Status:         s.status,
		PID:            s.pid,
		ProcessPath:    s.processPath,
		ProcessArgs:    s.processArgs,
		ConnectionInfo: s.connectionInfo,
		StartTime:      s.startTime,
		LastActivity:   s.lastActivity,
		Endpoints:      s.endpoints,
		Error:          s.error,
		Config:         s.config,
	}
}

// IsExpired checks if the session has expired based on the maximum idle time
func (s *PersistentSession) IsExpired(maxIdleTime int) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.status != Active {
		return false
	}

	if maxIdleTime <= 0 {
		return false // No expiration
	}

	idleTime := time.Since(s.lastActivity)
	return idleTime > time.Duration(maxIdleTime)*time.Second
}

// SetError sets the session status to error with the given message
func (s *PersistentSession) SetError(err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.status = Error
	if err != nil {
		s.error = err.Error()
	} else {
		s.error = "unknown error"
	}
}
