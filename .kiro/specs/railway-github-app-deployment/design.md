# Design Document

## Overview

This design outlines the deployment of a streamlined GitHub MCP server to Railway, configured as a GitHub App for secure authentication. The server will provide HTTP-based MCP protocol access for LangGraph agents to automate GitHub workflows from backlog item processing through pull request creation.

The design focuses on:
- Simplifying the codebase by removing unnecessary components
- Deploying via Railway with Docker
- Using GitHub App authentication for secure, fine-grained access
- Providing a stable HTTP endpoint for LangGraph integration
- Supporting the autonomous development pipeline workflow

## Architecture

### High-Level Architecture

```
┌─────────────────┐
│   LangGraph     │
│     Agent       │
└────────┬────────┘
         │ HTTP/MCP Protocol
         │ (Bearer Token Auth)
         ▼
┌─────────────────────────────────┐
│      Railway Platform           │
│  ┌───────────────────────────┐  │
│  │  GitHub MCP Server        │  │
│  │  (mini-mcp-http)          │  │
│  │                           │  │
│  │  ┌─────────────────────┐  │  │
│  │  │ Token Store         │  │  │
│  │  │ (GitHub App)        │  │  │
│  │  └─────────────────────┘  │  │
│  │                           │  │
│  │  ┌─────────────────────┐  │  │
│  │  │ MCP Server Core     │  │  │
│  │  └─────────────────────┘  │  │
│  └───────────────────────────┘  │
└─────────────┬───────────────────┘
              │ GitHub API
              │ (Installation Tokens)
              ▼
┌─────────────────────────────────┐
│        GitHub Platform          │
│  - Repositories                 │
│  - Issues/PRs                   │
│  - Actions                      │
│  - Git Operations               │
└─────────────────────────────────┘
```

### Deployment Flow

1. **Build Phase**: Railway builds Docker image using optimized Dockerfile
2. **Runtime Phase**: Container starts mini-mcp-http server on Railway-assigned port
3. **Authentication**: GitHub App credentials loaded from Railway environment variables
4. **Connection**: LangGraph connects via HTTPS with MCP bearer token
5. **Token Resolution**: MCP token mapped to GitHub App installation ID
6. **API Access**: Installation token minted and used for GitHub API calls

## Components and Interfaces

### 1. Mini-MCP-HTTP Server (`cmd/mini-mcp-http/main.go`)

**Purpose**: HTTP endpoint that handles MCP protocol over HTTP connections

**Key Responsibilities**:
- Listen on Railway-assigned PORT
- Accept POST requests to `/mcp` endpoint
- Extract and validate Bearer tokens from Authorization header
- Resolve MCP tokens to GitHub tokens via TokenStore
- Stream MCP protocol messages over HTTP
- Handle connection lifecycle

**Configuration**:
```go
type ServerConfig struct {
    ListenAddr  string // Railway PORT env var
    Host        string // GitHub API host (default: github.com)
    Version     string // Build version
    TokenMapEnv string // Env var containing token mapping
}
```

**HTTP Interface**:
- **Endpoint**: `POST /mcp`
- **Headers**: 
  - `Authorization: Bearer <mcp_token>`
  - `Content-Type: application/octet-stream`
- **Response**: Streaming MCP protocol messages
- **Status Codes**:
  - 200: Success (streaming connection)
  - 401: Unauthorized (missing/invalid token)
  - 405: Method not allowed (non-POST)

### 2. Token Store (`internal/ghmcp/tokenstore.go`)

**Purpose**: Map MCP API keys to GitHub App installation tokens

**Implementations**:

**a) EnvTokenStore** (Simple mapping):
```go
type EnvTokenStore struct {
    mapping map[string]string // mcp_token -> github_pat
}
```

**b) InstallationTokenStore** (GitHub App):
```go
type InstallationTokenStore struct {
    appID      int64
    privateKey *rsa.PrivateKey
    mapping    map[string]string // mcp_token -> "installation:<id>"
    apiBase    string
    cache      map[int64]*cachedToken
}
```

