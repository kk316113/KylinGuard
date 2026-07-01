package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
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
	Storage        runtimeStatusStorage      `json:"storage"`
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

type runtimeStatusStorage struct {
	Backend     string `json:"backend"`
	DBPath      string `json:"db_path,omitempty"`
	JSONDir     string `json:"json_dir,omitempty"`
	Limit       int    `json:"limit"`
	Persistence string `json:"persistence"`
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
	ToolCatalog    tools.CatalogSummary `json:"tool_catalog"`
	ToolPolicy     capabilityToolPolicy `json:"tool_policy"`
	AgentLoop      capabilityAgentLoop  `json:"agent_loop"`
	Extensibility  extensibilityModel   `json:"extensibility"`
}

type capabilityTool struct {
	ToolName          string         `json:"tool_name"`
	DisplayName       string         `json:"display_name"`
	Description       string         `json:"description"`
	Category          string         `json:"category"`
	OperationType     string         `json:"operation_type"`
	ResourceType      string         `json:"resource_type"`
	BoundaryLevel     string         `json:"boundary_level"`
	RiskLevel         string         `json:"risk_level"`
	RequiresPrivilege bool           `json:"requires_privilege"`
	ReadOnly          bool           `json:"read_only"`
	LLMCallable       bool           `json:"llm_callable"`
	PolicyControlled  bool           `json:"policy_controlled"`
	TraceShieldMapped bool           `json:"traceshield_mapped"`
	Enabled           bool           `json:"enabled"`
	Platforms         []string       `json:"platforms,omitempty"`
	Architectures     []string       `json:"architectures,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	UseCases          []string       `json:"use_cases,omitempty"`
	SafetyNotes       []string       `json:"safety_notes,omitempty"`
	ExampleInput      map[string]any `json:"example_input,omitempty"`
	AuditEventType    string         `json:"audit_event_type,omitempty"`
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
	MainPath         []string `json:"main_path"`
	ExecutionModel   string   `json:"execution_model"`
	OutputContract   []string `json:"output_contract"`
}

type extensibilityModel struct {
	ToolProtocol        string   `json:"tool_protocol"`
	ToolProtocolVersion string   `json:"tool_protocol_version"`
	AddToolChecklist    []string `json:"add_tool_checklist"`
	PluginBoundaries    []string `json:"plugin_boundaries"`
}

type agentProfileResponse struct {
	OK                    bool                    `json:"ok"`
	Agent                 agentProfileAgent       `json:"agent"`
	RuntimeHost           agentProfileRuntimeHost `json:"runtime_host"`
	TargetEnvironment     agentProfileTarget      `json:"target_environment"`
	OperatingPrinciples   []string                `json:"operating_principles"`
	SafetyBoundaries      []string                `json:"safety_boundaries"`
	RunContract           []string                `json:"run_contract"`
	ToolCatalog           tools.CatalogSummary    `json:"tool_catalog"`
	RecommendedDeployment agentProfileDeployment  `json:"recommended_deployment"`
	UpdatedAt             string                  `json:"updated_at"`
}

type agentProfileAgent struct {
	Name        string   `json:"name"`
	CodeName    string   `json:"code_name"`
	Version     string   `json:"version"`
	Role        string   `json:"role"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
}

type agentProfileRuntimeHost struct {
	GOOS        string            `json:"goos"`
	GOARCH      string            `json:"goarch"`
	Hostname    string            `json:"hostname,omitempty"`
	OSRelease   map[string]string `json:"os_release,omitempty"`
	IsKylinLike bool              `json:"is_kylin_like"`
}

type agentProfileTarget struct {
	OSFamilies     []string `json:"os_families"`
	KylinEditions  []string `json:"kylin_editions"`
	Architectures  []string `json:"architectures"`
	ServiceManager string   `json:"service_manager"`
	PackageManager string   `json:"package_manager"`
	LogSources     []string `json:"log_sources"`
}

