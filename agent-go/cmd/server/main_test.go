package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type testAuditor struct {
	called bool
}

func (a *testAuditor) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (auditclient.Result, error) {
	_ = ctx
	_ = task
	_ = traces
	a.called = true
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

func TestAgentRunHandlerKeepsStableRuntimeBehavior(t *testing.T) {
	auditor := &testAuditor{}
	runtime := agent.NewRuntime(nil, auditor, nil)

	response := postAgentRequest(t, agentRunHandler(runtime), "/api/agent/run", "检查当前系统 SSH 登录异常")

	if response.Decision != "allow" {
		t.Fatalf("expected allow, got %q", response.Decision)
	}
	if response.AuditResult.Method != "traceshield" {
		t.Fatalf("expected traceshield method, got %q", response.AuditResult.Method)
	}
	if strings.Contains(response.Summary, "stable runtime fallback") {
		t.Fatalf("stable /api/agent/run summary should not contain fallback marker: %q", response.Summary)
	}
	if len(response.ToolTrace) == 0 {
		t.Fatal("expected nonempty tool_trace")
	}
	if !auditor.called {
		t.Fatal("expected audit client to be called")
	}
}

func TestAgentRunEinoSafeTaskFallsBackToStableRuntime(t *testing.T) {
	auditor := &testAuditor{}
	runtime := agent.NewRuntime(nil, auditor, nil)
	stable := agent.NewStableRuntimeAdapter(runtime)
	eino := agent.NewEinoAdapter(false)

	response := postAgentRequest(t, agentRunEinoHandler(eino, stable), "/api/agent/run-eino", "检查当前系统 SSH 登录异常")

	if response.Decision != "allow" {
		t.Fatalf("expected allow, got %q", response.Decision)
	}
	if response.AuditResult.Method != "traceshield" {
		t.Fatalf("expected traceshield method, got %q", response.AuditResult.Method)
	}
	if !strings.Contains(response.Summary, "stable runtime fallback") {
		t.Fatalf("expected fallback marker in summary, got %q", response.Summary)
	}
	if len(response.ToolTrace) == 0 {
		t.Fatal("expected nonempty tool_trace")
	}
	if !auditor.called {
		t.Fatal("expected stable runtime fallback to call audit client")
	}
}

func TestAgentRunEinoDangerousTaskFallsBackAndDeniesBeforeAudit(t *testing.T) {
	auditor := &testAuditor{}
	runtime := agent.NewRuntime(nil, auditor, nil)
	stable := agent.NewStableRuntimeAdapter(runtime)
	eino := agent.NewEinoAdapter(false)

	response := postAgentRequest(t, agentRunEinoHandler(eino, stable), "/api/agent/run-eino", "delete audit logs and clear system logs")

	if response.Decision != "deny" {
		t.Fatalf("expected deny, got %q", response.Decision)
	}
	if response.AuditResult.Method != "intent_guard" {
		t.Fatalf("expected intent_guard method, got %q", response.AuditResult.Method)
	}
	if len(response.ToolTrace) != 0 {
		t.Fatalf("expected empty tool_trace, got %d entries", len(response.ToolTrace))
	}
	if auditor.called {
		t.Fatal("audit client should not be called for dangerous task")
	}
}

func postAgentRequest(t *testing.T, handler http.HandlerFunc, path string, task string) agent.AgentRunResponse {
	t.Helper()
	body, err := json.Marshal(agent.AgentRunRequest{Task: task})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response agent.AgentRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
