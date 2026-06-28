package auditclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

func TestLocalSafetyFallbackUsesOnlyRetainedTraces(t *testing.T) {
	trace := logtrace.ToolTrace{
		StepID: "step-1", ToolName: "os_info", Status: "ok",
		StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
		OperationType: "read", ResourceType: "os_info", ResourcePath: "system:os",
		BoundaryLevel: "public", AllowedByPolicy: true,
	}
	result, err := NewLocalSafetyClient().AuditTrace(context.Background(), "inspect OS", []logtrace.ToolTrace{trace})
	if err != nil {
		t.Fatalf("unexpected fallback error: %v", err)
	}
	if result.Method != "local-safety-fallback" || result.Decision != "review" {
		t.Fatalf("fallback must be explicitly conservative: %#v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("public read-only fallback should not invent violations: %#v", result)
	}
	if len(result.EvidenceChain) != 1 || result.EvidenceChain[0].StepID != trace.StepID {
		t.Fatalf("fallback evidence must derive from retained traces: %#v", result.EvidenceChain)
	}
	if result.RiskGraph == nil || len(result.RiskGraph.Nodes) != 1 || result.RiskGraph.Nodes[0]["step_id"] != trace.StepID {
		t.Fatalf("fallback graph must derive from the retained trace: %#v", result.RiskGraph)
	}
}

func TestLocalSafetyFallbackFlagsDeniedTrace(t *testing.T) {
	trace := logtrace.ToolTrace{
		StepID:          "step-deny",
		ToolName:        "safe_shell",
		Status:          "denied",
		OperationType:   "execute",
		ResourceType:    "shell_command",
		BoundaryLevel:   "dangerous",
		AllowedByPolicy: false,
		PolicyReason:    "safe_shell direct call is disabled",
	}

	result, err := NewLocalSafetyClient().AuditTrace(context.Background(), "run shell", []logtrace.ToolTrace{trace})
	if err != nil {
		t.Fatalf("unexpected fallback error: %v", err)
	}
	if result.Decision != "deny" || result.RiskScore < 1.0 {
		t.Fatalf("denied trace should become high-risk deny, got %#v", result)
	}
	if len(result.Violations) == 0 {
		t.Fatalf("expected violation for denied trace: %#v", result)
	}
	if result.RiskGraph == nil || result.RiskGraph.Nodes[0]["risk_level"] != "high" {
		t.Fatalf("expected high-risk graph node, got %#v", result.RiskGraph)
	}
}

func TestHTTPClientEnrichesSparseAuditCoreResultWithTraceEvidence(t *testing.T) {
	trace := logtrace.ToolTrace{
		StepID: "step-1", ToolName: "service_status", Status: "ok",
		OperationType: "inspect", ResourceType: "system_service", ResourcePath: "systemd:sshd",
		BoundaryLevel: "low", AllowedByPolicy: true, OutputSummary: "service sshd is active",
		StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Result{
			Decision: "allow",
			Method:   "traceshield",
			Message:  "sparse audit-core response",
		})
	}))
	defer server.Close()

	result, err := NewHTTPClient(server.URL).AuditTrace(context.Background(), "inspect service", []logtrace.ToolTrace{trace})
	if err != nil {
		t.Fatalf("unexpected HTTP audit error: %v", err)
	}
	if result.Decision != "allow" || result.Method != "traceshield" {
		t.Fatalf("TraceShield decision/method should be preserved, got %#v", result)
	}
	if len(result.EvidenceChain) != 1 || result.EvidenceChain[0].Reason != trace.OutputSummary {
		t.Fatalf("expected trace-backed evidence, got %#v", result.EvidenceChain)
	}
	if result.RiskGraph == nil || len(result.RiskGraph.Nodes) != 1 {
		t.Fatalf("expected trace-backed risk graph, got %#v", result.RiskGraph)
	}
}
