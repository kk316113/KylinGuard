package agent

import (
	"context"
	"errors"
	"strings"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
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
	Task        string               `json:"task"`
	Decision    string               `json:"decision"`
	Summary     string               `json:"summary"`
	Plan        *Plan                `json:"plan,omitempty"`
	ToolTrace   []logtrace.ToolTrace `json:"tool_trace"`
	AuditResult auditclient.Result   `json:"audit_result"`
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
		return RunResponse{
			Task:        task,
			Decision:    string(security.DecisionDeny),
			Summary:     "request denied by intent guard",
			ToolTrace:   []logtrace.ToolTrace{},
			AuditResult: audit,
		}, nil
	}

	plan, err := r.planner.Plan(ctx, task)
	if err != nil {
		return RunResponse{}, err
	}

	traces := make([]logtrace.ToolTrace, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		result, _ := r.registry.InvokeWithStepID(ctx, step.StepID, step.ToolName, step.Input)
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

	return RunResponse{
		Task:        task,
		Decision:    audit.Decision,
		Summary:     "agent run completed",
		Plan:        &plan,
		ToolTrace:   traces,
		AuditResult: audit,
	}, nil
}
