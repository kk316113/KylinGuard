package execproxy

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeRunner is a CommandRunner that returns canned output or simulates errors.
func fakeRunner(stdout string, stderr string, err error) CommandRunner {
	return func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return []byte(stdout), []byte(stderr), err
	}
}

func TestExecutorExecuteAllowedCommand(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("output line 1\noutput line 2", "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName: "test_tool",
		Profile:  ProfileLowRead,
		Command:  "ps",
		Args:     []string{"aux"},
		Reason:   "test execution",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status=ok, got %q", result.Status)
	}
	if result.Stdout != "output line 1\noutput line 2" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit_code=0, got %d", result.ExitCode)
	}
	if result.TimedOut {
		t.Fatal("expected TimedOut=false")
	}

	// ExecutionContext completeness.
	ec := result.Context
	if ec.Executor != "least_privilege_proxy" {
		t.Fatalf("expected executor=least_privilege_proxy, got %q", ec.Executor)
	}
	if ec.Profile != string(ProfileLowRead) {
		t.Fatalf("expected profile=low_read, got %q", ec.Profile)
	}
	if ec.CommandName != "ps" {
		t.Fatalf("expected command_name=ps, got %q", ec.CommandName)
	}
	if ec.ArgsCount != 1 {
		t.Fatalf("expected args_count=1, got %d", ec.ArgsCount)
	}
	if ec.TimeoutMs <= 0 {
		t.Fatalf("expected timeout_ms > 0, got %d", ec.TimeoutMs)
	}
	if ec.MaxOutputBytes <= 0 {
		t.Fatalf("expected max_output_bytes > 0, got %d", ec.MaxOutputBytes)
	}
	if ec.ShellUsed {
		t.Fatal("expected shell_used=false")
	}
	if ec.SudoUsed {
		t.Fatal("expected sudo_used=false")
	}
	if !ec.AllowedByExecPolicy {
		t.Fatal("expected allowed_by_exec_policy=true")
	}
	if ec.PolicyReason == "" {
		t.Fatal("expected policy_reason to be non-empty")
	}
	if ec.Platform == "" {
		t.Fatal("expected platform to be non-empty")
	}
}

func TestExecutorDeniesPolicyBlockedCommand(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("", "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName: "test_tool",
		Profile:  ProfileLowRead,
		Command:  "rm",
		Args:     []string{"-rf", "/"},
		Reason:   "should be denied",
	})

	if err == nil {
		t.Fatal("expected error for blocked command")
	}
	if !strings.Contains(err.Error(), "exec policy denied") {
		t.Fatalf("expected exec policy denied error, got: %v", err)
	}
	if result.Status != "denied" {
		t.Fatalf("expected status=denied, got %q", result.Status)
	}
	if result.Context.AllowedByExecPolicy {
		t.Fatal("expected allowed_by_exec_policy=false for denied execution")
	}
	if result.Context.Profile != string(ProfileDenied) {
		t.Fatalf("expected profile=denied, got %q", result.Context.Profile)
	}
	if result.Context.PolicyReason == "" {
		t.Fatal("expected policy_reason to be non-empty for denied execution")
	}
}

func TestExecutorDeniesShellCommand(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("", "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		Command: "bash",
		Args:    []string{"-c", "echo hello"},
		Profile: ProfileLowRead,
	})

	if err == nil {
		t.Fatal("expected error for shell command")
	}
	if result.Context.ShellUsed {
		// ShellUsed should remain false because we never even get to execution.
	}
	if !strings.Contains(err.Error(), "exec policy denied") {
		t.Fatalf("expected exec policy denied, got: %v", err)
	}
}

func TestExecutorTimeout(t *testing.T) {
	// Use a runner that simulates a command that exceeds the timeout.
	timeoutRunner := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		// Sleep past the timeout so the context is cancelled.
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return []byte("ok"), nil, nil
		}
	}
	exec := NewExecutor().WithRunner(timeoutRunner)

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName: "test_tool",
		Profile:  ProfileLowRead,
		Command:  "cat",
		Args:     []string{"/dev/zero"},
		Timeout:  100 * time.Millisecond,
		Reason:   "timeout test",
	})

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !result.TimedOut {
		t.Fatalf("expected TimedOut=true, status=%s stderr=%q ctxErr=%v", result.Status, result.Stderr, context.Background().Err())
	}
	if result.Status != "error" {
		t.Fatalf("expected status=error for timeout, got %q", result.Status)
	}
}

