package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/reasoningtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

const (
	DefaultMaxSteps   = 8
	MinMaxSteps       = 2
	MaxMaxSteps       = 12
	ActionToolCall    = "tool_call"
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
	// LastTrace returns the ToolTrace produced by the most recent ExecuteTool call.
	LastTrace() logtrace.ToolTrace
}

// Engine runs the agent loop: plan → act → observe → repeat.
type Engine struct {
	generator NextActionGenerator
	executor  StepExecutor
	auditor   StepAuditor
	guard     security.IntentGuard
	registry  *tools.Registry
	MaxSteps  int
}

func NewEngine(generator NextActionGenerator, executor StepExecutor, registry *tools.Registry) *Engine {
	return NewEngineWithAuditor(generator, executor, registry, NoOpStepAuditor{})
}

// NewEngineWithAuditor builds an Engine whose every executed step is audited by auditor,
// producing a per-step AuditReport and an aggregated RiskGraph.
func NewEngineWithAuditor(generator NextActionGenerator, executor StepExecutor, registry *tools.Registry, auditor StepAuditor) *Engine {
	return &Engine{
		generator: generator,
		executor:  executor,
		auditor:   auditor,
		guard:     security.NewIntentGuard(),
		registry:  registry,
		MaxSteps:  DefaultMaxSteps,
	}
}

// NoOpStepAuditor is a StepAuditor that performs no audit; used by NewEngine for
// backward compatibility with callers that don't supply an auditor.
type NoOpStepAuditor struct{}

func (NoOpStepAuditor) AuditStep(_ context.Context, _ string, _ int, _ logtrace.ToolTrace) (AuditReport, error) {
	return AuditReport{Decision: "allow", Method: "no-audit"}, nil
}

// Run executes the agent loop and returns a structured AgentResponse.
func (e *Engine) Run(ctx context.Context, task string, rtb *reasoningtrace.TraceBuilder, requestSpanID string) (*AgentResponse, error) {
	// Check intent guard first.
	if intent := e.guard.Evaluate(task); intent.Decision == security.DecisionDeny {
		return &AgentResponse{
			AgentMode:   ModeAgentLoop,
			AgentSteps:  []AgentStep{},
			FinalAnswer: "该请求包含危险意图，已在安全检查阶段阻断，未执行任何系统工具。请确认操作目的后再试。",
			Confidence:  "high",
			StepCount:   0,
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
				RiskGraph:      buildRiskGraph(steps),
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
				RiskGraph:      buildRiskGraph(steps),
			}, nil
		}

		if action.ActionType == ActionFinalAnswer {
			return &AgentResponse{
				AgentMode: ModeAgentLoop,
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
				RiskGraph:       buildRiskGraph(steps),
			}, nil
		}

		if action.ActionType != ActionToolCall {
			return &AgentResponse{
				AgentMode:      ModeAgentLoop,
				AgentSteps:     steps,
				FinalAnswer:    "模型返回了无法识别的操作，安全起见未执行任何新增工具。请稍后重试。",
				Confidence:     "low",
				FallbackReason: fmt.Sprintf("invalid action_type %q", action.ActionType),
				StepCount:      len(steps),
				RiskGraph:      buildRiskGraph(steps),
			}, nil
		}

		// Execute tool call through security pipeline.
		step := e.executeStep(ctx, task, i+1, action, rtb, requestSpanID)
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
		AgentMode: ModeAgentLoop,
		TaskUnderstanding: &TaskUnderstanding{
			UserGoal:   task,
			IntentType: classifyIntent(task),
			RiskLevel:  classifyRisk(task),
		},
		AgentSteps:      steps,
		FinalAnswer:     buildMaxStepsFinalAnswer(task, steps),
		Confidence:      "medium",
		NextSuggestions: buildMaxStepsSuggestions(steps),
		StepCount:       len(steps),
		RiskGraph:       buildRiskGraph(steps),
	}, nil
}

