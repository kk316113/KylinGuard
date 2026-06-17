package agentloop

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
	ActionType        string            `json:"action_type"` // "tool_call" or "final_answer"
	ToolName          string            `json:"tool_name,omitempty"`
	ToolArgs          map[string]any    `json:"tool_args,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	UserVisibleSummary string           `json:"user_visible_summary,omitempty"`
	FinalAnswer       string            `json:"final_answer,omitempty"`
	Confidence        string            `json:"confidence,omitempty"`
	NextSuggestions   []string          `json:"next_suggestions,omitempty"`
}

// AgentStep records one tool execution step in the agent loop.
type AgentStep struct {
	StepIndex          int               `json:"step_index"`
	ActionType         string            `json:"action_type"`
	ToolName           string            `json:"tool_name"`
	ToolArgs           map[string]any    `json:"tool_args"`
	Reason             string            `json:"reason"`
	UserVisibleSummary string            `json:"user_visible_summary"`
	PolicyDecision     string            `json:"policy_decision"`
	Observation        map[string]any    `json:"observation"`
	OperationType      string            `json:"operation_type,omitempty"`
	ResourceType       string            `json:"resource_type,omitempty"`
	ResourcePath       string            `json:"resource_path,omitempty"`
	BoundaryLevel      string            `json:"boundary_level,omitempty"`
	AllowedByPolicy    bool              `json:"allowed_by_policy"`
	PolicyReason       string            `json:"policy_reason,omitempty"`
}

// AgentResponse is the structured response from the agent loop.
type AgentResponse struct {
	AgentMode        AgentMode            `json:"agent_mode"`
	TaskUnderstanding *TaskUnderstanding  `json:"task_understanding,omitempty"`
	AgentSteps       []AgentStep          `json:"agent_steps"`
	FinalAnswer      string              `json:"final_answer"`
	Confidence       string              `json:"confidence,omitempty"`
	NextSuggestions  []string            `json:"next_suggestions,omitempty"`
	FallbackReason   string              `json:"fallback_reason,omitempty"`
	StepCount        int                 `json:"step_count"`
}

// NextActionRequest is the prompt context sent to the LLM.
type NextActionRequest struct {
	Task        string       `json:"task"`
	StepHistory []AgentStep  `json:"step_history,omitempty"`
	AvailableTools []ToolDef `json:"available_tools"`
	MaxSteps    int          `json:"max_steps"`
}

// ToolDef is a safe subset of tool metadata for the LLM prompt.
type ToolDef struct {
	ToolName      string   `json:"tool_name"`
	Description   string   `json:"description"`
	ArgKeys       []string `json:"arg_keys,omitempty"`
	OperationType string   `json:"operation_type"`
	ResourceType  string   `json:"resource_type"`
	BoundaryLevel string   `json:"boundary_level"`
	RiskLevel     string   `json:"risk_level"`
}