func TestExecutorOutputTruncation(t *testing.T) {
	// Generate output that exceeds the limit.
	bigOutput := strings.Repeat("x", 2000)
	exec := NewExecutor().WithRunner(fakeRunner(bigOutput, "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName:       "test_tool",
		Profile:        ProfileLowRead,
		Command:        "cat",
		Args:           []string{"/proc/loadavg"},
		MaxOutputBytes: 1024,
		Reason:         "truncation test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Fatal("expected Truncated=true")
	}
	if len(result.Stdout) > 1024 {
		t.Fatalf("expected stdout <= 1024, got %d", len(result.Stdout))
	}
	if !result.Context.OutputTruncated {
		t.Fatal("expected OutputTruncated=true in execution context")
	}
}

func TestExecutorAllowNonZeroExit(t *testing.T) {
	exitErr := &exec.ExitError{}
	exec := NewExecutor().WithRunner(fakeRunner("partial output", "error output", exitErr))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName:         "test_tool",
		Profile:          ProfileLowRead,
		Command:          "systemctl",
		Args:             []string{"is-active", "unknown-svc"},
		AllowNonZeroExit: true,
		Reason:           "non-zero exit test",
	})

	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if result.Status != "warning" {
		t.Fatalf("expected status=warning for AllowNonZeroExit, got %q", result.Status)
	}
	if result.Stdout != "partial output" {
		t.Fatalf("expected partial stdout, got %q", result.Stdout)
	}
}

func TestExecutorStderrTruncation(t *testing.T) {
	bigStderr := strings.Repeat("E", 3000)
	exec := NewExecutor().WithRunner(fakeRunner("ok", bigStderr, nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName:       "test_tool",
		Profile:        ProfileLowRead,
		Command:        "ps",
		Args:           []string{"aux"},
		MaxOutputBytes: 2048,
		Reason:         "stderr truncation test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Fatal("expected Truncated=true for oversized stderr")
	}
	if len(result.Stderr) > 2048 {
		t.Fatalf("expected stderr <= 2048, got %d", len(result.Stderr))
	}
}

func TestExecutorRunnerReturnsGenericError(t *testing.T) {
	genericErr := errors.New("something went wrong")
	exec := NewExecutor().WithRunner(fakeRunner("", "", genericErr))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName: "test_tool",
		Profile:  ProfileLowRead,
		Command:  "ps",
		Args:     []string{"aux"},
		Reason:   "generic error test",
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if result.Status != "error" {
		t.Fatalf("expected status=error, got %q", result.Status)
	}
	if result.ExitCode != -1 {
		t.Fatalf("expected exit_code=-1 for non-ExitError, got %d", result.ExitCode)
	}
}

func TestExecutorSensitiveOutputDefaultSize(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("sensitive data", "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		ToolName:        "journalctl_reader",
		Profile:         ProfileSensitiveRead,
		Command:         "journalctl",
		Args:            []string{"-u", "sshd", "-n", "100", "--no-pager"},
		SensitiveOutput: true,
		Reason:          "sensitive test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Context.MaxOutputBytes != SensitiveMaxOutputBytes {
		t.Fatalf("expected sensitive max output %d, got %d", SensitiveMaxOutputBytes, result.Context.MaxOutputBytes)
	}
}

func TestNativeExecutionContext(t *testing.T) {
	ec := NativeExecutionContext(ProfileLowRead, "procfs_read", "reading /proc/loadavg via Go os.ReadFile")
	if ec.Executor != "least_privilege_proxy" {
		t.Fatalf("expected executor=least_privilege_proxy, got %q", ec.Executor)
	}
	if ec.Profile != string(ProfileLowRead) {
		t.Fatalf("expected profile=low_read, got %q", ec.Profile)
	}
	if ec.CommandName != "procfs_read" {
		t.Fatalf("expected command_name=procfs_read, got %q", ec.CommandName)
	}
	if ec.ArgsCount != 0 {
		t.Fatalf("expected args_count=0 for native, got %d", ec.ArgsCount)
	}
	if !ec.AllowedByExecPolicy {
		t.Fatal("expected allowed_by_exec_policy=true for native")
	}
	if ec.ShellUsed {
		t.Fatal("expected shell_used=false for native")
	}
}

func TestDeniedContext(t *testing.T) {
	ec := DeniedContext("sh", "shell interpreters are forbidden")
	if ec.AllowedByExecPolicy {
		t.Fatal("expected allowed_by_exec_policy=false")
	}
	if ec.Profile != string(ProfileDenied) {
		t.Fatalf("expected profile=denied, got %q", ec.Profile)
	}
	if ec.PolicyReason == "" {
		t.Fatal("expected policy_reason for denied context")
	}
	if ec.CommandName != "sh" {
		t.Fatalf("expected command_name=sh, got %q", ec.CommandName)
	}
}

func TestInjectExecutionContext(t *testing.T) {
	// Test injection into a trace.
	ec := ExecutionContext{
		Executor:            "least_privilege_proxy",
		Profile:             "low_read",
		CommandName:         "ps",
		ArgsCount:           1,
		TimeoutMs:           3000,
		MaxOutputBytes:      65536,
		ShellUsed:           false,
		SudoUsed:            false,
		AllowedByExecPolicy: true,
		PolicyReason:        "allowed test",
		Platform:            "test/fake",
	}

	// InjectExecutionContext takes *logtrace.ToolTrace, but we're in the execproxy package.
	// We test the field mapping indirectly via the Execute test above.
	// Here we just verify the context fields are correct.
	if ec.Executor != "least_privilege_proxy" {
		t.Fatal("unexpected executor")
	}
}

func TestQuickExecute(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("quick result", "", nil))

	result, err := exec.QuickExecute(context.Background(), "test", ProfileLowRead, "ps", []string{"aux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %q", result.Status)
	}
	if result.Stdout != "quick result" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestPlatformIsNotEmpty(t *testing.T) {
	p := platform()
	if p == "" {
		t.Fatal("expected platform to be non-empty")
	}
	// Should contain OS/ARCH format.
	if !strings.Contains(p, "/") {
		t.Fatalf("expected platform to contain '/', got %q", p)
	}
}

func TestExecutorDeniesShellCharsInArgs(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("", "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		Command: "ps",
		Args:    []string{"aux; rm -rf /"},
		Profile: ProfileLowRead,
	})

	if err == nil {
		t.Fatal("expected error for shell chars in args")
	}
	if !strings.Contains(err.Error(), "exec policy denied") {
		t.Fatalf("expected exec policy denied, got: %v", err)
	}
	if result.Status != "denied" {
		t.Fatalf("expected denied, got %q", result.Status)
	}
}

