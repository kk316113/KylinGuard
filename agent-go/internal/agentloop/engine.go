package agentloop

import (
	"context"
	"fmt"
	"strings"

	"kylin-guard-agent/agent-go/internal/reasoningtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

const (
	DefaultMaxSteps = 6
	ActionToolCall  = "tool_call"
	ActionFinalAnswer = "final_answer"
)

// NextActionGenerator generates the next action given the task and step history.
type NextActionGenerator interface {
	GenerateNextAction(ctx context.Context, req NextActionRequest) (*NextAction, error)
}

// StepExecutor executes a tool call step through the security pipeline.
type StepExecutor interface {
	ExecuteTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error)
	CheckToolPolicy(toolName string, args map[string]any) (bool, string)
}

// Engine runs the agent loop: plan → act → observe → repeat.
type Engine struct {
	generator      NextActionGenerator
	executor       StepExecutor
	guard          security.IntentGuard
	registry       *tools.Registry
	MaxSteps       int
}

func NewEngine(generator NextActionGenerator, executor StepExecutor, registry *tools.Registry) *Engine {
	return &Engine{
		generator: generator,
		executor:  executor,
		guard:     security.NewIntentGuard(),
		registry:  registry,
		MaxSteps:  DefaultMaxSteps,
	}
}

// Run executes the agent loop and returns a structured AgentResponse.
func (e *Engine) Run(ctx context.Context, task string, rtb *reasoningtrace.TraceBuilder, requestSpanID string) (*AgentResponse, error) {
	// Check intent guard first.
	if intent := e.guard.Evaluate(task); intent.Decision == security.DecisionDeny {
		return &AgentResponse{
			AgentMode:  ModeAgentLoop,
			AgentSteps: []AgentStep{},
			FinalAnswer: "该请求包含危险意图，已在安全检查阶段阻断，未执行任何系统工具。请确认操作目的后再试。",
			Confidence: "high",
			StepCount:  0,
		}, nil
	}

	availableTools := buildToolDefs(e.registry)
	steps := make([]AgentStep, 0, e.MaxSteps)

	for i := 0; i < e.MaxSteps; i++ {
		req := NextActionRequest{
			Task:           task,
			StepHistory:    steps,
			AvailableTools: availableTools,
			MaxSteps:       e.MaxSteps,
		}

		action, err := e.generator.GenerateNextAction(ctx, req)
		if err != nil {
			return &AgentResponse{
				AgentMode:      ModeAgentLoop,
				AgentSteps:     steps,
				FinalAnswer:    fmt.Sprintf("推理过程出错：%v。请稍后重试。", err),
				Confidence:     "low",
				FallbackReason: err.Error(),
				StepCount:      len(steps),
			}, nil
		}

		if action == nil {
			return &AgentResponse{
				AgentMode:      ModeAgentLoop,
				AgentSteps:     steps,
				FinalAnswer:    "无法生成推理步骤，请提供更详细的信息后重试。",
				Confidence:     "low",
				FallbackReason: "nil action from generator",
				StepCount:      len(steps),
			}, nil
		}

		if action.ActionType == ActionFinalAnswer {
			return &AgentResponse{
				AgentMode:       ModeAgentLoop,
				TaskUnderstanding: &TaskUnderstanding{
					UserGoal:   task,
					IntentType: classifyIntent(task),
					RiskLevel:  classifyRisk(task),
				},
				AgentSteps:      steps,
				FinalAnswer:     action.FinalAnswer,
				Confidence:      action.Confidence,
				NextSuggestions: action.NextSuggestions,
				StepCount:       len(steps),
			}, nil
		}

		if action.ActionType != ActionToolCall {
			continue
		}

		// Execute tool call through security pipeline.
		step := e.executeStep(ctx, i+1, action, rtb, requestSpanID)
		steps = append(steps, step)
	}

	// Span: max steps reached (optional).
	if rtb != nil && requestSpanID != "" {
		ms := rtb.StartSpan(requestSpanID, reasoningtrace.SpanSecurityReport, "agent loop max steps reached")
		rtb.SetAttr(ms.SpanID, "step_count", len(steps))
		rtb.EndSpan(ms.SpanID, "warning")
	}

	// Max steps reached without final answer.
	return &AgentResponse{
		AgentMode:  ModeAgentLoop,
		AgentSteps: steps,
		FinalAnswer: "已达到最大推理步数限制。建议重新描述问题或拆分后分别查询。",
		Confidence: "low",
		StepCount:  len(steps),
	}, nil
}

