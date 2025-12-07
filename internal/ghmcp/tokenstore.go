package ghmcp

import (
    "bytes"
    "context"
    "crypto"
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/json"
    "encoding/pem"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"
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

// Mapping returns a shallow copy of the internal mapping for callers that need
// to inspect or re-use it to build other stores.
func (e *EnvTokenStore) Mapping() map[string]string {
    out := make(map[string]string, len(e.mapping))
    for k, v := range e.mapping {
        out[k] = v
    }
    return out
}

// InstallationTokenStore resolves MCP API keys to GitHub App installation tokens.
// The mapping should map MCP_API_KEY -> "installation:<installation_id>".
type InstallationTokenStore struct {
    appID      int64
    privateKey *rsa.PrivateKey
    mapping    map[string]string // MCP token -> mapping value (installation:<id>)
    apiBase    string

    mu    sync.Mutex
    cache map[int64]*cachedToken // installationID -> token
}

type cachedToken struct {
    token  string
    expiry time.Time
}

// NewInstallationTokenStore creates a new InstallationTokenStore.
// apiBase should be the base API URL, e.g. "https://api.github.com/" (include scheme).
func NewInstallationTokenStore(appID int64, privateKeyPEM []byte, mapping map[string]string, apiBase string) (*InstallationTokenStore, error) {
    pk, err := parseRSAPrivateKeyFromPEM(privateKeyPEM)
    if err != nil {
        return nil, fmt.Errorf("failed to parse private key: %w", err)
    }
    if !strings.HasSuffix(apiBase, "/") {
        apiBase = apiBase + "/"
    }
    return &InstallationTokenStore{
        appID:      appID,
        privateKey: pk,
        mapping:    mapping,
        apiBase:    apiBase,
        cache:      map[int64]*cachedToken{},
    }, nil
}

func parseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
    var block *pem.Block
    for {
        block, pemBytes = pem.Decode(pemBytes)
        if block == nil {
            break
        }
        if block.Type == "RSA PRIVATE KEY" || strings.HasSuffix(block.Type, "PRIVATE KEY") {
            break
        }
    }
    if block == nil {
        return nil, errors.New("no PEM block found")
    }
    // Try PKCS1 first
    if pk, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
        return pk, nil
    }
    // Try PKCS8
    parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("failed to parse private key: %w", err)
    }
    rsaKey, ok := parsed.(*rsa.PrivateKey)
    if !ok {
        return nil, errors.New("private key is not RSA")
    }
    return rsaKey, nil
}

// GetGitHubToken implements TokenStore. It supports mapping values of the form
// "installation:<id>". For installation mappings, it will mint or reuse a
// cached installation access token.
func (s *InstallationTokenStore) GetGitHubToken(mcpToken string) (string, bool) {
    v, ok := s.mapping[mcpToken]
    if !ok {
        return "", false
    }
    if strings.HasPrefix(v, "installation:") {
        idStr := strings.TrimPrefix(v, "installation:")
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            return "", false
        }
        // Check cache
        s.mu.Lock()
        if ct, ok := s.cache[id]; ok && time.Until(ct.expiry) > time.Minute {
            token := ct.token
            s.mu.Unlock()
            return token, true
        }
        s.mu.Unlock()

        // Need to mint a new installation token
        tok, exp, err := s.createInstallationToken(id)
        if err != nil {
            return "", false
        }
        s.mu.Lock()
        s.cache[id] = &cachedToken{token: tok, expiry: exp}
        s.mu.Unlock()
        return tok, true
    }

    // If mapping isn't installation:, treat it as a direct token (compat)
    return v, true
}

func (s *InstallationTokenStore) createInstallationToken(installationID int64) (string, time.Time, error) {
    jwt, err := s.createAppJWT()
    if err != nil {
        return "", time.Time{}, fmt.Errorf("failed to create app jwt: %w", err)
    }
    url := fmt.Sprintf("%sapp/installations/%d/access_tokens", strings.TrimRight(s.apiBase, "/"), installationID)
    req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader([]byte("{}")))
    if err != nil {
        return "", time.Time{}, err
    }
    req.Header.Set("Authorization", "Bearer "+jwt)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", time.Time{}, err
    }
    defer func() { _ = resp.Body.Close() }()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        body, _ := io.ReadAll(resp.Body)
        return "", time.Time{}, fmt.Errorf("unexpected status creating installation token: %d: %s", resp.StatusCode, string(body))
    }
    var out struct {
        Token     string `json:"token"`
        ExpiresAt string `json:"expires_at"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        return "", time.Time{}, fmt.Errorf("failed to decode installation token response: %w", err)
    }
    exp, err := time.Parse(time.RFC3339, out.ExpiresAt)
    if err != nil {
        // try alternative parse
        exp = time.Now().Add(1 * time.Hour)
    }
    // subtract a small buffer
    expiry := exp.Add(-1 * time.Minute)
    return out.Token, expiry, nil
}

func (s *InstallationTokenStore) createAppJWT() (string, error) {
    now := time.Now()
    // JWT header
    header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
    // Payload
    // iat, exp (max 10 minutes recommended). iss = appID
    payloadMap := map[string]interface{}{
        "iat": now.Unix(),
        "exp": now.Add(9 * time.Minute).Unix(),
        "iss": s.appID,
    }
    payloadBytes, _ := json.Marshal(payloadMap)
    payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
    unsigned := header + "." + payload

    h := sha256.New()
    h.Write([]byte(unsigned))
    digest := h.Sum(nil)

    sig, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest)
    if err != nil {
        return "", fmt.Errorf("failed to sign jwt: %w", err)
    }
    signed := unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
    return signed, nil
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
