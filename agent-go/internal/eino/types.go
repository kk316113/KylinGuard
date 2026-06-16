package eino

import "kylin-guard-agent/agent-go/internal/tools"

const (
	RuntimeVersion        = "stage9a-v1"
	RuntimeSummary        = "Eino runtime executed deterministic planner-backed tool orchestration."
	DefaultRuntimeName    = "eino"
	DefaultRoute          = "eino-runtime"
	DefaultOrchestration  = "deterministic-planner-backed"
	defaultRuntimeEnabled = true
	defaultLLMEnabled     = false
)

type RuntimeConfig struct {
	RuntimeEnabled bool
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
	Orchestration string   `json:"orchestration"`
	ToolProtocol  string   `json:"tool_protocol"`
	Version       string   `json:"version"`
	ToolsUsed     []string `json:"tools_used,omitempty"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		RuntimeEnabled: defaultRuntimeEnabled,
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
		config.RuntimeEnabled = defaults.RuntimeEnabled
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
		Orchestration: DefaultOrchestration,
		ToolProtocol:  normalized.ToolProtocol,
		Version:       normalized.Version,
		ToolsUsed:     toolsUsed,
	}
}
