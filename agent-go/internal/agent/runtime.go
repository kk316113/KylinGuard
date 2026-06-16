package agent

import (
	"context"
	"errors"
	"strings"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/report"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type Runtime struct {
	registry *tools.Registry
	planner  Planner
	guard    security.IntentGuard
	auditor  auditclient.Client
	traces   *logtrace.Store
}

type RunRequest struct {
	Task string `json:"task"`
}

type RunResponse struct {
	Task           string                 `json:"task"`
	Decision       string                 `json:"decision"`
	Summary        string                 `json:"summary"`
	Plan           *Plan                  `json:"plan,omitempty"`
	Diagnosis      *Diagnosis             `json:"diagnosis,omitempty"`
	SecurityReport *report.SecurityReport `json:"security_report,omitempty"`
	ToolTrace      []logtrace.ToolTrace   `json:"tool_trace"`
	AuditResult    auditclient.Result     `json:"audit_result"`
}

type Diagnosis struct {
	Scenario        string         `json:"scenario"`
	RiskLevel       string         `json:"risk_level"`
	Findings        []string       `json:"findings"`
	Recommendations []string       `json:"recommendations"`
	Details         map[string]any `json:"details,omitempty"`
}

func NewRuntime(registry *tools.Registry, auditor auditclient.Client, traceStore *logtrace.Store) *Runtime {
	if registry == nil {
		registry = tools.NewDefaultRegistry()
	}
	if auditor == nil {
		auditor = auditclient.NewMockClient()
	}
	if traceStore == nil {
		traceStore = logtrace.NewStore()
	}

	return &Runtime{
		registry: registry,
		planner:  NewRuleBasedPlanner(),
		guard:    security.NewIntentGuard(),
		auditor:  auditor,
		traces:   traceStore,
	}
}

func (r *Runtime) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return RunResponse{}, errors.New("task is required")
	}

	intent := r.guard.Evaluate(task)
	if intent.Decision == security.DecisionDeny {
		audit := auditclient.Result{
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
		securityReport := report.BuildSecurityReport(report.BuildInput{
			Task:        task,
			Decision:    string(security.DecisionDeny),
			Summary:     "request denied by intent guard",
			ToolTrace:   []logtrace.ToolTrace{},
			AuditResult: audit,
			Route:       "stable",
		})
		return RunResponse{
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
		return RunResponse{}, err
	}

	traces := make([]logtrace.ToolTrace, 0, len(plan.Steps))
	var diagnosis *Diagnosis
	for _, step := range plan.Steps {
		result, _ := r.registry.InvokeWithStepID(ctx, step.StepID, step.ToolName, step.Input)
		if step.ToolName == "ssh_login_analyzer" {
			diagnosis = diagnosisFromSSHLoginAnalyzer(plan.Scenario, result.Output)
		}
		traces = append(traces, result.Trace)
		r.traces.Add(result.Trace)
	}

	audit, err := r.auditor.AuditTrace(ctx, task, traces)
	if err != nil {
		return RunResponse{}, err
	}
	if audit.Decision == "" {
		audit.Decision = string(intent.Decision)
	}

	securityReport := report.BuildSecurityReport(report.BuildInput{
		Task:        task,
		Decision:    audit.Decision,
		Summary:     "agent run completed",
		Plan:        reportPlanFromAgentPlan(&plan),
		ToolTrace:   traces,
		Diagnosis:   reportDiagnosisFromAgentDiagnosis(diagnosis),
		AuditResult: audit,
		Route:       "stable",
	})

	return RunResponse{
		Task:           task,
		Decision:       audit.Decision,
		Summary:        "agent run completed",
		Plan:           &plan,
		Diagnosis:      diagnosis,
		SecurityReport: securityReport,
		ToolTrace:      traces,
		AuditResult:    audit,
	}, nil
}

func reportPlanFromAgentPlan(plan *Plan) *report.Plan {
	if plan == nil {
		return nil
	}
	steps := make([]report.PlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, report.PlanStep{
			StepID:   step.StepID,
			ToolName: step.ToolName,
			Input:    step.Input,
			Reason:   step.Reason,
		})
	}
	return &report.Plan{
		Task:     plan.Task,
		Scenario: plan.Scenario,
		Summary:  plan.Summary,
		Steps:    steps,
	}
}

func reportDiagnosisFromAgentDiagnosis(diagnosis *Diagnosis) *report.Diagnosis {
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

func diagnosisFromSSHLoginAnalyzer(scenario string, output any) *Diagnosis {
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
	return &Diagnosis{
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
