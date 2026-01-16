package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mcp-cli-ent/mcp-cli/internal/client"
	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/daemon"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
	"github.com/mcp-cli-ent/mcp-cli/internal/session"
	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

// Global session manager singleton
var (
	globalSessionManager  *session.Manager
	sessionManagerOnce    sync.Once
	sessionManagerInitErr error
	VerboseMode           bool
)

var listServersCmd = &cobra.Command{
	Use:   "list-servers",
	Short: "List enabled MCP servers",
	Long: `List enabled MCP servers with their status, type, and configuration details.
Use --all to include disabled servers.`,
	RunE: runListServers,
}

func init() {
	// Add local flags for list-servers command
	listServersCmd.Flags().BoolVar(&showAllServers, "all", false, "show disabled servers as well")
}

var showAllServers bool

var listToolsCmd = &cobra.Command{
	Use:   "list-tools [server-name]",
	Short: "List tools from MCP servers",
	Long: `List available tools from MCP servers.
If server-name is provided, lists tools from that server only.
If omitted, lists tools from all enabled servers.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runListTools,
}

var callToolCmd = &cobra.Command{
	Use:   "call-tool <server-name> <tool-name> [arguments]",
	Short: "Call a specific tool on an MCP server",
	Long: `Call a specific tool on an MCP server with optional JSON arguments.
Arguments should be a valid JSON string, e.g., '{"libraryName": "react"}'`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runCallTool,
}

var requestInputCmd = &cobra.Command{
	Use:   "request-input <server-name> [message] [schema]",
	Short: "Request input from user via MCP server elicitation",
	Long: `Request specific information from the user through MCP server elicitation.
This enables servers to dynamically gather information during interactions.`,
	Args: cobra.RangeArgs(1, 3),
	RunE: runRequestInput,
}

var createMessageCmd = &cobra.Command{
	Use:   "create-message <server-name> [messages]",
	Short: "Create LLM message via MCP server sampling",
	Long: `Request LLM completions through MCP server sampling capability.
This enables agentic workflows where servers can request AI assistance.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCreateMessage,
}

var initializeCmd = &cobra.Command{
	Use:   "initialize <server-name>",
	Short: "Initialize connection with an MCP server",
	Long: `Initialize the MCP protocol connection with a server.
This negotiates capabilities and establishes the client-server relationship.`,
	Args: cobra.ExactArgs(1),
	RunE: runInitialize,
}

var createConfigCmd = &cobra.Command{
	Use:   "create-config [filename]",
	Short: "Create an example configuration file",
	Long: `Create an example mcp_servers.json configuration file with sample server configurations.
If filename is omitted, creates 'mcp_servers.json' in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreateConfig,
}

// Session management commands
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage MCP server sessions",
	Long: `Manage persistent MCP server sessions for browser-based servers.
Allows starting, stopping, and monitoring sessions that maintain state across commands.`,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active sessions",
	Long: `List all active MCP server sessions with their status, type, and activity information.
Shows both persistent and stateless sessions.`,
	RunE: runSessionList,
}

var sessionStatusCmd = &cobra.Command{
	Use:   "status <server-name>",
	Short: "Show status of a specific session",
	Long: `Show detailed status information for a specific MCP server session,
including health status, uptime, and activity metrics.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionStatus,
}

var sessionStartCmd = &cobra.Command{
	Use:   "start <server-name>",
	Short: "Start a persistent session",
	Long: `Start a persistent session for the specified MCP server.
This is useful for browser-based servers that need to maintain state across commands.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionStart,
}

var sessionStopCmd = &cobra.Command{
	Use:   "stop <server-name>",
	Short: "Stop a specific session",
	Long: `Stop a specific MCP server session and clean up its resources.
This will terminate any browser processes or other persistent connections.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionStop,
}

var sessionRestartCmd = &cobra.Command{
	Use:   "restart <server-name>",
	Short: "Restart a specific session",
	Long: `Restart a specific MCP server session.
This stops and then starts the session, useful for troubleshooting or recovering from errors.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionRestart,
}

var sessionCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up dead or expired sessions",
	Long: `Clean up dead, expired, or unhealthy sessions.
This removes sessions that are no longer responding or have exceeded their idle timeout.`,
	RunE: runSessionCleanup,
}

var sessionAttachCmd = &cobra.Command{
	Use:   "attach <server-name>",
	Short: "Attach to an existing session",
	Long: `Attach to an existing MCP server session.
This attempts to reconnect to a previously started persistent session, allowing you to resume work with browser-based servers without restarting them.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionAttach,
}

