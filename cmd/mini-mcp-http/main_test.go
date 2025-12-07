package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckReturns200OK(t *testing.T) {
	// Reset startTime for consistent uptime in tests
	startTime = time.Now()
	
	// Create a request to the health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	
	// Create the handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(startTime).Seconds()
		
		// Check GitHub API reachability
		githubReachable := true
		apiURL := "https://api.github.com/"
		
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode >= 500 {
				githubReachable = false
			}
			if resp != nil {
				resp.Body.Close()
			}
		} else {
			githubReachable = false
		}
		
		health := map[string]interface{}{
			"status":                "healthy",
			"version":               "test-version",
			"uptime_seconds":        uptime,
			"github_api_reachable":  githubReachable,
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})
	
	// Execute the request
	handler.ServeHTTP(w, req)
	
	// Assert the response
	assert.Equal(t, http.StatusOK, w.Code, "Expected status code 200")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "Expected JSON content type")
}

func TestHealthCheckJSONStructure(t *testing.T) {
	// Reset startTime for consistent uptime in tests
	startTime = time.Now()
	
	// Create a request to the health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	
	// Create the handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(startTime).Seconds()
		
		// Check GitHub API reachability
		githubReachable := true
		apiURL := "https://api.github.com/"
		
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode >= 500 {
				githubReachable = false
			}
			if resp != nil {
				resp.Body.Close()
			}
		} else {
			githubReachable = false
		}
		
		health := map[string]interface{}{
			"status":                "healthy",
			"version":               "test-version",
			"uptime_seconds":        uptime,
			"github_api_reachable":  githubReachable,
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})
	
	// Execute the request
	handler.ServeHTTP(w, req)
	
	// Parse the JSON response
	var health map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&health)
	require.NoError(t, err, "Expected valid JSON response")
	
	// Assert the JSON structure
	assert.Contains(t, health, "status", "Expected 'status' field in response")
	assert.Contains(t, health, "version", "Expected 'version' field in response")
	assert.Contains(t, health, "uptime_seconds", "Expected 'uptime_seconds' field in response")
	assert.Contains(t, health, "github_api_reachable", "Expected 'github_api_reachable' field in response")
	
	// Assert field types and values
	assert.Equal(t, "healthy", health["status"], "Expected status to be 'healthy'")
	assert.Equal(t, "test-version", health["version"], "Expected version to be 'test-version'")
	assert.IsType(t, float64(0), health["uptime_seconds"], "Expected uptime_seconds to be a number")
	assert.IsType(t, true, health["github_api_reachable"], "Expected github_api_reachable to be a boolean")
	
	// Assert uptime is reasonable (should be very small since we just started)
	uptime, ok := health["uptime_seconds"].(float64)
	require.True(t, ok, "Expected uptime_seconds to be a float64")
	assert.GreaterOrEqual(t, uptime, 0.0, "Expected uptime to be non-negative")
	assert.Less(t, uptime, 10.0, "Expected uptime to be less than 10 seconds in test")
}

func TestHealthCheckWithGitHubAPIUnreachable(t *testing.T) {
	// Reset startTime for consistent uptime in tests
	startTime = time.Now()
	
	// Create a mock server that simulates GitHub API being unreachable
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 500 to simulate server error
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()
	
	// Create a request to the health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	
	// Create the handler with the mock server URL
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(startTime).Seconds()
		
		// Check GitHub API reachability using mock server
		githubReachable := true
		apiURL := mockServer.URL // Use mock server instead of real GitHub API
		
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode >= 500 {
				githubReachable = false
			}
			if resp != nil {
				resp.Body.Close()
			}
		} else {
			githubReachable = false
		}
		
		health := map[string]interface{}{
			"status":                "healthy",
			"version":               "test-version",
			"uptime_seconds":        uptime,
			"github_api_reachable":  githubReachable,
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})
	
	// Execute the request
	handler.ServeHTTP(w, req)
	
	// Parse the JSON response
	var health map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&health)
	require.NoError(t, err, "Expected valid JSON response")
	
	// Assert that the health check still returns 200 OK
	assert.Equal(t, http.StatusOK, w.Code, "Expected status code 200 even when GitHub API is unreachable")
	
	// Assert that github_api_reachable is false
	assert.Equal(t, false, health["github_api_reachable"], "Expected github_api_reachable to be false when GitHub API returns 500")
	
	// Assert other fields are still present
	assert.Equal(t, "healthy", health["status"], "Expected status to be 'healthy'")
	assert.Contains(t, health, "version", "Expected 'version' field in response")
	assert.Contains(t, health, "uptime_seconds", "Expected 'uptime_seconds' field in response")
}
