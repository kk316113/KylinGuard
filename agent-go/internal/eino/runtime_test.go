package eino

import (
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
	if strings.TrimSpace(response.Summary) == "" {
		t.Fatal("expected nonempty user-facing summary")
	}
	if strings.Contains(response.Summary, "stable runtime fallback used") {
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
	// req 8: aggregated semantic risk_graph with per-step and semantic nodes.
	if response.RiskGraph == nil {
		t.Fatal("expected non-nil risk_graph")
	}
	if response.AuditResult.RiskGraph == nil {
		t.Fatal("expected aggregate audit_result risk_graph")
	}
	for _, step := range response.AgentSteps {
		toolName, _ := step["tool_name"].(string)
		if !runtimeGraphHasNode(response.RiskGraph, "tool_name", toolName) {
			t.Fatalf("risk_graph missing tool node %q: %#v", toolName, response.RiskGraph.Nodes)
		}
	}
	if !runtimeGraphHasEdgeType(response.RiskGraph, "performs") ||
		!runtimeGraphHasEdgeType(response.RiskGraph, "targets") ||
		!runtimeGraphHasEdgeType(response.RiskGraph, "audited_as") {
		t.Fatalf("expected semantic graph edges, got %#v", response.RiskGraph.Edges)
	}
	if len(response.AuditResult.EvidenceChain) == 0 {
		t.Fatal("expected aggregate audit evidence_chain")
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

func TestRuntimeNormalChatDoesNotEnterGraphOrAudit(t *testing.T) {
	auditor := &testAuditor{}
	adapter := &countingAdapter{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())
	runtime.toolAdapter = adapter
	runtime.graphRuntime = NewGraphRuntime(runtime.chatModel, adapter)

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "你好呀请你回答我的问题"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if response.InteractionType != agent.InteractionTypeChat || response.AgentMode != agent.AgentModeChatOnly {
		t.Fatalf("expected chat-only response, got interaction=%q mode=%q", response.InteractionType, response.AgentMode)
	}
	if response.FinalAnswer == "" || response.UserMessage == nil || response.UserMessage.Answer == "" {
		t.Fatalf("expected readable chat answer, got final=%q message=%#v", response.FinalAnswer, response.UserMessage)
	}
	if len(response.ToolTrace) != 0 || len(response.AgentSteps) != 0 {
		t.Fatalf("normal chat should not execute tools, got traces=%d steps=%d", len(response.ToolTrace), len(response.AgentSteps))
	}
	if response.SecurityReport != nil {
		t.Fatalf("normal chat should not produce security_report, got %#v", response.SecurityReport)
	}
	if adapter.calls != 0 {
		t.Fatalf("tool adapter should not be called for normal chat, got %d calls", adapter.calls)
	}
	if auditor.called {
		t.Fatal("audit client should not be called for normal chat")
	}
}

func TestRuntimeRemoteChatUsesChatModelWithoutTools(t *testing.T) {
	callCount := 0
	sawHistory := false
	sawModelMetadata := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if callCount == 2 {
			for _, message := range request.Messages {
				if message.Role == "assistant" && message.Content == "我叫麒盾。" {
					sawHistory = true
				}
				if message.Role == "system" && strings.Contains(message.Content, "test-model") {
					sawModelMetadata = true
				}
			}
		}
		content := "我是测试模型返回的正常聊天回答。"
		if callCount == 1 {
			content = `{"interaction_type":"chat","confidence":"high","needs_tool_execution":false,"reason":"casual conversation"}`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": content},
			}},
		})
	}))
	defer server.Close()

	auditor := &testAuditor{}
	adapter := &countingAdapter{}
	config := DefaultRuntimeConfig()
	config.LLMEnabled = true
	config.LLMProvider = "openai_compatible"
	config.LLMEndpoint = server.URL
	config.LLMModel = "test-model"
	config.LLMAPIKey = "test-api-key"
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), config)
	runtime.toolAdapter = adapter
	runtime.graphRuntime = NewGraphRuntime(runtime.chatModel, adapter)

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{
		Task: "那你能做什么？",
		Messages: []agent.ConversationMessage{
			{Role: "user", Content: "你叫什么？"},
			{Role: "assistant", Content: "我叫麒盾。"},
			{Role: "user", Content: "那你能做什么？"},
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if response.InteractionType != agent.InteractionTypeChat {
		t.Fatalf("expected chat response, got %q", response.InteractionType)
	}
	if response.FinalAnswer != "我是测试模型返回的正常聊天回答。" {
		t.Fatalf("expected remote chat answer, got %q", response.FinalAnswer)
	}
	if callCount != 2 {
		t.Fatalf("expected router and chat completion calls, got %d", callCount)
	}
	if !sawHistory {
		t.Fatal("expected remote chat completion to receive conversation history")
	}
	if !sawModelMetadata {
		t.Fatal("expected remote chat completion to receive configured model metadata")
	}
	if adapter.calls != 0 || len(response.ToolTrace) != 0 || len(response.AgentSteps) != 0 {
		t.Fatalf("chat must not execute tools, calls=%d traces=%d steps=%d", adapter.calls, len(response.ToolTrace), len(response.AgentSteps))
	}
	if auditor.called || response.SecurityReport != nil {
		t.Fatal("chat must not call audit or produce a security report")
	}
}

func TestRuntimeNonOperationalInputDefaultsToChatWithoutGraphOrAudit(t *testing.T) {
	auditor := &testAuditor{}
	adapter := &countingAdapter{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())
	runtime.toolAdapter = adapter
	runtime.graphRuntime = NewGraphRuntime(runtime.chatModel, adapter)

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "你帮我看看"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

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
	if adapter.calls != 0 {
		t.Fatalf("tool adapter should not be called for ambiguous input, got %d calls", adapter.calls)
	}
	if auditor.called {
		t.Fatal("audit client should not be called for ambiguous input")
	}
}

func TestRuntimeModelQuestionReportsActualDeterministicModel(t *testing.T) {
	auditor := &testAuditor{}
	adapter := &countingAdapter{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())
	runtime.toolAdapter = adapter
	runtime.graphRuntime = NewGraphRuntime(runtime.chatModel, adapter)

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "你是什么模型"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if response.InteractionType != agent.InteractionTypeChat {
		t.Fatalf("expected chat response, got %q", response.InteractionType)
	}
	if !strings.Contains(response.FinalAnswer, DefaultChatModel) {
		t.Fatalf("expected actual model %q in answer, got %q", DefaultChatModel, response.FinalAnswer)
	}
	if adapter.calls != 0 || len(response.ToolTrace) != 0 || len(response.AgentSteps) != 0 {
		t.Fatalf("model question must not execute tools, calls=%d traces=%d steps=%d", adapter.calls, len(response.ToolTrace), len(response.AgentSteps))
	}
	if auditor.called || response.SecurityReport != nil {
		t.Fatal("model question must not call audit or produce a security report")
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
	if response.FinalAnswer == "" || response.UserMessage == nil || response.UserMessage.Status != agent.RunStatusBlocked {
		t.Fatalf("expected blocked user-facing answer, got final=%q message=%#v", response.FinalAnswer, response.UserMessage)
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

func runtimeGraphHasNode(graph *auditclient.RiskGraph, key string, value any) bool {
	for _, node := range graph.Nodes {
		if node[key] == value {
			return true
		}
	}
	return false
}

func runtimeGraphHasEdgeType(graph *auditclient.RiskGraph, edgeType string) bool {
	for _, edge := range graph.Edges {
		if edge["type"] == edgeType {
			return true
		}
	}
	return false
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
