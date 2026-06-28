package eino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/agentloop"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/reasoningtrace"
	"kylin-guard-agent/agent-go/internal/report"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type Runtime struct {
	config       RuntimeConfig
	registry     *tools.Registry
	guard        security.IntentGuard
	auditor      auditclient.Client
	traces       *logtrace.Store
	toolAdapter  PlanToolAdapter
	chatModel    ChatModelAdapter
	graphRuntime *GraphRuntime
}

func NewRuntime(registry *tools.Registry, auditor auditclient.Client, traceStore *logtrace.Store, config RuntimeConfig) *Runtime {
	if registry == nil {
		registry = tools.NewDefaultRegistry()
	}
	if auditor == nil {
		auditor = auditclient.NewLocalSafetyClient()
	}
	if traceStore == nil {
		traceStore = logtrace.NewStore()
	}
	normalized := NormalizeRuntimeConfig(config)
	toolAdapter := NewMCPToolAdapter(registry, security.NewToolPolicy())

	var primaryModel ChatModelAdapter
	adapterCfg := ChatModelAdapterConfig{
		Provider: normalized.LLMProvider,
		Endpoint: normalized.LLMEndpoint,
		Model:    normalized.LLMModel,
		APIKey:   normalized.LLMAPIKey,
	}
	if normalized.LLMEnabled && normalized.LLMProvider != "deterministic" {
		primaryModel = NewRemoteLLMAdapter(adapterCfg, registry)
	} else {
		primaryModel = NewDeterministicChatModelStub(registry)
	}
	// Always wrap with fallback adapter for resilience.
	chatModel := NewFallbackChatModelAdapter(primaryModel, registry)

	return &Runtime{
		config:       normalized,
		registry:     registry,
		guard:        security.NewIntentGuard(),
		auditor:      auditor,
		traces:       traceStore,
		toolAdapter:  toolAdapter,
		chatModel:    chatModel,
		graphRuntime: NewGraphRuntime(chatModel, toolAdapter),
	}
}

func (r *Runtime) Run(ctx context.Context, req agent.AgentRunRequest) (agent.AgentRunResponse, error) {
	if !r.Enabled() {
		return agent.AgentRunResponse{}, errors.New("eino graph runtime is disabled")
	}

	task := strings.TrimSpace(req.Task)
	if task == "" {
		return agent.AgentRunResponse{}, errors.New("task is required")
	}
	ctx, fallbackOutcome := withFallbackOutcome(ctx)

	// Initialize reasoning trace.
	rtb := reasoningtrace.NewTraceBuilder("eino", task)
	requestSpan := rtb.StartSpan("", reasoningtrace.SpanRequest, "Eino Graph Runtime request")

	// Intent guard.
	intent := r.guard.Evaluate(task)
	intentGuardSpan := rtb.StartSpan(requestSpan.SpanID, reasoningtrace.SpanIntentGuard, "intent_guard evaluate")
	rtb.SetAttr(intentGuardSpan.SpanID, "decision", string(intent.Decision))
	if intent.Decision == security.DecisionDeny {
		rtb.SetAttr(intentGuardSpan.SpanID, "blocked_reason", "dangerous task denied before tool execution")
		rtb.EndSpan(intentGuardSpan.SpanID, "deny")
		rtb.EndSpan(requestSpan.SpanID, "deny")
		rtb.Finish()

		audit := intentGuardAuditResult(intent)
		securityReport := report.BuildSecurityReport(report.BuildInput{
			Task:        task,
			Decision:    string(security.DecisionDeny),
			Summary:     "request denied by intent guard",
			ToolTrace:   []logtrace.ToolTrace{},
			AuditResult: audit,
			Route:       r.config.Route,
		})
		attachRuntimeMetadata(securityReport, r.config.Metadata(nil), r.registry)
		resp := agent.AgentRunResponse{
			Task:               task,
			InteractionType:    agent.InteractionTypeSafeRefusal,
			RouterSource:       agent.RouterSourceSafetyGuard,
			RouterConfidence:   agent.RouterConfidenceHigh,
			NeedsToolExecution: false,
			RouterReason:       intent.Reason,
			Decision:           string(security.DecisionDeny),
			Summary:            "request denied by intent guard",
			SecurityReport:     securityReport,
			ToolTrace:          []logtrace.ToolTrace{},
			AuditResult:        audit,
			ReasoningTrace:     rtb.Finish(),
		}
		agent.AttachScenarioWorkspaceMetadata(&resp, task, agent.RunStatusBlocked)
		agent.AttachUserMessage(&resp)
		return resp, nil
	}
	rtb.EndSpan(intentGuardSpan.SpanID, "allow")

	route := r.routeInteraction(ctx, task)
	if route.InteractionType == agent.InteractionTypeChat {
		rtb.EndSpan(requestSpan.SpanID, "chat_only")
		rtb.Finish()
		return r.runChatOnly(ctx, task, req.Messages, route), nil
	}
	if route.InteractionType == agent.InteractionTypeClarify {
		rtb.EndSpan(requestSpan.SpanID, "clarify")
		rtb.Finish()
		return agent.NewClarifyResponse(task, route), nil
	}
	if route.InteractionType == agent.InteractionTypeSafeRefusal {
		rtb.EndSpan(requestSpan.SpanID, "safe_refusal")
		rtb.Finish()
		return agent.NewSafeRefusalResponse(task, route), nil
	}

	// Stage 16A+: run-eino uses the Agent Loop for concrete operations tasks.
	// When a remote LLM is configured it drives the loop; otherwise the
	// deterministic adapter acts as the fallback generator.
	return r.runAgentLoop(ctx, task, rtb, requestSpan, intent, route, fallbackOutcome)
}