type agentProfileDeployment struct {
	Mode       string   `json:"mode"`
	Components []string `json:"components"`
	Notes      []string `json:"notes"`
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

func agentProfileHandler(registry *tools.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, buildAgentProfile(registry))
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
		Storage: runtimeStatusStorage{
			Backend:     cfg.RunStoreBackend,
			DBPath:      cfg.RunStoreDBPath,
			JSONDir:     cfg.RunStoreDir,
			Limit:       cfg.RunStoreLimit,
			Persistence: "agent runs, steps, tool traces, audit results",
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
	toolMetadata := registry.ListDirectCallTools()
	available := make([]capabilityTool, 0, len(toolMetadata))
	for _, meta := range toolMetadata {
		available = append(available, capabilityTool{
			ToolName:          meta.Name,
			DisplayName:       firstNonEmptyString(meta.DisplayName, meta.Name),
			Description:       meta.Description,
			Category:          meta.Category,
			OperationType:     meta.OperationType,
			ResourceType:      meta.ResourceType,
			BoundaryLevel:     meta.BoundaryLevel,
			RiskLevel:         meta.RiskLevel,
			RequiresPrivilege: meta.RequiresPrivilege,
			ReadOnly:          meta.IsReadOnly(),
			LLMCallable:       meta.LLMCallable,
			PolicyControlled:  true,
			TraceShieldMapped: meta.OperationType != "" && meta.ResourceType != "" && meta.BoundaryLevel != "",
			Enabled:           meta.Enabled,
			Platforms:         meta.Platforms,
			Architectures:     meta.Architectures,
			Tags:              meta.Tags,
			UseCases:          meta.UseCases,
			SafetyNotes:       meta.SafetyNotes,
			ExampleInput:      meta.ExampleInput,
			AuditEventType:    meta.AuditEventType,
		})
	}
	return capabilitiesResponse{
		AvailableTools: available,
		ToolCatalog:    registry.CatalogSummary(),
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
			MainPath:         []string{"natural_language_task", "llm_next_action", "intent_guard", "tool_policy", "exec_proxy", "tool_trace", "traceshield_audit", "final_answer"},
			ExecutionModel:   "LLM proposes structured next_action; system validates and executes read-only tools through policy gates.",
			OutputContract:   []string{"final_answer", "agent_steps", "tool_trace", "security_report", "risk_graph", "reasoning_trace"},
		},
		Extensibility: extensibilityModel{
			ToolProtocol:        tools.ToolProtocol,
			ToolProtocolVersion: tools.ToolProtocolVersion,
			AddToolChecklist: []string{
				"implement a bounded read-only handler",
				"register handler with ToolMetadata",
				"declare input_schema and output_schema",
				"declare operation_type/resource_type/boundary_level/risk_level",
				"add Tool Policy coverage and unit tests",
				"ensure tool_trace fields are populated for audit",
			},
			PluginBoundaries: []string{
				"no raw shell from LLM",
				"no mutation tools exposed as direct calls",
				"unknown tools are denied",
				"tool output is untrusted evidence, not instructions",
			},
		},
	}
}

func buildAgentProfile(registry *tools.Registry) agentProfileResponse {
	hostname, _ := os.Hostname()
	osRelease := readOSRelease("/etc/os-release")
	return agentProfileResponse{
		OK: true,
		Agent: agentProfileAgent{
			Name:        "KylinGuard",
			CodeName:    "麒盾",
			Version:     serviceVersion,
			Role:        "security-aware Kylin OS operations agent",
			Description: "A natural-language Agent for safe diagnosis, evidence collection, and security-audited operations on Kylin/Linux servers.",
			Languages:   []string{"zh-CN", "en"},
		},
		RuntimeHost: agentProfileRuntimeHost{
			GOOS:        runtime.GOOS,
			GOARCH:      runtime.GOARCH,
			Hostname:    hostname,
			OSRelease:   osRelease,
			IsKylinLike: isKylinLike(osRelease),
		},
		TargetEnvironment: agentProfileTarget{
			OSFamilies:     []string{"Kylin Linux Advanced Server", "Linux systemd/RPM"},
			KylinEditions:  []string{"银河麒麟高级服务器操作系统 V11"},
			Architectures:  []string{"x86_64", "aarch64", "loongarch64"},
			ServiceManager: "systemd",
			PackageManager: "rpm",
			LogSources:     []string{"/var/log/secure", "/var/log/auth.log", "journalctl", "/var/log/messages"},
		},
		OperatingPrinciples: []string{
			"user natural-language task first",
			"LLM proposes structured next_action only",
			"system decides whether and how tools execute",
			"final_answer is user-facing; audit and risk graph explain evidence",
		},
		SafetyBoundaries: []string{
			"intent_guard runs before tool execution",
			"Tool Policy and Exec Proxy guard every tool call",
			"direct-call tools must be read-only",
			"dangerous mutation operations are denied by default",
			"real API keys are never returned by status/profile APIs",
		},
		RunContract: []string{
			"Every operational run should produce agent_steps when tools are used.",
			"Every executed or denied tool action should produce tool_trace evidence.",
			"Every tool_trace should be auditable by TraceShield or local safety fallback.",
			"Risk graph must be derived from real execution/audit evidence, not fabricated.",
		},
		ToolCatalog: registry.CatalogSummary(),
		RecommendedDeployment: agentProfileDeployment{
			Mode:       "systemd services on Kylin V11",
			Components: []string{"kylin-guard-agent", "kylin-guard-audit", "kylin-guard-web"},
			Notes: []string{
				"Run with non-root service accounts where possible.",
				"Grant only the read permissions required by selected diagnostic tools.",
				"Configure remote LLM credentials through environment or local untracked files only.",
			},
		},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func readOSRelease(path string) map[string]string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if key != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func isKylinLike(osRelease map[string]string) bool {
	for _, key := range []string{"ID", "ID_LIKE", "NAME", "PRETTY_NAME"} {
		value := strings.ToLower(osRelease[key])
		if strings.Contains(value, "kylin") || strings.Contains(value, "麒麟") {
			return true
		}
	}
	return false
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
