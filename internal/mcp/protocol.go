package mcp

import (
	"context"
	"fmt"
)

// MCPClient defines the interface for MCP clients
type MCPClient interface {
	// Core protocol
	Initialize(ctx context.Context, params *InitializeParams) (*InitializeResult, error)
	Close() error

	// Tools
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)

	// Resources
	ListResources(ctx context.Context) ([]Resource, error)

	// Sampling - enables agentic workflows
	CreateMessage(ctx context.Context, request *CreateMessageRequest) (*CreateMessageResult, error)

	// Elicitation - enables dynamic information gathering
	RequestInput(ctx context.Context, params *RequestInputParams) (*RequestInputResult, error)

	// Roots - filesystem boundary management
	ListRoots(ctx context.Context) ([]Root, error)

	// Notifications (one-way)
	NotifyRootsListChanged(roots []Root) error
}

// SamplingHandler defines how clients should handle sampling requests
type SamplingHandler interface {
	HandleSamplingRequest(ctx context.Context, request *CreateMessageRequest) (*CreateMessageResult, error)
}

// ElicitationHandler defines how clients should handle elicitation requests
type ElicitationHandler interface {
	HandleElicitationRequest(ctx context.Context, params *RequestInputParams) (*RequestInputResult, error)
}

// RootsHandler defines how clients should handle roots changes
type RootsHandler interface {
	HandleRootsChange(roots []Root) error
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
