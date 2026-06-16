package report

import (
	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

const ReportVersion = "stage6-v1"

type SecurityReport struct {
	Title              string                  `json:"title"`
	Scenario           string                  `json:"scenario"`
	OverallDecision    string                  `json:"overall_decision"`
	RiskLevel          string                  `json:"risk_level"`
	Summary            string                  `json:"summary"`
	EvidenceChain      []EvidenceItem          `json:"evidence_chain"`
	RiskExplanation    []RiskExplanationItem   `json:"risk_explanation"`
	Recommendations    []RecommendationItem    `json:"recommendations"`
	SensitiveResources []SensitiveResourceItem `json:"sensitive_resources"`
	AuditMetadata      map[string]any          `json:"audit_metadata,omitempty"`
}

type EvidenceItem struct {
	EvidenceID    string `json:"evidence_id"`
	StepID        string `json:"step_id,omitempty"`
	ToolName      string `json:"tool_name"`
	OperationType string `json:"operation_type"`
	ResourceType  string `json:"resource_type"`
	ResourcePath  string `json:"resource_path,omitempty"`
	BoundaryLevel string `json:"boundary_level"`
	Status        string `json:"status"`
	Summary       string `json:"summary"`
	WhyRelevant   string `json:"why_relevant"`
	AuditMeaning  string `json:"audit_meaning"`
}

type RiskExplanationItem struct {
	ReasonID    string   `json:"reason_id"`
	Severity    string   `json:"severity"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type RecommendationItem struct {
	RecommendationID string `json:"recommendation_id"`
	Priority         string `json:"priority"`
	Action           string `json:"action"`
	Rationale        string `json:"rationale"`
	IsDestructive    bool   `json:"is_destructive"`
}

type SensitiveResourceItem struct {
	ResourceType    string `json:"resource_type"`
	ResourcePath    string `json:"resource_path,omitempty"`
	BoundaryLevel   string `json:"boundary_level"`
	AccessReason    string `json:"access_reason"`
	AllowedByPolicy bool   `json:"allowed_by_policy"`
}

type BuildInput struct {
	Task        string
	Decision    string
	Summary     string
	Plan        *Plan
	ToolTrace   []logtrace.ToolTrace
	Diagnosis   *Diagnosis
	AuditResult auditclient.Result
	Route       string
}

type Plan struct {
	Task     string
	Scenario string
	Summary  string
	Steps    []PlanStep
}

type PlanStep struct {
	StepID          string
	ToolName        string
	Input           map[string]any
	Reason          string
	ToolCategory    string
	RiskLevel       string
	PermissionScope string
}

type Diagnosis struct {
	Scenario        string
	RiskLevel       string
	Findings        []string
	Recommendations []string
	Details         map[string]any
}
