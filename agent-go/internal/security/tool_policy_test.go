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
