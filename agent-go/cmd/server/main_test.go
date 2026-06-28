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
	"kylin-guard-agent/agent-go/internal/config"
	einoruntime "kylin-guard-agent/agent-go/internal/eino"
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

func TestAgentRunHandlerUsesPrimaryAgentLoopAPI(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())
	store := newAgentRunStore()

	response := postAgentRequest(t, agentRunHandler(eino, store), "/api/agent/run", "检查当前系统 SSH 登录异常")

	assertAgentLoopResponse(t, response)
	if response.AuditResult.Method == "" {
		t.Fatal("expected aggregate audit result method")
	}
	if !auditor.called {
		t.Fatal("expected audit client to be called")
	}
	stored, ok := store.Get(response.RunID)
	if !ok {
		t.Fatalf("expected run %q to be stored", response.RunID)
	}
	if stored.Task != response.Task {
		t.Fatalf("stored run mismatch: %#v", stored)
	}
}

func TestAgentRunEinoCompatibilityUsesAgentLoop(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())

	response := postAgentRequest(t, agentRunEinoHandler(eino, newAgentRunStore()), "/api/agent/run-eino", "检查当前系统 SSH 登录异常")

	assertAgentLoopResponse(t, response)
	metadata := response.SecurityReport.AuditMetadata
	if metadata["route"] != einoruntime.DefaultRoute {
		t.Fatalf("expected eino-runtime route, got %#v", metadata["route"])
	}
	if metadata["runtime"] != einoruntime.DefaultRuntimeName {
		t.Fatalf("expected runtime=eino, got %#v", metadata["runtime"])
	}
	if metadata["eino_graph_enabled"] != true {
		t.Fatalf("expected eino_graph_enabled=true, got %#v", metadata["eino_graph_enabled"])
	}
	if metadata["llm_enabled"] != false {
		t.Fatalf("expected llm_enabled=false, got %#v", metadata["llm_enabled"])
	}
	if metadata["chat_model"] != einoruntime.DefaultChatModel {
		t.Fatalf("expected deterministic stub chat model, got %#v", metadata["chat_model"])
	}
	if metadata["chat_model_adapter"] != einoruntime.DefaultChatModelAdapter {
		t.Fatalf("expected chat_model_adapter=interface-v1, got %#v", metadata["chat_model_adapter"])
	}
	if metadata["orchestration"] != einoruntime.DefaultOrchestration {
		t.Fatalf("expected Eino graph orchestration, got %#v", metadata["orchestration"])
	}
	if metadata["eino_runtime_version"] != einoruntime.RuntimeVersion {
		t.Fatalf("expected Stage 9B runtime version, got %#v", metadata["eino_runtime_version"])
	}
	if !auditor.called {
		t.Fatal("expected agent loop to call audit client")
	}
}

func TestAgentRunQueryEndpoints(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())
	store := newAgentRunStore()

	run := postAgentRequest(t, agentRunHandler(eino, store), "/api/agent/run", "检查当前系统 SSH 登录异常")

	getJSON(t, agentRunsHandler(store), "/api/agent/runs/"+run.RunID, http.StatusOK, &agent.AgentRunResponse{})
	var auditReports auditReportsResponse
	getJSON(t, agentRunsHandler(store), "/api/agent/runs/"+run.RunID+"/audit-reports", http.StatusOK, &auditReports)
	if auditReports.RunID != run.RunID || len(auditReports.AuditReports) == 0 {
		t.Fatalf("expected audit reports for run, got %#v", auditReports)
	}
	var graph riskGraphResponse
	getJSON(t, agentRunsHandler(store), "/api/agent/runs/"+run.RunID+"/risk-graph", http.StatusOK, &graph)
	if graph.RunID != run.RunID || graph.RiskGraph == nil {
		t.Fatalf("expected risk graph response, got %#v", graph)
	}
	var report runReportResponse
	getJSON(t, agentRunsHandler(store), "/api/agent/runs/"+run.RunID+"/report", http.StatusOK, &report)
	if report.RunID != run.RunID || report.FinalAnswer == "" || report.Counts["agent_steps"] == 0 {
		t.Fatalf("expected run report response, got %#v", report)
	}
}

func TestAgentRunListExactAndSlashEndpoints(t *testing.T) {
	store := newAgentRunStore()
	store.Save(sampleStoredRun("kg-list", "检查历史", "review"))

	var exact agentRunListResponse
	getJSON(t, agentRunListHandler(store), "/api/agent/runs?limit=10", http.StatusOK, &exact)
	if exact.Count != 1 || exact.Runs[0].RunID != "kg-list" {
		t.Fatalf("expected exact list response, got %#v", exact)
	}

	var slash agentRunListResponse
	getJSON(t, agentRunsHandler(store), "/api/agent/runs/?limit=10", http.StatusOK, &slash)
	if slash.Count != 1 || slash.Runs[0].RunID != "kg-list" {
		t.Fatalf("expected slash list response, got %#v", slash)
	}
}

func TestAgentRunQueryEndpointNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/agent/runs/kg-missing", nil)
	recorder := httptest.NewRecorder()

	agentRunsHandler(newAgentRunStore()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAgentRunEinoNormalChatDoesNotExecuteToolsOrAudit(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())

	response := postAgentRequest(t, agentRunEinoHandler(eino, newAgentRunStore()), "/api/agent/run-eino", "你好呀，请你回答我的问题")

	if response.InteractionType != agent.InteractionTypeChat || response.AgentMode != agent.AgentModeChatOnly {
		t.Fatalf("expected chat-only response, got interaction=%q mode=%q", response.InteractionType, response.AgentMode)
	}
	if response.FinalAnswer == "" || response.UserMessage == nil || response.UserMessage.Answer == "" {
		t.Fatalf("expected readable chat answer, got final=%q message=%#v", response.FinalAnswer, response.UserMessage)
	}
	if len(response.AgentSteps) != 0 {
		t.Fatalf("normal chat should not return agent steps, got %d", len(response.AgentSteps))
	}
	if len(response.ToolTrace) != 0 {
		t.Fatalf("normal chat should not execute tools, got %d traces", len(response.ToolTrace))
	}
	if response.SecurityReport != nil {
		t.Fatalf("normal chat should not produce security report, got %#v", response.SecurityReport)
	}
	if auditor.called {
		t.Fatal("normal chat should not call audit client")
	}
}

func TestAgentRunPrimaryNormalChatReturnsReadableAnswer(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())

	response := postAgentRequest(t, agentRunHandler(eino, newAgentRunStore()), "/api/agent/run", "hello")

	if response.InteractionType != agent.InteractionTypeChat || response.AgentMode != agent.AgentModeChatOnly {
		t.Fatalf("expected chat-only response, got interaction=%q mode=%q", response.InteractionType, response.AgentMode)
	}
	if strings.TrimSpace(response.FinalAnswer) == "" {
		t.Fatal("expected non-empty final_answer")
	}
	if len(response.AgentSteps) != 0 || len(response.ToolTrace) != 0 {
		t.Fatalf("normal chat must not execute tools, got steps=%d traces=%d", len(response.AgentSteps), len(response.ToolTrace))
	}
	if auditor.called {
		t.Fatal("normal chat must not call audit client")
	}
}

