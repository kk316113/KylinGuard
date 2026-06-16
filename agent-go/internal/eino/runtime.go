package eino

import (
	"context"
	"errors"
	"strings"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/report"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type Runtime struct {
	config      RuntimeConfig
	registry    *tools.Registry
	planner     agent.Planner
	guard       security.IntentGuard
	auditor     auditclient.Client
	traces      *logtrace.Store
	toolAdapter PlanToolAdapter
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
	return &Runtime{
		config:      normalized,
		registry:    registry,
		planner:     agent.NewRuleBasedPlannerWithRegistry(registry),
		guard:       security.NewIntentGuard(),
		auditor:     auditor,
		traces:      traceStore,
		toolAdapter: NewMCPToolAdapter(registry, security.NewToolPolicy()),
	}
}

func (r *Runtime) Run(ctx context.Context, req agent.AgentRunRequest) (agent.AgentRunResponse, error) {
	if !r.Enabled() {
		return agent.AgentRunResponse{}, errors.New("eino runtime skeleton is disabled")
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

	plan, err := r.planner.Plan(ctx, task)
	if err != nil {
		return agent.AgentRunResponse{}, err
	}

	traces := make([]logtrace.ToolTrace, 0, len(plan.Steps))
	var diagnosis *agent.Diagnosis
	for _, step := range plan.Steps {
		result, _ := r.toolAdapter.Execute(ctx, step)
		if step.ToolName == "ssh_login_analyzer" {
			diagnosis = diagnosisFromSSHLoginAnalyzer(plan.Scenario, result.Output)
		}
		traces = append(traces, result.Trace)
		r.traces.Add(result.Trace)
	}

	audit, err := r.auditor.AuditTrace(ctx, task, traces)
	if err != nil {
		return agent.AgentRunResponse{}, err
	}
	if audit.Decision == "" {
		audit.Decision = string(intent.Decision)
	}

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
	return "eino_runtime_skeleton"
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
	securityReport.AuditMetadata["llm_enabled"] = metadata.LLMEnabled
	securityReport.AuditMetadata["orchestration"] = metadata.Orchestration
	securityReport.AuditMetadata["tool_protocol"] = metadata.ToolProtocol
	securityReport.AuditMetadata["tool_protocol_version"] = tools.ToolProtocolVersion
	securityReport.AuditMetadata["eino_runtime_version"] = metadata.Version
	securityReport.AuditMetadata["tools_used"] = append([]string{}, metadata.ToolsUsed...)
	if registry != nil {
		securityReport.AuditMetadata["registered_tool_count"] = len(registry.ListTools())
	}
}
