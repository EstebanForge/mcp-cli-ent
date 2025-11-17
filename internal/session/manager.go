package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
)

// Manager manages MCP client sessions
type Manager struct {
	sessions      map[string]Session
	mutex         sync.RWMutex
	configDir     string
	sessionsDir   string
	clientFactory ClientFactory
	fileStore     *FileStore
	processManager *ProcessManager
}

// NewManager creates a new session manager
func NewManager(configDir string, clientFactory ClientFactory) (*Manager, error) {
	sessionsDir := filepath.Join(configDir, "sessions")

	// Create sessions directory if it doesn't exist
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	fileStore := NewFileStore(sessionsDir)
	processManager := NewProcessManager()

	manager := &Manager{
		sessions:       make(map[string]Session),
		configDir:      configDir,
		sessionsDir:    sessionsDir,
		clientFactory:  clientFactory,
		fileStore:      fileStore,
		processManager: processManager,
	}

	// Load existing sessions from disk
	if err := manager.loadSessions(); err != nil {
		// Log error but don't fail creation
		fmt.Printf("Warning: Failed to load existing sessions: %v\n", err)
	}

	// Clean up dead sessions on startup
	go manager.cleanupDeadSessions()

	return manager, nil
}

// GetSession gets or creates a session for the given server
func (m *Manager) GetSession(serverName string, serverConfig config.ServerConfig) (Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if session already exists in memory
	if session, exists := m.sessions[serverName]; exists {
		// Update activity time
		session.UpdateActivity()
		return session, nil
	}

	// Check if we can reattach to an existing persistent session
	sessionType := DetectSessionType(serverConfig)
	if sessionType == Persistent || sessionType == Hybrid {
		var existingSession Session
		var reattachErr error
		existingSession, reattachErr = m.tryReattachSession(serverName, serverConfig)
		if reattachErr == nil {
			m.sessions[serverName] = existingSession
			return existingSession, nil
		}
		// Reattachment failed, continue with creating new session
		fmt.Printf("Warning: Failed to reattach to existing session for %s: %v\n", serverName, reattachErr)
	}

	// Create new session
	var session Session
	var err error

	switch sessionType {
	case Stateless:
		// For stateless sessions, we don't create a persistent session
		// Instead, we create clients on-demand
		session, err = NewStatelessSession(serverName, serverConfig, m.clientFactory)
	case Persistent, Hybrid:
		// Create persistent session with file store
		session, err = NewPersistentSessionWithFileStore(serverName, serverConfig, m.clientFactory, m.fileStore)
	default:
		return nil, fmt.Errorf("unsupported session type: %s", sessionType.String())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Auto-start persistent sessions if configured
	if (sessionType == Persistent || sessionType == Hybrid) && ShouldAutoStart(serverConfig) {
		if err := session.Start(); err != nil {
			return nil, fmt.Errorf("failed to auto-start persistent session: %w", err)
		}
	}

	m.sessions[serverName] = session

	// Save session info to disk
	if err := m.saveSession(session); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: Failed to save session info: %v\n", err)
	}

	return session, nil
}

// ListSessions returns a list of all sessions
func (m *Manager) ListSessions() ([]SessionInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	sessions := make([]SessionInfo, 0, len(m.sessions))
	for _, session := range m.sessions {
		if persistentSession, ok := session.(*PersistentSession); ok {
			sessions = append(sessions, persistentSession.GetInfo())
		}
	}

	return sessions, nil
}

// GetSession returns a specific session by name
func (m *Manager) GetSessionByName(serverName string) (Session, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	session, exists := m.sessions[serverName]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", serverName)
	}

	return session, nil
}

// StopSession stops a specific session
func (m *Manager) StopSession(serverName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[serverName]
	if !exists {
		return fmt.Errorf("session not found: %s", serverName)
	}

	if err := session.Stop(); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	// Remove session file
	sessionFile := filepath.Join(m.sessionsDir, serverName+".json")
	os.Remove(sessionFile) // Ignore error

	// Remove from memory
	delete(m.sessions, serverName)

	return nil
}

// StopAllSessions stops all sessions
func (m *Manager) StopAllSessions() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errors []error

	for name, session := range m.sessions {
		if err := session.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop session %s: %w", name, err))
		}

		// Remove session file
		sessionFile := filepath.Join(m.sessionsDir, name+".json")
		os.Remove(sessionFile) // Ignore error
	}

	// Clear memory
	m.sessions = make(map[string]Session)

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping sessions: %v", errors)
	}

	return nil
}

// CleanupSessions cleans up dead or expired sessions
func (m *Manager) CleanupSessions() error {
	return m.cleanupDeadSessions()
}

