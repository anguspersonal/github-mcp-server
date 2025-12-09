# LangGraph Integration Guide

This guide explains how to integrate LangGraph agents with the GitHub MCP server deployed on Railway.

## Overview

The GitHub MCP server provides a hosted HTTP endpoint that LangGraph agents can use to interact with GitHub's API. This enables autonomous development workflows where LangGraph agents can:

- Query backlog items (issues, project items)
- Create feature branches
- Commit code changes
- Create pull requests
- Monitor workflow status

## Prerequisites

- Railway-deployed GitHub MCP server (see main README)
- GitHub App configured with appropriate permissions
- MCP API token configured in Railway environment variables
- Python 3.8+ with LangGraph installed

## MCP Client Configuration

### Installation

```bash
pip install langgraph-mcp
```

### Basic Client Setup

```python
from langgraph.mcp import MCPClient

# Initialize the MCP client
client = MCPClient(
    url="https://your-app.railway.app/mcp",
    headers={
        "Authorization": f"Bearer {mcp_token}"
    }
)
```

### Configuration with Environment Variables

```python
import os
from langgraph.mcp import MCPClient

# Load configuration from environment
MCP_SERVER_URL = os.getenv("MCP_SERVER_URL", "https://your-app.railway.app/mcp")
MCP_API_TOKEN = os.getenv("MCP_API_TOKEN")

if not MCP_API_TOKEN:
    raise ValueError("MCP_API_TOKEN environment variable is required")

client = MCPClient(
    url=MCP_SERVER_URL,
    headers={
        "Authorization": f"Bearer {MCP_API_TOKEN}"
    },
    timeout=30  # Request timeout in seconds
)
```

### Advanced Configuration

```python
from langgraph.mcp import MCPClient
import httpx

# Custom HTTP client with retry logic
http_client = httpx.Client(
    timeout=30.0,
    limits=httpx.Limits(max_connections=10, max_keepalive_connections=5),
    transport=httpx.HTTPTransport(retries=3)
)

client = MCPClient(
    url=MCP_SERVER_URL,
    headers={
        "Authorization": f"Bearer {MCP_API_TOKEN}",
        "User-Agent": "MyLangGraphAgent/1.0"
    },
    http_client=http_client
)
```

## Token Management

### Understanding Token Flow

The MCP server uses a two-tier token system:

1. **MCP API Token**: Bearer token you provide to authenticate with the MCP server
2. **GitHub Installation Token**: Server-side token minted from GitHub App credentials

```
LangGraph Agent → [MCP API Token] → MCP Server → [Installation Token] → GitHub API
```

### Obtaining MCP API Tokens

MCP API tokens are configured in the Railway environment variable `GITHUB_MCP_TOKEN_MAP`:

```json
{
  "langgraph_prod_token": "installation:12345678",
  "langgraph_dev_token": "installation:87654321"
}
```

Each token maps to a specific GitHub App installation ID. Contact your system administrator to obtain tokens.

### Token Security Best Practices

1. **Store tokens securely**: Use environment variables or secret management systems
2. **Use different tokens per environment**: Separate tokens for dev, staging, production
3. **Rotate tokens regularly**: Update tokens every 90 days
4. **Never commit tokens**: Add `.env` files to `.gitignore`

### Token Validation

```python
async def validate_token(client: MCPClient) -> bool:
    """Validate MCP token by calling a simple tool."""
    try:
        result = await client.call_tool("get_me", {})
        return True
    except Exception as e:
        print(f"Token validation failed: {e}")
        return False

# Use at startup
if not await validate_token(client):
    raise RuntimeError("Invalid MCP API token")
```

## Example Workflows

### 1. Query Backlog Items

```python
async def get_backlog_items(client: MCPClient, owner: str, repo: str):
    """Retrieve open issues labeled as backlog items."""
    result = await client.call_tool("list_issues", {
        "owner": owner,
        "repo": repo,
        "state": "open",
        "labels": "backlog",
        "per_page": 50
    })
    
    # Parse the response
    issues = result.get("content", [])
    return [issue for issue in issues if issue.get("type") == "text"]

# Usage
issues = await get_backlog_items(client, "myorg", "myrepo")
for issue in issues:
    print(f"Issue #{issue['number']}: {issue['title']}")
```

