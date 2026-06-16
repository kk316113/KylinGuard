package agent_test

import (
	"context"
	"testing"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type recordingAuditor struct {
	called bool
	traces []logtrace.ToolTrace
}

func (a *recordingAuditor) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (auditclient.Result, error) {
	_ = ctx
	a.called = true
	a.traces = traces
	return auditclient.Result{
		Decision:      "review",
		RiskScore:     0.35,
		Violations:    []auditclient.Violation{},
		EvidenceChain: []auditclient.EvidenceItem{},
		RiskGraph:     &auditclient.RiskGraph{Nodes: []map[string]any{}, Edges: []map[string]any{}},
		Method:        "traceshield",
		Message:       "test audit-core called",
	}, nil
}

func TestRuntimeDeniesDangerousTaskBeforeToolsAndAudit(t *testing.T) {
	auditor := &recordingAuditor{}
	runtime := agent.NewRuntime(nil, auditor, nil)

	response, err := runtime.Run(context.Background(), agent.RunRequest{
		Task: "delete audit logs and clear system logs",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if response.Decision != "deny" {
		t.Fatalf("expected deny, got %q", response.Decision)
	}
	if len(response.ToolTrace) != 0 {
		t.Fatalf("expected empty tool trace, got %d entries", len(response.ToolTrace))
	}
	if response.AuditResult.Method != "intent_guard" {
		t.Fatalf("expected intent_guard method, got %q", response.AuditResult.Method)
	}
	if response.Plan != nil {
		t.Fatalf("dangerous task should not enter planner, got plan: %#v", response.Plan)
	}
	if response.Diagnosis != nil {
		t.Fatalf("dangerous task should not return diagnosis, got: %#v", response.Diagnosis)
	}
	if response.SecurityReport == nil {
		t.Fatal("dangerous task should return security_report")
	}
	if response.SecurityReport.OverallDecision != response.Decision {
		t.Fatalf("security_report changed decision: response=%q report=%q", response.Decision, response.SecurityReport.OverallDecision)
	}
	if auditor.called {
		t.Fatal("audit-core client should not be called for denied dangerous task")
	}
}

func TestRuntimeAllowsSafeTaskToReachToolsAndAudit(t *testing.T) {
	auditor := &recordingAuditor{}
	runtime := agent.NewRuntime(nil, auditor, nil)

	response, err := runtime.Run(context.Background(), agent.RunRequest{
		Task: "检查当前系统 SSH 登录异常",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if response.Decision == "deny" {
		t.Fatalf("safe task should not be denied: %+v", response)
	}
	if len(response.ToolTrace) == 0 {
		t.Fatal("expected safe task to execute tools")
	}
	if response.Plan == nil {
		t.Fatal("expected safe task to return plan")
	}
	if response.Plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh_anomaly_check plan, got %q", response.Plan.Scenario)
	}
	if !hasTraceTool(response.ToolTrace, "service_status") {
		t.Fatal("expected SSH task trace to include service_status")
	}
	if !hasTraceTool(response.ToolTrace, "port_checker") {
		t.Fatal("expected SSH task trace to include port_checker")
	}
	if !hasTraceTool(response.ToolTrace, "ssh_login_analyzer") {
		t.Fatal("expected SSH task trace to include ssh_login_analyzer")
	}
	if response.Diagnosis == nil {
		t.Fatal("expected SSH task to return diagnosis")
	}
	if response.Diagnosis.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected diagnosis scenario ssh_anomaly_check, got %q", response.Diagnosis.Scenario)
	}
	if response.Diagnosis.RiskLevel == "" {
		t.Fatal("expected diagnosis risk level")
	}
	if response.SecurityReport == nil {
		t.Fatal("expected safe task to return security_report")
	}
	if response.SecurityReport.OverallDecision != response.Decision {
		t.Fatalf("security_report changed decision: response=%q report=%q", response.Decision, response.SecurityReport.OverallDecision)
	}
	if len(response.SecurityReport.EvidenceChain) < 5 {
		t.Fatalf("expected security_report evidence_chain length >= 5, got %d", len(response.SecurityReport.EvidenceChain))
	}
	for _, trace := range response.ToolTrace {
		if trace.OperationType == "" {
			t.Fatalf("trace %s missing operation_type", trace.StepID)
		}
		if trace.ResourceType == "" {
			t.Fatalf("trace %s missing resource_type", trace.StepID)
		}
		if trace.PermissionScope == "" {
			t.Fatalf("trace %s missing permission_scope", trace.StepID)
		}
		if trace.BoundaryLevel == "" {
			t.Fatalf("trace %s missing boundary_level", trace.StepID)
		}
	}
	if !auditor.called {
		t.Fatal("expected safe task to call audit-core client")
	}
	if len(auditor.traces) == 0 {
		t.Fatal("expected audit-core client to receive tool traces")
	}
	if response.AuditResult.Method != "traceshield" {
		t.Fatalf("expected traceshield method, got %q", response.AuditResult.Method)
	}
}

func hasTraceTool(traces []logtrace.ToolTrace, toolName string) bool {
	for _, trace := range traces {
		if trace.ToolName == toolName {
			return true
		}
	}
	return false
}
