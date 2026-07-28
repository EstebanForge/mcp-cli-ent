package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

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
	Messages         []Message              `json:"messages"`
	ModelPreferences *ModelPreferences      `json:"modelPreferences,omitempty"`
	SystemPrompt     string                 `json:"systemPrompt,omitempty"`
	MaxTokens        int                    `json:"maxTokens,omitempty"`
	StopSequences    []string               `json:"stopSequences,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a message in sampling request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ModelPreferences represents model selection hints and priorities
type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         float64     `json:"costPriority,omitempty"`
	SpeedPriority        float64     `json:"speedPriority,omitempty"`
	IntelligencePriority float64     `json:"intelligencePriority,omitempty"`
}

// ModelHint represents a suggested model
type ModelHint struct {
	Name string `json:"name"`
}

// CreateMessageResult represents the result of sampling/createMessage
type CreateMessageResult struct {
	Role       string                 `json:"role"`
	Content    Content                `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`
	TokenUsage *TokenUsage            `json:"tokenUsage,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
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
	Tools       *ToolsCapability       `json:"tools,omitempty"`
	Resources   *ResourcesCapability   `json:"resources,omitempty"`
	Sampling    *SamplingCapability    `json:"sampling,omitempty"`
	Roots       *RootsCapability       `json:"roots,omitempty"`
	Elicitation *ElicitationCapability `json:"elicit,omitempty"`
	Extensions  map[string]interface{} `json:"extensions,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
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
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
	Roots        *RootsCapability       `json:"roots,omitempty"`
	Extensions   map[string]interface{} `json:"extensions,omitempty"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents initialize response
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// DiscoverResult is the result of server/discover (MCP 2026-07-28). The server
// advertises its supported protocol versions, capabilities, and identity.
type DiscoverResult struct {
	ResultType        string                 `json:"resultType"`
	SupportedVersions []string               `json:"supportedVersions,omitempty"`
	Capabilities      ServerCapabilities     `json:"capabilities,omitempty"`
	Instructions      string                 `json:"instructions,omitempty"`
	Meta              map[string]interface{} `json:"_meta,omitempty"`
	TTLms             int                    `json:"ttlMs,omitempty"`
	CacheScope        string                 `json:"cacheScope,omitempty"`
}

// GetServerInfo extracts the server's self-reported identity from _meta, if present.
func (d *DiscoverResult) GetServerInfo() (name, ver string, ok bool) {
	if d.Meta == nil {
		return "", "", false
	}
	si, found := d.Meta[MetaKeyServerInfo].(map[string]interface{})
	if !found {
		return "", "", false
	}
	n, _ := si["name"].(string)
	v, _ := si["version"].(string)
	return n, v, true
}

// Error codes as defined in JSON-RPC 2.0 specification
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP 2026-07-28 protocol constants. The protocol is stateless: every request
// carries its version and capabilities in _meta, negotiated per request rather
// than via an initialize handshake.
const (
	// ProtocolVersion is the version this client speaks on the modern path.
	ProtocolVersion = "2026-07-28"

	// ClientName identifies this client in _meta.clientInfo.
	ClientName = "mcp-cli-ent"
)

// ClientVersion is sent in _meta.clientInfo.version; sourced from pkg/version.
var ClientVersion = version.Version

// _meta keys, all namespaced under io.modelcontextprotocol/ per the spec.
const (
	MetaKeyProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	MetaKeyClientInfo         = "io.modelcontextprotocol/clientInfo"
	MetaKeyClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	MetaKeyServerInfo         = "io.modelcontextprotocol/serverInfo"
	MetaKeyLogLevel           = "io.modelcontextprotocol/logLevel"
	MetaKeySubscriptionID     = "io.modelcontextprotocol/subscriptionId"
)

// HTTP headers that mirror JSON-RPC body fields for routing by intermediaries.
const (
	HeaderProtocolVersion = "MCP-Protocol-Version"
	HeaderMethod          = "Mcp-Method"
	HeaderName            = "Mcp-Name"
)

