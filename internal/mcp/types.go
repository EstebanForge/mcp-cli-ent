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
	Arguments map[string]interface{} `json:"arguments"`
}

// ListResourcesParams represents parameters for resources/list
type ListResourcesParams struct{}

// ListResourcesResult represents the result of resources/list
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// Sampling related types

// CreateMessageRequest represents sampling/createMessage
type CreateMessageRequest struct {
	Messages        []Message              `json:"messages"`
	ModelPreferences *ModelPreferences      `json:"modelPreferences,omitempty"`
	SystemPrompt    string                 `json:"systemPrompt,omitempty"`
	MaxTokens       int                    `json:"maxTokens,omitempty"`
	StopSequences   []string               `json:"stopSequences,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a message in sampling request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ModelPreferences represents model selection hints and priorities
type ModelPreferences struct {
	Hints               []ModelHint `json:"hints,omitempty"`
	CostPriority        float64     `json:"costPriority,omitempty"`
	SpeedPriority       float64     `json:"speedPriority,omitempty"`
	IntelligencePriority float64     `json:"intelligencePriority,omitempty"`
}

// ModelHint represents a suggested model
type ModelHint struct {
	Name string `json:"name"`
}

// CreateMessageResult represents the result of sampling/createMessage
type CreateMessageResult struct {
	Role         string                 `json:"role"`
	Content      Content                `json:"content"`
	Model        string                 `json:"model,omitempty"`
	StopReason   string                 `json:"stopReason,omitempty"`
	TokenUsage   *TokenUsage            `json:"tokenUsage,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Content represents message content
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	TotalTokens      int `json:"totalTokens,omitempty"`
}

// Elicitation related types

// RequestInputParams represents elicitation/requestInput
type RequestInputParams struct {
	Message string                 `json:"message"`
	Schema  map[string]interface{} `json:"schema"`
}

// RequestInputResult represents the result of elicitation/requestInput
type RequestInputResult struct {
	Data map[string]interface{} `json:"data"`
}

// Roots related types

// Root represents a filesystem root
type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// ListChangedResult represents roots/list_changed notification
type ListChangedResult struct {
	Roots []Root `json:"roots"`
}

// Server capability types

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
	Tools        *ToolsCapability        `json:"tools,omitempty"`
	Resources    *ResourcesCapability    `json:"resources,omitempty"`
	Sampling     *SamplingCapability     `json:"sampling,omitempty"`
	Roots        *RootsCapability        `json:"roots,omitempty"`
	Elicitation  *ElicitationCapability  `json:"elicit,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	Subscribe bool   `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents sampling capability
type SamplingCapability struct{}

// RootsCapability represents roots capability
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ElicitationCapability represents elicitation capability
type ElicitationCapability struct{}

// InitializeParams represents initialize request parameters
type InitializeParams struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
	Roots        *RootsCapability       `json:"roots,omitempty"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents initialize response
type InitializeResult struct {
	ProtocolVersion string              `json:"protocolVersion"`
	Capabilities    ServerCapabilities  `json:"capabilities"`
	ServerInfo      ServerInfo          `json:"serverInfo"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
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