func (r *Runtime) Name() string {
	return "eino_graph_runtime"
}

func (r *Runtime) Enabled() bool {
	return r != nil && r.config.RuntimeEnabled
}

func (r *Runtime) routeInteraction(ctx context.Context, task string) agent.InteractionRoute {
	if r.config.LLMEnabled && r.config.LLMProvider != "deterministic" && r.config.LLMAPIKey != "" {
		if route, err := r.routeInteractionWithLLM(ctx, task); err == nil {
			return route
		}
	}
	return agent.ConservativeRoute(task)
}

func (r *Runtime) routeInteractionWithLLM(ctx context.Context, task string) (agent.InteractionRoute, error) {
	llmAdapter := r.remoteLLMAdapter()
	if llmAdapter == nil {
		return agent.InteractionRoute{}, errors.New("remote LLM adapter not available for router")
	}
	messages := []map[string]any{
		{"role": "system", "content": interactionRouterPrompt()},
		{"role": "user", "content": task},
	}
	content, err := llmAdapter.callChatCompletions(ctx, messages)
	if err != nil {
		return agent.InteractionRoute{}, err
	}
	content = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(content, "```"), "```json"))
	var raw struct {
		InteractionType    string `json:"interaction_type"`
		Confidence         string `json:"confidence"`
		NeedsToolExecution bool   `json:"needs_tool_execution"`
		Reason             string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return agent.InteractionRoute{}, err
	}
	route := agent.InteractionRoute{
		InteractionType:    normalizeRouterInteraction(raw.InteractionType),
		RouterSource:       agent.RouterSourceLLMRouter,
		Confidence:         normalizeRouterConfidence(raw.Confidence),
		NeedsToolExecution: raw.NeedsToolExecution,
		Reason:             raw.Reason,
	}
	if route.InteractionType == agent.InteractionTypeAgentRun {
		route.NeedsToolExecution = true
	} else {
		route.NeedsToolExecution = false
	}
	return route, nil
}

func (r *Runtime) runChatOnly(ctx context.Context, task string, history []agent.ConversationMessage, route agent.InteractionRoute) agent.AgentRunResponse {
	llmAdapter := r.remoteLLMAdapter()
	if llmAdapter == nil {
		return agent.NewChatOnlyResponseWithAnswer(task, route, r.deterministicChatAnswer(task))
	}

	messages := []map[string]any{
		{
			"role":    "system",
			"content": fmt.Sprintf(`You are KylinGuard's conversational assistant. Answer the user naturally and directly in the user's language. The configured provider is %s and the configured model identifier is %s. If the user asks which model you use, state those values accurately. Do not claim that you inspected the system, executed tools, or observed live machine state unless tool results were actually provided. Keep the answer concise and helpful.`, r.config.LLMProvider, r.config.LLMModel),
		},
	}
	messages = append(messages, chatHistoryMessages(task, history)...)
	answer, err := llmAdapter.callChatCompletions(ctx, messages)
	if err != nil || strings.TrimSpace(answer) == "" {
		return agent.NewChatOnlyResponse(task, route)
	}
	return agent.NewChatOnlyResponseWithAnswer(task, route, answer)
}