func (e *Engine) executeStep(ctx context.Context, task string, index int, action *NextAction, rtb *reasoningtrace.TraceBuilder, parentSpanID string) AgentStep {
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

	// Add semantic metadata if available (before policy check so denied steps carry it too).
	if md, ok := e.registry.GetTool(action.ToolName); ok {
		step.OperationType = md.OperationType
		step.ResourceType = md.ResourceType
		step.BoundaryLevel = md.BoundaryLevel
	}

	// Tool policy check (the "context audit" allow/deny gate).
	allowed, reason := e.executor.CheckToolPolicy(action.ToolName, action.ToolArgs)
	step.AllowedByPolicy = allowed
	step.PolicyReason = reason
	if !allowed {
		step.PolicyDecision = "deny"
		step.Observation["status"] = "denied"
		step.Observation["result"] = fmt.Sprintf("tool call denied by policy: %s", reason)
		// A denied tool_call is not executed, but still produces an audit_report
		// (decision=deny) so it appears in the risk_graph. Synthesize a minimal trace.
		now := time.Now().UTC()
		deniedTrace := logtrace.ToolTrace{
			StepID:          logtrace.NextStepID(),
			ToolName:        action.ToolName,
			Input:           action.ToolArgs,
			Status:          "denied",
			StartedAt:       now,
			FinishedAt:      now,
			OperationType:   step.OperationType,
			ResourceType:    step.ResourceType,
			BoundaryLevel:   step.BoundaryLevel,
			AllowedByPolicy: false,
			PolicyReason:    reason,
		}
		rep := e.auditStepSafe(ctx, task, step.StepIndex, deniedTrace)
		rep.Decision = "deny"
		rep.Method = "tool_policy"
		step.AuditReport = &rep

		if hasRT {
			ps := rtb.StartSpan(parentSpanID, reasoningtrace.SpanToolPolicy, fmt.Sprintf("step %d: %s policy", index, action.ToolName))
			rtb.SetAttr(ps.SpanID, "tool_name", action.ToolName)
			rtb.SetAttr(ps.SpanID, "allowed_by_policy", false)
			rtb.SetAttr(ps.SpanID, "reason", reason)
			rtb.EndSpan(ps.SpanID, "deny")
		}
		return step
	}
	step.PolicyDecision = "allow"

	// Execute tool.
	obs, err := e.executor.ExecuteTool(ctx, action.ToolName, action.ToolArgs)
	if err != nil {
		step.Observation["status"] = "error"
		step.Observation["result"] = err.Error()
	} else {
		step.Observation = obs
	}

	// Per-step audit on the real trace produced by execution.
	if trace := e.executor.LastTrace(); trace.StepID != "" {
		rep := e.auditStepSafe(ctx, task, step.StepIndex, trace)
		step.AuditReport = &rep
	}

	// Add reasoning trace spans (optional).
	if hasRT {
		ps := rtb.StartSpan(parentSpanID, reasoningtrace.SpanToolPolicy, fmt.Sprintf("step %d: %s policy", index, action.ToolName))
		rtb.SetAttr(ps.SpanID, "tool_name", action.ToolName)
		rtb.SetAttr(ps.SpanID, "allowed_by_policy", allowed)
		rtb.EndSpan(ps.SpanID, "ok")
		es := rtb.StartSpan(parentSpanID, reasoningtrace.SpanExecProxy, fmt.Sprintf("step %d: %s exec", index, action.ToolName))
		rtb.SetAttr(es.SpanID, "tool_name", action.ToolName)
		rtb.SetAttr(es.SpanID, "operation_type", step.OperationType)
		rtb.SetAttr(es.SpanID, "resource_type", step.ResourceType)
		rtb.SetAttr(es.SpanID, "boundary_level", step.BoundaryLevel)
		rtb.SetAttr(es.SpanID, "status", step.Observation["status"])
		execStatus := "ok"
		if step.Observation["status"] == "error" || step.Observation["status"] == "denied" {
			execStatus = "error"
		}
		rtb.EndSpan(es.SpanID, execStatus)
	}
	return step
}

// auditStepSafe calls the StepAuditor for one step and never aborts the loop on
// failure (mirrors auditclient.HTTPClient's fallback philosophy at the engine layer).
func (e *Engine) auditStepSafe(ctx context.Context, task string, stepIndex int, trace logtrace.ToolTrace) AuditReport {
	if e.auditor == nil {
		return AuditReport{Decision: "allow", Method: "no-audit"}
	}
	rep, err := e.auditor.AuditStep(ctx, task, stepIndex, trace)
	if err != nil || rep.Decision == "" {
		return AuditReport{
			StepID:     trace.StepID,
			StepIndex:  stepIndex,
			ToolName:   trace.ToolName,
			Decision:   "review",
			RiskScore:  0.35,
			Method:     "local-safety-fallback",
			Message:    "audit service unavailable; local safety review required",
			Violations: []string{"external audit result unavailable"},
			Evidence:   []string{"tool execution trace retained for later audit"},
		}
	}
	rep.StepID = trace.StepID
	rep.StepIndex = stepIndex
	rep.ToolName = trace.ToolName
	return rep
}