### 2. Create Feature Branch

```python
async def create_feature_branch(
    client: MCPClient,
    owner: str,
    repo: str,
    feature_name: str,
    base_branch: str = "main"
):
    """Create a new feature branch from base branch."""
    branch_name = f"feature/{feature_name}"
    
    result = await client.call_tool("create_branch", {
        "owner": owner,
        "repo": repo,
        "branch": branch_name,
        "from_branch": base_branch
    })
    
    return branch_name

# Usage
branch = await create_feature_branch(
    client,
    "myorg",
    "myrepo",
    "add-user-authentication"
)
print(f"Created branch: {branch}")
```

### 3. Commit Code Changes

```python
async def commit_file(
    client: MCPClient,
    owner: str,
    repo: str,
    branch: str,
    file_path: str,
    content: str,
    commit_message: str
):
    """Commit a file to the specified branch."""
    result = await client.call_tool("create_or_update_file", {
        "owner": owner,
        "repo": repo,
        "path": file_path,
        "content": content,
        "message": commit_message,
        "branch": branch
    })
    
    return result

# Usage
await commit_file(
    client,
    "myorg",
    "myrepo",
    "feature/add-user-authentication",
    "src/auth.py",
    "def authenticate(user, password):\n    # Implementation\n    pass",
    "Add authentication module"
)
```

### 4. Commit Multiple Files

```python
async def commit_multiple_files(
    client: MCPClient,
    owner: str,
    repo: str,
    branch: str,
    files: list[dict],
    commit_message: str
):
    """Commit multiple files in a single operation."""
    result = await client.call_tool("push_files", {
        "owner": owner,
        "repo": repo,
        "branch": branch,
        "files": files,
        "message": commit_message
    })
    
    return result

# Usage
files = [
    {"path": "src/auth.py", "content": "# Auth module\n..."},
    {"path": "tests/test_auth.py", "content": "# Tests\n..."},
    {"path": "README.md", "content": "# Updated docs\n..."}
]

await commit_multiple_files(
    client,
    "myorg",
    "myrepo",
    "feature/add-user-authentication",
    files,
    "Implement authentication with tests and docs"
)
```

### 5. Create Pull Request

```python
async def create_pull_request(
    client: MCPClient,
    owner: str,
    repo: str,
    title: str,
    head_branch: str,
    base_branch: str = "main",
    body: str = ""
):
    """Create a pull request."""
    result = await client.call_tool("create_pull_request", {
        "owner": owner,
        "repo": repo,
        "title": title,
        "head": head_branch,
        "base": base_branch,
        "body": body
    })
    
    return result

# Usage
pr = await create_pull_request(
    client,
    "myorg",
    "myrepo",
    "Add user authentication",
    "feature/add-user-authentication",
    "main",
    "This PR implements user authentication as described in issue #123"
)
print(f"Created PR #{pr['number']}: {pr['html_url']}")
```

### 6. Complete Autonomous Workflow

```python
async def autonomous_development_workflow(
    client: MCPClient,
    owner: str,
    repo: str,
    issue_number: int
):
    """
    Complete workflow: fetch issue → create branch → implement → create PR.
    """
    # 1. Fetch issue details
    issue = await client.call_tool("issue_read", {
        "owner": owner,
        "repo": repo,
        "issue_number": issue_number
    })
    
    issue_title = issue["title"]
    issue_body = issue["body"]
    
    # 2. Create feature branch
    branch_name = f"feature/issue-{issue_number}"
    await client.call_tool("create_branch", {
        "owner": owner,
        "repo": repo,
        "branch": branch_name,
        "from_branch": "main"
    })
    
    # 3. Generate and commit code (using your LLM/agent logic)
    generated_code = await generate_code_from_issue(issue_title, issue_body)
    
    await client.call_tool("create_or_update_file", {
        "owner": owner,
        "repo": repo,
        "path": generated_code["file_path"],
        "content": generated_code["content"],
        "message": f"Implement solution for issue #{issue_number}",
        "branch": branch_name
    })
    
    # 4. Create pull request
    pr = await client.call_tool("create_pull_request", {
        "owner": owner,
        "repo": repo,
        "title": f"Fix: {issue_title}",
        "head": branch_name,
        "base": "main",
        "body": f"Closes #{issue_number}\n\n{generated_code['description']}"
    })
    
    return pr

# Usage
pr = await autonomous_development_workflow(client, "myorg", "myrepo", 123)
print(f"Autonomous workflow complete! PR: {pr['html_url']}")
```

