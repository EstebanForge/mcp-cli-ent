package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

// HTTPProcessClient starts a local HTTP MCP server and talks to it over HTTP.
type HTTPProcessClient struct {
	*HTTPClient
	cmd *exec.Cmd
}

// NewHTTPProcessClient creates a new HTTP MCP client backed by a local process.
func NewHTTPProcessClient(command string, args []string, env map[string]string, url string, config *mcp.ClientConfig) (*HTTPProcessClient, error) {
	cmd := exec.CommandContext(context.Background(), command, args...)

	if len(env) > 0 {
		cmdEnv := os.Environ()
		for k, v := range env {
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = cmdEnv
	}

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return &HTTPProcessClient{
		HTTPClient: NewHTTPClient(url, config),
		cmd:        cmd,
	}, nil
}

// Close terminates the local HTTP MCP server process.
func (c *HTTPProcessClient) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	return nil
}
