package errors

import (
	"encoding/json"
	"net/http"
)

// ErrorType represents the category of error
type ErrorType string

const (
	// ErrorTypeAuthentication indicates authentication-related errors
	ErrorTypeAuthentication ErrorType = "authentication_error"
	
	// ErrorTypeAuthorization indicates permission/authorization errors
	ErrorTypeAuthorization ErrorType = "authorization_error"
	
	// ErrorTypeRateLimit indicates rate limit exceeded
	ErrorTypeRateLimit ErrorType = "rate_limit_exceeded"
	
	// ErrorTypeNotFound indicates resource not found
	ErrorTypeNotFound ErrorType = "not_found"
	
	// ErrorTypeValidation indicates validation errors
	ErrorTypeValidation ErrorType = "validation_error"
	
	// ErrorTypeGitHubAPI indicates GitHub API errors
	ErrorTypeGitHubAPI ErrorType = "github_api_error"
	
	// ErrorTypeInternal indicates internal server errors
	ErrorTypeInternal ErrorType = "internal_error"
	
	// ErrorTypeMCPProtocol indicates MCP protocol errors
	ErrorTypeMCPProtocol ErrorType = "mcp_protocol_error"
	
	// ErrorTypeMethodNotAllowed indicates HTTP method not allowed
	ErrorTypeMethodNotAllowed ErrorType = "method_not_allowed"
	
	// ErrorTypeMissingAuthorization indicates missing authorization header
	ErrorTypeMissingAuthorization ErrorType = "missing_authorization"
	
	// ErrorTypeInvalidToken indicates invalid token
	ErrorTypeInvalidToken ErrorType = "invalid_token"
)

// JSONRPCErrorCode represents standard JSON-RPC 2.0 error codes
type JSONRPCErrorCode int

const (
	// JSONRPCParseError indicates invalid JSON was received
	JSONRPCParseError JSONRPCErrorCode = -32700
	
	// JSONRPCInvalidRequest indicates the JSON sent is not a valid Request object
	JSONRPCInvalidRequest JSONRPCErrorCode = -32600
	
	// JSONRPCMethodNotFound indicates the method does not exist
	JSONRPCMethodNotFound JSONRPCErrorCode = -32601
	
	// JSONRPCInvalidParams indicates invalid method parameters
	JSONRPCInvalidParams JSONRPCErrorCode = -32602
	
	// JSONRPCInternalError indicates internal JSON-RPC error
	JSONRPCInternalError JSONRPCErrorCode = -32603
	
	// JSONRPCServerError indicates server error (custom range -32000 to -32099)
	JSONRPCServerError JSONRPCErrorCode = -32000
)

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	// Type categorizes the error
	Type ErrorType `json:"type"`
	
	// Message provides a human-readable error message
	Message string `json:"message"`
	
	// Data contains additional error-specific information
	Data map[string]interface{} `json:"data,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error response
type JSONRPCError struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      interface{}      `json:"id"`
	Error   JSONRPCErrorData `json:"error"`
}

// JSONRPCErrorData represents the error object in JSON-RPC 2.0
type JSONRPCErrorData struct {
	Code    JSONRPCErrorCode `json:"code"`
	Message string           `json:"message"`
	Data    interface{}      `json:"data,omitempty"`
}

// HTTPErrorResponse represents an HTTP error response wrapper
type HTTPErrorResponse struct {
	Error ErrorResponse `json:"error"`
}

// NewErrorResponse creates a new ErrorResponse
func NewErrorResponse(errType ErrorType, message string, data map[string]interface{}) *ErrorResponse {
	return &ErrorResponse{
		Type:    errType,
		Message: message,
		Data:    data,
	}
}

// NewJSONRPCError creates a new JSON-RPC error response
func NewJSONRPCError(id interface{}, code JSONRPCErrorCode, message string, data interface{}) *JSONRPCError {
	return &JSONRPCError{
		JSONRPC: "2.0",
		ID:      id,
		Error: JSONRPCErrorData{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// NewHTTPErrorResponse creates a new HTTP error response
func NewHTTPErrorResponse(errType ErrorType, message string, data map[string]interface{}) *HTTPErrorResponse {
	return &HTTPErrorResponse{
		Error: ErrorResponse{
			Type:    errType,
			Message: message,
			Data:    data,
		},
	}
}

// WriteHTTPError writes an error response to an HTTP response writer
func WriteHTTPError(w http.ResponseWriter, statusCode int, errType ErrorType, message string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := NewHTTPErrorResponse(errType, message, data)
	json.NewEncoder(w).Encode(response)
}

// MapErrorTypeToHTTPStatus maps an ErrorType to an appropriate HTTP status code
func MapErrorTypeToHTTPStatus(errType ErrorType) int {
	switch errType {
	case ErrorTypeAuthentication, ErrorTypeMissingAuthorization, ErrorTypeInvalidToken:
		return http.StatusUnauthorized
	case ErrorTypeAuthorization:
		return http.StatusForbidden
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeValidation, ErrorTypeMCPProtocol:
		return http.StatusBadRequest
	case ErrorTypeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case ErrorTypeGitHubAPI, ErrorTypeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// MapErrorTypeToJSONRPCCode maps an ErrorType to a JSON-RPC error code
func MapErrorTypeToJSONRPCCode(errType ErrorType) JSONRPCErrorCode {
	switch errType {
	case ErrorTypeMCPProtocol:
		return JSONRPCInvalidRequest
	case ErrorTypeValidation:
		return JSONRPCInvalidParams
	case ErrorTypeNotFound:
		return JSONRPCMethodNotFound
	default:
		return JSONRPCServerError
	}
}
