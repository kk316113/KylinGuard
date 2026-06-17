package eino

import (
	"context"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/tools"
)

// ChatModelAdapter defines the interface for chat model backends.
// Implementations: DeterministicChatModelStub (rule-based), RemoteLLMMockAdapter (placeholder).
type ChatModelAdapter interface {
	// GenerateToolCalls produces tool calls given a task and available tool definitions.
	// toolDefs may be nil when the adapter supplies its own tool selection logic.
	GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error)

	// Name returns the adapter identifier (e.g., "deterministic-stub", "remote-llm-mock-openai").
	Name() string

	// Provider returns the LLM provider type (e.g., "deterministic", "openai", "anthropic").
	Provider() string
}

// ChatModelAdapterConfig holds configuration for adapter selection and remote LLM connection.
type ChatModelAdapterConfig struct {
	Provider    string // "deterministic", "openai", "anthropic"
	Endpoint    string // LLM API endpoint URL
	Model       string // Model name (e.g., "gpt-4", "claude-3")
	APIKey      string // API key (placeholder, not used in mock)
	Timeout     int    // Request timeout in seconds
	AdapterName string // Custom adapter name override
}
