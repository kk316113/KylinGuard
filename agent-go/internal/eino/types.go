package eino

import "kylin-guard-agent/agent-go/internal/tools"

const (
	RuntimeVersion        = "stage9b-v1"
	RuntimeSummary        = "Eino graph runtime executed deterministic tool-calling orchestration."
	DefaultRuntimeName    = "eino"
	DefaultRoute          = "eino-runtime"
	DefaultOrchestration  = "eino-graph-tool-calling"
	DefaultChatModel      = "deterministic-stub"
	defaultRuntimeEnabled = true
	defaultGraphEnabled   = true
	defaultLLMEnabled     = false
)

type RuntimeConfig struct {
	RuntimeEnabled bool
	GraphEnabled   bool
	LLMEnabled     bool
	RuntimeName    string
	Route          string
	ToolProtocol   string
	Version        string
}

type RuntimeMetadata struct {
	Route         string   `json:"route"`
	Runtime       string   `json:"runtime"`
	LLMEnabled    bool     `json:"llm_enabled"`
	EinoGraph     bool     `json:"eino_graph_enabled"`
	ChatModel     string   `json:"chat_model"`
	Orchestration string   `json:"orchestration"`
	ToolProtocol  string   `json:"tool_protocol"`
	Version       string   `json:"version"`
	ToolsUsed     []string `json:"tools_used,omitempty"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		RuntimeEnabled: defaultRuntimeEnabled,
		GraphEnabled:   defaultGraphEnabled,
		LLMEnabled:     defaultLLMEnabled,
		RuntimeName:    DefaultRuntimeName,
		Route:          DefaultRoute,
		ToolProtocol:   tools.ToolProtocol,
		Version:        RuntimeVersion,
	}
}

func NormalizeRuntimeConfig(config RuntimeConfig) RuntimeConfig {
	defaults := DefaultRuntimeConfig()
	if config == (RuntimeConfig{}) {
		return defaults
	}
	if config.RuntimeName == "" {
		config.RuntimeName = defaults.RuntimeName
	}
	if config.Route == "" {
		config.Route = defaults.Route
	}
	if config.ToolProtocol == "" {
		config.ToolProtocol = defaults.ToolProtocol
	}
	if config.Version == "" {
		config.Version = defaults.Version
	}
	return config
}

func (c RuntimeConfig) Metadata(toolsUsed []string) RuntimeMetadata {
	normalized := NormalizeRuntimeConfig(c)
	return RuntimeMetadata{
		Route:         normalized.Route,
		Runtime:       normalized.RuntimeName,
		LLMEnabled:    normalized.LLMEnabled,
		EinoGraph:     normalized.GraphEnabled,
		ChatModel:     DefaultChatModel,
		Orchestration: DefaultOrchestration,
		ToolProtocol:  normalized.ToolProtocol,
		Version:       normalized.Version,
		ToolsUsed:     toolsUsed,
	}
}
