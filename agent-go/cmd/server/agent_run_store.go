package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/auditclient"
)

const maxStoredAgentRuns = 32

type agentRunStore struct {
	mu     sync.RWMutex
	runs   map[string]agent.AgentRunResponse
	order  []string
	latest string
}

func newAgentRunStore() *agentRunStore {
	return &agentRunStore{
		runs: make(map[string]agent.AgentRunResponse),
	}
}

func (s *agentRunStore) Save(resp agent.AgentRunResponse) agent.AgentRunResponse {
	if s == nil {
		return resp
	}
	if resp.RunID == "" || resp.TaskID == "" || resp.CreatedAt == "" {
		status := resp.RunStatus
		if status == "" {
			status = statusFromDecision(resp.Decision)
		}
		agent.AttachScenarioWorkspaceMetadata(&resp, resp.Task, status)
	}
	if resp.RunID == "" {
		resp.RunID = resp.TaskID
	}
	if resp.TaskID == "" {
		resp.TaskID = resp.RunID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[resp.RunID]; !exists {
		s.order = append(s.order, resp.RunID)
	}
	s.runs[resp.RunID] = resp
	s.latest = resp.RunID
	for len(s.order) > maxStoredAgentRuns {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.runs, oldest)
	}
	return resp
}

func (s *agentRunStore) Get(runID string) (agent.AgentRunResponse, bool) {
	if s == nil {
		return agent.AgentRunResponse{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if runID == "latest" {
		runID = s.latest
	}
	resp, ok := s.runs[runID]
	return resp, ok
}

func statusFromDecision(decision string) string {
	switch decision {
	case "deny":
		return agent.RunStatusBlocked
	default:
		return agent.RunStatusCompleted
	}
}

type auditReportsResponse struct {
	RunID        string             `json:"run_id"`
	AuditReports []map[string]any   `json:"audit_reports"`
	AuditResult  auditclient.Result `json:"audit_result"`
}

type riskGraphResponse struct {
	RunID     string                 `json:"run_id"`
	RiskGraph *auditclient.RiskGraph `json:"risk_graph"`
}

type runReportResponse struct {
	RunID       string         `json:"run_id"`
	TaskID      string         `json:"task_id"`
	Task        string         `json:"task"`
	SceneType   string         `json:"scene_type"`
	RunStatus   string         `json:"run_status"`
	CreatedAt   string         `json:"created_at"`
	Decision    string         `json:"decision"`
	Summary     string         `json:"summary"`
	FinalAnswer string         `json:"final_answer"`
	Counts      map[string]int `json:"counts"`
}

func agentRunsHandler(store *agentRunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}

		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/agent/runs/"), "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "run_id is required")
			return
		}
		parts := strings.Split(path, "/")
		runID := parts[0]
		resp, ok := store.Get(runID)
		if !ok {
			writeError(w, http.StatusNotFound, "agent run not found")
			return
		}
		if len(parts) == 1 {
			writeJSON(w, http.StatusOK, resp)
			return
		}
		if len(parts) > 2 {
			writeError(w, http.StatusNotFound, "agent run resource not found")
			return
		}

		switch parts[1] {
		case "audit-reports":
			writeJSON(w, http.StatusOK, buildAuditReportsResponse(resp))
		case "risk-graph":
			writeJSON(w, http.StatusOK, riskGraphResponse{
				RunID:     resp.RunID,
				RiskGraph: riskGraphFromResponse(resp),
			})
		case "report":
			writeJSON(w, http.StatusOK, buildRunReportResponse(resp))
		default:
			writeError(w, http.StatusNotFound, "agent run resource not found")
		}
	}
}

func buildAuditReportsResponse(resp agent.AgentRunResponse) auditReportsResponse {
	reports := make([]map[string]any, 0, len(resp.AgentSteps)+1)
	for index, step := range resp.AgentSteps {
		if raw, ok := step["audit_report"]; ok {
			if reportMap, ok := raw.(map[string]any); ok {
				reports = append(reports, reportMap)
				continue
			}
		}
		if toolName, ok := step["tool_name"].(string); ok && toolName != "" {
			reports = append(reports, map[string]any{
				"audit_id":   "audit-step-" + strconv.Itoa(index+1),
				"run_id":     resp.RunID,
				"step_index": index + 1,
				"tool_name":  toolName,
				"decision":   step["policy_decision"],
				"message":    step["user_visible_summary"],
			})
		}
	}
	if len(reports) == 0 && resp.AuditResult.Method != "" {
		reports = append(reports, map[string]any{
			"audit_id":       "audit-aggregate",
			"run_id":         resp.RunID,
			"step_index":     0,
			"tool_name":      "agent_run",
			"decision":       resp.AuditResult.Decision,
			"risk_score":     resp.AuditResult.RiskScore,
			"violations":     resp.AuditResult.Violations,
			"evidence_chain": resp.AuditResult.EvidenceChain,
			"method":         resp.AuditResult.Method,
			"message":        resp.AuditResult.Message,
		})
	}
	return auditReportsResponse{
		RunID:        resp.RunID,
		AuditReports: reports,
		AuditResult:  resp.AuditResult,
	}
}

func riskGraphFromResponse(resp agent.AgentRunResponse) *auditclient.RiskGraph {
	if resp.RiskGraph != nil {
		return resp.RiskGraph
	}
	if resp.AuditResult.RiskGraph != nil {
		return resp.AuditResult.RiskGraph
	}
	return &auditclient.RiskGraph{Nodes: []map[string]any{}, Edges: []map[string]any{}}
}

func buildRunReportResponse(resp agent.AgentRunResponse) runReportResponse {
	return runReportResponse{
		RunID:       resp.RunID,
		TaskID:      resp.TaskID,
		Task:        resp.Task,
		SceneType:   resp.SceneType,
		RunStatus:   resp.RunStatus,
		CreatedAt:   resp.CreatedAt,
		Decision:    resp.Decision,
		Summary:     resp.Summary,
		FinalAnswer: resp.FinalAnswer,
		Counts: map[string]int{
			"agent_steps": len(resp.AgentSteps),
			"tool_trace":  len(resp.ToolTrace),
			"evidence":    len(resp.AuditResult.EvidenceChain),
			"violations":  len(resp.AuditResult.Violations),
		},
	}
}