**Token Resolution Flow**:
1. Receive MCP token from HTTP request
2. Look up mapping: `mcp_token -> installation:<installation_id>`
3. Check cache for valid installation token
4. If cache miss or expired:
   - Create GitHub App JWT
   - Call GitHub API to mint installation token
   - Cache token with expiry
5. Return installation token for API calls

**Caching Strategy**:
- Cache installation tokens by installation ID
- Refresh when < 1 minute remaining
- Tokens valid for ~1 hour

### 3. MCP Server Core (`internal/ghmcp/server.go`)

**Purpose**: Core MCP protocol implementation and GitHub API integration

**Key Components**:
- `NewMCPServer()`: Factory function to create configured server
- `MCPServerConfig`: Configuration structure
- GitHub client factories (REST, GraphQL, Raw)
- Toolset registration and management
- Middleware for error handling and user agents

**Per-Request GitHub Client Creation**:
```go
getClient := func(ctx context.Context) (*gogithub.Client, error) {
    if t, ok := GitHubTokenFromContext(ctx); ok && t != "" {
        // Create client with per-request token
        c := gogithub.NewClient(nil).WithAuthToken(t)
        c.UserAgent = restClient.UserAgent
        c.BaseURL = restClient.BaseURL
        return c, nil
    }
    return restClient, nil
}
```

### 4. GitHub API Clients

**REST Client** (`google/go-github`):
- Repository operations
- Issue/PR management
- Actions workflows
- File operations

**GraphQL Client** (`shurcooL/githubv4`):
- Complex queries
- Project boards
- Discussions
- Bulk operations

**Raw Client** (`pkg/raw`):
- Direct file content access
- Raw repository content

## Data Models

### Environment Variables

**Required**:
```bash
# Railway automatically provides
PORT=8080

# GitHub App Configuration
GITHUB_APP_ID=123456
GITHUB_APP_PRIVATE_KEY_B64=<base64-encoded-pem>

# Token Mapping (JSON)
GITHUB_MCP_TOKEN_MAP='{"langgraph_token_1":"installation:12345678"}'
```

**Optional**:
```bash
# GitHub Enterprise
GITHUB_HOST=https://github.enterprise.com

# Toolset Configuration
GITHUB_TOOLSETS=repos,issues,pull_requests,actions,git

# Server Configuration
GITHUB_READ_ONLY=false
GITHUB_LOCKDOWN_MODE=false
```

### Token Mapping Format

```json
{
  "mcp_api_key_for_langgraph": "installation:12345678",
  "another_mcp_key": "installation:87654321"
}
```

Where `12345678` is the GitHub App installation ID for the target organization/repository.

### GitHub App Configuration

**Required Permissions**:
- **Repository permissions**:
  - Contents: Read & Write (for file operations, branches)
  - Issues: Read & Write (for backlog item access)
  - Pull requests: Read & Write (for PR creation)
  - Metadata: Read (for repository info)
  - Actions: Read (for workflow status)
  - Workflows: Read & Write (for triggering workflows)

**Webhook Events** (optional for this use case):
- None required for LangGraph polling model

### MCP Protocol Messages

