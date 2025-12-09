package main

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// WorkflowPhase represents a phase in the backlog-to-PR workflow
type WorkflowPhase struct {
	Name         string
	RequiredTool string
	Toolset      string
}

// **Feature: railway-github-app-deployment, Property 8: Backlog-to-PR workflow completeness**
// **Validates: Requirements 8.1, 8.2, 8.3, 8.4**
//
// Property: For any complete development workflow (query issues → create branch → commit files → create PR),
// all required GitHub API operations should be available through MCP tools
func TestProperty_WorkflowCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Define the complete workflow phases
	workflowPhases := []WorkflowPhase{
		{Name: "Query Issues", RequiredTool: "list_issues", Toolset: "issues"},
		{Name: "Read Issue Details", RequiredTool: "issue_read", Toolset: "issues"},
		{Name: "Create Branch", RequiredTool: "create_branch", Toolset: "repos"},
		{Name: "Commit Files", RequiredTool: "push_files", Toolset: "repos"},
		{Name: "Create Pull Request", RequiredTool: "create_pull_request", Toolset: "pull_requests"},
	}

	// Property: For any workflow phase, the required tool must be available
	properties.Property("All workflow phases have required tools available", prop.ForAll(
		func(phaseIndex int) bool {
			// Get the phase
			phase := workflowPhases[phaseIndex]

			// Create toolset group
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

			// Enable the required toolset
			toolset, err := tsg.GetToolset(phase.Toolset)
			if err != nil {
				t.Logf("Toolset '%s' not found for phase '%s': %v", phase.Toolset, phase.Name, err)
				return false
			}
			toolset.Enabled = true

			// Get all tools from the toolset
			tools := toolset.GetAvailableTools()
			
			// Check if the required tool is available
			for _, tool := range tools {
				if tool.Tool.Name == phase.RequiredTool {
					return true
				}
			}

			t.Logf("Required tool '%s' not found for phase '%s' in toolset '%s'",
				phase.RequiredTool, phase.Name, phase.Toolset)
			return false
		},
		gen.IntRange(0, len(workflowPhases)-1),
	))

	// Property: For any subset of workflow phases, all required tools are available
	properties.Property("Any workflow subset has all required tools", prop.ForAll(
		func(phaseIndices []int) bool {
			if len(phaseIndices) == 0 {
				return true // Empty workflow is trivially complete
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

			// Collect all required toolsets
			requiredToolsets := make(map[string]bool)
			for _, idx := range phaseIndices {
				if idx >= 0 && idx < len(workflowPhases) {
					requiredToolsets[workflowPhases[idx].Toolset] = true
				}
			}

			// Enable all required toolsets
			for toolsetID := range requiredToolsets {
				toolset, err := tsg.GetToolset(toolsetID)
				if err != nil {
					return false
				}
				toolset.Enabled = true
			}

			// Get all available tools from all toolsets
			availableTools := make(map[string]bool)
			for _, toolset := range tsg.Toolsets {
				tools := toolset.GetAvailableTools()
				for _, tool := range tools {
					availableTools[tool.Tool.Name] = true
				}
			}

			// Verify all required tools are available
			for _, idx := range phaseIndices {
				if idx >= 0 && idx < len(workflowPhases) {
					phase := workflowPhases[idx]
					if !availableTools[phase.RequiredTool] {
						t.Logf("Missing tool '%s' for phase '%s'", phase.RequiredTool, phase.Name)
						return false
					}
				}
			}

			return true
		},
		gen.SliceOf(gen.IntRange(0, len(workflowPhases)-1)),
	))

	// Property: Complete workflow (all phases) has all required tools
	properties.Property("Complete workflow has all required tools", prop.ForAll(
		func(_ bool) bool {
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

			// Enable all toolsets used in the workflow
			toolsetsToEnable := map[string]bool{
				"issues":        true,
				"repos":         true,
				"pull_requests": true,
			}

			for toolsetID := range toolsetsToEnable {
				toolset, err := tsg.GetToolset(toolsetID)
				if err != nil {
					t.Logf("Toolset '%s' not found: %v", toolsetID, err)
					return false
				}
				toolset.Enabled = true
			}

			// Get all available tools from all toolsets
			availableTools := make(map[string]bool)
			for _, toolset := range tsg.Toolsets {
				tools := toolset.GetAvailableTools()
				for _, tool := range tools {
					availableTools[tool.Tool.Name] = true
				}
			}

			// Verify all workflow phases have their required tools
			for _, phase := range workflowPhases {
				if !availableTools[phase.RequiredTool] {
					t.Logf("Complete workflow missing tool '%s' for phase '%s'",
						phase.RequiredTool, phase.Name)
					return false
				}
			}

			return true
		},
		gen.Const(true), // No input needed, just verify the complete workflow
	))

	// Property: Toolset enablement is idempotent
	properties.Property("Enabling toolsets multiple times produces same result", prop.ForAll(
		func(enableCount int) bool {
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

			// Enable the repos toolset multiple times
			for i := 0; i < enableCount; i++ {
				toolset, err := tsg.GetToolset("repos")
				if err != nil {
					return false
				}
				toolset.Enabled = true
			}

			// Get tools from repos toolset
			toolset, _ := tsg.GetToolset("repos")
			tools := toolset.GetAvailableTools()
			
			// Count how many times each tool appears
			toolCounts := make(map[string]int)
			for _, tool := range tools {
				toolCounts[tool.Tool.Name]++
			}

			// Verify no tool appears more than once
			for toolName, count := range toolCounts {
				if count > 1 {
					t.Logf("Tool '%s' appears %d times after enabling toolset %d times",
						toolName, count, enableCount)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ToolsetCombinations tests that different combinations of toolsets
// provide the expected tools
func TestProperty_ToolsetCombinations(t *testing.T) {
	properties := gopter.NewProperties(nil)

	allToolsets := []string{"issues", "repos", "pull_requests", "actions", "git"}

	// Property: For any combination of toolsets, enabling them provides their tools
	properties.Property("Toolset combinations provide expected tools", prop.ForAll(
		func(toolsetIndices []int) bool {
			if len(toolsetIndices) == 0 {
				return true // No toolsets enabled is valid
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

			// Enable selected toolsets
			enabledToolsets := make(map[string]bool)
			for _, idx := range toolsetIndices {
				if idx >= 0 && idx < len(allToolsets) {
					toolsetID := allToolsets[idx]
					toolset, err := tsg.GetToolset(toolsetID)
					if err != nil {
						t.Logf("Toolset '%s' not found: %v", toolsetID, err)
						return false
					}
					toolset.Enabled = true
					enabledToolsets[toolsetID] = true
				}
			}

			// Get all tools from all enabled toolsets
			toolCount := 0
			for _, toolset := range tsg.Toolsets {
				if toolset.Enabled {
					tools := toolset.GetAvailableTools()
					toolCount += len(tools)
				}
			}

			// Verify we have at least some tools if we enabled toolsets
			if len(enabledToolsets) > 0 && toolCount == 0 {
				t.Logf("No tools available despite enabling %d toolsets", len(enabledToolsets))
				return false
			}

			return true
		},
		gen.SliceOf(gen.IntRange(0, len(allToolsets)-1)),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_WorkflowToolDependencies tests that workflow phases have
// their dependencies satisfied
func TestProperty_WorkflowToolDependencies(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Define workflow dependencies: some phases depend on others
	type WorkflowDependency struct {
		Phase      WorkflowPhase
		DependsOn  []string // Tool names this phase might depend on
	}

	dependencies := []WorkflowDependency{
		{
			Phase:     WorkflowPhase{Name: "Create Branch", RequiredTool: "create_branch", Toolset: "repos"},
			DependsOn: []string{"list_branches"}, // Might want to check existing branches
		},
		{
			Phase:     WorkflowPhase{Name: "Commit Files", RequiredTool: "push_files", Toolset: "repos"},
			DependsOn: []string{"create_branch", "get_file_contents"}, // Need branch first, might read existing files
		},
		{
			Phase:     WorkflowPhase{Name: "Create Pull Request", RequiredTool: "create_pull_request", Toolset: "pull_requests"},
			DependsOn: []string{"push_files", "create_branch"}, // Need commits and branch
		},
	}

	// Property: For any workflow phase with dependencies, all dependencies are available
	properties.Property("Workflow dependencies are satisfied", prop.ForAll(
		func(depIndex int) bool {
			dep := dependencies[depIndex]

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

			// Enable all relevant toolsets (repos and pull_requests for this workflow)
			for _, toolsetID := range []string{"repos", "pull_requests"} {
				toolset, err := tsg.GetToolset(toolsetID)
				if err == nil {
					toolset.Enabled = true
				}
			}

			// Get all tools from all toolsets
			availableTools := make(map[string]bool)
			for _, toolset := range tsg.Toolsets {
				tools := toolset.GetAvailableTools()
				for _, tool := range tools {
					availableTools[tool.Tool.Name] = true
				}
			}

			// Verify the phase's required tool is available
			if !availableTools[dep.Phase.RequiredTool] {
				t.Logf("Phase '%s' missing required tool '%s'",
					dep.Phase.Name, dep.Phase.RequiredTool)
				return false
			}

			// Verify all dependencies are available
			for _, depTool := range dep.DependsOn {
				if !availableTools[depTool] {
					t.Logf("Phase '%s' missing dependency tool '%s'",
						dep.Phase.Name, depTool)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, len(dependencies)-1),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
