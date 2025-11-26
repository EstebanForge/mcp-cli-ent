package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Embedded example configuration - keep in sync with root mcp_servers.example.json
//
//go:embed mcp_servers.example.json
var exampleConfigJSON []byte

// GetConfigDir returns the standard configuration directory for the current platform
func GetConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		// Windows: %USERPROFILE%\AppData\Roaming\mcp-cli-ent
		appData := os.Getenv("APPDATA")
		if appData == "" {
			// Fallback to user profile if APPDATA is not set
			userProfile := os.Getenv("USERPROFILE")
			if userProfile == "" {
				return "", fmt.Errorf("could not determine user profile directory")
			}
			appData = filepath.Join(userProfile, "AppData", "Roaming")
		}
		return filepath.Join(appData, "mcp-cli-ent"), nil
	} else {
		// Linux, macOS, WSL: ~/.config/mcp-cli-ent
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("could not determine home directory")
		}
		return filepath.Join(home, ".config", "mcp-cli-ent"), nil
	}
}

// EnsureConfigDirectory creates the config directory and initial files if they don't exist
func EnsureConfigDirectory() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to determine config directory: %w", err)
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if mcp_servers.json already exists
	mcpServersPath := filepath.Join(configDir, "mcp_servers.json")
	if _, err := os.Stat(mcpServersPath); os.IsNotExist(err) {
		// Create default mcp_servers.json
		exampleConfig := Configuration{
			MCPServers: map[string]ServerConfig{
				"context7": {
					Type: "http",
					URL:  "https://mcp.context7.com/mcp",
					Headers: map[string]string{
						"CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}",
					},
					Timeout: 30,
				},
				"sequential-thinking": {
					Command: "npx",
					Args:    []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
					Timeout: 30,
				},
				"deepwiki": {
					Command: "npx",
					Args:    []string{"-y", "mcp-remote", "https://mcp.deepwiki.com/sse"},
					Timeout: 30,
				},
			},
		}

		data, err := json.MarshalIndent(exampleConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal example config: %w", err)
		}

		// Add newline at the end
		data = append(data, '\n')

		if err := os.WriteFile(mcpServersPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write mcp_servers.json: %w", err)
		}
	}

	// Check if config.json already exists
	configPath := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create an empty config.json for future tool configuration
		emptyConfig := map[string]interface{}{}

		data, err := json.MarshalIndent(emptyConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal empty config: %w", err)
		}

		// Add newline at the end
		data = append(data, '\n')

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write config.json: %w", err)
		}
	}

	return nil
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(configPath string) (*Configuration, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, &ConfigError{fmt.Sprintf("configuration file '%s' not found", configPath)}
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse JSON
	var config Configuration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Validate configuration
	if err := ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Resolve environment variables in headers, env, and args
	for name, server := range config.MCPServers {
		server.ResolveHeaders()
		server.ResolveEnv()
		server.ResolveArgs()
		config.MCPServers[name] = server
	}

	return &config, nil
}

// FindConfigFile searches for the configuration file in standard locations
func FindConfigFile() (string, error) {
	// First, check standard config directory
	configDir, err := GetConfigDir()
	if err == nil {
		standardConfig := filepath.Join(configDir, "mcp_servers.json")
		if _, err := os.Stat(standardConfig); err == nil {
			return standardConfig, nil
		}
	}

	// Fall back to current directory for backward compatibility
	possiblePaths := []string{
		"mcp_servers.json",  // Current directory
		".mcp_servers.json", // Hidden file in current directory
	}

	for _, path := range possiblePaths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", &ConfigError{"no configuration file found in standard locations"}
}

// ValidateConfig validates the entire configuration
func ValidateConfig(config *Configuration) error {
	if config.MCPServers == nil {
		return &ConfigError{"no MCP servers configured"}
	}

	for name, server := range config.MCPServers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("server '%s': %w", name, err)
		}
	}

	return nil
}

// GetServerNames returns a list of all configured server names
func (c *Configuration) GetServerNames() []string {
	names := make([]string, 0, len(c.MCPServers))
	for name := range c.MCPServers {
		names = append(names, name)
	}
	return names
}

// GetEnabledServers returns a map of enabled servers
func (c *Configuration) GetEnabledServers() map[string]ServerConfig {
	enabled := make(map[string]ServerConfig)
	for name, config := range c.MCPServers {
		if config.IsEnabled() {
			enabled[name] = config
		}
	}
	return enabled
}

// GetServer returns the configuration for a specific server
func (c *Configuration) GetServer(name string) (ServerConfig, bool) {
	server, exists := c.MCPServers[name]
	return server, exists
}

// GetServerStatus returns the status of all servers
func (c *Configuration) GetServerStatus() []ServerStatus {
	statuses := make([]ServerStatus, 0, len(c.MCPServers))

	for name, server := range c.MCPServers {
		status := ServerStatus{
			Name:    name,
			Type:    server.GetServerType(),
			Details: server.GetServerDetails(),
		}

		if server.IsEnabled() {
			status.Status = "enabled"
		} else {
			status.Status = "disabled"
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// CreateExampleConfig creates an example configuration file
func CreateExampleConfig(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write embedded example config to file
	if err := os.WriteFile(filename, exampleConfigJSON, 0644); err != nil {
		return fmt.Errorf("failed to write example config: %w", err)
	}

	return nil
}

// Config represents the MCP servers configuration (alias for backwards compatibility)
type Config = Configuration

// GetConfigPath returns the configuration file path
func GetConfigPath(configPath string) string {
	if configPath != "" {
		return configPath
	}

	// Try to find config in standard locations
	if path, err := FindConfigFile(); err == nil {
		return path
	}

	// Fall back to default location
	if configDir, err := GetConfigDir(); err == nil {
		return filepath.Join(configDir, "mcp_servers.json")
	}

	return "mcp_servers.json"
}
