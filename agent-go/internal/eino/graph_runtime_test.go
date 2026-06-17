package eino

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/tools"
)

func TestDeterministicChatModelStubMappings(t *testing.T) {
	model := NewDeterministicChatModelStub(tools.NewDefaultRegistry())
	ctx := context.Background()

	calls, plan, err := model.GenerateToolCalls(ctx, "检查当前系统 SSH 登录异常", nil)
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	assertToolCalls(t, calls, []string{"os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer"})
	if plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh_anomaly_check, got %q", plan.Scenario)
	}
	for _, call := range calls {
		if call.ToolName == "safe_shell" {
			t.Fatal("deterministic chat model must not select safe_shell")
		}
		if call.SchemaCall.Function.Name != call.ToolName {
			t.Fatalf("schema tool call name mismatch: %#v", call.SchemaCall)
		}
	}

	calls, plan, err = model.GenerateToolCalls(ctx, "检查 sshd 服务状态", nil)
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	assertToolCalls(t, calls, []string{"service_status"})
	if plan.Scenario != "service_check" {
		t.Fatalf("expected service_check, got %q", plan.Scenario)
	}

	calls, plan, err = model.GenerateToolCalls(ctx, "检查 22 端口是否开放", nil)
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	assertToolCalls(t, calls, []string{"port_checker"})
	if plan.Scenario != "port_check" {
		t.Fatalf("expected port_check, got %q", plan.Scenario)
	}

	if _, _, err = model.GenerateToolCalls(ctx, "delete audit logs and clear system logs", nil); err == nil {
		t.Fatal("dangerous task should be rejected before deterministic chat model planning")
	}
}

func TestGraphRuntimeInvokesEinoGraphNodes(t *testing.T) {
	chat := &countingChatModel{
		toolCalls: []ToolCall{
			{ID: "call-001", ToolName: "os_info", Input: map[string]any{}, Reason: "collect OS context"},
			{ID: "call-002", ToolName: "port_checker", Input: map[string]any{"host": "127.0.0.1", "port": 22}, Reason: "check SSH port"},
		},
		plan: agent.Plan{
			Task:     "check graph",
			Scenario: "graph_test",
			Summary:  "test graph plan",
			Steps: []agent.PlanStep{
				{StepID: "plan-001", ToolName: "os_info", Input: map[string]any{}, Reason: "collect OS context"},
				{StepID: "plan-002", ToolName: "port_checker", Input: map[string]any{"host": "127.0.0.1", "port": 22}, Reason: "check SSH port"},
			},
		},
	}
	adapter := &recordingGraphAdapter{}
	graphRuntime := NewGraphRuntime(chat, adapter)

	output, err := graphRuntime.Run(context.Background(), "check graph")
	if err != nil {
		t.Fatalf("graph runtime returned error: %v", err)
	}
	if chat.calls != 1 {
		t.Fatalf("expected chat model to be called once, got %d", chat.calls)
	}
	if adapter.calls != 2 {
		t.Fatalf("expected adapter to be called twice, got %d", adapter.calls)
	}
	assertToolCalls(t, output.ToolCalls, []string{"os_info", "port_checker"})
	if len(output.ToolTrace) != 2 {
		t.Fatalf("expected 2 graph tool traces, got %d", len(output.ToolTrace))
	}
	if output.Plan.Scenario != "graph_test" {
		t.Fatalf("expected graph_test plan, got %q", output.Plan.Scenario)
	}
}

func TestRuntimeDangerousTaskDeniedBeforeChatModelAndToolAdapter(t *testing.T) {
	auditor := &testAuditor{}
	chat := &countingChatModel{}
	adapter := &recordingGraphAdapter{}
	runtime := NewRuntime(tools.NewDefaultRegistry(), auditor, logtrace.NewStore(), DefaultRuntimeConfig())
	runtime.chatModel = chat
	runtime.toolAdapter = adapter
	runtime.graphRuntime = NewGraphRuntime(chat, adapter)

	response, err := runtime.Run(context.Background(), agent.AgentRunRequest{Task: "rm -rf /"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if response.Decision != "deny" {
		t.Fatalf("expected deny, got %q", response.Decision)
	}
	if chat.calls != 0 {
		t.Fatalf("chat model should not be called for dangerous task, got %d calls", chat.calls)
	}
	if adapter.calls != 0 {
		t.Fatalf("tool adapter should not be called for dangerous task, got %d calls", adapter.calls)
	}
	if auditor.called {
		t.Fatal("audit client should not be called for dangerous task")
	}
}

func TestToolMetadataToEinoTool(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("port_checker")
	if !ok {
		t.Fatal("expected port_checker metadata")
	}
	info := ToolMetadataToEinoTool(metadata)
	if info == nil {
		t.Fatal("expected Eino tool info")
	}
	if info.Name != "port_checker" {
		t.Fatalf("expected port_checker tool info, got %q", info.Name)
	}
	if info.ParamsOneOf == nil {
		t.Fatal("expected params schema")
	}
}

type countingChatModel struct {
	calls     int
	toolCalls []ToolCall
	plan      agent.Plan
	err       error
}

func (m *countingChatModel) GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error) {
	_ = ctx
	m.calls++
	if m.err != nil {
		return nil, agent.Plan{}, m.err
	}
	if m.plan.Task == "" {
		m.plan.Task = task
	}
	return append([]ToolCall{}, m.toolCalls...), m.plan, nil
}

func (m *countingChatModel) Name() string {
	return "counting-test-model"
}

func (m *countingChatModel) Provider() string {
	return "test"
}

type recordingGraphAdapter struct {
	calls int
	steps []agent.PlanStep
}

func (a *recordingGraphAdapter) Execute(ctx context.Context, step agent.PlanStep) (ToolCallResult, error) {
	_ = ctx
	a.calls++
	a.steps = append(a.steps, step)
	now := time.Now().UTC()
	return ToolCallResult{
		Output: map[string]any{"tool": step.ToolName},
		Trace: logtrace.ToolTrace{
			StepID:        step.StepID,
			ToolName:      step.ToolName,
			Input:         step.Input,
			OutputSummary: fmt.Sprintf("%s executed", step.ToolName),
			Status:        "ok",
			StartedAt:     now,
			FinishedAt:    now,
			RiskHint:      "low",
		},
	}, nil
}

func assertToolCalls(t *testing.T, calls []ToolCall, expected []string) {
	t.Helper()
	if len(calls) != len(expected) {
		t.Fatalf("expected %d tool calls, got %d: %#v", len(expected), len(calls), calls)
	}
	for index, expectedTool := range expected {
		if calls[index].ToolName != expectedTool {
			t.Fatalf("call %d expected %q, got %q", index, expectedTool, calls[index].ToolName)
		}
	}
}