**Tool Invocation Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "create_pull_request",
    "arguments": {
      "owner": "myorg",
      "repo": "myrepo",
      "title": "Implement feature X",
      "head": "feature/x",
      "base": "main",
      "body": "Automated PR from LangGraph"
    }
  }
}
```

**Tool Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Pull request created: #123"
      }
    ]
  }
}
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Service availability on Railway

*For any* Railway deployment, when the service starts, it should successfully bind to the PORT environment variable and respond to HTTP health checks
**Validates: Requirements 1.1, 1.2**

### Property 2: GitHub App authentication success

*For any* valid GitHub App credentials (App ID and private key), the system should successfully create JWT tokens and mint installation tokens
**Validates: Requirements 2.1, 2.2, 2.3**

### Property 3: Docker build cache format compliance

*For any* Docker build operation, all cache mount directives should use the format `--mount=type=cache,id=<cache-id>`
**Validates: Requirements 3.2**

### Property 4: MCP protocol message handling

*For any* valid MCP protocol message received over HTTP, the server should process it according to the MCP specification and return a valid response
**Validates: Requirements 4.2, 4.3**

### Property 5: Token resolution correctness

*For any* MCP token in the token mapping, the system should resolve it to the correct GitHub installation token or return an authentication error
**Validates: Requirements 2.4, 4.5**

### Property 6: Installation token caching

*For any* installation token with remaining validity > 1 minute, subsequent requests should reuse the cached token rather than minting a new one
**Validates: Requirements 2.5**

### Property 7: Environment variable validation

*For any* missing required environment variable, the system should fail startup with a clear error message indicating which variable is required
**Validates: Requirements 6.2**

### Property 8: Backlog-to-PR workflow completeness

*For any* complete development workflow (query issues → create branch → commit files → create PR), all required GitHub API operations should be available through MCP tools
**Validates: Requirements 8.1, 8.2, 8.3, 8.4**

### Property 9: Concurrent request handling

*For any* set of concurrent MCP requests from multiple LangGraph instances, each request should be processed independently with correct token isolation
**Validates: Requirements 8.5**

### Property 10: Error information structure

*For any* error condition (API failure, auth failure, rate limit), the system should return structured error information that includes error type, message, and actionable details
**Validates: Requirements 5.1, 5.2, 5.4, 8.7**

## Error Handling

### Error Categories

**1. Authentication Errors**:
- Missing Authorization header → 401 with message
- Invalid MCP token → 401 with message
- GitHub App JWT creation failure → 500 with details
- Installation token minting failure → 500 with GitHub API error

**2. Configuration Errors**:
- Missing required env vars → Fail at startup with clear message
- Invalid GitHub App credentials → Fail at startup
- Malformed token mapping JSON → Fail at startup

**3. GitHub API Errors**:
- Rate limit exceeded → Return rate limit info to client
- Permission denied → Return permission error with required scopes
- Resource not found → Return 404 equivalent in MCP response
- Network timeout → Retry with exponential backoff (3 attempts)

**4. MCP Protocol Errors**:
- Invalid JSON-RPC → Return JSON-RPC error response
- Unknown tool → Return tool not found error
- Invalid tool arguments → Return validation error

### Error Response Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32000,
    "message": "GitHub API error",
    "data": {
      "type": "rate_limit_exceeded",
      "details": "Rate limit will reset at 2024-12-07T18:00:00Z",
      "retry_after": 3600
    }
  }
}
```

### Logging Strategy

**Structured Logging** (JSON format for Railway):
```json
{
  "timestamp": "2024-12-07T17:00:00Z",
  "level": "error",
  "component": "token_store",
  "message": "Failed to mint installation token",
  "installation_id": 12345678,
  "error": "API returned 403",
  "request_id": "abc123"
}
```

**Log Levels**:
- **DEBUG**: Token resolution, cache hits/misses
- **INFO**: Server startup, connection accepted, tool invocations
- **WARN**: Rate limits approaching, token expiry soon
- **ERROR**: Authentication failures, API errors, unexpected conditions

## Testing Strategy

### Unit Tests

**Token Store Tests**:
- Test EnvTokenStore mapping resolution
- Test InstallationTokenStore JWT creation
- Test installation token caching logic
- Test token expiry handling
- Test private key parsing (PKCS1, PKCS8)

**HTTP Handler Tests**:
- Test Bearer token extraction
- Test Authorization header validation
- Test HTTP method validation (POST only)
- Test response header configuration

**Error Handling Tests**:
- Test missing environment variables
- Test invalid GitHub App credentials
- Test malformed token mapping JSON

### Property-Based Tests

