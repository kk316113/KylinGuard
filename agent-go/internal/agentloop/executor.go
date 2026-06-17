package agentloop

import (
	"context"
	"sync"

	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

// ToolStepExecutor implements StepExecutor using the real tool registry and security policy,
// and collects tool traces for audit.
type ToolStepExecutor struct {
	registry *tools.Registry
	policy   security.ToolPolicy
	mu       sync.Mutex
	traces   []logtrace.ToolTrace
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

// ExecuteTool invokes a tool through the registry, records the trace, and returns the observation.
func (e *ToolStepExecutor) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	result, err := e.registry.Invoke(ctx, toolName, args)
	// Record trace.
	e.mu.Lock()
	e.traces = append(e.traces, result.Trace)
	e.mu.Unlock()

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

// GetTraces returns all collected tool traces and clears the buffer.
func (e *ToolStepExecutor) GetTraces() []logtrace.ToolTrace {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]logtrace.ToolTrace, len(e.traces))
	copy(result, e.traces)
	return result
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
