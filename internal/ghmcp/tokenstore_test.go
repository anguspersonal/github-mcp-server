package ghmcp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: railway-github-app-deployment, Property 5: Token resolution correctness
// Validates: Requirements 2.4, 4.5
// Property: For any MCP token in the token mapping, the system should resolve it to the correct GitHub installation token or return an authentication error
func TestTokenResolutionCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: EnvTokenStore resolves mapped tokens correctly
	properties.Property("EnvTokenStore resolves mapped MCP tokens to correct GitHub tokens", prop.ForAll(
		func(mcpToken string, githubToken string) bool {
			// Create a mapping
			mapping := map[string]string{
				mcpToken: githubToken,
			}
			
			store := &EnvTokenStore{mapping: mapping}
			
			// Resolve the token
			resolved, ok := store.GetGitHubToken(mcpToken)
			
			// Should successfully resolve to the correct token
			return ok && resolved == githubToken
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 2: EnvTokenStore returns false for unmapped tokens
	properties.Property("EnvTokenStore returns false for unmapped MCP tokens", prop.ForAll(
		func(mcpToken string, unmappedToken string) bool {
			// Ensure unmappedToken is different from mcpToken
			if mcpToken == unmappedToken {
				return true // Skip this case
			}
			
			// Create a mapping with only mcpToken
			mapping := map[string]string{
				mcpToken: "some-github-token",
			}
			
			store := &EnvTokenStore{mapping: mapping}
			
			// Try to resolve an unmapped token
			_, ok := store.GetGitHubToken(unmappedToken)
			
			// Should return false
			return !ok
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 3: Token resolution is consistent
	properties.Property("token resolution is consistent across multiple calls", prop.ForAll(
		func(mcpToken string, githubToken string) bool {
			mapping := map[string]string{
				mcpToken: githubToken,
			}
			
			store := &EnvTokenStore{mapping: mapping}
			
			// Resolve the token multiple times
			resolved1, ok1 := store.GetGitHubToken(mcpToken)
			resolved2, ok2 := store.GetGitHubToken(mcpToken)
			resolved3, ok3 := store.GetGitHubToken(mcpToken)
			
			// All resolutions should be identical
			return ok1 && ok2 && ok3 &&
				resolved1 == githubToken &&
				resolved2 == githubToken &&
				resolved3 == githubToken &&
				resolved1 == resolved2 &&
				resolved2 == resolved3
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 4: Different MCP tokens resolve to different GitHub tokens
	properties.Property("different MCP tokens resolve to different GitHub tokens", prop.ForAll(
		func(mcpToken1 string, mcpToken2 string, githubToken1 string, githubToken2 string) bool {
			// Ensure tokens are different
			if mcpToken1 == mcpToken2 || githubToken1 == githubToken2 {
				return true // Skip this case
			}
			
			mapping := map[string]string{
				mcpToken1: githubToken1,
				mcpToken2: githubToken2,
			}
			
			store := &EnvTokenStore{mapping: mapping}
			
			// Resolve both tokens
			resolved1, ok1 := store.GetGitHubToken(mcpToken1)
			resolved2, ok2 := store.GetGitHubToken(mcpToken2)
			
			// Both should resolve successfully to their respective tokens
			return ok1 && ok2 &&
				resolved1 == githubToken1 &&
				resolved2 == githubToken2 &&
				resolved1 != resolved2
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 5 }), // Ensure different
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 5 }), // Ensure different
	))

	properties.TestingRun(t)
}

// Feature: railway-github-app-deployment, Property 5: Token resolution correctness
// Validates: Requirements 2.4, 4.5
// Property: InstallationTokenStore resolves installation mappings correctly
func TestInstallationTokenStoreResolution(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generate a test RSA private key for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Property 1: InstallationTokenStore correctly parses installation mappings
	properties.Property("InstallationTokenStore parses installation:<id> format correctly", prop.ForAll(
		func(mcpToken string, installationID int64) bool {
			// Ensure positive installation ID
			if installationID <= 0 {
				installationID = -installationID
				if installationID <= 0 {
					installationID = 1
				}
			}
			
			mapping := map[string]string{
				mcpToken: fmt.Sprintf("installation:%d", installationID),
			}
			
			// Create InstallationTokenStore (will fail to mint tokens without real API, but should parse mapping)
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// The mapping should be stored correctly
			// We can't test actual token minting without a real GitHub API,
			// but we can verify the store was created successfully with the mapping
			return store != nil && store.mapping[mcpToken] == fmt.Sprintf("installation:%d", installationID)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.Int64(),
	))

	// Property 2: InstallationTokenStore returns false for unmapped tokens
	properties.Property("InstallationTokenStore returns false for unmapped MCP tokens", prop.ForAll(
		func(mcpToken string, unmappedToken string, installationID int64) bool {
			// Ensure tokens are different
			if mcpToken == unmappedToken {
				return true // Skip this case
			}
			
			// Ensure positive installation ID
			if installationID <= 0 {
				installationID = -installationID
				if installationID <= 0 {
					installationID = 1
				}
			}
			
			mapping := map[string]string{
				mcpToken: fmt.Sprintf("installation:%d", installationID),
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Try to resolve an unmapped token
			_, ok := store.GetGitHubToken(unmappedToken)
			
			// Should return false
			return !ok
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 5 }), // Ensure different
		gen.Int64(),
	))

	// Property 3: InstallationTokenStore handles direct token mappings (compatibility)
	properties.Property("InstallationTokenStore handles direct token mappings for compatibility", prop.ForAll(
		func(mcpToken string, directToken string) bool {
			// Create a mapping with a direct token (not installation: format)
			mapping := map[string]string{
				mcpToken: directToken,
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Resolve the token
			resolved, ok := store.GetGitHubToken(mcpToken)
			
			// Should resolve to the direct token
			return ok && resolved == directToken
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && !hasPrefix(s, "installation:") }),
	))

	// Property 4: Invalid installation ID format returns false
	properties.Property("InstallationTokenStore returns false for invalid installation ID format", prop.ForAll(
		func(mcpToken string, invalidID string) bool {
			// Ensure invalidID is not a valid integer
			if _, err := fmt.Sscanf(invalidID, "%d", new(int64)); err == nil {
				return true // Skip valid integers
			}
			
			mapping := map[string]string{
				mcpToken: "installation:" + invalidID,
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Try to resolve the token with invalid installation ID
			_, ok := store.GetGitHubToken(mcpToken)
			
			// Should return false due to invalid format
			return !ok
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// Helper function to check if string has prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Feature: railway-github-app-deployment, Property 6: Installation token caching
// Validates: Requirements 2.5
// Property: For any installation token with remaining validity > 1 minute, subsequent requests should reuse the cached token rather than minting a new one
func TestInstallationTokenCaching(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generate a test RSA private key for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Property 1: Tokens with > 1 minute validity are cached and reused
	properties.Property("tokens with remaining validity > 1 minute are reused from cache", prop.ForAll(
		func(installationID int64, tokenValue string, minutesValid int) bool {
			// Ensure positive installation ID
			if installationID <= 0 {
				installationID = -installationID
				if installationID <= 0 {
					installationID = 1
				}
			}
			
			// Ensure validity is > 1 minute (between 2 and 60 minutes)
			if minutesValid < 2 {
				minutesValid = 2
			}
			if minutesValid > 60 {
				minutesValid = 60
			}
			
			// Create store
			mapping := map[string]string{
				"test-mcp-token": fmt.Sprintf("installation:%d", installationID),
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Manually populate the cache with a token that has > 1 minute validity
			expiry := time.Now().Add(time.Duration(minutesValid) * time.Minute)
			store.mu.Lock()
			store.cache[installationID] = &cachedToken{
				token:  tokenValue,
				expiry: expiry,
			}
			store.mu.Unlock()
			
			// First call should return the cached token
			resolved1, ok1 := store.GetGitHubToken("test-mcp-token")
			
			// Second call should also return the same cached token
			resolved2, ok2 := store.GetGitHubToken("test-mcp-token")
			
			// Third call should also return the same cached token
			resolved3, ok3 := store.GetGitHubToken("test-mcp-token")
			
			// All calls should succeed and return the same cached token
			return ok1 && ok2 && ok3 &&
				resolved1 == tokenValue &&
				resolved2 == tokenValue &&
				resolved3 == tokenValue &&
				resolved1 == resolved2 &&
				resolved2 == resolved3
		},
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 10 }),
		gen.IntRange(2, 60),
	))

	// Property 2: Tokens with <= 1 minute validity are not reused (cache miss)
	properties.Property("tokens with remaining validity <= 1 minute trigger cache miss", prop.ForAll(
		func(installationID int64, tokenValue string, secondsValid int) bool {
			// Ensure positive installation ID
			if installationID <= 0 {
				installationID = -installationID
				if installationID <= 0 {
					installationID = 1
				}
			}
			
			// Ensure validity is <= 1 minute (between 0 and 60 seconds)
			if secondsValid < 0 {
				secondsValid = 0
			}
			if secondsValid > 60 {
				secondsValid = 60
			}
			
			// Create store
			mapping := map[string]string{
				"test-mcp-token": fmt.Sprintf("installation:%d", installationID),
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Manually populate the cache with a token that has <= 1 minute validity
			expiry := time.Now().Add(time.Duration(secondsValid) * time.Second)
			store.mu.Lock()
			store.cache[installationID] = &cachedToken{
				token:  tokenValue,
				expiry: expiry,
			}
			store.mu.Unlock()
			
			// This call should trigger a cache miss because the token is expiring soon
			// It will fail to mint a new token (no real API), but we can verify the cache miss occurred
			// by checking that it doesn't return the cached token
			resolved, ok := store.GetGitHubToken("test-mcp-token")
			
			// Should fail because it tries to mint a new token and there's no real API
			// The important thing is it didn't return the cached token
			return !ok || resolved != tokenValue
		},
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 10 }),
		gen.IntRange(0, 60),
	))

	// Property 3: Different installation IDs have independent caches
	properties.Property("different installation IDs maintain independent cache entries", prop.ForAll(
		func(installationID1 int64, installationID2 int64, token1 string, token2 string) bool {
			// Ensure positive installation IDs
			if installationID1 <= 0 {
				installationID1 = -installationID1
				if installationID1 <= 0 {
					installationID1 = 1
				}
			}
			if installationID2 <= 0 {
				installationID2 = -installationID2
				if installationID2 <= 0 {
					installationID2 = 2
				}
			}
			
			// Ensure different installation IDs
			if installationID1 == installationID2 {
				installationID2 = installationID1 + 1
			}
			
			// Ensure different tokens
			if token1 == token2 {
				token2 = token2 + "_different"
			}
			
			// Create store with two different installation mappings
			mapping := map[string]string{
				"mcp-token-1": fmt.Sprintf("installation:%d", installationID1),
				"mcp-token-2": fmt.Sprintf("installation:%d", installationID2),
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Populate cache with different tokens for different installation IDs
			expiry := time.Now().Add(10 * time.Minute)
			store.mu.Lock()
			store.cache[installationID1] = &cachedToken{
				token:  token1,
				expiry: expiry,
			}
			store.cache[installationID2] = &cachedToken{
				token:  token2,
				expiry: expiry,
			}
			store.mu.Unlock()
			
			// Resolve both tokens
			resolved1, ok1 := store.GetGitHubToken("mcp-token-1")
			resolved2, ok2 := store.GetGitHubToken("mcp-token-2")
			
			// Both should resolve successfully to their respective cached tokens
			return ok1 && ok2 &&
				resolved1 == token1 &&
				resolved2 == token2 &&
				resolved1 != resolved2
		},
		gen.Int64(),
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 10 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 10 }),
	))

	// Property 4: Cache is thread-safe for concurrent access
	properties.Property("cache handles concurrent access safely", prop.ForAll(
		func(installationID int64, tokenValue string) bool {
			// Ensure positive installation ID
			if installationID <= 0 {
				installationID = -installationID
				if installationID <= 0 {
					installationID = 1
				}
			}
			
			// Create store
			mapping := map[string]string{
				"test-mcp-token": fmt.Sprintf("installation:%d", installationID),
			}
			
			store, err := NewInstallationTokenStore(12345, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				return false
			}
			
			// Populate cache
			expiry := time.Now().Add(10 * time.Minute)
			store.mu.Lock()
			store.cache[installationID] = &cachedToken{
				token:  tokenValue,
				expiry: expiry,
			}
			store.mu.Unlock()
			
			// Perform concurrent reads
			results := make(chan string, 10)
			for i := 0; i < 10; i++ {
				go func() {
					resolved, ok := store.GetGitHubToken("test-mcp-token")
					if ok {
						results <- resolved
					} else {
						results <- ""
					}
				}()
			}
			
			// Collect results
			allMatch := true
			for i := 0; i < 10; i++ {
				result := <-results
				if result != tokenValue {
					allMatch = false
				}
			}
			
			return allMatch
		},
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 10 }),
	))

	properties.TestingRun(t)
}

// Unit Tests for Token Store
// Requirements: 2.1, 2.2, 2.3

// Test EnvTokenStore mapping resolution
func TestEnvTokenStore_MappingResolution(t *testing.T) {
	tests := []struct {
		name          string
		mapping       map[string]string
		mcpToken      string
		expectToken   string
		expectSuccess bool
	}{
		{
			name: "resolves existing token",
			mapping: map[string]string{
				"mcp-key-1": "github-token-1",
				"mcp-key-2": "github-token-2",
			},
			mcpToken:      "mcp-key-1",
			expectToken:   "github-token-1",
			expectSuccess: true,
		},
		{
			name: "returns false for non-existent token",
			mapping: map[string]string{
				"mcp-key-1": "github-token-1",
			},
			mcpToken:      "non-existent-key",
			expectToken:   "",
			expectSuccess: false,
		},
		{
			name:          "handles empty mapping",
			mapping:       map[string]string{},
			mcpToken:      "any-key",
			expectToken:   "",
			expectSuccess: false,
		},
		{
			name: "handles empty string token",
			mapping: map[string]string{
				"": "github-token-empty",
			},
			mcpToken:      "",
			expectToken:   "github-token-empty",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &EnvTokenStore{mapping: tt.mapping}
			token, ok := store.GetGitHubToken(tt.mcpToken)
			
			if ok != tt.expectSuccess {
				t.Errorf("GetGitHubToken() ok = %v, want %v", ok, tt.expectSuccess)
			}
			if token != tt.expectToken {
				t.Errorf("GetGitHubToken() token = %v, want %v", token, tt.expectToken)
			}
		})
	}
}

// Test EnvTokenStore creation from environment variable
func TestNewEnvTokenStoreFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectError bool
		expectMap   map[string]string
	}{
		{
			name:        "parses valid JSON",
			envValue:    `{"key1":"token1","key2":"token2"}`,
			expectError: false,
			expectMap: map[string]string{
				"key1": "token1",
				"key2": "token2",
			},
		},
		{
			name:        "handles empty environment variable",
			envValue:    "",
			expectError: false,
			expectMap:   map[string]string{},
		},
		{
			name:        "returns error for invalid JSON",
			envValue:    `{invalid json}`,
			expectError: true,
			expectMap:   nil,
		},
		{
			name:        "handles empty JSON object",
			envValue:    `{}`,
			expectError: false,
			expectMap:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVar := "TEST_TOKEN_MAP_" + t.Name()
			if tt.envValue != "" {
				os.Setenv(envVar, tt.envValue)
				defer os.Unsetenv(envVar)
			}

			store, err := NewEnvTokenStoreFromEnv(envVar)
			
			if tt.expectError {
				if err == nil {
					t.Error("NewEnvTokenStoreFromEnv() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("NewEnvTokenStoreFromEnv() unexpected error: %v", err)
				return
			}
			
			if store == nil {
				t.Fatal("NewEnvTokenStoreFromEnv() returned nil store")
			}
			
			// Verify mapping
			for key, expectedValue := range tt.expectMap {
				actualValue, ok := store.GetGitHubToken(key)
				if !ok {
					t.Errorf("Expected key %q not found in mapping", key)
				}
				if actualValue != expectedValue {
					t.Errorf("For key %q, got value %q, want %q", key, actualValue, expectedValue)
				}
			}
		})
	}
}

// Test InstallationTokenStore JWT creation
func TestInstallationTokenStore_JWTCreation(t *testing.T) {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	tests := []struct {
		name        string
		appID       int64
		privateKey  []byte
		expectError bool
	}{
		{
			name:        "creates JWT with valid credentials",
			appID:       12345,
			privateKey:  privateKeyPEM,
			expectError: false,
		},
		{
			name:        "creates JWT with different app ID",
			appID:       99999,
			privateKey:  privateKeyPEM,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := map[string]string{
				"test-token": "installation:123",
			}
			
			store, err := NewInstallationTokenStore(tt.appID, tt.privateKey, mapping, "https://api.github.com/")
			if err != nil {
				t.Fatalf("Failed to create InstallationTokenStore: %v", err)
			}
			
			// Create JWT
			jwt, err := store.createAppJWT()
			
			if tt.expectError {
				if err == nil {
					t.Error("createAppJWT() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("createAppJWT() unexpected error: %v", err)
				return
			}
			
			if jwt == "" {
				t.Error("createAppJWT() returned empty JWT")
			}
			
			// JWT should have 3 parts separated by dots
			parts := splitString(jwt, ".")
			if len(parts) != 3 {
				t.Errorf("JWT should have 3 parts, got %d", len(parts))
			}
			
			// Decode and verify payload contains app ID
			if len(parts) >= 2 {
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				if err != nil {
					t.Errorf("Failed to decode JWT payload: %v", err)
					return
				}
				
				var payload map[string]interface{}
				if err := json.Unmarshal(payloadBytes, &payload); err != nil {
					t.Errorf("Failed to unmarshal JWT payload: %v", err)
					return
				}
				
				// Check that iss (issuer) matches app ID
				iss, ok := payload["iss"]
				if !ok {
					t.Error("JWT payload missing 'iss' field")
				} else {
					// JSON numbers are float64
					issFloat, ok := iss.(float64)
					if !ok {
						t.Errorf("JWT 'iss' field is not a number: %T", iss)
					} else if int64(issFloat) != tt.appID {
						t.Errorf("JWT 'iss' = %d, want %d", int64(issFloat), tt.appID)
					}
				}
				
				// Check that iat and exp exist
				if _, ok := payload["iat"]; !ok {
					t.Error("JWT payload missing 'iat' field")
				}
				if _, ok := payload["exp"]; !ok {
					t.Error("JWT payload missing 'exp' field")
				}
			}
		})
	}
}

// Helper function to split string
func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// Test private key parsing (PKCS1, PKCS8)
func TestParseRSAPrivateKeyFromPEM(t *testing.T) {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}

	tests := []struct {
		name        string
		pemBytes    []byte
		expectError bool
	}{
		{
			name: "parses PKCS1 format",
			pemBytes: pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
			}),
			expectError: false,
		},
		{
			name: "parses PKCS8 format",
			pemBytes: func() []byte {
				pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
				if err != nil {
					t.Fatalf("Failed to marshal PKCS8: %v", err)
				}
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: pkcs8Bytes,
				})
			}(),
			expectError: false,
		},
		{
			name:        "returns error for invalid PEM",
			pemBytes:    []byte("not a valid pem"),
			expectError: true,
		},
		{
			name:        "returns error for empty input",
			pemBytes:    []byte(""),
			expectError: true,
		},
		{
			name: "returns error for non-RSA key",
			pemBytes: pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: []byte("not a private key"),
			}),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := parseRSAPrivateKeyFromPEM(tt.pemBytes)
			
			if tt.expectError {
				if err == nil {
					t.Error("parseRSAPrivateKeyFromPEM() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("parseRSAPrivateKeyFromPEM() unexpected error: %v", err)
				return
			}
			
			if key == nil {
				t.Error("parseRSAPrivateKeyFromPEM() returned nil key")
			}
			
			// Verify it's a valid RSA key by checking it can be used
			if key.N == nil {
				t.Error("Parsed key has nil modulus")
			}
		})
	}
}

