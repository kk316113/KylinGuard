package eino

import (
	"strings"

	"kylin-guard-agent/agent-go/internal/agentloop"
	"kylin-guard-agent/agent-go/internal/tools"
)

const (
	RuntimeVersion          = "stage13a-v1"
	RuntimeSummary          = "Eino graph runtime executed chat model adapter orchestration."
	DefaultRuntimeName      = "eino"
	DefaultRoute            = "eino-runtime"
	DefaultOrchestration    = "eino-graph-tool-calling"
	DefaultChatModel        = "deterministic-stub"
	DefaultChatModelAdapter = "interface-v1"
	defaultRuntimeEnabled   = true
	defaultGraphEnabled     = true
	defaultLLMEnabled       = false
)

type RuntimeConfig struct {
	RuntimeEnabled bool
	GraphEnabled   bool
	LLMEnabled     bool
	RuntimeName    string
	Route          string
	ToolProtocol   string
	Version        string
	LLMProvider    string // "deterministic", "openai", "anthropic"
	LLMEndpoint    string // LLM API endpoint URL
	LLMModel       string // Model name
	LLMAPIKey      string // API key (placeholder)
	AgentMaxSteps  int    // Maximum Agent Loop tool-call iterations.
}

type RuntimeMetadata struct {
	Route            string   `json:"route"`
	Runtime          string   `json:"runtime"`
	LLMEnabled       bool     `json:"llm_enabled"`
	EinoGraph        bool     `json:"eino_graph_enabled"`
	ChatModel        string   `json:"chat_model"`
	ChatModelAdapter string   `json:"chat_model_adapter"`
	Orchestration    string   `json:"orchestration"`
	ToolProtocol     string   `json:"tool_protocol"`
	Version          string   `json:"version"`
	ToolsUsed        []string `json:"tools_used,omitempty"`
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
		LLMProvider:    "deterministic",
		AgentMaxSteps:  agentloop.DefaultMaxSteps,
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
	if config.LLMProvider == "" {
		config.LLMProvider = defaults.LLMProvider
	}
	if config.AgentMaxSteps == 0 {
		config.AgentMaxSteps = defaults.AgentMaxSteps
	}
	if config.AgentMaxSteps < agentloop.MinMaxSteps {
		config.AgentMaxSteps = agentloop.MinMaxSteps
	}
	if config.AgentMaxSteps > agentloop.MaxMaxSteps {
		config.AgentMaxSteps = agentloop.MaxMaxSteps
	}
	return config
}

// ChatModelName returns the display name for the chat model based on configuration.
func (c RuntimeConfig) ChatModelName() string {
	normalized := NormalizeRuntimeConfig(c)
	if !normalized.LLMEnabled || normalized.LLMProvider == "deterministic" {
		return DefaultChatModel
	}
	// Detect mock mode: DEMO_MOCK_LLM=true sets key to "sk-mock-key".
	if c.LLMAPIKey == "sk-mock-key" {
		return "remote-llm-mock-" + normalized.LLMProvider
	}
	// Detect DeepSeek by endpoint URL or provider name.
	endpoint := strings.ToLower(normalized.LLMEndpoint)
	provider := strings.ToLower(normalized.LLMProvider)
	if strings.Contains(endpoint, "deepseek") || provider == "deepseek" {
		return "remote-llm-deepseek-" + normalized.LLMProvider
	}
	return "remote-llm-" + normalized.LLMProvider
}

func (c RuntimeConfig) Metadata(toolsUsed []string) RuntimeMetadata {
	normalized := NormalizeRuntimeConfig(c)
	chatModelName := c.ChatModelName()
	return RuntimeMetadata{
		Route:            normalized.Route,
		Runtime:          normalized.RuntimeName,
		LLMEnabled:       normalized.LLMEnabled,
		EinoGraph:        normalized.GraphEnabled,
		ChatModel:        chatModelName,
		ChatModelAdapter: DefaultChatModelAdapter,
		Orchestration:    DefaultOrchestration,
		ToolProtocol:     normalized.ToolProtocol,
		Version:          normalized.Version,
		ToolsUsed:        toolsUsed,
	}
}
