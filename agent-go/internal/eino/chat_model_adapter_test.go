package eino

import (
	"context"
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

func TestRemoteLLMMockAdapterReturnsNotImplemented(t *testing.T) {
	config := ChatModelAdapterConfig{
		Provider: "openai",
		Endpoint: "https://api.openai.com/v1",
		Model:    "gpt-4",
	}
	adapter := NewRemoteLLMMockAdapter(config)

	ctx := context.Background()
	_, _, err := adapter.GenerateToolCalls(ctx, "test task", nil)

	if err == nil {
		t.Fatal("expected error from mock adapter")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected 'not implemented' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Fatalf("expected error to mention provider, got: %v", err)
	}
}

func TestRemoteLLMMockAdapterName(t *testing.T) {
	config := ChatModelAdapterConfig{
		Provider:    "anthropic",
		AdapterName: "custom-adapter",
	}
	adapter := NewRemoteLLMMockAdapter(config)

	if adapter.Name() != "custom-adapter" {
		t.Fatalf("expected 'custom-adapter', got %q", adapter.Name())
	}

	config2 := ChatModelAdapterConfig{Provider: "openai"}
	adapter2 := NewRemoteLLMMockAdapter(config2)

	if adapter2.Name() != "remote-llm-mock-openai" {
		t.Fatalf("expected 'remote-llm-mock-openai', got %q", adapter2.Name())
	}
}

func TestRemoteLLMMockAdapterProvider(t *testing.T) {
	config := ChatModelAdapterConfig{Provider: "anthropic"}
	adapter := NewRemoteLLMMockAdapter(config)

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

func TestRuntimeMetadataWithLLMProvider(t *testing.T) {
	config := DefaultRuntimeConfig()
	config.LLMEnabled = true
	config.LLMProvider = "openai"
	metadata := config.Metadata(nil)

	if metadata.ChatModel != "remote-llm-mock-openai" {
		t.Fatalf("expected chat_model=remote-llm-mock-openai, got %q", metadata.ChatModel)
	}
	if metadata.LLMEnabled != true {
		t.Fatal("expected llm_enabled=true")
	}
}
