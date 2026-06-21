package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type toolsListResponse struct {
	Protocol string               `json:"protocol"`
	Version  string               `json:"version"`
	Count    int                  `json:"count"`
	Tools    []tools.ToolMetadata `json:"tools"`
}

type toolCallRequest struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
	Reason   string         `json:"reason"`
}

type toolCallResponse struct {
	ToolName    string              `json:"tool_name"`
	Status      string              `json:"status"`
	Output      map[string]any      `json:"output,omitempty"`
	Trace       *logtrace.ToolTrace `json:"trace,omitempty"`
	AuditResult auditclient.Result  `json:"audit_result,omitempty"`
	Message     string              `json:"message,omitempty"`
	Protocol    string              `json:"protocol"`
	Version     string              `json:"version"`
}

func toolsListHandler(registry *tools.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}

		registered := registry.ListTools()
		writeJSON(w, http.StatusOK, toolsListResponse{
			Protocol: tools.ToolProtocol,
			Version:  tools.ToolProtocolVersion,
			Count:    len(registered),
			Tools:    registered,
		})
	}
}

func toolDetailHandler(registry *tools.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}

		name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tools/"), "/")
		if name == "" || strings.Contains(name, "/") {
			writeAPIError(w, http.StatusNotFound, "TOOL_NOT_FOUND", "tool not found", map[string]any{"tool_name": name})
			return
		}

		metadata, ok := registry.GetTool(name)
		if !ok {
			writeAPIError(w, http.StatusNotFound, "TOOL_NOT_FOUND", "tool not found", map[string]any{"tool_name": name})
			return
		}

		writeJSON(w, http.StatusOK, metadata)
	}
}

func toolCallHandler(registry *tools.Registry, auditor auditclient.Client, traceStore *logtrace.Store, policy security.ToolPolicy) http.HandlerFunc {
	if auditor == nil {
		auditor = auditclient.NewLocalSafetyClient()
	}
	if traceStore == nil {
		traceStore = logtrace.NewStore()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}

		var req toolCallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ToolName = strings.TrimSpace(req.ToolName)
		if req.Input == nil {
			req.Input = map[string]any{}
		}

		metadata, exists := registry.GetTool(req.ToolName)
		decision := policy.Evaluate(req.ToolName, metadata, exists, req.Input)
		if decision.Decision == "deny" {
			writeJSON(w, http.StatusOK, toolCallResponse{
				ToolName:    req.ToolName,
				Status:      "denied",
				AuditResult: toolPolicyAuditResult(decision),
				Message:     decisionMessage(decision),
				Protocol:    tools.ToolProtocol,
				Version:     tools.ToolProtocolVersion,
			})
			return
		}

		result, toolErr := registry.CallTool(r.Context(), req.ToolName, req.Input)
		traceStore.Add(result.Trace)
		audit, auditErr := auditor.AuditTrace(r.Context(), syntheticToolTask(req.ToolName), []logtrace.ToolTrace{result.Trace})
		// Normalize TraceShield denial for read-only OS sensing tools.
		if auditErr == nil {
			audit.Decision = normalizeToolCallDecision(audit.Decision, audit.Method, result.Trace)
		}
		status := "ok"
		message := "tool call audited"
		if toolErr != nil {
			status = "error"
			message = toolErr.Error()
		}
		if auditErr != nil {
			status = "error"
			message = appendMessage(message, "audit failed: "+auditErr.Error())
		}

		output := mapOutput(result.Output)
		writeJSON(w, http.StatusOK, toolCallResponse{
			ToolName:    req.ToolName,
			Status:      status,
			Output:      output,
			Trace:       &result.Trace,
			AuditResult: audit,
			Message:     message,
			Protocol:    tools.ToolProtocol,
			Version:     tools.ToolProtocolVersion,
		})
	}
}

func syntheticToolTask(toolName string) string {
	if strings.TrimSpace(toolName) == "" {
		return "Manual MCP-like tool call"
	}
	return "Manual MCP-like tool call: " + toolName
}

func toolPolicyAuditResult(decision security.ToolPolicyDecision) auditclient.Result {
	return auditclient.Result{
		Decision:  "deny",
		RiskScore: 1.0,
		Violations: []auditclient.Violation{
			{
				Type:     "tool_policy",
				Severity: "high",
				Message:  "tool call denied by tool policy",
				StepID:   "",
			},
		},
		EvidenceChain: []auditclient.EvidenceItem{},
		RiskGraph:     nil,
		Method:        decision.Method,
		Message:       decision.Message,
	}
}

func decisionMessage(decision security.ToolPolicyDecision) string {
	if decision.Reason == "" {
		return decision.Message
	}
	return decision.Message + ": " + decision.Reason
}

func appendMessage(base string, detail string) string {
	if strings.TrimSpace(base) == "" {
		return detail
	}
	if strings.TrimSpace(detail) == "" {
		return base
	}
	return base + "; " + detail
}

func mapOutput(output any) map[string]any {
	if output == nil {
		return nil
	}
	if typed, ok := output.(map[string]any); ok {
		return typed
	}

	body, err := json.Marshal(output)
	if err != nil {
		return map[string]any{"value": output}
	}
	var mapped map[string]any
	if err := json.Unmarshal(body, &mapped); err != nil {
		return map[string]any{"value": output}
	}
	return mapped
}

func normalizeToolCallDecision(auditDecision string, auditMethod string, trace logtrace.ToolTrace) string {
	if auditMethod == "intent_guard" || auditMethod == "tool_policy" {
		return "deny"
	}
	if auditMethod != "traceshield" {
		return auditDecision
	}
	if auditDecision != "deny" {
		return auditDecision
	}
	// TraceShield returned "deny" — check if this is a read-only safe tool.
	if trace.AllowedByPolicy && trace.OperationType != "execute" && trace.BoundaryLevel != "dangerous" && trace.BoundaryLevel != "privileged" {
		if trace.BoundaryLevel == "sensitive_system_resource" || trace.RequiresPrivilege {
			return "review"
		}
		return "allow"
	}
	return auditDecision
}

func auditToolCall(ctx context.Context, auditor auditclient.Client, toolName string, trace logtrace.ToolTrace) (auditclient.Result, error) {
	return auditor.AuditTrace(ctx, syntheticToolTask(toolName), []logtrace.ToolTrace{trace})
}
