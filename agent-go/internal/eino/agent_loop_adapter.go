package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kylin-guard-agent/agent-go/internal/agentloop"
)

// RemoteLLMAgentAdapter wraps RemoteLLMAdapter to implement agentloop.NextActionGenerator.
type RemoteLLMAgentAdapter struct {
	llm *RemoteLLMAdapter
}

func NewRemoteLLMAgentAdapter(llm *RemoteLLMAdapter) *RemoteLLMAgentAdapter {
	return &RemoteLLMAgentAdapter{llm: llm}
}

func (a *RemoteLLMAgentAdapter) GenerateNextAction(ctx context.Context, req agentloop.NextActionRequest) (*agentloop.NextAction, error) {
	prompt := a.buildAgentPrompt(req)
	raw, err := a.callLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return a.parseNextAction(raw)
}

func (a *RemoteLLMAgentAdapter) callLLM(ctx context.Context, prompt string) (string, error) {
	// Build messages with chat history.
	messages := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": prompt}, // The prompt already includes the task.
	}
	return a.llm.callChatCompletions(ctx, messages)
}

func (a *RemoteLLMAgentAdapter) buildAgentPrompt(req agentloop.NextActionRequest) string {
	// Build tool list.
	toolLines := make([]string, len(req.AvailableTools))
	for i, t := range req.AvailableTools {
		toolLines[i] = fmt.Sprintf("  - %s: %s (args: %v, boundary: %s)",
			t.ToolName, t.Description, t.ArgKeys, t.BoundaryLevel)
	}

	// Build step history.
	historyLines := make([]string, len(req.StepHistory))
	for i, s := range req.StepHistory {
		obsJSON, _ := json.Marshal(s.Observation)
		historyLines[i] = fmt.Sprintf("  Step %d: %s(%v) → policy=%s obs=%s",
			s.StepIndex, s.ToolName, s.ToolArgs, s.PolicyDecision, string(obsJSON))
	}

	return fmt.Sprintf(`You are a KylinGuard security operations agent. You must output ONLY valid JSON.

Available tools:
%s

Step history:
%s

Task: %s

Respond with EXACTLY one of:
{"action_type":"tool_call","tool_name":"...","tool_args":{...},"reason":"...","user_visible_summary":"..."}
{"action_type":"final_answer","final_answer":"...","confidence":"low|medium|high","next_suggestions":[...]}

Rules:
- Choose tools only from the available list above.
- Do not invent tool names.
- Do not output shell commands.
- Once you have enough evidence, output final_answer.
- final_answer must be in Chinese, explaining what was checked, findings, possible causes, and next steps.`,
		strings.Join(toolLines, "\n"),
		strings.Join(historyLines, "\n"),
		req.Task)
}

func (a *RemoteLLMAgentAdapter) parseNextAction(raw string) (*agentloop.NextAction, error) {
	// Clean the response.
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var action agentloop.NextAction
	if err := json.Unmarshal([]byte(cleaned), &action); err != nil {
		return nil, fmt.Errorf("failed to parse next_action JSON: %w", err)
	}

	// Validate.
	switch action.ActionType {
	case "tool_call":
		if action.ToolName == "" {
			return nil, fmt.Errorf("tool_call with empty tool_name")
		}
	case "final_answer":
		if action.FinalAnswer == "" {
			return nil, fmt.Errorf("final_answer with empty answer text")
		}
	default:
		return nil, fmt.Errorf("unknown action_type: %s", action.ActionType)
	}

	if action.ActionType == "tool_call" && action.ToolArgs == nil {
		action.ToolArgs = map[string]any{}
	}
	return &action, nil
}

// DeterministicAgentAdapter implements agentloop.NextActionGenerator using the deterministic stub.
type DeterministicAgentAdapter struct {
	stub *DeterministicChatModelStub
}

func NewDeterministicAgentAdapter(stub *DeterministicChatModelStub) *DeterministicAgentAdapter {
	return &DeterministicAgentAdapter{stub: stub}
}

func (a *DeterministicAgentAdapter) GenerateNextAction(ctx context.Context, req agentloop.NextActionRequest) (*agentloop.NextAction, error) {
	if len(req.StepHistory) > 0 {
		// After first step, just finalize.
		return &agentloop.NextAction{
			ActionType:  "final_answer",
			FinalAnswer: "已完成检查。详细结果请查看安全报告。",
			Confidence:  "high",
		}, nil
	}
	// First step: generate tool calls.
	calls, _, err := a.stub.GenerateToolCalls(ctx, req.Task, nil)
	if err != nil {
		return nil, err
	}
	if len(calls) == 0 {
		return &agentloop.NextAction{
			ActionType:  "final_answer",
			FinalAnswer: "已完成检查。",
			Confidence:  "high",
		}, nil
	}
	// Return the first tool call.
	call := calls[0]
	return &agentloop.NextAction{
		ActionType:  "tool_call",
		ToolName:    call.ToolName,
		ToolArgs:    call.Input,
		Reason:      call.Reason,
		UserVisibleSummary: fmt.Sprintf("正在执行: %s", call.ToolName),
	}, nil
}
