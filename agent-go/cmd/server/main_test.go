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
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
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
	if response.Plan == nil || response.Plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh_anomaly_check plan, got %#v", response.Plan)
	}
	if response.Diagnosis == nil || response.Diagnosis.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh anomaly diagnosis, got %#v", response.Diagnosis)
	}
	if response.SecurityReport == nil || response.SecurityReport.OverallDecision != response.Decision {
		t.Fatalf("expected security_report to preserve decision, got %#v", response.SecurityReport)
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
	if response.Plan == nil || response.Plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected run-eino fallback to use ssh_anomaly_check plan, got %#v", response.Plan)
	}
	if response.Diagnosis == nil || response.Diagnosis.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected run-eino fallback to return diagnosis, got %#v", response.Diagnosis)
	}
	if response.SecurityReport == nil {
		t.Fatal("expected run-eino fallback to return security_report")
	}
	if response.SecurityReport.AuditMetadata["route"] != "eino-fallback" {
		t.Fatalf("expected eino-fallback route, got %#v", response.SecurityReport.AuditMetadata["route"])
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
	if response.Plan != nil {
		t.Fatalf("dangerous run-eino task should not enter planner, got %#v", response.Plan)
	}
	if response.Diagnosis != nil {
		t.Fatalf("dangerous run-eino task should not return diagnosis, got %#v", response.Diagnosis)
	}
	if response.SecurityReport == nil {
		t.Fatal("expected dangerous run-eino task to return security_report")
	}
	if response.SecurityReport.OverallDecision != "deny" {
		t.Fatalf("expected deny report, got %q", response.SecurityReport.OverallDecision)
	}
	if response.SecurityReport.AuditMetadata["route"] != "eino-fallback" {
		t.Fatalf("expected eino-fallback route, got %#v", response.SecurityReport.AuditMetadata["route"])
	}
	if auditor.called {
		t.Fatal("audit client should not be called for dangerous task")
	}
}

func TestToolsListHandler(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	request := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	recorder := httptest.NewRecorder()

	toolsListHandler(registry).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response toolsListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Protocol != tools.ToolProtocol {
		t.Fatalf("expected protocol %q, got %q", tools.ToolProtocol, response.Protocol)
	}
	if response.Version != tools.ToolProtocolVersion {
		t.Fatalf("expected version %q, got %q", tools.ToolProtocolVersion, response.Version)
	}
	if response.Count < 6 {
		t.Fatalf("expected at least 6 tools, got %d", response.Count)
	}
}

func TestToolDetailHandler(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	request := httptest.NewRequest(http.MethodGet, "/api/tools/ssh_login_analyzer", nil)
	recorder := httptest.NewRecorder()

	toolDetailHandler(registry).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response tools.ToolMetadata
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Name != "ssh_login_analyzer" {
		t.Fatalf("expected ssh_login_analyzer, got %q", response.Name)
	}
	if response.BoundaryLevel != "sensitive_system_resource" {
		t.Fatalf("unexpected boundary_level: %q", response.BoundaryLevel)
	}
	if response.InputSchema == nil || response.OutputSchema == nil {
		t.Fatalf("expected input_schema and output_schema, got %#v", response)
	}
}

func TestToolDetailHandlerNotFound(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	request := httptest.NewRequest(http.MethodGet, "/api/tools/unknown", nil)
	recorder := httptest.NewRecorder()

	toolDetailHandler(registry).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestToolCallHandlerUnknownToolDenied(t *testing.T) {
	auditor := &testAuditor{}
	response := postToolCallRequest(t, auditor, toolCallRequest{
		ToolName: "unknown_tool",
		Input:    map[string]any{},
		Reason:   "test unknown tool",
	})

	if response.Status != "denied" {
		t.Fatalf("expected denied, got %q", response.Status)
	}
	if response.AuditResult.Method != security.ToolPolicyMethod {
		t.Fatalf("expected tool_policy method, got %q", response.AuditResult.Method)
	}
	if response.AuditResult.Decision != "deny" {
		t.Fatalf("expected deny audit decision, got %q", response.AuditResult.Decision)
	}
	if auditor.called {
		t.Fatal("audit client should not be called for denied unknown tool")
	}
}

func TestToolCallHandlerPortCheckerAudited(t *testing.T) {
	auditor := &testAuditor{}
	response := postToolCallRequest(t, auditor, toolCallRequest{
		ToolName: "port_checker",
		Input:    map[string]any{"host": "127.0.0.1", "port": 22},
		Reason:   "test direct tool call",
	})

	if response.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", response.Status, response.Message)
	}
	if response.Trace == nil {
		t.Fatal("expected trace")
	}
	if response.Trace.ResourceType != "network_port" {
		t.Fatalf("expected trace.resource_type=network_port, got %q", response.Trace.ResourceType)
	}
	if response.AuditResult.Method != "traceshield" {
		t.Fatalf("expected traceshield audit result, got %q", response.AuditResult.Method)
	}
	if !auditor.called {
		t.Fatal("expected audit client to be called")
	}
}

func TestToolCallHandlerSafeShellDangerousCommandDenied(t *testing.T) {
	auditor := &testAuditor{}
	response := postToolCallRequest(t, auditor, toolCallRequest{
		ToolName: "safe_shell",
		Input:    map[string]any{"command": "rm -rf /"},
		Reason:   "must be denied",
	})

	if response.Status != "denied" {
		t.Fatalf("expected denied, got %q", response.Status)
	}
	if response.Trace != nil {
		t.Fatalf("denied safe_shell should not return trace, got %#v", response.Trace)
	}
	if response.AuditResult.Method != security.ToolPolicyMethod {
		t.Fatalf("expected tool_policy method, got %q", response.AuditResult.Method)
	}
	if auditor.called {
		t.Fatal("audit client should not be called for denied safe_shell")
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

func postToolCallRequest(t *testing.T, auditor *testAuditor, req toolCallRequest) toolCallResponse {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/tools/call", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	recorder := httptest.NewRecorder()

	handler := toolCallHandler(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), security.NewToolPolicy())
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response toolCallResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
