package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorResponse(t *testing.T) {
	t.Run("creates error response with all fields", func(t *testing.T) {
		data := map[string]interface{}{
			"retry_after": 3600,
			"limit":       5000,
		}
		
		resp := NewErrorResponse(ErrorTypeRateLimit, "Rate limit exceeded", data)
		
		assert.Equal(t, ErrorTypeRateLimit, resp.Type)
		assert.Equal(t, "Rate limit exceeded", resp.Message)
		assert.Equal(t, data, resp.Data)
	})
	
	t.Run("creates error response without data", func(t *testing.T) {
		resp := NewErrorResponse(ErrorTypeAuthentication, "Authentication failed", nil)
		
		assert.Equal(t, ErrorTypeAuthentication, resp.Type)
		assert.Equal(t, "Authentication failed", resp.Message)
		assert.Nil(t, resp.Data)
	})
}

func TestNewJSONRPCError(t *testing.T) {
	t.Run("creates JSON-RPC error with all fields", func(t *testing.T) {
		data := map[string]interface{}{
			"details": "Invalid parameter format",
		}
		
		rpcErr := NewJSONRPCError(1, JSONRPCInvalidParams, "Invalid parameters", data)
		
		assert.Equal(t, "2.0", rpcErr.JSONRPC)
		assert.Equal(t, 1, rpcErr.ID)
		assert.Equal(t, JSONRPCInvalidParams, rpcErr.Error.Code)
		assert.Equal(t, "Invalid parameters", rpcErr.Error.Message)
		assert.Equal(t, data, rpcErr.Error.Data)
	})
	
	t.Run("creates JSON-RPC error without data", func(t *testing.T) {
		rpcErr := NewJSONRPCError("test-id", JSONRPCMethodNotFound, "Method not found", nil)
		
		assert.Equal(t, "2.0", rpcErr.JSONRPC)
		assert.Equal(t, "test-id", rpcErr.ID)
		assert.Equal(t, JSONRPCMethodNotFound, rpcErr.Error.Code)
		assert.Equal(t, "Method not found", rpcErr.Error.Message)
		assert.Nil(t, rpcErr.Error.Data)
	})
	
	t.Run("serializes to valid JSON", func(t *testing.T) {
		rpcErr := NewJSONRPCError(1, JSONRPCServerError, "Server error", map[string]interface{}{
			"type": "github_api_error",
		})
		
		jsonBytes, err := json.Marshal(rpcErr)
		require.NoError(t, err)
		
		var decoded JSONRPCError
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		
		assert.Equal(t, "2.0", decoded.JSONRPC)
		assert.Equal(t, float64(1), decoded.ID) // JSON numbers decode as float64
		assert.Equal(t, JSONRPCServerError, decoded.Error.Code)
		assert.Equal(t, "Server error", decoded.Error.Message)
	})
}

func TestNewHTTPErrorResponse(t *testing.T) {
	t.Run("creates HTTP error response wrapper", func(t *testing.T) {
		data := map[string]interface{}{
			"hint": "Check your token configuration",
		}
		
		httpResp := NewHTTPErrorResponse(ErrorTypeInvalidToken, "Invalid token", data)
		
		assert.Equal(t, ErrorTypeInvalidToken, httpResp.Error.Type)
		assert.Equal(t, "Invalid token", httpResp.Error.Message)
		assert.Equal(t, data, httpResp.Error.Data)
	})
	
	t.Run("serializes to valid JSON", func(t *testing.T) {
		httpResp := NewHTTPErrorResponse(ErrorTypeAuthentication, "Auth failed", nil)
		
		jsonBytes, err := json.Marshal(httpResp)
		require.NoError(t, err)
		
		var decoded HTTPErrorResponse
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeAuthentication, decoded.Error.Type)
		assert.Equal(t, "Auth failed", decoded.Error.Message)
	})
}

