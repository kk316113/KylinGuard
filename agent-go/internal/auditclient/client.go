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
	RiskGraph     *RiskGraph     `json:"risk_graph"`
	Method        string         `json:"method"`
	Message       string         `json:"message"`
}

type Violation struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	StepID   any    `json:"step_id"`
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

type LocalSafetyClient struct{}

func NewLocalSafetyClient() LocalSafetyClient {
	return LocalSafetyClient{}
}

func (LocalSafetyClient) AuditTrace(ctx context.Context, task string, traces []logtrace.ToolTrace) (Result, error) {
	_ = ctx
	_ = task
	_ = traces
	return Result{
		Decision:      "review",
		RiskScore:     0.35,
		Violations:    []Violation{},
		EvidenceChain: []EvidenceItem{},
		RiskGraph:     riskGraphFromRealTraces(traces),
		Method:        "local-safety-fallback",
		Message:       "external audit unavailable; local safety review based on retained execution traces",
	}, nil
}

type HTTPClient struct {
	baseURL     string
	client      *http.Client
	localSafety LocalSafetyClient
}

func NewHTTPClient(baseURL string) HTTPClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:8001"
	}
	return HTTPClient{
		baseURL:     strings.TrimRight(baseURL, "/"),
		client:      &http.Client{Timeout: 5 * time.Second},
		localSafety: NewLocalSafetyClient(),
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
			StepID:            trace.StepID,
			ToolName:          trace.ToolName,
			Input:             trace.Input,
			OutputSummary:     trace.OutputSummary,
			Status:            normalizeStatus(trace.Status),
			StartedAt:         trace.StartedAt.Format(time.RFC3339Nano),
			FinishedAt:        trace.FinishedAt.Format(time.RFC3339Nano),
			RiskHint:          trace.RiskHint,
			OperationType:     trace.OperationType,
			ResourceType:      trace.ResourceType,
			ResourcePath:      trace.ResourcePath,
			PermissionScope:   trace.PermissionScope,
			BoundaryLevel:     trace.BoundaryLevel,
			ToolSemantic:      trace.ToolSemantic,
			RequiresPrivilege: trace.RequiresPrivilege,
			AllowedByPolicy:   trace.AllowedByPolicy,
			PolicyReason:      trace.PolicyReason,
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
		return c.fallback(ctx, task, traces, "audit-core-py unavailable; local safety review used")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.fallback(ctx, task, traces, fmt.Sprintf("audit-core-py returned HTTP %d; local safety review used", resp.StatusCode))
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
	result, err := c.localSafety.AuditTrace(ctx, task, traces)
	if reason != "" {
		result.Message = reason
	}
	return result, err
}

func riskGraphFromRealTraces(traces []logtrace.ToolTrace) *RiskGraph {
	nodes := make([]map[string]any, 0, len(traces))
	edges := make([]map[string]any, 0, len(traces)-1)
	for i, trace := range traces {
		nodes = append(nodes, map[string]any{
			"step_id": trace.StepID, "tool_name": trace.ToolName, "status": trace.Status,
			"operation_type": trace.OperationType, "resource_type": trace.ResourceType,
			"resource_path": trace.ResourcePath, "boundary_level": trace.BoundaryLevel,
			"allowed_by_policy": trace.AllowedByPolicy,
		})
		if i > 0 {
			edges = append(edges, map[string]any{"from": traces[i-1].StepID, "to": trace.StepID, "type": "sequence"})
		}
	}
	return &RiskGraph{Nodes: nodes, Edges: edges}
}

type auditTraceRequest struct {
	TaskID   string           `json:"task_id"`
	UserGoal string           `json:"user_goal"`
	Source   string           `json:"source"`
	Steps    []auditTraceStep `json:"steps"`
	Metadata map[string]any   `json:"metadata"`
}

type auditTraceStep struct {
	StepID            string `json:"step_id"`
	ToolName          string `json:"tool_name"`
	Input             any    `json:"input"`
	OutputSummary     string `json:"output_summary"`
	Status            string `json:"status"`
	StartedAt         string `json:"started_at"`
	FinishedAt        string `json:"finished_at"`
	RiskHint          string `json:"risk_hint"`
	OperationType     string `json:"operation_type"`
	ResourceType      string `json:"resource_type"`
	ResourcePath      string `json:"resource_path"`
	PermissionScope   string `json:"permission_scope"`
	BoundaryLevel     string `json:"boundary_level"`
	ToolSemantic      string `json:"tool_semantic"`
	RequiresPrivilege bool   `json:"requires_privilege"`
	AllowedByPolicy   bool   `json:"allowed_by_policy"`
	PolicyReason      string `json:"policy_reason"`
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
