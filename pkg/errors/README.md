# Error Handling Package

This package provides comprehensive, structured error handling for the GitHub MCP server Railway deployment.

## Overview

The error handling system provides:
- Structured error responses with type, message, and actionable details
- JSON-RPC 2.0 compliant error formatting
- HTTP error response helpers
- Specific handlers for common error scenarios (rate limits, authentication, permissions, GitHub API errors)

## Components

### 1. Error Response Structure (`response.go`)

Defines the core error response types:

- `ErrorResponse`: Base structured error with type, message, and optional data
- `JSONRPCError`: JSON-RPC 2.0 compliant error format
- `HTTPErrorResponse`: HTTP wrapper for error responses
- `ErrorType`: Enumeration of error categories
- `JSONRPCErrorCode`: Standard JSON-RPC error codes

**Error Types:**
- `ErrorTypeAuthentication`: Authentication failures
- `ErrorTypeAuthorization`: Permission/authorization errors
- `ErrorTypeRateLimit`: Rate limit exceeded
- `ErrorTypeNotFound`: Resource not found
- `ErrorTypeValidation`: Validation errors
- `ErrorTypeGitHubAPI`: GitHub API errors
- `ErrorTypeInternal`: Internal server errors
- `ErrorTypeMCPProtocol`: MCP protocol errors
- `ErrorTypeMethodNotAllowed`: HTTP method not allowed
- `ErrorTypeMissingAuthorization`: Missing authorization header
- `ErrorTypeInvalidToken`: Invalid token

### 2. Specific Error Handlers (`handlers.go`)

Provides specialized error handlers for common scenarios:

#### Rate Limit Errors
```go
errResp := NewRateLimitError(limit, remaining, resetAt)
// or
errResp := NewRateLimitErrorFromGitHub(githubResponse)
```

Returns structured error with:
- `limit`: Rate limit cap
- `remaining`: Remaining requests
- `reset_at`: When the limit resets (RFC3339 format)
- `retry_after`: Seconds until reset

#### Permission Denied Errors
```go
errResp := NewPermissionDeniedError(message, requiredScopes, resource)
```

Returns structured error with:
- `message`: Error description
- `required_scopes`: Array of required OAuth scopes (if applicable)
- `resource`: Resource that was denied access

#### Authentication Errors
```go
errResp := NewAuthenticationError(message, hint)
```

Returns structured error with:
- `message`: Error description
- `hint`: Helpful hint for resolving the issue

#### GitHub API Errors
```go
errResp := NewGitHubAPIErrorFromResponse(githubResponse, err)
```

Automatically detects error type from GitHub response and returns appropriate structured error with:
- `status_code`: HTTP status code
- `message`: Error message
- `documentation_url`: Link to GitHub docs (if available)
- `errors`: Field-specific errors (if available)

### 3. HTTP Response Helpers

Write errors directly to HTTP responses:

```go
WriteRateLimitError(w, limit, remaining, resetAt)
WritePermissionDeniedError(w, message, scopes, resource)
WriteAuthenticationError(w, message, hint)
WriteGitHubAPIError(w, githubResponse, err)
```

## Usage Examples

### Basic Error Response
```go
import "github.com/github/github-mcp-server/pkg/errors"

// Create a structured error
errResp := errors.NewErrorResponse(
    errors.ErrorTypeValidation,
    "Invalid input parameters",
    map[string]interface{}{
        "field": "title",
        "issue": "cannot be empty",
    },
)

// Write to HTTP response
errors.WriteHTTPError(w, http.StatusBadRequest, 
    errResp.Type, errResp.Message, errResp.Data)
```

### Rate Limit Error
```go
// From GitHub response
if resp.StatusCode == 429 {
    errResp := errors.NewRateLimitErrorFromGitHub(resp)
    errors.WriteHTTPError(w, http.StatusTooManyRequests,
        errResp.Type, errResp.Message, errResp.Data)
}
```

### Authentication Error
```go
if token == "" {
    errors.WriteAuthenticationError(w,
        "Missing authentication token",
        "Include Authorization: Bearer <token> header")
}
```

### GitHub API Error Passthrough
```go
// Automatically handles rate limits, permissions, auth, etc.
if err != nil {
    errors.WriteGitHubAPIError(w, resp, err)
}
```

### JSON-RPC Error
```go
rpcErr := errors.NewJSONRPCError(
    requestID,
    errors.JSONRPCInvalidParams,
    "Invalid parameters",
    map[string]interface{}{
        "expected": "string",
        "received": "number",
    },
)

json.NewEncoder(w).Encode(rpcErr)
```

## Error Response Format

All errors follow this structure:

```json
{
  "error": {
    "type": "rate_limit_exceeded",
    "message": "GitHub API rate limit exceeded. Limit: 5000, Remaining: 0. Resets at 2024-12-08T18:00:00Z",
    "data": {
      "limit": 5000,
      "remaining": 0,
      "reset_at": "2024-12-08T18:00:00Z",
      "retry_after": 3600
    }
  }
}
```

## Testing

### Unit Tests
Run unit tests for error handling:
```bash
go test ./pkg/errors -v -run TestNew
go test ./pkg/errors -v -run TestWrite
go test ./pkg/errors -v -run TestMap
```

### Property-Based Tests
Run property tests to verify error structure correctness:
```bash
go test ./pkg/errors -v -run TestProperty_ErrorInformationStructure
```

The property test validates:
- All error responses have required structure (type, message)
- Rate limit errors contain actionable retry information
- Permission errors contain scope information when provided
- Authentication errors contain helpful hints
- All error types map to valid HTTP status codes
- All error types map to valid JSON-RPC error codes
- JSON-RPC errors are properly formatted

### Run All Tests
```bash
go test ./pkg/errors -v
```

## Design Validation

This implementation validates the following requirements:

- **Requirement 5.1**: Errors are logged with structured information
- **Requirement 5.2**: Rate limit information is included in error responses
- **Requirement 5.4**: Authentication failures return clear error messages
- **Requirement 8.7**: Structured error information includes type, message, and actionable details

## Property Test Coverage

**Property 10: Error information structure**
- Validates that all error conditions return structured information
- Ensures error responses are JSON serializable
- Verifies actionable details are included for rate limits, permissions, and authentication
- Confirms error type and HTTP status code mappings are valid
