package client

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

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
		if serverConfig.Command != "" {
			if missing := unresolvedEnvVars(serverConfig.Env); len(missing) > 0 {
				return nil, &ClientError{fmt.Sprintf("missing required environment variables: %s", strings.Join(missing, ", "))}
			}
			// Inject mcp-remote header if needed for HTTP process clients
			args := injectMcpRemoteHeader(serverConfig.Command, serverConfig.Args)
			return NewHTTPProcessClient(serverConfig.Command, args, serverConfig.Env, serverConfig.URL, clientConfig)
		}
		return NewHTTPClient(serverConfig.URL, clientConfig), nil
	} else if serverConfig.Command != "" {
		if missing := unresolvedEnvVars(serverConfig.Env); len(missing) > 0 {
			return nil, &ClientError{fmt.Sprintf("missing required environment variables: %s", strings.Join(missing, ", "))}
		}

		// Stdio client - inject mcp-remote header if needed
		args := injectMcpRemoteHeader(serverConfig.Command, serverConfig.Args)
		return NewStdioClient(serverConfig.Command, args, serverConfig.Env)
	}

	return nil, &ClientError{"invalid server configuration: neither URL nor command specified"}
}

// injectMcpRemoteHeader automatically adds the required Accept header for mcp-remote HTTP connections
func injectMcpRemoteHeader(command string, args []string) []string {
	// Check if this is an npx mcp-remote command
	if command != "npx" && command != "npm" {
		return args
	}

	// Find mcp-remote in args
	mcpRemoteIdx := -1
	for i, arg := range args {
		if strings.Contains(arg, "mcp-remote") {
			mcpRemoteIdx = i
			break
		}
	}
	if mcpRemoteIdx == -1 {
		return args
	}

	// Check if --header is already present
	for _, arg := range args {
		if strings.HasPrefix(arg, "--header") {
			return args // User already specified custom headers
		}
	}

	// Find the URL (starts with http:// or https://)
	urlIdx := -1
	for i := mcpRemoteIdx; i < len(args); i++ {
		if strings.HasPrefix(args[i], "http://") || strings.HasPrefix(args[i], "https://") {
			urlIdx = i
			break
		}
	}
	if urlIdx == -1 {
		return args
	}

	// Insert --header before the URL
	result := make([]string, 0, len(args)+2)
	result = append(result, args[:urlIdx]...)
	result = append(result, "--header", "Accept:application/json,text/event-stream")
	result = append(result, args[urlIdx:]...)
	return result
}

// ClientError represents an error in client creation or operation
type ClientError struct {
	Message string
}

func (e *ClientError) Error() string {
	return e.Message
}

func unresolvedEnvVars(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	reCurly := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	reSimple := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)

	for _, value := range env {
		for _, match := range reCurly.FindAllStringSubmatch(value, -1) {
			if len(match) != 2 {
				continue
			}
			varName := match[1]
			if os.Getenv(varName) == "" {
				seen[varName] = struct{}{}
			}
		}
		for _, match := range reSimple.FindAllStringSubmatch(value, -1) {
			if len(match) != 2 {
				continue
			}
			varName := match[1]
			if os.Getenv(varName) == "" {
				seen[varName] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	missing := make([]string, 0, len(seen))
	for name := range seen {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}
