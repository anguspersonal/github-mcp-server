package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v79/github"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: railway-github-app-deployment, Property 10: Error information structure**
// **Validates: Requirements 5.1, 5.2, 5.4, 8.7**
//
// Property: For any error condition (API failure, auth failure, rate limit), the system
// should return structured error information that includes error type, message, and
// actionable details
func TestProperty_ErrorInformationStructure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for HTTP status codes that represent errors
	genErrorStatusCode := gen.OneConstOf(
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	)

	// Generator for error messages
	genErrorMessage := gen.OneConstOf(
		"Authentication failed",
		"Permission denied",
		"Rate limit exceeded",
		"Resource not found",
		"Validation failed",
		"Internal server error",
	)

	// Generator for rate limit data (unused but kept for potential future use)
	_ = gen.Struct(reflect.TypeOf(struct {
		Limit     int
		Remaining int
		ResetTime time.Time
	}{}), map[string]gopter.Gen{
		"Limit":     gen.IntRange(1000, 10000),
		"Remaining": gen.IntRange(0, 1000),
		"ResetTime": gen.TimeRange(time.Now(), 24*time.Hour),
	})

	properties.Property("All error responses have required structure", prop.ForAll(
		func(statusCode int, message string) bool {
			// Create a GitHub response with the given status code
			resp := &gogithub.Response{
				Response: &http.Response{StatusCode: statusCode},
			}

			// For rate limit errors, add rate limit info
			if statusCode == http.StatusTooManyRequests || statusCode == http.StatusForbidden {
				resp.Rate = gogithub.Rate{
					Limit:     5000,
					Remaining: 0,
					Reset:     gogithub.Timestamp{Time: time.Now().Add(1 * time.Hour)},
				}
			}

			// Create error response
			errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("%s", message))

			// Verify error response has required fields
			if errResp == nil {
				return false
			}

			// 1. Must have error type
			if errResp.Type == "" {
				return false
			}

			// 2. Must have message
			if errResp.Message == "" {
				return false
			}

			// 3. Error type must be valid
			validTypes := map[ErrorType]bool{
				ErrorTypeAuthentication:       true,
				ErrorTypeAuthorization:        true,
				ErrorTypeRateLimit:            true,
				ErrorTypeNotFound:             true,
				ErrorTypeValidation:           true,
				ErrorTypeGitHubAPI:            true,
				ErrorTypeInternal:             true,
				ErrorTypeMCPProtocol:          true,
				ErrorTypeMethodNotAllowed:     true,
				ErrorTypeMissingAuthorization: true,
				ErrorTypeInvalidToken:         true,
			}

			if !validTypes[errResp.Type] {
				return false
			}

			// 4. For rate limit errors, must have actionable details
			if errResp.Type == ErrorTypeRateLimit {
				if errResp.Data == nil {
					return false
				}
				// Must have retry_after
				if _, ok := errResp.Data["retry_after"]; !ok {
					return false
				}
				// Must have limit
				if _, ok := errResp.Data["limit"]; !ok {
					return false
				}
				// Must have remaining
				if _, ok := errResp.Data["remaining"]; !ok {
					return false
				}
			}

			// 5. Error response must be JSON serializable
			jsonBytes, err := json.Marshal(errResp)
			if err != nil {
				return false
			}

			// 6. Must be able to deserialize back
			var decoded ErrorResponse
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				return false
			}

			// 7. Deserialized version must match original
			if decoded.Type != errResp.Type {
				return false
			}
			if decoded.Message != errResp.Message {
				return false
			}

			return true
		},
		genErrorStatusCode,
		genErrorMessage,
	))

	properties.Property("Rate limit errors contain actionable retry information", prop.ForAll(
		func(limit, remaining int, resetTime time.Time) bool {
			// Ensure resetTime is in the future
			if resetTime.Before(time.Now()) {
				resetTime = time.Now().Add(1 * time.Hour)
			}

			errResp := NewRateLimitError(limit, remaining, resetTime)

			// Must have error type
			if errResp.Type != ErrorTypeRateLimit {
				return false
			}

			// Must have data
			if errResp.Data == nil {
				return false
			}

			// Must have retry_after in seconds
			retryAfter, ok := errResp.Data["retry_after"].(int)
			if !ok {
				return false
			}

			// retry_after must be non-negative
			if retryAfter < 0 {
				return false
			}

			// Must have limit
			if errResp.Data["limit"] != limit {
				return false
			}

			// Must have remaining
			if errResp.Data["remaining"] != remaining {
				return false
			}

			// Must have reset_at timestamp
			if _, ok := errResp.Data["reset_at"].(string); !ok {
				return false
			}

			return true
		},
		gen.IntRange(1000, 10000),
		gen.IntRange(0, 1000),
		gen.TimeRange(time.Now(), 24*time.Hour),
	))

	properties.Property("Permission errors contain required scope information when provided", prop.ForAll(
		func(message string, hasScopes bool) bool {
			var scopes []string
			if hasScopes {
				scopes = []string{"repo", "read:org", "write:packages"}
			}

			errResp := NewPermissionDeniedError(message, scopes, "repository")

			// Must have authorization error type
			if errResp.Type != ErrorTypeAuthorization {
				return false
			}

			// Must have message
			if errResp.Message == "" {
				return false
			}

			// Must have data
			if errResp.Data == nil {
				return false
			}

			// If scopes were provided, they must be in the data
			if hasScopes {
				dataScopes, ok := errResp.Data["required_scopes"].([]string)
				if !ok {
					return false
				}
				if len(dataScopes) != len(scopes) {
					return false
				}
			}

			return true
		},
		gen.AlphaString(),
		gen.Bool(),
	))

	properties.Property("Authentication errors contain helpful hints", prop.ForAll(
		func(message string, hasHint bool) bool {
			// Skip empty messages as they're not valid
			if message == "" {
				return true
			}

			hint := ""
			if hasHint {
				hint = "Check your token configuration"
			}

			errResp := NewAuthenticationError(message, hint)

			// Must have authentication error type
			if errResp.Type != ErrorTypeAuthentication {
				return false
			}

			// Must have message
			if errResp.Message == "" {
				return false
			}

			// Must have data
			if errResp.Data == nil {
				return false
			}

			// If hint was provided, it must be in the data
			if hasHint {
				dataHint, ok := errResp.Data["hint"].(string)
				if !ok || dataHint == "" {
					return false
				}
			}

			return true
		},
		gen.AlphaString(),
		gen.Bool(),
	))

	properties.Property("All error types map to valid HTTP status codes", prop.ForAll(
		func(errType ErrorType) bool {
			statusCode := MapErrorTypeToHTTPStatus(errType)

			// Status code must be in error range (4xx or 5xx)
			if statusCode < 400 || statusCode >= 600 {
				return false
			}

			// Must be a valid HTTP status code
			statusText := http.StatusText(statusCode)
			if statusText == "" {
				return false
			}

			return true
		},
		gen.OneConstOf(
			ErrorTypeAuthentication,
			ErrorTypeAuthorization,
			ErrorTypeRateLimit,
			ErrorTypeNotFound,
			ErrorTypeValidation,
			ErrorTypeGitHubAPI,
			ErrorTypeInternal,
			ErrorTypeMCPProtocol,
			ErrorTypeMethodNotAllowed,
			ErrorTypeMissingAuthorization,
			ErrorTypeInvalidToken,
		),
	))

	properties.Property("All error types map to valid JSON-RPC error codes", prop.ForAll(
		func(errType ErrorType) bool {
			code := MapErrorTypeToJSONRPCCode(errType)

			// JSON-RPC error codes must be in valid ranges
			// Standard errors: -32768 to -32000
			// Server errors: -32000 to -32099
			if code < -32768 || code > -32000 {
				return false
			}

			return true
		},
		gen.OneConstOf(
			ErrorTypeAuthentication,
			ErrorTypeAuthorization,
			ErrorTypeRateLimit,
			ErrorTypeNotFound,
			ErrorTypeValidation,
			ErrorTypeGitHubAPI,
			ErrorTypeInternal,
			ErrorTypeMCPProtocol,
			ErrorTypeMethodNotAllowed,
			ErrorTypeMissingAuthorization,
			ErrorTypeInvalidToken,
		),
	))

	properties.Property("JSON-RPC errors are properly formatted", prop.ForAll(
		func(idType int, code JSONRPCErrorCode, message string) bool {
			// Generate different ID types based on idType
			var id interface{}
			switch idType {
			case 0:
				id = 1 // numeric ID
			case 1:
				id = "test-id" // string ID
			case 2:
				id = nil // null ID
			}

			// Create JSON-RPC error
			rpcErr := NewJSONRPCError(id, code, message, nil)

			// Must have jsonrpc version
			if rpcErr.JSONRPC != "2.0" {
				return false
			}

			// Must have ID
			if rpcErr.ID != id {
				return false
			}

			// Must have error object
			if rpcErr.Error.Code != code {
				return false
			}

			if rpcErr.Error.Message != message {
				return false
			}

			// Must be JSON serializable
			jsonBytes, err := json.Marshal(rpcErr)
			if err != nil {
				return false
			}

			// Must be deserializable
			var decoded JSONRPCError
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				return false
			}

			// Must have correct version after deserialization
			if decoded.JSONRPC != "2.0" {
				return false
			}

			return true
		},
		gen.IntRange(0, 2), // 0=numeric, 1=string, 2=nil
		gen.OneConstOf(
			JSONRPCParseError,
			JSONRPCInvalidRequest,
			JSONRPCMethodNotFound,
			JSONRPCInvalidParams,
			JSONRPCInternalError,
			JSONRPCServerError,
		),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Helper test to verify basic error structure requirements
func TestErrorStructureBasics(t *testing.T) {
	t.Run("all error types have required fields", func(t *testing.T) {
		errorTypes := []ErrorType{
			ErrorTypeAuthentication,
			ErrorTypeAuthorization,
			ErrorTypeRateLimit,
			ErrorTypeNotFound,
			ErrorTypeValidation,
			ErrorTypeGitHubAPI,
			ErrorTypeInternal,
			ErrorTypeMCPProtocol,
			ErrorTypeMethodNotAllowed,
			ErrorTypeMissingAuthorization,
			ErrorTypeInvalidToken,
		}

		for _, errType := range errorTypes {
			errResp := NewErrorResponse(errType, "Test error", nil)

			// Must have type
			if errResp.Type == "" {
				t.Errorf("Error type %s has empty Type field", errType)
			}

			// Must have message
			if errResp.Message == "" {
				t.Errorf("Error type %s has empty Message field", errType)
			}

			// Must be JSON serializable
			jsonBytes, err := json.Marshal(errResp)
			if err != nil {
				t.Errorf("Error type %s is not JSON serializable: %v", errType, err)
			}

			// Must be deserializable
			var decoded ErrorResponse
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				t.Errorf("Error type %s cannot be deserialized: %v", errType, err)
			}

			// Type must match after deserialization
			if decoded.Type != errType {
				t.Errorf("Error type %s does not match after deserialization: got %s", errType, decoded.Type)
			}
		}
	})

	t.Run("rate limit errors have actionable information", func(t *testing.T) {
		resetAt := time.Now().Add(1 * time.Hour)
		errResp := NewRateLimitError(5000, 0, resetAt)

		// Must have retry_after
		if _, ok := errResp.Data["retry_after"]; !ok {
			t.Error("Rate limit error missing retry_after")
		}

		// Must have limit
		if _, ok := errResp.Data["limit"]; !ok {
			t.Error("Rate limit error missing limit")
		}

		// Must have remaining
		if _, ok := errResp.Data["remaining"]; !ok {
			t.Error("Rate limit error missing remaining")
		}

		// Must have reset_at
		if _, ok := errResp.Data["reset_at"]; !ok {
			t.Error("Rate limit error missing reset_at")
		}
	})

	t.Run("permission errors have scope information when provided", func(t *testing.T) {
		scopes := []string{"repo", "read:org"}
		errResp := NewPermissionDeniedError("Permission denied", scopes, "repository")

		// Must have required_scopes
		if _, ok := errResp.Data["required_scopes"]; !ok {
			t.Error("Permission error missing required_scopes")
		}

		// Must have resource
		if _, ok := errResp.Data["resource"]; !ok {
			t.Error("Permission error missing resource")
		}
	})

	t.Run("authentication errors have hints when provided", func(t *testing.T) {
		errResp := NewAuthenticationError("Auth failed", "Check your token")

		// Must have hint
		if _, ok := errResp.Data["hint"]; !ok {
			t.Error("Authentication error missing hint")
		}
	})
}