// RestartSession restarts a specific session
func (m *Manager) RestartSession(serverName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[serverName]
	if !exists {
		return fmt.Errorf("session not found: %s", serverName)
	}

	return session.Restart()
}

// cleanupDeadSessions removes dead or expired sessions
func (m *Manager) cleanupDeadSessions() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var toDelete []string

	for name, session := range m.sessions {
		if persistentSession, ok := session.(*PersistentSession); ok {
			// Check if session is expired
			maxIdle := GetSessionTimeout(persistentSession.Config())
			if persistentSession.IsExpired(maxIdle) {
				toDelete = append(toDelete, name)
				continue
			}

			// Perform health check
			if persistentSession.Status() == Active {
				if err := persistentSession.HealthCheck(); err != nil {
					fmt.Printf("Health check failed for session %s: %v\n", name, err)
					toDelete = append(toDelete, name)
					continue
				}
			}
		}
	}

	// Delete dead sessions
	for _, name := range toDelete {
		if session := m.sessions[name]; session != nil {
			session.Stop() // Ignore error
		}
		delete(m.sessions, name)

		// Remove session file
		sessionFile := filepath.Join(m.sessionsDir, name+".json")
		os.Remove(sessionFile) // Ignore error
	}

	return nil
}


// saveSession saves session information to disk
func (m *Manager) saveSession(session Session) error {
	if persistentSession, ok := session.(*PersistentSession); ok {
		info := persistentSession.GetInfo()

		sessionFile := filepath.Join(m.sessionsDir, session.Name()+".json")

		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal session info: %w", err)
		}

		if err := os.WriteFile(sessionFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write session file: %w", err)
		}
	}

	return nil
}

// loadSessions loads existing sessions from disk
func (m *Manager) loadSessions() error {
	sessionInfos, err := m.fileStore.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	validSessions := 0
	invalidSessions := 0

	for _, sessionInfo := range sessionInfos {
		// Skip invalid sessions
		if err := m.fileStore.ValidateSession(sessionInfo); err != nil {
			invalidSessions++
			// Clean up invalid session files silently
			if deleteErr := m.fileStore.DeleteSession(sessionInfo.SessionID); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete invalid session %s: %v\n", sessionInfo.Name, deleteErr)
			}
			continue
		}

		validSessions++
		// For now, don't load sessions into memory automatically
		// They will be loaded on-demand when GetSession is called
		// This prevents starting all sessions at startup
	}

	// Only report if we found sessions to process
	if validSessions > 0 || invalidSessions > 0 {
		fmt.Printf("Session cleanup: %d valid sessions found, %d invalid sessions removed\n", validSessions, invalidSessions)
	}

	return nil
}

// tryReattachSession attempts to reattach to an existing session
func (m *Manager) tryReattachSession(serverName string, serverConfig config.ServerConfig) (Session, error) {
	// Look for existing session in file store
	sessionInfo, err := m.fileStore.FindExistingSession(serverName)
	if err != nil {
		return nil, err
	}

	// Validate server config matches
	if !m.configMatches(sessionInfo.Config, serverConfig) {
		return nil, fmt.Errorf("server configuration mismatch")
	}

	// Load the persistent session from session info
	session, err := LoadPersistentSession(sessionInfo, m.clientFactory, m.fileStore)
	if err != nil {
		return nil, fmt.Errorf("failed to load persistent session: %w", err)
	}

	// Try to start the session (which will attempt reattachment)
	if err := session.Start(); err != nil {
		return nil, fmt.Errorf("failed to start reattached session: %w", err)
	}

	return session, nil
}

// configMatches checks if two server configs are compatible for reattachment
func (m *Manager) configMatches(existing, new config.ServerConfig) bool {
	// For HTTP servers, just check URL matches
	if existing.Type == "http" && new.Type == "http" {
		return existing.URL == new.URL
	}

	// For stdio servers, check command and args match
	if existing.Command != "" && new.Command != "" {
		if existing.Command != new.Command {
			return false
		}
		// Simple args comparison (could be made more sophisticated)
		if len(existing.Args) != len(new.Args) {
			return false
		}
		for i, arg := range existing.Args {
			if arg != new.Args[i] {
				return false
			}
		}
		return true
	}

	return false
}

// CleanupStaleSessions removes dead or expired sessions
func (m *Manager) CleanupStaleSessions() error {
	return m.fileStore.CleanupStaleSessions(24 * time.Hour) // Default to 24 hours
}

// GetFileStore returns the file store for external access
func (m *Manager) GetFileStore() *FileStore {
	return m.fileStore
}

// GetProcessManager returns the process manager for external access
func (m *Manager) GetProcessManager() *ProcessManager {
	return m.processManager
}