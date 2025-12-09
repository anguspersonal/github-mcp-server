# Implementation Plan

- [x] 1. Codebase cleanup and simplification





  - Remove all unnecessary files and directories not needed for Railway deployment
  - Keep only mini-mcp-http server and core GitHub integration
  - _Requirements: 9.2, 9.3, 9.4, 9.5_

- [x] 1.1 Remove unused command-line tools


  - Delete `cmd/github-mcp-server/` directory (stdio version)
  - Delete `cmd/mcpcurl/` directory (testing utility)
  - Delete `cmd/mini-mcp/` directory (stdio mini version)
  - _Requirements: 9.2, 9.5_

- [x] 1.2 Remove unnecessary documentation


  - Delete `docs/installation-guides/` directory (IDE-specific guides)
  - Delete `docs/remote-server.md` (GitHub-hosted version)
  - Delete `docs/server-configuration.md` (stdio configuration)
  - _Requirements: 9.4, 9.7_

- [x] 1.3 Remove unused packages and utilities


  - Delete `internal/profiler/` directory (development tool)
  - Delete `internal/toolsnaps/` directory (testing utility)
  - Delete `script/` directory (build scripts)
  - Delete `.goreleaser.yaml` (multi-platform releases)
  - Delete `third-party-licenses.*.md` files
  - _Requirements: 9.5_

- [x] 1.4 Clean up dependencies


  - Run `go mod tidy` to remove unused dependencies
  - Verify all remaining dependencies are used by mini-mcp-http
  - Update `go.mod` if needed
  - _Requirements: 9.6_

- [x] 2. Update Dockerfile for Railway deployment





  - Modify Dockerfile to build only mini-mcp-http
  - Ensure cache mounts use correct format: `--mount=type=cache,id=<cache-id>`
  - Optimize for minimal image size
  - _Requirements: 3.1, 3.2, 3.3, 3.4_


- [x] 2.1 Implement Dockerfile build stage

  - Update FROM and WORKDIR directives
  - Add cache mounts with proper ID format for apk, go mod, and go build
  - Build mini-mcp-http binary with version ldflags
  - _Requirements: 3.2, 3.3_

- [x] 2.2 Implement Dockerfile runtime stage


  - Use distroless base image
  - Copy mini-mcp-http binary
  - Set correct ENTRYPOINT and CMD
  - _Requirements: 3.3, 3.4_

- [x] 2.3 Write property test for Docker cache mount format







  - **Property 3: Docker build cache format compliance**
  - **Validates: Requirements 3.2**

- [x] 3. Create Railway configuration files





  - Create `railway.json` with service configuration
  - Configure build and deployment settings
  - Set up health check endpoint
  - _Requirements: 7.1, 7.2, 7.3_


- [x] 3.1 Implement railway.json configuration

  - Define build configuration (Dockerfile builder)
  - Define deploy configuration (start command, health check)
  - Configure restart policy
  - _Requirements: 7.1, 7.2, 7.5_

- [x] 3.2 Add health check endpoint to mini-mcp-http


  - Implement `/health` endpoint handler
  - Return JSON with status, version, uptime
  - Check GitHub API reachability
  - _Requirements: 1.5_

- [x] 3.3 Write unit tests for health check endpoint






  - Test health check returns 200 OK
  - Test health check JSON structure
  - Test health check with GitHub API unreachable
  - _Requirements: 1.5_

- [ ] 4. Enhance token store for GitHub App support

  - Verify InstallationTokenStore implementation
  - Add token caching with expiry
  - Implement JWT creation for GitHub App
  - _Requirements: 2.1, 2.2, 2.3_

- [x] 4.1 Review and enhance InstallationTokenStore



  - Verify JWT creation logic
  - Verify installation token minting
  - Verify token caching with expiry
  - Add logging for token operations
  - _Requirements: 2.1, 2.2, 2.3_

- [x] 4.2 Write property test for token resolution






  - **Property 5: Token resolution correctness**
  - **Validates: Requirements 2.4, 4.5**

- [x] 4.3 Write property test for token caching






  - **Property 6: Installation token caching**
  - **Validates: Requirements 2.5**

- [x] 4.4 Write unit tests for token store








  - Test EnvTokenStore mapping resolution
  - Test InstallationTokenStore JWT creation
  - Test private key parsing (PKCS1, PKCS8)
  - Test token expiry handling
  - _Requirements: 2.1, 2.2, 2.3_

- [x] 5. Update mini-mcp-http server for Railway





  - Ensure server reads PORT from environment
  - Verify Bearer token extraction and validation
  - Verify token resolution via TokenStore
  - Add structured logging for Railway
  - _Requirements: 1.1, 1.4, 4.1, 4.5, 5.1_

- [x] 5.1 Implement PORT environment variable support


  - Read PORT from environment with fallback to 8080
  - Update listen address configuration
  - _Requirements: 1.1, 1.4_

- [x] 5.2 Enhance authentication and error handling


  - Verify Bearer token extraction logic
  - Add clear error messages for missing/invalid tokens
  - Implement structured error responses
  - _Requirements: 4.5, 5.4_

- [x] 5.3 Add structured logging


  - Implement JSON logging for Railway
  - Log server startup, connections, errors
  - Include request IDs for tracing
  - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [x] 5.4 Write property test for MCP protocol handling



  - **Property 4: MCP protocol message handling**
  - **Validates: Requirements 4.2, 4.3**

