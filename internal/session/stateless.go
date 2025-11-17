package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// StatelessSession represents a stateless session that creates clients on demand
type StatelessSession struct {
	name          string
	config        config.ServerConfig
	sessionType   SessionType
	status        SessionStatus
	clientFactory ClientFactory
	mutex         sync.RWMutex
	lastActivity  time.Time
}

// NewStatelessSession creates a new stateless session
func NewStatelessSession(name string, serverConfig config.ServerConfig, clientFactory ClientFactory) (*StatelessSession, error) {
	session := &StatelessSession{
		name:          name,
		config:        serverConfig,
		sessionType:   Stateless,
		status:        Active, // Stateless sessions are always "active"
		clientFactory: clientFactory,
		lastActivity:  time.Now(),
	}

	return session, nil
}

// Name returns the session name
func (s *StatelessSession) Name() string {
	return s.name
}

// Type returns the session type
func (s *StatelessSession) Type() SessionType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.sessionType
}

// Status returns the current session status
func (s *StatelessSession) Status() SessionStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.status
}

// Client returns the MCP client for this session
func (s *StatelessSession) Client() mcp.MCPClient {
	// For stateless sessions, we create a new client each time
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.clientFactory == nil {
		return nil // Return nil on error, caller should handle this
	}

	client, err := s.clientFactory(s.config)
	if err != nil {
		return nil // Return nil on error, caller should handle this
	}

	return client
}

// Config returns the server configuration
func (s *StatelessSession) Config() config.ServerConfig {
	return s.config
}

// Start is a no-op for stateless sessions
func (s *StatelessSession) Start() error {
	// Stateless sessions don't need starting
	return nil
}

// Stop is a no-op for stateless sessions
func (s *StatelessSession) Stop() error {
	// Stateless sessions don't need stopping
	return nil
}

// Restart is a no-op for stateless sessions
func (s *StatelessSession) Restart() error {
	// Stateless sessions don't need restarting
	return nil
}

// HealthCheck performs a health check by creating a temporary client
func (s *StatelessSession) HealthCheck() error {
	client := s.Client()
	if client == nil {
		return fmt.Errorf("failed to create client for health check")
	}
	if client != nil {
		defer client.Close()
	}

	// Perform a simple health check by listing tools
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Update last activity time on successful health check
	s.UpdateActivity()

	return nil
}

// LastActivity returns the time of last activity
func (s *StatelessSession) LastActivity() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastActivity
}

// UpdateActivity updates the last activity time
func (s *StatelessSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.lastActivity = time.Now()
}