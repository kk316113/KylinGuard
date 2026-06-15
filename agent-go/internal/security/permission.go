package security

type PermissionScope string

const (
	PermissionObserve PermissionScope = "observe"
	PermissionReadOnly PermissionScope = "read_only"
	PermissionExecute  PermissionScope = "execute"
)

type PermissionRequest struct {
	ToolName string          `json:"tool_name"`
	Scope    PermissionScope `json:"scope"`
}

type PermissionDecision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

func EvaluatePermission(req PermissionRequest) PermissionDecision {
	if req.Scope == PermissionObserve || req.Scope == PermissionReadOnly {
		return PermissionDecision{Allowed: true, Reason: "read-only operation allowed in Stage 0"}
	}
	return PermissionDecision{Allowed: false, Reason: "execute permission requires explicit future policy"}
}
