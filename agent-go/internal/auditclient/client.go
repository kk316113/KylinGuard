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
	result := localSafetyResultFromTraces(traces)
	result.Method = "local-safety-fallback"
	result.Message = "external audit unavailable; local safety review based on retained execution traces"
	return result, nil
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
	result = enrichResultWithTraceEvidence(result, traces)
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
	edgeCapacity := 0
	if len(traces) > 1 {
		edgeCapacity = len(traces) - 1
	}
	edges := make([]map[string]any, 0, edgeCapacity)
	for i, trace := range traces {
		nodes = append(nodes, map[string]any{
			"id":             trace.StepID,
			"step_id":        trace.StepID,
			"type":           "tool_call",
			"label":          trace.ToolName,
			"tool_name":      trace.ToolName,
			"status":         trace.Status,
			"operation_type": trace.OperationType, "resource_type": trace.ResourceType,
			"resource_path": trace.ResourcePath, "boundary_level": trace.BoundaryLevel,
			"allowed_by_policy":  trace.AllowedByPolicy,
			"policy_reason":      trace.PolicyReason,
			"requires_privilege": trace.RequiresPrivilege,
			"risk_hint":          trace.RiskHint,
			"risk_level":         traceRiskLevel(trace),
		})
		if i > 0 {
			edges = append(edges, map[string]any{
				"from":   traces[i-1].StepID,
				"to":     trace.StepID,
				"source": traces[i-1].StepID,
				"target": trace.StepID,
				"type":   "sequence",
				"label":  "sequence",
			})
		}
	}
	return &RiskGraph{Nodes: nodes, Edges: edges}
}

func localSafetyResultFromTraces(traces []logtrace.ToolTrace) Result {
	decision := "review"
	riskScore := 0.35
	violations := make([]Violation, 0)

	for _, trace := range traces {
		switch {
		case isDeniedTrace(trace):
			decision = "deny"
			riskScore = maxRisk(riskScore, 1.0)
			violations = append(violations, Violation{
				Type:     "tool_policy",
				Severity: "high",
				Message:  "tool call was denied or marked unsafe by local policy context",
				StepID:   trace.StepID,
			})
		case isDangerousTrace(trace):
			decision = "deny"
			riskScore = maxRisk(riskScore, 0.95)
			violations = append(violations, Violation{
				Type:     "dangerous_boundary",
				Severity: "high",
				Message:  "tool trace reached a dangerous or privileged boundary",
				StepID:   trace.StepID,
			})
		case isErrorTrace(trace):
			riskScore = maxRisk(riskScore, 0.6)
			violations = append(violations, Violation{
				Type:     "tool_execution_error",
				Severity: "medium",
				Message:  "tool execution returned an error and needs review",
				StepID:   trace.StepID,
			})
		case isSensitiveTrace(trace):
			riskScore = maxRisk(riskScore, 0.55)
		default:
			riskScore = maxRisk(riskScore, 0.25)
		}
	}

	result := Result{
		Decision:      decision,
		RiskScore:     riskScore,
		Violations:    violations,
		EvidenceChain: traceEvidence(traces),
		RiskGraph:     riskGraphFromRealTraces(traces),
	}
	if result.Violations == nil {
		result.Violations = []Violation{}
	}
	if result.EvidenceChain == nil {
		result.EvidenceChain = []EvidenceItem{}
	}
	return result
}

func enrichResultWithTraceEvidence(result Result, traces []logtrace.ToolTrace) Result {
	if result.Violations == nil {
		result.Violations = []Violation{}
	}
	if len(result.EvidenceChain) == 0 {
		result.EvidenceChain = traceEvidence(traces)
	}
	if result.RiskGraph == nil || len(result.RiskGraph.Nodes) == 0 {
		result.RiskGraph = riskGraphFromRealTraces(traces)
	}
	if result.RiskScore == 0 {
		result.RiskScore = riskScoreFromTraces(traces)
	}
	return result
}

func traceEvidence(traces []logtrace.ToolTrace) []EvidenceItem {
	evidence := make([]EvidenceItem, 0, len(traces))
	for _, trace := range traces {
		reason := strings.TrimSpace(trace.OutputSummary)
		if reason == "" {
			reason = fmt.Sprintf("%s %s on %s", trace.OperationType, trace.ToolName, trace.ResourceType)
		}
		evidence = append(evidence, EvidenceItem{
			StepID:   trace.StepID,
			ToolName: trace.ToolName,
			Resource: trace.ResourcePath,
			Reason:   reason,
		})
	}
	return evidence
}

func riskScoreFromTraces(traces []logtrace.ToolTrace) float64 {
	score := 0.2
	if len(traces) == 0 {
		return 0.0
	}
	for _, trace := range traces {
		switch {
		case isDeniedTrace(trace):
			score = maxRisk(score, 1.0)
		case isDangerousTrace(trace):
			score = maxRisk(score, 0.95)
		case isErrorTrace(trace):
			score = maxRisk(score, 0.6)
		case isSensitiveTrace(trace):
			score = maxRisk(score, 0.55)
		default:
			score = maxRisk(score, 0.25)
		}
	}
	return score
}

func traceRiskLevel(trace logtrace.ToolTrace) string {
	switch {
	case isDeniedTrace(trace) || isDangerousTrace(trace):
		return "high"
	case isErrorTrace(trace) || isSensitiveTrace(trace):
		return "medium"
	default:
		return "low"
	}
}

func isDeniedTrace(trace logtrace.ToolTrace) bool {
	return trace.Status == "denied" || (!trace.AllowedByPolicy && strings.TrimSpace(trace.PolicyReason) != "")
}

func isDangerousTrace(trace logtrace.ToolTrace) bool {
	switch trace.BoundaryLevel {
	case "dangerous", "privileged", "high":
		return true
	default:
		return false
	}
}

func isErrorTrace(trace logtrace.ToolTrace) bool {
	return trace.Status == "error"
}

func isSensitiveTrace(trace logtrace.ToolTrace) bool {
	if trace.RequiresPrivilege {
		return true
	}
	switch trace.BoundaryLevel {
	case "sensitive_system_resource":
		return true
	default:
		return false
	}
}

func maxRisk(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
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
