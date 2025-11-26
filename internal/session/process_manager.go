package session

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ProcessManager handles cross-platform process detection and management
type ProcessManager struct {
	platform string
}

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		platform: runtime.GOOS,
	}
}

// ProcessInfo contains information about a process
type ProcessInfo struct {
	PID        int       `json:"pid"`
	Executable string    `json:"executable"`
	Args       []string  `json:"args"`
	CmdLine    string    `json:"cmdLine"`
	ParentPID  int       `json:"parentPid,omitempty"`
	CreateTime time.Time `json:"createTime,omitempty"`
}

// IsProcessAlive checks if a process with the given PID is alive
func (pm *ProcessManager) IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	switch pm.platform {
	case "windows":
		return pm.isProcessAliveWindows(pid)
	default:
		return pm.isProcessAliveUnix(pid)
	}
}

// isProcessAliveUnix checks if process is alive on Unix-like systems
func (pm *ProcessManager) isProcessAliveUnix(pid int) bool {
	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Try to signal the process (doesn't actually kill it)
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isProcessAliveWindows checks if process is alive on Windows
func (pm *ProcessManager) isProcessAliveWindows(pid int) bool {
	// On Windows, we can use tasklist command
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), strconv.Itoa(pid))
}

// FindProcess finds a process by PID and returns detailed information
func (pm *ProcessManager) FindProcess(pid int) (*ProcessInfo, error) {
	if !pm.IsProcessAlive(pid) {
		return nil, fmt.Errorf("process %d is not alive", pid)
	}

	switch pm.platform {
	case "windows":
		return pm.findProcessWindows(pid)
	default:
		return pm.findProcessUnix(pid)
	}
}

// findProcessUnix gets process information on Unix-like systems
func (pm *ProcessManager) findProcessUnix(pid int) (*ProcessInfo, error) {
	// Read from /proc filesystem if available
	if _, err := os.Stat("/proc"); err == nil {
		return pm.findProcessProcFS(pid)
	}

	// Fallback to ps command
	return pm.findProcessPs(pid)
}

// findProcessProcFS reads process info from /proc filesystem
func (pm *ProcessManager) findProcessProcFS(pid int) (*ProcessInfo, error) {
	// Get executable
	execPath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		execPath = "unknown"
	}

	// Get command line
	cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read cmdline: %w", err)
	}

	cmdline := string(cmdlineBytes)
	// Replace null bytes with spaces for readability
	cmdline = strings.ReplaceAll(cmdline, "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)

	// Get stat info for parent PID and creation time
	statBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read stat: %w", err)
	}

	statFields := strings.Fields(string(statBytes))
	if len(statFields) < 4 {
		return nil, fmt.Errorf("invalid stat format")
	}

	parentPID := 0
	if len(statFields) > 3 {
		if ppid, err := strconv.Atoi(statFields[3]); err == nil {
			parentPID = ppid
		}
	}

	// Parse creation time (field 22 in stat)
	var createTime time.Time
	if len(statFields) > 21 {
		if clockTicks, err := strconv.Atoi(statFields[21]); err == nil {
			// Convert clock ticks to seconds (rough approximation)
			secondsSinceBoot := clockTicks / 100 // Assuming 100Hz clock
			createTime = time.Now().Add(-time.Duration(secondsSinceBoot) * time.Second)
		}
	}

	return &ProcessInfo{
		PID:        pid,
		Executable: execPath,
		Args:       strings.Fields(cmdline),
		CmdLine:    cmdline,
		ParentPID:  parentPID,
		CreateTime: createTime,
	}, nil
}

// findProcessPs uses ps command to get process info
func (pm *ProcessManager) findProcessPs(pid int) (*ProcessInfo, error) {
	// Use ps command to get process information
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid,ppid,command", "--no-headers")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ps command failed: %w", err)
	}

	fields := strings.Fields(string(output))
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid ps output format")
	}

	parsedPid, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("invalid PID: %w", err)
	}

	if parsedPid != pid {
		return nil, fmt.Errorf("PID mismatch")
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		ppid = 0
	}

	cmdline := strings.Join(fields[2:], " ")

	return &ProcessInfo{
		PID:        parsedPid,
		Executable: fields[2],
		Args:       fields[2:],
		CmdLine:    cmdline,
		ParentPID:  ppid,
		CreateTime: time.Now(), // Best effort
	}, nil
}

// findProcessWindows gets process information on Windows
func (pm *ProcessManager) findProcessWindows(pid int) (*ProcessInfo, error) {
	// Use wmic command to get detailed process information
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid),
		"get", "ProcessId,ParentProcessId,CommandLine,ExecutablePath", "/format:csv")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wmic command failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("no process information found")
	}

	// Skip header line and empty lines
	var dataLine string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			dataLine = line
			break
		}
	}

	if dataLine == "" {
		return nil, fmt.Errorf("no valid process data found")
	}

	fields := strings.Split(dataLine, ",")
	if len(fields) < 5 {
		return nil, fmt.Errorf("invalid wmic output format")
	}

	processPID, err := strconv.Atoi(strings.TrimSpace(fields[1]))
	if err != nil || processPID != pid {
		return nil, fmt.Errorf("PID mismatch")
	}

	parentPID := 0
	if ppid, err := strconv.Atoi(strings.TrimSpace(fields[2])); err == nil {
		parentPID = ppid
	}

	cmdline := strings.TrimSpace(fields[3])
	execPath := strings.TrimSpace(fields[4])

	return &ProcessInfo{
		PID:        pid,
		Executable: execPath,
		Args:       strings.Fields(cmdline),
		CmdLine:    cmdline,
		ParentPID:  parentPID,
		CreateTime: time.Now(), // Best effort
	}, nil
}

