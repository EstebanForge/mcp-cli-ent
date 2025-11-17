package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// getDaemonEndpoint returns the appropriate daemon endpoint for the platform
func getDaemonEndpoint(platform string) string {
	switch platform {
	case "windows":
		return `\\.\pipe\mcp-cli-ent-daemon`
	case "wsl":
		// WSL uses Unix socket approach but with Windows path awareness
		return getWSLEndpoint()
	default: // linux, darwin
		return getUnixSocketEndpoint()
	}
}

// getUnixSocketEndpoint returns the Unix domain socket path
func getUnixSocketEndpoint() string {
	// For testing, use HTTP instead of Unix socket to avoid socket issues
	return "127.0.0.1:8080"

	// Original Unix socket logic (commented out for testing)
	/*
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to temp directory
		return "/tmp/mcp-cli-ent.sock"
	}

	daemonDir := filepath.Join(configDir, "mcp-cli-ent")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		// Fallback to temp directory
		return "/tmp/mcp-cli-ent.sock"
	}

	return filepath.Join(daemonDir, "daemon.sock")
	*/
}

// getWSLEndpoint returns the endpoint for WSL
func getWSLEndpoint() string {
	// WSL can use Unix sockets, but we need to be careful about path handling
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "/tmp/mcp-cli-ent-wsl.sock"
	}

	daemonDir := filepath.Join(configDir, "mcp-cli-ent")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		return "/tmp/mcp-cli-ent-wsl.sock"
	}

	return filepath.Join(daemonDir, "daemon-wsl.sock")
}

// isUnixSocket checks if the endpoint is a Unix domain socket
func isUnixSocket(endpoint string) bool {
	// Unix sockets typically don't contain colons and are file paths
	return !isNamedPipe(endpoint) && !strings.Contains(endpoint, ":") && !strings.HasPrefix(endpoint, "http://")
}

// isNamedPipe checks if the endpoint is a Windows named pipe
func isNamedPipe(endpoint string) bool {
	return len(endpoint) >= 9 && endpoint[:9] == `\\.\pipe\`
}

// getPIDFilePath returns the path to the daemon PID file
func getPIDFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to temp directory
		if runtime.GOOS == "windows" {
			return filepath.Join(os.TempDir(), "mcp-cli-ent-daemon.pid")
		}
		return "/tmp/mcp-cli-ent-daemon.pid"
	}

	daemonDir := filepath.Join(configDir, "mcp-cli-ent")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		// Fallback to temp directory
		if runtime.GOOS == "windows" {
			return filepath.Join(os.TempDir(), "mcp-cli-ent-daemon.pid")
		}
		return "/tmp/mcp-cli-ent-daemon.pid"
	}

	return filepath.Join(daemonDir, "daemon.pid")
}

// GetLogFilePath returns the path to the daemon log file
func GetLogFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to temp directory
		if runtime.GOOS == "windows" {
			return filepath.Join(os.TempDir(), "mcp-cli-ent-daemon.log")
		}
		return "/tmp/mcp-cli-ent-daemon.log"
	}

	daemonDir := filepath.Join(configDir, "mcp-cli-ent")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		// Fallback to temp directory
		if runtime.GOOS == "windows" {
			return filepath.Join(os.TempDir(), "mcp-cli-ent-daemon.log")
		}
		return "/tmp/mcp-cli-ent-daemon.log"
	}

	return filepath.Join(daemonDir, "daemon.log")
}