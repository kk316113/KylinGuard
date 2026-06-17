package agentloop

import (
	"context"

	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

// ToolStepExecutor implements StepExecutor using the real tool registry and security policy.
type ToolStepExecutor struct {
	registry   *tools.Registry
	policy     security.ToolPolicy
}

func NewToolStepExecutor(registry *tools.Registry) *ToolStepExecutor {
	return &ToolStepExecutor{
		registry: registry,
		policy:   security.NewToolPolicy(),
	}
}

// CheckToolPolicy evaluates the tool call against Tool Policy.
func (e *ToolStepExecutor) CheckToolPolicy(toolName string, args map[string]any) (bool, string) {
	meta, exists := e.registry.GetTool(toolName)
	decision := e.policy.Evaluate(toolName, meta, exists, args)
	if decision.Decision == "deny" {
		return false, decision.Reason
	}
	return true, ""
}

// ExecuteTool invokes a tool through the registry and returns the observation.
func (e *ToolStepExecutor) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	result, err := e.registry.Invoke(ctx, toolName, args)
	if err != nil {
		return map[string]any{
			"status": "error",
			"result": err.Error(),
		}, err
	}
	obs := map[string]any{
		"status":         result.Trace.Status,
		"output_summary": result.Trace.OutputSummary,
		"operation_type": result.Trace.OperationType,
		"resource_type":  result.Trace.ResourceType,
		"boundary_level": result.Trace.BoundaryLevel,
	}
	if result.Output != nil {
		obs["output"] = truncateOutput(result.Output)
	}
	return obs, nil
}

func truncateOutput(output any) any {
	if m, ok := output.(map[string]any); ok {
		truncated := make(map[string]any)
		for k, v := range m {
			if s, ok := v.(string); ok && len(s) > 200 {
				truncated[k] = s[:200] + "..."
			} else {
				truncated[k] = v
			}
		}
		return truncated
	}
	return output
}

// compile-time interface check
var _ StepExecutor = (*ToolStepExecutor)(nil)