// Test token expiry handling
func TestInstallationTokenStore_TokenExpiry(t *testing.T) {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	tests := []struct {
		name              string
		cacheExpiry       time.Time
		expectCacheHit    bool
		expectTokenReturn bool
	}{
		{
			name:              "uses cached token with > 1 minute remaining",
			cacheExpiry:       time.Now().Add(5 * time.Minute),
			expectCacheHit:    true,
			expectTokenReturn: true,
		},
		{
			name:              "refreshes token with exactly 1 minute remaining",
			cacheExpiry:       time.Now().Add(1 * time.Minute),
			expectCacheHit:    false,
			expectTokenReturn: false, // Will fail to mint without real API
		},
		{
			name:              "refreshes token with < 1 minute remaining",
			cacheExpiry:       time.Now().Add(30 * time.Second),
			expectCacheHit:    false,
			expectTokenReturn: false, // Will fail to mint without real API
		},
		{
			name:              "refreshes expired token",
			cacheExpiry:       time.Now().Add(-1 * time.Minute),
			expectCacheHit:    false,
			expectTokenReturn: false, // Will fail to mint without real API
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := map[string]string{
				"test-mcp-token": "installation:12345",
			}
			
			store, err := NewInstallationTokenStore(99999, privateKeyPEM, mapping, "https://api.github.com/")
			if err != nil {
				t.Fatalf("Failed to create InstallationTokenStore: %v", err)
			}
			
			// Populate cache with a token
			cachedTokenValue := "cached-token-value-12345"
			store.mu.Lock()
			store.cache[12345] = &cachedToken{
				token:  cachedTokenValue,
				expiry: tt.cacheExpiry,
			}
			store.mu.Unlock()
			
			// Try to get the token
			token, ok := store.GetGitHubToken("test-mcp-token")
			
			if tt.expectCacheHit {
				// Should return the cached token
				if !ok {
					t.Error("Expected cache hit, but GetGitHubToken returned false")
				}
				if token != cachedTokenValue {
					t.Errorf("Expected cached token %q, got %q", cachedTokenValue, token)
				}
			} else {
				// Should attempt to refresh (will fail without real API)
				if tt.expectTokenReturn {
					if !ok {
						t.Error("Expected token return, but GetGitHubToken returned false")
					}
				} else {
					// Without real API, refresh will fail
					if ok && token == cachedTokenValue {
						t.Error("Expected cache miss and refresh attempt, but got cached token")
					}
				}
			}
		})
	}
}