func (r *Runtime) deterministicChatAnswer(task string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(task), " "))
	if strings.Contains(normalized, "什么模型") ||
		strings.Contains(normalized, "哪个模型") ||
		strings.Contains(normalized, "模型是什么") ||
		strings.Contains(normalized, "what model") ||
		strings.Contains(normalized, "which model") {
		return fmt.Sprintf("我是 KylinGuard Agent。当前运行的是 %s，未启用远程大模型。配置远程模型后，普通对话会由对应模型生成回答。", r.config.ChatModelName())
	}
	return "你好，我是 KylinGuard。有什么可以帮你？"
}

func chatHistoryMessages(task string, history []agent.ConversationMessage) []map[string]any {
	const maxMessages = 20
	const maxContentLength = 4000

	if len(history) > maxMessages {
		history = history[len(history)-maxMessages:]
	}
	messages := make([]map[string]any, 0, len(history)+1)
	for _, message := range history {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		contentRunes := []rune(content)
		if len(contentRunes) > maxContentLength {
			content = string(contentRunes[:maxContentLength])
		}
		content, _ = security.NeutralizeUntrustedText(content)
		messages = append(messages, map[string]any{"role": role, "content": content})
	}
	if len(messages) == 0 || messages[len(messages)-1]["role"] != "user" || messages[len(messages)-1]["content"] != task {
		messages = append(messages, map[string]any{"role": "user", "content": task})
	}
	return messages
}

func (r *Runtime) remoteLLMAdapter() *RemoteLLMAdapter {
	if fb, ok := r.chatModel.(*FallbackChatModelAdapter); ok {
		if primary, ok := fb.primary.(*RemoteLLMAdapter); ok {
			return primary
		}
	}
	return nil
}

func interactionRouterPrompt() string {
	return `You are a semantic interaction router for KylinGuard.
Classify the user input. Output ONLY JSON with:
{"interaction_type":"chat|agent_run|safe_refusal|clarify","confidence":"high|medium|low","needs_tool_execution":false,"reason":"short reason"}

Use semantic meaning, not keyword matching.
- chat: greeting, help, product explanation, or casual conversation. The user is not asking to inspect real system state.
- agent_run: the user clearly asks to diagnose, inspect, check, or troubleshoot a concrete system/service/port/log/performance/login problem, requiring tool execution.
- safe_refusal: the user asks for destructive, evasive, audit-removal, log-deletion, firewall-disabling, privilege-escalation, or similarly unsafe actions.
- clarify: the request may relate to operations, but lacks a concrete symptom, target, or objective. When uncertain, choose clarify.

The router must not output tool names, commands, or plans.`
}

func normalizeRouterInteraction(value string) string {
	switch strings.TrimSpace(value) {
	case agent.InteractionTypeChat, agent.InteractionTypeAgentRun, agent.InteractionTypeSafeRefusal, agent.InteractionTypeClarify:
		return value
	default:
		return agent.InteractionTypeClarify
	}
}

func normalizeRouterConfidence(value string) string {
	switch strings.TrimSpace(value) {
	case agent.RouterConfidenceHigh, agent.RouterConfidenceMedium, agent.RouterConfidenceLow:
		return value
	default:
		return agent.RouterConfidenceLow
	}
}

func intentGuardAuditResult(intent security.IntentResult) auditclient.Result {
	threatType := intent.ThreatType
	if threatType == "" {
		threatType = security.ThreatTypeDangerousIntent
	}
	message := "dangerous task denied before tool execution"
	if threatType == security.ThreatTypePromptInjection {
		message = "prompt injection attempt denied before tool execution"
	}
	return auditclient.Result{
		Decision:  string(security.DecisionDeny),
		RiskScore: 1.0,
		Violations: []auditclient.Violation{
			{
				Type:     threatType,
				Severity: "high",
				Message:  message,
				StepID:   "",
			},
		},
		EvidenceChain: []auditclient.EvidenceItem{},
		RiskGraph:     nil,
		Method:        "intent_guard",
		Message:       message,
	}
}

func reportPlanFromAgentPlan(plan *agent.Plan) *report.Plan {
	if plan == nil {
		return nil
	}
	steps := make([]report.PlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, report.PlanStep{
			StepID:          step.StepID,
			ToolName:        step.ToolName,
			Input:           step.Input,
			Reason:          step.Reason,
			ToolCategory:    step.ToolCategory,
			RiskLevel:       step.RiskLevel,
			PermissionScope: step.PermissionScope,
		})
	}
	return &report.Plan{
		Task:     plan.Task,
		Scenario: plan.Scenario,
		Summary:  plan.Summary,
		Steps:    steps,
	}
}

