package errors

import (
	"fmt"
	"net/http"
	"time"

	gogithub "github.com/google/go-github/v79/github"
)

// RateLimitErrorData contains rate limit specific information
type RateLimitErrorData struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
	RetryAfter int      `json:"retry_after"` // seconds until reset
}

// PermissionErrorData contains permission denied specific information
type PermissionErrorData struct {
	RequiredScopes []string `json:"required_scopes,omitempty"`
	Message        string   `json:"message"`
	Resource       string   `json:"resource,omitempty"`
}

// AuthenticationErrorData contains authentication error specific information
type AuthenticationErrorData struct {
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// GitHubAPIErrorData contains GitHub API error specific information
type GitHubAPIErrorData struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	Errors     []GitHubFieldError `json:"errors,omitempty"`
}

// GitHubFieldError represents field-specific errors from GitHub API
type GitHubFieldError struct {
	Resource string `json:"resource,omitempty"`
	Field    string `json:"field,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

// NewRateLimitError creates a structured rate limit error response
func NewRateLimitError(limit, remaining int, resetAt time.Time) *ErrorResponse {
	retryAfter := int(time.Until(resetAt).Seconds())
	if retryAfter < 0 {
		retryAfter = 0
	}
	
	data := map[string]interface{}{
		"limit":       limit,
		"remaining":   remaining,
		"reset_at":    resetAt.Format(time.RFC3339),
		"retry_after": retryAfter,
	}
	
	message := fmt.Sprintf("GitHub API rate limit exceeded. Limit: %d, Remaining: %d. Resets at %s",
		limit, remaining, resetAt.Format(time.RFC3339))
	
	return NewErrorResponse(ErrorTypeRateLimit, message, data)
}

// NewRateLimitErrorFromGitHub creates a rate limit error from GitHub response
func NewRateLimitErrorFromGitHub(resp *gogithub.Response) *ErrorResponse {
	if resp == nil || resp.Rate.Limit == 0 {
		// Fallback if rate limit info not available
		return NewErrorResponse(ErrorTypeRateLimit, 
			"GitHub API rate limit exceeded. Please try again later.", 
			map[string]interface{}{
				"retry_after": 3600, // Default 1 hour
			})
	}
	
	return NewRateLimitError(resp.Rate.Limit, resp.Rate.Remaining, resp.Rate.Reset.Time)
}

// NewPermissionDeniedError creates a structured permission denied error response
func NewPermissionDeniedError(message string, requiredScopes []string, resource string) *ErrorResponse {
	data := map[string]interface{}{
		"message": message,
	}
	
	if len(requiredScopes) > 0 {
		data["required_scopes"] = requiredScopes
	} else {
		data["required_scopes"] = nil
	}
	
	if resource != "" {
		data["resource"] = resource
	} else {
		data["resource"] = ""
	}
	
	fullMessage := "Permission denied"
	if message != "" {
		fullMessage = message
	}
	
	if len(requiredScopes) > 0 {
		fullMessage += fmt.Sprintf(". Required scopes: %v", requiredScopes)
	}
	
	return NewErrorResponse(ErrorTypeAuthorization, fullMessage, data)
}

// NewAuthenticationError creates a structured authentication error response
func NewAuthenticationError(message, hint string) *ErrorResponse {
	data := map[string]interface{}{
		"message": message,
	}
	
	if hint != "" {
		data["hint"] = hint
	} else {
		data["hint"] = ""
	}
	
	return NewErrorResponse(ErrorTypeAuthentication, message, data)
}

// NewGitHubAPIError creates a structured GitHub API error response
func NewGitHubAPIError(statusCode int, message, documentationURL string, fieldErrors []GitHubFieldError) *ErrorResponse {
	data := map[string]interface{}{
		"status_code": statusCode,
		"message":     message,
	}
	
	if documentationURL != "" {
		data["documentation_url"] = documentationURL
	} else {
		data["documentation_url"] = ""
	}
	
	if fieldErrors != nil && len(fieldErrors) > 0 {
		data["errors"] = fieldErrors
	} else {
		data["errors"] = nil
	}
	
	fullMessage := fmt.Sprintf("GitHub API error (status %d): %s", statusCode, message)
	
	return NewErrorResponse(ErrorTypeGitHubAPI, fullMessage, data)
}

// NewGitHubAPIErrorFromResponse creates a GitHub API error from a GitHub response
func NewGitHubAPIErrorFromResponse(resp *gogithub.Response, err error) *ErrorResponse {
	if resp == nil {
		return NewErrorResponse(ErrorTypeGitHubAPI, 
			fmt.Sprintf("GitHub API error: %v", err), 
			nil)
	}
	
	statusCode := resp.StatusCode
	message := http.StatusText(statusCode)
	
	// Try to extract error message from the error
	if err != nil {
		message = err.Error()
	}
	
	// Check if this is a rate limit error
	if statusCode == http.StatusForbidden && resp.Rate.Remaining == 0 {
		return NewRateLimitErrorFromGitHub(resp)
	}
	
	// Check if this is a permission error
	if statusCode == http.StatusForbidden {
		return NewPermissionDeniedError(message, nil, "")
	}
	
	// Check if this is an authentication error
	if statusCode == http.StatusUnauthorized {
		return NewAuthenticationError(message, "Verify your GitHub token is valid and has not expired")
	}
	
	// Check if this is a not found error
	if statusCode == http.StatusNotFound {
		return NewErrorResponse(ErrorTypeNotFound, message, map[string]interface{}{
			"status_code": statusCode,
		})
	}
	
	// Try to extract field errors if available (GitHub error responses often include these)
	// Check this before generic validation error to preserve field error information
	if ghErr, ok := err.(*gogithub.ErrorResponse); ok {
		var fieldErrors []GitHubFieldError
		for _, e := range ghErr.Errors {
			fieldErrors = append(fieldErrors, GitHubFieldError{
				Resource: e.Resource,
				Field:    e.Field,
				Code:     e.Code,
				Message:  e.Message,
			})
		}
		
		if ghErr.Message != "" {
			message = ghErr.Message
		}
		
		return NewGitHubAPIError(statusCode, message, ghErr.DocumentationURL, fieldErrors)
	}
	
	// Check if this is a validation error
	if statusCode == http.StatusUnprocessableEntity || statusCode == http.StatusBadRequest {
		return NewErrorResponse(ErrorTypeValidation, message, map[string]interface{}{
			"status_code": statusCode,
		})
	}
	
	// Generic GitHub API error
	return NewGitHubAPIError(statusCode, message, "", nil)
}

// WriteRateLimitError writes a rate limit error to HTTP response
func WriteRateLimitError(w http.ResponseWriter, limit, remaining int, resetAt time.Time) {
	errResp := NewRateLimitError(limit, remaining, resetAt)
	WriteHTTPError(w, http.StatusTooManyRequests, errResp.Type, errResp.Message, errResp.Data)
}

// WritePermissionDeniedError writes a permission denied error to HTTP response
func WritePermissionDeniedError(w http.ResponseWriter, message string, requiredScopes []string, resource string) {
	errResp := NewPermissionDeniedError(message, requiredScopes, resource)
	WriteHTTPError(w, http.StatusForbidden, errResp.Type, errResp.Message, errResp.Data)
}

// WriteAuthenticationError writes an authentication error to HTTP response
func WriteAuthenticationError(w http.ResponseWriter, message, hint string) {
	errResp := NewAuthenticationError(message, hint)
	WriteHTTPError(w, http.StatusUnauthorized, errResp.Type, errResp.Message, errResp.Data)
}

// WriteGitHubAPIError writes a GitHub API error to HTTP response
func WriteGitHubAPIError(w http.ResponseWriter, resp *gogithub.Response, err error) {
	errResp := NewGitHubAPIErrorFromResponse(resp, err)
	statusCode := MapErrorTypeToHTTPStatus(errResp.Type)
	WriteHTTPError(w, statusCode, errResp.Type, errResp.Message, errResp.Data)
}
