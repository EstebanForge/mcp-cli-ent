package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mcp-cli-ent/mcp-cli/internal/client"
	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

var listServersCmd = &cobra.Command{
	Use:   "list-servers",
	Short: "List all configured MCP servers",
	Long: `List all configured MCP servers with their status, type, and configuration details.
This shows both enabled and disabled servers from the configuration file.`,
	RunE: runListServers,
}

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

var listResourcesCmd = &cobra.Command{
	Use:   "list-resources <server-name>",
	Short: "List resources from an MCP server",
	Long: `List available resources from a specific MCP server.
Resources can include files, documentation, and other data sources.`,
	Args: cobra.ExactArgs(1),
	RunE: runListResources,
}

var createConfigCmd = &cobra.Command{
	Use:   "create-config [filename]",
	Short: "Create an example configuration file",
	Long: `Create an example mcp_servers.json configuration file with sample server configurations.
If filename is omitted, creates 'mcp_servers.json' in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreateConfig,
}

func init() {
	rootCmd.AddCommand(listServersCmd)
	rootCmd.AddCommand(listToolsCmd)
	rootCmd.AddCommand(callToolCmd)
	rootCmd.AddCommand(listResourcesCmd)
	rootCmd.AddCommand(createConfigCmd)

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

func runListServers(cmd *cobra.Command, args []string) error {
	configPath := getConfigPath()

	// Load configuration
	cfg, err := loadConfiguration(configPath)
	if err != nil {
		return err
	}

	// Get server statuses
	statuses := cfg.GetServerStatus()

	if len(statuses) == 0 {
		fmt.Println("No MCP servers configured.")
		return nil
	}

	fmt.Printf("Configured MCP servers (%d):\n", len(statuses))
	for _, status := range statuses {
		statusIcon := "✓"
		if status.Status == "disabled" {
			statusIcon = "✗"
		}
		fmt.Printf("  %s %s [%s] - %s\n", statusIcon, status.Name, status.Status, status.Details)
	}

	return nil
}

func runListTools(cmd *cobra.Command, args []string) error {
	configPath := getConfigPath()

	// Load configuration
	cfg, err := loadConfiguration(configPath)
	if err != nil {
		return err
	}

	ctx := context.Background()

	if len(args) == 0 {
		// List tools from all enabled servers
		enabledServers := cfg.GetEnabledServers()
		if len(enabledServers) == 0 {
			fmt.Println("No enabled MCP servers found.")
			return nil
		}

		for serverName, serverConfig := range enabledServers {
			fmt.Printf("\n=== %s ===\n", serverName)
			if err := listToolsFromServer(ctx, serverName, serverConfig); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	} else {
		// List tools from specific server
		serverName := args[0]
		serverConfig, exists := cfg.GetServer(serverName)
		if !exists {
			return fmt.Errorf("server '%s' not found in configuration", serverName)
		}

		if !serverConfig.IsEnabled() {
			return fmt.Errorf("server '%s' is disabled", serverName)
		}

		return listToolsFromServer(ctx, serverName, serverConfig)
	}

	return nil
}

func listToolsFromServer(ctx context.Context, serverName string, serverConfig config.ServerConfig) error {
	// Create client
	mcpClient, err := client.NewMCPClient(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer mcpClient.Close()

	// List tools
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	if len(tools) == 0 {
		fmt.Println("No tools found.")
		return nil
	}

	fmt.Printf("Available tools (%d):\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  • %s\n", tool.Name)
		if tool.Description != "" {
			fmt.Printf("    %s\n", tool.Description)
		}
		if tool.InputSchema != nil {
			if properties, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
				var paramNames []string
				for name := range properties {
					paramNames = append(paramNames, name)
				}
				if len(paramNames) > 0 {
					fmt.Printf("    Parameters: %s\n", strings.Join(paramNames, ", "))
				}
			}
		}
		fmt.Println()
	}

	return nil
}

func runCallTool(cmd *cobra.Command, args []string) error {
	configPath := getConfigPath()

	// Load configuration
	cfg, err := loadConfiguration(configPath)
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
	}

	// Get server configuration
	serverConfig, exists := cfg.GetServer(serverName)
	if !exists {
		return fmt.Errorf("server '%s' not found in configuration", serverName)
	}

	if !serverConfig.IsEnabled() {
		return fmt.Errorf("server '%s' is disabled", serverName)
	}

	// Create client
	mcpClient, err := client.NewMCPClient(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer mcpClient.Close()

	// Call tool
	ctx := context.Background()
	result, err := mcpClient.CallTool(ctx, toolName, arguments)
	if err != nil {
		return fmt.Errorf("failed to call tool: %w", err)
	}

	// Pretty print result
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println("Result:")
	fmt.Println(string(resultBytes))
	return nil
}

func runListResources(cmd *cobra.Command, args []string) error {
	configPath := getConfigPath()

	// Load configuration
	cfg, err := loadConfiguration(configPath)
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

	// Create client
	mcpClient, err := client.NewMCPClient(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer mcpClient.Close()

	// List resources
	ctx := context.Background()
	resources, err := mcpClient.ListResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	if len(resources) == 0 {
		fmt.Println("No resources found.")
		return nil
	}

	fmt.Printf("Available resources (%d):\n", len(resources))
	for _, resource := range resources {
		fmt.Printf("  • %s\n", resource.URI)
		if resource.Name != "" {
			fmt.Printf("    Name: %s\n", resource.Name)
		}
		if resource.Description != "" {
			fmt.Printf("    Description: %s\n", resource.Description)
		}
		if resource.MimeType != "" {
			fmt.Printf("    Type: %s\n", resource.MimeType)
		}
		fmt.Println()
	}

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

func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}

	// Try to find config file in standard locations
	if path, err := config.FindConfigFile(); err == nil {
		return path
	}

	return "mcp_servers.json" // Default fallback
}

func loadConfiguration(configPath string) (*config.Configuration, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration from '%s': %w", configPath, err)
	}
	return cfg, nil
}
