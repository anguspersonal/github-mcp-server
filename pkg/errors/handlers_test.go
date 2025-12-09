package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimitError(t *testing.T) {
	t.Run("creates rate limit error with correct data", func(t *testing.T) {
		resetAt := time.Now().Add(1 * time.Hour)
		
		errResp := NewRateLimitError(5000, 0, resetAt)
		
		assert.Equal(t, ErrorTypeRateLimit, errResp.Type)
		assert.Contains(t, errResp.Message, "rate limit exceeded")
		assert.Contains(t, errResp.Message, "5000")
		assert.Contains(t, errResp.Message, "0")
		
		assert.Equal(t, 5000, errResp.Data["limit"])
		assert.Equal(t, 0, errResp.Data["remaining"])
		assert.NotNil(t, errResp.Data["reset_at"])
		
		retryAfter, ok := errResp.Data["retry_after"].(int)
		assert.True(t, ok)
		assert.Greater(t, retryAfter, 3500) // Should be close to 3600 seconds
		assert.LessOrEqual(t, retryAfter, 3600)
	})
	
	t.Run("handles past reset time", func(t *testing.T) {
		resetAt := time.Now().Add(-1 * time.Hour)
		
		errResp := NewRateLimitError(5000, 0, resetAt)
		
		retryAfter, ok := errResp.Data["retry_after"].(int)
		assert.True(t, ok)
		assert.Equal(t, 0, retryAfter, "retry_after should be 0 for past reset times")
	})
}

func TestNewRateLimitErrorFromGitHub(t *testing.T) {
	t.Run("creates error from GitHub response with rate limit info", func(t *testing.T) {
		resetTime := time.Now().Add(30 * time.Minute)
		resp := &gogithub.Response{
			Rate: gogithub.Rate{
				Limit:     5000,
				Remaining: 0,
				Reset:     gogithub.Timestamp{Time: resetTime},
			},
		}
		
		errResp := NewRateLimitErrorFromGitHub(resp)
		
		assert.Equal(t, ErrorTypeRateLimit, errResp.Type)
		assert.Equal(t, 5000, errResp.Data["limit"])
		assert.Equal(t, 0, errResp.Data["remaining"])
	})
	
	t.Run("handles nil response with fallback", func(t *testing.T) {
		errResp := NewRateLimitErrorFromGitHub(nil)
		
		assert.Equal(t, ErrorTypeRateLimit, errResp.Type)
		assert.Contains(t, errResp.Message, "rate limit exceeded")
		assert.Equal(t, 3600, errResp.Data["retry_after"]) // Default 1 hour
	})
	
	t.Run("handles response without rate limit info", func(t *testing.T) {
		resp := &gogithub.Response{
			Rate: gogithub.Rate{
				Limit: 0, // No rate limit info
			},
		}
		
		errResp := NewRateLimitErrorFromGitHub(resp)
		
		assert.Equal(t, ErrorTypeRateLimit, errResp.Type)
		assert.Equal(t, 3600, errResp.Data["retry_after"]) // Default fallback
	})
}

func TestNewPermissionDeniedError(t *testing.T) {
	t.Run("creates permission error with required scopes", func(t *testing.T) {
		scopes := []string{"repo", "read:org"}
		
		errResp := NewPermissionDeniedError("Insufficient permissions", scopes, "repository")
		
		assert.Equal(t, ErrorTypeAuthorization, errResp.Type)
		assert.Contains(t, errResp.Message, "Insufficient permissions")
		assert.Contains(t, errResp.Message, "repo")
		assert.Contains(t, errResp.Message, "read:org")
		
		assert.Equal(t, "Insufficient permissions", errResp.Data["message"])
		assert.Equal(t, scopes, errResp.Data["required_scopes"])
		assert.Equal(t, "repository", errResp.Data["resource"])
	})
	
	t.Run("creates permission error without scopes", func(t *testing.T) {
		errResp := NewPermissionDeniedError("Access denied", nil, "")
		
		assert.Equal(t, ErrorTypeAuthorization, errResp.Type)
		assert.Equal(t, "Access denied", errResp.Message)
		assert.Equal(t, "Access denied", errResp.Data["message"])
		assert.Nil(t, errResp.Data["required_scopes"])
		assert.Equal(t, "", errResp.Data["resource"])
	})
	
	t.Run("uses default message when empty", func(t *testing.T) {
		errResp := NewPermissionDeniedError("", []string{"repo"}, "")
		
		assert.Contains(t, errResp.Message, "Permission denied")
		assert.Contains(t, errResp.Message, "repo")
	})
}

