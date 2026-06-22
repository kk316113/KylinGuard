package eino

import (
	"encoding/json"
	"fmt"
	"strings"

	"kylin-guard-agent/agent-go/internal/tools"
)

// ToolPlanItem is a single tool call in a structured plan from a remote LLM.
type ToolPlanItem struct {
	ToolName  string         `json:"tool_name"`
	Reason    string         `json:"reason"`
	Arguments map[string]any `json:"arguments"`
}

// ToolPlan is the structured JSON output expected from a remote LLM.
type ToolPlan struct {
	Scenario        string         `json:"scenario"`
	Intent          string         `json:"intent,omitempty"`
	ToolPlan        []ToolPlanItem `json:"tool_plan"`
	RiskHint        string         `json:"risk_hint,omitempty"`
	RequiresReview  bool           `json:"requires_review"`
	UserExplanation string         `json:"user_explanation,omitempty"`
}

// ValidateToolPlanResult holds the outcome of tool plan validation.
type ValidateToolPlanResult struct {
	Valid          bool
	Plan           *ToolPlan
	FallbackReason string
	RejectedTools  []string
	ValidTools     []string
}

// ValidateToolPlan validates a parsed ToolPlan against the tool registry and security rules.
func ValidateToolPlan(plan *ToolPlan, registry *tools.Registry) ValidateToolPlanResult {
	if plan == nil {
		return ValidateToolPlanResult{
			Valid:          false,
			FallbackReason: "tool plan is nil",
		}
	}
	if plan.Scenario == "" {
		return ValidateToolPlanResult{
			Valid:          false,
			FallbackReason: "scenario is empty",
		}
	}
	if len(plan.ToolPlan) == 0 {
		return ValidateToolPlanResult{
			Valid:          false,
			FallbackReason: "tool_plan is empty",
		}
	}

	var rejectedTools []string
	var validTools []string

	for _, item := range plan.ToolPlan {
		toolName := strings.TrimSpace(item.ToolName)
		if toolName == "" {
			rejectedTools = append(rejectedTools, "(unnamed tool)")
			continue
		}
		// Check for dangerous command intents masked as tool names.
		if isDangerousToolPlanIntent(toolName) {
			rejectedTools = append(rejectedTools, toolName)
			continue
		}
		// Check if tool exists in registry.
		meta, exists := registry.GetTool(toolName)
		if !exists {
			rejectedTools = append(rejectedTools, toolName)
			continue
		}
		// safe_shell is never allowed in LLM tool plans.
		if toolName == "safe_shell" {
			rejectedTools = append(rejectedTools, toolName)
			continue
		}
		// Arguments must be an object (map).
		if item.Arguments == nil {
			item.Arguments = map[string]any{}
		}
		_ = meta
		validTools = append(validTools, toolName)
	}

	if len(rejectedTools) > 0 && len(validTools) == 0 {
		return ValidateToolPlanResult{
			Valid:          false,
			FallbackReason: fmt.Sprintf("all tools rejected: %v", rejectedTools),
			RejectedTools:  rejectedTools,
		}
	}

	return ValidateToolPlanResult{
		Valid:         true,
		Plan:          plan,
		RejectedTools: rejectedTools,
		ValidTools:    validTools,
	}
}

// isDangerousToolPlanIntent checks if a tool plan item name is a dangerous command intent.
// The LLM might try to output shell commands or dangerous system operations.
func isDangerousToolPlanIntent(name string) bool {
	dangerous := map[string]bool{
		"shell": true, "bash": true, "sh": true, "zsh": true,
		"cmd": true, "powershell": true, "pwsh": true,
		"sudo": true, "su": true,
		"rm": true, "mv": true, "cp": true,
		"chmod": true, "chown": true,
		"kill": true, "pkill": true,
		"mount": true, "umount": true, "mkfs": true, "dd": true,
		"iptables": true, "firewall-cmd": true, "nft": true,
		"reboot": true, "shutdown": true, "poweroff": true, "halt": true,
		"systemctl": true, // systemctl must go through Tool Policy
	}
	return dangerous[name]
}

// ParseToolPlanJSON parses a JSON string into a ToolPlan.
func ParseToolPlanJSON(data []byte) (*ToolPlan, error) {
	var plan ToolPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("tool plan JSON parse error: %w", err)
	}
	return &plan, nil
}

// BuildToolDefsForPrompt builds a safe subset of tool metadata for inclusion in LLM prompts.
// It does NOT include sensitive fields like API keys, internal paths, or full command templates.
func BuildToolDefsForPrompt(registry *tools.Registry) []map[string]any {
	allTools := registry.ListTools()
	defs := make([]map[string]any, 0, len(allTools))
	for _, t := range allTools {
		// Only expose tools that are safe for structured direct invocation.
		if !registry.IsToolEnabledForDirectCall(t.Name) {
			continue
		}
		def := map[string]any{
			"tool_name":          t.Name,
			"description":        t.Description,
			"category":           t.Category,
			"operation_type":     t.OperationType,
			"resource_type":      t.ResourceType,
			"boundary_level":     t.BoundaryLevel,
			"risk_level":         t.RiskLevel,
			"requires_privilege": t.RequiresPrivilege,
		}
		// Extract allowed argument keys from input_schema properties.
		if props, ok := t.InputSchema["properties"].(map[string]any); ok {
			argKeys := make([]string, 0, len(props))
			for key := range props {
				argKeys = append(argKeys, key)
			}
			def["allowed_argument_keys"] = argKeys
		}
		defs = append(defs, def)
	}
	return defs
}