func TestAgentRunRejectsEmptyTask(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, &testAuditor{}, nil, einoruntime.DefaultRuntimeConfig())
	request := httptest.NewRequest(http.MethodPost, "/api/agent/run", strings.NewReader(`{"task":"   "}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	recorder := httptest.NewRecorder()

	agentRunHandler(eino, newAgentRunStore()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "task is required") {
		t.Fatalf("expected clear validation error, got %s", recorder.Body.String())
	}
	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected structured JSON error: %v", err)
	}
	if payload.Error.Code != "INVALID_REQUEST" || payload.Error.Message == "" || payload.Error.Details == nil {
		t.Fatalf("unexpected structured error: %#v", payload.Error)
	}
}

func TestAgentRunEinoNonOperationalInputDefaultsToChatWithoutTools(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())

	response := postAgentRequest(t, agentRunEinoHandler(eino, newAgentRunStore()), "/api/agent/run-eino", "你帮我看看")

	if response.InteractionType != agent.InteractionTypeChat {
		t.Fatalf("expected safe chat response, got %q", response.InteractionType)
	}
	if response.FinalAnswer == "" || response.UserMessage == nil {
		t.Fatalf("expected chat answer, got final=%q message=%#v", response.FinalAnswer, response.UserMessage)
	}
	if len(response.ToolTrace) != 0 || len(response.AgentSteps) != 0 {
		t.Fatalf("ambiguous input should not execute tools, got traces=%d steps=%d", len(response.ToolTrace), len(response.AgentSteps))
	}
	if response.SecurityReport != nil {
		t.Fatalf("ambiguous input should not produce security_report, got %#v", response.SecurityReport)
	}
	if auditor.called {
		t.Fatal("ambiguous input should not call audit client")
	}
}

func TestAgentRunEinoDangerousTaskDeniesBeforeAudit(t *testing.T) {
	auditor := &testAuditor{}
	registry := tools.NewDefaultRegistry()
	eino := einoruntime.NewRuntime(registry, auditor, nil, einoruntime.DefaultRuntimeConfig())

	response := postAgentRequest(t, agentRunEinoHandler(eino, newAgentRunStore()), "/api/agent/run-eino", "delete audit logs and clear system logs")

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
	if response.FinalAnswer == "" || response.UserMessage == nil || response.UserMessage.Status != agent.RunStatusBlocked {
		t.Fatalf("expected blocked user-facing answer, got final=%q message=%#v", response.FinalAnswer, response.UserMessage)
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

func TestRuntimeStatusHandlerDoesNotExposeAPIKey(t *testing.T) {
	cfg := config.Load()
	cfg.EinoLLMEnabled = true
	cfg.EinoLLMProvider = "openai_compatible"
	cfg.EinoLLMEndpoint = "https://api.deepseek.com"
	cfg.EinoLLMModel = "deepseek-v4-flash"
	cfg.EinoLLMAPIKey = "secret-test-key"
	cfg.AuditCoreURL = ""

	request := httptest.NewRequest(http.MethodGet, "/api/agent/runtime-status", nil)
	recorder := httptest.NewRecorder()
	runtimeStatusHandler(cfg).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "secret-test-key") {
		t.Fatal("runtime-status leaked API key")
	}
	var response runtimeStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.SecretSafety.APIKeyPresent || response.SecretSafety.APIKeyDisplay != "[REDACTED]" {
		t.Fatalf("expected redacted API key metadata, got %#v", response.SecretSafety)
	}
	if response.Runtime.CurrentMode != "real-deepseek" {
		t.Fatalf("expected real-deepseek mode, got %q", response.Runtime.CurrentMode)
	}
}

func TestCapabilitiesHandlerUsesRegisteredTools(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	request := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	recorder := httptest.NewRecorder()
	capabilitiesHandler(registry, 10).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response capabilitiesResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expected := len(registry.ListDirectCallTools())
	foundDrift := false
	for _, tool := range response.AvailableTools {
		if tool.ToolName == "safe_shell" {
			t.Fatal("disabled safe_shell must not be advertised as available")
		}
		if !tool.ReadOnly {
			t.Fatalf("available tool %s should be read-only", tool.ToolName)
		}
		if tool.ToolName == "configuration_drift_detector" {
			foundDrift = true
		}
	}
	if len(response.AvailableTools) != expected || !foundDrift {
		t.Fatalf("expected %d enabled tools including configuration drift, got %#v", expected, response.AvailableTools)
	}
	if !response.ToolPolicy.Enabled || !response.ToolPolicy.DangerousActionsBlocked {
		t.Fatalf("expected enabled tool policy, got %#v", response.ToolPolicy)
	}
	if response.AgentLoop.MaxSteps != 10 {
		t.Fatalf("expected configured max_steps=10, got %d", response.AgentLoop.MaxSteps)
	}
}

func TestAcceptanceSummaryHandlerIsStaticMetadata(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/agent/acceptance-summary", nil)
	recorder := httptest.NewRecorder()
	acceptanceSummaryHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response acceptanceSummaryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Stages) == 0 || len(response.Commands) == 0 {
		t.Fatalf("expected acceptance metadata, got %#v", response)
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

func assertAgentLoopResponse(t *testing.T, response agent.AgentRunResponse) {
	t.Helper()
	if response.Decision != "allow" && response.Decision != "review" {
		t.Fatalf("expected allow or review, got %q", response.Decision)
	}
	if response.AgentMode != "agent_loop" {
		t.Fatalf("expected agent_loop mode, got %q", response.AgentMode)
	}
	if response.RunID == "" || response.TaskID == "" || response.CreatedAt == "" {
		t.Fatalf("expected run metadata, got run=%q task=%q created=%q", response.RunID, response.TaskID, response.CreatedAt)
	}
	if response.SceneType == "" || response.SceneSummary == "" || response.RunStatus == "" {
		t.Fatalf("expected scene metadata, got type=%q summary=%q status=%q", response.SceneType, response.SceneSummary, response.RunStatus)
	}
	if strings.TrimSpace(response.Summary) == "" {
		t.Fatal("expected nonempty user-facing summary")
	}
	if strings.Contains(response.Summary, "stable runtime fallback") {
		t.Fatalf("summary should not contain fallback marker: %q", response.Summary)
	}
	if response.FinalAnswer == "" || response.UserMessage == nil || response.UserMessage.Answer == "" {
		t.Fatalf("expected user-facing final answer, got final=%q message=%#v", response.FinalAnswer, response.UserMessage)
	}
	if len(response.AgentSteps) == 0 {
		t.Fatal("expected agent_steps for agent loop run")
	}
	for i, step := range response.AgentSteps {
		ar, ok := step["audit_report"]
		if !ok {
			t.Fatalf("step %d missing audit_report", i)
		}
		if m, ok := ar.(map[string]any); !ok || m["decision"] == "" {
			t.Fatalf("step %d audit_report missing decision", i)
		}
	}
	if len(response.ToolTrace) == 0 {
		t.Fatal("expected nonempty tool_trace")
	}
	if response.RiskGraph == nil || len(response.RiskGraph.Nodes) != len(response.AgentSteps) {
		t.Fatalf("expected risk_graph with %d nodes", len(response.AgentSteps))
	}
	if response.SecurityReport == nil {
		t.Fatal("expected security_report")
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

func getJSON(t *testing.T, handler http.HandlerFunc, path string, wantStatus int, out any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("expected HTTP %d, got %d: %s", wantStatus, recorder.Code, recorder.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(recorder.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
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
