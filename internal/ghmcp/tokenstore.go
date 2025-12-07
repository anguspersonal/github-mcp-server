package ghmcp

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
)

// TokenStore maps an MCP API key to a GitHub token (PAT or installation token).
// Implementations should load/store this mapping from a secure location.
type TokenStore interface {
    // GetGitHubToken returns the GitHub token for the given mcpToken.
    GetGitHubToken(mcpToken string) (string, bool)
}

// EnvTokenStore loads a JSON mapping from an environment variable.
// The environment variable should contain JSON object: {"mcp_token":"github_pat", ...}
type EnvTokenStore struct {
    mapping map[string]string
}

// NewEnvTokenStoreFromEnv loads mapping from the given env var name.
func NewEnvTokenStoreFromEnv(envVar string) (*EnvTokenStore, error) {
    v := os.Getenv(envVar)
    if v == "" {
        return &EnvTokenStore{mapping: map[string]string{}}, nil
    }
    var m map[string]string
    if err := json.Unmarshal([]byte(v), &m); err != nil {
        return nil, fmt.Errorf("failed to parse %s JSON: %w", envVar, err)
    }
    return &EnvTokenStore{mapping: m}, nil
}

func (e *EnvTokenStore) GetGitHubToken(mcpToken string) (string, bool) {
    t, ok := e.mapping[mcpToken]
    return t, ok
}

// Context helpers for carrying a resolved GitHub token for the current request.
type ctxKeyType string

const ctxKeyGitHubToken ctxKeyType = "ghmcp.githubToken"

// ContextWithGitHubToken returns a new context with the provided GitHub token.
func ContextWithGitHubToken(ctx context.Context, ghToken string) context.Context {
    return context.WithValue(ctx, ctxKeyGitHubToken, ghToken)
}

// GitHubTokenFromContext extracts the GitHub token from context, if present.
func GitHubTokenFromContext(ctx context.Context) (string, bool) {
    v := ctx.Value(ctxKeyGitHubToken)
    if v == nil {
        return "", false
    }
    s, ok := v.(string)
    return s, ok
}