// buildRiskGraph aggregates all per-step AuditReports into one risk_graph: one node
// per step (carrying the audit conclusion), sequence edges between consecutive steps.
// Returns nil when there are no steps (pure final_answer / intent-guard deny).
func buildRiskGraph(steps []AgentStep) *auditclient.RiskGraph {
	if len(steps) == 0 {
		return nil
	}
	nodes := make([]map[string]any, 0, len(steps))
	edges := make([]map[string]any, 0, len(steps)-1)
	for i, s := range steps {
		ar := AuditReport{}
		if s.AuditReport != nil {
			ar = *s.AuditReport
		}
		node := map[string]any{
			"step_id":          ar.StepID,
			"step_index":       s.StepIndex,
			"tool_name":        s.ToolName,
			"decision":         ar.Decision,
			"risk_score":       ar.RiskScore,
			"violations_count": len(ar.Violations),
			"method":           ar.Method,
			"policy_decision":  s.PolicyDecision,
			"operation_type":   s.OperationType,
			"resource_type":    s.ResourceType,
			"boundary_level":   s.BoundaryLevel,
		}
		nodes = append(nodes, node)
		if i > 0 {
			prevID := ""
			if steps[i-1].AuditReport != nil {
				prevID = steps[i-1].AuditReport.StepID
			}
			edges = append(edges, map[string]any{
				"from": prevID,
				"to":   ar.StepID,
				"type": "sequence",
			})
		}
	}
	return &auditclient.RiskGraph{Nodes: nodes, Edges: edges}
}

func buildMaxStepsFinalAnswer(task string, steps []AgentStep) string {
	if len(steps) == 0 {
		return "我没有获得可用的工具观察结果，因此无法给出可靠诊断。请补充现象、目标主机或服务名称后再试。"
	}
	var b strings.Builder
	b.WriteString("已完成多步受控检查，但模型在达到安全步数上限前没有主动结束。我已根据现有真实观察先给出阶段性结论：\n\n")
	b.WriteString("检查目标：")
	b.WriteString(strings.TrimSpace(task))
	b.WriteString("\n\n已检查的证据：\n")
	for _, step := range steps {
		toolName := step.ToolName
		if toolName == "" {
			toolName = "unknown_tool"
		}
		status := fmt.Sprint(step.Observation["status"])
		if status == "" || status == "<nil>" {
			status = step.PolicyDecision
		}
		summary := summarizeObservation(step.Observation)
		if summary == "" {
			summary = firstNonEmpty(step.UserVisibleSummary, step.Reason, "该步骤没有返回额外摘要")
		}
		fmt.Fprintf(&b, "- 第 %d 步 `%s`：%s；结果：%s\n", step.StepIndex, toolName, firstNonEmpty(step.UserVisibleSummary, step.Reason, "执行受控检查"), truncateText(summary, 220))
	}
	b.WriteString("\n阶段性判断：以上工具调用均来自受控执行链，可作为当前排查依据；由于已达到单次请求的安全步数上限，结论应按“需复核”处理，而不是继续无限调用工具。\n")
	b.WriteString("\n建议下一步：优先查看上面状态异常、被策略拒绝或风险较高的步骤；如果还需要继续排查，请基于这些证据提出更具体的后续问题。")
	return b.String()
}

func buildMaxStepsSuggestions(steps []AgentStep) []string {
	suggestions := []string{"基于当前证据继续追问一个更具体的问题"}
	for _, step := range steps {
		if step.PolicyDecision == "deny" {
			suggestions = append(suggestions, "先处理被安全策略拒绝的工具请求或缩小检查范围")
			break
		}
	}
	for _, step := range steps {
		if status := fmt.Sprint(step.Observation["status"]); status == "error" || status == "denied" {
			suggestions = append(suggestions, "优先复核执行失败或被拒绝的检查项")
			break
		}
	}
	if len(suggestions) == 1 {
		suggestions = append(suggestions, "如需更深入排查，可指定服务名、端口、进程或软件包")
	}
	return suggestions
}

func summarizeObservation(observation map[string]any) string {
	if len(observation) == 0 {
		return ""
	}
	for _, key := range []string{"summary", "result", "message", "deny_reason", "error"} {
		if value := strings.TrimSpace(fmt.Sprint(observation[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	data, err := json.Marshal(observation)
	if err != nil {
		return ""
	}
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truncateText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func buildToolDefs(registry *tools.Registry) []ToolDef {
	all := registry.ListTools()
	defs := make([]ToolDef, 0, len(all))
	for _, t := range all {
		if !registry.IsToolEnabledForDirectCall(t.Name) {
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
