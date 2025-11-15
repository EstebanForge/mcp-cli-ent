package config

import (
	"os"
	"regexp"
	"strings"
)

// Configuration represents the MCP servers configuration
type Configuration struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig represents configuration for a single MCP server
type ServerConfig struct {
	Disabled bool              `json:"disabled,omitempty"`
	Type     string            `json:"type,omitempty"`
	URL      string            `json:"url,omitempty"`
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Timeout  int               `json:"timeout,omitempty"`
}

// ServerStatus represents the status of a server
type ServerStatus struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"` // "enabled" or "disabled"
	Details string `json:"details"`
	Error   string `json:"error,omitempty"`
}

// ResolveEnvironmentVariables substitutes environment variables in string values
// Supports ${VAR_NAME} and $VAR_NAME formats
func ResolveEnvironmentVariables(input string) string {
	// Match ${VAR_NAME} pattern
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		varName := strings.Trim(match, "${}")
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match // Keep original if environment variable not found
	})

	// Match $VAR_NAME pattern (simple variables)
	re2 := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	result = re2.ReplaceAllStringFunc(result, func(match string) string {
		varName := strings.Trim(match, "$")
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match // Keep original if environment variable not found
	})

	return result
}

// ResolveHeaders resolves environment variables in header values
func (c *ServerConfig) ResolveHeaders() {
	if c.Headers == nil {
		c.Headers = make(map[string]string)
		return
	}

	resolved := make(map[string]string)
	for key, value := range c.Headers {
		resolved[key] = ResolveEnvironmentVariables(value)
	}
	c.Headers = resolved
}

// GetServerType returns a human-readable type description
func (c *ServerConfig) GetServerType() string {
	if c.Type == "http" || c.URL != "" {
		return "HTTP"
	}
	if c.Command != "" {
		return "Stdio"
	}
	return "Unknown"
}

// GetServerDetails returns a detailed description of the server configuration
func (c *ServerConfig) GetServerDetails() string {
	if c.Type == "http" || c.URL != "" {
		return c.URL
	}
	if c.Command != "" {
		details := c.Command
		if len(c.Args) > 0 {
			details += " " + strings.Join(c.Args, " ")
		}
		return details
	}
	return "No configuration"
}

// IsEnabled returns whether the server is enabled
func (c *ServerConfig) IsEnabled() bool {
	return !c.Disabled
}

// Validate validates the server configuration
func (c *ServerConfig) Validate() error {
	if c.Type == "http" || c.URL != "" {
		if c.URL == "" {
			return &ConfigError{"HTTP server type requires URL"}
		}
	} else if c.Command != "" {
		// Stdio server
		if c.Command == "" {
			return &ConfigError{"Stdio server type requires command"}
		}
	} else {
		return &ConfigError{"Server must have either URL (for HTTP) or command (for stdio)"}
	}

	return nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}
