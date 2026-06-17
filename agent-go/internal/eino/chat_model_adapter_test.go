package eino

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kylin-guard-agent/agent-go/internal/tools"
)

func TestDeterministicChatModelStubImplementsChatModelAdapter(t *testing.T) {
	stub := NewDeterministicChatModelStub(tools.NewDefaultRegistry())

	var adapter ChatModelAdapter = stub

	if adapter.Name() != "deterministic-stub" {
		t.Fatalf("expected name 'deterministic-stub', got %q", adapter.Name())
	}
	if adapter.Provider() != "deterministic" {
		t.Fatalf("expected provider 'deterministic', got %q", adapter.Provider())
	}
}

func TestDeterministicChatModelStubGenerateToolCallsWithNilToolDefs(t *testing.T) {
	stub := NewDeterministicChatModelStub(tools.NewDefaultRegistry())
	ctx := context.Background()

	calls, plan, err := stub.GenerateToolCalls(ctx, "检查当前系统 SSH 登录异常", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) == 0 {
		t.Fatal("expected tool calls")
	}
	if plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh_anomaly_check, got %q", plan.Scenario)
	}
}

func TestRemoteLLMAdapterRequiresAPIKey(t *testing.T) {
	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: "https://api.openai.com/v1",
		Model:    "gpt-4",
		APIKey:   "",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test task", nil)

	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
	if !strings.Contains(err.Error(), "API_KEY") && !strings.Contains(err.Error(), "config invalid") {
		t.Fatalf("expected error about missing API key, got: %v", err)
	}
}

func TestRemoteLLMAdapterParsesValidToolPlan(t *testing.T) {
	// Create a mock HTTP server that returns a valid tool plan.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request has auth header.
		if r.Header.Get("Authorization") != "Bearer test-key-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Return a valid tool plan response.
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{
							"scenario": "system_resource_check",
							"intent": "check system resources",
							"tool_plan": [
								{"tool_name": "os_info", "reason": "collect OS context", "arguments": {}},
								{"tool_name": "resource_usage_checker", "reason": "check load and memory", "arguments": {}},
								{"tool_name": "disk_memory_checker", "reason": "check disk and memory", "arguments": {"include_tmpfs": false}}
							],
							"risk_hint": "low",
							"requires_review": false,
							"user_explanation": "Checking system resources"
						}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key-123",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	calls, plan, err := adapter.GenerateToolCalls(ctx, "check system resource usage", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(calls))
	}
	if plan.Scenario != "system_resource_check" {
		t.Fatalf("expected system_resource_check, got %q", plan.Scenario)
	}
	if calls[0].ToolName != "os_info" {
		t.Fatalf("expected first tool os_info, got %q", calls[0].ToolName)
	}
}

func TestRemoteLLMAdapterRejectsNonJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "this is not json"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test task", nil)
	if err == nil {
		t.Fatal("expected error for non-JSON output")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("expected 'not valid JSON' error, got: %v", err)
	}
}

func TestRemoteLLMAdapterRejectsUnknownTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{
							"scenario": "test",
							"intent": "test",
							"tool_plan": [
								{"tool_name": "unknown_tool_xyz", "reason": "test", "arguments": {}}
							],
							"risk_hint": "low",
							"requires_review": false
						}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test task", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "all tools rejected") {
		t.Fatalf("expected 'all tools rejected', got: %v", err)
	}
}

func TestRemoteLLMAdapterRejectsDangerousTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{
							"scenario": "test",
							"intent": "delete everything",
							"tool_plan": [
								{"tool_name": "rm", "reason": "delete files", "arguments": {}}
							],
							"risk_hint": "high",
							"requires_review": true
						}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "rm -rf /", nil)
	if err == nil {
		t.Fatal("expected error for dangerous tool rm")
	}
}

func TestRemoteLLMAdapterRejectsSafeShell(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{
							"scenario": "test",
							"intent": "run command",
							"tool_plan": [
								{"tool_name": "safe_shell", "reason": "execute command", "arguments": {"command": "whoami"}}
							],
							"risk_hint": "medium",
							"requires_review": true
						}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "run whoami", nil)
	if err == nil {
		t.Fatal("expected error for safe_shell in tool plan")
	}
}

func TestRemoteLLMAdapterRejectsEmptyToolPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"scenario": "test", "tool_plan": []}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error for empty tool plan")
	}
}