// GetProcessChildren finds all child processes of the given PID
func (pm *ProcessManager) GetProcessChildren(pid int) ([]int, error) {
	switch pm.platform {
	case "windows":
		return pm.getProcessChildrenWindows(pid)
	default:
		return pm.getProcessChildrenUnix(pid)
	}
}

// getProcessChildrenUnix finds child processes on Unix-like systems
func (pm *ProcessManager) getProcessChildrenUnix(pid int) ([]int, error) {
	// Use pgrep to find child processes
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 if no processes found, which is OK
		return []int{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var children []int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if childPID, err := strconv.Atoi(line); err == nil {
			children = append(children, childPID)
		}
	}

	return children, nil
}

// getProcessChildrenWindows finds child processes on Windows
func (pm *ProcessManager) getProcessChildrenWindows(pid int) ([]int, error) {
	// Use wmic to find child processes
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ParentProcessId=%d", pid),
		"get", "ProcessId", "/format:csv")

	output, err := cmd.Output()
	if err != nil {
		return []int{}, nil
	}

	lines := strings.Split(string(output), "\n")
	var children []int

	for _, line := range lines[1:] { // Skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) >= 2 {
			if childPID, err := strconv.Atoi(strings.TrimSpace(fields[1])); err == nil {
				children = append(children, childPID)
			}
		}
	}

	return children, nil
}

// TerminateProcessTree terminates a process and all its children
func (pm *ProcessManager) TerminateProcessTree(pid int) error {
	if !pm.IsProcessAlive(pid) {
		return nil // Already dead
	}

	// First, find and terminate children
	children, err := pm.GetProcessChildren(pid)
	if err != nil {
		return fmt.Errorf("failed to get child processes: %w", err)
	}

	// Terminate children recursively
	for _, childPID := range children {
		if err := pm.TerminateProcessTree(childPID); err != nil {
			// Log but continue with other children
			fmt.Printf("Warning: failed to terminate child process %d: %v\n", childPID, err)
		}
	}

	// Finally, terminate the main process
	return pm.TerminateProcess(pid)
}

// TerminateProcess terminates a single process
func (pm *ProcessManager) TerminateProcess(pid int) error {
	if !pm.IsProcessAlive(pid) {
		return nil // Already dead
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Try graceful termination first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		if err := process.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to terminate process %d: %w", pid, err)
		}
	}

	// Wait a bit for process to actually terminate
	time.Sleep(100 * time.Millisecond)

	// Check if it's still alive
	if pm.IsProcessAlive(pid) {
		// Force kill if still alive
		if err := process.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to force kill process %d: %w", pid, err)
		}
	}

	return nil
}

// FindBrowserProcesses finds processes that are likely browser automation servers
func (pm *ProcessManager) FindBrowserProcesses() ([]*ProcessInfo, error) {
	var browserProcesses []*ProcessInfo

	// Common browser automation process patterns
	patterns := []string{
		"node.*playwright",
		"node.*puppeteer",
		"chrome.*remote-debugging",
		"chromium.*remote-debugging",
		"msedge.*remote-debugging",
		"playwright",
		"puppeteer",
	}

	for _, pattern := range patterns {
		processes, err := pm.findProcessesByPattern(pattern)
		if err != nil {
			continue // Skip patterns that don't work on this platform
		}
		browserProcesses = append(browserProcesses, processes...)
	}

	return browserProcesses, nil
}

// findProcessesByPattern finds processes matching a pattern
func (pm *ProcessManager) findProcessesByPattern(pattern string) ([]*ProcessInfo, error) {
	switch pm.platform {
	case "windows":
		return pm.findProcessesByPatternWindows(pattern)
	default:
		return pm.findProcessesByPatternUnix(pattern)
	}
}

// findProcessesByPatternUnix finds processes by pattern on Unix-like systems
func (pm *ProcessManager) findProcessesByPatternUnix(pattern string) ([]*ProcessInfo, error) {
	cmd := exec.Command("pgrep", "-f", pattern)
	output, err := cmd.Output()
	if err != nil {
		return []*ProcessInfo{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var processes []*ProcessInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pid, err := strconv.Atoi(line); err == nil {
			if procInfo, err := pm.FindProcess(pid); err == nil {
				processes = append(processes, procInfo)
			}
		}
	}

	return processes, nil
}

// findProcessesByPatternWindows finds processes by pattern on Windows
func (pm *ProcessManager) findProcessesByPatternWindows(pattern string) ([]*ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s*", pattern), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return []*ProcessInfo{}, nil
	}

	lines := strings.Split(string(output), "\n")
	var processes []*ProcessInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse CSV format: "process.exe","PID","session","session#","mem usage"
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}

		// Extract PID from second field, removing quotes
		pidStr := strings.Trim(fields[1], "\"")
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if procInfo, err := pm.FindProcess(pid); err == nil {
				processes = append(processes, procInfo)
			}
		}
	}

	return processes, nil
}
