package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileStore handles file-based session persistence
type FileStore struct {
	sessionsDir    string
	processManager *ProcessManager
}

// NewFileStore creates a new file store
func NewFileStore(sessionsDir string) *FileStore {
	return &FileStore{
		sessionsDir:    sessionsDir,
		processManager: NewProcessManager(),
	}
}

// SaveSession saves session metadata to disk
func (fs *FileStore) SaveSession(sessionInfo *SessionInfo) error {
	if err := os.MkdirAll(fs.sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	filename := fs.sessionFilename(sessionInfo.Name)
	data, err := json.MarshalIndent(sessionInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads session metadata from disk
func (fs *FileStore) LoadSession(sessionID string) (*SessionInfo, error) {
	filename := fs.sessionFilename(sessionID)

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session file not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sessionInfo SessionInfo
	if err := json.Unmarshal(data, &sessionInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session info: %w", err)
	}

	return &sessionInfo, nil
}

// LoadSessionByName loads a session by server name
func (fs *FileStore) LoadSessionByName(serverName string) (*SessionInfo, error) {
	// Try exact name match first
	sessionInfo, err := fs.LoadSession(serverName)
	if err == nil {
		return sessionInfo, nil
	}

	// If not found, try to find by scanning session files
	sessions, err := fs.ListSessions()
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.Name == serverName {
			return session, nil
		}
	}

	return nil, fmt.Errorf("session not found: %s", serverName)
}

// ListSessions returns all sessions stored on disk
func (fs *FileStore) ListSessions() ([]*SessionInfo, error) {
	if _, err := os.Stat(fs.sessionsDir); os.IsNotExist(err) {
		return []*SessionInfo{}, nil
	}

	files, err := os.ReadDir(fs.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*SessionInfo

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filename := filepath.Join(fs.sessionsDir, file.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue // Skip unreadable files
		}

		var sessionInfo SessionInfo
		if err := json.Unmarshal(data, &sessionInfo); err != nil {
			continue // Skip invalid files
		}

		sessions = append(sessions, &sessionInfo)
	}

	return sessions, nil
}

// DeleteSession deletes a session file
func (fs *FileStore) DeleteSession(sessionID string) error {
	filename := fs.sessionFilename(sessionID)

	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// CleanupStaleSessions removes sessions that are no longer valid
func (fs *FileStore) CleanupStaleSessions(olderThan time.Duration) error {
	sessions, err := fs.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	var toDelete []string

	for _, session := range sessions {
		shouldDelete := false

		// Check if session is too old
		if !session.LastActivity.IsZero() {
			if time.Since(session.LastActivity) > olderThan {
				shouldDelete = true
			}
		} else if !session.StartTime.IsZero() {
			if time.Since(session.StartTime) > olderThan {
				shouldDelete = true
			}
		}

		// Check if process is still alive for active sessions
		if session.Status == Active && session.PID > 0 {
			if !fs.processManager.IsProcessAlive(session.PID) {
				shouldDelete = true
			}
		}

		// Check for error sessions
		if session.Status == Error {
			// Keep error sessions for a shorter time
			errorExpiry := 1 * time.Hour
			if !session.LastActivity.IsZero() {
				if time.Since(session.LastActivity) > errorExpiry {
					shouldDelete = true
				}
			}
		}

		if shouldDelete {
			toDelete = append(toDelete, session.SessionID)
		}
	}

	// Delete stale sessions
	for _, sessionID := range toDelete {
		if err := fs.DeleteSession(sessionID); err != nil {
			fmt.Printf("Warning: failed to delete stale session %s: %v\n", sessionID, err)
		}
	}

	return nil
}

// ValidateSession checks if a session on disk is still valid
func (fs *FileStore) ValidateSession(sessionInfo *SessionInfo) error {
	if sessionInfo == nil {
		return fmt.Errorf("session info is nil")
	}

	// Check required fields
	if sessionInfo.Name == "" {
		return fmt.Errorf("session name is required")
	}

	if sessionInfo.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// For active sessions, check if process is still alive
	if sessionInfo.Status == Active && sessionInfo.PID > 0 {
		if !fs.processManager.IsProcessAlive(sessionInfo.PID) {
			return fmt.Errorf("session process (PID %d) is no longer alive", sessionInfo.PID)
		}

		// Verify process executable matches what we expect (with more lenient matching)
		processInfo, err := fs.processManager.FindProcess(sessionInfo.PID)
		if err != nil {
			return fmt.Errorf("failed to get process info: %w", err)
		}

		// More flexible process matching - allow for slight path differences
		if sessionInfo.ProcessPath != "" && !fs.processesCompatible(sessionInfo.ProcessPath, processInfo.Executable) {
			return fmt.Errorf("process executable mismatch: expected %s, got %s",
				sessionInfo.ProcessPath, processInfo.Executable)
		}
	}

	return nil
}

// processesCompatible checks if two process paths are compatible (more lenient matching)
func (fs *FileStore) processesCompatible(expected, actual string) bool {
	if expected == actual {
		return true
	}

	// Check basename compatibility
	expectedBase := filepath.Base(expected)
	actualBase := filepath.Base(actual)

	// Allow common browser process names
	browserProcesses := map[string]bool{
		"chrome":        true,
		"chromium":      true,
		"google-chrome": true,
		"msedge":        true,
		"node":          true,
	}

	return browserProcesses[expectedBase] && browserProcesses[actualBase]
}

// FindExistingSession finds an existing session that can be reattached to
func (fs *FileStore) FindExistingSession(serverName string) (*SessionInfo, error) {
	// First try exact name match
	sessionInfo, err := fs.LoadSessionByName(serverName)
	if err != nil {
		return nil, err
	}

	// Validate the session
	if err := fs.ValidateSession(sessionInfo); err != nil {
		// Session is invalid, delete it
		if deleteErr := fs.DeleteSession(sessionInfo.SessionID); deleteErr != nil {
			// Log deletion error but don't fail the response
			_ = deleteErr
		}
		return nil, fmt.Errorf("existing session is invalid: %w", err)
	}

	return sessionInfo, nil
}

// UpdateSessionStatus updates the status of a session
func (fs *FileStore) UpdateSessionStatus(sessionID string, status SessionStatus, errorMsg string) error {
	sessionInfo, err := fs.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	sessionInfo.Status = status
	sessionInfo.LastActivity = time.Now()
	if errorMsg != "" {
		sessionInfo.Error = errorMsg
	} else {
		sessionInfo.Error = ""
	}

	return fs.SaveSession(sessionInfo)
}

// UpdateSessionActivity updates the last activity time for a session
func (fs *FileStore) UpdateSessionActivity(sessionID string) error {
	sessionInfo, err := fs.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	sessionInfo.LastActivity = time.Now()
	return fs.SaveSession(sessionInfo)
}

// GenerateSessionID generates a unique session ID
func (fs *FileStore) GenerateSessionID(serverName string) string {
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	return fmt.Sprintf("%s-%s-%s", serverName, timestamp, randomString(6))
}

// sessionFilename returns the filename for a session
func (fs *FileStore) sessionFilename(sessionID string) string {
	return filepath.Join(fs.sessionsDir, sessionID+".json")
}

// randomString generates a cryptographically secure random string of the given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			// Fallback to timestamp-based (should rarely happen)
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		} else {
			b[i] = charset[n.Int64()]
		}
	}
	return string(b)
}
