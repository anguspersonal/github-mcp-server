# Troubleshooting Guide

This guide covers common issues you may encounter when deploying and using the GitHub MCP server on Railway.

## Table of Contents

- [Railway Deployment Issues](#railway-deployment-issues)
- [GitHub App Configuration Issues](#github-app-configuration-issues)
- [Token Resolution Issues](#token-resolution-issues)
- [Authentication Errors](#authentication-errors)
- [GitHub API Errors](#github-api-errors)
- [Connection Issues](#connection-issues)
- [Performance Issues](#performance-issues)

---

## Railway Deployment Issues

### Service Fails to Start

**Symptom**: Railway shows the service as "crashed" or "failed" immediately after deployment.

**Common Causes**:

1. **Missing required environment variables**
   ```
   Error: GITHUB_APP_ID environment variable is required
   ```
   
   **Solution**: Ensure all required environment variables are set in Railway:
   - `GITHUB_APP_ID`
   - `GITHUB_APP_PRIVATE_KEY_B64`
   - `GITHUB_MCP_TOKEN_MAP`
   
   Go to Railway dashboard → Your service → Variables tab → Add missing variables

2. **Invalid private key format**
   ```
   Error: failed to parse private key: invalid key format
   ```
   
   **Solution**: Ensure your private key is base64-encoded:
   ```bash
   # Encode your private key
   cat github-app-private-key.pem | base64 -w 0
   ```
   
   Copy the output and set it as `GITHUB_APP_PRIVATE_KEY_B64` in Railway.

3. **Malformed token mapping JSON**
   ```
   Error: failed to parse token mapping: invalid character '}' looking for beginning of object key string
   ```
   
   **Solution**: Validate your JSON format:
   ```json
   {"your_mcp_token":"installation:12345678"}
   ```
   
   Use a JSON validator to check syntax before setting the environment variable.

### Docker Build Fails

**Symptom**: Railway build logs show Docker build errors.

**Common Causes**:

1. **Cache mount ID prefix error**
   ```
   Error: Cache mount ID is not prefixed with cache key
   ```
   
   **Solution**: Railway requires cache mount IDs to be prefixed with `s/<SERVICE_ID>-`. The Dockerfile uses a build argument, so you need to set it as an environment variable:
   
   a. Get your Railway Service ID:
      - Open Railway dashboard → Your service
      - Press `Cmd + K` (macOS) or `Ctrl + K` (Windows/Linux) to open command palette
      - Look for "Copy Service ID" or "Copy Workspace ID" (Railway may show either)
      - Copy the ID
   
   b. Set as a SERVICE variable (not shared):
      - Go to Railway dashboard → Your service → **Variables** tab
      - **Important**: Use the service's Variables tab, NOT Project Settings → Shared Variables
      - Click "New Variable"
      - Name: `RAILWAY_SERVICE_ID`
      - Value: Your Service ID (paste the one you copied)
      - Click "Add"
      - Railway will automatically redeploy with the new variable
   
   **Why Service Variable?**: 
   - **Service Variables** (in service → Variables tab): Available during Docker builds and runtime
   - **Shared Variables** (in Project Settings): Primarily for runtime sharing, may not be available during builds
   - **Railway-Provided Variables**: Automatically injected by Railway (like `RAILWAY_PUBLIC_DOMAIN`)
   
   **How it works**: Railway makes service environment variables available during Docker builds. The Dockerfile uses `ARG RAILWAY_SERVICE_ID` which Railway passes as a build argument.
   
   **Note**: If you see "Copy Workspace ID" instead of "Copy Service ID", that's fine - for single-service projects, the Workspace ID is the same as the Service ID.

2. **Cache mount format errors**
   ```
   Error: invalid mount config for type "cache": invalid field 'target' must not be empty
   ```
   
   **Solution**: Verify Dockerfile uses correct cache mount format with target:
   ```dockerfile
   RUN --mount=type=cache,id=s/YOUR_SERVICE_ID-go-mod,target=/go/pkg/mod \
       go build ...
   ```

2. **Go module download failures**
   ```
   Error: go: github.com/some/package@v1.0.0: Get "https://proxy.golang.org/...": dial tcp: lookup proxy.golang.org: no such host
   ```
   
   **Solution**: This is usually a temporary network issue. Trigger a rebuild in Railway.

### Health Check Failures

**Symptom**: Railway shows "Unhealthy" status or restarts the service repeatedly.

**Common Causes**:

1. **Health check endpoint not responding**
   
   **Solution**: Verify the health check path in `railway.json`:
   ```json
   {
     "deploy": {
       "healthcheckPath": "/health",
       "healthcheckTimeout": 30
     }
   }
   ```
   
   Test the endpoint manually:
   ```bash
   curl https://your-app.railway.app/health
   ```

2. **Service taking too long to start**
   
   **Solution**: Increase the health check timeout in `railway.json`:
   ```json
   {
     "deploy": {
       "healthcheckTimeout": 60
     }
   }
   ```

### Port Binding Issues

**Symptom**: Service starts but is not accessible via Railway URL.

**Solution**: Ensure the server reads the `PORT` environment variable:
```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

Railway automatically provides the `PORT` variable - do not hardcode the port.

---

## GitHub App Configuration Issues

### Permission Denied Errors

**Symptom**: API calls fail with 403 Forbidden errors.

**Common Causes**:

1. **Insufficient GitHub App permissions**
   ```json
   {
     "error": {
       "message": "Resource not accessible by integration",
       "type": "permission_denied",
       "data": {
         "required_permissions": ["contents:write"]
       }
     }
   }
   ```
   
   **Solution**: Update GitHub App permissions:
   - Go to GitHub → Settings → Developer settings → GitHub Apps → Your App
   - Click "Permissions & events"
   - Update required permissions:
     - **Contents**: Read & Write (for file operations)
     - **Issues**: Read & Write (for issue access)
     - **Pull requests**: Read & Write (for PR creation)
     - **Metadata**: Read (always required)
     - **Actions**: Read (for workflow status)
   - Save changes
   - **Important**: Users must accept the new permissions in their installations

2. **GitHub App not installed on repository**
   ```json
   {
     "error": {
       "message": "Installation not found for repository",
       "type": "not_found"
     }
   }
   ```
   
   **Solution**: Install the GitHub App on the target repository:
   - Go to GitHub → Settings → Integrations → Applications
   - Find your GitHub App
   - Click "Configure"
   - Select repositories to grant access

3. **Installation suspended or revoked**
   
   **Solution**: Check installation status:
   - Go to repository → Settings → Integrations
   - Verify the app is active and not suspended
   - Reinstall if necessary

### Invalid GitHub App Credentials

**Symptom**: Server fails to start or authentication fails immediately.

**Common Causes**:

1. **Wrong App ID**
   ```
   Error: failed to create JWT: invalid app ID
   ```
   
   **Solution**: Verify your App ID:
   - Go to GitHub → Settings → Developer settings → GitHub Apps → Your App
   - Copy the "App ID" (numeric value)
   - Update `GITHUB_APP_ID` in Railway

2. **Expired or regenerated private key**
   ```
   Error: failed to mint installation token: 401 Unauthorized
   ```
   
   **Solution**: Generate a new private key:
   - Go to GitHub → Settings → Developer settings → GitHub Apps → Your App
   - Scroll to "Private keys"
   - Click "Generate a private key"
   - Download the `.pem` file
   - Base64 encode it and update `GITHUB_APP_PRIVATE_KEY_B64` in Railway

3. **Private key format issues (PKCS1 vs PKCS8)**
   ```
   Error: x509: failed to parse private key
   ```
   
   **Solution**: The server supports both PKCS1 and PKCS8 formats. If you encounter issues, convert to PKCS8:
   ```bash
   openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt \
     -in github-app-private-key.pem -out github-app-private-key-pkcs8.pem
   ```

---

## Token Resolution Issues

### MCP Token Not Found

**Symptom**: LangGraph connection fails with authentication error.

**Error**:
```json
{
  "error": {
    "code": 401,
    "message": "Invalid or missing MCP token"
  }
}
```

**Solution**: Verify token mapping configuration:

1. Check `GITHUB_MCP_TOKEN_MAP` format:
   ```json
   {
     "your_mcp_token_here": "installation:12345678"
   }
   ```

2. Ensure the token you're using in LangGraph matches exactly:
   ```python
   client = MCPClient(
       url="https://your-app.railway.app/mcp",
       headers={
           "Authorization": "Bearer your_mcp_token_here"
       }
   )
   ```

3. Check for whitespace or special characters in the token

### Installation ID Not Found

**Symptom**: Token resolves but GitHub API calls fail.

**Error**:
```json
{
  "error": {
    "message": "Installation not found: 12345678",
    "type": "not_found"
  }
}
```

**Solution**: Verify the installation ID:

1. Get the correct installation ID:
   ```bash
   # List installations for your GitHub App
   curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     https://api.github.com/app/installations
   ```

2. Update the token mapping with the correct installation ID:
   ```json
   {
     "your_mcp_token": "installation:CORRECT_ID_HERE"
   }
   ```

3. Redeploy the Railway service after updating environment variables

### Token Caching Issues

**Symptom**: API calls work initially but fail after ~1 hour.

**Error**:
```json
{
  "error": {
    "message": "401 Bad credentials",
    "type": "authentication_error"
  }
}
```

**Solution**: This usually indicates a token caching bug. Check server logs:

1. Look for token refresh messages in Railway logs:
   ```
   level=debug component=token_store message="Token expired, minting new token" installation_id=12345678
   ```

2. If you don't see refresh messages, the cache may not be expiring correctly. Restart the service:
   - Railway dashboard → Your service → Settings → Restart

3. If the issue persists, check for clock skew issues (rare on Railway)

---

## Authentication Errors

### Missing Authorization Header

**Symptom**: All requests fail with 401.

**Error**:
```json
{
  "error": {
    "code": 401,
    "message": "Missing Authorization header"
  }
}
```

**Solution**: Ensure your MCP client includes the Authorization header:

```python
# Correct
client = MCPClient(
    url="https://your-app.railway.app/mcp",
    headers={
        "Authorization": "Bearer your_mcp_token"
    }
)

# Incorrect - missing Authorization header
client = MCPClient(
    url="https://your-app.railway.app/mcp"
)
```

### Invalid Bearer Token Format

**Symptom**: Authentication fails despite providing a token.

**Error**:
```json
{
  "error": {
    "code": 401,
    "message": "Invalid Authorization header format"
  }
}
```

**Solution**: Verify the header format:

- **Correct**: `Authorization: Bearer your_token_here`
- **Incorrect**: `Authorization: your_token_here` (missing "Bearer")
- **Incorrect**: `Authorization: bearer your_token_here` (lowercase "bearer")

### JWT Creation Failures

**Symptom**: Server logs show JWT creation errors.

**Error**:
```
Error: failed to create JWT: crypto/rsa: invalid key
```

**Solution**:

1. Verify private key is valid:
   ```bash
   openssl rsa -in github-app-private-key.pem -check
   ```

2. Ensure base64 encoding is correct:
   ```bash
   # Encode without line breaks
   cat github-app-private-key.pem | base64 -w 0
   ```

3. Check for extra whitespace or newlines in the environment variable

---

## GitHub API Errors

### Rate Limit Exceeded

**Symptom**: API calls fail with rate limit errors.

**Error**:
```json
{
  "error": {
    "message": "API rate limit exceeded",
    "type": "rate_limit_exceeded",
    "data": {
      "retry_after": 3600,
      "reset_at": "2024-12-08T18:00:00Z"
    }
  }
}
```

**Solution**:

1. **Immediate**: Wait for the rate limit to reset (check `reset_at` timestamp)

2. **Short-term**: Implement retry logic in your LangGraph code:
   ```python
   try:
       result = await client.call_tool("create_pull_request", params)
   except MCPError as e:
       if e.data.get("type") == "rate_limit_exceeded":
           retry_after = e.data.get("retry_after", 3600)
           await asyncio.sleep(retry_after)
           # Retry operation
   ```

3. **Long-term**: 
   - Optimize your workflow to make fewer API calls
   - Use GraphQL for bulk operations
   - Consider using multiple GitHub App installations for different repositories

### Resource Not Found (404)

**Symptom**: API calls fail with "not found" errors.

**Error**:
```json
{
  "error": {
    "message": "Repository not found: owner/repo",
    "type": "not_found"
  }
}
```

**Solution**:

1. Verify repository name and owner are correct (case-sensitive)
2. Ensure GitHub App is installed on the repository
3. Check if repository is private and app has access
4. Verify the repository hasn't been renamed or deleted

### Secondary Rate Limit

**Symptom**: API calls fail even though primary rate limit is not exceeded.

**Error**:
```json
{
  "error": {
    "message": "You have exceeded a secondary rate limit",
    "type": "secondary_rate_limit"
  }
}
```

**Solution**:

1. Reduce request frequency (GitHub recommends max 1 request per second for mutations)
2. Add delays between operations:
   ```python
   await asyncio.sleep(1)  # Wait 1 second between operations
   ```
3. Batch operations when possible

---

## Connection Issues

### Connection Timeout

**Symptom**: LangGraph cannot connect to the MCP server.

**Error**:
```
ConnectionError: Connection timeout after 30s
```

**Solution**:

1. Verify Railway service is running:
   - Check Railway dashboard for service status
   - Look for "Active" status

2. Test the endpoint manually:
   ```bash
   curl -X POST https://your-app.railway.app/mcp \
     -H "Authorization: Bearer your_token" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
   ```

3. Check for Railway outages: https://status.railway.app

4. Verify your network allows HTTPS connections to Railway

### SSL/TLS Errors

**Symptom**: Connection fails with SSL certificate errors.

**Error**:
```
SSLError: certificate verify failed
```

**Solution**:

1. Ensure you're using HTTPS (not HTTP):
   ```python
   # Correct
   url="https://your-app.railway.app/mcp"
   
   # Incorrect
   url="http://your-app.railway.app/mcp"
   ```

2. Update your SSL certificates:
   ```bash
   pip install --upgrade certifi
   ```

3. If using a corporate proxy, configure SSL verification appropriately

### Connection Refused

**Symptom**: Connection is actively refused.

**Error**:
```
ConnectionRefusedError: [Errno 111] Connection refused
```

**Solution**:

1. Verify the Railway URL is correct (check Railway dashboard)
2. Ensure the service is deployed and running
3. Check Railway logs for startup errors
4. Verify the PORT environment variable is being used correctly

---

## Performance Issues

### Slow Response Times

**Symptom**: API calls take longer than expected (> 5 seconds).

**Possible Causes**:

1. **Cold start**: Railway may pause services on free tier
   - **Solution**: Upgrade to a paid plan for always-on services

2. **GitHub API latency**: Some operations are inherently slow
   - **Solution**: Use GraphQL for bulk operations when possible

3. **Token minting overhead**: First request after cache expiry is slower
   - **Solution**: This is expected behavior; subsequent requests will be faster

4. **Network latency**: Distance between your location and Railway/GitHub servers
   - **Solution**: Consider Railway region selection if available

### High Memory Usage

**Symptom**: Railway shows high memory usage or OOM (out of memory) errors.

**Solution**:

1. Check Railway logs for memory-related errors
2. Upgrade Railway plan for more memory
3. Review token cache size (should be minimal for typical usage)
4. Check for memory leaks in custom code

### Concurrent Request Issues

**Symptom**: Some requests fail when multiple LangGraph instances connect simultaneously.

**Error**:
```json
{
  "error": {
    "message": "Too many concurrent connections",
    "type": "server_error"
  }
}
```

**Solution**:

1. Upgrade Railway plan for higher connection limits
2. Implement request queuing in your LangGraph code
3. Use connection pooling in your MCP client
4. Consider deploying multiple Railway services for different teams/projects

---

## Debugging Tips

### Enable Debug Logging

Set environment variable in Railway:
```
LOG_LEVEL=debug
```

This will provide detailed logs for:
- Token resolution
- GitHub API calls
- Cache hits/misses
- Request/response details

### Check Railway Logs

View logs in Railway dashboard:
1. Go to your service
2. Click "Deployments" tab
3. Click on the latest deployment
4. View "Logs" section

Look for:
- Startup messages
- Error messages
- Authentication events
- API call traces

### Test with curl

Test the MCP endpoint directly:

```bash
# Test health check
curl https://your-app.railway.app/health

# Test MCP connection
curl -X POST https://your-app.railway.app/mcp \
  -H "Authorization: Bearer your_mcp_token" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
  }'
```

### Verify GitHub App Configuration

Use GitHub's API to verify your app configuration:

```bash
# Get app info (requires JWT)
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  https://api.github.com/app

# List installations
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  https://api.github.com/app/installations

# Get installation token (requires JWT)
curl -X POST \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  https://api.github.com/app/installations/INSTALLATION_ID/access_tokens
```

### Common Log Messages

**Normal operation**:
```
level=info message="Server starting" port=8080
level=info message="GitHub App configured" app_id=123456
level=info message="Token mapping loaded" token_count=1
level=info message="Server ready"
```

**Warning signs**:
```
level=warn message="Token cache miss" installation_id=12345678
level=warn message="Rate limit approaching" remaining=100
level=error message="Failed to mint installation token" error="401 Unauthorized"
level=error message="GitHub API error" status=403 message="Resource not accessible"
```

---

## Getting Help

If you've tried the solutions above and still have issues:

1. **Check Railway Status**: https://status.railway.app
2. **Check GitHub Status**: https://www.githubstatus.com
3. **Review Documentation**: 
   - [Railway Docs](https://docs.railway.app)
   - [GitHub Apps Docs](https://docs.github.com/en/apps)
   - [MCP Protocol Spec](https://modelcontextprotocol.io)
4. **Enable Debug Logging**: Set `LOG_LEVEL=debug` and review logs
5. **Create an Issue**: Include:
   - Railway logs (sanitize sensitive data)
   - Steps to reproduce
   - Expected vs actual behavior
   - Environment details

---

## Quick Reference

### Required Environment Variables

```bash
GITHUB_APP_ID=123456
GITHUB_APP_PRIVATE_KEY_B64=LS0tLS1CRUdJTi...
GITHUB_MCP_TOKEN_MAP={"token":"installation:12345678"}
```

### Optional Environment Variables

```bash
PORT=8080                    # Railway provides this
GITHUB_HOST=github.com       # For GitHub Enterprise
LOG_LEVEL=info              # debug, info, warn, error
GITHUB_TOOLSETS=repos,issues,pull_requests,actions,git
```

### Health Check Endpoint

```bash
GET /health
Response: {"status":"healthy","version":"1.0.0","uptime_seconds":3600}
```

### MCP Endpoint

```bash
POST /mcp
Headers: Authorization: Bearer <token>
Content-Type: application/json
Body: MCP protocol JSON-RPC messages
```