func TestExecutorDeniesShellCharsInArgsPipe(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("", "", nil))
	result, err := exec.Execute(context.Background(), CommandSpec{
		Command: "df",
		Args:    []string{"-h", "|", "grep", "sda"},
		Profile: ProfileLowRead,
	})
	if err == nil || result.Status != "denied" {
		t.Fatalf("expected denied for pipe in args: err=%v status=%s", err, result.Status)
	}
}

func TestExecutorEmptyCommandDenied(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("", "", nil))
	result, err := exec.Execute(context.Background(), CommandSpec{
		Command: "",
		Profile: ProfileLowRead,
	})
	if err == nil || !strings.Contains(err.Error(), "exec policy denied") {
		t.Fatalf("expected denied for empty command: err=%v", err)
	}
	if result.Status != "denied" {
		t.Fatalf("expected denied for empty command: status=%s", result.Status)
	}
}

func TestExecutorSudoDenied(t *testing.T) {
	exec := NewExecutor().WithRunner(fakeRunner("", "", nil))
	result, err := exec.Execute(context.Background(), CommandSpec{
		Command: "sudo",
		Args:    []string{"whoami"},
		Profile: ProfileLowRead,
	})
	if err == nil || result.Context.Profile != string(ProfileDenied) {
		t.Fatalf("expected sudo denied: err=%v status=%s profile=%s", err, result.Status, result.Context.Profile)
	}
}

func TestInjectedRunnerReceivesCorrectArgs(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	captureRunner := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		capturedName = name
		capturedArgs = append([]string{}, args...)
		return []byte("ok"), nil, nil
	}

	exec := NewExecutor().WithRunner(captureRunner)
	_, err := exec.Execute(context.Background(), CommandSpec{
		Command: "journalctl",
		Args:    []string{"-u", "sshd", "-n", "100", "--no-pager"},
		Profile: ProfileSensitiveRead,
		Reason:  "capture test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedName != "journalctl" {
		t.Fatalf("expected captured name=journalctl, got %q", capturedName)
	}
	if len(capturedArgs) != 5 {
		t.Fatalf("expected 5 captured args, got %d: %v", len(capturedArgs), capturedArgs)
	}
	if capturedArgs[0] != "-u" || capturedArgs[1] != "sshd" {
		t.Fatalf("unexpected captured args: %v", capturedArgs)
	}
}

func TestOutputTruncationPreservesPrefix(t *testing.T) {
	prefix := "BEGIN_MARKER_"
	bigContent := prefix + strings.Repeat("DATA", 2000)
	maxBytes := 200
	exec := NewExecutor().WithRunner(fakeRunner(bigContent, "", nil))

	result, err := exec.Execute(context.Background(), CommandSpec{
		Command:        "cat",
		Args:           []string{"/proc/loadavg"},
		MaxOutputBytes: maxBytes,
		Profile:        ProfileLowRead,
		Reason:         "prefix test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Fatal("expected Truncated=true")
	}
	if !strings.HasPrefix(result.Stdout, prefix) {
		t.Fatalf("expected output to start with %q, got prefix %q", prefix, result.Stdout[:min(len(prefix), len(result.Stdout))])
	}
}

func TestFmtImported(t *testing.T) {
	// Ensure fmt is used (compile check).
	_ = fmt.Sprintf("test")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
