package eino

import (
	"context"
	"fmt"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/tools"
)

// RemoteLLMMockAdapter is a placeholder that demonstrates the ChatModelAdapter interface
// but returns "not implemented" errors when called.
// It is intended for future integration with remote LLM APIs (OpenAI, Anthropic, etc.).
type RemoteLLMMockAdapter struct {
	config ChatModelAdapterConfig
}

func NewRemoteLLMMockAdapter(config ChatModelAdapterConfig) *RemoteLLMMockAdapter {
	return &RemoteLLMMockAdapter{config: config}
}

// GenerateToolCalls returns a not-implemented error.
// Future implementation will call the remote LLM API with tool definitions.
func (m *RemoteLLMMockAdapter) GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error) {
	return nil, agent.Plan{}, fmt.Errorf(
		"remote LLM adapter not implemented: provider=%s, endpoint=%s, model=%s",
		m.config.Provider, m.config.Endpoint, m.config.Model,
	)
}

// Name returns the adapter identifier.
func (m *RemoteLLMMockAdapter) Name() string {
	if m.config.AdapterName != "" {
		return m.config.AdapterName
	}
	return fmt.Sprintf("remote-llm-mock-%s", m.config.Provider)
}

// Provider returns the LLM provider type.
func (m *RemoteLLMMockAdapter) Provider() string {
	return m.config.Provider
}