## Error Handling Patterns

### Understanding MCP Error Responses

The MCP server returns JSON-RPC error responses with structured information:

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

### Basic Error Handling

```python
from langgraph.mcp import MCPClient, MCPError

async def safe_tool_call(client: MCPClient, tool_name: str, params: dict):
    """Call a tool with basic error handling."""
    try:
        result = await client.call_tool(tool_name, params)
        return result
    except MCPError as e:
        print(f"MCP Error: {e.message}")
        print(f"Error code: {e.code}")
        if e.data:
            print(f"Error details: {e.data}")
        raise
    except Exception as e:
        print(f"Unexpected error: {e}")
        raise
```

### Rate Limit Handling

```python
import asyncio
from datetime import datetime

async def call_with_rate_limit_retry(
    client: MCPClient,
    tool_name: str,
    params: dict,
    max_retries: int = 3
):
    """Call a tool with automatic rate limit retry."""
    for attempt in range(max_retries):
        try:
            return await client.call_tool(tool_name, params)
        except MCPError as e:
            if e.data and e.data.get("type") == "rate_limit_exceeded":
                retry_after = e.data.get("retry_after", 60)
                print(f"Rate limit exceeded. Retrying after {retry_after} seconds...")
                await asyncio.sleep(retry_after)
            else:
                raise
    
    raise RuntimeError(f"Failed after {max_retries} retries")
```

### Authentication Error Handling

```python
async def handle_auth_errors(client: MCPClient, tool_name: str, params: dict):
    """Handle authentication-related errors."""
    try:
        return await client.call_tool(tool_name, params)
    except MCPError as e:
        if e.code == 401 or (e.data and e.data.get("type") == "authentication_failed"):
            print("Authentication failed. Please check your MCP API token.")
            print(f"Details: {e.message}")
            # Optionally: trigger token refresh or alert
            raise RuntimeError("Authentication failed")
        elif e.data and e.data.get("type") == "permission_denied":
            required_scopes = e.data.get("required_scopes", [])
            print(f"Permission denied. Required scopes: {', '.join(required_scopes)}")
            print("Please update GitHub App permissions in GitHub settings.")
            raise RuntimeError("Insufficient permissions")
        else:
            raise
```

### Comprehensive Error Handler

```python
import logging
from typing import Optional

logger = logging.getLogger(__name__)

class MCPErrorHandler:
    """Centralized error handling for MCP operations."""
    
    def __init__(self, client: MCPClient, max_retries: int = 3):
        self.client = client
        self.max_retries = max_retries
    
    async def call_tool(
        self,
        tool_name: str,
        params: dict,
        retry_on_rate_limit: bool = True
    ) -> dict:
        """Call a tool with comprehensive error handling."""
        for attempt in range(self.max_retries):
            try:
                result = await self.client.call_tool(tool_name, params)
                return result
            
            except MCPError as e:
                error_type = e.data.get("type") if e.data else None
                
                # Rate limit handling
                if error_type == "rate_limit_exceeded" and retry_on_rate_limit:
                    retry_after = e.data.get("retry_after", 60)
                    logger.warning(
                        f"Rate limit exceeded. Retrying after {retry_after}s "
                        f"(attempt {attempt + 1}/{self.max_retries})"
                    )
                    await asyncio.sleep(retry_after)
                    continue
                
                # Authentication errors (don't retry)
                elif error_type in ["authentication_failed", "permission_denied"]:
                    logger.error(f"Authentication error: {e.message}")
                    raise
                
                # Network errors (retry with backoff)
                elif error_type == "network_error":
                    backoff = 2 ** attempt
                    logger.warning(f"Network error. Retrying in {backoff}s...")
                    await asyncio.sleep(backoff)
                    continue
                
                # Other errors
                else:
                    logger.error(f"MCP error: {e.message} (type: {error_type})")
                    raise
            
            except Exception as e:
                logger.error(f"Unexpected error: {e}")
                raise
        
        raise RuntimeError(f"Failed after {self.max_retries} retries")

# Usage
handler = MCPErrorHandler(client, max_retries=3)
result = await handler.call_tool("create_pull_request", {
    "owner": "myorg",
    "repo": "myrepo",
    "title": "My PR",
    "head": "feature/branch",
    "base": "main"
})
```

