package eino

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

const (
	graphNodeChatModelStub = "deterministic_chat_model_stub"
	graphNodeToolExecutor  = "mcp_like_tool_node"
)

type GraphInput struct {
	Task string
}

type GraphState struct {
	Task      string
	ToolCalls []ToolCall
	Plan      agent.Plan
}

type GraphOutput struct {
	Task        string
	ToolCalls   []ToolCall
	Plan        agent.Plan
	ToolResults []ToolCallResult
	ToolTrace   []logtrace.ToolTrace
}

type GraphRuntime struct {
	chatModel  ToolCallGenerator
	toolNode   *GraphToolNode
	runnable   compose.Runnable[GraphInput, GraphOutput]
	compileErr error
}

func NewGraphRuntime(chatModel ToolCallGenerator, toolAdapter PlanToolAdapter) *GraphRuntime {
	runtime := &GraphRuntime{
		chatModel: chatModel,
		toolNode:  NewGraphToolNode(toolAdapter),
	}
	runtime.runnable, runtime.compileErr = runtime.compile(context.Background())
	return runtime
}

func (r *GraphRuntime) Run(ctx context.Context, task string) (GraphOutput, error) {
	if r == nil {
		return GraphOutput{}, fmt.Errorf("eino graph runtime is not initialized")
	}
	if r.compileErr != nil {
		return GraphOutput{}, r.compileErr
	}
	if r.runnable == nil {
		return GraphOutput{}, fmt.Errorf("eino graph runtime is not compiled")
	}
	return r.runnable.Invoke(ctx, GraphInput{Task: task})
}

func (r *GraphRuntime) compile(ctx context.Context) (compose.Runnable[GraphInput, GraphOutput], error) {
	if r.chatModel == nil {
		return nil, fmt.Errorf("deterministic chat model stub is not configured")
	}
	if r.toolNode == nil {
		return nil, fmt.Errorf("graph tool node is not configured")
	}

	graph := compose.NewGraph[GraphInput, GraphOutput]()
	if err := graph.AddLambdaNode(graphNodeChatModelStub, compose.InvokableLambda[GraphInput, GraphState](
		func(ctx context.Context, input GraphInput) (GraphState, error) {
			calls, plan, err := r.chatModel.GenerateToolCalls(ctx, input.Task)
			if err != nil {
				return GraphState{}, err
			}
			return GraphState{
				Task:      input.Task,
				ToolCalls: calls,
				Plan:      plan,
			}, nil
		},
	)); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode(graphNodeToolExecutor, compose.InvokableLambda[GraphState, GraphOutput](
		func(ctx context.Context, state GraphState) (GraphOutput, error) {
			return r.toolNode.Invoke(ctx, state)
		},
	)); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(compose.START, graphNodeChatModelStub); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(graphNodeChatModelStub, graphNodeToolExecutor); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(graphNodeToolExecutor, compose.END); err != nil {
		return nil, err
	}
	return graph.Compile(ctx, compose.WithGraphName("kylin_guard_eino_graph_runtime"))
}
