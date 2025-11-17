package session

import (
	"strings"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
)

// DetectSessionType determines the appropriate session type for a server configuration
func DetectSessionType(serverConfig config.ServerConfig) SessionType {
	// HTTP servers are always stateless
	if serverConfig.Type == "http" || serverConfig.URL != "" {
		return Stateless
	}

	// Check for known browser-based servers that require persistent sessions
	command := strings.ToLower(serverConfig.Command)
	args := strings.Join(serverConfig.Args, " ")
	args = strings.ToLower(args)

	// Known browser automation servers
	browserServers := []string{
		"chrome-devtools",
		"playwright",
		"selenium",
		"puppeteer",
		"webdriver",
		"browser",
	}

	// Check command and arguments for browser server indicators
	for _, indicator := range browserServers {
		if strings.Contains(command, indicator) || strings.Contains(args, indicator) {
			return Persistent
		}
	}

	// Check for explicit session configuration
	if serverConfig.Session.Type != "" {
		switch serverConfig.Session.Type {
		case "persistent":
			return Persistent
		case "stateless":
			return Stateless
		case "hybrid":
			return Hybrid
		}
	}

	// Default stdio servers to hybrid (try persistent, fallback to stateless)
	return Hybrid
}

// RequiresPersistentSession checks if a server definitely requires a persistent session
func RequiresPersistentSession(serverConfig config.ServerConfig) bool {
	return DetectSessionType(serverConfig) == Persistent
}

// SupportsPersistentSession checks if a server can support persistent sessions
func SupportsPersistentSession(serverConfig config.ServerConfig) bool {
	sessionType := DetectSessionType(serverConfig)
	return sessionType == Persistent || sessionType == Hybrid
}

// ShouldAutoStart determines if a session should be automatically started
func ShouldAutoStart(serverConfig config.ServerConfig) bool {
	// Check explicit configuration
	if serverConfig.Session.AutoStart {
		return true
	}

	// Auto-start known browser servers
	return RequiresPersistentSession(serverConfig)
}

// GetSessionTimeout returns the session timeout in seconds
func GetSessionTimeout(serverConfig config.ServerConfig) int {
	if serverConfig.Session.Timeout > 0 {
		return serverConfig.Session.Timeout
	}

	// Default timeouts based on session type
	switch DetectSessionType(serverConfig) {
	case Persistent:
		return 300 // 5 minutes for browser servers
	case Hybrid:
		return 180 // 3 minutes for hybrid servers
	default:
		return 60 // 1 minute for stateless servers
	}
}