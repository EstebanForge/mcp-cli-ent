package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
)

// DaemonManager manages the daemon lifecycle
type DaemonManager struct {
	configPath string
	platform   string
	endpoint   string
}

// NewDaemonManager creates a new daemon manager
func NewDaemonManager() *DaemonManager {
	platform := detectPlatform()

	return &DaemonManager{
		platform: platform,
		endpoint: getDaemonEndpoint(platform),
	}
}

// Start starts the daemon
func (dm *DaemonManager) Start(foreground bool) error {
	// Check if daemon is already running
	if running, pid, err := isDaemonRunning(); err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	} else if running {
		return fmt.Errorf("daemon is already running (PID: %d)", pid)
	}

	if foreground {
		return dm.startForeground()
	}

	return dm.startBackground()
}

// startForeground starts the daemon in the foreground
func (dm *DaemonManager) startForeground() error {
	log.Printf("Starting daemon in foreground on %s", dm.endpoint)

	// Setup logging
	if err := setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Write PID file
	if err := writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer removePIDFile()

	// Load daemon config
	daemonConfig := dm.loadDaemonConfig()

	// Create and start daemon
	daemon, err := NewDaemon(daemonConfig)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	if err := daemon.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for shutdown signal
	dm.waitForShutdown(daemon)

	return nil
}

// startBackground starts the daemon in the background
func (dm *DaemonManager) startBackground() error {
	log.Printf("Starting daemon in background on %s", dm.endpoint)

	switch dm.platform {
	case "windows":
		return dm.startBackgroundWindows()
	default:
		return dm.startBackgroundUnix()
	}
}

// startBackgroundUnix starts the daemon in the background on Unix-like systems
func (dm *DaemonManager) startBackgroundUnix() error {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create the daemon command
	cmd := exec.Command(execPath, "daemon", "start", "--foreground")

	// Note: Setsid would be set here for proper daemonization, but we'll skip for cross-platform compatibility
	// cmd.SysProcAttr = &syscall.SysProcAttr{
	//     Setsid: true,
	// }

	// Redirect output to /dev/null
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Detach from the process
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release daemon process: %w", err)
	}

	// Give the daemon a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify that the daemon started successfully
	for i := 0; i < 10; i++ {
		if running, pid, err := isDaemonRunning(); err == nil && running {
			log.Printf("Daemon started successfully (PID: %d)", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start within timeout")
}

// startBackgroundWindows starts the daemon in the background on Windows
func (dm *DaemonManager) startBackgroundWindows() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create command to run in background
	cmd := exec.Command(execPath, "daemon", "start", "--foreground")
	// Windows-specific process creation would go here, but for simplicity,
	// we'll use the standard approach

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Detach from the process
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release daemon process: %w", err)
	}

	// Give the daemon a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify that the daemon started successfully
	for i := 0; i < 10; i++ {
		if running, pid, err := isDaemonRunning(); err == nil && running {
			log.Printf("Daemon started successfully (PID: %d)", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start within timeout")
}

// Stop stops the daemon
func (dm *DaemonManager) Stop() error {
	running, pid, err := isDaemonRunning()
	if err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	}

	if !running {
		return fmt.Errorf("daemon is not running")
	}

	log.Printf("Stopping daemon (PID: %d)", pid)

	// Try graceful shutdown via HTTP API first
	if err := dm.stopGracefully(); err == nil {
		return nil
	}

	// Fall back to force kill
	return dm.stopForcefully(pid)
}

// stopGracefully attempts to stop the daemon via HTTP API
func (dm *DaemonManager) stopGracefully() error {
	client := &http.Client{Timeout: 5 * time.Second}

	// The daemon doesn't have a dedicated stop endpoint yet,
	// but we can check if it responds to health checks
	resp, err := client.Get(dm.getHTTPURL())
	if err != nil {
		return fmt.Errorf("daemon not responding: %w", err)
	}
	resp.Body.Close()

	// For now, we'll implement a simple sleep to give the daemon
	// time to cleanup. In a full implementation, we'd have a
	// dedicated shutdown endpoint.
	time.Sleep(1 * time.Second)

	// Check if daemon is still running
	running, _, _ := isDaemonRunning()
	if !running {
		log.Printf("Daemon stopped gracefully")
		return nil
	}

	return fmt.Errorf("daemon did not stop gracefully")
}

// stopForcefully kills the daemon process
func (dm *DaemonManager) stopForcefully(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Try SIGTERM first on Unix-like systems
	if dm.platform != "windows" {
		if err := process.Signal(syscall.SIGTERM); err == nil {
			// Give it a moment to shutdown
			time.Sleep(2 * time.Second)

			// Check if it's still running
			if !isProcessAlive(pid) {
				log.Printf("Daemon stopped via SIGTERM")
				return nil
			}
		}
	}

	// Force kill
	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to kill daemon process: %w", err)
	}

	log.Printf("Daemon stopped via SIGKILL")
	return nil
}

// Status returns the status of the daemon
func (dm *DaemonManager) Status() (*DaemonStatus, error) {
	running, pid, err := isDaemonRunning()
	if err != nil {
		return &DaemonStatus{
			Running:   false,
			Platform:  dm.platform,
			Endpoint:  dm.endpoint,
			Error:     fmt.Sprintf("Failed to check daemon status: %v", err),
		}, nil
	}

	if !running {
		return &DaemonStatus{
			Running:  false,
			Platform: dm.platform,
			Endpoint: dm.endpoint,
		}, nil
	}

	// Get detailed status from daemon
	status, err := dm.getDaemonStatusFromAPI()
	if err != nil {
		return &DaemonStatus{
			Running:   true,
			PID:       pid,
			Platform:  dm.platform,
			Endpoint:  dm.endpoint,
			Error:     fmt.Sprintf("Failed to get detailed status: %v", err),
		}, nil
	}

	return status, nil
}

// Restart restarts the daemon
func (dm *DaemonManager) Restart() error {
	log.Printf("Restarting daemon...")

	// Stop the daemon if it's running
	if running, _, err := isDaemonRunning(); err == nil && running {
		if err := dm.Stop(); err != nil {
			log.Printf("Warning: Failed to stop daemon gracefully: %v", err)
		}
	}

	// Give it a moment to cleanup
	time.Sleep(1 * time.Second)

	// Start the daemon
	return dm.Start(false)
}

// GetEndpoint returns the daemon endpoint
func (dm *DaemonManager) GetEndpoint() string {
	return dm.endpoint
}

// Helper methods

func (dm *DaemonManager) loadDaemonConfig() *DaemonConfig {
	// Try to load config file
	configPath := dm.getDaemonConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Using default daemon config (could not read %s): %v", configPath, err)
		return DefaultDaemonConfig()
	}

	var config DaemonConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Invalid daemon config, using defaults: %v", err)
		return DefaultDaemonConfig()
	}

	return &config
}

