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
			return NewHTTPProcessClient(serverConfig.Command, serverConfig.Args, serverConfig.Env, serverConfig.URL, clientConfig)
		}
		return NewHTTPClient(serverConfig.URL, clientConfig), nil
	} else if serverConfig.Command != "" {
		if missing := unresolvedEnvVars(serverConfig.Env); len(missing) > 0 {
			return nil, &ClientError{fmt.Sprintf("missing required environment variables: %s", strings.Join(missing, ", "))}
		}

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