func reportDiagnosisFromAgentDiagnosis(diagnosis *agent.Diagnosis) *report.Diagnosis {
	if diagnosis == nil {
		return nil
	}
	return &report.Diagnosis{
		Scenario:        diagnosis.Scenario,
		RiskLevel:       diagnosis.RiskLevel,
		Findings:        append([]string{}, diagnosis.Findings...),
		Recommendations: append([]string{}, diagnosis.Recommendations...),
		Details:         diagnosis.Details,
	}
}

func diagnosisFromSSHLoginAnalyzer(scenario string, output any) *agent.Diagnosis {
	result, ok := output.(tools.SSHLoginAnalyzerResult)
	if !ok {
		if pointer, pointerOK := output.(*tools.SSHLoginAnalyzerResult); pointerOK && pointer != nil {
			result = *pointer
			ok = true
		}
	}
	if !ok {
		return nil
	}

	analysis := result.Analysis
	return &agent.Diagnosis{
		Scenario:        scenario,
		RiskLevel:       analysis.RiskLevel,
		Findings:        append([]string{}, analysis.Findings...),
		Recommendations: sshDiagnosisRecommendations(analysis.RiskLevel),
		Details: map[string]any{
			"log_collection": result.LogCollection,
			"analysis":       analysis,
		},
	}
}

// diagnosisFromToolOutput generates a diagnosis from Stage 10 OS sensing tool outputs.
func diagnosisFromToolOutput(scenario string, toolName string, output any) *agent.Diagnosis {
	switch toolName {
	case "resource_usage_checker":
		result, ok := output.(tools.ResourceUsageCheckerResult)
		if !ok {
			if ptr, ok := output.(*tools.ResourceUsageCheckerResult); ok && ptr != nil {
				result = *ptr
			} else {
				return nil
			}
		}
		riskLevel := result.RiskLevel
		if riskLevel == "" {
			riskLevel = "unknown"
		}
		findings := []string{}
		if result.LoadAvg != nil {
			findings = append(findings, fmt.Sprintf("loadavg_1m=%.2f, 5m=%.2f, 15m=%.2f", result.LoadAvg.OneMin, result.LoadAvg.FiveMin, result.LoadAvg.FifteenMin))
		} else {
			findings = append(findings, "Load average data unavailable")
		}
		if result.Memory != nil {
			findings = append(findings, fmt.Sprintf("mem_total_kb=%d, mem_available_kb=%d, mem_available_ratio=%.2f", result.Memory.MemTotalKB, result.Memory.MemAvailableKB, result.Memory.MemAvailableRatio))
		} else {
			findings = append(findings, "Memory data unavailable")
		}
		return &agent.Diagnosis{
			Scenario:        scenario,
			RiskLevel:       riskLevel,
			Findings:        findings,
			Recommendations: resourceDiagnosisRecommendations(riskLevel),
			Details:         map[string]any{"result": result},
		}
	case "disk_memory_checker":
		result, ok := output.(tools.DiskMemoryCheckerResult)
		if !ok {
			if ptr, ok := output.(*tools.DiskMemoryCheckerResult); ok && ptr != nil {
				result = *ptr
			} else {
				return nil
			}
		}
		riskLevel := result.RiskLevel
		if riskLevel == "" {
			riskLevel = "unknown"
		}
		findings := []string{}
		for _, fs := range result.Filesystems {
			findings = append(findings, fmt.Sprintf("filesystem %s mounted at %s: %.0f%% used", fs.Filesystem, fs.Mountpoint, fs.UsedPercent))
		}
		if result.Memory != nil {
			findings = append(findings, fmt.Sprintf("mem_total_kb=%d, mem_available_kb=%d", result.Memory.MemTotalKB, result.Memory.MemAvailableKB))
		}
		return &agent.Diagnosis{
			Scenario:        scenario,
			RiskLevel:       riskLevel,
			Findings:        findings,
			Recommendations: resourceDiagnosisRecommendations(riskLevel),
			Details:         map[string]any{"result": result},
		}
	case "network_connection_inspector":
		result, ok := output.(tools.NetworkConnectionInspectorResult)
		if !ok {
			if ptr, ok := output.(*tools.NetworkConnectionInspectorResult); ok && ptr != nil {
				result = *ptr
			} else {
				return nil
			}
		}
		riskLevel := "low"
		for _, conn := range result.Connections {
			if strings.Contains(conn.LocalAddress, "0.0.0.0:22") {
				riskLevel = "medium"
			}
		}
		findings := []string{fmt.Sprintf("Found %d connections", result.Count)}
		for _, conn := range result.Connections {
			findings = append(findings, fmt.Sprintf("%s %s %s %s", conn.Protocol, conn.State, conn.LocalAddress, conn.Process))
		}
		return &agent.Diagnosis{
			Scenario:        scenario,
			RiskLevel:       riskLevel,
			Findings:        findings,
			Recommendations: networkDiagnosisRecommendations(riskLevel),
			Details:         map[string]any{"result": result},
		}
	case "process_inspector":
		result, ok := output.(tools.ProcessInspectorResult)
		if !ok {
			if ptr, ok := output.(*tools.ProcessInspectorResult); ok && ptr != nil {
				result = *ptr
			} else {
				return nil
			}
		}
		riskLevel := "low"
		if result.Count == 0 {
			riskLevel = "medium"
		}
		findings := []string{fmt.Sprintf("Found %d processes", result.Count)}
		for _, proc := range result.Processes {
			findings = append(findings, fmt.Sprintf("pid=%d name=%s state=%s", proc.PID, proc.Name, proc.State))
		}
		return &agent.Diagnosis{
			Scenario:        scenario,
			RiskLevel:       riskLevel,
			Findings:        findings,
			Recommendations: processDiagnosisRecommendations(riskLevel),
			Details:         map[string]any{"result": result},
		}
	}
	return nil
}