// Daemon command and subcommands
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the MCP daemon",
	Long: `Manage the MCP daemon that provides persistent sessions and cross-command state.
The daemon runs in the background and maintains MCP server connections, enabling features like persistent browser sessions.`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start [--foreground]",
	Short: "Start the MCP daemon",
	Long: `Start the MCP daemon in the background (default) or foreground.
The daemon provides persistent sessions for MCP servers, especially useful for browser-based servers like Chrome DevTools.`,
	RunE: runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the MCP daemon",
	Long: `Stop the running MCP daemon and all its active sessions.
This will terminate all persistent MCP server connections.`,
	RunE: runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show MCP daemon status",
	Long:  `Display the current status of the MCP daemon, including active sessions and system information.`,
	RunE:  runDaemonStatus,
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the MCP daemon",
	Long:  `Restart the MCP daemon. This will stop all current sessions and start fresh ones.`,
	RunE:  runDaemonRestart,
}

var daemonLogsCmd = &cobra.Command{
	Use:   "logs [--tail <lines>]",
	Short: "Show MCP daemon logs",
	Long:  `Display the logs from the MCP daemon. Use --tail to show only the last N lines.`,
	RunE:  runDaemonLogs,
}

// Daemon flags
var daemonForeground bool
var daemonLogsTail int

func init() {
	// Add daemon command flags
	daemonStartCmd.Flags().BoolVar(&daemonForeground, "foreground", false, "Run daemon in foreground instead of background")
	daemonLogsCmd.Flags().IntVar(&daemonLogsTail, "tail", 50, "Number of lines to show from the end of the log file")

	// Add list-tools command (flags are now global: --refresh, --clear-cache)
	rootCmd.AddCommand(listServersCmd)
	rootCmd.AddCommand(listToolsCmd)
	rootCmd.AddCommand(callToolCmd)
	rootCmd.AddCommand(requestInputCmd)
	rootCmd.AddCommand(createMessageCmd)
	rootCmd.AddCommand(initializeCmd)
	rootCmd.AddCommand(createConfigCmd)

	// Add session management commands
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionStatusCmd)
	sessionCmd.AddCommand(sessionStartCmd)
	sessionCmd.AddCommand(sessionAttachCmd)
	sessionCmd.AddCommand(sessionStopCmd)
	sessionCmd.AddCommand(sessionRestartCmd)
	sessionCmd.AddCommand(sessionCleanupCmd)
	rootCmd.AddCommand(sessionCmd)

	// Add daemon management commands
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonCmd.AddCommand(daemonLogsCmd)
	rootCmd.AddCommand(daemonCmd)

	// Add version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			version.PrintVersion()
		},
	}
	rootCmd.AddCommand(versionCmd)
}

// displayServerNotFoundError shows available servers when a server name is not found
func displayServerNotFoundError(serverName string, cfg *config.Configuration) {
	// Show error with available servers
	fmt.Fprintf(os.Stderr, "Error: server '%s' not found in configuration\n\n", serverName)

	// Display available servers to help the agent
	enabledServers := cfg.GetEnabledServers()
	if len(enabledServers) > 0 {
		fmt.Fprintf(os.Stderr, "Available MCP servers (%d):\n", len(enabledServers))
		for name, config := range enabledServers {
			if config.Description != "" {
				fmt.Fprintf(os.Stderr, "  • %s | %s\n", name, config.Description)
			} else {
				fmt.Fprintf(os.Stderr, "  • %s\n", name)
			}
		}
		fmt.Fprintf(os.Stderr, "\n💡 Use 'mcp-cli-ent list-servers' to see all configured servers\n")
	} else {
		fmt.Fprintf(os.Stderr, "No enabled MCP servers found.\n")
		fmt.Fprintf(os.Stderr, "💡 Run 'mcp-cli-ent create-config' to create a sample configuration\n")
	}
}

func runListServers(cmd *cobra.Command, args []string) error {
	configPath := GetConfigPath()

	// Load configuration
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return err
	}

	// Get server statuses
	statuses := cfg.GetServerStatus()

	// Filter servers based on --all flag
	var filteredStatuses []config.ServerStatus
	for _, status := range statuses {
		if showAllServers || status.Status == "enabled" {
			filteredStatuses = append(filteredStatuses, status)
		}
	}

	if len(filteredStatuses) == 0 {
		if showAllServers {
			fmt.Println("No MCP servers configured.")
		} else {
			fmt.Println("No enabled MCP servers found.")
			fmt.Println("Use --all to see disabled servers.")
		}
		return nil
	}

	if showAllServers {
		fmt.Printf("All configured MCP servers (%d):\n", len(filteredStatuses))
	} else {
		fmt.Printf("Enabled MCP servers (%d):\n", len(filteredStatuses))
	}

	for _, status := range filteredStatuses {
		statusIcon := "✓"
		if status.Status == "disabled" {
			statusIcon = "✗"
		}

		// Get server config to determine session type and description
		serverConfig, exists := cfg.GetServer(status.Name)
		sessionInfo := ""
		description := ""
		if exists {
			sessionType := session.DetectSessionType(serverConfig)
			if sessionType == session.Persistent {
				sessionInfo = " [persistent]"
			}
			if serverConfig.Description != "" {
				description = " | " + serverConfig.Description
			}
		}

		// Show status only when --all flag is used (to reduce context pollution by default)
		var statusLabel string
		if showAllServers {
			statusLabel = fmt.Sprintf(" [%s]", status.Status)
		}

		fmt.Printf("  %s %s%s%s%s | %s\n", statusIcon, status.Name, statusLabel, sessionInfo, description, status.Details)
	}

	return nil
}

