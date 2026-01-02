package daemon

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// startUnixSocket starts the daemon on a Unix domain socket
func (d *Daemon) startUnixSocket() error {
	// Remove existing socket file if it exists
	if err := os.Remove(d.endpoint); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	listener, err := net.Listen("unix", d.endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}

	// Set socket permissions for security
	if err := os.Chmod(d.endpoint, 0600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	go func() {
		if err := d.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// startNamedPipe starts the daemon on a Windows named pipe
func (d *Daemon) startNamedPipe() error {
	// Windows named pipe implementation would go here
	// For now, fall back to HTTP server on localhost
	return d.startHTTPServer()
}

// startHTTPServer starts the daemon on an HTTP port
func (d *Daemon) startHTTPServer() error {
	// If endpoint contains a port, use it directly
	if strings.Contains(d.endpoint, ":") {
		listener, err := net.Listen("tcp", d.endpoint)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", d.endpoint, err)
		}

		go func() {
			if err := d.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP server error: %v", err)
			}
		}()

		return nil
	}

	// Find an available port (original logic)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	// Update endpoint with actual address
	d.endpoint = listener.Addr().String()

	go func() {
		if err := d.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// writePIDFile writes the daemon PID to a file
func writePIDFile() error {
	pidFile := getPIDFilePath()
	pid := os.Getpid()

	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

// removePIDFile removes the daemon PID file
func removePIDFile() error {
	pidFile := getPIDFilePath()
	return os.Remove(pidFile)
}

// isDaemonRunning checks if the daemon is already running
func isDaemonRunning() (bool, int, error) {
	pidFile := getPIDFilePath()

	// Read PID file
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false, 0, fmt.Errorf("invalid PID file content: %w", err)
	}

	// Check if process is actually running
	if isProcessAlive(pid) {
		return true, pid, nil
	}

	// Process is dead, remove stale PID file
	_ = os.Remove(pidFile)
	return false, 0, nil
}

// isProcessAlive checks if a process with the given PID is alive
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	switch runtime.GOOS {
	case "windows":
		return isProcessAliveWindows(pid)
	default:
		return isProcessAliveUnix(pid)
	}
}

// isProcessAliveUnix checks if a process is alive on Unix-like systems
func isProcessAliveUnix(pid int) bool {
	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 doesn't actually kill the process, just checks if it exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isProcessAliveWindows checks if a process is alive on Windows
func isProcessAliveWindows(pid int) bool {
	// Use tasklist command to check if process exists
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// tasklist output includes headers, so check if PID appears in output
	return len(output) > 0 && contains(string(output), fmt.Sprintf("%d", pid))
}

// contains checks if a string contains a substring (case-sensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring is a simple substring finder for Windows compatibility
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// setupLogging configures logging for the daemon
func setupLogging() error {
	logFile := GetLogFilePath()

	// Open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Redirect both stdout and stderr to the log file
	// Note: This is a simplified approach. In production, you might want
	// to use a proper logging library with rotation
	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	return nil
}
