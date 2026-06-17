package agentloop

import (
	"context"
	"testing"

	"kylin-guard-agent/agent-go/internal/tools"
)

// mockGenerator implements NextActionGenerator for testing.
type mockGenerator struct {
	actions []NextAction
	index   int
}

func (m *mockGenerator) GenerateNextAction(ctx context.Context, req NextActionRequest) (*NextAction, error) {
	if m.index >= len(m.actions) {
		return &NextAction{ActionType: "final_answer", FinalAnswer: "done"}, nil
	}
	a := m.actions[m.index]
	m.index++
	return &a, nil
}

// mockExecutor implements StepExecutor for testing.
type mockExecutor struct{}

func (m *mockExecutor) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	return map[string]any{"status": "ok", "result": toolName + " executed"}, nil
}

func (m *mockExecutor) CheckToolPolicy(toolName string, args map[string]any) (bool, string) {
	return true, ""
}

func TestEngineSSHAgentLoopMultiStep(t *testing.T) {
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "service_status", ToolArgs: map[string]any{"service_name": "sshd"}, Reason: "Check sshd"},
			{ActionType: "tool_call", ToolName: "port_checker", ToolArgs: map[string]any{"host": "127.0.0.1", "port": 22}, Reason: "Check port 22"},
			{ActionType: "tool_call", ToolName: "journalctl_reader", ToolArgs: map[string]any{"service_name": "sshd", "lines": 50}, Reason: "Read journal"},
			{ActionType: "final_answer", FinalAnswer: "完成排查", Confidence: "medium"},
		},
	}
	exec := &mockExecutor{}
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "我 SSH 连不上了，帮我看看", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AgentMode != ModeAgentLoop {
		t.Fatalf("expected agent_loop mode, got %q", resp.AgentMode)
	}
	if len(resp.AgentSteps) < 3 {
		t.Fatalf("expected >=3 agent steps, got %d", len(resp.AgentSteps))
	}
	if resp.StepCount < 3 {
		t.Fatalf("expected step_count >= 3, got %d", resp.StepCount)
	}
	if resp.FinalAnswer == "" {
		t.Fatal("expected non-empty final_answer")
	}

	// Check each step has observation.
	for i, step := range resp.AgentSteps {
		if step.Observation == nil || len(step.Observation) == 0 {
			t.Fatalf("step %d missing observation", i)
		}
		if step.ActionType != "tool_call" {
			t.Fatalf("step %d expected tool_call, got %q", i, step.ActionType)
		}
	}
}

func TestEngineDangerousTaskDenied(t *testing.T) {
	gen := &mockGenerator{}
	exec := &mockExecutor{}
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "delete audit logs and clear system logs", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) != 0 {
		t.Fatalf("expected 0 steps for dangerous task, got %d", len(resp.AgentSteps))
	}
	if resp.FinalAnswer == "" {
		t.Fatal("expected non-empty final_answer for dangerous task")
	}
}

func TestEngineMaxStepsReached(t *testing.T) {
	// Generator always returns tool_call, never final_answer.
	gen := &mockGenerator{
		actions: []NextAction{
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
			{ActionType: "tool_call", ToolName: "os_info", ToolArgs: map[string]any{}},
		},
	}
	exec := &mockExecutor{}
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())
	engine.MaxSteps = 3

	resp, err := engine.Run(context.Background(), "check system", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.AgentSteps) > 3 {
		t.Fatalf("expected <=3 steps for max_steps=3, got %d", len(resp.AgentSteps))
	}
	if resp.FinalAnswer == "" {
		t.Fatal("expected final_answer when max steps reached")
	}
}

func TestToolStepExecutorPolicy(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	exec := NewToolStepExecutor(registry)

	// Unknown tool should be denied.
	allowed, reason := exec.CheckToolPolicy("nonexistent_tool", nil)
	if allowed {
		t.Fatal("expected nonexistent_tool to be denied by policy")
	}
	if reason == "" {
		t.Fatal("expected non-empty policy reason")
	}
}

func TestEngineEmptyTaskReturnsStepZero(t *testing.T) {
	gen := &mockGenerator{}
	exec := &mockExecutor{}
	engine := NewEngine(gen, exec, tools.NewDefaultRegistry())

	resp, err := engine.Run(context.Background(), "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty task should result in some answer.
	if resp.FinalAnswer == "" {
		t.Fatal("expected non-empty final_answer")
	}
}