func runListTools(cmd *cobra.Command, args []string) error {
	// Initialize verbose mode to set environment variable
	_ = isVerbose()

	configPath := GetConfigPath()

	// Load configuration
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return err
	}

	ctx := context.Background()

	if len(args) == 0 {
		// Show all tools from all servers with usage examples (same behavior as root command)
		return showRootHelpWithServers(cmd)
	} else {
		// List tools from specific server
		serverName := args[0]
		serverConfig, exists := cfg.GetServer(serverName)
		if !exists {
			displayServerNotFoundError(serverName, cfg)
			return nil
		}

		if !serverConfig.IsEnabled() {
			return fmt.Errorf("server '%s' is disabled", serverName)
		}

		// Display server description if available
		if serverConfig.Description != "" {
			fmt.Printf("%s - %s\n\n", serverName, serverConfig.Description)
		}

		return listToolsFromServer(ctx, serverName, serverConfig)
	}
}

func listToolsFromServer(ctx context.Context, serverName string, serverConfig config.ServerConfig) error {
	// Ensure verbose mode is set (called from session management)
	_ = isVerbose()

	// If clearCache is set, clear cache file before proceeding
	if clearCache {
		cachePath, err := GetCachePath()
		if err == nil {
			_ = os.Remove(cachePath)
			fmt.Println("Cache cleared.")
		}
	}

	// Try to load from cache first (unless forced refresh or cache was cleared)
	var tools []mcp.Tool
	if !refreshCache && !clearCache {
		cache, err := LoadToolsFromCache()
		if err == nil && cache != nil {
			if entry, ok := cache.Servers[serverName]; ok && time.Since(entry.LastUpdate) < CacheTTL {
				tools = entry.Tools
			}
		}
	}

	// If not in cache or forced refresh, fetch from server
	if tools == nil {
		// Create session-aware client factory
		factory, err := getSessionAwareClientFactory()
		if err != nil {
			return fmt.Errorf("failed to create client factory: %w", err)
		}

		// Create session-aware client
		mcpClient, err := factory.CreateClient(serverName, serverConfig)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer func() { _ = mcpClient.Close() }()

		// List tools
		tools, err = mcpClient.ListTools(ctx)
		if err != nil {
			return fmt.Errorf("failed to list tools: %w", err)
		}

		// Update cache
		cache, err := LoadToolsFromCache()
		if err != nil || cache == nil {
			cache = &ToolsCache{Servers: make(map[string]ToolsCacheEntry)}
		}
		cache.Servers[serverName] = ToolsCacheEntry{
			Tools:      tools,
			LastUpdate: time.Now(),
		}
		_ = SaveToolsToCache(cache)
	}

	if len(tools) == 0 {
		fmt.Println("No tools found.")
		return nil
	}

	fmt.Printf("Available tools (%d):\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  • %s\n", tool.Name)
		if tool.Description != "" {
			fmt.Printf("    desc: %s\n", tool.Description)
		}
		if tool.InputSchema != nil {
			if properties, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
				var paramNames []string
				for name := range properties {
					paramNames = append(paramNames, name)
				}
				if len(paramNames) > 0 {
					fmt.Printf("    params: %s\n", strings.Join(paramNames, ", "))
				}
			}
		}
		// Build and display call example
		exampleArgs := BuildExampleArgs(&tool)
		if verbose {
			if exampleArgs == "'{}'" {
				fmt.Printf("    call: mcp-cli-ent call-tool %s %s\n\n", serverName, tool.Name)
			} else {
				fmt.Printf("    call: mcp-cli-ent call-tool %s %s %s\n\n", serverName, tool.Name, exampleArgs)
			}
		} else {
			if exampleArgs == "'{}'" {
				fmt.Printf("    call: mcp-cli-ent call-tool %s %s\n\n", serverName, tool.Name)
			} else {
				fmt.Printf("    call: mcp-cli-ent call-tool %s %s %s\n\n", serverName, tool.Name, exampleArgs)
			}
		}
	}

	return nil
}