### Error Recovery Strategies

```python
async def resilient_workflow(client: MCPClient, owner: str, repo: str):
    """Workflow with checkpointing and error recovery."""
    handler = MCPErrorHandler(client)
    
    # Checkpoint 1: Create branch
    try:
        branch = await handler.call_tool("create_branch", {
            "owner": owner,
            "repo": repo,
            "branch": "feature/resilient",
            "from_branch": "main"
        })
        print("✓ Branch created")
    except Exception as e:
        print(f"✗ Failed to create branch: {e}")
        return None
    
    # Checkpoint 2: Commit files
    try:
        await handler.call_tool("create_or_update_file", {
            "owner": owner,
            "repo": repo,
            "path": "src/feature.py",
            "content": "# Implementation",
            "message": "Add feature",
            "branch": "feature/resilient"
        })
        print("✓ Files committed")
    except Exception as e:
        print(f"✗ Failed to commit files: {e}")
        # Branch exists but no commits - could retry or clean up
        return None
    
    # Checkpoint 3: Create PR
    try:
        pr = await handler.call_tool("create_pull_request", {
            "owner": owner,
            "repo": repo,
            "title": "Resilient feature",
            "head": "feature/resilient",
            "base": "main"
        })
        print("✓ PR created")
        return pr
    except Exception as e:
        print(f"✗ Failed to create PR: {e}")
        # Branch and commits exist - could retry PR creation
        return None
```

## Advanced Patterns

### Concurrent Operations

```python
import asyncio

async def process_multiple_issues(
    client: MCPClient,
    owner: str,
    repo: str,
    issue_numbers: list[int]
):
    """Process multiple issues concurrently."""
    async def process_issue(issue_num: int):
        try:
            return await autonomous_development_workflow(
                client, owner, repo, issue_num
            )
        except Exception as e:
            print(f"Failed to process issue #{issue_num}: {e}")
            return None
    
    # Process up to 5 issues concurrently
    results = await asyncio.gather(
        *[process_issue(num) for num in issue_numbers],
        return_exceptions=True
    )
    
    return [r for r in results if r is not None]

# Usage
prs = await process_multiple_issues(client, "myorg", "myrepo", [101, 102, 103])
print(f"Created {len(prs)} pull requests")
```

### Workflow State Management

