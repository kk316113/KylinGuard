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
}

func (a *recordingAuditor) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (auditclient.Result, error) {
	_ = ctx
	a.called = true
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
	if !auditor.called {
		t.Fatal("expected safe task to call audit-core client")
	}
	if response.AuditResult.Method != "traceshield" {
		t.Fatalf("expected traceshield method, got %q", response.AuditResult.Method)
	}
}
