package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/agentloop"
	"kylin-guard-agent/agent-go/internal/config"
	einoruntime "kylin-guard-agent/agent-go/internal/eino"
	"kylin-guard-agent/agent-go/internal/tools"
)

type runtimeStatusResponse struct {
	OK             bool                      `json:"ok"`
	Runtime        runtimeStatusRuntime      `json:"runtime"`
	Services       runtimeStatusServices     `json:"services"`
	SecurityLayers map[string]string         `json:"security_layers"`
	SecretSafety   runtimeStatusSecretSafety `json:"secret_safety"`
	UpdatedAt      string                    `json:"updated_at"`
}

type runtimeStatusRuntime struct {
	AgentMode     string `json:"agent_mode"`
	CurrentMode   string `json:"current_mode"`
	LLMEnabled    bool   `json:"llm_enabled"`
	RemoteLLMUsed bool   `json:"remote_llm_used"`
	ChatModel     string `json:"chat_model"`
	Provider      string `json:"provider"`
	EndpointKind  string `json:"endpoint_kind"`
	Model         string `json:"model"`
}

type runtimeStatusServices struct {
	GoAgent   serviceStatus `json:"go_agent"`
	AuditCore serviceStatus `json:"audit_core"`
	Frontend  serviceStatus `json:"frontend"`
}

type serviceStatus struct {
	Status string `json:"status"`
	Port   int    `json:"port,omitempty"`
	URL    string `json:"url,omitempty"`
}

type runtimeStatusSecretSafety struct {
	APIKeyPresent bool   `json:"api_key_present"`
	APIKeyDisplay string `json:"api_key_display"`
}

type capabilitiesResponse struct {
	AvailableTools []capabilityTool     `json:"available_tools"`
	ToolPolicy     capabilityToolPolicy `json:"tool_policy"`
	AgentLoop      capabilityAgentLoop  `json:"agent_loop"`
}

type capabilityTool struct {
	ToolName          string `json:"tool_name"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	OperationType     string `json:"operation_type"`
	ResourceType      string `json:"resource_type"`
	BoundaryLevel     string `json:"boundary_level"`
	RequiresPrivilege bool   `json:"requires_privilege"`
	ReadOnly          bool   `json:"read_only"`
	PolicyControlled  bool   `json:"policy_controlled"`
	TraceShieldMapped bool   `json:"traceshield_mapped"`
	Enabled           bool   `json:"enabled"`
}

type capabilityToolPolicy struct {
	Enabled                   bool   `json:"enabled"`
	DefaultMode               string `json:"default_mode"`
	DangerousActionsBlocked   bool   `json:"dangerous_actions_blocked"`
	UnknownToolsDefaultDenied bool   `json:"unknown_tools_default_denied"`
	RawShellExecution         string `json:"raw_shell_execution"`
}

type capabilityAgentLoop struct {
	NextActionSchema []string `json:"next_action_schema"`
	MaxSteps         int      `json:"max_steps"`
}

type acceptanceSummaryResponse struct {
	Stages   []acceptanceStage `json:"stages"`
	Commands []string          `json:"commands"`
	Notes    []string          `json:"notes"`
}

type acceptanceStage struct {
	Name     string         `json:"name"`
	Title    string         `json:"title"`
	Status   string         `json:"status"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

func runtimeStatusHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, buildRuntimeStatus(r.Context(), cfg))
	}
}

func capabilitiesHandler(registry *tools.Registry, maxSteps int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, buildCapabilities(registry, maxSteps))
	}
}

func acceptanceSummaryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, buildAcceptanceSummary())
	}
}