func runCallTool(cmd *cobra.Command, args []string) error {
	configPath := GetConfigPath()

	// Load configuration
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return err
	}

	serverName := args[0]
	toolName := args[1]
	var arguments map[string]interface{}

	if len(args) >= 3 {
		// Parse arguments JSON
		if err := json.Unmarshal([]byte(args[2]), &arguments); err != nil {
			return fmt.Errorf("invalid JSON arguments: %w", err)
		}
	} else {
		// Initialize as empty object if no arguments provided
		arguments = make(map[string]interface{})
	}

	// Get server configuration
	serverConfig, exists := cfg.GetServer(serverName)
	if !exists {
		displayServerNotFoundError(serverName, cfg)
		return nil
	}

	if !serverConfig.IsEnabled() {
		return fmt.Errorf("server '%s' is disabled", serverName)
	}

	// Create smart client that uses daemon when appropriate
	smartClient := daemon.NewSmartClient()

	// Create client (will use daemon if persistent, direct connection otherwise)
	mcpClient, err := smartClient.CreateClient(serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer func() { _ = mcpClient.Close() }()

	// Call tool
	ctx := context.Background()
	result, err := mcpClient.CallTool(ctx, toolName, arguments)
	if err != nil {
		return fmt.Errorf("failed to call tool: %w", err)
	}

	// Handle result display with binary data detection
	displayToolResult(result)
	return nil
}

func runCreateConfig(cmd *cobra.Command, args []string) error {
	var filename string
	if len(args) > 0 {
		filename = args[0]
	} else {
		// Use standard config location when no filename specified
		configDir, err := config.GetConfigDir()
		if err != nil {
			return fmt.Errorf("failed to determine config directory: %w", err)
		}
		filename = filepath.Join(configDir, "mcp_servers.json")
	}

	// Check if file already exists
	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("file '%s' already exists", filename)
	}

	// Create example config
	if err := config.CreateExampleConfig(filename); err != nil {
		return fmt.Errorf("failed to create example config: %w", err)
	}

	fmt.Printf("Created example configuration file: %s\n", filename)
	fmt.Println("Edit this file with your MCP server configurations.")
	return nil
}

func GetConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}

	// Try to find config file in standard locations
	if path, err := config.FindConfigFile(); err == nil {
		return path
	}

	return "mcp_servers.json" // Default fallback
}

func LoadConfiguration(configPath string) (*config.Configuration, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration from '%s': %w", configPath, err)
	}
	return cfg, nil
}

// displayToolResult intelligently displays tool results, handling binary data gracefully
func displayToolResult(result *mcp.ToolResult) {
	if result == nil {
		fmt.Println("Result: null")
		return
	}

	fmt.Println("Result:")
	if result.IsError {
		fmt.Printf("Error: %v\n", result.IsError)
	}

	if len(result.Content) == 0 {
		fmt.Println("No content returned")
		return
	}

	for i, content := range result.Content {
		fmt.Printf("Content %d:\n", i+1)

		// Handle content as a map (typical for MCP responses)
		if contentMap, ok := content.(map[string]interface{}); ok {
			// Check if this is binary data
			if dataType, hasType := contentMap["type"].(string); hasType {
				switch dataType {
				case "image", "binary":
					displayBinaryContent(contentMap)
					continue
				}
			}

			// Check for data field (base64 encoded content)
			if data, hasData := contentMap["data"].(string); hasData {
				if isBase64Image(data) {
					displayBinaryContent(contentMap)
					continue
				}
				// For other base64 data, show summary
				fmt.Printf("  Data (base64): %d bytes\n", len(data))
				if len(data) <= 100 {
					fmt.Printf("  Content: %s\n", data)
				} else {
					truncLen := 50
					if len(data) < truncLen {
						truncLen = len(data)
					}
					fmt.Printf("  Content: %s... (truncated)\n", data[:truncLen])
				}
				continue
			}

			// Check for text field
			if text, hasText := contentMap["text"].(string); hasText {
				fmt.Printf("  Text: %s\n", text)
				continue
			}

			// Generic object display - show structure but truncate large values
			displayMapStructure(contentMap, "  ")
			continue
		}

		// Handle content as a simple string or other primitive
		switch v := content.(type) {
		case string:
			if len(v) > 200 {
				fmt.Printf("  String (length %d): %s... (truncated)\n", len(v), v[:100])
			} else {
				fmt.Printf("  String: %s\n", v)
			}
		default:
			// Convert to JSON for display
			if jsonBytes, err := json.MarshalIndent(v, "  ", "  "); err == nil {
				fmt.Printf("  %s", string(jsonBytes))
			} else {
				fmt.Printf("  %v\n", v)
			}
		}
	}
}

