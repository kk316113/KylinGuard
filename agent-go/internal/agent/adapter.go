package agent

import (
	"context"
	"errors"
)

type AgentRunRequest = RunRequest
type AgentRunResponse = RunResponse

type AgentAdapter interface {
	Run(ctx context.Context, req AgentRunRequest) (AgentRunResponse, error)
	Name() string
	Enabled() bool
}

type StableRuntimeAdapter struct {
	runtime *Runtime
}

func NewStableRuntimeAdapter(runtime *Runtime) StableRuntimeAdapter {
	return StableRuntimeAdapter{runtime: runtime}
}

func (a StableRuntimeAdapter) Run(ctx context.Context, req AgentRunRequest) (AgentRunResponse, error) {
	if a.runtime == nil {
		return AgentRunResponse{}, errors.New("stable runtime is not configured")
	}
	return a.runtime.Run(ctx, req)
}

func (StableRuntimeAdapter) Name() string {
	return "stable_runtime"
}

func (a StableRuntimeAdapter) Enabled() bool {
	return a.runtime != nil
}
