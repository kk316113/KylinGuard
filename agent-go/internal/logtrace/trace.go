package logtrace

import (
	"fmt"
	"sync/atomic"
	"time"
)

var stepSequence uint64

type ToolTrace struct {
	StepID            string    `json:"step_id"`
	ToolName          string    `json:"tool_name"`
	Input             any       `json:"input"`
	OutputSummary     string    `json:"output_summary"`
	Status            string    `json:"status"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	RiskHint          string    `json:"risk_hint"`
	OperationType     string    `json:"operation_type"`
	ResourceType      string    `json:"resource_type"`
	ResourcePath      string    `json:"resource_path"`
	PermissionScope   string    `json:"permission_scope"`
	BoundaryLevel     string    `json:"boundary_level"`
	ToolSemantic      string    `json:"tool_semantic"`
	RequiresPrivilege bool      `json:"requires_privilege"`
	AllowedByPolicy   bool      `json:"allowed_by_policy"`
	PolicyReason      string    `json:"policy_reason"`
}

func NextStepID() string {
	next := atomic.AddUint64(&stepSequence, 1)
	return fmt.Sprintf("step-%06d", next)
}