**Property Test 1: Token resolution consistency**
- **Feature: railway-github-app-deployment, Property 5: Token resolution correctness**
- Generate random MCP tokens and mappings
- Verify consistent resolution for same token
- Verify different tokens resolve to different installations

**Property Test 2: Installation token caching**
- **Feature: railway-github-app-deployment, Property 6: Installation token caching**
- Generate random installation IDs
- Mint tokens and verify caching
- Verify cache invalidation on expiry

**Property Test 3: Concurrent request isolation**
- **Feature: railway-github-app-deployment, Property 9: Concurrent request handling**
- Generate concurrent requests with different tokens
- Verify each request uses correct GitHub token
- Verify no token leakage between requests

**Property Test 4: Error structure completeness**
- **Feature: railway-github-app-deployment, Property 10: Error information structure**
- Generate various error conditions
- Verify all errors include type, message, and actionable details
- Verify error responses are valid JSON-RPC

### Integration Tests

**Railway Deployment Test**:
- Deploy to Railway staging environment
- Verify service starts and binds to PORT
- Verify health check endpoint responds
- Verify HTTPS access via Railway URL

**GitHub App Authentication Test**:
- Configure test GitHub App
- Verify JWT creation
- Verify installation token minting
- Verify API calls with installation token

**End-to-End Workflow Test**:
- Simulate LangGraph connection
- Execute complete workflow: list issues → create branch → commit → create PR
- Verify all operations succeed
- Verify PR created in test repository

### Testing Framework

- **Unit Tests**: Go standard `testing` package
- **Property Tests**: `gopter` (Go property testing library)
- **HTTP Tests**: `httptest` package
- **Integration Tests**: Real Railway deployment + test GitHub App

**Property Test Configuration**:
- Minimum 100 iterations per property test
- Use seed for reproducibility
- Generate realistic test data (valid tokens, installation IDs)

## Deployment Configuration

### Dockerfile Optimization

```dockerfile
FROM golang:1.25.4-alpine AS build
ARG VERSION="dev"

WORKDIR /build

# Install git for version info
RUN --mount=type=cache,id=apk-cache,target=/var/cache/apk \
    apk add git

# Build mini-mcp-http only
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /bin/mini-mcp-http \
    cmd/mini-mcp-http/main.go

# Runtime stage
FROM gcr.io/distroless/base-debian12

WORKDIR /server
COPY --from=build /bin/mini-mcp-http .

ENTRYPOINT ["/server/mini-mcp-http"]
CMD ["-listen", ":${PORT}"]
```

### Railway Configuration (`railway.json`)

```json
{
  "$schema": "https://railway.app/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE",
    "dockerfilePath": "Dockerfile"
  },
  "deploy": {
    "startCommand": "/server/mini-mcp-http -listen :$PORT",
    "healthcheckPath": "/health",
    "healthcheckTimeout": 30,
    "restartPolicyType": "ON_FAILURE",
    "restartPolicyMaxRetries": 3
  }
}
```

### Environment Variable Configuration

**Railway Dashboard Setup**:
1. Create new service from GitHub repository
2. Configure environment variables:
   - `GITHUB_APP_ID`: From GitHub App settings
   - `GITHUB_APP_PRIVATE_KEY_B64`: Base64-encoded private key
   - `GITHUB_MCP_TOKEN_MAP`: JSON mapping of MCP tokens
3. Railway automatically provides `PORT`
4. Deploy triggers on git push to main branch

## Codebase Cleanup Plan

### Files to Remove

**Command-line tools**:
- `cmd/github-mcp-server/` (stdio version)
- `cmd/mcpcurl/` (testing utility)
- `cmd/mini-mcp/` (stdio mini version)

**Documentation for other deployment methods**:
- `docs/installation-guides/` (all IDE-specific guides)
- `docs/remote-server.md` (GitHub-hosted version)
- `docs/server-configuration.md` (stdio configuration)