func TestRemoteLLMAdapterName(t *testing.T) {
	config := ChatModelAdapterConfig{
		Provider:    "anthropic",
		AdapterName: "custom-adapter",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	if adapter.Name() != "custom-adapter" {
		t.Fatalf("expected 'custom-adapter', got %q", adapter.Name())
	}

	config2 := ChatModelAdapterConfig{Provider: "openai"}
	adapter2 := NewRemoteLLMAdapter(config2, tools.NewDefaultRegistry())

	if adapter2.Name() != "remote-llm-openai" {
		t.Fatalf("expected 'remote-llm-openai', got %q", adapter2.Name())
	}
}

func TestRemoteLLMAdapterProvider(t *testing.T) {
	config := ChatModelAdapterConfig{Provider: "anthropic"}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	if adapter.Provider() != "anthropic" {
		t.Fatalf("expected 'anthropic', got %q", adapter.Provider())
	}
}

func TestGraphRuntimeAcceptsChatModelAdapter(t *testing.T) {
	stub := NewDeterministicChatModelStub(tools.NewDefaultRegistry())
	adapter := &recordingGraphAdapter{}

	runtime := NewGraphRuntime(stub, adapter)

	output, err := runtime.Run(context.Background(), "检查 sshd 服务状态")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.ToolCalls) == 0 {
		t.Fatal("expected tool calls")
	}
}

func TestRuntimeMetadataIncludesChatModelAdapter(t *testing.T) {
	config := DefaultRuntimeConfig()
	metadata := config.Metadata(nil)

	if metadata.ChatModelAdapter != DefaultChatModelAdapter {
		t.Fatalf("expected chat_model_adapter=%s, got %q", DefaultChatModelAdapter, metadata.ChatModelAdapter)
	}
	if metadata.ChatModel != DefaultChatModel {
		t.Fatalf("expected chat_model=%s, got %q", DefaultChatModel, metadata.ChatModel)
	}
	if metadata.Version != RuntimeVersion {
		t.Fatalf("expected version=%s, got %q", RuntimeVersion, metadata.Version)
	}
}

func TestFallbackChatModelAdapter(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	primary := NewDeterministicChatModelStub(registry)
	wrapper := NewFallbackChatModelAdapter(primary, registry)

	ctx := context.Background()
	calls, plan, err := wrapper.GenerateToolCalls(ctx, "检查当前系统 SSH 登录异常", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) == 0 {
		t.Fatal("expected tool calls from fallback wrapper")
	}
	if plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh_anomaly_check, got %q", plan.Scenario)
	}

	used, reason := wrapper.FallbackInfo()
	if used {
		t.Fatalf("expected no fallback for working primary, reason: %s", reason)
	}
}

func TestToolPlanValidationRejectsEmpty(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	result := ValidateToolPlan(nil, registry)
	if result.Valid {
		t.Fatal("expected nil plan to be invalid")
	}

	result2 := ValidateToolPlan(&ToolPlan{Scenario: "", ToolPlan: []ToolPlanItem{}}, registry)
	if result2.Valid {
		t.Fatal("expected empty scenario to be invalid")
	}

	result3 := ValidateToolPlan(&ToolPlan{Scenario: "test", ToolPlan: []ToolPlanItem{}}, registry)
	if result3.Valid {
		t.Fatal("expected empty tool_plan to be invalid")
	}
}

func TestBuildToolDefsForPrompt(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	defs := BuildToolDefsForPrompt(registry)

	if len(defs) == 0 {
		t.Fatal("expected non-empty tool defs")
	}

	// Check that sensitive fields are not present.
	for _, def := range defs {
		if _, ok := def["api_key"]; ok {
			t.Fatal("api_key should not be in tool defs")
		}
		if _, ok := def["command"]; ok {
			t.Fatal("command should not be in tool defs")
		}
	}

	// Should contain os_info.
	found := false
	for _, def := range defs {
		if def["tool_name"] == "os_info" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected os_info in tool defs")
	}
}

// --- Stage 13B: Endpoint normalization ---

func TestResolveEndpointFullURL(t *testing.T) {
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Endpoint: "https://api.deepseek.com/v1/chat/completions",
		APIKey:   "test",
	}, tools.NewDefaultRegistry())

	ep := adapter.resolveEndpoint()
	if ep != "https://api.deepseek.com/v1/chat/completions" {
		t.Fatalf("expected full URL unchanged, got %q", ep)
	}
}

func TestResolveEndpointBaseURL(t *testing.T) {
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Endpoint: "https://api.deepseek.com",
		APIKey:   "test",
	}, tools.NewDefaultRegistry())

	ep := adapter.resolveEndpoint()
	expected := "https://api.deepseek.com" + chatCompletionsPath
	if ep != expected {
		t.Fatalf("expected %q, got %q", expected, ep)
	}
}