func resourceDiagnosisRecommendations(riskLevel string) []string {
	switch riskLevel {
	case "high":
		return []string{
			"Immediately review high resource usage; disk or memory may be critically low.",
			"Consider expanding storage or freeing memory before further operations.",
		}
	case "medium":
		return []string{
			"Monitor resource usage trends; disk usage or memory pressure is elevated.",
		}
	case "low":
		return []string{
			"System resources are within normal operating range.",
		}
	default:
		return []string{
			"Resource usage data was unavailable; verify /proc is mounted and readable.",
		}
	}
}

func networkDiagnosisRecommendations(riskLevel string) []string {
	switch riskLevel {
	case "medium":
		return []string{
			"SSH is listening on 0.0.0.0:22; review whether this exposure is intended.",
			"Consider restricting SSH to 127.0.0.1 or trusted networks if remote access is not needed.",
		}
	case "low":
		return []string{
			"Network connections appear normal; no unexpected exposure detected.",
		}
	default:
		return []string{
			"Network connection data was unavailable; verify ss or netstat is available.",
		}
	}
}

func processDiagnosisRecommendations(riskLevel string) []string {
	switch riskLevel {
	case "medium":
		return []string{
			"Target process was not found; verify the service is running.",
			"Check systemctl status or review service configuration.",
		}
	case "low":
		return []string{
			"Target process is running normally.",
		}
	default:
		return []string{
			"Process data was unavailable; verify ps or tasklist is available.",
		}
	}
}

func sshDiagnosisRecommendations(riskLevel string) []string {
	switch riskLevel {
	case "high":
		return []string{
			"Review SSH exposure and restrict access to trusted IP ranges.",
			"Consider enabling rate limiting or fail2ban-style protection.",
			"Inspect top failed source IPs.",
		}
	case "medium":
		return []string{
			"Review repeated failed SSH login attempts.",
			"Check whether source IPs are expected.",
		}
	case "low":
		return []string{
			"No obvious SSH brute-force pattern detected in the available logs.",
		}
	default:
		return []string{
			"SSH authentication logs were unavailable; verify log configuration or permissions.",
		}
	}
}

func toolsUsed(traces []logtrace.ToolTrace) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, trace := range traces {
		if trace.ToolName == "" || seen[trace.ToolName] {
			continue
		}
		seen[trace.ToolName] = true
		result = append(result, trace.ToolName)
	}
	return result
}

func attachRuntimeMetadata(securityReport *report.SecurityReport, metadata RuntimeMetadata, registry *tools.Registry) {
	if securityReport == nil {
		return
	}
	if securityReport.AuditMetadata == nil {
		securityReport.AuditMetadata = map[string]any{}
	}
	securityReport.AuditMetadata["route"] = metadata.Route
	securityReport.AuditMetadata["runtime"] = metadata.Runtime
	securityReport.AuditMetadata["eino_graph_enabled"] = metadata.EinoGraph
	securityReport.AuditMetadata["llm_enabled"] = metadata.LLMEnabled
	securityReport.AuditMetadata["chat_model"] = metadata.ChatModel
	securityReport.AuditMetadata["chat_model_adapter"] = metadata.ChatModelAdapter
	securityReport.AuditMetadata["orchestration"] = metadata.Orchestration
	securityReport.AuditMetadata["tool_protocol"] = metadata.ToolProtocol
	securityReport.AuditMetadata["tool_protocol_version"] = tools.ToolProtocolVersion
	securityReport.AuditMetadata["eino_runtime_version"] = metadata.Version
	securityReport.AuditMetadata["tools_used"] = append([]string{}, metadata.ToolsUsed...)
	if registry != nil {
		securityReport.AuditMetadata["registered_tool_count"] = len(registry.ListTools())
	}
}

