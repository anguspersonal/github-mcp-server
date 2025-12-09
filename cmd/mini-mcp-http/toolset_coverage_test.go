package main

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
)

// TestBacklogToPRWorkflowToolsetCoverage verifies that all required tools
// for the autonomous backlog-to-PR workflow are available in the toolsets.
// This test validates Requirements 8.1, 8.2, 8.3, 8.4, 8.6
func TestBacklogToPRWorkflowToolsetCoverage(t *testing.T) {
	// Define required tools for each phase of the backlog-to-PR workflow
	requiredTools := map[string][]string{
		"issues": {
			// Requirement 8.1: Query backlog items
			"list_issues",
			"issue_read",
			"search_issues",
		},
		"repos": {
			// Requirement 8.2: Create development branch
			"create_branch",
			"list_branches",
			// Requirement 8.3: Commit code changes
			"create_or_update_file",
			"push_files",
			"delete_file",
			"get_file_contents",
		},
		"pull_requests": {
			// Requirement 8.4: Create pull requests
			"create_pull_request",
			"update_pull_request",
			"list_pull_requests",
			"pull_request_read",
		},
		"actions": {
			// Requirement 8.6: Verify work (workflow runs, checks, PR status)
			"list_workflows",
			"list_workflow_runs",
			"get_workflow_run",
			"list_workflow_jobs",
			"get_job_logs",
		},
		"git": {
			// Requirement 8.2, 8.3: Low-level Git operations
			"get_repository_tree",
		},
	}

	// Get all available toolset IDs
	validToolsets := github.GetValidToolsetIDs()

	// Verify each required toolset exists
	for toolsetID := range requiredTools {
		if !validToolsets[toolsetID] {
			t.Errorf("Required toolset '%s' is not available", toolsetID)
		}
	}

	// Create a test toolset group to verify tool registration
	tsg := github.DefaultToolsetGroup(
		false, // not read-only
		nil,   // getClient
		nil,   // getGQLClient
		nil,   // getRawClient
		func(key string, defaultValue string) string { return defaultValue }, // translation helper
		1000,  // contentWindowSize
		github.FeatureFlags{}, // flags
		nil,   // cache
	)

	// Enable all required toolsets
	for toolsetID := range requiredTools {
		toolset, err := tsg.GetToolset(toolsetID)
		if err != nil {
			t.Fatalf("Toolset '%s' not found in toolset group: %v", toolsetID, err)
		}
		toolset.Enabled = true
	}

	// Get all registered tools from all toolsets
	registeredTools := make(map[string]bool)
	for _, toolset := range tsg.Toolsets {
		tools := toolset.GetAvailableTools()
		for _, tool := range tools {
			registeredTools[tool.Tool.Name] = true
		}
	}

	// Verify each required tool is registered
	missingTools := []string{}
	for toolsetID, tools := range requiredTools {
		for _, toolName := range tools {
			if !registeredTools[toolName] {
				missingTools = append(missingTools, toolName+" ("+toolsetID+")")
			}
		}
	}

	if len(missingTools) > 0 {
		t.Errorf("Missing required tools for backlog-to-PR workflow: %v", missingTools)
	}
}

// TestWorkflowPhaseToolAvailability tests that each phase of the workflow
// has the minimum required tools available
func TestWorkflowPhaseToolAvailability(t *testing.T) {
	tests := []struct {
		phase       string
		requirement string
		toolset     string
		minTools    []string
	}{
		{
			phase:       "Query Backlog Items",
			requirement: "8.1",
			toolset:     "issues",
			minTools:    []string{"list_issues", "issue_read"},
		},
		{
			phase:       "Create Development Branch",
			requirement: "8.2",
			toolset:     "repos",
			minTools:    []string{"create_branch"},
		},
		{
			phase:       "Commit Code Changes",
			requirement: "8.3",
			toolset:     "repos",
			minTools:    []string{"create_or_update_file", "push_files"},
		},
		{
			phase:       "Create Pull Request",
			requirement: "8.4",
			toolset:     "pull_requests",
			minTools:    []string{"create_pull_request"},
		},
		{
			phase:       "Verify Work",
			requirement: "8.6",
			toolset:     "actions",
			minTools:    []string{"list_workflow_runs", "get_workflow_run"},
		},
	}

	// Create toolset group
	tsg := github.DefaultToolsetGroup(
		false,
		nil,
		nil,
		nil,
		func(key string, defaultValue string) string { return defaultValue },
		1000,
		github.FeatureFlags{},
		nil,
	)

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			// Enable the toolset
			toolset, err := tsg.GetToolset(tt.toolset)
			if err != nil {
				t.Fatalf("Toolset '%s' not found for phase '%s': %v", tt.toolset, tt.phase, err)
			}
			toolset.Enabled = true

			// Get tools from this toolset
			tools := toolset.GetAvailableTools()
			availableTools := make(map[string]bool)
			for _, tool := range tools {
				availableTools[tool.Tool.Name] = true
			}

			// Verify minimum required tools
			for _, toolName := range tt.minTools {
				if !availableTools[toolName] {
					t.Errorf("Phase '%s' (Requirement %s) missing required tool '%s' from toolset '%s'",
						tt.phase, tt.requirement, toolName, tt.toolset)
				}
			}
		})
	}
}

// TestDefaultToolsetIncludesWorkflowTools verifies that the default toolset
// configuration includes the core tools needed for the workflow
func TestDefaultToolsetIncludesWorkflowTools(t *testing.T) {
	defaultToolsets := github.GetDefaultToolsetIDs()

	// Core toolsets required for backlog-to-PR workflow
	requiredInDefault := []string{
		"repos",         // For branch and file operations
		"issues",        // For backlog item access
		"pull_requests", // For PR creation
	}

	defaultMap := make(map[string]bool)
	for _, toolset := range defaultToolsets {
		defaultMap[toolset] = true
	}

	for _, required := range requiredInDefault {
		if !defaultMap[required] {
			t.Errorf("Default toolset configuration missing required toolset '%s' for backlog-to-PR workflow", required)
		}
	}
}

// TestToolsetMetadataCompleteness verifies that all toolsets have proper metadata
func TestToolsetMetadataCompleteness(t *testing.T) {
	availableToolsets := github.AvailableTools()

	for _, toolset := range availableToolsets {
		if toolset.ID == "" {
			t.Error("Found toolset with empty ID")
		}
		if toolset.Description == "" {
			t.Errorf("Toolset '%s' has empty description", toolset.ID)
		}
	}
}