```python
from dataclasses import dataclass
from enum import Enum
from typing import Optional

class WorkflowState(Enum):
    PENDING = "pending"
    BRANCH_CREATED = "branch_created"
    CODE_COMMITTED = "code_committed"
    PR_CREATED = "pr_created"
    FAILED = "failed"

@dataclass
class WorkflowContext:
    issue_number: int
    owner: str
    repo: str
    state: WorkflowState = WorkflowState.PENDING
    branch_name: Optional[str] = None
    pr_number: Optional[int] = None
    error: Optional[str] = None

async def stateful_workflow(client: MCPClient, ctx: WorkflowContext):
    """Workflow with explicit state management for resumability."""
    handler = MCPErrorHandler(client)
    
    try:
        # State: Create branch
        if ctx.state == WorkflowState.PENDING:
            ctx.branch_name = f"feature/issue-{ctx.issue_number}"
            await handler.call_tool("create_branch", {
                "owner": ctx.owner,
                "repo": ctx.repo,
                "branch": ctx.branch_name,
                "from_branch": "main"
            })
            ctx.state = WorkflowState.BRANCH_CREATED
            print(f"State: {ctx.state}")
        
        # State: Commit code
        if ctx.state == WorkflowState.BRANCH_CREATED:
            await handler.call_tool("create_or_update_file", {
                "owner": ctx.owner,
                "repo": ctx.repo,
                "path": "src/feature.py",
                "content": "# Generated code",
                "message": f"Implement issue #{ctx.issue_number}",
                "branch": ctx.branch_name
            })
            ctx.state = WorkflowState.CODE_COMMITTED
            print(f"State: {ctx.state}")
        
        # State: Create PR
        if ctx.state == WorkflowState.CODE_COMMITTED:
            pr = await handler.call_tool("create_pull_request", {
                "owner": ctx.owner,
                "repo": ctx.repo,
                "title": f"Fix issue #{ctx.issue_number}",
                "head": ctx.branch_name,
                "base": "main"
            })
            ctx.pr_number = pr["number"]
            ctx.state = WorkflowState.PR_CREATED
            print(f"State: {ctx.state}")
        
        return ctx
    
    except Exception as e:
        ctx.state = WorkflowState.FAILED
        ctx.error = str(e)
        print(f"Workflow failed at state {ctx.state}: {e}")
        return ctx

# Usage with resumability
ctx = WorkflowContext(issue_number=123, owner="myorg", repo="myrepo")
ctx = await stateful_workflow(client, ctx)

# If failed, can resume from last successful state
if ctx.state == WorkflowState.FAILED:
    print(f"Resuming from state: {ctx.state}")
    ctx = await stateful_workflow(client, ctx)
```

### Monitoring and Observability

```python
import time
from contextlib import asynccontextmanager

class MCPMetrics:
    """Track MCP operation metrics."""
    
    def __init__(self):
        self.call_count = 0
        self.error_count = 0
        self.total_duration = 0.0
    
    @asynccontextmanager
    async def track_call(self, tool_name: str):
        """Context manager to track tool call metrics."""
        start = time.time()
        self.call_count += 1
        
        try:
            yield
        except Exception as e:
            self.error_count += 1
            raise
        finally:
            duration = time.time() - start
            self.total_duration += duration
            print(f"Tool {tool_name} took {duration:.2f}s")
    
    def report(self):
        """Generate metrics report."""
        avg_duration = self.total_duration / self.call_count if self.call_count > 0 else 0
        error_rate = self.error_count / self.call_count if self.call_count > 0 else 0
        
        return {
            "total_calls": self.call_count,
            "total_errors": self.error_count,
            "error_rate": f"{error_rate:.2%}",
            "avg_duration": f"{avg_duration:.2f}s",
            "total_duration": f"{self.total_duration:.2f}s"
        }

# Usage
metrics = MCPMetrics()

async def monitored_call(client: MCPClient, tool_name: str, params: dict):
    """Call a tool with metrics tracking."""
    async with metrics.track_call(tool_name):
        return await client.call_tool(tool_name, params)

# After workflow completion
print("Metrics:", metrics.report())
```

## Testing Your Integration

### Unit Testing with Mock MCP Server

```python
import pytest
from unittest.mock import AsyncMock, MagicMock

@pytest.fixture
def mock_mcp_client():
    """Create a mock MCP client for testing."""
    client = MagicMock()
    client.call_tool = AsyncMock()
    return client

@pytest.mark.asyncio
async def test_create_feature_branch(mock_mcp_client):
    """Test feature branch creation."""
    mock_mcp_client.call_tool.return_value = {
        "ref": "refs/heads/feature/test",
        "sha": "abc123"
    }
    
    branch = await create_feature_branch(
        mock_mcp_client,
        "myorg",
        "myrepo",
        "test"
    )
    
    assert branch == "feature/test"
    mock_mcp_client.call_tool.assert_called_once_with(
        "create_branch",
        {
            "owner": "myorg",
            "repo": "myrepo",
            "branch": "feature/test",
            "from_branch": "main"
        }
    )
```

### Integration Testing

