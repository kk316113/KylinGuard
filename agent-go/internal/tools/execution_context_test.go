package tools

import (
	"context"
	"testing"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

// assertExecutionContext checks required execution_context fields on a trace.
func assertExecutionContext(t *testing.T, trace logtrace.ToolTrace, expectProfile string, expectCommand string) {
	t.Helper()
	if trace.ExecutionContext == nil {
		t.Fatalf("%s: expected execution_context to be non-nil", trace.ToolName)
	}
	ec := trace.ExecutionContext
	if ec.Executor != "least_privilege_proxy" {
		t.Fatalf("%s: expected executor=least_privilege_proxy, got %q", trace.ToolName, ec.Executor)
	}
	if ec.Profile != expectProfile {
		t.Fatalf("%s: expected profile=%s, got %q", trace.ToolName, expectProfile, ec.Profile)
	}
	if ec.ShellUsed {
		t.Fatalf("%s: expected shell_used=false", trace.ToolName)
	}
	if ec.SudoUsed {
		t.Fatalf("%s: expected sudo_used=false", trace.ToolName)
	}
	if ec.Platform == "" {
		t.Fatalf("%s: expected platform to be non-empty", trace.ToolName)
	}
	if ec.PolicyReason == "" {
		t.Fatalf("%s: expected policy_reason to be non-empty", trace.ToolName)
	}
	if ec.AllowedByExecPolicy != true {
		t.Fatalf("%s: expected allowed_by_exec_policy=true, got %v", trace.ToolName, ec.AllowedByExecPolicy)
	}
	if expectCommand != "" && ec.CommandName != expectCommand {
		t.Fatalf("%s: expected command_name=%s, got %q", trace.ToolName, expectCommand, ec.CommandName)
	}
}

func TestProcessInspectorExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "process_inspector", map[string]any{"name": "sshd", "limit": 20})
	if err != nil && result.Trace.Status == "error" {
		t.Skipf("process_inspector not available on this platform: %v", err)
	}
	// Command may be "ps" (Linux) or "tasklist" (Windows); verify executor identity instead.
	assertExecutionContext(t, result.Trace, "low_read", "")
}

func TestNetworkConnectionInspectorExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "network_connection_inspector", map[string]any{"state": "LISTEN", "limit": 50})
	if err != nil && result.Trace.Status == "error" {
		t.Skipf("network_connection_inspector not available: %v", err)
	}
	assertExecutionContext(t, result.Trace, "low_read", "")
}

func TestJournalctlReaderExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "journalctl_reader", map[string]any{"service_name": "sshd", "lines": 10})
	if err != nil && result.Trace.Status == "error" {
		t.Skipf("journalctl_reader not available: %v", err)
	}
	// Command may be "journalctl" (Linux) or "unsupported_platform" (non-Linux).
	assertExecutionContext(t, result.Trace, "sensitive_read", "")
}

func TestResourceUsageCheckerExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "resource_usage_checker", map[string]any{})
	if err != nil && result.Trace.Status == "error" {
		t.Skipf("resource_usage_checker not available: %v", err)
	}
	assertExecutionContext(t, result.Trace, "low_read", "")
}

func TestDiskMemoryCheckerExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "disk_memory_checker", map[string]any{"include_tmpfs": false})
	if err != nil && result.Trace.Status == "error" {
		t.Skipf("disk_memory_checker not available: %v", err)
	}
	assertExecutionContext(t, result.Trace, "low_read", "")
}

func TestPortCheckerExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "port_checker", map[string]any{"host": "127.0.0.1", "port": 8080})
	if err != nil {
		t.Fatalf("port_checker failed: %v", err)
	}
	assertExecutionContext(t, result.Trace, "low_read", "")
}

func TestLogReaderExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "log_reader", map[string]any{
		"paths": []string{"/var/log/secure"},
		"lines": 5,
	})
	if err != nil {
		// LogReader will fail on Windows (no /var/log/secure) — that's OK, just check trace.
	}
	assertExecutionContext(t, result.Trace, "sensitive_read", "")
}

func TestOSInfoExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "os_info", map[string]any{})
	if err != nil {
		t.Fatalf("os_info failed: %v", err)
	}
	assertExecutionContext(t, result.Trace, "public_read", "")
}

func TestServiceStatusExecutionContext(t *testing.T) {
	registry := NewDefaultRegistry()
	result, err := registry.Invoke(context.Background(), "service_status", map[string]any{"service_name": "sshd"})
	if err != nil && result.Trace.Status == "error" {
		t.Skipf("service_status not available: %v", err)
	}
	assertExecutionContext(t, result.Trace, "low_read", "")
}

// assertAllTracesHaveExecutionContext checks that every trace has execution_context with shell_used=false.
func assertAllTracesHaveExecutionContext(t *testing.T, traces []logtrace.ToolTrace) {
	t.Helper()
	for i, trace := range traces {
		if trace.ExecutionContext == nil {
			t.Fatalf("trace[%d] tool=%s: expected execution_context, got nil", i, trace.ToolName)
		}
		if trace.ExecutionContext.ShellUsed {
			t.Fatalf("trace[%d] tool=%s: expected shell_used=false", i, trace.ToolName)
		}
		if trace.ExecutionContext.SudoUsed {
			t.Fatalf("trace[%d] tool=%s: expected sudo_used=false", i, trace.ToolName)
		}
	}
}
