package agentloop

import (
	"context"
	"strings"
	"testing"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

// mockGenerator implements NextActionGenerator for testing.
type mockGenerator struct {
	actions []NextAction
	index   int
}

func (m *mockGenerator) GenerateNextAction(ctx context.Context, req NextActionRequest) (*NextAction, error) {
	if m.index >= len(m.actions) {
		return &NextAction{ActionType: "final_answer", FinalAnswer: "done"}, nil
	}
	a := m.actions[m.index]
	m.index++
	return &a, nil
}

// mockExecutor implements StepExecutor for testing.
type mockExecutor struct {
	policy    security.ToolPolicy
	registry  *tools.Registry
	lastTrace logtrace.ToolTrace
}

func newMockExecutor(registry *tools.Registry) *mockExecutor {
	return &mockExecutor{policy: security.NewToolPolicy(), registry: registry}
}

func (m *mockExecutor) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	m.lastTrace = logtrace.ToolTrace{
		StepID:   logtrace.NextStepID(),
		ToolName: toolName,
		Input:    args,
		Status:   "ok",
	}
	return map[string]any{"status": "ok", "result": toolName + " executed"}, nil
}

func (m *mockExecutor) CheckToolPolicy(toolName string, args map[string]any) (bool, string) {
	// Delegate to the real ToolPolicy so disabled/dangerous tools (e.g. safe_shell) are denied.
	meta, exists := m.registry.GetTool(toolName)
	decision := m.policy.Evaluate(toolName, meta, exists, args)
	if decision.Decision == "deny" {
		return false, decision.Reason
	}
	return true, ""
}

func (m *mockExecutor) LastTrace() logtrace.ToolTrace {
	return m.lastTrace
}

// mockStepAuditor implements StepAuditor for testing: ssh-related tools get review,
// everything else gets allow.
type mockStepAuditor struct{}

func (mockStepAuditor) AuditStep(_ context.Context, _ string, _ int, trace logtrace.ToolTrace) (AuditReport, error) {
	switch trace.ToolName {
	case "service_status", "port_checker", "journalctl_reader", "ssh_login_analyzer":
		return AuditReport{Decision: "review", RiskScore: 0.5, Method: "mock-auditor", Violations: []string{"ssh resource touched"}}, nil
	default:
		return AuditReport{Decision: "allow", RiskScore: 0.2, Method: "mock-auditor"}, nil
	}
}

func TestEngineSSHAgentLoopMultiStep(t *testing.T) {
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "service_status", ToolArgs: map[string]any{"service_name": "sshd"}, Reason: "Check sshd"},
			{ActionType: "tool_call", ToolName: "port_checker", ToolArgs: map[string]any{"host": "127.0.0.1", "port": 22}, Reason: "Check port 22"},
			{ActionType: "tool_call", ToolName: "journalctl_reader", ToolArgs: map[string]any{"service_name": "sshd", "lines": 50}, Reason: "Read journal"},
			{ActionType: "final_answer", FinalAnswer: "完成排查", Confidence: "medium"},
		},
	}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "我 SSH 连不上了，帮我看看", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AgentMode != ModeAgentLoop {
		t.Fatalf("expected agent_loop mode, got %q", resp.AgentMode)
	}
	if len(resp.AgentSteps) < 3 {
		t.Fatalf("expected >=3 agent steps, got %d", len(resp.AgentSteps))
	}
	if resp.StepCount < 3 {
		t.Fatalf("expected step_count >= 3, got %d", resp.StepCount)
	}
	if resp.FinalAnswer == "" {
		t.Fatal("expected non-empty final_answer")
	}

	// Check each step has observation.
	for i, step := range resp.AgentSteps {
		if step.Observation == nil || len(step.Observation) == 0 {
			t.Fatalf("step %d missing observation", i)
		}
		if step.ActionType != "tool_call" {
			t.Fatalf("step %d expected tool_call, got %q", i, step.ActionType)
		}
	}
}