// displayBinaryContent handles binary data (images, files) with user-friendly output
func displayBinaryContent(content map[string]interface{}) {
	data, hasData := content["data"].(string)
	mimeType, hasMimeType := content["mimeType"].(string)

	fmt.Printf("  Binary data detected:\n")
	if hasMimeType {
		fmt.Printf("    MIME type: %s\n", mimeType)
	}
	if hasData {
		fmt.Printf("    Size: %d bytes (base64 encoded)\n", len(data))

		// Try to decode base64 to get actual size
		if decoded, err := base64.StdEncoding.DecodeString(data); err == nil {
			fmt.Printf("    Actual size: %d bytes\n", len(decoded))

			// For images, show dimensions if it's a PNG/JPEG
			if strings.HasPrefix(mimeType, "image/") {
				if strings.HasPrefix(mimeType, "image/png") && len(decoded) > 8 {
					// Basic PNG dimension detection
					if len(decoded) >= 24 {
						width := int(decoded[16])<<24 | int(decoded[17])<<16 | int(decoded[18])<<8 | int(decoded[19])
						height := int(decoded[20])<<24 | int(decoded[21])<<16 | int(decoded[22])<<8 | int(decoded[23])
						fmt.Printf("    Image dimensions: %dx%d (PNG)\n", width, height)
					}
				} else if strings.HasPrefix(mimeType, "image/jpeg") && len(decoded) > 4 {
					// Basic JPEG detection (look for SOF markers)
					fmt.Printf("    Image format: JPEG\n")
				}
			}
		}

		truncLen := 50
		if len(data) < truncLen {
			truncLen = len(data)
		}
		fmt.Printf("    Data: %s... (base64 data truncated)\n", data[:truncLen])
	}

	// Show other fields
	for key, value := range content {
		if key != "data" && key != "type" && key != "mimeType" {
			fmt.Printf("    %s: %v\n", key, value)
		}
	}

	fmt.Printf("    💡 Tip: Use a file save option or script to extract binary data\n")
}

// isBase64Image checks if a string appears to be base64-encoded image data
func isBase64Image(s string) bool {
	// Basic base64 pattern check
	base64Pattern := regexp.MustCompile(`^[A-Za-z0-9+/]+=*$`)
	if !base64Pattern.MatchString(s) || len(s) < 100 {
		return false
	}

	// Try to decode and check for common image signatures
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		if len(decoded) >= 4 {
			// PNG signature: 89 50 4E 47
			if len(decoded) >= 8 &&
				decoded[0] == 0x89 && decoded[1] == 0x50 &&
				decoded[2] == 0x4E && decoded[3] == 0x47 {
				return true
			}
			// JPEG signature: FF D8 FF
			if decoded[0] == 0xFF && decoded[1] == 0xD8 && decoded[2] == 0xFF {
				return true
			}
			// WebP signature: RIFF...WEBP
			if len(decoded) >= 12 &&
				string(decoded[0:4]) == "RIFF" &&
				string(decoded[8:12]) == "WEBP" {
				return true
			}
		}
	}

	return false
}

// displayMapStructure recursively displays map structure with truncation for large values
func displayMapStructure(m map[string]interface{}, indent string) {
	for key, value := range m {
		switch v := value.(type) {
		case string:
			if len(v) > 100 {
				fmt.Printf("%s%s: \"[string length %d - truncated]\"\n", indent, key, len(v))
			} else {
				fmt.Printf("%s%s: %q\n", indent, key, v)
			}
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, key)
			displayMapStructure(v, indent+"  ")
		case []interface{}:
			fmt.Printf("%s%s: [array with %d items]\n", indent, key, len(v))
			if len(v) > 0 && len(v) <= 3 {
				for i, item := range v {
					fmt.Printf("%s  [%d]: ", indent, i)
					if itemMap, ok := item.(map[string]interface{}); ok {
						displayMapStructure(itemMap, indent+"    ")
					} else {
						fmt.Printf("%v\n", item)
					}
				}
			}
		default:
			fmt.Printf("%s%s: %v\n", indent, key, v)
		}
	}
}

func runInitialize(cmd *cobra.Command, args []string) error {
	configPath := GetConfigPath()

	// Load configuration
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return err
	}

	serverName := args[0]

	// Get server configuration
	serverConfig, exists := cfg.GetServer(serverName)
	if !exists {
		displayServerNotFoundError(serverName, cfg)
		return nil
	}

	if !serverConfig.IsEnabled() {
		return fmt.Errorf("server '%s' is disabled", serverName)
	}

	// Create session-aware client factory
	factory, err := getSessionAwareClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create client factory: %w", err)
	}

	// Create session-aware client
	mcpClient, err := factory.CreateClient(serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer func() { _ = mcpClient.Close() }()

	// Initialize connection
	ctx := context.Background()

	// Create initialization parameters
	initParams := &mcp.InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: mcp.ClientCapabilities{
			Experimental: make(map[string]interface{}),
			Sampling:     &mcp.SamplingCapability{},
			Roots:        &mcp.RootsCapability{},
		},
		ClientInfo: mcp.ClientInfo{
			Name:    "mcp-cli-ent",
			Version: "0.1.0",
		},
	}

	result, err := mcpClient.Initialize(ctx, initParams)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Pretty print result
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println("Initialization successful:")
	fmt.Println(string(resultBytes))
	return nil
}

