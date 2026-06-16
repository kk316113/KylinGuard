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

func TestRuntimeSafeSSHAnomalyTaskUsesEinoSkeleton(t *testing.T) {
	auditor := &testAuditor{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "检查当前系统 SSH 登录异常"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if response.Decision != "allow" {
		t.Fatalf("expected allow, got %q", response.Decision)
	}
	if !strings.Contains(response.Summary, RuntimeSummary) {
		t.Fatalf("expected Eino runtime summary, got %q", response.Summary)
	}
	if strings.Contains(response.Summary, "stable runtime fallback used") {
		t.Fatalf("summary should not contain fallback marker: %q", response.Summary)
	}
	if response.AuditResult.Method != "traceshield" {
		t.Fatalf("expected traceshield audit, got %q", response.AuditResult.Method)
	}
	if response.Plan == nil {
		t.Fatal("expected plan")
	}
	assertPlanTools(t, response.Plan, []string{"os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer"})
	if len(response.ToolTrace) < 5 {
		t.Fatalf("expected at least 5 traces, got %d", len(response.ToolTrace))
	}
	if response.SecurityReport == nil {
		t.Fatal("expected security_report")
	}
	metadata := response.SecurityReport.AuditMetadata
	if metadata["route"] != DefaultRoute {
		t.Fatalf("expected route=%s, got %#v", DefaultRoute, metadata["route"])
	}
	if metadata["runtime"] != DefaultRuntimeName {
		t.Fatalf("expected runtime=%s, got %#v", DefaultRuntimeName, metadata["runtime"])
	}
	if metadata["llm_enabled"] != false {
		t.Fatalf("expected llm_enabled=false, got %#v", metadata["llm_enabled"])
	}
	if metadata["tool_protocol"] != tools.ToolProtocol {
		t.Fatalf("expected tool_protocol=%s, got %#v", tools.ToolProtocol, metadata["tool_protocol"])
	}
	if metadata["eino_runtime_version"] != RuntimeVersion {
		t.Fatalf("expected eino runtime version %s, got %#v", RuntimeVersion, metadata["eino_runtime_version"])
	}
	if !auditor.called {
		t.Fatal("expected audit client to be called")
	}
}

func TestRuntimeDangerousTaskDeniedBeforeToolAdapter(t *testing.T) {
	auditor := &testAuditor{}
	adapter := &countingAdapter{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())
	runtime.toolAdapter = adapter

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

type countingAdapter struct {
	calls int
}

func (a *countingAdapter) Execute(ctx context.Context, step agent.PlanStep) (ToolCallResult, error) {
	_ = ctx
	_ = step
	a.calls++
	return ToolCallResult{}, nil
}