// runAgentLoop executes the Stage 16A Agent Loop for remote LLM.
func (r *Runtime) runAgentLoop(ctx context.Context, task string, rtb *reasoningtrace.TraceBuilder, requestSpan *reasoningtrace.ReasoningSpan, intent security.IntentResult, route agent.InteractionRoute, fallbackOutcome *FallbackOutcome) (agent.AgentRunResponse, error) {
	// Choose the agent-loop generator. A configured remote LLM drives the loop;
	// otherwise the deterministic adapter acts as the fallback generator so run-eino
	// always runs an Agent Loop (req 1).
	var gen agentloop.NextActionGenerator
	if r.config.LLMEnabled && r.config.LLMProvider != "deterministic" && r.config.LLMAPIKey != "" {
		var llmAdapter *RemoteLLMAdapter
		if fb, ok := r.chatModel.(*FallbackChatModelAdapter); ok {
			if remote, ok2 := fb.primary.(*RemoteLLMAdapter); ok2 {
				llmAdapter = remote
			}
		}
		if llmAdapter == nil {
			return agent.AgentRunResponse{}, errors.New("remote LLM adapter not available for agent loop")
		}
		gen = NewFallbackNextActionGenerator(NewRemoteLLMAgentAdapter(llmAdapter), NewDeterministicAgentAdapter(NewDeterministicChatModelStub(r.registry)), fallbackOutcome)
	} else {
		var stub *DeterministicChatModelStub
		if fb, ok := r.chatModel.(*FallbackChatModelAdapter); ok {
			if s, ok2 := fb.primary.(*DeterministicChatModelStub); ok2 {
				stub = s
			}
		}
		if stub == nil {
			stub = NewDeterministicChatModelStub(r.registry)
		}
		gen = NewDeterministicAgentAdapter(stub)
	}

	exec := agentloop.NewToolStepExecutor(r.registry)
	engine := agentloop.NewEngineWithAuditor(gen, exec, r.registry, NewAuditClientStepAuditor(r.auditor))
	engine.MaxSteps = r.config.AgentMaxSteps

	loopResp, err := engine.Run(ctx, task, rtb, requestSpan.SpanID)
	if err != nil {
		return agent.AgentRunResponse{}, err
	}

	// Collect tool traces from executor for the global trace store / Inspector.
	traces := exec.GetTraces()
	for _, tr := range traces {
		r.traces.Add(tr)
	}

	// Per-step audit_reports are produced by the engine (one per tool_call, req 7);
	// the aggregated risk_graph is built by the engine (req 8). No more whole-sale
	// AuditTrace call — the audit happens per step inside the loop.

	// Aggregate an audit result for backward-compatible fields (DecisionCard,
	// security_report) from the per-step audit_reports.
	audit := aggregateAuditFromSteps(loopResp.AgentSteps, intent)
	if audit.Decision == "" {
		audit.Decision = string(intent.Decision)
	}

	// Convert agent steps to maps for JSON serialization (include per-step audit_report).
	stepMaps := make([]map[string]any, 0, len(loopResp.AgentSteps))
	for _, step := range loopResp.AgentSteps {
		m := map[string]any{
			"step_index":           step.StepIndex,
			"action_type":          step.ActionType,
			"tool_name":            step.ToolName,
			"tool_args":            step.ToolArgs,
			"reason":               step.Reason,
			"user_visible_summary": step.UserVisibleSummary,
			"policy_decision":      step.PolicyDecision,
			"observation":          step.Observation,
			"operation_type":       step.OperationType,
			"resource_type":        step.ResourceType,
			"boundary_level":       step.BoundaryLevel,
			"allowed_by_policy":    step.AllowedByPolicy,
			"policy_reason":        step.PolicyReason,
		}
		if step.AuditReport != nil {
			m["audit_report"] = map[string]any{
				"step_id":    step.AuditReport.StepID,
				"step_index": step.AuditReport.StepIndex,
				"tool_name":  step.AuditReport.ToolName,
				"decision":   step.AuditReport.Decision,
				"risk_score": step.AuditReport.RiskScore,
				"violations": step.AuditReport.Violations,
				"evidence":   step.AuditReport.Evidence,
				"method":     step.AuditReport.Method,
				"message":    step.AuditReport.Message,
			}
		}
		stepMaps = append(stepMaps, m)
	}

	decision := audit.Decision
	if loopResp.FinalAnswer != "" && audit.Decision != "deny" {
		decision = string(security.DecisionReview)
	}

	// Collect distinct tool names actually used by the agent loop, for the
	// tools_used audit metadata (parity with the former graph-runtime metadata).
	toolsUsed := make([]string, 0, len(traces))
	seen := make(map[string]bool, len(traces))
	for _, tr := range traces {
		if !seen[tr.ToolName] {
			seen[tr.ToolName] = true
			toolsUsed = append(toolsUsed, tr.ToolName)
		}
	}

	// Build security report with real traces and the aggregated audit result.
	metadata := r.config.Metadata(toolsUsed)
	buildInput := report.BuildInput{
		Task:        task,
		Decision:    decision,
		Summary:     "Agent loop completed",
		ToolTrace:   traces,
		AuditResult: audit,
		Route:       metadata.Route,
	}
	securityReport := report.BuildSecurityReport(buildInput)
	attachRuntimeMetadata(securityReport, metadata, r.registry)
	r.attachLLMMetadata(securityReport, fallbackOutcome)

	resp := agent.AgentRunResponse{
		Task:               task,
		InteractionType:    agent.InteractionTypeAgentRun,
		RouterSource:       route.RouterSource,
		RouterConfidence:   route.Confidence,
		NeedsToolExecution: route.NeedsToolExecution,
		RouterReason:       route.Reason,
		Decision:           decision,
		Summary:            "Eino graph runtime executed agent loop orchestration.",
		AgentMode:          string(agentloop.ModeAgentLoop),
		TaskUnderstanding: func() map[string]any {
			if loopResp.TaskUnderstanding != nil {
				return map[string]any{
					"user_goal":   loopResp.TaskUnderstanding.UserGoal,
					"intent_type": loopResp.TaskUnderstanding.IntentType,
					"risk_level":  loopResp.TaskUnderstanding.RiskLevel,
				}
			}
			return nil
		}(),
		AgentSteps:     stepMaps,
		FinalAnswer:    loopResp.FinalAnswer,
		SecurityReport: securityReport,
		ToolTrace:      traces,
		AuditResult:    audit,
		RiskGraph:      loopResp.RiskGraph,
		ReasoningTrace: rtb.Finish(),
	}
	agent.AttachScenarioWorkspaceMetadata(&resp, task, agent.RunStatusCompleted)
	agent.AttachUserMessage(&resp)
	return resp, nil
}