func runRequestInput(cmd *cobra.Command, args []string) error {
	configPath := GetConfigPath()

	// Load configuration
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return err
	}

	serverName := args[0]

	// Get server configuration
	serverConfig, exists := cfg.GetServer(serverName)
	if !exists {
		return fmt.Errorf("server '%s' not found in configuration", serverName)
	}

	if !serverConfig.IsEnabled() {
		return fmt.Errorf("server '%s' is disabled", serverName)
	}

	// Create session-aware client factory
	factory, err := getSessionAwareClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create client factory: %w", err)
	}

	// Create session-aware client
	mcpClient, err := factory.CreateClient(serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer func() { _ = mcpClient.Close() }()

	// Prepare parameters
	params := &mcp.RequestInputParams{}

	if len(args) >= 2 {
		params.Message = args[1]
	} else {
		params.Message = "Please provide the requested information:"
	}

	if len(args) >= 3 {
		// Parse schema JSON
		if err := json.Unmarshal([]byte(args[2]), &params.Schema); err != nil {
			return fmt.Errorf("invalid schema JSON: %w", err)
		}
	} else {
		// Default simple schema
		params.Schema = map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"response": map[string]interface{}{
					"type":        "string",
					"description": "Your response",
				},
			},
			"required": []interface{}{"response"},
		}
	}

	// Request input
	ctx := context.Background()
	result, err := mcpClient.RequestInput(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to request input: %w", err)
	}

	// Pretty print result
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println("Input received:")
	fmt.Println(string(resultBytes))
	return nil
}

func runCreateMessage(cmd *cobra.Command, args []string) error {
	configPath := GetConfigPath()

	// Load configuration
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return err
	}

	serverName := args[0]

	// Get server configuration
	serverConfig, exists := cfg.GetServer(serverName)
	if !exists {
		return fmt.Errorf("server '%s' not found in configuration", serverName)
	}

	if !serverConfig.IsEnabled() {
		return fmt.Errorf("server '%s' is disabled", serverName)
	}

	// Create session-aware client factory
	factory, err := getSessionAwareClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create client factory: %w", err)
	}

	// Create session-aware client
	mcpClient, err := factory.CreateClient(serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer func() { _ = mcpClient.Close() }()

	// Prepare request
	request := &mcp.CreateMessageRequest{}

	if len(args) >= 2 {
		// Parse messages JSON
		if err := json.Unmarshal([]byte(args[1]), &request.Messages); err != nil {
			return fmt.Errorf("invalid messages JSON: %w", err)
		}
	} else {
		// Default message
		request.Messages = []mcp.Message{
			{
				Role:    "user",
				Content: "Hello, please respond with a greeting.",
			},
		}
	}

	// Set some reasonable defaults
	request.MaxTokens = 1000
	request.SystemPrompt = "You are a helpful AI assistant."

	// Create message
	ctx := context.Background()
	result, err := mcpClient.CreateMessage(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// Pretty print result
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println("Message created:")
	fmt.Println(string(resultBytes))
	return nil
}

// Session command implementations

// isVerbose returns true if verbose flag is set and updates global VerboseMode
func isVerbose() bool {
	VerboseMode = viper.GetBool("verbose")
	// Set environment variable for session managers to check
	if VerboseMode {
		_ = os.Setenv("MCP_VERBOSE", "true")
	} else {
		_ = os.Setenv("MCP_VERBOSE", "false")
	}
	return VerboseMode
}

func getSessionManager() (*session.Manager, error) {
	sessionManagerOnce.Do(func() {
		configDir, err := config.GetConfigDir()
		if err != nil {
			sessionManagerInitErr = fmt.Errorf("failed to get config directory: %w", err)
			return
		}

		manager, err := client.NewSessionManager(configDir)
		if err != nil {
			sessionManagerInitErr = fmt.Errorf("failed to create session manager: %w", err)
			return
		}

		// sync.Once.Do provides memory barrier, no mutex needed
		globalSessionManager = manager
	})

	if sessionManagerInitErr != nil {
		return nil, sessionManagerInitErr
	}

	if globalSessionManager == nil {
		return nil, fmt.Errorf("failed to initialize session manager")
	}

	return globalSessionManager, nil
}

func getSessionAwareClientFactory() (*client.SessionAwareClientFactory, error) {
	manager, err := getSessionManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}
	return client.NewSessionAwareClientFactory(manager), nil
}

// runSessionList lists all active sessions
func runSessionList(cmd *cobra.Command, args []string) error {
	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active sessions found.")
		return nil
	}

	fmt.Printf("Active sessions (%d):\n", len(sessions))
	for _, sessionInfo := range sessions {
		uptime := "N/A"
		if !sessionInfo.StartTime.IsZero() {
			uptime = time.Since(sessionInfo.StartTime).Round(time.Second).String()
		}

		idleTime := "N/A"
		if !sessionInfo.LastActivity.IsZero() {
			idleTime = time.Since(sessionInfo.LastActivity).Round(time.Second).String()
		}

		status := sessionInfo.Status.String()
		if sessionInfo.Status == session.Error && sessionInfo.Error != "" {
			status += fmt.Sprintf(" (%s)", sessionInfo.Error)
		}

		fmt.Printf("  • %s [%s] - %s\n", sessionInfo.Name, sessionInfo.Type.String(), status)
		fmt.Printf("    Uptime: %s, Idle: %s\n", uptime, idleTime)
		if sessionInfo.PID > 0 {
			fmt.Printf("    PID: %d\n", sessionInfo.PID)
		}
		if len(sessionInfo.Endpoints) > 0 {
			fmt.Printf("    Endpoints: %v\n", sessionInfo.Endpoints)
		}
		fmt.Println()
	}

	return nil
}

