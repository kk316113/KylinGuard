package eino

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
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
	chatModel    ToolCallGenerator
	graphRuntime *GraphRuntime
}

func NewRuntime(registry *tools.Registry, auditor auditclient.Client, traceStore *logtrace.Store, config RuntimeConfig) *Runtime {
	if registry == nil {
		registry = tools.NewDefaultRegistry()
	}
	if auditor == nil {
		auditor = auditclient.NewMockClient()
	}
	if traceStore == nil {
		traceStore = logtrace.NewStore()
	}
	normalized := NormalizeRuntimeConfig(config)
	toolAdapter := NewMCPToolAdapter(registry, security.NewToolPolicy())
	chatModel := NewDeterministicChatModelStub(registry)
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

	intent := r.guard.Evaluate(task)
	if intent.Decision == security.DecisionDeny {
		audit := intentGuardAuditResult()
		securityReport := report.BuildSecurityReport(report.BuildInput{
			Task:        task,
			Decision:    string(security.DecisionDeny),
			Summary:     "request denied by intent guard",
			ToolTrace:   []logtrace.ToolTrace{},
			AuditResult: audit,
			Route:       r.config.Route,
		})
		attachRuntimeMetadata(securityReport, r.config.Metadata(nil), r.registry)
		return agent.AgentRunResponse{
			Task:           task,
			Decision:       string(security.DecisionDeny),
			Summary:        "request denied by intent guard",
			SecurityReport: securityReport,
			ToolTrace:      []logtrace.ToolTrace{},
			AuditResult:    audit,
		}, nil
	}

	if !r.config.GraphEnabled {
		return agent.AgentRunResponse{}, errors.New("eino graph runtime is disabled")
	}
	if r.graphRuntime == nil {
		r.graphRuntime = NewGraphRuntime(r.chatModel, r.toolAdapter)
	}
	graphOutput, err := r.graphRuntime.Run(ctx, task)
	if err != nil {
		return agent.AgentRunResponse{}, err
	}

	plan := graphOutput.Plan
	traces := graphOutput.ToolTrace
	if traces == nil {
		traces = []logtrace.ToolTrace{}
	}
	var diagnosis *agent.Diagnosis
	for _, result := range graphOutput.ToolResults {
		if result.Trace.ToolName == "ssh_login_analyzer" {
			diagnosis = diagnosisFromSSHLoginAnalyzer(plan.Scenario, result.Output)
		}
		if diagnosis == nil {
			diagnosis = diagnosisFromToolOutput(plan.Scenario, result.Trace.ToolName, result.Output)
		}
		if result.Trace.ToolName != "" {
			r.traces.Add(result.Trace)
		}
	}

	audit, err := r.auditor.AuditTrace(ctx, task, traces)
	if err != nil {
		return agent.AgentRunResponse{}, err
	}
	if audit.Decision == "" {
		audit.Decision = string(intent.Decision)
	}
	// Normalize TraceShield denial for read-only OS sensing tools.
	diagRisk := ""
	if diagnosis != nil {
		diagRisk = diagnosis.RiskLevel
	}
	audit.Decision = agent.NormalizeAgentDecision(audit.Decision, audit.Method, traces, diagRisk)

	metadata := r.config.Metadata(toolsUsed(traces))
	securityReport := report.BuildSecurityReport(report.BuildInput{
		Task:        task,
		Decision:    audit.Decision,
		Summary:     RuntimeSummary,
		Plan:        reportPlanFromAgentPlan(&plan),
		ToolTrace:   traces,
		Diagnosis:   reportDiagnosisFromAgentDiagnosis(diagnosis),
		AuditResult: audit,
		Route:       metadata.Route,
	})
	attachRuntimeMetadata(securityReport, metadata, r.registry)

	return agent.AgentRunResponse{
		Task:           task,
		Decision:       audit.Decision,
		Summary:        RuntimeSummary,
		Plan:           &plan,
		Diagnosis:      diagnosis,
		SecurityReport: securityReport,
		ToolTrace:      traces,
		AuditResult:    audit,
	}, nil
}

func (r *Runtime) Name() string {
	return "eino_graph_runtime"
}

func (r *Runtime) Enabled() bool {
	return r != nil && r.config.RuntimeEnabled
}

func intentGuardAuditResult() auditclient.Result {
	return auditclient.Result{
		Decision:  string(security.DecisionDeny),
		RiskScore: 1.0,
		Violations: []auditclient.Violation{
			{
				Type:     "dangerous_intent",
				Severity: "high",
				Message:  "dangerous task denied before tool execution",
				StepID:   "",
			},
		},
		EvidenceChain: []auditclient.EvidenceItem{},
		RiskGraph:     nil,
		Method:        "intent_guard",
		Message:       "dangerous task denied before tool execution",
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
	securityReport.AuditMetadata["orchestration"] = metadata.Orchestration
	securityReport.AuditMetadata["tool_protocol"] = metadata.ToolProtocol
	securityReport.AuditMetadata["tool_protocol_version"] = tools.ToolProtocolVersion
	securityReport.AuditMetadata["eino_runtime_version"] = metadata.Version
	securityReport.AuditMetadata["tools_used"] = append([]string{}, metadata.ToolsUsed...)
	if registry != nil {
		securityReport.AuditMetadata["registered_tool_count"] = len(registry.ListTools())
	}
}