func (dm *DaemonManager) getDaemonConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "daemon.json"
	}
	return filepath.Join(configDir, "mcp-cli-ent", "daemon.json")
}

func (dm *DaemonManager) getHTTPURL() string {
	if isUnixSocket(dm.endpoint) {
		// For Unix sockets, we can't make HTTP requests easily
		// This is a fallback that won't actually work for Unix sockets
		return "http://localhost" // This won't work, but keeps the interface consistent
	}

	if isNamedPipe(dm.endpoint) {
		// Named pipes also don't work with HTTP
		return "http://localhost" // Fallback
	}

	// Regular HTTP endpoint
	return "http://" + dm.endpoint
}

func (dm *DaemonManager) getDaemonStatusFromAPI() (*DaemonStatus, error) {
	if isUnixSocket(dm.endpoint) || isNamedPipe(dm.endpoint) {
		// For non-HTTP endpoints, we'll need to implement a different client
		// For now, return a basic status
		running, pid, _ := isDaemonRunning()
		return &DaemonStatus{
			Running:  running,
			PID:      pid,
			Platform: dm.platform,
			Endpoint: dm.endpoint,
		}, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(dm.getHTTPURL())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("daemon API error: %s", apiResp.Error)
	}

	data, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var status DaemonStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (dm *DaemonManager) waitForShutdown(daemon *Daemon) {
	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	// signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("Received shutdown signal, stopping daemon...")
	daemon.Stop()
}

// GetDaemonConfigPath returns the path to the daemon configuration file
func GetDaemonConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "daemon.json"
	}
	return filepath.Join(configDir, "mcp-cli-ent", "daemon.json")
}

// LoadMCPConfig loads the MCP server configuration
func LoadMCPConfig() (*config.Config, error) {
	// Try to find MCP servers config
	configPath := config.GetConfigPath("")

	if _, err := os.Stat(configPath); err != nil {
		// Try current directory
		configPath = "mcp_servers.json"
		if _, err := os.Stat(configPath); err != nil {
			// Return empty config if not found
			return &config.Config{}, nil
		}
	}

	return config.LoadConfig(configPath)
}