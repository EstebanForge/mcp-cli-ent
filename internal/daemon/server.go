package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
)

// setupRoutes configures the HTTP routes for the daemon
func (d *Daemon) setupRoutes(mux *http.ServeMux) {
	// Root endpoint for health check
	mux.HandleFunc("/", d.handleHealth)

	// Session management and tool execution endpoints (combined handler)
	mux.HandleFunc("/sessions", d.handleSessionAndToolActions)
	mux.HandleFunc("/sessions/", d.handleSessionAndToolActions)
}

// handleHealth handles the health check endpoint
func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := d.GetStatus()
	d.writeJSONResponse(w, APIResponse{
		Success: true,
		Data:    status,
	})
}

// handleSessionAndToolActions handles all session and tool operations
func (d *Daemon) handleSessionAndToolActions(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	pathParts := strings.Split(path, "/")

	// Handle /sessions (no server name)
	if len(pathParts) == 1 || (len(pathParts) == 2 && pathParts[1] == "") {
		d.handleSessionsList(w, r)
		return
	}

	// Handle /sessions/{server} and /sessions/{server}/{action}
	if len(pathParts) >= 2 {
		serverName := pathParts[1]

		// Check if it's a tool action: /sessions/{server}/call-tool/{tool}
		if len(pathParts) >= 4 && pathParts[2] == "call-tool" {
			d.handleToolCall(w, r, serverName, pathParts[3])
			return
		}

		// Handle other session actions
		d.handleSessionAction(w, r, serverName, pathParts[2:])
		return
	}

	http.Error(w, "Invalid request", http.StatusBadRequest)
}

// handleSessionsList handles session list operations
func (d *Daemon) handleSessionsList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions := d.ListSessions()
		d.writeJSONResponse(w, APIResponse{
			Success: true,
			Data:    sessions,
		})

	case http.MethodDelete:
		// Stop all sessions
		d.sessionMutex.Lock()
		for serverName, session := range d.sessions {
			if session.Client != nil {
				_ = session.Client.Close()
			}
			log.Printf("Stopped session: %s", serverName)
		}
		d.sessions = make(map[string]*PersistentSession)
		d.sessionMutex.Unlock()

		d.writeJSONResponse(w, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "All sessions stopped"},
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionAction handles individual session operations
func (d *Daemon) handleSessionAction(w http.ResponseWriter, r *http.Request, serverName string, actionParts []string) {
	action := ""
	if len(actionParts) > 0 {
		action = actionParts[0]
	}

	switch r.Method {
	case http.MethodPost:
		switch action {
		case "start":
			d.handleStartSession(w, r, serverName)
		case "tools":
			d.handleListSessionTools(w, r, serverName)
		default:
			http.Error(w, "Invalid session action", http.StatusBadRequest)
		}

	case http.MethodDelete:
		d.handleStopSession(w, r, serverName)

	case http.MethodGet:
		d.handleGetSession(w, r, serverName)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStartSession starts a new session
func (d *Daemon) handleStartSession(w http.ResponseWriter, r *http.Request, serverName string) {
	var req struct {
		Config config.ServerConfig `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	if err := d.StartSession(serverName, req.Config); err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	d.writeJSONResponse(w, APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Session starting", "server": serverName},
	})
}

// handleStopSession stops a session
func (d *Daemon) handleStopSession(w http.ResponseWriter, r *http.Request, serverName string) {
	if err := d.StopSession(serverName); err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	d.writeJSONResponse(w, APIResponse{
		Success: true,
		Data:    map[string]string{"message": "Session stopped", "server": serverName},
	})
}

// handleGetSession gets session information
func (d *Daemon) handleGetSession(w http.ResponseWriter, r *http.Request, serverName string) {
	session, err := d.GetSession(serverName)
	if err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	info := SessionInfo{
		ServerName: session.ServerName,
		Status:     session.Status.String(),
		StartTime:  session.StartTime,
		LastUsed:   session.LastUsed,
		Duration:   session.LastUsed.Sub(session.StartTime),
		Error:      session.Error,
		PID:        session.PID,
	}

	d.writeJSONResponse(w, APIResponse{
		Success: true,
		Data:    info,
	})
}

// handleListSessionTools lists tools for a session
func (d *Daemon) handleListSessionTools(w http.ResponseWriter, r *http.Request, serverName string) {
	tools, err := d.ListTools(serverName)
	if err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	d.writeJSONResponse(w, APIResponse{
		Success: true,
		Data:    tools,
	})
}

// handleToolCall handles tool execution operations
func (d *Daemon) handleToolCall(w http.ResponseWriter, r *http.Request, serverName, toolName string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Args map[string]interface{} `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	result, err := d.CallTool(serverName, toolName, req.Args)
	if err != nil {
		d.writeJSONResponse(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	d.writeJSONResponse(w, APIResponse{
		Success: true,
		Data:    result,
	})
}
