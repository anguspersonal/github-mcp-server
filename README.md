# GitHub MCP Server for Railway

A streamlined GitHub MCP (Model Context Protocol) server designed for deployment on Railway as a GitHub App. This server provides HTTP-based access to GitHub's API for autonomous development workflows powered by LangGraph agents.

## Overview

This MCP server acts as the bridge between LangGraph's autonomous development capabilities and GitHub's API. It enables AI agents to:

- Query backlog items (issues, project items)
- Create development branches
- Commit code changes
- Create and manage pull requests
- Monitor workflow runs and checks

The server is optimized for Railway deployment with GitHub App authentication, providing secure, fine-grained access to GitHub resources.

## Architecture

```
┌─────────────────┐
│   LangGraph     │
│     Agent       │
└────────┬────────┘
         │ HTTPS/MCP Protocol
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

## Quick Start

### Prerequisites

1. A [Railway](https://railway.app) account
2. A GitHub account with admin access to the repositories you want to automate
3. Basic understanding of GitHub Apps and MCP protocol

### Deployment Steps

1. **Create a GitHub App**
2. **Deploy to Railway**
3. **Configure Environment Variables**
4. **Connect from LangGraph**

Detailed instructions for each step are provided below.

---

## 1. GitHub App Setup

### Create a GitHub App

1. Navigate to your GitHub account settings:
   - For personal account: `https://github.com/settings/apps`
   - For organization: `https://github.com/organizations/YOUR_ORG/settings/apps`

2. Click **"New GitHub App"**

3. Configure the app:
   - **GitHub App name**: Choose a unique name (e.g., "My LangGraph MCP Server")
   - **Homepage URL**: Your Railway app URL (you can update this later)
   - **Webhook**: Uncheck "Active" (not needed for this use case)

4. Set **Repository permissions**:
   - **Contents**: Read & Write (for file operations, branches)
   - **Issues**: Read & Write (for backlog item access)
   - **Pull requests**: Read & Write (for PR creation)
   - **Metadata**: Read (always required)
   - **Actions**: Read (for workflow status)
   - **Workflows**: Read & Write (for triggering workflows)

5. Set **Where can this GitHub App be installed?**:
   - Choose "Only on this account" for personal use
   - Or "Any account" if you want to share it

6. Click **"Create GitHub App"**

### Generate Private Key

1. After creating the app, scroll down to **"Private keys"**
2. Click **"Generate a private key"**
3. Save the downloaded `.pem` file securely

### Install the GitHub App

1. In your GitHub App settings, click **"Install App"** in the left sidebar
2. Select the account/organization where you want to install it
3. Choose:
   - **All repositories** (for full access)
   - **Only select repositories** (for limited access)
4. Click **"Install"**
5. Note the **Installation ID** from the URL (e.g., `https://github.com/settings/installations/12345678` → ID is `12345678`)

### Collect Required Information

You'll need these values for Railway configuration:

- **App ID**: Found in your GitHub App settings (e.g., `123456`)
- **Private Key**: The `.pem` file you downloaded
- **Installation ID**: From the installation URL (e.g., `12345678`)

---

## 2. Railway Deployment

### Deploy from GitHub

1. **Fork or clone this repository** to your GitHub account

