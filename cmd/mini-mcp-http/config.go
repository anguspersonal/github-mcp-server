package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the validated configuration for the mini-mcp-http server
type Config struct {
	ListenAddr  string
	Host        string
	Version     string
	TokenMapEnv string
	Port        string
	
	// GitHub App configuration
	GitHubAppID         int64
	GitHubAppPrivateKey []byte
	TokenMapping        map[string]string
	
	// Optional configuration with defaults
	GitHubHost    string
	GitHubToolsets string
	ReadOnly      bool
	LockdownMode  bool
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("configuration error [%s]: %s", e.Field, e.Message)
}

// ValidateConfig validates all required environment variables and returns a Config
func ValidateConfig(listenAddr, host, version, tokenMapEnv string) (*Config, error) {
	cfg := &Config{
		ListenAddr:  listenAddr,
		Host:        host,
		Version:     version,
		TokenMapEnv: tokenMapEnv,
	}
	
	// Apply defaults for optional configuration
	cfg.Port = os.Getenv("PORT")
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	
	cfg.GitHubHost = os.Getenv("GITHUB_HOST")
	if cfg.GitHubHost == "" {
		cfg.GitHubHost = "github.com"
	}
	
	cfg.GitHubToolsets = os.Getenv("GITHUB_TOOLSETS")
	// Default toolsets will be handled by the server if empty
	
	cfg.ReadOnly = os.Getenv("GITHUB_READ_ONLY") == "true"
	cfg.LockdownMode = os.Getenv("GITHUB_LOCKDOWN_MODE") == "true"
	
	// Validate required environment variables
	if err := cfg.validateRequiredEnvVars(); err != nil {
		return nil, err
	}
	
	return cfg, nil
}

// validateRequiredEnvVars checks for required environment variables
func (c *Config) validateRequiredEnvVars() error {
	// Validate GITHUB_MCP_TOKEN_MAP
	tokenMapJSON := os.Getenv(c.TokenMapEnv)
	if tokenMapJSON == "" {
		return &ValidationError{
			Field:   c.TokenMapEnv,
			Message: fmt.Sprintf("required environment variable %s is not set. This should contain a JSON mapping of MCP tokens to GitHub tokens (e.g., {\"mcp_token\":\"installation:12345\"})", c.TokenMapEnv),
		}
	}
	
	// Validate token mapping JSON format
	var tokenMap map[string]string
	if err := json.Unmarshal([]byte(tokenMapJSON), &tokenMap); err != nil {
		return &ValidationError{
			Field:   c.TokenMapEnv,
			Message: fmt.Sprintf("invalid JSON format in %s: %v. Expected format: {\"mcp_token\":\"installation:12345\"}", c.TokenMapEnv, err),
		}
	}
	
	if len(tokenMap) == 0 {
		return &ValidationError{
			Field:   c.TokenMapEnv,
			Message: fmt.Sprintf("%s contains empty mapping. At least one MCP token mapping is required", c.TokenMapEnv),
		}
	}
	
	c.TokenMapping = tokenMap
	
	// Check for GitHub App credentials
	appIDStr := os.Getenv("GITHUB_APP_ID")
	pkb64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_B64")
	
	// Both must be present or both must be absent
	if (appIDStr == "") != (pkb64 == "") {
		if appIDStr == "" {
			return &ValidationError{
				Field:   "GITHUB_APP_ID",
				Message: "GITHUB_APP_ID is required when GITHUB_APP_PRIVATE_KEY_B64 is set",
			}
		}
		return &ValidationError{
			Field:   "GITHUB_APP_PRIVATE_KEY_B64",
			Message: "GITHUB_APP_PRIVATE_KEY_B64 is required when GITHUB_APP_ID is set",
		}
	}
	
	// If GitHub App credentials are provided, validate them
	if appIDStr != "" && pkb64 != "" {
		// Validate GITHUB_APP_ID format
		appID, err := strconv.ParseInt(appIDStr, 10, 64)
		if err != nil {
			return &ValidationError{
				Field:   "GITHUB_APP_ID",
				Message: fmt.Sprintf("invalid GITHUB_APP_ID format: %v. Expected a numeric value (e.g., 123456)", err),
			}
		}
		
		if appID <= 0 {
			return &ValidationError{
				Field:   "GITHUB_APP_ID",
				Message: "GITHUB_APP_ID must be a positive integer",
			}
		}
		
		c.GitHubAppID = appID
		
		// Validate GITHUB_APP_PRIVATE_KEY_B64 format
		pkBytes, err := base64.StdEncoding.DecodeString(pkb64)
		if err != nil {
			// Try treating it as raw PEM
			pkBytes = []byte(pkb64)
		}
		
		// Basic validation that it looks like a PEM key
		pkStr := string(pkBytes)
		if !strings.Contains(pkStr, "BEGIN") || !strings.Contains(pkStr, "PRIVATE KEY") {
			return &ValidationError{
				Field:   "GITHUB_APP_PRIVATE_KEY_B64",
				Message: "GITHUB_APP_PRIVATE_KEY_B64 does not appear to contain a valid PEM-encoded private key. Expected format: base64-encoded PEM key with BEGIN/END markers",
			}
		}
		
		c.GitHubAppPrivateKey = pkBytes
		
		// Validate that token mappings use installation: format when using GitHub App
		for mcpToken, mapping := range c.TokenMapping {
			if !strings.HasPrefix(mapping, "installation:") {
				return &ValidationError{
					Field:   c.TokenMapEnv,
					Message: fmt.Sprintf("when using GitHub App authentication, all token mappings must use 'installation:<id>' format. Invalid mapping for token '%s': '%s'", mcpToken, mapping),
				}
			}
			
			// Validate installation ID format
			instIDStr := strings.TrimPrefix(mapping, "installation:")
			instID, err := strconv.ParseInt(instIDStr, 10, 64)
			if err != nil || instID <= 0 {
				return &ValidationError{
					Field:   c.TokenMapEnv,
					Message: fmt.Sprintf("invalid installation ID in mapping '%s': '%s'. Expected format: 'installation:<numeric_id>'", mcpToken, mapping),
				}
			}
		}
	}
	
	return nil
}
