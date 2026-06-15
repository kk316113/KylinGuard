package agent

import "context"

type PlannedStep struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
}

type Planner interface {
	Plan(ctx context.Context, task string) ([]PlannedStep, error)
}

type StaticPlanner struct{}

func NewStaticPlanner() StaticPlanner {
	return StaticPlanner{}
}

func (StaticPlanner) Plan(ctx context.Context, task string) ([]PlannedStep, error) {
	_ = ctx
	// TODO: Replace this static adapter after the current Eino package path and API are confirmed.
	return []PlannedStep{
		{
			ToolName: "os_info",
			Input: map[string]any{
				"task": task,
			},
		},
		{
			ToolName: "port_checker",
			Input: map[string]any{
				"host": "127.0.0.1",
				"port": 8080,
			},
		},
	}, nil
}
