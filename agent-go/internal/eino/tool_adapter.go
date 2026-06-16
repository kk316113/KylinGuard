package eino

import (
	"context"
	"fmt"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type PlanToolAdapter interface {
	Execute(ctx context.Context, step agent.PlanStep) (ToolCallResult, error)
}

type ToolCallResult struct {
	Output any
	Trace  logtrace.ToolTrace
}

type MCPToolAdapter struct {
	registry *tools.Registry
	policy   security.ToolPolicy
}

func NewMCPToolAdapter(registry *tools.Registry, policy security.ToolPolicy) *MCPToolAdapter {
	if registry == nil {
		registry = tools.NewDefaultRegistry()
	}
	return &MCPToolAdapter{
		registry: registry,
		policy:   policy,
	}
}

func (a *MCPToolAdapter) Execute(ctx context.Context, step agent.PlanStep) (ToolCallResult, error) {
	if step.Input == nil {
		step.Input = map[string]any{}
	}
	metadata, exists := a.registry.GetTool(step.ToolName)
	decision := a.policy.Evaluate(step.ToolName, metadata, exists, step.Input)
	if decision.Decision == "deny" {
		trace := deniedTrace(step, decision)
		return ToolCallResult{Trace: trace}, fmt.Errorf("%s: %s", decision.Message, decision.Reason)
	}

	result, err := a.registry.InvokeWithStepID(ctx, step.StepID, step.ToolName, step.Input)
	return ToolCallResult{
		Output: result.Output,
		Trace:  result.Trace,
	}, err
}

func deniedTrace(step agent.PlanStep, decision security.ToolPolicyDecision) logtrace.ToolTrace {
	now := time.Now().UTC()
	trace := logtrace.ToolTrace{
		StepID:        step.StepID,
		ToolName:      step.ToolName,
		Input:         step.Input,
		OutputSummary: decisionMessage(decision),
		Status:        "error",
		StartedAt:     now,
		FinishedAt:    now,
		RiskHint:      "high",
	}
	semantic := tools.SemanticForTool(step.ToolName, step.Input)
	trace.OperationType = semantic.OperationType
	trace.ResourceType = semantic.ResourceType
	trace.ResourcePath = semantic.ResourcePath
	trace.PermissionScope = semantic.PermissionScope
	trace.BoundaryLevel = semantic.BoundaryLevel
	trace.ToolSemantic = semantic.ToolSemantic
	trace.RequiresPrivilege = semantic.RequiresPrivilege
	trace.AllowedByPolicy = false
	trace.PolicyReason = decisionMessage(decision)
	return trace
}

func decisionMessage(decision security.ToolPolicyDecision) string {
	if decision.Reason == "" {
		return decision.Message
	}
	return decision.Message + ": " + decision.Reason
}