- [x] 5.5 Write property test for concurrent requests



  - **Property 9: Concurrent request handling**
  - **Validates: Requirements 8.5**

- [x] 5.6 Write unit tests for HTTP handlers



  - Test Bearer token extraction
  - Test Authorization header validation
  - Test HTTP method validation (POST only)
  - Test response header configuration
  - _Requirements: 4.1, 4.5_

- [x] 6. Implement environment variable validation




  - Add startup validation for required env vars
  - Provide clear error messages for missing configuration
  - Set sensible defaults for optional variables
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

- [x] 6.1 Implement configuration validation


  - Check for required env vars (GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY_B64, GITHUB_MCP_TOKEN_MAP)
  - Validate token mapping JSON format
  - Validate GitHub App credentials format
  - _Requirements: 6.2, 6.4, 6.5_

- [x] 6.2 Add default values for optional configuration


  - Set default PORT to 8080
  - Set default GITHUB_HOST to github.com
  - Set default toolsets if not specified
  - _Requirements: 6.3_

- [x] 6.3 Write property test for environment validation



  - **Property 7: Environment variable validation**
  - **Validates: Requirements 6.2**

- [x] 6.4 Write unit tests for configuration validation



  - Test missing required env vars fail startup
  - Test invalid JSON in token mapping fails startup
  - Test invalid GitHub App credentials fail startup
  - Test default values are applied correctly
  - _Requirements: 6.2, 6.3, 6.4, 6.5_

- [x] 7. Verify GitHub API toolset coverage for autonomous workflow





  - Ensure all required tools are available for backlog-to-PR workflow
  - Verify issue querying tools
  - Verify branch creation tools
  - Verify file commit tools
  - Verify PR creation tools
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.6_


- [x] 7.1 Audit and verify toolset coverage

  - Check `repos` toolset for branch and file operations
  - Check `issues` toolset for backlog item access
  - Check `pull_requests` toolset for PR creation
  - Check `actions` toolset for workflow status
  - Check `git` toolset for low-level operations
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.6_

- [x] 7.2 Write property test for workflow completeness



  - **Property 8: Backlog-to-PR workflow completeness**
  - **Validates: Requirements 8.1, 8.2, 8.3, 8.4**

- [x] 8. Implement comprehensive error handling





  - Add structured error responses for all error types
  - Include error type, message, and actionable details
  - Implement rate limit error handling
  - Implement authentication error handling

  - _Requirements: 5.1, 5.2, 5.4, 8.7_

- [x] 8.1 Implement error response structure

  - Create error response type with type, message, data fields
  - Implement JSON-RPC error format
  - Add error code mapping
  - _Requirements: 5.1, 5.4, 8.7_

- [x] 8.2 Add specific error handlers


  - Implement rate limit error with retry_after
  - Implement permission denied error with required scopes
  - Implement authentication error with clear message
  - Implement GitHub API error passthrough
  - _Requirements: 5.2, 5.4, 8.7_

- [x] 8.3 Write property test for error structure



  - **Property 10: Error information structure**
  - **Validates: Requirements 5.1, 5.2, 5.4, 8.7**

- [x] 8.4 Write unit tests for error handling





  - Test rate limit error format
  - Test authentication error format
  - Test permission denied error format
  - Test GitHub API error passthrough
  - _Requirements: 5.1, 5.2, 5.4, 8.7_

- [x] 9. Update README for Railway deployment





  - Rewrite README to focus on Railway deployment
  - Document GitHub App setup process
  - Document environment variable configuration
  - Document LangGraph integration
  - Remove references to other deployment methods
  - _Requirements: 9.7_


- [x] 9.1 Write Railway deployment documentation

  - Document Railway project setup
  - Document GitHub App creation and configuration
  - Document environment variable setup in Railway
  - Document deployment process
  - _Requirements: 9.7_


- [x] 9.2 Write LangGraph integration documentation









  - Document MCP client configuration for LangGraph
  - Provide example workflow code
  - Document error handling patterns
  - Document token management
  - _Requirements: 9.7_


- [x] 9.3 Write troubleshooting guide





  - Document common errors and solutions
  - Document GitHub App permission issues
  - Document token resolution issues
  - Document Railway deployment issues
  - _Requirements: 9.7_

- [x] 10. Checkpoint - Ensure all tests pass







  - Ensure all tests pass, ask the user if questions arise.


- [-] 11. Create deployment verification script


  - Create script to verify Railway deployment
  - Test health check endpoint
  - Test MCP protocol connection
  - Test GitHub API access
  - _Requirements: 1.1, 1.2, 1.3_


- [-] 11.1 Implement deployment verification script




  - Write script to check Railway service status
  - Test health check endpoint returns 200
  - Test MCP connection with test token
  - Test GitHub API call through MCP
  - _Requirements: 1.1, 1.2, 1.3_

- [ ] 11.2 Write property test for service availability


  - **Property 1: Service availability on Railway**
  - **Validates: Requirements 1.1, 1.2**

- [ ] 11.3 Write property test for GitHub App authentication


  - **Property 2: GitHub App authentication success**
  - **Validates: Requirements 2.1, 2.2, 2.3**

- [ ] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