func buildRuntimeStatus(ctx context.Context, cfg config.Config) runtimeStatusResponse {
	runtimeCfg := einoruntime.RuntimeConfig{
		RuntimeEnabled: cfg.EinoRuntimeEnabled,
		GraphEnabled:   cfg.EinoGraphEnabled,
		LLMEnabled:     cfg.EinoLLMEnabled,
		RuntimeName:    einoruntime.DefaultRuntimeName,
		Route:          einoruntime.DefaultRoute,
		ToolProtocol:   tools.ToolProtocol,
		Version:        einoruntime.RuntimeVersion,
		LLMProvider:    cfg.EinoLLMProvider,
		LLMEndpoint:    cfg.EinoLLMEndpoint,
		LLMModel:       cfg.EinoLLMModel,
		LLMAPIKey:      cfg.EinoLLMAPIKey,
		AgentMaxSteps:  cfg.EinoAgentMaxSteps,
	}
	chatModel := runtimeCfg.ChatModelName()
	remoteUsed := cfg.EinoLLMEnabled && cfg.EinoLLMProvider != "deterministic" && cfg.EinoLLMAPIKey != ""
	resp := runtimeStatusResponse{
		OK: true,
		Runtime: runtimeStatusRuntime{
			AgentMode:     "agent_loop",
			CurrentMode:   currentRuntimeMode(chatModel, cfg),
			LLMEnabled:    cfg.EinoLLMEnabled,
			RemoteLLMUsed: remoteUsed,
			ChatModel:     chatModel,
			Provider:      cfg.EinoLLMProvider,
			EndpointKind:  endpointKind(cfg.EinoLLMEndpoint),
			Model:         cfg.EinoLLMModel,
		},
		Services: runtimeStatusServices{
			GoAgent:   serviceStatus{Status: "ok", Port: portFromAddr(cfg.Addr, 8080)},
			AuditCore: auditCoreStatus(ctx, cfg.AuditCoreURL),
			Frontend:  serviceStatus{Status: "unknown", Port: getenvInt("FRONTEND_PORT", 5173)},
		},
		SecurityLayers: map[string]string{
			"intent_guard":    "enabled",
			"tool_policy":     "enabled",
			"exec_proxy":      "enabled",
			"traceshield":     "enabled",
			"reasoning_trace": "enabled",
		},
		SecretSafety: runtimeStatusSecretSafety{
			APIKeyPresent: cfg.EinoLLMAPIKey != "",
			APIKeyDisplay: "[REDACTED]",
		},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return resp
}

func buildCapabilities(registry *tools.Registry, maxSteps int) capabilitiesResponse {
	toolMetadata := registry.ListTools()
	available := make([]capabilityTool, 0, len(toolMetadata))
	for _, meta := range toolMetadata {
		if !registry.IsToolEnabledForDirectCall(meta.Name) {
			continue
		}
		available = append(available, capabilityTool{
			ToolName:          meta.Name,
			DisplayName:       meta.Name,
			Description:       meta.Description,
			OperationType:     meta.OperationType,
			ResourceType:      meta.ResourceType,
			BoundaryLevel:     meta.BoundaryLevel,
			RequiresPrivilege: meta.RequiresPrivilege,
			ReadOnly:          meta.OperationType == "read" || meta.OperationType == "inspect" || meta.OperationType == "analyze" || meta.OperationType == "verify",
			PolicyControlled:  true,
			TraceShieldMapped: meta.OperationType != "" && meta.ResourceType != "" && meta.BoundaryLevel != "",
			Enabled:           meta.Enabled,
		})
	}
	return capabilitiesResponse{
		AvailableTools: available,
		ToolPolicy: capabilityToolPolicy{
			Enabled:                   true,
			DefaultMode:               "least_privilege",
			DangerousActionsBlocked:   true,
			UnknownToolsDefaultDenied: true,
			RawShellExecution:         "not_allowed",
		},
		AgentLoop: capabilityAgentLoop{
			NextActionSchema: []string{agentloop.ActionToolCall, agentloop.ActionFinalAnswer},
			MaxSteps:         einoruntime.NormalizeRuntimeConfig(einoruntime.RuntimeConfig{AgentMaxSteps: maxSteps}).AgentMaxSteps,
		},
	}
}

func buildAcceptanceSummary() acceptanceSummaryResponse {
	return acceptanceSummaryResponse{
		Stages: []acceptanceStage{
			{Name: "Stage 15A", Title: "One-click Demo Runtime & Acceptance Hardening", Status: "PASS"},
			{Name: "Stage 16A", Title: "LLM-driven Agent Loop Runtime", Status: "PASS"},
			{Name: "Stage 16B-1", Title: "Frontend Agent Loop Message Mapping", Status: "PASS"},
			{Name: "Stage 16C-lite", Title: "Observability & Acceptance Hardening", Status: "PASS"},
			{Name: "Stage 16D-lite", Title: "Demo Closure & Acceptance Assets", Status: "PASS"},
			{
				Name:   "Stage 16E-lite",
				Title:  "Real DeepSeek natural-language Agent Loop Acceptance",
				Status: "PASS",
				Evidence: map[string]any{
					"chat_model":   "remote-llm-deepseek-openai_compatible",
					"tasks_passed": 4,
					"tasks_failed": 0,
				},
			},
			{Name: "Stage 16F-lite", Title: "Frontend Demo Polish", Status: "PASS"},
			{Name: "Stage 17A-UI-0", Title: "Frontend Framework / Template Reference Audit", Status: "PASS"},
			{Name: "Stage 17A-BE-0", Title: "Product Shell Backend API Plan", Status: "PASS"},
		},
		Commands: []string{
			"bash scripts/linux/check_demo.sh",
			"bash scripts/linux/test_agent_loop_tasks.sh",
		},
		Notes: []string{
			"Acceptance scripts must be run manually on Kylin VM.",
			"This API reports known baseline metadata and does not execute scripts.",
			"Natural-language acceptance samples are not fixed workflows.",
		},
	}
}

func auditCoreStatus(ctx context.Context, auditCoreURL string) serviceStatus {
	status := serviceStatus{Status: "unknown", URL: auditCoreURL, Port: portFromURL(auditCoreURL, 8001)}
	if strings.TrimSpace(auditCoreURL) == "" {
		status.Status = "not_configured"
		return status
	}
	reqCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	endpoint := strings.TrimRight(auditCoreURL, "/") + "/health"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		status.Status = "error"
		return status
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		status.Status = "unreachable"
		return status
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status.Status = "ok"
		return status
	}
	status.Status = "error"
	return status
}

