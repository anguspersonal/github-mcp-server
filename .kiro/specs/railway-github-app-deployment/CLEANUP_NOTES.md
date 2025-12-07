# Codebase Cleanup Notes

## Completed Tasks

### 1.1 Remove unused command-line tools ✓
- Deleted `cmd/github-mcp-server/` directory (stdio version)
- Deleted `cmd/mcpcurl/` directory (testing utility)
- Deleted `cmd/mini-mcp/` directory (stdio mini version)

### 1.2 Remove unnecessary documentation ✓
- Deleted `docs/installation-guides/` directory (IDE-specific guides)
- Deleted `docs/remote-server.md` (GitHub-hosted version)
- Deleted `docs/server-configuration.md` (stdio configuration)

### 1.3 Remove unused packages and utilities ✓
- Deleted `internal/profiler/` directory (development tool)
- Deleted `internal/toolsnaps/` directory (testing utility)
- Deleted `script/` directory (build scripts)
- Deleted `.goreleaser.yaml` (multi-platform releases)
- Deleted `third-party-licenses.*.md` files (all platforms)

### 1.4 Clean up dependencies (Pending Go installation)

**Action Required**: Run the following command when Go is available:

```bash
go mod tidy
```

This will:
- Remove unused dependencies from go.mod
- Update go.sum accordingly
- Verify all remaining dependencies are used by mini-mcp-http

**Core Dependencies Verified**:
The following dependencies are essential for mini-mcp-http and should remain:
- `github.com/modelcontextprotocol/go-sdk` - MCP protocol implementation
- `github.com/google/go-github/v79/github` - GitHub REST API client
- `github.com/shurcooL/githubv4` - GitHub GraphQL client
- Standard library packages (crypto, encoding, net/http, etc.)

**Dependencies Used by Core Packages**:
- `github.com/microcosm-cc/bluemonday` - HTML sanitization (used by pkg/github)
- `github.com/muesli/cache2go` - Caching (used by pkg/lockdown)
- `github.com/spf13/viper` - Configuration (used by internal/ghmcp)
- `github.com/google/jsonschema-go` - JSON schema validation (used by pkg/github)

**Potentially Unused Dependencies** (to be removed by go mod tidy):
- `github.com/spf13/cobra` - CLI framework (only used by removed cmd tools)
- `github.com/josephburnett/jd` - JSON diff (may be unused)
- `github.com/migueleliasweb/go-github-mock` - Testing mock (dev dependency)
- `github.com/stretchr/testify` - Testing framework (dev dependency, should remain)

## Summary

All file and directory cleanup tasks have been completed successfully. The codebase now contains only:
- `cmd/mini-mcp-http/` - The HTTP server for Railway deployment
- `internal/ghmcp/` - Core MCP server and token store
- `pkg/github/` - GitHub API integration and toolsets
- Supporting packages: `pkg/errors/`, `pkg/raw/`, `pkg/lockdown/`, `pkg/translations/`, `pkg/utils/`
- Essential documentation: `README.md`, `docs/error-handling.md`, `docs/testing.md`
- Configuration files: `Dockerfile`, `go.mod`, `go.sum`, `.gitignore`, `.dockerignore`

The final step (running `go mod tidy`) should be executed in an environment with Go installed to complete the dependency cleanup.
