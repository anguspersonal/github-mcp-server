package main

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: railway-github-app-deployment, Property 3: Docker build cache format compliance
// Validates: Requirements 3.2
// Property: For any Docker build operation, all cache mount directives should use the format --mount=type=cache,id=<cache-id>
func TestDockerCacheMountFormatCompliance(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Read the Dockerfile
	dockerfileContent, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}

	// Property: All cache mount directives must follow the correct format
	properties.Property("all cache mounts use correct format with id parameter", prop.ForAll(
		func() bool {
			// Parse Dockerfile and extract all cache mount directives
			cacheMounts := extractCacheMounts(string(dockerfileContent))
			
			if len(cacheMounts) == 0 {
				return false
			}

			// Verify each cache mount has the correct format
			for _, mount := range cacheMounts {
				if !isValidCacheMountFormat(mount) {
					return false
				}
			}

			return true
		},
	))

	properties.TestingRun(t)
}

// extractCacheMounts finds all --mount directives with type=cache in the Dockerfile
func extractCacheMounts(dockerfileContent string) []string {
	var cacheMounts []string
	scanner := bufio.NewScanner(strings.NewReader(dockerfileContent))
	
	var currentLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Handle line continuations with backslash
		if strings.HasSuffix(currentLine, "\\") {
			currentLine = strings.TrimSuffix(currentLine, "\\") + " " + line
			continue
		} else if currentLine != "" {
			currentLine += " " + line
		} else {
			currentLine = line
		}
		
		// Process complete line
		if !strings.HasSuffix(currentLine, "\\") && currentLine != "" {
			// Look for --mount directives with type=cache
			if strings.Contains(currentLine, "--mount=") && strings.Contains(currentLine, "type=cache") {
				// Extract all --mount directives from the line
				mounts := extractMountDirectives(currentLine)
				for _, mount := range mounts {
					if strings.Contains(mount, "type=cache") {
						cacheMounts = append(cacheMounts, mount)
					}
				}
			}
			currentLine = ""
		}
	}
	
	return cacheMounts
}

// extractMountDirectives extracts individual --mount directives from a line
func extractMountDirectives(line string) []string {
	var mounts []string
	
	// Find all --mount= occurrences
	re := regexp.MustCompile(`--mount=[^\s]+`)
	matches := re.FindAllString(line, -1)
	
	return append(mounts, matches...)
}

// isValidCacheMountFormat checks if a cache mount follows the required format:
// --mount=type=cache,id=<cache-id>[,other-params]
func isValidCacheMountFormat(mount string) bool {
	// Must start with --mount=
	if !strings.HasPrefix(mount, "--mount=") {
		return false
	}
	
	// Remove --mount= prefix
	params := strings.TrimPrefix(mount, "--mount=")
	
	// Split by comma to get individual parameters
	parts := strings.Split(params, ",")
	
	hasTypeCache := false
	hasId := false
	
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		
		if key == "type" && value == "cache" {
			hasTypeCache = true
		}
		if key == "id" && value != "" {
			hasId = true
		}
	}
	
	// Must have both type=cache and id=<something>
	return hasTypeCache && hasId
}

// Property test: Generate random Dockerfile-like content and verify cache mount validation
func TestCacheMountFormatValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("valid cache mounts are recognized", prop.ForAll(
		func(cacheId string) bool {
			// Generate a valid cache mount
			validMount := "--mount=type=cache,id=" + cacheId + ",target=/some/path"
			return isValidCacheMountFormat(validMount)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.Property("cache mounts without id are invalid", prop.ForAll(
		func() bool {
			invalidMount := "--mount=type=cache,target=/some/path"
			return !isValidCacheMountFormat(invalidMount)
		},
	))

	properties.Property("cache mounts without type=cache are invalid", prop.ForAll(
		func(cacheId string) bool {
			invalidMount := "--mount=type=bind,id=" + cacheId + ",target=/some/path"
			return !isValidCacheMountFormat(invalidMount)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}