func TestResolveEndpointTrailingSlash(t *testing.T) {
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Endpoint: "https://api.deepseek.com/",
		APIKey:   "test",
	}, tools.NewDefaultRegistry())

	ep := adapter.resolveEndpoint()
	expected := "https://api.deepseek.com" + chatCompletionsPath
	if ep != expected {
		t.Fatalf("expected %q, got %q", expected, ep)
	}
}

func TestResolveEndpointV1Base(t *testing.T) {
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Endpoint: "https://api.openai.com/v1",
		APIKey:   "test",
	}, tools.NewDefaultRegistry())

	ep := adapter.resolveEndpoint()
	if ep != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions, got %q", ep)
	}
}

func TestResolveEndpointEmpty(t *testing.T) {
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Endpoint: "",
		APIKey:   "test",
	}, tools.NewDefaultRegistry())

	ep := adapter.resolveEndpoint()
	if ep != "" {
		t.Fatalf("expected empty endpoint to resolve to empty, got %q", ep)
	}
}

// --- Stage 13B: ValidateLLMConfig ---

func TestValidateLLMConfigDeterministic(t *testing.T) {
	if r := ValidateLLMConfig("deterministic", "", ""); r == "" {
		t.Fatal("expected validation to fail for deterministic provider without remote config")
	}
}

func TestValidateLLMConfigMissingAPIKey(t *testing.T) {
	r := ValidateLLMConfig("openai_compatible", "https://api.openai.com/v1", "")
	if r == "" {
		t.Fatal("expected validation to fail when API key is missing")
	}
}

func TestValidateLLMConfigMissingEndpoint(t *testing.T) {
	if r := ValidateLLMConfig("deepseek", "", "sk-test"); r == "" {
		t.Fatal("expected validation to fail for deepseek without endpoint")
	}
	if r := ValidateLLMConfig("openai_compatible", "", "sk-test"); r == "" {
		t.Fatal("expected validation to fail for openai_compatible without endpoint")
	}
}

func TestValidateLLMConfigValid(t *testing.T) {
	if r := ValidateLLMConfig("openai_compatible", "https://api.openai.com/v1", "sk-test"); r != "" {
		t.Fatalf("expected valid config, got: %s", r)
	}
}

// --- Stage 13B: GenerateToolCalls fails gracefully with missing config ---

func TestRemoteLLMAdapterFailsGracefullyOnMissingConfig(t *testing.T) {
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Provider: "openai_compatible",
		Endpoint: "",
		APIKey:   "",
	}, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
	if !strings.Contains(err.Error(), "config invalid") {
		t.Fatalf("expected config invalid error, got: %v", err)
	}
}

// --- Stage 13B: Mock server integration tests ---

func TestRemoteLLMAdapterRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai_compatible",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
}

func TestRemoteLLMAdapterFallsBackOnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai_compatible",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())
	fallback := NewFallbackChatModelAdapter(adapter, tools.NewDefaultRegistry())

	ctx := context.Background()
	calls, plan, err := fallback.GenerateToolCalls(ctx, "check system resource usage", nil)
	if err != nil {
		t.Fatalf("fallback adapter should not propagate error: %v", err)
	}
	if len(calls) == 0 {
		t.Fatal("expected fallback adapter to produce tool calls")
	}
	if plan.Scenario == "" {
		t.Fatal("expected fallback adapter to produce plan")
	}

	used, reason := fallback.FallbackInfo()
	if !used {
		t.Fatal("expected fallback to be used")
	}
	if reason == "" {
		t.Fatal("expected fallback reason to be non-empty")
	}
	if strings.Contains(reason, "test-key") || strings.Contains(reason, "Bearer") {
		t.Fatal("fallback reason should not contain API key")
	}
}

func TestRemoteLLMAdapterEndpointRequirement(t *testing.T) {
	// Without endpoint but with key, should get clear error for openai_compatible.
	adapter := NewRemoteLLMAdapter(ChatModelAdapterConfig{
		Provider: "openai_compatible",
		APIKey:   "sk-test",
	}, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error when endpoint is empty")
	}
	if !strings.Contains(err.Error(), "ENDPOINT") && !strings.Contains(err.Error(), "config invalid") {
		t.Fatalf("expected error mentioning endpoint or config, got: %v", err)
	}
}

func TestValidateLLMConfigCustomProvider(t *testing.T) {
	if r := ValidateLLMConfig("custom", "https://custom.example.com/v1", "sk-test"); r != "" {
		t.Fatalf("expected 'custom' provider with endpoint and key to be valid, got: %s", r)
	}
}