func (e *Engine) executeStep(ctx context.Context, index int, action *NextAction, rtb *reasoningtrace.TraceBuilder, parentSpanID string) AgentStep {
	step := AgentStep{
		StepIndex:          index,
		ActionType:         ActionToolCall,
		ToolName:           action.ToolName,
		ToolArgs:           action.ToolArgs,
		Reason:             action.Reason,
		UserVisibleSummary: action.UserVisibleSummary,
		Observation:        map[string]any{},
	}

	hasRT := rtb != nil && parentSpanID != ""

	// Tool policy check.
	allowed, reason := e.executor.CheckToolPolicy(action.ToolName, action.ToolArgs)
	step.AllowedByPolicy = allowed
	step.PolicyReason = reason
	if !allowed {
		step.PolicyDecision = "deny"
		step.Observation["status"] = "denied"
		step.Observation["result"] = fmt.Sprintf("tool call denied by policy: %s", reason)
		return step
	}
	step.PolicyDecision = "allow"

	// Add semantic metadata if available.
	if md, ok := e.registry.GetTool(action.ToolName); ok {
		step.OperationType = md.OperationType
		step.ResourceType = md.ResourceType
		step.BoundaryLevel = md.BoundaryLevel
	}

	// Execute tool.
	obs, err := e.executor.ExecuteTool(ctx, action.ToolName, action.ToolArgs)
	if err != nil {
		step.Observation["status"] = "error"
		step.Observation["result"] = err.Error()
	} else {
		step.Observation = obs
	}

	// Add reasoning trace spans (optional).
	if hasRT {
		ps := rtb.StartSpan(parentSpanID, reasoningtrace.SpanToolPolicy, fmt.Sprintf("step %d: %s policy", index, action.ToolName))
		rtb.SetAttr(ps.SpanID, "tool_name", action.ToolName)
		rtb.SetAttr(ps.SpanID, "allowed_by_policy", allowed)
		if !allowed {
			rtb.SetAttr(ps.SpanID, "reason", reason)
			rtb.EndSpan(ps.SpanID, "deny")
		} else {
			rtb.EndSpan(ps.SpanID, "ok")
		}
		es := rtb.StartSpan(parentSpanID, reasoningtrace.SpanExecProxy, fmt.Sprintf("step %d: %s exec", index, action.ToolName))
		rtb.SetAttr(es.SpanID, "tool_name", action.ToolName)
		rtb.SetAttr(es.SpanID, "operation_type", step.OperationType)
		rtb.SetAttr(es.SpanID, "resource_type", step.ResourceType)
		rtb.SetAttr(es.SpanID, "boundary_level", step.BoundaryLevel)
		rtb.SetAttr(es.SpanID, "status", step.Observation["status"])
		rtb.EndSpan(es.SpanID, "ok")
	}
	return step
}

func buildToolDefs(registry *tools.Registry) []ToolDef {
	all := registry.ListTools()
	defs := make([]ToolDef, 0, len(all))
	for _, t := range all {
		if !t.Enabled {
			continue
		}
		keys := make([]string, 0)
		if props, ok := t.InputSchema["properties"].(map[string]any); ok {
			for k := range props {
				keys = append(keys, k)
			}
		}
		defs = append(defs, ToolDef{
			ToolName:      t.Name,
			Description:   t.Description,
			ArgKeys:       keys,
			OperationType: t.OperationType,
			ResourceType:  t.ResourceType,
			BoundaryLevel: t.BoundaryLevel,
			RiskLevel:     t.RiskLevel,
		})
	}
	return defs
}

func classifyIntent(task string) string {
	t := strings.ToLower(task)
	if strings.Contains(t, "ssh") || strings.Contains(t, "连不上") || strings.Contains(t, "login") {
		return "diagnosis"
	}
	if strings.Contains(t, "安全") || strings.Contains(t, "audit") {
		return "security_check"
	}
	return "general"
}

func classifyRisk(task string) string {
	t := strings.ToLower(task)
	if strings.Contains(t, "删除") || strings.Contains(t, "delete") || strings.Contains(t, "清空") || strings.Contains(t, "clear") {
		return "high"
	}
	if strings.Contains(t, "日志") || strings.Contains(t, "log") || strings.Contains(t, "auth") {
		return "medium"
	}
	return "low"
}
