package eino

import (
	"context"

	"kylin-guard-agent/agent-go/internal/agentloop"
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

// auditClientStepAuditor implements agentloop.StepAuditor by calling auditclient
// once per executed tool-call step. When audit-core-py is unreachable, auditclient's
// HTTPClient falls back to a trace-backed local safety review, so the loop is never
// blocked by an audit outage.
type auditClientStepAuditor struct {
	client auditclient.Client
}

// NewAuditClientStepAuditor wraps an auditclient.Client as a per-step StepAuditor.
func NewAuditClientStepAuditor(c auditclient.Client) agentloop.StepAuditor {
	if c == nil {
		c = auditclient.NewLocalSafetyClient()
	}
	return &auditClientStepAuditor{client: c}
}

func (a *auditClientStepAuditor) AuditStep(ctx context.Context, task string, stepIndex int, trace logtrace.ToolTrace) (agentloop.AuditReport, error) {
	// Single-trace slice = audit exactly this one tool_call (req 7: one audit_report per tool_call).
	res, err := a.client.AuditTrace(ctx, task, []logtrace.ToolTrace{trace})
	if err != nil {
		return agentloop.AuditReport{}, err
	}

	violations := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		msg := v.Message
		if v.Severity != "" {
			msg = v.Severity + ": " + v.Message
		}
		violations = append(violations, msg)
	}
	evidence := make([]string, 0, len(res.EvidenceChain))
	for _, ev := range res.EvidenceChain {
		evidence = append(evidence, ev.Reason)
	}

	return agentloop.AuditReport{
		StepID:     trace.StepID,
		StepIndex:  stepIndex,
		ToolName:   trace.ToolName,
		Decision:   res.Decision,
		RiskScore:  res.RiskScore,
		Violations: violations,
		Evidence:   evidence,
		Method:     res.Method,
		Message:    res.Message,
	}, nil
}