func TestNewAuthenticationError(t *testing.T) {
	t.Run("creates authentication error with hint", func(t *testing.T) {
		errResp := NewAuthenticationError("Invalid token", "Check your token configuration")
		
		assert.Equal(t, ErrorTypeAuthentication, errResp.Type)
		assert.Equal(t, "Invalid token", errResp.Message)
		assert.Equal(t, "Invalid token", errResp.Data["message"])
		assert.Equal(t, "Check your token configuration", errResp.Data["hint"])
	})
	
	t.Run("creates authentication error without hint", func(t *testing.T) {
		errResp := NewAuthenticationError("Authentication failed", "")
		
		assert.Equal(t, ErrorTypeAuthentication, errResp.Type)
		assert.Equal(t, "Authentication failed", errResp.Message)
		assert.Equal(t, "", errResp.Data["hint"])
	})
}

func TestNewGitHubAPIError(t *testing.T) {
	t.Run("creates GitHub API error with all fields", func(t *testing.T) {
		fieldErrors := []GitHubFieldError{
			{Resource: "Issue", Field: "title", Code: "missing", Message: "Title is required"},
		}
		
		errResp := NewGitHubAPIError(422, "Validation failed", "https://docs.github.com", fieldErrors)
		
		assert.Equal(t, ErrorTypeGitHubAPI, errResp.Type)
		assert.Contains(t, errResp.Message, "422")
		assert.Contains(t, errResp.Message, "Validation failed")
		
		assert.Equal(t, 422, errResp.Data["status_code"])
		assert.Equal(t, "Validation failed", errResp.Data["message"])
		assert.Equal(t, "https://docs.github.com", errResp.Data["documentation_url"])
		assert.NotNil(t, errResp.Data["errors"])
	})
	
	t.Run("creates GitHub API error without optional fields", func(t *testing.T) {
		errResp := NewGitHubAPIError(500, "Internal server error", "", nil)
		
		assert.Equal(t, ErrorTypeGitHubAPI, errResp.Type)
		assert.Contains(t, errResp.Message, "500")
		assert.Equal(t, 500, errResp.Data["status_code"])
		assert.Equal(t, "", errResp.Data["documentation_url"])
		assert.Nil(t, errResp.Data["errors"])
	})
}