2. **Create a new project on Railway**:
   - Go to [railway.app](https://railway.app)
   - Click **"New Project"**
   - Select **"Deploy from GitHub repo"**
   - Choose your forked repository

3. **Configure Railway Service ID for cache mounts**:
   
   Railway requires cache mount IDs to be prefixed with your Service ID. Set it as an environment variable:
   
   a. **Get your Railway Service ID**:
      - In Railway dashboard, open your service
      - Press `Cmd + K` (macOS) or `Ctrl + K` (Windows/Linux) to open command palette
      - Look for "Copy Service ID" or "Copy Workspace ID" (Railway UI may show either)
      - If you see "Copy Workspace ID", that's the same as Service ID for single-service projects
      - Copy the ID (it looks like: `abc123-def456-ghi789-...`)
   
   b. **Set as a SERVICE variable (not shared)**:
      - Go to Railway dashboard → Your service → **Variables** tab (NOT Project Settings)
      - Click "New Variable"
      - Name: `RAILWAY_SERVICE_ID`
      - Value: Paste your Service ID (the one you copied)
      - Click "Add"
      - **Important**: Set this as a **Service Variable**, not a Shared Variable
   
   **Why Service Variable?**: Service variables are available during Docker builds. Shared variables are primarily for runtime sharing across services.
   
   **Note**: Railway makes service environment variables available during Docker builds, and the Dockerfile uses `ARG RAILWAY_SERVICE_ID` to access it.

4. **Railway will automatically**:
   - Detect the `Dockerfile`
   - Build the Docker image
   - Deploy the service
   - Assign a public URL

5. **Note your Railway URL**:
   - Find it in the Railway dashboard under your service
   - Format: `https://your-app.railway.app`

### Local Development (Optional)

If you want to test Docker builds locally, you can set `RAILWAY_SERVICE_ID` as an environment variable:

```bash
# Linux/Mac
export RAILWAY_SERVICE_ID="your-service-id-here"
docker build --build-arg RAILWAY_SERVICE_ID=$RAILWAY_SERVICE_ID -t github-mcp-server .

# Windows (PowerShell)
$env:RAILWAY_SERVICE_ID="your-service-id-here"
docker build --build-arg RAILWAY_SERVICE_ID=$env:RAILWAY_SERVICE_ID -t github-mcp-server .
```

Or create a `.env` file (for local development only - don't commit it):
```
RAILWAY_SERVICE_ID=your-service-id-here
```

Then load it before building:
```bash
# Linux/Mac
export $(cat .env | xargs)
docker build --build-arg RAILWAY_SERVICE_ID=$RAILWAY_SERVICE_ID -t github-mcp-server .
```

### Manual Deployment (Alternative)

If you prefer to deploy manually:

```bash
# Install Railway CLI
npm install -g @railway/cli

# Login to Railway
railway login

# Initialize project
railway init

# Deploy
railway up
```

---

## 3. Environment Variable Configuration

### Required Environment Variables

Configure these in your Railway dashboard (Settings → Variables):

#### `GITHUB_APP_ID`
Your GitHub App ID (numeric value from GitHub App settings)

```
GITHUB_APP_ID=123456
```

#### `GITHUB_APP_PRIVATE_KEY_B64`
Base64-encoded private key. Convert your `.pem` file:

**On Linux/Mac:**
```bash
base64 -i your-private-key.pem | tr -d '\n'
```

**On Windows (PowerShell):**
```powershell
[Convert]::ToBase64String([IO.File]::ReadAllBytes("your-private-key.pem"))
```

Then set in Railway:
```
GITHUB_APP_PRIVATE_KEY_B64=LS0tLS1CRUdJTi...
```

#### `GITHUB_MCP_TOKEN_MAP`
JSON mapping of MCP tokens to GitHub App installation IDs:

```json
{
  "your_secret_mcp_token_1": "installation:12345678",
  "your_secret_mcp_token_2": "installation:87654321"
}
```

**Generate secure tokens:**
```bash
# Linux/Mac
openssl rand -hex 32

# Or use any secure random string generator
```

Set in Railway:
```
GITHUB_MCP_TOKEN_MAP={"langgraph_prod":"installation:12345678"}
```

### Optional Environment Variables

#### `PORT`
Railway automatically sets this. Default: `8080`

#### `GITHUB_HOST`
For GitHub Enterprise Server or Enterprise Cloud with data residency:

```
GITHUB_HOST=https://github.enterprise.com
```

Or for GitHub Enterprise Cloud with data residency:
```
GITHUB_HOST=https://yoursubdomain.ghe.com
```

#### `GITHUB_TOOLSETS`
Comma-separated list of toolsets to enable. Default: `context,repos,issues,pull_requests,users`

```
GITHUB_TOOLSETS=repos,issues,pull_requests,actions,git
```

Available toolsets:
- `context` - User and team context
- `repos` - Repository operations
- `issues` - Issue management
- `pull_requests` - PR operations
- `actions` - GitHub Actions workflows
- `git` - Low-level Git operations
- `labels` - Label management
- `projects` - GitHub Projects
- `discussions` - GitHub Discussions
- `gists` - Gist operations
- `notifications` - Notification management
- `code_security` - Code scanning
- `secret_protection` - Secret scanning
- `security_advisories` - Security advisories
- `dependabot` - Dependabot alerts

#### `GITHUB_READ_ONLY`
Set to `true` to enable read-only mode (no write operations):

```
GITHUB_READ_ONLY=false
```

#### `GITHUB_LOCKDOWN_MODE`
Set to `true` to limit content from public repositories:

```
GITHUB_LOCKDOWN_MODE=false
```

### Verify Configuration

After setting environment variables:

1. Railway will automatically redeploy
2. Check the deployment logs for any errors
3. Look for: `Server starting on port 8080`
4. Test the health endpoint: `https://your-app.railway.app/health`

Expected health response:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 3600,
  "github_api_reachable": true
}
```

---

## 4. LangGraph Integration

### MCP Client Configuration

Configure your LangGraph application to connect to the Railway-hosted MCP server:

```python
from langgraph.mcp import MCPClient

# Initialize MCP client
client = MCPClient(
    url="https://your-app.railway.app/mcp",
    headers={
        "Authorization": f"Bearer {your_mcp_token}"
    }
)

# List available tools
tools = await client.list_tools()
print(f"Available tools: {len(tools)}")
```

### Example: Autonomous Development Workflow

Here's a complete example of an autonomous workflow that goes from backlog item to pull request:

```python
import asyncio
from langgraph.mcp import MCPClient

async def autonomous_development_workflow():
    # Initialize client
    client = MCPClient(
        url="https://your-app.railway.app/mcp",
        headers={"Authorization": "Bearer your_mcp_token"}
    )
    
    # 1. Query backlog items
    print("📋 Fetching backlog items...")
    issues = await client.call_tool("list_issues", {
        "owner": "myorg",
        "repo": "myrepo",
        "state": "open",
        "labels": ["backlog", "ready"]
    })
    
    if not issues:
        print("No backlog items found")
        return
    
    # Select first issue
    issue = issues[0]
    issue_number = issue["number"]
    issue_title = issue["title"]
    
    print(f"🎯 Working on: #{issue_number} - {issue_title}")
    
    # 2. Create feature branch
    print("🌿 Creating feature branch...")
    branch_name = f"feature/issue-{issue_number}"
    branch = await client.call_tool("create_branch", {
        "owner": "myorg",
        "repo": "myrepo",
        "branch": branch_name,
        "from_branch": "main"
    })
    
    print(f"✓ Branch created: {branch_name}")
    
    # 3. Generate and commit code
    print("💻 Generating code...")
    # Your AI code generation logic here
    generated_code = generate_code_for_issue(issue)
    
    print("📝 Committing changes...")
    commit = await client.call_tool("create_or_update_file", {
        "owner": "myorg",
        "repo": "myrepo",
        "path": "src/feature.py",
        "content": generated_code,
        "message": f"Implement {issue_title}\n\nCloses #{issue_number}",
        "branch": branch_name
    })
    
    print(f"✓ Committed: {commit['sha'][:7]}")
    
    # 4. Create pull request
    print("🔀 Creating pull request...")
    pr = await client.call_tool("create_pull_request", {
        "owner": "myorg",
        "repo": "myrepo",
        "title": f"Implement {issue_title}",
        "head": branch_name,
        "base": "main",
        "body": f"""
## Description
Automated implementation of #{issue_number}

## Changes
- Implemented {issue_title}

## Testing
- [ ] Unit tests added
- [ ] Integration tests pass

Closes #{issue_number}
        """.strip()
    })
    
    pr_number = pr["number"]
    pr_url = pr["html_url"]
    
    print(f"✅ Pull request created: #{pr_number}")
    print(f"🔗 {pr_url}")
    
    # 5. Monitor workflow status (optional)
    print("⏳ Monitoring CI/CD...")
    await asyncio.sleep(30)  # Wait for workflows to start
    
    workflows = await client.call_tool("list_workflow_runs", {
        "owner": "myorg",
        "repo": "myrepo",
        "branch": branch_name
    })
    
    if workflows:
        status = workflows[0]["status"]
        conclusion = workflows[0].get("conclusion")
        print(f"📊 Workflow status: {status} ({conclusion})")

# Run the workflow
asyncio.run(autonomous_development_workflow())
```

### Error Handling

Implement robust error handling for production use:

```python
from langgraph.mcp import MCPClient, MCPError

async def safe_tool_call(client, tool_name, params, max_retries=3):
    """Call MCP tool with error handling and retries"""
    
    for attempt in range(max_retries):
        try:
            result = await client.call_tool(tool_name, params)
            return result
            
        except MCPError as e:
            error_type = e.data.get("type") if e.data else None
            
            # Handle rate limiting
            if error_type == "rate_limit_exceeded":
                retry_after = e.data.get("retry_after", 3600)
                print(f"⚠️  Rate limited. Waiting {retry_after}s...")
                await asyncio.sleep(retry_after)
                continue
            
            # Handle authentication errors
            elif error_type == "authentication_failed":
                print(f"❌ Authentication failed: {e.message}")
                raise  # Don't retry auth errors
            
            # Handle permission errors
            elif error_type == "permission_denied":
                required_scopes = e.data.get("required_scopes", [])
                print(f"❌ Permission denied. Required: {required_scopes}")
                raise  # Don't retry permission errors
            
            # Handle GitHub API errors
            elif error_type == "github_api_error":
                status_code = e.data.get("status_code")
                if status_code >= 500:
                    # Retry on server errors
                    wait_time = 2 ** attempt  # Exponential backoff
                    print(f"⚠️  GitHub API error. Retrying in {wait_time}s...")
                    await asyncio.sleep(wait_time)
                    continue
                else:
                    # Don't retry client errors
                    print(f"❌ GitHub API error: {e.message}")
                    raise
            
            # Unknown error
            else:
                print(f"❌ Unexpected error: {e.message}")
                if attempt < max_retries - 1:
                    wait_time = 2 ** attempt
                    print(f"⚠️  Retrying in {wait_time}s...")
                    await asyncio.sleep(wait_time)
                    continue
                raise
    
    raise Exception(f"Failed after {max_retries} attempts")
```

### Token Management

For production deployments, manage tokens securely:

```python
import os
from typing import Dict

class MCPTokenManager:
    """Manage MCP tokens for different environments"""
    
    def __init__(self):
        self.tokens: Dict[str, str] = {
            "production": os.getenv("MCP_TOKEN_PROD"),
            "staging": os.getenv("MCP_TOKEN_STAGING"),
            "development": os.getenv("MCP_TOKEN_DEV"),
        }
    
    def get_client(self, environment: str = "production") -> MCPClient:
        """Get MCP client for specific environment"""
        token = self.tokens.get(environment)
        if not token:
            raise ValueError(f"No token configured for {environment}")
        
        # Use different Railway deployments per environment
        urls = {
            "production": "https://mcp-prod.railway.app/mcp",
            "staging": "https://mcp-staging.railway.app/mcp",
            "development": "https://mcp-dev.railway.app/mcp",
        }
        
        return MCPClient(
            url=urls[environment],
            headers={"Authorization": f"Bearer {token}"}
        )

# Usage
manager = MCPTokenManager()
client = manager.get_client("production")
```

### Scheduled Workflows

Run autonomous workflows on a schedule:

```python
import asyncio
import schedule
import time

async def scheduled_backlog_processor():
    """Process backlog items on a schedule"""
    client = MCPClient(
        url="https://your-app.railway.app/mcp",
        headers={"Authorization": f"Bearer {os.getenv('MCP_TOKEN')}"}
    )
    
    print(f"🕐 Running scheduled backlog processor at {time.ctime()}")
    
    # Query ready backlog items
    issues = await client.call_tool("list_issues", {
        "owner": "myorg",
        "repo": "myrepo",
        "state": "open",
        "labels": ["backlog", "ready", "automated"]
    })
    
    print(f"📋 Found {len(issues)} ready items")
    
    # Process each issue
    for issue in issues[:5]:  # Limit to 5 per run
        try:
            await autonomous_development_workflow(client, issue)
        except Exception as e:
            print(f"❌ Failed to process #{issue['number']}: {e}")
            continue

def run_scheduler():
    """Run the scheduler"""
    # Schedule to run every hour
    schedule.every().hour.do(
        lambda: asyncio.run(scheduled_backlog_processor())
    )
    
    # Or run at specific times
    schedule.every().day.at("09:00").do(
        lambda: asyncio.run(scheduled_backlog_processor())
    )
    
    print("🚀 Scheduler started")
    while True:
        schedule.run_pending()
        time.sleep(60)

# Run
run_scheduler()
```

---

## Troubleshooting

### Common Errors and Solutions

#### 1. Authentication Failed

**Error:**
```json
{
  "error": {
    "code": -32000,
    "message": "Authentication failed",
    "data": {
      "type": "authentication_failed",
      "details": "Invalid or missing Bearer token"
    }
  }
}
```

**Solutions:**
- Verify your MCP token is correct in the `Authorization` header
- Check that the token exists in `GITHUB_MCP_TOKEN_MAP` on Railway
- Ensure the token mapping format is correct: `{"token": "installation:ID"}`

#### 2. GitHub App JWT Creation Failed

**Error in Railway logs:**
```
Failed to create GitHub App JWT: invalid private key format
```

**Solutions:**
- Verify `GITHUB_APP_PRIVATE_KEY_B64` is correctly base64-encoded
- Ensure no extra whitespace or newlines in the environment variable
- Re-download the private key from GitHub and re-encode it
- Check that the private key matches the GitHub App ID

#### 3. Installation Token Minting Failed

**Error:**
```json
{
  "error": {
    "code": -32000,
    "message": "Failed to mint installation token",
    "data": {
      "type": "github_api_error",
      "status_code": 404
    }
  }
}
```

**Solutions:**
- Verify the installation ID in `GITHUB_MCP_TOKEN_MAP` is correct
- Check that the GitHub App is installed on the target account/org
- Ensure the GitHub App has not been uninstalled or suspended
- Verify the App ID matches the private key

#### 4. Permission Denied

**Error:**
```json
{
  "error": {
    "code": -32000,
    "message": "Permission denied",
    "data": {
      "type": "permission_denied",
      "required_scopes": ["contents:write"],
      "details": "GitHub App lacks required permissions"
    }
  }
}
```

**Solutions:**
- Review GitHub App permissions in app settings
- Add the required permissions (e.g., Contents: Write)
- Users must accept the new permissions after updating
- Reinstall the app if permissions were added after installation

#### 5. Rate Limit Exceeded

**Error:**
```json
{
  "error": {
    "code": -32000,
    "message": "Rate limit exceeded",
    "data": {
      "type": "rate_limit_exceeded",
      "retry_after": 3600,
      "details": "Rate limit will reset at 2024-12-07T18:00:00Z"
    }
  }
}
```

**Solutions:**
- Implement exponential backoff in your LangGraph code
- Use the `retry_after` value to wait before retrying
- Consider caching responses to reduce API calls
- For higher limits, use a GitHub App (not PAT)

#### 6. Railway Deployment Issues

**Service won't start:**

Check Railway logs for:
```
Missing required environment variable: GITHUB_APP_ID
```

**Solutions:**
- Verify all required environment variables are set
- Check for typos in variable names
- Ensure values don't have extra quotes or spaces
- Redeploy after setting variables

**Health check failing:**

```
Health check timeout after 30s
```

**Solutions:**
- Check that the service is listening on the `PORT` environment variable
- Verify the Dockerfile builds correctly
- Check Railway logs for startup errors
- Ensure the health endpoint `/health` is accessible

#### 7. Connection Timeout

**Error:**
```
Connection timeout after 30s
```

**Solutions:**
- Check Railway service status in dashboard
- Verify the Railway URL is correct
- Ensure Railway service is not sleeping (upgrade plan if needed)
- Check for network/firewall issues

#### 8. Invalid MCP Protocol Message

**Error:**
```json
{
  "error": {
    "code": -32700,
    "message": "Parse error"
  }
}
```

**Solutions:**
- Verify you're sending valid JSON-RPC 2.0 messages
- Check that the `Content-Type` header is set correctly
- Ensure the request body is valid JSON
- Use the MCP client library instead of raw HTTP

### GitHub App Permission Issues

If you're getting permission errors, verify your GitHub App has these permissions:

**Required:**
- Contents: Read & Write
- Issues: Read & Write
- Pull requests: Read & Write
- Metadata: Read

**Optional (based on toolsets):**
- Actions: Read
- Workflows: Read & Write
- Discussions: Read & Write
- Projects: Read & Write

**To update permissions:**
1. Go to GitHub App settings
2. Scroll to "Permissions"
3. Update required permissions
4. Save changes
5. Users will need to accept new permissions

### Token Resolution Issues

**Problem:** Token not resolving to installation

**Debug steps:**
1. Check Railway logs for token resolution attempts
2. Verify token mapping JSON is valid
3. Ensure installation ID is correct (numeric, not string)
4. Test with a simple curl command:

```bash
curl -X POST https://your-app.railway.app/mcp \
  -H "Authorization: Bearer your_mcp_token" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

### Railway Deployment Issues

**Build failures:**

Check Railway build logs for:
- Docker build errors
- Missing dependencies
- Go compilation errors

**Solutions:**
- Verify Dockerfile syntax
- Check that all Go dependencies are in `go.mod`
- Ensure the build context includes all necessary files

**Service crashes:**

Check Railway logs for:
- Panic messages
- Startup errors
- Configuration errors

**Solutions:**
- Verify environment variables are set correctly
- Check for missing required configuration
- Review error messages in logs
- Ensure the service can bind to the PORT

### Getting Help

If you're still experiencing issues:

1. **Check Railway logs**: Railway Dashboard → Your Service → Logs
2. **Test health endpoint**: `curl https://your-app.railway.app/health`
3. **Verify GitHub App**: Check installation status and permissions
4. **Review configuration**: Double-check all environment variables
5. **Test locally**: Run the server locally with Docker to isolate issues

**Local testing:**
```bash
# Build Docker image
docker build -t github-mcp-server .

# Run locally
docker run -p 8080:8080 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_APP_PRIVATE_KEY_B64=your_base64_key \
  -e GITHUB_MCP_TOKEN_MAP='{"test":"installation:12345678"}' \
  github-mcp-server

# Test health endpoint
curl http://localhost:8080/health
```

---

## Security Best Practices

### Token Security

1. **Generate Strong MCP Tokens**
   ```bash
   # Use cryptographically secure random tokens
   openssl rand -hex 32
   ```

2. **Rotate Tokens Regularly**
   - Rotate MCP tokens every 90 days
   - Rotate GitHub App private keys annually
   - Update tokens in Railway environment variables

3. **Use Different Tokens Per Environment**
   ```json
   {
     "production_token": "installation:12345678",
     "staging_token": "installation:87654321",
     "development_token": "installation:11111111"
   }
   ```

4. **Never Commit Tokens**
   - Add `.env` files to `.gitignore`
   - Use Railway's environment variables
   - Never hardcode tokens in source code

### GitHub App Security

1. **Principle of Least Privilege**
   - Only request permissions you need
   - Use repository-level installation when possible
   - Review permissions regularly

2. **Private Key Protection**
   - Store private keys securely
   - Never commit private keys to version control
   - Use base64 encoding for environment variables
   - Rotate keys if compromised

3. **Installation Scope**
   - Install only on required repositories
   - Review installation access regularly
   - Remove unused installations

### Network Security

1. **HTTPS Only**
   - Railway provides automatic HTTPS
   - Never use HTTP for production

2. **Authentication Required**
   - All requests must include valid Bearer token
   - No anonymous access allowed

3. **Rate Limiting**
   - Implement client-side rate limiting
   - Respect GitHub API rate limits
   - Use exponential backoff on errors

### Monitoring and Auditing

1. **Enable Logging**
   - Monitor Railway logs for suspicious activity
   - Log all authentication attempts
   - Track API usage patterns

2. **Set Up Alerts**
   - Alert on authentication failures
   - Monitor for unusual API usage
   - Track error rates

3. **Regular Audits**
   - Review GitHub App installations
   - Audit token usage
   - Check for unauthorized access

---

## Advanced Configuration

### Multiple GitHub Apps

Support multiple GitHub Apps for different organizations:

```json
{
  "org1_token": "installation:12345678",
  "org2_token": "installation:87654321",
  "personal_token": "installation:11111111"
}
```

Each token maps to a different installation ID, allowing one Railway deployment to serve multiple organizations.

### Custom Toolsets

Enable only the toolsets you need:

```bash
GITHUB_TOOLSETS=repos,issues,pull_requests,actions
```

This reduces the number of available tools, helping the LLM with tool selection and reducing context size.

### Read-Only Mode

For monitoring or analysis workflows:

```bash
GITHUB_READ_ONLY=true
```

This prevents any write operations, ensuring the agent can only read data.

### Lockdown Mode

For public repositories, limit content to collaborators:

```bash
GITHUB_LOCKDOWN_MODE=true
```

This filters out content from non-collaborators in public repositories.

### GitHub Enterprise

For GitHub Enterprise Server:

```bash
GITHUB_HOST=https://github.enterprise.com
```

For GitHub Enterprise Cloud with data residency:

```bash
GITHUB_HOST=https://yoursubdomain.ghe.com
```

---

## Performance Optimization

### Installation Token Caching

The server automatically caches installation tokens for ~1 hour, reducing GitHub API calls by ~99%. Cache hit rate should be >90% under normal operation.

### Connection Pooling

HTTP clients reuse connections to GitHub API, with a default pool size of 100 connections.

### Concurrent Requests

The server handles concurrent connections efficiently using Go's goroutines. No explicit connection limit (Railway handles this based on your plan).

### Expected Performance

- **Token resolution**: <1ms (cache hit), <500ms (cache miss)
- **GitHub API calls**: 100-500ms (depends on operation)
- **End-to-end request**: 200-1000ms
- **Concurrent connections**: 100+ (limited by Railway plan)
- **Requests per second**: 50+ (limited by GitHub API rate limits)

### Resource Usage

- **Memory**: <100MB under normal load
- **CPU**: <0.1 vCPU under normal load
- **Network**: Depends on GitHub API usage

---

## Monitoring

### Health Checks

The server provides a health endpoint at `/health`:

```bash
curl https://your-app.railway.app/health
```

Response:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 3600,
  "github_api_reachable": true
}
```

### Railway Logs

Monitor logs in Railway dashboard:
- Server startup messages
- Token resolution attempts
- GitHub API calls
- Error messages

### Key Metrics

Monitor these metrics:
- Request count and latency
- Token resolution success/failure rate
- GitHub API call count and latency
- Installation token cache hit rate
- Error rates by type

---

## License

This project is licensed under the terms of the MIT open source license. Please refer to [MIT](./LICENSE) for the full terms.

---

## Support

For issues and questions:
- Check the [Troubleshooting](#troubleshooting) section
- Review Railway logs for error messages
- Verify GitHub App configuration and permissions
- Test the health endpoint

---

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices
- Tests pass (`go test ./...`)
- Documentation is updated
- Commits are clear and descriptive
