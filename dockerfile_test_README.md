# Docker Cache Mount Format Property Test

## Overview

This property-based test validates that all Docker cache mount directives in the Dockerfile follow the required format specified in Requirements 3.2.

## Property Being Tested

**Property 3: Docker build cache format compliance**

*For any* Docker build operation, all cache mount directives should use the format `--mount=type=cache,id=<cache-id>`

**Validates: Requirements 3.2**

## Test Structure

The test file `dockerfile_test.go` contains three main test functions:

### 1. TestDockerCacheMountFormatCompliance

This is the main property test that:
- Reads the actual Dockerfile
- Extracts all cache mount directives
- Verifies each one follows the required format: `--mount=type=cache,id=<cache-id>[,other-params]`
- Runs 100 iterations to ensure consistency

### 2. TestCacheMountFormatValidation

This property test validates the format checker itself by:
- Generating random valid cache mount strings and verifying they pass validation
- Testing that cache mounts without `id` parameter are correctly rejected
- Testing that mounts with `type=bind` instead of `type=cache` are correctly rejected

## Required Format

A valid cache mount must:
1. Start with `--mount=`
2. Include `type=cache` parameter
3. Include `id=<cache-id>` parameter where cache-id is non-empty
4. May include additional parameters like `target=`

### Valid Examples
```dockerfile
--mount=type=cache,id=apk-cache,target=/var/cache/apk
--mount=type=cache,id=go-mod,target=/go/pkg/mod
--mount=type=cache,id=go-build,target=/root/.cache/go-build
```

### Invalid Examples
```dockerfile
--mount=type=cache,target=/var/cache/apk          # Missing id parameter
--mount=type=bind,id=my-cache,target=/cache       # Wrong type (bind instead of cache)
--mount=id=my-cache,target=/cache                 # Missing type parameter
```

## Running the Tests

To run the property tests:

```bash
go test -v -run TestDockerCacheMountFormatCompliance
go test -v -run TestCacheMountFormatValidation
```

To run all tests in the file:

```bash
go test -v dockerfile_test.go
```

## Dependencies

- `github.com/leanovate/gopter` - Property-based testing library for Go
- Go standard library (`testing`, `os`, `bufio`, `strings`, `regexp`)

## Implementation Details

### extractCacheMounts(dockerfileContent string) []string

Parses the Dockerfile content and extracts all `--mount` directives that contain `type=cache`. Handles multi-line directives with backslash continuations.

### isValidCacheMountFormat(mount string) bool

Validates that a cache mount directive:
1. Starts with `--mount=`
2. Contains `type=cache` parameter
3. Contains `id=<non-empty-value>` parameter

Returns `true` if valid, `false` otherwise.

### extractMountDirectives(line string) []string

Uses regex to extract all `--mount=` directives from a single line, handling cases where multiple mount directives appear on the same line.

## Test Configuration

- **Minimum successful tests**: 100 iterations per property
- **Test framework**: gopter (Go property testing)
- **Reporter**: Default gopter reporter with test output