// Modern (2026-07-28) resultType discriminators.
const (
	ResultTypeComplete      = "complete"
	ResultTypeInputRequired = "input_required"
)

// MCP-defined JSON-RPC error codes, allocated in the -32020..-32099 range
// reserved for the specification by revision 2026-07-28.
const (
	HeaderMismatchError             = -32020
	MissingRequiredClientCapability = -32021
	UnsupportedProtocolVersion      = -32022
)

// KnownVersions lists recognized MCP protocol versions, oldest first.
var KnownVersions = []string{
	"2024-11-05",
	"2025-03-26",
	"2025-06-18",
	"2025-11-25",
	"2026-07-28",
}

// IsKnownVersion reports whether v is a recognized MCP protocol version date.
func IsKnownVersion(v string) bool {
	for _, k := range KnownVersions {
		if k == v {
			return true
		}
	}
	return false
}

// Era classifies a server's protocol generation, driving transport behavior.
type Era int

const (
	// EraUnknown means the era is not pinned and must be detected at runtime.
	EraUnknown Era = iota
	// EraModern: 2026-07-28 and later. Stateless, per-request _meta, no session.
	EraModern
	// EraLegacy: 2025-11-25 and earlier. initialize handshake, Mcp-Session-Id.
	EraLegacy
)

// ClassifyEra maps a protocol version date to an Era. Unknown versions return
// EraUnknown, which the transports treat as "auto-detect".
func ClassifyEra(version string) Era {
	switch version {
	case "2026-07-28":
		return EraModern
	case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
		return EraLegacy
	default:
		return EraUnknown
	}
}

// String returns a human-readable era label for logs and diagnostics.
func (e Era) String() string {
	switch e {
	case EraModern:
		return "modern"
	case EraLegacy:
		return "legacy"
	default:
		return "auto"
	}
}

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

// NewRequestMeta builds the per-request _meta block required by MCP 2026-07-28.
// Every modern request carries protocolVersion and clientCapabilities;
// clientInfo is SHOULD. Legacy (initialize-handshake) requests do not use this.
func NewRequestMeta(capabilities ClientCapabilities) map[string]interface{} {
	return map[string]interface{}{
		MetaKeyProtocolVersion:    ProtocolVersion,
		MetaKeyClientInfo:         ClientInfo{Name: ClientName, Version: ClientVersion},
		MetaKeyClientCapabilities: capabilities,
	}
}

// InjectMeta attaches the per-request _meta block to req, converting its Params
// to a map if necessary. Intended for the modern transport path only.
func InjectMeta(req *JSONRPCRequest, capabilities ClientCapabilities) error {
	if req == nil {
		return errors.New("cannot inject _meta into a nil request")
	}
	var pm map[string]interface{}
	if req.Params != nil {
		b, err := json.Marshal(req.Params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		// UseNumber preserves large integers and exact number representation,
		// avoiding float64 corruption of tool arguments on the modern path.
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(&pm); err != nil {
			return fmt.Errorf("decode params to map: %w", err)
		}
		if pm == nil {
			pm = map[string]interface{}{}
		}
	} else {
		pm = map[string]interface{}{}
	}
	pm["_meta"] = NewRequestMeta(capabilities)
	req.Params = pm
	return nil
}

// NameForRequest returns the Mcp-Name header value for a request: params.name
// for tools/call and prompts/get, params.uri for resources/read. Returns "" for
// methods without a name. Params must already be a map (call InjectMeta first).
func NameForRequest(req *JSONRPCRequest) string {
	if req == nil {
		return ""
	}
	pm, ok := req.Params.(map[string]interface{})
	if !ok {
		return ""
	}
	switch req.Method {
	case "tools/call", "prompts/get":
		if n, ok := pm["name"].(string); ok {
			return n
		}
	case "resources/read":
		if u, ok := pm["uri"].(string); ok {
			return u
		}
	}
	return ""
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