// Test InstallationTokenStore initialization
func TestNewInstallationTokenStore(t *testing.T) {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	tests := []struct {
		name        string
		appID       int64
		privateKey  []byte
		mapping     map[string]string
		apiBase     string
		expectError bool
	}{
		{
			name:       "creates store with valid parameters",
			appID:      12345,
			privateKey: privateKeyPEM,
			mapping: map[string]string{
				"token1": "installation:111",
			},
			apiBase:     "https://api.github.com/",
			expectError: false,
		},
		{
			name:       "adds trailing slash to apiBase if missing",
			appID:      12345,
			privateKey: privateKeyPEM,
			mapping:    map[string]string{},
			apiBase:    "https://api.github.com",
			expectError: false,
		},
		{
			name:       "returns error for invalid private key",
			appID:      12345,
			privateKey: []byte("invalid key"),
			mapping:    map[string]string{},
			apiBase:    "https://api.github.com/",
			expectError: true,
		},
		{
			name:       "handles empty mapping",
			appID:      12345,
			privateKey: privateKeyPEM,
			mapping:    map[string]string{},
			apiBase:    "https://api.github.com/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewInstallationTokenStore(tt.appID, tt.privateKey, tt.mapping, tt.apiBase)
			
			if tt.expectError {
				if err == nil {
					t.Error("NewInstallationTokenStore() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("NewInstallationTokenStore() unexpected error: %v", err)
				return
			}
			
			if store == nil {
				t.Fatal("NewInstallationTokenStore() returned nil store")
			}
			
			// Verify store properties
			if store.appID != tt.appID {
				t.Errorf("store.appID = %d, want %d", store.appID, tt.appID)
			}
			
			if store.privateKey == nil {
				t.Error("store.privateKey is nil")
			}
			
			// Verify apiBase has trailing slash
			if store.apiBase[len(store.apiBase)-1] != '/' {
				t.Errorf("store.apiBase should end with '/', got %q", store.apiBase)
			}
			
			// Verify mapping
			if len(store.mapping) != len(tt.mapping) {
				t.Errorf("store.mapping length = %d, want %d", len(store.mapping), len(tt.mapping))
			}
			
			// Verify cache is initialized
			if store.cache == nil {
				t.Error("store.cache is nil")
			}
		})
	}
}

// Test EnvTokenStore Mapping method
func TestEnvTokenStore_Mapping(t *testing.T) {
	original := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	
	store := &EnvTokenStore{mapping: original}
	
	// Get a copy of the mapping
	copy := store.Mapping()
	
	// Verify the copy has the same content
	if len(copy) != len(original) {
		t.Errorf("Mapping() returned map with length %d, want %d", len(copy), len(original))
	}
	
	for key, value := range original {
		if copy[key] != value {
			t.Errorf("Mapping()[%q] = %q, want %q", key, copy[key], value)
		}
	}
	
	// Modify the copy and verify original is unchanged (shallow copy test)
	copy["key1"] = "modified"
	if store.mapping["key1"] == "modified" {
		t.Error("Modifying returned mapping affected original mapping")
	}
	
	// Add new key to copy and verify original is unchanged
	copy["new-key"] = "new-value"
	if _, exists := store.mapping["new-key"]; exists {
		t.Error("Adding key to returned mapping affected original mapping")
	}
}
