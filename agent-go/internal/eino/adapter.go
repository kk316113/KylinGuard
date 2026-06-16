package eino

import "kylin-guard-agent/agent-go/internal/agent"

var _ agent.AgentAdapter = (*Runtime)(nil)

func NewRuntimeAdapter(runtime *Runtime) agent.AgentAdapter {
	return runtime
}
