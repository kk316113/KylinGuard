package eino

import (
	"context"
	"strings"
	"testing"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type testAuditor struct {
	called bool
	task   string
	traces []logtrace.ToolTrace
}

func (a *testAuditor) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (auditclient.Result, error) {
	_ = ctx
	a.called = true
	a.task = task
	a.traces = append([]logtrace.ToolTrace{}, traces...)
	return auditclient.Result{
		Decision:      "allow",
		RiskScore:     0.1,
		Violations:    []auditclient.Violation{},
		EvidenceChain: []auditclient.EvidenceItem{},
		RiskGraph:     &auditclient.RiskGraph{Nodes: []map[string]any{}, Edges: []map[string]any{}},
		Method:        "traceshield",
		Message:       "test audit-core called",
	}, nil
}

func TestRuntimeSafeSSHAnomalyTaskUsesEinoGraphRuntime(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	runtime := NewRuntime(registry, auditor, logtrace.NewStore(), DefaultRuntimeConfig())

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "检查当前系统 SSH 登录异常"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// run-eino now always uses the Agent Loop (req 1). With no remote LLM configured,
	// the deterministic adapter drives the loop, so the response carries agent_loop
	// mode, agent_steps with per-step audit_reports, and an aggregated risk_graph.
	if response.AgentMode != "agent_loop" {
		t.Fatalf("expected agent_loop mode, got %q", response.AgentMode)
	}
	// run-eino always runs the agent loop; its summary mentions agent loop orchestration.
	if !strings.Contains(response.Summary, "agent loop") {
		t.Fatalf("expected agent loop summary, got %q", response.Summary)
	}
	if strings.Contains(response.Summary, "stable runtime fallback used") {
		t.Fatalf("summary should not contain fallback marker: %q", response.Summary)
	}
	if len(response.AgentSteps) == 0 {
		t.Fatal("expected agent_steps for agent loop run")
	}
	for i, step := range response.AgentSteps {
		ar, ok := step["audit_report"]
		if !ok {
			t.Fatalf("step %d missing audit_report", i)
		}
		auditMap, ok := ar.(map[string]any)
		if !ok {
			t.Fatalf("step %d audit_report not a map", i)
		}
		if auditMap["decision"] == "" {
			t.Fatalf("step %d audit_report has empty decision", i)
		}
		if auditMap["method"] == "" {
			t.Fatalf("step %d audit_report has empty method", i)
		}
	}
	// req 8: aggregated risk_graph with one node per step and sequence edges.
	if response.RiskGraph == nil {
		t.Fatal("expected non-nil risk_graph")
	}
	if len(response.RiskGraph.Nodes) != len(response.AgentSteps) {
		t.Fatalf("expected %d risk_graph nodes, got %d", len(response.AgentSteps), len(response.RiskGraph.Nodes))
	}
	wantEdges := len(response.AgentSteps) - 1
	if len(response.RiskGraph.Edges) != wantEdges {
		t.Fatalf("expected %d risk_graph edges, got %d", wantEdges, len(response.RiskGraph.Edges))
	}
	if response.SecurityReport == nil {
		t.Fatal("expected security_report")
	}
	metadata := response.SecurityReport.AuditMetadata
	assertRuntimeMetadata(t, metadata, registry, true)
	// req 7: audit happens once per executed tool_call.
	if !auditor.called {
		t.Fatal("expected audit client to be called (per step)")
	}
}

func TestRuntimeDangerousTaskDeniedBeforeGraph(t *testing.T) {
	auditor := &testAuditor{}
	adapter := &countingAdapter{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())
	runtime.toolAdapter = adapter
	runtime.graphRuntime = NewGraphRuntime(runtime.chatModel, adapter)

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "delete audit logs and clear system logs"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if response.Decision != "deny" {
		t.Fatalf("expected deny, got %q", response.Decision)
	}
	if response.AuditResult.Method != "intent_guard" {
		t.Fatalf("expected intent_guard, got %q", response.AuditResult.Method)
	}
	if len(response.ToolTrace) != 0 {
		t.Fatalf("expected empty tool_trace, got %d", len(response.ToolTrace))
	}
	if adapter.calls != 0 {
		t.Fatalf("tool adapter should not be called for dangerous task, got %d calls", adapter.calls)
	}
	if auditor.called {
		t.Fatal("audit client should not be called for dangerous task")
	}
	if response.SecurityReport == nil || response.SecurityReport.Title != "KylinGuard Dangerous Intent Audit Report" {
		t.Fatalf("expected dangerous intent report, got %#v", response.SecurityReport)
	}
}

func TestMCPToolAdapterPolicyAndExecution(t *testing.T) {
	adapter := NewMCPToolAdapter(tools.NewDefaultRegistry(), security.NewToolPolicy())
	ctx := context.Background()

	portResult, err := adapter.Execute(ctx, agent.PlanStep{
		StepID:   "plan-001",
		ToolName: "port_checker",
		Input:    map[string]any{"host": "127.0.0.1", "port": 22},
	})
	if err != nil {
		t.Fatalf("port_checker should be allowed, got error: %v", err)
	}
	if portResult.Trace.ResourceType != "network_port" {
		t.Fatalf("expected network_port trace, got %q", portResult.Trace.ResourceType)
	}

	assertAdapterDeny(t, adapter, agent.PlanStep{StepID: "plan-002", ToolName: "unknown_tool", Input: map[string]any{}})
	assertAdapterDeny(t, adapter, agent.PlanStep{StepID: "plan-003", ToolName: "safe_shell", Input: map[string]any{"command": "rm -rf /"}})
	assertAdapterDeny(t, adapter, agent.PlanStep{StepID: "plan-004", ToolName: "log_reader", Input: map[string]any{"paths": []any{"/etc/shadow"}, "lines": 100}})
	assertAdapterDeny(t, adapter, agent.PlanStep{StepID: "plan-005", ToolName: "service_status", Input: map[string]any{"service_name": "sshd; rm -rf /"}})
	assertAdapterDeny(t, adapter, agent.PlanStep{StepID: "plan-006", ToolName: "port_checker", Input: map[string]any{"host": "127.0.0.1", "port": 70000}})
}

func assertAdapterDeny(t *testing.T, adapter *MCPToolAdapter, step agent.PlanStep) {
	t.Helper()
	result, err := adapter.Execute(context.Background(), step)
	if err == nil {
		t.Fatalf("expected adapter deny for %s", step.ToolName)
	}
	if result.Trace.Status != "error" {
		t.Fatalf("expected error trace for %s, got %q", step.ToolName, result.Trace.Status)
	}
	if result.Trace.AllowedByPolicy {
		t.Fatalf("expected allowed_by_policy=false for %s", step.ToolName)
	}
	if !strings.Contains(result.Trace.PolicyReason, "tool call denied by tool policy") {
		t.Fatalf("expected tool policy reason, got %q", result.Trace.PolicyReason)
	}
}

func assertPlanTools(t *testing.T, plan *agent.Plan, expected []string) {
	t.Helper()
	if len(plan.Steps) != len(expected) {
		t.Fatalf("expected %d steps, got %d", len(expected), len(plan.Steps))
	}
	for index, toolName := range expected {
		if plan.Steps[index].ToolName != toolName {
			t.Fatalf("step %d expected %q, got %q", index, toolName, plan.Steps[index].ToolName)
		}
	}
}

func assertRuntimeMetadata(t *testing.T, metadata map[string]any, registry *tools.Registry, expectToolsUsed bool) {
	t.Helper()
	if metadata["route"] != DefaultRoute {
		t.Fatalf("expected route=%s, got %#v", DefaultRoute, metadata["route"])
	}
	if metadata["runtime"] != DefaultRuntimeName {
		t.Fatalf("expected runtime=%s, got %#v", DefaultRuntimeName, metadata["runtime"])
	}
	if metadata["llm_enabled"] != false {
		t.Fatalf("expected llm_enabled=false, got %#v", metadata["llm_enabled"])
	}
	if metadata["eino_graph_enabled"] != true {
		t.Fatalf("expected eino_graph_enabled=true, got %#v", metadata["eino_graph_enabled"])
	}
	if metadata["chat_model"] != DefaultChatModel {
		t.Fatalf("expected chat_model=%s, got %#v", DefaultChatModel, metadata["chat_model"])
	}
	if metadata["chat_model_adapter"] != DefaultChatModelAdapter {
		t.Fatalf("expected chat_model_adapter=%s, got %#v", DefaultChatModelAdapter, metadata["chat_model_adapter"])
	}
	if metadata["orchestration"] != DefaultOrchestration {
		t.Fatalf("expected orchestration=%s, got %#v", DefaultOrchestration, metadata["orchestration"])
	}
	if metadata["tool_protocol"] != tools.ToolProtocol {
		t.Fatalf("expected tool_protocol=%s, got %#v", tools.ToolProtocol, metadata["tool_protocol"])
	}
	if metadata["tool_protocol_version"] != tools.ToolProtocolVersion {
		t.Fatalf("expected tool_protocol_version=%s, got %#v", tools.ToolProtocolVersion, metadata["tool_protocol_version"])
	}
	if metadata["eino_runtime_version"] != RuntimeVersion {
		t.Fatalf("expected eino runtime version %s, got %#v", RuntimeVersion, metadata["eino_runtime_version"])
	}
	if registry != nil && metadata["registered_tool_count"] != len(registry.ListTools()) {
		t.Fatalf("expected registered_tool_count=%d, got %#v", len(registry.ListTools()), metadata["registered_tool_count"])
	}
	if expectToolsUsed {
		if used, ok := metadata["tools_used"].([]string); !ok || len(used) == 0 {
			t.Fatalf("expected tools_used metadata, got %#v", metadata["tools_used"])
		}
	}
}

type countingAdapter struct {
	calls int
}

func (a *countingAdapter) Execute(ctx context.Context, step agent.PlanStep) (ToolCallResult, error) {
	_ = ctx
	_ = step
	a.calls++
	return ToolCallResult{}, nil
}
