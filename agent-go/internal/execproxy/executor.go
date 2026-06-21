package execproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

const (
	// DefaultTimeout is the default execution timeout for proxied commands.
	DefaultTimeout = 3 * time.Second

	// DefaultMaxOutputBytes is the default maximum output size (64 KB).
	DefaultMaxOutputBytes = 64 * 1024

	// SensitiveMaxOutputBytes is the output limit for sensitive operations (128 KB).
	SensitiveMaxOutputBytes = 128 * 1024
)

// CommandSpec describes a proxied command execution request.
type CommandSpec struct {
	ToolName         string
	Profile          ExecutionProfile
	Command          string
	Args             []string
	Timeout          time.Duration
	MaxOutputBytes   int
	AllowNonZeroExit bool
	SensitiveOutput  bool
	Reason           string
}

// CommandRunner is the signature of a function that executes a command.
// It returns stdout, stderr, and any error.
type CommandRunner func(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, err error)

// Executor is the least-privilege execution proxy that validates and runs system commands.
type Executor struct {
	Policy ExecPolicy
	runner CommandRunner // injectable for testing; nil means use real exec.CommandContext
}

// NewExecutor creates a new Executor with the default execution policy.
func NewExecutor() *Executor {
	return &Executor{
		Policy: NewExecPolicy(),
	}
}

// WithRunner returns a copy of the Executor with the given command runner injected.
// This is intended for testing; production code should use the default (nil runner).
func (e *Executor) WithRunner(runner CommandRunner) *Executor {
	return &Executor{
		Policy: e.Policy,
		runner: runner,
	}
}

// Execute validates the command through the execution policy, runs it if allowed,
// and returns a result with full execution context for audit trailing.
func (e *Executor) Execute(ctx context.Context, spec CommandSpec) (ExecutionResult, error) {
	// Validate through execution policy.
	decision := e.Policy.Evaluate(spec.Command, spec.Args, spec.Profile)
	if !decision.Allowed {
		return ExecutionResult{
			Status:   "denied",
			ExitCode: -1,
			Context:  DeniedContext(spec.Command, decision.Reason),
		}, fmt.Errorf("exec policy denied: %s", decision.Reason)
	}

	// Apply defaults.
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxBytes := spec.MaxOutputBytes
	if maxBytes <= 0 {
		if spec.SensitiveOutput {
			maxBytes = SensitiveMaxOutputBytes
		} else {
			maxBytes = DefaultMaxOutputBytes
		}
	}

	// Build execution context fields.
	execCtx := ExecutionContext{
		Executor:            "least_privilege_proxy",
		Profile:             string(decision.Profile),
		CommandName:         spec.Command,
		ArgsCount:           len(spec.Args),
		TimeoutMs:           int(timeout.Milliseconds()),
		MaxOutputBytes:      maxBytes,
		ShellUsed:           false,
		SudoUsed:            false,
		AllowedByExecPolicy: true,
		PolicyReason:        decision.Reason,
		EffectiveUser:       effectiveUser(),
		Platform:            platform(),
		OutputTruncated:     false,
	}

	// Create a context with timeout.
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build and run the command — no shell, direct exec.
	var stdoutStr, stderrStr string
	var runErr error
	start := time.Now()

	if e.runner != nil {
		// Use injected runner (for testing).
		stdoutBytes, stderrBytes, runnerErr := e.runner(runCtx, spec.Command, spec.Args...)
		stdoutStr = string(stdoutBytes)
		stderrStr = string(stderrBytes)
		runErr = runnerErr
	} else {
		// Real execution via exec.CommandContext.
		cmd := exec.CommandContext(runCtx, spec.Command, spec.Args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr = cmd.Run()
		stdoutStr = stdout.String()
		stderrStr = stderr.String()
	}
	elapsed := time.Since(start)

	result := ExecutionResult{
		Context:  execCtx,
		Duration: elapsed,
	}

	// Check for timeout.
	if runCtx.Err() == context.DeadlineExceeded {
		result.Status = "error"
		result.TimedOut = true
		result.Stderr = fmt.Sprintf("command timed out after %dms", execCtx.TimeoutMs)
		return result, fmt.Errorf("command %s timed out after %dms", spec.Command, execCtx.TimeoutMs)
	}

	// Read output with size limiting.
	if len(stdoutStr) > maxBytes {
		stdoutStr = stdoutStr[:maxBytes]
		result.Truncated = true
		execCtx.OutputTruncated = true
	}
	if len(stderrStr) > maxBytes {
		stderrStr = stderrStr[:maxBytes]
		result.Truncated = true
		execCtx.OutputTruncated = true
	}

	result.Stdout = stdoutStr
	result.Stderr = stderrStr

	// Determine exit code and status.
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			if spec.AllowNonZeroExit {
				result.Status = "warning"
			} else {
				result.Status = "error"
			}
		} else {
			result.ExitCode = -1
			result.Status = "error"
		}
	} else {
		result.ExitCode = 0
		result.Status = "ok"
	}

	// Update context in result (may have been modified for truncation).
	result.Context = execCtx

	return result, runErr
}

// QuickExecute is a convenience method for simple read-only commands.
// It uses the default profile, timeout, and output limits.
func (e *Executor) QuickExecute(ctx context.Context, toolName string, profile ExecutionProfile, command string, args []string) (ExecutionResult, error) {
	return e.Execute(ctx, CommandSpec{
		ToolName: toolName,
		Profile:  profile,
		Command:  command,
		Args:     args,
		Reason:   fmt.Sprintf("execute %s %s", command, strings.Join(args, " ")),
	})
}

// InjectExecutionContext adds the execution context fields to a ToolTrace.
func InjectExecutionContext(trace *logtrace.ToolTrace, ec ExecutionContext) {
	if trace == nil {
		return
	}
	if trace.ExecutionContext == nil {
		trace.ExecutionContext = &logtrace.ExecutionContext{}
	}
	trace.ExecutionContext.Executor = ec.Executor
	trace.ExecutionContext.Profile = ec.Profile
	trace.ExecutionContext.CommandName = ec.CommandName
	trace.ExecutionContext.ArgsCount = ec.ArgsCount
	trace.ExecutionContext.TimeoutMs = ec.TimeoutMs
	trace.ExecutionContext.MaxOutputBytes = ec.MaxOutputBytes
	trace.ExecutionContext.ShellUsed = ec.ShellUsed
	trace.ExecutionContext.SudoUsed = ec.SudoUsed
	trace.ExecutionContext.AllowedByExecPolicy = ec.AllowedByExecPolicy
	trace.ExecutionContext.PolicyReason = ec.PolicyReason
	trace.ExecutionContext.EffectiveUser = ec.EffectiveUser
	trace.ExecutionContext.Platform = ec.Platform
}
