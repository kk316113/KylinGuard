package auditclient

import (
	"context"
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
	if len(result.Violations) != 0 || len(result.EvidenceChain) != 0 {
		t.Fatalf("fallback must not invent violations or evidence: %#v", result)
	}
	if result.RiskGraph == nil || len(result.RiskGraph.Nodes) != 1 || result.RiskGraph.Nodes[0]["step_id"] != trace.StepID {
		t.Fatalf("fallback graph must derive from the retained trace: %#v", result.RiskGraph)
	}
}
