package auditclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

type Result struct {
	Decision      string         `json:"decision"`
	RiskScore     float64        `json:"risk_score"`
	Violations    []Violation    `json:"violations"`
	EvidenceChain []EvidenceItem `json:"evidence_chain"`
	RiskGraph     RiskGraph      `json:"risk_graph"`
	Method        string         `json:"method"`
	Message       string         `json:"message"`
}

type Violation struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	StepID   any    `json:"step_id,omitempty"`
}

type EvidenceItem struct {
	StepID   any    `json:"step_id,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	Resource string `json:"resource,omitempty"`
	Reason   string `json:"reason"`
}

type RiskGraph struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

type Client interface {
	AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (Result, error)
}

type MockClient struct{}

func NewMockClient() MockClient {
	return MockClient{}
}

func (MockClient) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (Result, error) {
	_ = ctx
	_ = task
	_ = traces
	return Result{
		Decision:      "review",
		RiskScore:     0.35,
		Violations:    []Violation{},
		EvidenceChain: []EvidenceItem{},
		RiskGraph:     RiskGraph{Nodes: []map[string]any{}, Edges: []map[string]any{}},
		Method:        "fallback-mock",
		Message:       "audit-core-py unavailable, fallback mock used",
	}, nil
}

type HTTPClient struct {
	baseURL string
	client  *http.Client
	mock    MockClient
}

func NewHTTPClient(baseURL string) HTTPClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:8001"
	}
	return HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
		mock:    NewMockClient(),
	}
}

func (c HTTPClient) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (Result, error) {
	payload := auditTraceRequest{
		TaskID:   fmt.Sprintf("task-%d", time.Now().UTC().UnixNano()),
		UserGoal: task,
		Source:   "kylin-guard-agent",
		Steps:    make([]auditTraceStep, 0, len(traces)),
		Metadata: map[string]any{
			"os":     runtime.GOOS,
			"arch":   runtime.GOARCH,
			"agent":  "KylinGuard",
			"client": "agent-go",
		},
	}
	for _, trace := range traces {
		payload.Steps = append(payload.Steps, auditTraceStep{
			StepID:        trace.StepID,
			ToolName:      trace.ToolName,
			Input:         trace.Input,
			OutputSummary: trace.OutputSummary,
			Status:        normalizeStatus(trace.Status),
			StartedAt:     trace.StartedAt.Format(time.RFC3339Nano),
			FinishedAt:    trace.FinishedAt.Format(time.RFC3339Nano),
			RiskHint:      trace.RiskHint,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return c.fallback(ctx, task, traces, "failed to encode audit request: "+err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audit/trace", bytes.NewReader(body))
	if err != nil {
		return c.fallback(ctx, task, traces, "failed to build audit request: "+err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return c.fallback(ctx, task, traces, "audit-core-py unavailable, fallback mock used")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.fallback(ctx, task, traces, fmt.Sprintf("audit-core-py returned HTTP %d, fallback mock used", resp.StatusCode))
	}

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return c.fallback(ctx, task, traces, "failed to decode audit-core-py response: "+err.Error())
	}
	if result.Decision == "" {
		result.Decision = "review"
	}
	return result, nil
}

func (c HTTPClient) fallback(ctx context.Context, task string, traces []logtrace.ToolTrace, reason string) (Result, error) {
	result, err := c.mock.AuditTrace(ctx, task, traces)
	if reason != "" {
		result.Message = reason
	}
	return result, err
}

type auditTraceRequest struct {
	TaskID   string           `json:"task_id"`
	UserGoal string           `json:"user_goal"`
	Source   string           `json:"source"`
	Steps    []auditTraceStep `json:"steps"`
	Metadata map[string]any   `json:"metadata"`
}

type auditTraceStep struct {
	StepID        string    `json:"step_id"`
	ToolName      string    `json:"tool_name"`
	Input         any       `json:"input"`
	OutputSummary string    `json:"output_summary"`
	Status        string    `json:"status"`
	StartedAt     string    `json:"started_at"`
	FinishedAt    string    `json:"finished_at"`
	RiskHint      string    `json:"risk_hint"`
}

func normalizeStatus(status string) string {
	switch status {
	case "ok":
		return "success"
	case "error":
		return "error"
	default:
		return status
	}
}
