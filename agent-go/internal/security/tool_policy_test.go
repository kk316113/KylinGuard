package security

import (
	"testing"

	"kylin-guard-agent/agent-go/internal/tools"
)

func TestToolPolicyDeniesUnknownTool(t *testing.T) {
	policy := NewToolPolicy()
	decision := policy.Evaluate("unknown_tool", tools.ToolMetadata{}, false, map[string]any{})

	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyDeniesSafeShellDirectCall(t *testing.T) {
	policy := NewToolPolicy()
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("safe_shell")
	if !ok {
		t.Fatal("missing safe_shell metadata")
	}

	decision := policy.Evaluate("safe_shell", metadata, true, map[string]any{"command": "rm -rf /"})

	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyAllowsValidPortChecker(t *testing.T) {
	policy := NewToolPolicy()
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("port_checker")
	if !ok {
		t.Fatal("missing port_checker metadata")
	}

	decision := policy.Evaluate("port_checker", metadata, true, map[string]any{"port": 22})

	if decision.Decision != "allow" {
		t.Fatalf("expected allow, got %#v", decision)
	}
}

func TestToolPolicyDeniesInvalidPortChecker(t *testing.T) {
	policy := NewToolPolicy()
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("port_checker")
	if !ok {
		t.Fatal("missing port_checker metadata")
	}

	decision := policy.Evaluate("port_checker", metadata, true, map[string]any{"port": 99999})

	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyDeniesNonWhitelistedLogPath(t *testing.T) {
	policy := NewToolPolicy()
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("log_reader")
	if !ok {
		t.Fatal("missing log_reader metadata")
	}

	decision := policy.Evaluate("log_reader", metadata, true, map[string]any{"paths": []any{"/etc/shadow"}, "lines": 100})

	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyDeniesUnsafeServiceName(t *testing.T) {
	policy := NewToolPolicy()
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("service_status")
	if !ok {
		t.Fatal("missing service_status metadata")
	}

	decision := policy.Evaluate("service_status", metadata, true, map[string]any{"service_name": "sshd; rm -rf /"})

	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyDeniesUnknownArguments(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	metadata, _ := registry.GetTool("os_info")
	decision := NewToolPolicy().Evaluate("os_info", metadata, true, map[string]any{
		"command": "cat /etc/shadow",
	})
	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyDeniesNestedInjection(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	metadata, _ := registry.GetTool("log_reader")
	decision := NewToolPolicy().Evaluate("log_reader", metadata, true, map[string]any{
		"paths": []any{"/var/log/auth.log", map[string]any{"payload": "ok; rm -rf /"}},
	})
	assertToolPolicyDeny(t, decision)
}

func TestToolPolicyValidatesOpenFileInspectorScope(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	metadata, ok := registry.GetTool("open_file_inspector")
	if !ok {
		t.Fatal("missing open_file_inspector metadata")
	}
	allowed := NewToolPolicy().Evaluate("open_file_inspector", metadata, true, map[string]any{"path": "/var/log/messages"})
	if allowed.Decision != "allow" {
		t.Fatalf("expected approved lsof path, got %#v", allowed)
	}
	for _, input := range []map[string]any{
		{},
		{"path": "/etc/shadow"},
		{"pid": 0},
		{"path": "/tmp/a", "pid": 1},
		{"pid": 1, "limit": 1000},
		{"path": "/tmp/a", "pid": "not-a-number"},
		{"pid": 1, "limit": "many"},
	} {
		assertToolPolicyDeny(t, NewToolPolicy().Evaluate("open_file_inspector", metadata, true, input))
	}
}

func TestToolPolicyValidatesProcessStateAndDiskIOSample(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	processMetadata, _ := registry.GetTool("process_inspector")
	if decision := NewToolPolicy().Evaluate("process_inspector", processMetadata, true, map[string]any{"state": "ZOMBIE"}); decision.Decision != "allow" {
		t.Fatalf("expected ZOMBIE filter allowed: %#v", decision)
	}
	assertToolPolicyDeny(t, NewToolPolicy().Evaluate("process_inspector", processMetadata, true, map[string]any{"state": "KILLED"}))

	diskMetadata, _ := registry.GetTool("disk_io_checker")
	if decision := NewToolPolicy().Evaluate("disk_io_checker", diskMetadata, true, map[string]any{"sample_ms": 250}); decision.Decision != "allow" {
		t.Fatalf("expected bounded disk sample allowed: %#v", decision)
	}
	assertToolPolicyDeny(t, NewToolPolicy().Evaluate("disk_io_checker", diskMetadata, true, map[string]any{"sample_ms": 5000}))
	assertToolPolicyDeny(t, NewToolPolicy().Evaluate("disk_io_checker", diskMetadata, true, map[string]any{"sample_ms": "slow"}))
}

func assertToolPolicyDeny(t *testing.T, decision ToolPolicyDecision) {
	t.Helper()
	if decision.Decision != "deny" {
		t.Fatalf("expected deny, got %#v", decision)
	}
	if decision.Method != ToolPolicyMethod {
		t.Fatalf("expected method %q, got %q", ToolPolicyMethod, decision.Method)
	}
	if decision.Message != "tool call denied by tool policy" {
		t.Fatalf("unexpected message: %q", decision.Message)
	}
}
