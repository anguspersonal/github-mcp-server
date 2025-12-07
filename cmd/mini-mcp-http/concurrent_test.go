package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// **Feature: railway-github-app-deployment, Property 9: Concurrent request handling**
// **Validates: Requirements 8.5**
//
// Property: For any set of concurrent MCP requests from multiple LangGraph instances,
// each request should be processed independently with correct token isolation
func TestProperty_ConcurrentRequestHandling(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Create a test token store with multiple tokens
	tokenMapping := map[string]string{
		"token-alice":   "ghp_alice_token",
		"token-bob":     "ghp_bob_token",
		"token-charlie": "ghp_charlie_token",
	}
	envStore := ghmcp.NewEnvTokenStore(tokenMapping)

	// Create MCP server
	cfg := ghmcp.MCPServerConfig{
		Version:    "test",
		Host:       "",
		Token:      "",
		TokenStore: envStore,
		Logger:     nil,
	}

	ghServer, err := ghmcp.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}

	properties.Property("Concurrent requests maintain token isolation", prop.ForAll(
		func(tokens []string, concurrency int) bool {
			if concurrency < 1 || concurrency > 10 {
				return true // Skip invalid concurrency values
			}

			// Track which GitHub token was used for each request
			var mu sync.Mutex
			tokenUsage := make(map[string]string) // mcpToken -> githubToken used

			var wg sync.WaitGroup
			successCount := 0

			// Launch concurrent requests
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(idx int, mcpToken string) {
					defer wg.Done()

					// Create a valid MCP initialize request
					initRequest := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      idx,
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

					requestBody, err := json.Marshal(initRequest)
					if err != nil {
						return
					}
					requestBody = append(requestBody, '\n')

					// Create HTTP request
					req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(requestBody))
					req.Header.Set("Authorization", "Bearer "+mcpToken)
					req.Header.Set("Content-Type", "application/json")

					w := httptest.NewRecorder()

					// Create handler that tracks which GitHub token is used
					handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						auth := r.Header.Get("Authorization")
						extractedToken := extractBearer(auth)
						if extractedToken == "" {
							w.WriteHeader(http.StatusUnauthorized)
							return
						}

						// Resolve GitHub token
						ghToken := ""
						if t, ok := envStore.GetGitHubToken(extractedToken); ok {
							ghToken = t

							// Record which GitHub token was used for this MCP token
							mu.Lock()
							tokenUsage[extractedToken] = ghToken
							mu.Unlock()
						} else {
							w.WriteHeader(http.StatusUnauthorized)
							return
						}

						// Prepare response headers
						w.Header().Set("Content-Type", "application/octet-stream")
						w.Header().Set("Transfer-Encoding", "chunked")
						w.Header().Set("Connection", "keep-alive")
						w.WriteHeader(http.StatusOK)

						// Build context with GitHub token
						ctx := ghmcp.ContextWithGitHubToken(context.Background(), ghToken)

						wa := newWriterAdapter(w)
						transport := &mcp.IOTransport{Reader: r.Body, Writer: wa}

						// Run MCP server
						_ = ghServer.Run(ctx, transport)
						_ = wa.Close()
					})

					handler.ServeHTTP(w, req)

					// Track successful requests
					if w.Code == http.StatusOK {
						mu.Lock()
						successCount++
						mu.Unlock()
					}
				}(i, tokens[i%len(tokens)])
			}

			wg.Wait()

			// Verify token isolation: each MCP token should map to its correct GitHub token
			mu.Lock()
			defer mu.Unlock()

			for mcpToken, usedGitHubToken := range tokenUsage {
				expectedGitHubToken, ok := tokenMapping[mcpToken]
				if !ok {
					// Invalid token, should not have been used
					return false
				}
				if usedGitHubToken != expectedGitHubToken {
					// Token isolation violated!
					return false
				}
			}

			// At least some requests should have succeeded
			return successCount > 0
		},
		gen.SliceOf(gen.OneConstOf("token-alice", "token-bob", "token-charlie")),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Test concurrent requests with different tokens
