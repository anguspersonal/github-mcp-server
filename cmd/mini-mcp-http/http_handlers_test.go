package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Bearer token extraction
func TestBearerTokenExtraction(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		expectedToken string
	}{
		{
			name:          "Valid Bearer token",
			authHeader:    "Bearer test-token-123",
			expectedToken: "test-token-123",
		},
		{
			name:          "Valid Bearer token with extra spaces",
			authHeader:    "Bearer   test-token-456  ",
			expectedToken: "test-token-456",
		},
		{
			name:          "Case insensitive Bearer",
			authHeader:    "bearer test-token-789",
			expectedToken: "test-token-789",
		},
		{
			name:          "Mixed case Bearer",
			authHeader:    "BeArEr test-token-abc",
			expectedToken: "test-token-abc",
		},
		{
			name:          "Empty auth header",
			authHeader:    "",
			expectedToken: "",
		},
		{
			name:          "Missing Bearer prefix",
			authHeader:    "test-token-xyz",
			expectedToken: "",
		},
		{
			name:          "Wrong auth type",
			authHeader:    "Basic dGVzdDp0ZXN0",
			expectedToken: "",
		},
		{
			name:          "Bearer without token",
			authHeader:    "Bearer",
			expectedToken: "",
		},
		{
			name:          "Bearer with empty token",
			authHeader:    "Bearer ",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := extractBearer(tt.authHeader)
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

// Test Authorization header validation
func TestAuthorizationHeaderValidation(t *testing.T) {
	tokenMapping := map[string]string{
		"valid-token": "ghp_valid",
	}
	envStore := ghmcp.NewEnvTokenStore(tokenMapping)

	cfg := ghmcp.MCPServerConfig{
		Version:    "test",
		Host:       "",
		Token:      "",
		TokenStore: envStore,
		Logger:     nil,
	}

	ghServer, err := ghmcp.NewMCPServer(cfg)
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Valid token",
			authHeader:     "Bearer valid-token",
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
		{
			name:           "Missing Authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing_authorization",
		},
		{
			name:           "Invalid token format",
			authHeader:     "invalid-format",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing_authorization",
		},
		{
			name:           "Invalid token value",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid_token",
		},
		{
			name:           "Empty Bearer token",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing_authorization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal valid MCP request
			initRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"clientInfo": map[string]interface{}{
						"name":    "test-client",
						"version": "1.0.0",
					},
				},
			}
			requestBody, _ := json.Marshal(initRequest)
			requestBody = append(requestBody, '\n')

			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(requestBody))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Only allow POST
				if r.Method != http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "method_not_allowed",
							"message": "Only POST method is supported for MCP endpoint",
						},
					})
					return
				}

				auth := r.Header.Get("Authorization")
				mcpToken := extractBearer(auth)
				if mcpToken == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "missing_authorization",
							"message": "Authorization header with Bearer token is required",
						},
					})
					return
				}

				// Resolve GitHub token
				ghToken := ""
				if t, ok := envStore.GetGitHubToken(mcpToken); ok {
					ghToken = t
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "invalid_token",
							"message": "The provided MCP token is not valid or not found in token mapping",
						},
					})
					return
				}

				// Prepare response headers
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Transfer-Encoding", "chunked")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)

				ctx := ghmcp.ContextWithGitHubToken(context.Background(), ghToken)

				wa := newWriterAdapter(w)
				transport := &mcp.IOTransport{Reader: r.Body, Writer: wa}

				_ = ghServer.Run(ctx, transport)
				_ = wa.Close()
			})

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				errorObj, ok := response["error"].(map[string]interface{})
				require.True(t, ok, "Expected error object in response")

				errorType, ok := errorObj["type"].(string)
				require.True(t, ok, "Expected error type in response")

				assert.Equal(t, tt.expectedError, errorType)
			}
		})
	}
}