// aggregateAuditFromSteps synthesizes a single auditclient.Result from the
// per-step AuditReports: worst decision (deny>review>allow), max risk score,
// and flattened violations/evidence. Keeps the legacy DecisionCard/security_report
// fields meaningful now that audit happens per step rather than wholesale.
func aggregateAuditFromSteps(steps []agentloop.AgentStep, intent security.IntentResult) auditclient.Result {
	if len(steps) == 0 {
		// No steps (pure final_answer): no audit ran.
		return auditclient.Result{Method: "no-audit", Decision: string(security.DecisionAllow)}
	}
	worst := string(security.DecisionAllow)
	rank := map[string]int{"allow": 0, "review": 1, "deny": 2}
	var maxRisk float64
	var violations []auditclient.Violation
	var evidence []auditclient.EvidenceItem
	for _, s := range steps {
		if s.AuditReport == nil {
			continue
		}
		ar := s.AuditReport
		if r, ok := rank[ar.Decision]; ok {
			if cur, ok2 := rank[worst]; !ok2 || r > cur {
				worst = ar.Decision
			}
		}
		if ar.RiskScore > maxRisk {
			maxRisk = ar.RiskScore
		}
		for _, v := range ar.Violations {
			violations = append(violations, auditclient.Violation{
				Type:     "step_audit",
				Severity: severityForAuditDecision(ar.Decision),
				Message:  v,
				StepID:   ar.StepID,
			})
		}
		if len(ar.Evidence) == 0 {
			evidence = append(evidence, auditclient.EvidenceItem{
				StepID:   ar.StepID,
				ToolName: ar.ToolName,
				Resource: s.ResourcePath,
				Reason:   stepAuditEvidenceReason(s, ar),
			})
		} else {
			for _, e := range ar.Evidence {
				evidence = append(evidence, auditclient.EvidenceItem{
					ToolName: ar.ToolName,
					Resource: s.ResourcePath,
					Reason:   e,
					StepID:   ar.StepID,
				})
			}
		}
	}
	if violations == nil {
		violations = []auditclient.Violation{}
	}
	if evidence == nil {
		evidence = []auditclient.EvidenceItem{}
	}
	return auditclient.Result{
		Decision:      worst,
		RiskScore:     maxRisk,
		Violations:    violations,
		EvidenceChain: evidence,
		RiskGraph:     aggregateRiskGraphFromSteps(steps),
		Method:        "per-step-aggregate",
		Message:       "aggregated from per-step audit reports",
	}
}