func TestNewGitHubAPIErrorFromResponse(t *testing.T) {
	t.Run("handles nil response", func(t *testing.T) {
		err := fmt.Errorf("network error")
		
		errResp := NewGitHubAPIErrorFromResponse(nil, err)
		
		assert.Equal(t, ErrorTypeGitHubAPI, errResp.Type)
		assert.Contains(t, errResp.Message, "network error")
	})
	
	t.Run("detects rate limit error from 403 with zero remaining", func(t *testing.T) {
		resetTime := time.Now().Add(1 * time.Hour)
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusForbidden},
			Rate: gogithub.Rate{
				Limit:     5000,
				Remaining: 0,
				Reset:     gogithub.Timestamp{Time: resetTime},
			},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("rate limit"))
		
		assert.Equal(t, ErrorTypeRateLimit, errResp.Type)
	})
	
	t.Run("detects permission error from 403", func(t *testing.T) {
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusForbidden},
			Rate: gogithub.Rate{
				Remaining: 100, // Not a rate limit issue
			},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("forbidden"))
		
		assert.Equal(t, ErrorTypeAuthorization, errResp.Type)
	})
	
	t.Run("detects authentication error from 401", func(t *testing.T) {
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusUnauthorized},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("unauthorized"))
		
		assert.Equal(t, ErrorTypeAuthentication, errResp.Type)
		assert.Contains(t, errResp.Data["hint"].(string), "token")
	})
	
	t.Run("detects not found error from 404", func(t *testing.T) {
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("not found"))
		
		assert.Equal(t, ErrorTypeNotFound, errResp.Type)
	})
	
	t.Run("detects validation error from 422", func(t *testing.T) {
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusUnprocessableEntity},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("validation failed"))
		
		assert.Equal(t, ErrorTypeValidation, errResp.Type)
	})
	
	t.Run("detects validation error from 400", func(t *testing.T) {
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusBadRequest},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, fmt.Errorf("bad request"))
		
		assert.Equal(t, ErrorTypeValidation, errResp.Type)
	})
	
	t.Run("extracts field errors from ErrorResponse", func(t *testing.T) {
		ghErr := &gogithub.ErrorResponse{
			Response: &http.Response{StatusCode: 422},
			Message:  "Validation Failed",
			DocumentationURL: "https://docs.github.com/rest",
			Errors: []gogithub.Error{
				{Resource: "Issue", Field: "title", Code: "missing"},
				{Resource: "Issue", Field: "body", Code: "invalid"},
			},
		}
		
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: 422},
		}
		
		errResp := NewGitHubAPIErrorFromResponse(resp, ghErr)
		
		assert.Equal(t, ErrorTypeGitHubAPI, errResp.Type)
		assert.Contains(t, errResp.Message, "Validation Failed")
		assert.Equal(t, "https://docs.github.com/rest", errResp.Data["documentation_url"])
		
		errors, ok := errResp.Data["errors"].([]GitHubFieldError)
		assert.True(t, ok)
		assert.Len(t, errors, 2)
		assert.Equal(t, "Issue", errors[0].Resource)
		assert.Equal(t, "title", errors[0].Field)
		assert.Equal(t, "missing", errors[0].Code)
	})
}

func TestWriteRateLimitError(t *testing.T) {
	t.Run("writes rate limit error to HTTP response", func(t *testing.T) {
		w := httptest.NewRecorder()
		resetAt := time.Now().Add(30 * time.Minute)
		
		WriteRateLimitError(w, 5000, 0, resetAt)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		
		var response HTTPErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeRateLimit, response.Error.Type)
		assert.Contains(t, response.Error.Message, "rate limit")
	})
}

func TestWritePermissionDeniedError(t *testing.T) {
	t.Run("writes permission denied error to HTTP response", func(t *testing.T) {
		w := httptest.NewRecorder()
		scopes := []string{"repo", "read:org"}
		
		WritePermissionDeniedError(w, "Insufficient permissions", scopes, "repository")
		
		assert.Equal(t, http.StatusForbidden, w.Code)
		
		var response HTTPErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeAuthorization, response.Error.Type)
		assert.Contains(t, response.Error.Message, "Insufficient permissions")
	})
}

func TestWriteAuthenticationError(t *testing.T) {
	t.Run("writes authentication error to HTTP response", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		WriteAuthenticationError(w, "Invalid token", "Check your configuration")
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		
		var response HTTPErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeAuthentication, response.Error.Type)
		assert.Equal(t, "Invalid token", response.Error.Message)
	})
}

func TestWriteGitHubAPIError(t *testing.T) {
	t.Run("writes GitHub API error to HTTP response", func(t *testing.T) {
		w := httptest.NewRecorder()
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		}
		err := fmt.Errorf("repository not found")
		
		WriteGitHubAPIError(w, resp, err)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		var response HTTPErrorResponse
		jsonErr := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, jsonErr)
		
		assert.Equal(t, ErrorTypeNotFound, response.Error.Type)
	})
	
	t.Run("writes rate limit error with correct status", func(t *testing.T) {
		w := httptest.NewRecorder()
		resetTime := time.Now().Add(1 * time.Hour)
		resp := &gogithub.Response{
			Response: &http.Response{StatusCode: http.StatusForbidden},
			Rate: gogithub.Rate{
				Limit:     5000,
				Remaining: 0,
				Reset:     gogithub.Timestamp{Time: resetTime},
			},
		}
		
		WriteGitHubAPIError(w, resp, fmt.Errorf("rate limit"))
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		
		var response HTTPErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, ErrorTypeRateLimit, response.Error.Type)
	})
}
