package eino

import (
	"context"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

type GraphToolNode struct {
	adapter PlanToolAdapter
}

func NewGraphToolNode(adapter PlanToolAdapter) *GraphToolNode {
	return &GraphToolNode{adapter: adapter}
}

func (n *GraphToolNode) Invoke(ctx context.Context, state GraphState) (GraphOutput, error) {
	output := GraphOutput{
		Task:      state.Task,
		ToolCalls: append([]ToolCall{}, state.ToolCalls...),
		Plan:      state.Plan,
	}
	if n == nil || n.adapter == nil {
		return output, nil
	}

	for _, step := range state.Plan.Steps {
		result, _ := n.adapter.Execute(ctx, step)
		output.ToolResults = append(output.ToolResults, result)
		if result.Trace.ToolName != "" {
			output.ToolTrace = append(output.ToolTrace, result.Trace)
		}
	}
	if output.ToolTrace == nil {
		output.ToolTrace = []logtrace.ToolTrace{}
	}
	return output, nil
}
