package execproxy

import (
	"os/user"
	"time"
)

// ExecutionProfile classifies the privilege level of a tool execution.
type ExecutionProfile string

const (
	ProfilePublicRead     ExecutionProfile = "public_read"
	ProfileLowRead        ExecutionProfile = "low_read"
	ProfileSensitiveRead  ExecutionProfile = "sensitive_read"
	ProfilePrivilegedRead ExecutionProfile = "privileged_read"
	ProfileDenied         ExecutionProfile = "denied"
)

// ExecutionContext captures the execution environment metadata for audit trails.
type ExecutionContext struct {
	Executor            string `json:"executor"`
	Profile             string `json:"profile"`
	CommandName         string `json:"command_name"`
	ArgsCount           int    `json:"args_count"`
	TimeoutMs           int    `json:"timeout_ms"`
	MaxOutputBytes      int    `json:"max_output_bytes"`
	ShellUsed           bool   `json:"shell_used"`
	SudoUsed            bool   `json:"sudo_used"`
	AllowedByExecPolicy bool   `json:"allowed_by_exec_policy"`
	PolicyReason        string `json:"policy_reason"`
	EffectiveUser       string `json:"effective_user,omitempty"`
	Platform            string `json:"platform"`
	OutputTruncated     bool   `json:"output_truncated"`
}

// ExecutionResult holds the outcome of a proxied command execution.
type ExecutionResult struct {
	Status     string           `json:"status"`
	ExitCode   int              `json:"exit_code"`
	Stdout     string           `json:"stdout"`
	Stderr     string           `json:"stderr"`
	TimedOut   bool             `json:"timed_out"`
	Truncated  bool             `json:"truncated"`
	Duration   time.Duration    `json:"-"` // internal; not serialized
	DurationMs int              `json:"duration_ms"`
	Context    ExecutionContext `json:"execution_context"`
}

// NativeExecutionContext produces an execution context for tools that use
// native Go APIs (procfs, net.Dial, os.Open) rather than external commands.
func NativeExecutionContext(profile ExecutionProfile, method string, reason string) ExecutionContext {
	return ExecutionContext{
		Executor:            "least_privilege_proxy",
		Profile:             string(profile),
		CommandName:         method,
		ArgsCount:           0,
		TimeoutMs:           0,
		MaxOutputBytes:      0,
		ShellUsed:           false,
		SudoUsed:            false,
		AllowedByExecPolicy: true,
		PolicyReason:        reason,
		EffectiveUser:       effectiveUser(),
		Platform:            platform(),
		OutputTruncated:     false,
	}
}

func effectiveUser() string {
	current, err := user.Current()
	if err != nil || current == nil {
		return "unknown"
	}
	if current.Uid != "" {
		return current.Username + " (uid=" + current.Uid + ")"
	}
	return current.Username
}

// DeniedContext produces an execution context for exec-policy-denied requests.
func DeniedContext(command string, reason string) ExecutionContext {
	return ExecutionContext{
		Executor:            "least_privilege_proxy",
		Profile:             string(ProfileDenied),
		CommandName:         command,
		ArgsCount:           0,
		TimeoutMs:           0,
		MaxOutputBytes:      0,
		ShellUsed:           false,
		SudoUsed:            false,
		AllowedByExecPolicy: false,
		PolicyReason:        reason,
		Platform:            platform(),
		OutputTruncated:     false,
	}
}