```python
import os
import pytest

@pytest.mark.integration
@pytest.mark.asyncio
async def test_real_mcp_connection():
    """Test connection to real MCP server (requires credentials)."""
    mcp_url = os.getenv("MCP_SERVER_URL")
    mcp_token = os.getenv("MCP_API_TOKEN")
    
    if not mcp_url or not mcp_token:
        pytest.skip("MCP credentials not configured")
    
    client = MCPClient(
        url=mcp_url,
        headers={"Authorization": f"Bearer {mcp_token}"}
    )
    
    # Test basic connectivity
    result = await client.call_tool("get_me", {})
    assert result is not None
    assert "login" in result

@pytest.mark.integration
@pytest.mark.asyncio
async def test_end_to_end_workflow():
    """Test complete workflow against test repository."""
    # This test requires a dedicated test repository
    client = MCPClient(
        url=os.getenv("MCP_SERVER_URL"),
        headers={"Authorization": f"Bearer {os.getenv('MCP_API_TOKEN')}"}
    )
    
    test_owner = "myorg"
    test_repo = "test-repo"
    test_branch = f"test/integration-{int(time.time())}"
    
    try:
        # Create branch
        await client.call_tool("create_branch", {
            "owner": test_owner,
            "repo": test_repo,
            "branch": test_branch,
            "from_branch": "main"
        })
        
        # Commit file
        await client.call_tool("create_or_update_file", {
            "owner": test_owner,
            "repo": test_repo,
            "path": "test.txt",
            "content": "Integration test",
            "message": "Test commit",
            "branch": test_branch
        })
        
        # Create PR
        pr = await client.call_tool("create_pull_request", {
            "owner": test_owner,
            "repo": test_repo,
            "title": "Integration test PR",
            "head": test_branch,
            "base": "main"
        })
        
        assert pr["number"] > 0
        
    finally:
        # Cleanup: close PR if created
        pass
```

## Troubleshooting

### Common Issues

#### 1. Authentication Failed (401)

**Symptom**: `MCPError: Authentication failed`

**Solutions**:
- Verify MCP API token is correct
- Check token is properly configured in Railway environment
- Ensure token mapping includes your token
- Verify Authorization header format: `Bearer <token>`

#### 2. Permission Denied (403)

**Symptom**: `MCPError: Permission denied`

**Solutions**:
- Check GitHub App has required permissions
- Verify App is installed on target repository
- Review required scopes in error message
- Update GitHub App permissions in GitHub settings

#### 3. Rate Limit Exceeded

**Symptom**: `MCPError: Rate limit exceeded`

**Solutions**:
- Implement retry logic with exponential backoff
- Use `retry_after` value from error response
- Consider caching frequently accessed data
- Spread operations over time

#### 4. Connection Timeout

**Symptom**: `TimeoutError` or connection errors

**Solutions**:
- Increase timeout in client configuration
- Check Railway service is running (health check)
- Verify network connectivity
- Check Railway logs for server errors

#### 5. Invalid Tool Name

**Symptom**: `MCPError: Tool not found`

**Solutions**:
- List available tools: `await client.list_tools()`
- Check tool name spelling
- Verify toolset is enabled in Railway configuration
- Review MCP server logs

### Debugging Tips

#### Enable Debug Logging

```python
import logging

# Enable debug logging for MCP client
logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger("langgraph.mcp")
logger.setLevel(logging.DEBUG)
```

#### Inspect Available Tools

```python
async def list_available_tools(client: MCPClient):
    """List all available tools from MCP server."""
    tools = await client.list_tools()
    for tool in tools:
        print(f"Tool: {tool['name']}")
        print(f"  Description: {tool.get('description', 'N/A')}")
        print(f"  Parameters: {tool.get('inputSchema', {})}")
        print()

# Usage
await list_available_tools(client)
```

#### Test Connection

```python
async def test_connection(client: MCPClient):
    """Test MCP server connection and authentication."""
    try:
        result = await client.call_tool("get_me", {})
        print(f"✓ Connected as: {result.get('login', 'unknown')}")
        return True
    except Exception as e:
        print(f"✗ Connection failed: {e}")
        return False

# Usage
if not await test_connection(client):
    print("Please check your configuration")
```

## Best Practices

### 1. Always Use Error Handling

Never call MCP tools without proper error handling. Use try-except blocks or the `MCPErrorHandler` class.

### 2. Implement Retry Logic

