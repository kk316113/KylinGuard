package agentloop

import (
	"context"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

// AgentMode indicates the execution mode.
type AgentMode string

const (
	ModeDeterministic AgentMode = "deterministic"
	ModeAgentLoop     AgentMode = "agent_loop"
)

// TaskUnderstanding captures the LLM's interpretation of the user task.
type TaskUnderstanding struct {
	UserGoal   string `json:"user_goal"`
	IntentType string `json:"intent_type"`
	RiskLevel  string `json:"risk_level"`
}

// NextAction is the structured output from the LLM in each loop iteration.
type NextAction struct {
	ActionType         string         `json:"action_type"` // "tool_call" or "final_answer"
	ToolName           string         `json:"tool_name,omitempty"`
	ToolArgs           map[string]any `json:"tool_args,omitempty"`
	Reason             string         `json:"reason,omitempty"`
	UserVisibleSummary string         `json:"user_visible_summary,omitempty"`
	FinalAnswer        string         `json:"final_answer,omitempty"`
	Confidence         string         `json:"confidence,omitempty"`
	NextSuggestions    []string       `json:"next_suggestions,omitempty"`
}

// AgentStep records one tool execution step in the agent loop.
type AgentStep struct {
	StepIndex          int            `json:"step_index"`
	ActionType         string         `json:"action_type"`
	ToolName           string         `json:"tool_name"`
	ToolArgs           map[string]any `json:"tool_args"`
	Reason             string         `json:"reason"`
	UserVisibleSummary string         `json:"user_visible_summary"`
	PolicyDecision     string         `json:"policy_decision"`
	Observation        map[string]any `json:"observation"`
	OperationType      string         `json:"operation_type,omitempty"`
	ResourceType       string         `json:"resource_type,omitempty"`
	ResourcePath       string         `json:"resource_path,omitempty"`
	BoundaryLevel      string         `json:"boundary_level,omitempty"`
	AllowedByPolicy    bool           `json:"allowed_by_policy"`
	PolicyReason       string         `json:"policy_reason,omitempty"`
	// AuditReport is the per-tool-call audit result produced by the StepAuditor.
	AuditReport *AuditReport `json:"audit_report,omitempty"`
}

// AuditReport is the audit conclusion for a single tool_call. Each tool_call
// produces one AuditReport; all reports in a run aggregate into the RiskGraph.
type AuditReport struct {
	StepID     string   `json:"step_id,omitempty"`
	StepIndex  int      `json:"step_index"`
	ToolName   string   `json:"tool_name"`
	Decision   string   `json:"decision"` // allow|review|deny
	RiskScore  float64  `json:"risk_score"`
	Violations []string `json:"violations"`
	Evidence   []string `json:"evidence"`
	Method     string   `json:"method"` // traceshield|local-safety-fallback|tool_policy|intent_guard|no-audit
	Message    string   `json:"message,omitempty"`
}

// StepAuditor audits a single executed tool-call step and produces an AuditReport.
// Implementations call auditclient once per step; failures must not abort the loop.
type StepAuditor interface {
	AuditStep(ctx context.Context, task string, stepIndex int, trace logtrace.ToolTrace) (AuditReport, error)
}

// AgentResponse is the structured response from the agent loop.
type AgentResponse struct {
	AgentMode         AgentMode          `json:"agent_mode"`
	TaskUnderstanding *TaskUnderstanding `json:"task_understanding,omitempty"`
	AgentSteps        []AgentStep        `json:"agent_steps"`
	FinalAnswer       string             `json:"final_answer"`
	Confidence        string             `json:"confidence,omitempty"`
	NextSuggestions   []string           `json:"next_suggestions,omitempty"`
	FallbackReason    string             `json:"fallback_reason,omitempty"`
	StepCount         int                `json:"step_count"`
	// RiskGraph is the global risk graph aggregated from all per-step AuditReports.
	// nil when there were no steps (pure final_answer / intent-guard deny).
	RiskGraph *auditclient.RiskGraph `json:"risk_graph,omitempty"`
}

// NextActionRequest is the prompt context sent to the LLM.
type NextActionRequest struct {
	Task           string      `json:"task"`
	StepHistory    []AgentStep `json:"step_history,omitempty"`
	AvailableTools []ToolDef   `json:"available_tools"`
	MaxSteps       int         `json:"max_steps"`
}

// ToolDef is a safe subset of tool metadata for the LLM prompt.
type ToolDef struct {
	ToolName      string   `json:"tool_name"`
	Description   string   `json:"description"`
	Category      string   `json:"category,omitempty"`
	ArgKeys       []string `json:"arg_keys,omitempty"`
	OperationType string   `json:"operation_type"`
	ResourceType  string   `json:"resource_type"`
	BoundaryLevel string   `json:"boundary_level"`
	RiskLevel     string   `json:"risk_level"`
	UseCases      []string `json:"use_cases,omitempty"`
	SafetyNotes   []string `json:"safety_notes,omitempty"`
}
