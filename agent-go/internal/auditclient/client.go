package auditclient

import (
	"context"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

type Result struct {
	Decision      string   `json:"decision"`
	RiskScore     float64  `json:"risk_score"`
	Violations    []string `json:"violations"`
	EvidenceChain []string `json:"evidence_chain"`
}

type Client interface {
	AuditTrace(ctx context.Context, traces []logtrace.ToolTrace) (Result, error)
}

type MockClient struct{}

func NewMockClient() MockClient {
	return MockClient{}
}

func (MockClient) AuditTrace(ctx context.Context, traces []logtrace.ToolTrace) (Result, error) {
	_ = ctx
	_ = traces
	return Result{
		Decision:      "review",
		RiskScore:     0.35,
		Violations:    []string{},
		EvidenceChain: []string{},
	}, nil
}