func currentRuntimeMode(chatModel string, cfg config.Config) string {
	switch chatModel {
	case "remote-llm-deepseek-openai_compatible":
		return "real-deepseek"
	case "remote-llm-mock-openai_compatible":
		return "mock-llm"
	case einoruntime.DefaultChatModel:
		return "deterministic-baseline"
	}
	if cfg.EinoLLMEnabled && cfg.EinoLLMProvider != "deterministic" {
		return "remote-llm"
	}
	return "deterministic-baseline"
}

func endpointKind(endpoint string) string {
	normalized := strings.ToLower(endpoint)
	switch {
	case strings.Contains(normalized, "deepseek"):
		return "deepseek"
	case strings.TrimSpace(endpoint) == "":
		return "none"
	default:
		return "openai_compatible"
	}
}

func portFromAddr(addr string, fallback int) int {
	if _, port, err := net.SplitHostPort(addr); err == nil {
		if parsed, parseErr := strconv.Atoi(port); parseErr == nil {
			return parsed
		}
	}
	if strings.HasPrefix(addr, ":") {
		if parsed, err := strconv.Atoi(strings.TrimPrefix(addr, ":")); err == nil {
			return parsed
		}
	}
	return fallback
}

func portFromURL(rawURL string, fallback int) int {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fallback
	}
	if port := parsed.Port(); port != "" {
		if parsedPort, parseErr := strconv.Atoi(port); parseErr == nil {
			return parsedPort
		}
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
