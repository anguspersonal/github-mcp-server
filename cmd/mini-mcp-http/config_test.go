package main

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

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: railway-github-app-deployment, Property 7: Environment variable validation
// Validates: Requirements 6.2
// Property: For any missing required environment variable, the system should fail startup with a clear error message indicating which variable is required
func TestEnvironmentVariableValidation(t *testing.T) {
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
	privateKeyB64 := base64.StdEncoding.EncodeToString(privateKeyPEM)

	// Property 1: Missing GITHUB_MCP_TOKEN_MAP causes validation error
	properties.Property("missing GITHUB_MCP_TOKEN_MAP causes validation error with clear message", prop.ForAll(
		func(tokenMapEnv string) bool {
			// Ensure the env var is not set
			os.Unsetenv(tokenMapEnv)

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention the missing env var
			errMsg := err.Error()
			return containsString(errMsg, tokenMapEnv) && containsString(errMsg, "required")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 5 }),
	))

	// Property 2: Invalid JSON in GITHUB_MCP_TOKEN_MAP causes validation error
	properties.Property("invalid JSON in token mapping causes validation error", prop.ForAll(
		func(invalidJSON string) bool {
			// Skip if it's actually valid JSON
			var testMap map[string]string
			if json.Unmarshal([]byte(invalidJSON), &testMap) == nil {
				return true // Skip valid JSON
			}

			tokenMapEnv := "TEST_TOKEN_MAP_INVALID_" + randomString(8)
			os.Setenv(tokenMapEnv, invalidJSON)
			defer os.Unsetenv(tokenMapEnv)

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention JSON format
			errMsg := err.Error()
			return containsString(errMsg, "JSON") || containsString(errMsg, "format")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && s != "{}" }),
	))

	// Property 3: Empty token mapping causes validation error
	properties.Property("empty token mapping causes validation error", prop.ForAll(
		func(tokenMapEnv string) bool {
			os.Setenv(tokenMapEnv, "{}")
			defer os.Unsetenv(tokenMapEnv)

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention empty mapping
			errMsg := err.Error()
			return containsString(errMsg, "empty")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 5 }),
	))

	// Property 4: Valid token mapping without GitHub App credentials succeeds
	properties.Property("valid token mapping without GitHub App credentials succeeds", prop.ForAll(
		func(mcpToken string, githubToken string) bool {
			tokenMapEnv := "TEST_TOKEN_MAP_VALID_" + randomString(8)
			mapping := map[string]string{
				mcpToken: githubToken,
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			// Ensure GitHub App env vars are not set
			os.Unsetenv("GITHUB_APP_ID")
			os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			config, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should succeed
			if err != nil {
				return false
			}

			// Should have the correct mapping
			return config != nil && config.TokenMapping[mcpToken] == githubToken
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 5: Invalid GITHUB_APP_ID format causes validation error
	properties.Property("invalid GITHUB_APP_ID format causes validation error", prop.ForAll(
		func(invalidAppID string, mcpToken string) bool {
			// Skip if it's actually a valid integer
			var testInt int64
			if _, err := fmt.Sscanf(invalidAppID, "%d", &testInt); err == nil {
				return true // Skip valid integers
			}

			tokenMapEnv := "TEST_TOKEN_MAP_APPID_" + randomString(8)
			mapping := map[string]string{
				mcpToken: "installation:12345",
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			os.Setenv("GITHUB_APP_ID", invalidAppID)
			os.Setenv("GITHUB_APP_PRIVATE_KEY_B64", privateKeyB64)
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention GITHUB_APP_ID
			errMsg := err.Error()
			return containsString(errMsg, "GITHUB_APP_ID")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 6: Missing GITHUB_APP_PRIVATE_KEY_B64 when GITHUB_APP_ID is set causes error
	properties.Property("missing GITHUB_APP_PRIVATE_KEY_B64 when GITHUB_APP_ID is set causes error", prop.ForAll(
		func(appID int64, mcpToken string) bool {
			// Ensure positive app ID
			if appID <= 0 {
				appID = -appID
				if appID <= 0 {
					appID = 1
				}
			}

			tokenMapEnv := "TEST_TOKEN_MAP_NOPK_" + randomString(8)
			mapping := map[string]string{
				mcpToken: "installation:12345",
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			os.Setenv("GITHUB_APP_ID", fmt.Sprintf("%d", appID))
			os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")
			defer os.Unsetenv("GITHUB_APP_ID")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention GITHUB_APP_PRIVATE_KEY_B64
			errMsg := err.Error()
			return containsString(errMsg, "GITHUB_APP_PRIVATE_KEY_B64")
		},
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 7: Missing GITHUB_APP_ID when GITHUB_APP_PRIVATE_KEY_B64 is set causes error
	properties.Property("missing GITHUB_APP_ID when GITHUB_APP_PRIVATE_KEY_B64 is set causes error", prop.ForAll(
		func(mcpToken string) bool {
			tokenMapEnv := "TEST_TOKEN_MAP_NOAPPID_" + randomString(8)
			mapping := map[string]string{
				mcpToken: "installation:12345",
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			os.Unsetenv("GITHUB_APP_ID")
			os.Setenv("GITHUB_APP_PRIVATE_KEY_B64", privateKeyB64)
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention GITHUB_APP_ID
			errMsg := err.Error()
			return containsString(errMsg, "GITHUB_APP_ID")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 8: Invalid private key format causes validation error
	properties.Property("invalid private key format causes validation error", prop.ForAll(
		func(invalidKey string, appID int64, mcpToken string) bool {
			// Ensure positive app ID
			if appID <= 0 {
				appID = -appID
				if appID <= 0 {
					appID = 1
				}
			}

			// Skip if it looks like a valid PEM key
			if containsString(invalidKey, "BEGIN") && containsString(invalidKey, "PRIVATE KEY") {
				return true
			}

			tokenMapEnv := "TEST_TOKEN_MAP_BADKEY_" + randomString(8)
			mapping := map[string]string{
				mcpToken: "installation:12345",
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			os.Setenv("GITHUB_APP_ID", fmt.Sprintf("%d", appID))
			os.Setenv("GITHUB_APP_PRIVATE_KEY_B64", base64.StdEncoding.EncodeToString([]byte(invalidKey)))
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention private key
			errMsg := err.Error()
			return containsString(errMsg, "GITHUB_APP_PRIVATE_KEY_B64") || containsString(errMsg, "private key")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property 9: Non-installation mapping with GitHub App credentials causes error
	properties.Property("non-installation mapping with GitHub App credentials causes error", prop.ForAll(
		func(mcpToken string, directToken string, appID int64) bool {
			// Ensure positive app ID
			if appID <= 0 {
				appID = -appID
				if appID <= 0 {
					appID = 1
				}
			}

			// Ensure directToken doesn't start with "installation:"
			if containsString(directToken, "installation:") {
				directToken = "token_" + directToken
			}

			tokenMapEnv := "TEST_TOKEN_MAP_DIRECT_" + randomString(8)
			mapping := map[string]string{
				mcpToken: directToken,
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			os.Setenv("GITHUB_APP_ID", fmt.Sprintf("%d", appID))
			os.Setenv("GITHUB_APP_PRIVATE_KEY_B64", privateKeyB64)
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should return an error
			if err == nil {
				return false
			}

			// Error should mention installation format
			errMsg := err.Error()
			return containsString(errMsg, "installation:")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.Int64(),
	))

	// Property 10: Valid GitHub App configuration succeeds
	properties.Property("valid GitHub App configuration succeeds", prop.ForAll(
		func(mcpToken string, installationID int64, appID int64) bool {
			// Ensure positive IDs
			if appID <= 0 {
				appID = -appID
				if appID <= 0 {
					appID = 1
				}
			}
			if installationID <= 0 {
				installationID = -installationID
				if installationID <= 0 {
					installationID = 1
				}
			}

			tokenMapEnv := "TEST_TOKEN_MAP_VALIDAPP_" + randomString(8)
			mapping := map[string]string{
				mcpToken: fmt.Sprintf("installation:%d", installationID),
			}
			mappingJSON, _ := json.Marshal(mapping)
			os.Setenv(tokenMapEnv, string(mappingJSON))
			defer os.Unsetenv(tokenMapEnv)

			os.Setenv("GITHUB_APP_ID", fmt.Sprintf("%d", appID))
			os.Setenv("GITHUB_APP_PRIVATE_KEY_B64", privateKeyB64)
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			config, err := ValidateConfig("", "", "test", tokenMapEnv)

			// Should succeed
			if err != nil {
				return false
			}

			// Should have correct configuration
			return config != nil &&
				config.GitHubAppID == appID &&
				len(config.GitHubAppPrivateKey) > 0 &&
				config.TokenMapping[mcpToken] == fmt.Sprintf("installation:%d", installationID)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.Int64(),
		gen.Int64(),
	))

	properties.TestingRun(t)
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

// Unit Tests for Configuration Validation
// Requirements: 6.2, 6.3, 6.4, 6.5

// Test missing required env vars fail startup
func TestValidateConfig_MissingRequiredEnvVars(t *testing.T) {
	tests := []struct {
		name          string
		setupEnv      func(tokenMapEnv string)
		tokenMapEnv   string
		expectError   bool
		errorContains string
	}{
		{
			name: "missing GITHUB_MCP_TOKEN_MAP",
			setupEnv: func(tokenMapEnv string) {
				os.Unsetenv(tokenMapEnv)
			},
			tokenMapEnv:   "TEST_MISSING_TOKEN_MAP",
			expectError:   true,
			errorContains: "required environment variable",
		},
		{
			name: "empty GITHUB_MCP_TOKEN_MAP",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, "")
			},
			tokenMapEnv:   "TEST_EMPTY_TOKEN_MAP",
			expectError:   true,
			errorContains: "required environment variable",
		},
		{
			name: "valid token map present",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"test":"token"}`)
			},
			tokenMapEnv:   "TEST_VALID_TOKEN_MAP",
			expectError:   false,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			tt.setupEnv(tt.tokenMapEnv)
			defer os.Unsetenv(tt.tokenMapEnv)
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			// Validate config
			_, err := ValidateConfig("", "", "test", tt.tokenMapEnv)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContains)
					return
				}
				if !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// Test invalid JSON in token mapping fails startup
func TestValidateConfig_InvalidJSON(t *testing.T) {
	tests := []struct {
		name          string
		tokenMapJSON  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "invalid JSON syntax",
			tokenMapJSON:  `{invalid json}`,
			expectError:   true,
			errorContains: "invalid JSON format",
		},
		{
			name:          "not a JSON object",
			tokenMapJSON:  `["array", "not", "object"]`,
			expectError:   true,
			errorContains: "invalid JSON format",
		},
		{
			name:          "empty JSON object",
			tokenMapJSON:  `{}`,
			expectError:   true,
			errorContains: "empty mapping",
		},
		{
			name:          "valid JSON with mapping",
			tokenMapJSON:  `{"mcp_token":"github_token"}`,
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "valid JSON with multiple mappings",
			tokenMapJSON:  `{"token1":"github1","token2":"github2"}`,
			expectError:   false,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenMapEnv := "TEST_JSON_VALIDATION_" + randomString(8)
			os.Setenv(tokenMapEnv, tt.tokenMapJSON)
			defer os.Unsetenv(tokenMapEnv)
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContains)
					return
				}
				if !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// Test invalid GitHub App credentials fail startup
func TestValidateConfig_InvalidGitHubAppCredentials(t *testing.T) {
	// Generate a valid test key for comparison
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	validPrivateKeyB64 := base64.StdEncoding.EncodeToString(privateKeyPEM)

	tests := []struct {
		name          string
		appID         string
		privateKeyB64 string
		tokenMapping  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "invalid GITHUB_APP_ID format (non-numeric)",
			appID:         "not-a-number",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:12345"}`,
			expectError:   true,
			errorContains: "GITHUB_APP_ID",
		},
		{
			name:          "negative GITHUB_APP_ID",
			appID:         "-12345",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:12345"}`,
			expectError:   true,
			errorContains: "GITHUB_APP_ID",
		},
		{
			name:          "zero GITHUB_APP_ID",
			appID:         "0",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:12345"}`,
			expectError:   true,
			errorContains: "GITHUB_APP_ID",
		},
		{
			name:          "invalid private key (not PEM)",
			appID:         "12345",
			privateKeyB64: base64.StdEncoding.EncodeToString([]byte("not a valid key")),
			tokenMapping:  `{"token":"installation:12345"}`,
			expectError:   true,
			errorContains: "GITHUB_APP_PRIVATE_KEY_B64",
		},
		{
			name:          "missing GITHUB_APP_ID when private key is set",
			appID:         "",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:12345"}`,
			expectError:   true,
			errorContains: "GITHUB_APP_ID",
		},
		{
			name:          "missing GITHUB_APP_PRIVATE_KEY_B64 when app ID is set",
			appID:         "12345",
			privateKeyB64: "",
			tokenMapping:  `{"token":"installation:12345"}`,
			expectError:   true,
			errorContains: "GITHUB_APP_PRIVATE_KEY_B64",
		},
		{
			name:          "non-installation mapping with GitHub App credentials",
			appID:         "12345",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"direct_github_token"}`,
			expectError:   true,
			errorContains: "installation:",
		},
		{
			name:          "invalid installation ID format",
			appID:         "12345",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:not-a-number"}`,
			expectError:   true,
			errorContains: "invalid installation ID",
		},
		{
			name:          "negative installation ID",
			appID:         "12345",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:-999"}`,
			expectError:   true,
			errorContains: "invalid installation ID",
		},
		{
			name:          "valid GitHub App configuration",
			appID:         "12345",
			privateKeyB64: validPrivateKeyB64,
			tokenMapping:  `{"token":"installation:67890"}`,
			expectError:   false,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenMapEnv := "TEST_GITHUB_APP_" + randomString(8)
			os.Setenv(tokenMapEnv, tt.tokenMapping)
			defer os.Unsetenv(tokenMapEnv)

			if tt.appID != "" {
				os.Setenv("GITHUB_APP_ID", tt.appID)
			} else {
				os.Unsetenv("GITHUB_APP_ID")
			}
			defer os.Unsetenv("GITHUB_APP_ID")

			if tt.privateKeyB64 != "" {
				os.Setenv("GITHUB_APP_PRIVATE_KEY_B64", tt.privateKeyB64)
			} else {
				os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")
			}
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			_, err := ValidateConfig("", "", "test", tokenMapEnv)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContains)
					return
				}
				if !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// Test default values are applied correctly
func TestValidateConfig_DefaultValues(t *testing.T) {
	tests := []struct {
		name              string
		setupEnv          func(tokenMapEnv string)
		tokenMapEnv       string
		expectPort        string
		expectGitHubHost  string
		expectReadOnly    bool
		expectLockdown    bool
	}{
		{
			name: "default PORT is 8080",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Unsetenv("PORT")
			},
			tokenMapEnv:      "TEST_DEFAULT_PORT",
			expectPort:       "8080",
			expectGitHubHost: "github.com",
			expectReadOnly:   false,
			expectLockdown:   false,
		},
		{
			name: "custom PORT is used",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Setenv("PORT", "3000")
			},
			tokenMapEnv:      "TEST_CUSTOM_PORT",
			expectPort:       "3000",
			expectGitHubHost: "github.com",
			expectReadOnly:   false,
			expectLockdown:   false,
		},
		{
			name: "default GITHUB_HOST is github.com",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Unsetenv("GITHUB_HOST")
			},
			tokenMapEnv:      "TEST_DEFAULT_HOST",
			expectPort:       "8080",
			expectGitHubHost: "github.com",
			expectReadOnly:   false,
			expectLockdown:   false,
		},
		{
			name: "custom GITHUB_HOST is used",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Setenv("GITHUB_HOST", "github.enterprise.com")
			},
			tokenMapEnv:      "TEST_CUSTOM_HOST",
			expectPort:       "8080",
			expectGitHubHost: "github.enterprise.com",
			expectReadOnly:   false,
			expectLockdown:   false,
		},
		{
			name: "GITHUB_READ_ONLY defaults to false",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Unsetenv("GITHUB_READ_ONLY")
			},
			tokenMapEnv:      "TEST_DEFAULT_READONLY",
			expectPort:       "8080",
			expectGitHubHost: "github.com",
			expectReadOnly:   false,
			expectLockdown:   false,
		},
		{
			name: "GITHUB_READ_ONLY can be set to true",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Setenv("GITHUB_READ_ONLY", "true")
			},
			tokenMapEnv:      "TEST_READONLY_TRUE",
			expectPort:       "8080",
			expectGitHubHost: "github.com",
			expectReadOnly:   true,
			expectLockdown:   false,
		},
		{
			name: "GITHUB_LOCKDOWN_MODE defaults to false",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Unsetenv("GITHUB_LOCKDOWN_MODE")
			},
			tokenMapEnv:      "TEST_DEFAULT_LOCKDOWN",
			expectPort:       "8080",
			expectGitHubHost: "github.com",
			expectReadOnly:   false,
			expectLockdown:   false,
		},
		{
			name: "GITHUB_LOCKDOWN_MODE can be set to true",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Setenv("GITHUB_LOCKDOWN_MODE", "true")
			},
			tokenMapEnv:      "TEST_LOCKDOWN_TRUE",
			expectPort:       "8080",
			expectGitHubHost: "github.com",
			expectReadOnly:   false,
			expectLockdown:   true,
		},
		{
			name: "all custom values are used",
			setupEnv: func(tokenMapEnv string) {
				os.Setenv(tokenMapEnv, `{"token":"github_token"}`)
				os.Setenv("PORT", "9000")
				os.Setenv("GITHUB_HOST", "custom.github.com")
				os.Setenv("GITHUB_READ_ONLY", "true")
				os.Setenv("GITHUB_LOCKDOWN_MODE", "true")
			},
			tokenMapEnv:      "TEST_ALL_CUSTOM",
			expectPort:       "9000",
			expectGitHubHost: "custom.github.com",
			expectReadOnly:   true,
			expectLockdown:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			tt.setupEnv(tt.tokenMapEnv)
			defer os.Unsetenv(tt.tokenMapEnv)
			defer os.Unsetenv("PORT")
			defer os.Unsetenv("GITHUB_HOST")
			defer os.Unsetenv("GITHUB_READ_ONLY")
			defer os.Unsetenv("GITHUB_LOCKDOWN_MODE")
			defer os.Unsetenv("GITHUB_APP_ID")
			defer os.Unsetenv("GITHUB_APP_PRIVATE_KEY_B64")

			// Validate config
			config, err := ValidateConfig("", "", "test", tt.tokenMapEnv)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check default values
			if config.Port != tt.expectPort {
				t.Errorf("Port = %q, want %q", config.Port, tt.expectPort)
			}
			if config.GitHubHost != tt.expectGitHubHost {
				t.Errorf("GitHubHost = %q, want %q", config.GitHubHost, tt.expectGitHubHost)
			}
			if config.ReadOnly != tt.expectReadOnly {
				t.Errorf("ReadOnly = %v, want %v", config.ReadOnly, tt.expectReadOnly)
			}
			if config.LockdownMode != tt.expectLockdown {
				t.Errorf("LockdownMode = %v, want %v", config.LockdownMode, tt.expectLockdown)
			}
		})
	}
}

// Test ValidationError formatting
func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		message  string
		expected string
	}{
		{
			name:     "formats error with field and message",
			field:    "GITHUB_APP_ID",
			message:  "invalid format",
			expected: "configuration error [GITHUB_APP_ID]: invalid format",
		},
		{
			name:     "handles empty field",
			field:    "",
			message:  "some error",
			expected: "configuration error []: some error",
		},
		{
			name:     "handles empty message",
			field:    "SOME_FIELD",
			message:  "",
			expected: "configuration error [SOME_FIELD]: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{
				Field:   tt.field,
				Message: tt.message,
			}

			if err.Error() != tt.expected {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}
