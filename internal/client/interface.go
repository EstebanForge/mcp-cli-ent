package client

import (
	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// NewMCPClient creates an appropriate MCP client based on server configuration
func NewMCPClient(serverConfig config.ServerConfig) (mcp.MCPClient, error) {
	if serverConfig.Type == "http" || serverConfig.URL != "" {
		// HTTP client
		clientConfig := &mcp.ClientConfig{
			Timeout: serverConfig.Timeout,
			Headers: serverConfig.Headers,
		}
		return NewHTTPClient(serverConfig.URL, clientConfig), nil
	} else if serverConfig.Command != "" {
		// Stdio client
		return NewStdioClient(serverConfig.Command, serverConfig.Args, serverConfig.Env)
	}

	return nil, &ClientError{"invalid server configuration: neither URL nor command specified"}
}

// ClientError represents an error in client creation or operation
type ClientError struct {
	Message string
}

func (e *ClientError) Error() string {
	return e.Message
}
