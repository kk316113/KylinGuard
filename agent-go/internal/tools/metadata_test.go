package tools

import "testing"

func TestDefaultToolMetadata(t *testing.T) {
	registry := NewDefaultRegistry()
	registered := registry.ListTools()
	if len(registered) < 6 {
		t.Fatalf("expected at least 6 tools, got %d", len(registered))
	}

	sshAnalyzer, ok := registry.GetTool("ssh_login_analyzer")
	if !ok {
		t.Fatal("expected ssh_login_analyzer metadata")
	}
	if sshAnalyzer.BoundaryLevel != "sensitive_system_resource" {
		t.Fatalf("unexpected ssh_login_analyzer boundary_level: %q", sshAnalyzer.BoundaryLevel)
	}

	safeShell, ok := registry.GetTool("safe_shell")
	if !ok {
		t.Fatal("expected safe_shell metadata")
	}
	if registry.IsToolEnabledForDirectCall("safe_shell") {
		t.Fatal("safe_shell direct call should be disabled")
	}
	if safeShell.Enabled || safeShell.DirectCallAllowed {
		t.Fatalf("expected safe_shell disabled for direct call, got enabled=%v direct_call_allowed=%v", safeShell.Enabled, safeShell.DirectCallAllowed)
	}

	for _, metadata := range registered {
		if metadata.Name == "" {
			t.Fatalf("tool missing name: %#v", metadata)
		}
		if metadata.Description == "" {
			t.Fatalf("tool %s missing description", metadata.Name)
		}
		if metadata.Category == "" {
			t.Fatalf("tool %s missing category", metadata.Name)
		}
		if metadata.Version == "" {
			t.Fatalf("tool %s missing version", metadata.Name)
		}
		if metadata.InputSchema == nil {
			t.Fatalf("tool %s missing input_schema", metadata.Name)
		}
		if metadata.OutputSchema == nil {
			t.Fatalf("tool %s missing output_schema", metadata.Name)
		}
	}
}