func TestEngineDangerousTaskDenied(t *testing.T) {
	gen := &mockGenerator{}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "delete audit logs and clear system logs", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) != 0 {
		t.Fatalf("expected 0 steps for dangerous task, got %d", len(resp.AgentSteps))
	}
	if resp.FinalAnswer == "" {
		t.Fatal("expected non-empty final_answer for dangerous task")
	}
}

func TestEngineUnknownActionStopsWithoutExecution(t *testing.T) {
	gen := &mockGenerator{actions: []NextAction{{ActionType: "run_shell", ToolName: "safe_shell"}}}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "检查系统状态", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) != 0 || exec.LastTrace().StepID != "" {
		t.Fatalf("unknown action must not execute or create a tool step: %#v", resp.AgentSteps)
	}
	if resp.FallbackReason == "" || resp.FinalAnswer == "" {
		t.Fatalf("expected controlled diagnostic response: %#v", resp)
	}
}

func TestEngineMaxStepsReached(t *testing.T) {
	// Generator always returns tool_call, never final_answer.
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
		},
	}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())
	engine.MaxSteps = 3

	resp, err := engine.Run(context.Background(), "check system", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) > 3 {
		t.Fatalf("expected <=3 steps for max_steps=3, got %d", len(resp.AgentSteps))
	}
	if resp.FinalAnswer == "" {
		t.Fatal("expected final_answer when max steps reached")
	}
	if strings.Contains(resp.FinalAnswer, "已达到最大推理步数限制") || strings.Contains(resp.FinalAnswer, "拆分后分别查询") {
		t.Fatalf("max steps should produce evidence-based summary, got %q", resp.FinalAnswer)
	}
	if !strings.Contains(resp.FinalAnswer, "已检查的证据") || !strings.Contains(resp.FinalAnswer, "阶段性判断") {
		t.Fatalf("expected evidence-based max-step summary, got %q", resp.FinalAnswer)
	}
	if resp.Confidence != "medium" {
		t.Fatalf("expected medium confidence for bounded summary, got %q", resp.Confidence)
	}
}

func TestToolStepExecutorPolicy(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	exec := NewToolStepExecutor(registry)

	// Unknown tool should be denied.
	allowed, reason := exec.CheckToolPolicy("nonexistent_tool", nil)
	if allowed {
		t.Fatal("expected nonexistent_tool to be denied by policy")
	}
	if reason == "" {
		t.Fatal("expected non-empty policy reason")
	}
}

func TestEngineEmptyTaskReturnsStepZero(t *testing.T) {
	gen := &mockGenerator{}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty task should result in some answer.
	if resp.FinalAnswer == "" {
		t.Fatal("expected non-empty final_answer")
	}
}

func TestEngineProducesPerStepAuditReports(t *testing.T) {
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "service_status", ToolArgs: map[string]any{"service_name": "sshd"}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "port_checker", ToolArgs: map[string]any{"port": 22}},
			{ActionType: "final_answer", FinalAnswer: "完成排查"},
		},
	}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngineWithAuditor(gen, exec, tools.NewDefaultRegistry(), mockStepAuditor{})

	resp, err := engine.Run(context.Background(), "ssh 排查", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, step := range resp.AgentSteps {
		if step.AuditReport == nil {
			t.Fatalf("step %d missing audit_report", i)
		}
		if step.AuditReport.Decision == "" {
			t.Fatalf("step %d audit_report has empty decision", i)
		}
		if step.AuditReport.StepID == "" {
			t.Fatalf("step %d audit_report has empty step_id", i)
		}
	}
}