func severityForAuditDecision(decision string) string {
	switch decision {
	case "deny":
		return "high"
	case "review":
		return "medium"
	default:
		return "low"
	}
}

func stepAuditEvidenceReason(step agentloop.AgentStep, report *agentloop.AuditReport) string {
	if report != nil && strings.TrimSpace(report.Message) != "" {
		return report.Message
	}
	if strings.TrimSpace(step.UserVisibleSummary) != "" {
		return step.UserVisibleSummary
	}
	if strings.TrimSpace(step.Reason) != "" {
		return step.Reason
	}
	return fmt.Sprintf("step %d executed %s against %s", step.StepIndex, step.ToolName, step.ResourceType)
}

func aggregateRiskGraphFromSteps(steps []agentloop.AgentStep) *auditclient.RiskGraph {
	if len(steps) == 0 {
		return nil
	}
	nodes := make([]map[string]any, 0, len(steps))
	edges := make([]map[string]any, 0, maxInt(0, len(steps)-1))
	for index, step := range steps {
		stepID := fmt.Sprintf("step-%03d", step.StepIndex)
		decision := ""
		method := ""
		riskScore := 0.0
		violationsCount := 0
		if step.AuditReport != nil {
			if step.AuditReport.StepID != "" {
				stepID = step.AuditReport.StepID
			}
			decision = step.AuditReport.Decision
			method = step.AuditReport.Method
			riskScore = step.AuditReport.RiskScore
			violationsCount = len(step.AuditReport.Violations)
		}
		nodes = append(nodes, map[string]any{
			"id":                stepID,
			"step_id":           stepID,
			"step_index":        step.StepIndex,
			"type":              "tool_call",
			"label":             step.ToolName,
			"tool_name":         step.ToolName,
			"decision":          decision,
			"risk_score":        riskScore,
			"risk_level":        severityForAuditDecision(decision),
			"violations_count":  violationsCount,
			"method":            method,
			"policy_decision":   step.PolicyDecision,
			"operation_type":    step.OperationType,
			"resource_type":     step.ResourceType,
			"resource_path":     step.ResourcePath,
			"boundary_level":    step.BoundaryLevel,
			"allowed_by_policy": step.AllowedByPolicy,
			"policy_reason":     step.PolicyReason,
		})
		if index > 0 {
			prevID := fmt.Sprintf("step-%03d", steps[index-1].StepIndex)
			if steps[index-1].AuditReport != nil && steps[index-1].AuditReport.StepID != "" {
				prevID = steps[index-1].AuditReport.StepID
			}
			edges = append(edges, map[string]any{
				"from":   prevID,
				"to":     stepID,
				"source": prevID,
				"target": stepID,
				"type":   "sequence",
				"label":  "sequence",
			})
		}
	}
	return &auditclient.RiskGraph{Nodes: nodes, Edges: edges}
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

// attachLLMMetadata injects remote LLM execution metadata into the security report.
func (r *Runtime) attachLLMMetadata(securityReport *report.SecurityReport, outcome *FallbackOutcome) {
	if securityReport == nil || securityReport.AuditMetadata == nil {
		return
	}
	_, ok := r.chatModel.(*FallbackChatModelAdapter)
	if !ok {
		securityReport.AuditMetadata["remote_llm_used"] = false
		securityReport.AuditMetadata["fallback_used"] = false
		return
	}
	used, reason := outcome.Info()
	isRemote := r.config.LLMEnabled && r.config.LLMProvider != "deterministic"
	securityReport.AuditMetadata["remote_llm_used"] = isRemote && !used
	if !isRemote {
		securityReport.AuditMetadata["remote_llm_used"] = false
	}
	securityReport.AuditMetadata["fallback_used"] = used
	if used && reason != "" {
		securityReport.AuditMetadata["fallback_reason"] = reason
	}
	// Provider and model info (sanitized: no API key).
	securityReport.AuditMetadata["llm_provider"] = r.config.LLMProvider
	if r.config.LLMModel != "" {
		securityReport.AuditMetadata["llm_model"] = r.config.LLMModel
	}
}