**Unused packages**:
- `pkg/log/` (if only used for stdio)
- `internal/profiler/` (development tool)
- `internal/toolsnaps/` (testing utility)

**Build/release tooling**:
- `.goreleaser.yaml` (multi-platform releases)
- `script/` directory (build scripts)

**Third-party licenses** (regenerate for minimal deps):
- `third-party-licenses.*.md` (all platforms)

### Files to Keep

**Core server**:
- `cmd/mini-mcp-http/main.go`
- `internal/ghmcp/server.go`
- `internal/ghmcp/tokenstore.go`

**GitHub integration**:
- `pkg/github/` (all toolsets and API wrappers)
- `pkg/errors/` (error handling)
- `pkg/raw/` (raw content access)
- `pkg/translations/` (if used by toolsets)
- `pkg/utils/` (utility functions)
- `pkg/lockdown/` (if using lockdown mode)

**Configuration**:
- `Dockerfile` (modified for mini-mcp-http)
- `go.mod`, `go.sum`
- `.gitignore`
- `.dockerignore`

**Documentation** (updated):
- `README.md` (rewritten for Railway deployment)
- `docs/error-handling.md`
- `docs/testing.md`
- `LICENSE`, `SECURITY.md`, `CODE_OF_CONDUCT.md`

### Dependency Cleanup

**Review and remove unused dependencies**:
1. Run `go mod tidy` after removing code
2. Check for stdio-specific dependencies
3. Verify all remaining dependencies are used by mini-mcp-http

**Expected core dependencies**:
- `github.com/modelcontextprotocol/go-sdk/mcp`
- `github.com/google/go-github/v79/github`
- `github.com/shurcooL/githubv4`
- Standard library packages

## LangGraph Integration

### Connection Configuration

**LangGraph MCP Client Setup**:
```python
from langgraph.mcp import MCPClient

client = MCPClient(
    url="https://your-app.railway.app/mcp",
    headers={
        "Authorization": f"Bearer {mcp_token}"
    }
)
```

### Workflow Example

**Autonomous Development Flow**:
```python
# 1. Query backlog
issues = await client.call_tool("list_issues", {
    "owner": "myorg",
    "repo": "myrepo",
    "state": "open",
    "labels": "backlog"
})

# 2. Create feature branch
branch = await client.call_tool("create_branch", {
    "owner": "myorg",
    "repo": "myrepo",
    "branch": "feature/new-feature",
    "from_branch": "main"
})

# 3. Commit changes
commit = await client.call_tool("create_or_update_file", {
    "owner": "myorg",
    "repo": "myrepo",
    "path": "src/feature.py",
    "content": generated_code,
    "message": "Implement new feature",
    "branch": "feature/new-feature"
})

# 4. Create pull request
pr = await client.call_tool("create_pull_request", {
    "owner": "myorg",
    "repo": "myrepo",
    "title": "Implement new feature",
    "head": "feature/new-feature",
    "base": "main",
    "body": "Automated PR from LangGraph agent"
})
```

### Error Handling in LangGraph

```python
try:
    result = await client.call_tool("create_pull_request", params)
except MCPError as e:
    if e.data.get("type") == "rate_limit_exceeded":
        retry_after = e.data.get("retry_after", 3600)
        await asyncio.sleep(retry_after)
        # Retry operation
    elif e.data.get("type") == "permission_denied":
        # Log and skip this operation
        logger.error(f"Permission denied: {e.message}")
    else:
        # Unexpected error, escalate
        raise
```

## Security Considerations

### Token Security

1. **GitHub App Private Key**:
   - Store as base64-encoded in Railway environment variables
   - Never commit to repository
   - Rotate periodically (every 90 days)

2. **MCP API Tokens**:
   - Generate strong random tokens (32+ characters)
   - Store mapping in Railway environment variables
   - Use different tokens for different LangGraph instances

3. **Installation Tokens**:
   - Short-lived (1 hour)
   - Cached securely in memory
   - Never logged or persisted to disk