func TestEngineBuildsRiskGraph(t *testing.T) {
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "service_status", ToolArgs: map[string]any{"service_name": "sshd"}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "port_checker", ToolArgs: map[string]any{"port": 22}},
			{ActionType: "final_answer", FinalAnswer: "完成排查"},
		},
	}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngineWithAuditor(gen, exec, tools.NewDefaultRegistry(), mockStepAuditor{})

	resp, err := engine.Run(context.Background(), "ssh 排查", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RiskGraph == nil {
		t.Fatal("expected non-nil risk_graph for multi-step run")
	}
	for _, step := range resp.AgentSteps {
		if !riskGraphHasNode(resp.RiskGraph, "step_id", step.AuditReport.StepID) {
			t.Fatalf("risk graph missing step node %q: %#v", step.AuditReport.StepID, resp.RiskGraph.Nodes)
		}
	}
	if !riskGraphHasEdgeType(resp.RiskGraph, "performs") ||
		!riskGraphHasEdgeType(resp.RiskGraph, "targets") ||
		!riskGraphHasEdgeType(resp.RiskGraph, "crosses_boundary") ||
		!riskGraphHasEdgeType(resp.RiskGraph, "audited_as") {
		t.Fatalf("expected semantic risk graph edges, got %#v", resp.RiskGraph.Edges)
	}
	if !riskGraphHasEdgeType(resp.RiskGraph, "next_action") {
		t.Fatalf("expected next_action edge for multi-step graph, got %#v", resp.RiskGraph.Edges)
	}
}

func TestEngineDeniedStepHasDenyAuditReport(t *testing.T) {
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "safe_shell", ToolArgs: map[string]any{"command": "ls"}},
			{ActionType: "final_answer", FinalAnswer: "完成排查"},
		},
	}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	// Use NoOp auditor: the deny decision must come from tool_policy, not the auditor.
	engine := NewEngineWithAuditor(gen, exec, tools.NewDefaultRegistry(), NoOpStepAuditor{})

	resp, err := engine.Run(context.Background(), "run ls", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(resp.AgentSteps))
	}
	step := resp.AgentSteps[0]
	if step.AuditReport == nil {
		t.Fatal("denied step missing audit_report")
	}
	if step.AuditReport.Decision != "deny" {
		t.Fatalf("expected denied step audit decision=deny, got %q", step.AuditReport.Decision)
	}
	if step.AuditReport.Method != "tool_policy" {
		t.Fatalf("expected denied step audit method=tool_policy, got %q", step.AuditReport.Method)
	}
	// Denied step must appear as a node in the semantic risk_graph.
	if resp.RiskGraph == nil || !riskGraphHasNode(resp.RiskGraph, "decision", "deny") {
		t.Fatal("expected risk_graph with a denied decision node")
	}
	if !riskGraphHasEdgeType(resp.RiskGraph, "governs") || !riskGraphHasEdgeType(resp.RiskGraph, "audited_as") {
		t.Fatalf("expected denied graph to preserve policy and audit edges, got %#v", resp.RiskGraph.Edges)
	}
}

func riskGraphHasNode(graph *auditclient.RiskGraph, key string, value any) bool {
	for _, node := range graph.Nodes {
		if node[key] == value {
			return true
		}
	}
	return false
}

func riskGraphHasEdgeType(graph *auditclient.RiskGraph, edgeType string) bool {
	for _, edge := range graph.Edges {
		if edge["type"] == edgeType {
			return true
		}
	}
	return false
}

func TestEngineFinalAnswerOnlyHasNilRiskGraph(t *testing.T) {
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "final_answer", FinalAnswer: "直接回答，无需工具"},
		},
	}
	exec := newMockExecutor(tools.NewDefaultRegistry())
	engine := NewEngineWithAuditor(gen, exec, tools.NewDefaultRegistry(), mockStepAuditor{})

	resp, err := engine.Run(context.Background(), "你好", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) != 0 {
		t.Fatalf("expected 0 steps for pure final_answer, got %d", len(resp.AgentSteps))
	}
	if resp.RiskGraph != nil {
		t.Fatalf("expected nil risk_graph for pure final_answer, got %+v", resp.RiskGraph)
	}
}