func TestRemoteLLMAdapterDeepSeekMissingEndpoint(t *testing.T) {
	r := ValidateLLMConfig("deepseek", "", "sk-test")
	if r == "" {
		t.Fatal("expected deepseek without endpoint to be invalid")
	}
	if !strings.Contains(r, "ENDPOINT") {
		t.Fatalf("expected error about missing endpoint, got: %s", r)
	}
}

func TestRemoteLLMAdapterRejectsRequestError(t *testing.T) {
	// Use an unreachable endpoint to simulate network error.
	config := ChatModelAdapterConfig{
		Provider: "openai_compatible",
		Endpoint: "http://127.0.0.1:1/v1/chat/completions",
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}

func TestRemoteLLMAdapterSSHTaskReturnsSSHPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode request to verify task-aware routing.
		var reqBody struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&reqBody)
		userMsg := ""
		for _, m := range reqBody.Messages {
			if m.Content != "" {
				userMsg = m.Content
			}
		}

		var plan map[string]any
		if containsAny(userMsg, []string{"ssh", "login", "auth"}) {
			plan = map[string]any{
				"scenario": "ssh_anomaly_check",
				"intent":   "check SSH login anomaly",
				"tool_plan": []map[string]any{
					{"tool_name": "os_info", "reason": "collect OS context", "arguments": map[string]any{}},
					{"tool_name": "service_status", "reason": "check sshd", "arguments": map[string]any{"service_name": "sshd"}},
					{"tool_name": "ssh_login_analyzer", "reason": "analyze", "arguments": map[string]any{"lines": 200}},
				},
				"risk_hint":       "medium",
				"requires_review": true,
			}
		} else {
			plan = map[string]any{
				"scenario": "system_resource_check",
				"intent":   "check resources",
				"tool_plan": []map[string]any{
					{"tool_name": "os_info", "reason": "collect", "arguments": map[string]any{}},
					{"tool_name": "resource_usage_checker", "reason": "check", "arguments": map[string]any{}},
				},
				"risk_hint":       "low",
				"requires_review": false,
			}
		}
		content, _ := json.Marshal(plan)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": string(content)}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai_compatible",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()

	// SSH task should get ssh_anomaly_check.
	calls, plan, err := adapter.GenerateToolCalls(ctx, "check SSH login anomaly", nil)
	if err != nil {
		t.Fatalf("SSH task error: %v", err)
	}
	if plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("SSH task: expected ssh_anomaly_check, got %q", plan.Scenario)
	}
	if len(calls) == 0 {
		t.Fatal("SSH task: expected tool calls")
	}
}

func TestRemoteLLMAdapterSystemResourceTaskReturnsResourcePlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&reqBody)
		userMsg := ""
		for _, m := range reqBody.Messages {
			if m.Content != "" {
				userMsg = m.Content
			}
		}

		var plan map[string]any
		if containsAny(userMsg, []string{"resource", "cpu", "memory", "disk", "load"}) {
			plan = map[string]any{
				"scenario": "system_resource_check",
				"intent":   "check resources",
				"tool_plan": []map[string]any{
					{"tool_name": "os_info", "reason": "collect", "arguments": map[string]any{}},
					{"tool_name": "resource_usage_checker", "reason": "check", "arguments": map[string]any{}},
					{"tool_name": "disk_memory_checker", "reason": "check disk", "arguments": map[string]any{"include_tmpfs": false}},
				},
				"risk_hint":       "low",
				"requires_review": false,
			}
		} else {
			plan = map[string]any{
				"scenario": "system_security_overview",
				"intent":   "security check",
				"tool_plan": []map[string]any{
					{"tool_name": "os_info", "reason": "collect", "arguments": map[string]any{}},
				},
				"risk_hint":       "medium",
				"requires_review": true,
			}
		}
		content, _ := json.Marshal(plan)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": string(content)}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ChatModelAdapterConfig{
		Provider: "openai_compatible",
		Endpoint: server.URL,
		Model:    "gpt-4",
		APIKey:   "test-key",
	}
	adapter := NewRemoteLLMAdapter(config, tools.NewDefaultRegistry())

	ctx := context.Background()

	calls, plan, err := adapter.GenerateToolCalls(ctx, "check system resource usage", nil)
	if err != nil {
		t.Fatalf("resource task error: %v", err)
	}
	if plan.Scenario != "system_resource_check" {
		t.Fatalf("resource task: expected system_resource_check, got %q", plan.Scenario)
	}
	if len(calls) < 2 {
		t.Fatalf("resource task: expected >=2 tool calls, got %d", len(calls))
	}
}

func containsAny(s string, items []string) bool {
	for _, item := range items {
		if strings.Contains(s, item) {
			return true
		}
	}
	return false
}