### Network Security

1. **HTTPS Only**:
   - Railway provides automatic HTTPS
   - No HTTP fallback

2. **Authentication Required**:
   - All requests must include valid Bearer token
   - No anonymous access

3. **Rate Limiting**:
   - Respect GitHub API rate limits
   - Return rate limit info to clients
   - Implement client-side backoff

### GitHub App Permissions

**Principle of Least Privilege**:
- Only request permissions needed for autonomous workflow
- Use repository-level installation (not organization-wide if possible)
- Review and audit permissions regularly

**Recommended Permissions**:
- Contents: Read & Write (minimum for file operations)
- Issues: Read & Write (for backlog access)
- Pull Requests: Read & Write (for PR creation)
- Metadata: Read (always required)
- Actions: Read (for workflow status)

## Monitoring and Observability

### Railway Logging

**Structured Logs**:
- All logs output as JSON for Railway log aggregation
- Include request IDs for tracing
- Log levels: DEBUG, INFO, WARN, ERROR

**Key Metrics to Log**:
- Request count and latency
- Token resolution success/failure
- GitHub API call count and latency
- Installation token cache hit rate
- Error rates by type

### Health Checks

**Health Endpoint** (`/health`):
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 3600,
  "github_api_reachable": true
}
```

**Readiness Checks**:
- GitHub App credentials loaded
- Token mapping parsed successfully
- HTTP server listening

### Alerting

**Critical Alerts**:
- Service down (Railway health check fails)
- GitHub App authentication failures
- High error rate (> 5% of requests)

**Warning Alerts**:
- Approaching GitHub API rate limits
- High latency (> 5s per request)
- Installation token cache miss rate > 10%

## Performance Considerations

### Optimization Strategies

1. **Installation Token Caching**:
   - Reduces GitHub API calls by ~99%
   - Cache hit rate should be > 90%

2. **Connection Pooling**:
   - HTTP client reuses connections to GitHub API
   - Default pool size: 100 connections

3. **Concurrent Request Handling**:
   - Go's goroutines handle concurrent connections efficiently
   - No explicit connection limit (Railway handles this)

4. **Docker Image Size**:
   - Multi-stage build reduces image size
   - Distroless base image (~20MB)
   - Total image size target: < 50MB

### Expected Performance

**Latency**:
- Token resolution: < 1ms (cache hit)
- Token resolution: < 500ms (cache miss, mint new token)
- GitHub API calls: 100-500ms (depends on operation)
- End-to-end request: 200-1000ms

**Throughput**:
- Concurrent connections: 100+ (limited by Railway plan)
- Requests per second: 50+ (limited by GitHub API rate limits)

**Resource Usage**:
- Memory: < 100MB under normal load
- CPU: < 0.1 vCPU under normal load
- Network: Depends on GitHub API usage

## Future Enhancements

### Potential Improvements

1. **Webhook Support**:
   - Add webhook endpoint for GitHub events
   - Enable push-based notifications to LangGraph
   - Reduce polling overhead

2. **Multi-Repository Support**:
   - Support multiple GitHub App installations
   - Route requests based on repository
   - Centralized MCP server for multiple projects

3. **Metrics Dashboard**:
   - Expose Prometheus metrics endpoint
   - Visualize request rates, latency, errors
   - Monitor GitHub API quota usage

4. **Request Queuing**:
   - Queue requests during rate limit periods
   - Automatic retry with exponential backoff
   - Priority queue for critical operations

5. **Audit Logging**:
   - Log all GitHub operations for compliance
   - Store audit logs in external system
   - Support audit log queries

### Scalability Considerations

**Horizontal Scaling**:
- Railway supports multiple instances
- Stateless design enables easy scaling
- Token cache is per-instance (acceptable trade-off)

**Vertical Scaling**:
- Increase Railway plan for more resources
- Monitor memory usage as cache grows
- Consider external cache (Redis) for large deployments