// runSessionStatus shows detailed status of a specific session
func runSessionStatus(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	// Load configuration to check if server exists
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	serverConfig, exists := cfg.MCPServers[serverName]
	if !exists {
		displayServerNotFoundError(serverName, cfg)
		return nil
	}

	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	sess, err := manager.GetSessionByName(serverName)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	fmt.Printf("Session Status: %s\n", sess.Name())
	fmt.Printf("Type: %s\n", sess.Type().String())
	fmt.Printf("Status: %s\n", sess.Status().String())

	if sess.Status() == session.Active {
		fmt.Printf("Last Activity: %s ago\n", time.Since(sess.LastActivity()).Round(time.Second))

		// Perform health check
		fmt.Print("Health Check: ")
		if err := sess.HealthCheck(); err != nil {
			fmt.Printf("FAILED - %v\n", err)
		} else {
			fmt.Println("OK")
		}
	}

	// Show server configuration
	fmt.Printf("\nServer Configuration:\n")
	fmt.Printf("Command: %s %v\n", serverConfig.Command, serverConfig.Args)
	if serverConfig.URL != "" {
		fmt.Printf("URL: %s\n", serverConfig.URL)
	}
	fmt.Printf("Session Type: %s\n", session.DetectSessionType(serverConfig).String())
	fmt.Printf("Auto Start: %v\n", session.ShouldAutoStart(serverConfig))
	fmt.Printf("Timeout: %d seconds\n", session.GetSessionTimeout(serverConfig))

	return nil
}

// runSessionStart starts a persistent session
func runSessionStart(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	// Load configuration
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	serverConfig, exists := cfg.MCPServers[serverName]
	if !exists {
		displayServerNotFoundError(serverName, cfg)
		return nil
	}

	// Check if server supports persistent sessions
	sessionType := session.DetectSessionType(serverConfig)
	if sessionType == session.Stateless {
		fmt.Printf("Server %s uses stateless sessions and doesn't need manual starting.\n", serverName)
		return nil
	}

	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	sess, err := manager.GetSession(serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	fmt.Printf("Starting session for %s...\n", serverName)
	if err := sess.Start(); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	fmt.Printf("Session %s started successfully.\n", serverName)
	fmt.Printf("Status: %s\n", sess.Status().String())

	return nil
}

// runSessionStop stops a specific session
func runSessionStop(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	fmt.Printf("Stopping session for %s...\n", serverName)
	if err := manager.StopSession(serverName); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	fmt.Printf("Session %s stopped successfully.\n", serverName)
	return nil
}

// runSessionRestart restarts a specific session
func runSessionRestart(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	fmt.Printf("Restarting session for %s...\n", serverName)
	if err := manager.RestartSession(serverName); err != nil {
		return fmt.Errorf("failed to restart session: %w", err)
	}

	fmt.Printf("Session %s restarted successfully.\n", serverName)
	return nil
}

// runSessionCleanup cleans up dead or expired sessions
func runSessionCleanup(cmd *cobra.Command, args []string) error {
	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	fmt.Println("Cleaning up dead and expired sessions...")
	if err := manager.CleanupSessions(); err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}

	fmt.Println("Session cleanup completed.")
	return nil
}

