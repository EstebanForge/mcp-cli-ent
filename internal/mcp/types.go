package mcp

import "encoding/json"

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// ToolResult represents the result of calling a tool
type ToolResult struct {
	Content []interface{} `json:"content,omitempty"`
	IsError bool          `json:"isError,omitempty"`
}

// Resource represents an MCP resource definition
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListToolsParams represents parameters for tools/list
type ListToolsParams struct{}

// ListToolsResult represents the result of tools/list
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents parameters for tools/call
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ListResourcesParams represents parameters for resources/list
type ListResourcesParams struct{}

// ListResourcesResult represents the result of resources/list
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// Error codes as defined in JSON-RPC 2.0 specification
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// NewError creates a new JSON-RPC error
func NewError(code int, message string, data interface{}) *JSONRPCError {
	return &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewRequest creates a new JSON-RPC request
func NewRequest(id interface{}, method string, params interface{}) *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewResponse creates a new JSON-RPC response
func NewResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates a new JSON-RPC error response
func NewErrorResponse(id interface{}, err *JSONRPCError) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}
}

// MarshalRequest marshals a JSON-RPC request to bytes
func MarshalRequest(req *JSONRPCRequest) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalResponse unmarshals JSON-RPC response from bytes
func UnmarshalResponse(data []byte) (*JSONRPCResponse, error) {
	var resp JSONRPCResponse
	err := json.Unmarshal(data, &resp)
	return &resp, err
}