GitHub API operations can fail due to transient issues. Always implement retry logic with exponential backoff.

### 3. Use Structured Logging

Log all operations with structured data for easier debugging and monitoring.

```python
import logging
import json

logger = logging.getLogger(__name__)

async def logged_tool_call(client: MCPClient, tool_name: str, params: dict):
    """Call tool with structured logging."""
    logger.info(json.dumps({
        "event": "mcp_call_start",
        "tool": tool_name,
        "params": params
    }))
    
    try:
        result = await client.call_tool(tool_name, params)
        logger.info(json.dumps({
            "event": "mcp_call_success",
            "tool": tool_name
        }))
        return result
    except Exception as e:
        logger.error(json.dumps({
            "event": "mcp_call_error",
            "tool": tool_name,
            "error": str(e)
        }))
        raise
```

### 4. Validate Inputs

Always validate inputs before making API calls to catch errors early.

```python
def validate_repo_params(owner: str, repo: str):
    """Validate repository parameters."""
    if not owner or not owner.strip():
        raise ValueError("Owner cannot be empty")
    if not repo or not repo.strip():
        raise ValueError("Repo cannot be empty")
    if "/" in owner or "/" in repo:
        raise ValueError("Owner and repo cannot contain '/'")

async def safe_create_branch(client: MCPClient, owner: str, repo: str, branch: str):
    """Create branch with input validation."""
    validate_repo_params(owner, repo)
    
    if not branch or not branch.strip():
        raise ValueError("Branch name cannot be empty")
    
    return await client.call_tool("create_branch", {
        "owner": owner,
        "repo": repo,
        "branch": branch,
        "from_branch": "main"
    })
```

### 5. Use Timeouts

Always set reasonable timeouts to prevent hanging operations.

```python
import asyncio

async def call_with_timeout(
    client: MCPClient,
    tool_name: str,
    params: dict,
    timeout: int = 30
):
    """Call tool with timeout."""
    try:
        return await asyncio.wait_for(
            client.call_tool(tool_name, params),
            timeout=timeout
        )
    except asyncio.TimeoutError:
        raise RuntimeError(f"Tool call timed out after {timeout}s")
```

### 6. Implement Idempotency

Design workflows to be idempotent where possible.

```python
async def idempotent_create_branch(
    client: MCPClient,
    owner: str,
    repo: str,
    branch: str
):
    """Create branch idempotently (doesn't fail if exists)."""
    try:
        return await client.call_tool("create_branch", {
            "owner": owner,
            "repo": repo,
            "branch": branch,
            "from_branch": "main"
        })
    except MCPError as e:
        if "already exists" in str(e).lower():
            print(f"Branch {branch} already exists, continuing...")
            return {"ref": f"refs/heads/{branch}"}
        raise
```

## Performance Optimization

### Connection Pooling

```python
from langgraph.mcp import MCPClient
import httpx

# Create a shared HTTP client with connection pooling
http_client = httpx.AsyncClient(
    limits=httpx.Limits(
        max_connections=20,
        max_keepalive_connections=10
    )
)

client = MCPClient(
    url=MCP_SERVER_URL,
    headers={"Authorization": f"Bearer {MCP_API_TOKEN}"},
    http_client=http_client
)

# Remember to close the client when done
await http_client.aclose()
```

### Batch Operations

When possible, use batch operations to reduce API calls:

```python
# Instead of multiple single-file commits
for file in files:
    await client.call_tool("create_or_update_file", {...})

# Use push_files for multiple files
await client.call_tool("push_files", {
    "owner": owner,
    "repo": repo,
    "branch": branch,
    "files": files,
    "message": "Batch commit"
})
```

## Additional Resources

- [MCP Protocol Specification](https://modelcontextprotocol.io)
- [GitHub REST API Documentation](https://docs.github.com/en/rest)
- [GitHub GraphQL API Documentation](https://docs.github.com/en/graphql)
- [Railway Documentation](https://docs.railway.app)
- [LangGraph Documentation](https://langchain-ai.github.io/langgraph/)

## Support

For issues or questions:
- Check Railway logs for server-side errors
- Review GitHub App permissions
- Verify environment variable configuration
- Open an issue in the repository