// Test HTTP method validation (POST only)
func TestHTTPMethodValidation(t *testing.T) {
	tokenMapping := map[string]string{
		"test-token": "ghp_test",
	}
	envStore := ghmcp.NewEnvTokenStore(tokenMapping)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "POST method allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
		{
			name:           "GET method not allowed",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method_not_allowed",
		},
		{
			name:           "PUT method not allowed",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method_not_allowed",
		},
		{
			name:           "DELETE method not allowed",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method_not_allowed",
		},
		{
			name:           "PATCH method not allowed",
			method:         http.MethodPatch,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method_not_allowed",
		},
		{
			name:           "OPTIONS method not allowed",
			method:         http.MethodOptions,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method_not_allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal valid MCP request
			initRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"clientInfo": map[string]interface{}{
						"name":    "test-client",
						"version": "1.0.0",
					},
				},
			}
			requestBody, _ := json.Marshal(initRequest)
			requestBody = append(requestBody, '\n')

			req := httptest.NewRequest(tt.method, "/mcp", bytes.NewReader(requestBody))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Only allow POST
				if r.Method != http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "method_not_allowed",
							"message": "Only POST method is supported for MCP endpoint",
							"details": map[string]string{
								"method_received": r.Method,
								"method_required": "POST",
							},
						},
					})
					return
				}

				auth := r.Header.Get("Authorization")
				mcpToken := extractBearer(auth)
				if mcpToken == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				ghToken := ""
				if t, ok := envStore.GetGitHubToken(mcpToken); ok {
					ghToken = t
				} else {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Transfer-Encoding", "chunked")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)
			})

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				errorObj, ok := response["error"].(map[string]interface{})
				require.True(t, ok, "Expected error object in response")

				errorType, ok := errorObj["type"].(string)
				require.True(t, ok, "Expected error type in response")

				assert.Equal(t, tt.expectedError, errorType)

				// Verify details are present
				details, ok := errorObj["details"].(map[string]interface{})
				require.True(t, ok, "Expected details in error response")

				methodReceived, ok := details["method_received"].(string)
				require.True(t, ok, "Expected method_received in details")
				assert.Equal(t, tt.method, methodReceived)

				methodRequired, ok := details["method_required"].(string)
				require.True(t, ok, "Expected method_required in details")
				assert.Equal(t, "POST", methodRequired)
			}
		})
	}
}

// Test response header configuration
func TestResponseHeaderConfiguration(t *testing.T) {
	tokenMapping := map[string]string{
		"test-token": "ghp_test",
	}
	envStore := ghmcp.NewEnvTokenStore(tokenMapping)

	cfg := ghmcp.MCPServerConfig{
		Version:    "test",
		Host:       "",
		Token:      "",
		TokenStore: envStore,
		Logger:     nil,
	}

	ghServer, err := ghmcp.NewMCPServer(cfg)
	require.NoError(t, err)

	// Create a valid MCP request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	requestBody, _ := json.Marshal(initRequest)
	requestBody = append(requestBody, '\n')

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(requestBody))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mcpToken := extractBearer(auth)
		if mcpToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ghToken := ""
		if t, ok := envStore.GetGitHubToken(mcpToken); ok {
			ghToken = t
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Prepare response headers for streaming
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		ctx := ghmcp.ContextWithGitHubToken(context.Background(), ghToken)

		wa := newWriterAdapter(w)
		transport := &mcp.IOTransport{Reader: r.Body, Writer: wa}

		_ = ghServer.Run(ctx, transport)
		_ = wa.Close()
	})

	handler.ServeHTTP(w, req)

	// Verify response headers
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Transfer-Encoding"), "chunked")
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
}

// Test error response structure
func TestErrorResponseStructure(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		authHeader     string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "Method not allowed error",
			method:         http.MethodGet,
			authHeader:     "Bearer test-token",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedType:   "method_not_allowed",
		},
		{
			name:           "Missing authorization error",
			method:         http.MethodPost,
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedType:   "missing_authorization",
		},
		{
			name:           "Invalid token error",
			method:         http.MethodPost,
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectedType:   "invalid_token",
		},
	}

	tokenMapping := map[string]string{
		"test-token": "ghp_test",
	}
	envStore := ghmcp.NewEnvTokenStore(tokenMapping)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/mcp", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "method_not_allowed",
							"message": "Only POST method is supported for MCP endpoint",
							"details": map[string]string{
								"method_received": r.Method,
								"method_required": "POST",
							},
						},
					})
					return
				}

				auth := r.Header.Get("Authorization")
				mcpToken := extractBearer(auth)
				if mcpToken == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "missing_authorization",
							"message": "Authorization header with Bearer token is required",
							"details": map[string]string{
								"header_format": "Authorization: Bearer <token>",
							},
						},
					})
					return
				}

				if _, ok := envStore.GetGitHubToken(mcpToken); !ok {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"type":    "invalid_token",
							"message": "The provided MCP token is not valid or not found in token mapping",
							"details": map[string]string{
								"hint": "Verify that your token is correctly configured in GITHUB_MCP_TOKEN_MAP",
							},
						},
					})
					return
				}
			})

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			// Verify error structure
			errorObj, ok := response["error"].(map[string]interface{})
			require.True(t, ok, "Expected error object in response")

			// Verify error has type
			errorType, ok := errorObj["type"].(string)
			require.True(t, ok, "Expected error type")
			assert.Equal(t, tt.expectedType, errorType)

			// Verify error has message
			message, ok := errorObj["message"].(string)
			require.True(t, ok, "Expected error message")
			assert.NotEmpty(t, message)

			// Verify error has details
			details, ok := errorObj["details"].(map[string]interface{})
			require.True(t, ok, "Expected error details")
			assert.NotEmpty(t, details)
		})
	}
}