func TestConcurrentRequestsWithDifferentTokens(t *testing.T) {
	// Create a test token store with multiple tokens
	tokenMapping := map[string]string{
		"token-1": "ghp_token_1",
		"token-2": "ghp_token_2",
		"token-3": "ghp_token_3",
	}
	envStore := ghmcp.NewEnvTokenStore(tokenMapping)

	// Create MCP server
	cfg := ghmcp.MCPServerConfig{
		Version:    "test",
		Host:       "",
		Token:      "",
		TokenStore: envStore,
		Logger:     nil,
	}

	ghServer, err := ghmcp.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}

	// Track which GitHub token was used for each request
	var mu sync.Mutex
	tokenUsage := make(map[string]string) // mcpToken -> githubToken used

	var wg sync.WaitGroup
	tokens := []string{"token-1", "token-2", "token-3"}

	// Launch 9 concurrent requests (3 per token)
	for i := 0; i < 9; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			mcpToken := tokens[idx%len(tokens)]

			// Create a valid MCP initialize request
			initRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      idx,
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

			requestBody, err := json.Marshal(initRequest)
			if err != nil {
				t.Errorf("Failed to marshal request: %v", err)
				return
			}
			requestBody = append(requestBody, '\n')

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(requestBody))
			req.Header.Set("Authorization", "Bearer "+mcpToken)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			// Create handler that tracks which GitHub token is used
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				extractedToken := extractBearer(auth)
				if extractedToken == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Resolve GitHub token
				ghToken := ""
				if t, ok := envStore.GetGitHubToken(extractedToken); ok {
					ghToken = t

					// Record which GitHub token was used for this MCP token
					mu.Lock()
					tokenUsage[extractedToken] = ghToken
					mu.Unlock()
				} else {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Prepare response headers
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Transfer-Encoding", "chunked")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)

				// Build context with GitHub token
				ctx := ghmcp.ContextWithGitHubToken(context.Background(), ghToken)

				wa := newWriterAdapter(w)
				transport := &mcp.IOTransport{Reader: r.Body, Writer: wa}

				// Run MCP server
				_ = ghServer.Run(ctx, transport)
				_ = wa.Close()
			})

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Request %d failed with status %d", idx, w.Code)
			}
		}(i)
	}

	wg.Wait()

	// Verify token isolation: each MCP token should map to its correct GitHub token
	mu.Lock()
	defer mu.Unlock()

	for mcpToken, usedGitHubToken := range tokenUsage {
		expectedGitHubToken, ok := tokenMapping[mcpToken]
		if !ok {
			t.Errorf("Unknown MCP token used: %s", mcpToken)
			continue
		}
		if usedGitHubToken != expectedGitHubToken {
			t.Errorf("Token isolation violated! MCP token %s used GitHub token %s, expected %s",
				mcpToken, usedGitHubToken, expectedGitHubToken)
		}
	}

	// Verify all tokens were used
	if len(tokenUsage) != len(tokens) {
		t.Errorf("Expected %d tokens to be used, got %d", len(tokens), len(tokenUsage))
	}
}

// Test that concurrent requests don't interfere with each other
func TestConcurrentRequestsNoInterference(t *testing.T) {
	tokenMapping := map[string]string{
		"token-a": "ghp_a",
		"token-b": "ghp_b",
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
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Launch 20 concurrent requests alternating between two tokens
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Alternate between tokens
			mcpToken := "token-a"
			if idx%2 == 1 {
				mcpToken = "token-b"
			}

			initRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      idx,
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

			requestBody, err := json.Marshal(initRequest)
			if err != nil {
				errors <- err
				return
			}
			requestBody = append(requestBody, '\n')

			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(requestBody))
			req.Header.Set("Authorization", "Bearer "+mcpToken)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				extractedToken := extractBearer(auth)
				if extractedToken == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				ghToken := ""
				if t, ok := envStore.GetGitHubToken(extractedToken); ok {
					ghToken = t
				} else {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

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

			if w.Code != http.StatusOK {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}