// runSessionAttach attaches to an existing session
func runSessionAttach(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	// Load configuration to check if server exists
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	serverConfig, exists := cfg.MCPServers[serverName]
	if !exists {
		displayServerNotFoundError(serverName, cfg)
		return nil
	}

	manager, err := getSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	// Try to find and attach to an existing session
	fileStore := manager.GetFileStore()
	sessionInfo, err := fileStore.FindExistingSession(serverName)
	if err != nil {
		return fmt.Errorf("no existing session found for %s: %w", serverName, err)
	}

	fmt.Printf("Found existing session for %s:\n", serverName)
	fmt.Printf("  Session ID: %s\n", sessionInfo.SessionID)
	fmt.Printf("  Status: %s\n", sessionInfo.Status.String())
	fmt.Printf("  PID: %d\n", sessionInfo.PID)
	if !sessionInfo.StartTime.IsZero() {
		fmt.Printf("  Started: %s\n", sessionInfo.StartTime.Format("2006-01-02 15:04:05"))
	}
	if !sessionInfo.LastActivity.IsZero() {
		fmt.Printf("  Last Activity: %s\n", sessionInfo.LastActivity.Format("2006-01-02 15:04:05"))
	}

	// Attempt to create and start the session (which will try reattachment)
	sess, err := manager.GetSession(serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to attach to session: %w", err)
	}

	// Verify session is actually active
	if sess.Status() != session.Active {
		return fmt.Errorf("session attachment failed - session status: %s", sess.Status().String())
	}

	fmt.Printf("\nSuccessfully attached to session %s!\n", serverName)

	// If it's a persistent session, provide additional info
	if sess.Type() == session.Persistent {
		if persistentSession, ok := sess.(*session.PersistentSession); ok {
			info := persistentSession.GetInfo()
			if info.ConnectionInfo != nil {
				fmt.Printf("Connection Type: %s\n", info.ConnectionInfo.Type)
				if info.ConnectionInfo.URL != "" {
					fmt.Printf("Endpoint: %s\n", info.ConnectionInfo.URL)
				}
				if len(info.Endpoints) > 0 {
					fmt.Printf("Endpoints: %v\n", info.Endpoints)
				}
			}
		}
	}

	fmt.Println("\nSession is ready for use. The session will remain active and can be reused across commands.")
	return nil
}

// Daemon command implementations

// runDaemonStart starts the MCP daemon
func runDaemonStart(cmd *cobra.Command, args []string) error {
	manager := daemon.NewDaemonManager()

	if daemonForeground {
		fmt.Println("Starting MCP daemon in foreground...")
		return manager.Start(true)
	}

	fmt.Println("Starting MCP daemon in background...")
	if err := manager.Start(false); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Println("MCP daemon started successfully!")
	fmt.Println("Use 'mcp-cli-ent daemon status' to check the daemon status.")
	return nil
}

// runDaemonStop stops the MCP daemon
func runDaemonStop(cmd *cobra.Command, args []string) error {
	manager := daemon.NewDaemonManager()

	fmt.Println("Stopping MCP daemon...")
	if err := manager.Stop(); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	fmt.Println("MCP daemon stopped successfully!")
	return nil
}

// runDaemonStatus shows the MCP daemon status
func runDaemonStatus(cmd *cobra.Command, args []string) error {
	client := daemon.NewDaemonClient()

	status, err := client.GetStatus()
	if err != nil {
		fmt.Printf("Error getting daemon status: %v\n", err)
		return nil
	}

	if !status.Running {
		fmt.Println("MCP daemon is not running")
		fmt.Println("Use 'mcp-cli-ent daemon start' to start the daemon.")
		return nil
	}

	fmt.Printf("MCP daemon is running (PID: %d)\n", status.PID)
	fmt.Printf("Platform: %s\n", status.Platform)
	fmt.Printf("Endpoint: %s\n", status.Endpoint)
	if !status.StartTime.IsZero() {
		fmt.Printf("Uptime: %s\n", time.Since(status.StartTime).Round(time.Second))
	}

	if status.Version != "" {
		fmt.Printf("Version: %s\n", status.Version)
	}

	fmt.Printf("Active sessions: %d\n", status.SessionCount)

	if len(status.ActiveSessions) > 0 {
		fmt.Println("\nActive sessions:")
		for _, session := range status.ActiveSessions {
			fmt.Printf("  • %s (%s)", session.ServerName, session.Status)
			if session.PID > 0 {
				fmt.Printf(" [PID: %d]", session.PID)
			}
			if !session.StartTime.IsZero() {
				fmt.Printf(" [Uptime: %s]", session.Duration.Round(time.Second))
			}
			fmt.Println()
			if session.Error != "" {
				fmt.Printf("    Error: %s\n", session.Error)
			}
		}
	}

	return nil
}

// runDaemonRestart restarts the MCP daemon
func runDaemonRestart(cmd *cobra.Command, args []string) error {
	manager := daemon.NewDaemonManager()

	fmt.Println("Restarting MCP daemon...")
	if err := manager.Restart(); err != nil {
		return fmt.Errorf("failed to restart daemon: %w", err)
	}

	fmt.Println("MCP daemon restarted successfully!")
	fmt.Println("Use 'mcp-cli-ent daemon status' to check the daemon status.")
	return nil
}

// runDaemonLogs shows the MCP daemon logs
func runDaemonLogs(cmd *cobra.Command, args []string) error {
	logFile := daemon.GetLogFilePath()

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("Daemon log file not found: %s\n", logFile)
		fmt.Println("The daemon may not be running or may not have started yet.")
		return nil
	}

	// Open and read log file
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read all content first to determine tail
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Apply tail filter
	startIndex := 0
	if daemonLogsTail > 0 && len(lines) > daemonLogsTail {
		startIndex = len(lines) - daemonLogsTail
	}

	fmt.Printf("Daemon logs (%s):\n", logFile)
	fmt.Println(strings.Repeat("=", 50))

	for i := startIndex; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			fmt.Println(lines[i])
		}
	}

	return nil
}
