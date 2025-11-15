package mcp

import (
	"context"
	"fmt"
)

// MCPClient defines the interface for MCP clients
type MCPClient interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)
	ListResources(ctx context.Context) ([]Resource, error)
	Close() error
}

// ClientConfig holds configuration for MCP clients
type ClientConfig struct {
	Timeout int               `json:"timeout"`
	Headers map[string]string `json:"headers,omitempty"`
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout: 30,
		Headers: make(map[string]string),
	}
}

// ValidateArguments validates tool arguments against the input schema
func (t *Tool) ValidateArguments(args map[string]interface{}) error {
	if t.InputSchema == nil {
		return nil
	}

	schema := t.InputSchema
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Check for required properties
	if required, ok := schema["required"].([]interface{}); ok {
		for _, req := range required {
			if reqName, ok := req.(string); ok {
				if _, exists := args[reqName]; !exists {
					return fmt.Errorf("missing required argument: %s", reqName)
				}
			}
		}
	}

	// Check for unknown properties (optional validation)
	if _, ok := schema["additionalProperties"].(bool); ok && !schema["additionalProperties"].(bool) {
		for argName := range args {
			if _, exists := properties[argName]; !exists {
				return fmt.Errorf("unknown argument: %s", argName)
			}
		}
	}

	return nil
}