func TestWriteHTTPError(t *testing.T) {
	t.Run("writes error response with correct status code and JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]interface{}{
			"retry_after": 3600,
		}
		
		WriteHTTPError(w, http.StatusTooManyRequests, ErrorTypeRateLimit, "Rate limit exceeded", data)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		
		var response HTTPErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeRateLimit, response.Error.Type)
		assert.Equal(t, "Rate limit exceeded", response.Error.Message)
		assert.Equal(t, float64(3600), response.Error.Data["retry_after"]) // JSON numbers decode as float64
	})
	
	t.Run("writes error response without data", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		WriteHTTPError(w, http.StatusUnauthorized, ErrorTypeAuthentication, "Authentication required", nil)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		
		var response HTTPErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeAuthentication, response.Error.Type)
		assert.Equal(t, "Authentication required", response.Error.Message)
		assert.Nil(t, response.Error.Data)
	})
}

func TestMapErrorTypeToHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		errorType  ErrorType
		wantStatus int
	}{
		{"authentication error", ErrorTypeAuthentication, http.StatusUnauthorized},
		{"missing authorization", ErrorTypeMissingAuthorization, http.StatusUnauthorized},
		{"invalid token", ErrorTypeInvalidToken, http.StatusUnauthorized},
		{"authorization error", ErrorTypeAuthorization, http.StatusForbidden},
		{"rate limit", ErrorTypeRateLimit, http.StatusTooManyRequests},
		{"not found", ErrorTypeNotFound, http.StatusNotFound},
		{"validation error", ErrorTypeValidation, http.StatusBadRequest},
		{"mcp protocol error", ErrorTypeMCPProtocol, http.StatusBadRequest},
		{"method not allowed", ErrorTypeMethodNotAllowed, http.StatusMethodNotAllowed},
		{"github api error", ErrorTypeGitHubAPI, http.StatusInternalServerError},
		{"internal error", ErrorTypeInternal, http.StatusInternalServerError},
		{"unknown error type", ErrorType("unknown"), http.StatusInternalServerError},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := MapErrorTypeToHTTPStatus(tt.errorType)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

func TestMapErrorTypeToJSONRPCCode(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		wantCode  JSONRPCErrorCode
	}{
		{"mcp protocol error", ErrorTypeMCPProtocol, JSONRPCInvalidRequest},
		{"validation error", ErrorTypeValidation, JSONRPCInvalidParams},
		{"not found", ErrorTypeNotFound, JSONRPCMethodNotFound},
		{"authentication error", ErrorTypeAuthentication, JSONRPCServerError},
		{"rate limit", ErrorTypeRateLimit, JSONRPCServerError},
		{"github api error", ErrorTypeGitHubAPI, JSONRPCServerError},
		{"internal error", ErrorTypeInternal, JSONRPCServerError},
		{"unknown error type", ErrorType("unknown"), JSONRPCServerError},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := MapErrorTypeToJSONRPCCode(tt.errorType)
			assert.Equal(t, tt.wantCode, code)
		})
	}
}

func TestErrorResponseJSONSerialization(t *testing.T) {
	t.Run("error response with data serializes correctly", func(t *testing.T) {
		resp := NewErrorResponse(ErrorTypeRateLimit, "Rate limit exceeded", map[string]interface{}{
			"retry_after": 3600,
			"limit":       5000,
			"remaining":   0,
		})
		
		jsonBytes, err := json.Marshal(resp)
		require.NoError(t, err)
		
		var decoded ErrorResponse
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeRateLimit, decoded.Type)
		assert.Equal(t, "Rate limit exceeded", decoded.Message)
		assert.Equal(t, float64(3600), decoded.Data["retry_after"])
		assert.Equal(t, float64(5000), decoded.Data["limit"])
		assert.Equal(t, float64(0), decoded.Data["remaining"])
	})
	
	t.Run("error response without data omits data field", func(t *testing.T) {
		resp := NewErrorResponse(ErrorTypeAuthentication, "Auth failed", nil)
		
		jsonBytes, err := json.Marshal(resp)
		require.NoError(t, err)
		
		// Verify data field is omitted in JSON
		var raw map[string]interface{}
		err = json.Unmarshal(jsonBytes, &raw)
		require.NoError(t, err)
		
		_, hasData := raw["data"]
		assert.False(t, hasData, "data field should be omitted when nil")
	})
}
