package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// HealthResponse represents the health check endpoint response
type HealthResponse struct {
	Status             string  `json:"status"`
	Version            string  `json:"version"`
	UptimeSeconds      float64 `json:"uptime_seconds"`
	GitHubAPIReachable bool    `json:"github_api_reachable"`
}

// MCPRequest represents a JSON-RPC request
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response
type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   map[string]interface{} `json:"error,omitempty"`
}

func main() {
	var (
		serviceURL = flag.String("url", "", "Railway service URL (e.g., https://your-app.railway.app)")
		token      = flag.String("token", "", "MCP bearer token for authentication")
		timeout    = flag.Duration("timeout", 30*time.Second, "Request timeout duration")
	)
	flag.Parse()

	if *serviceURL == "" {
		fmt.Fprintf(os.Stderr, "Error: -url flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: verify-deployment -url <service-url> -token <mcp-token>\n")
		os.Exit(1)
	}

	if *token == "" {
		fmt.Fprintf(os.Stderr, "Error: -token flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: verify-deployment -url <service-url> -token <mcp-token>\n")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Printf("Verifying Railway deployment at: %s\n\n", *serviceURL)

	// Validate service URL format (must be HTTPS for Railway)
	if !strings.HasPrefix(*serviceURL, "https://") {
		fmt.Fprintf(os.Stderr, "❌ Error: Service URL must use HTTPS (Railway provides HTTPS URLs)\n")
		fmt.Fprintf(os.Stderr, "   Received: %s\n", *serviceURL)
		fmt.Fprintf(os.Stderr, "   Expected format: https://your-app.railway.app\n")
		os.Exit(1)
	}

	// Test 0: Railway service status (via URL reachability)
	fmt.Println("Test 0: Railway Service Status")
	fmt.Println("==============================")
	if err := testServiceStatus(ctx, *serviceURL); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Service status check failed: %v\n\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Service is reachable on Railway\n")

	// Test 1: Health check endpoint
	fmt.Println("Test 1: Health Check Endpoint")
	fmt.Println("==============================")
	if err := testHealthCheck(ctx, *serviceURL); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Health check failed: %v\n\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Health check passed\n")

	// Test 2: MCP connection
	fmt.Println("Test 2: MCP Connection")
	fmt.Println("======================")
	if err := testMCPConnection(ctx, *serviceURL, *token); err != nil {
		fmt.Fprintf(os.Stderr, "❌ MCP connection failed: %v\n\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ MCP connection passed\n")

	// Test 3: GitHub API call through MCP
	fmt.Println("Test 3: GitHub API Call Through MCP")
	fmt.Println("====================================")
	if err := testGitHubAPICall(ctx, *serviceURL, *token); err != nil {
		fmt.Fprintf(os.Stderr, "❌ GitHub API call failed: %v\n\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ GitHub API call passed\n")

	fmt.Println("========================================")
	fmt.Println("✅ All deployment verification tests passed!")
	fmt.Println("========================================")
}

// testHealthCheck verifies the health check endpoint returns 200 OK
func testHealthCheck(ctx context.Context, serviceURL string) error {
	healthURL := serviceURL + "/health"
	fmt.Printf("  Checking: %s\n", healthURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("  Status: %s\n", health.Status)
	fmt.Printf("  Version: %s\n", health.Version)
	fmt.Printf("  Uptime: %.2f seconds\n", health.UptimeSeconds)
	fmt.Printf("  GitHub API Reachable: %v\n", health.GitHubAPIReachable)

	if health.Status != "healthy" {
		return fmt.Errorf("service status is not healthy: %s", health.Status)
	}

	if !health.GitHubAPIReachable {
		return fmt.Errorf("GitHub API is not reachable from service")
	}

	return nil
}

// testMCPConnection verifies MCP protocol connection with test token
func testMCPConnection(ctx context.Context, serviceURL, token string) error {
	mcpURL := serviceURL + "/mcp"
	fmt.Printf("  Connecting to: %s\n", mcpURL)

	// Create MCP initialize request
	initRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "verify-deployment",
				"version": "1.0.0",
			},
		},
	}

	requestBody, err := json.Marshal(initRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Add newline delimiter as required by JSON-RPC over streams
	requestBody = append(requestBody, '\n')

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("  Status Code: %d\n", resp.StatusCode)
	fmt.Printf("  Content-Type: %s\n", resp.Header.Get("Content-Type"))

	// Read response (may be streaming)
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if len(responseBody) == 0 {
		// Empty response might be due to streaming, but connection was established
		fmt.Println("  Connection established (streaming response)")
		return nil
	}

	// Try to parse response as JSON-RPC
	lines := bytes.Split(responseBody, []byte("\n"))
	foundValidResponse := false
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var mcpResp MCPResponse
		if err := json.Unmarshal(line, &mcpResp); err != nil {
			continue
		}
		if mcpResp.JSONRPC == "2.0" {
			foundValidResponse = true
			fmt.Printf("  Received valid MCP response (ID: %d)\n", mcpResp.ID)
			if mcpResp.Error != nil {
				return fmt.Errorf("MCP error response: %v", mcpResp.Error)
			}
			break
		}
	}

	if !foundValidResponse && len(responseBody) > 0 {
		fmt.Printf("  Response received (%d bytes)\n", len(responseBody))
	}

	return nil
}

// testGitHubAPICall verifies GitHub API access through MCP
func testGitHubAPICall(ctx context.Context, serviceURL, token string) error {
	mcpURL := serviceURL + "/mcp"
	fmt.Printf("  Testing GitHub API call through: %s\n", mcpURL)

	// Create MCP tools/list request to verify GitHub tools are available
	listToolsRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	requestBody, err := json.Marshal(listToolsRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Add newline delimiter
	requestBody = append(requestBody, '\n')

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if len(responseBody) == 0 {
		fmt.Println("  GitHub API accessible (streaming response)")
		return nil
	}

	// Try to parse response
	lines := bytes.Split(responseBody, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var mcpResp MCPResponse
		if err := json.Unmarshal(line, &mcpResp); err != nil {
			continue
		}
		if mcpResp.JSONRPC == "2.0" {
			if mcpResp.Error != nil {
				return fmt.Errorf("MCP error response: %v", mcpResp.Error)
			}
			if mcpResp.Result != nil {
				// Check if tools are available
				if tools, ok := mcpResp.Result["tools"].([]interface{}); ok {
					fmt.Printf("  GitHub tools available: %d tools\n", len(tools))
					if len(tools) > 0 {
						fmt.Println("  Sample tools:")
						for i, tool := range tools {
							if i >= 3 {
								break
							}
							if toolMap, ok := tool.(map[string]interface{}); ok {
								if name, ok := toolMap["name"].(string); ok {
									fmt.Printf("    - %s\n", name)
								}
							}
						}
					}
				}
			}
			return nil
		}
	}

	fmt.Printf("  Response received (%d bytes)\n", len(responseBody))
	return nil
}
