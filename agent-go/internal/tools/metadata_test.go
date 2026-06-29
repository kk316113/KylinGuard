package tools

import (
	"context"
	"testing"
)

func TestDefaultToolMetadata(t *testing.T) {
	registry := NewDefaultRegistry()
	if err := registry.Validate(); err != nil {
		t.Fatalf("default registry should validate: %v", err)
	}
	registered := registry.ListTools()
	if len(registered) != RegisteredToolCount() {
		t.Fatalf("expected %d tools, got %d", RegisteredToolCount(), len(registered))
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
	if safeShell.LLMCallable {
		t.Fatal("safe_shell must not be LLM callable")
	}

	directTools := registry.ListDirectCallTools()
	if len(directTools) != 17 {
		t.Fatalf("expected 17 direct-call tools, got %d: %#v", len(directTools), directTools)
	}
	directNames := map[string]bool{}
	for _, metadata := range directTools {
		directNames[metadata.Name] = true
		if !metadata.IsReadOnly() {
			t.Fatalf("direct-call tool %s must be read-only", metadata.Name)
		}
	}
	for _, expected := range []string{
		"configuration_drift_detector",
		"block_device_inventory",
		"disk_io_checker",
		"disk_memory_checker",
		"journalctl_reader",
		"log_reader",
		"mount_inventory",
		"network_connection_inspector",
		"open_file_inspector",
		"os_info",
		"port_checker",
		"process_inspector",
		"resource_usage_checker",
		"rpm_package_inventory",
		"service_status",
		"ssh_login_analyzer",
		"systemd_unit_inventory",
	} {
		if !directNames[expected] {
			t.Fatalf("expected direct-call tool %s in registry: %#v", expected, directNames)
		}
	}
	if directNames["safe_shell"] {
		t.Fatal("safe_shell must not be exposed as a direct-call tool")
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
		if metadata.DisplayName == "" {
			t.Fatalf("tool %s missing display_name", metadata.Name)
		}
		if len(metadata.Platforms) == 0 || len(metadata.Architectures) == 0 {
			t.Fatalf("tool %s missing platform/architecture metadata", metadata.Name)
		}
		if len(metadata.Tags) == 0 || len(metadata.UseCases) == 0 || len(metadata.SafetyNotes) == 0 {
			t.Fatalf("tool %s missing extensibility metadata: %#v", metadata.Name, metadata)
		}
		if metadata.AuditEventType == "" {
			t.Fatalf("tool %s missing audit_event_type", metadata.Name)
		}
		if registry.IsToolEnabledForDirectCall(metadata.Name) && !metadata.LLMCallable {
			t.Fatalf("direct-call tool %s should be LLM callable", metadata.Name)
		}
		keys := metadata.ArgumentKeys()
		for i := 1; i < len(keys); i++ {
			if keys[i-1] > keys[i] {
				t.Fatalf("tool %s argument keys are not sorted: %#v", metadata.Name, keys)
			}
		}
	}
}

func TestRegistryCatalogSummary(t *testing.T) {
	registry := NewDefaultRegistry()
	catalog := registry.CatalogSummary()
	if catalog.Protocol != ToolProtocol || catalog.Version != ToolProtocolVersion {
		t.Fatalf("unexpected catalog protocol: %#v", catalog)
	}
	if catalog.ToolCount != RegisteredToolCount() || catalog.DirectCallCount != len(registry.ListDirectCallTools()) {
		t.Fatalf("unexpected catalog counts: %#v", catalog)
	}
	if !catalog.SafetyModel.ReadOnlyDirectCallsOnly || catalog.SafetyModel.RawShellExposed {
		t.Fatalf("unexpected safety model: %#v", catalog.SafetyModel)
	}
	if len(catalog.Categories) == 0 || len(catalog.ExtensionPoints) == 0 {
		t.Fatalf("expected categories and extension points: %#v", catalog)
	}
	foundSecurity := false
	for _, category := range catalog.Categories {
		if category.Category == "security" {
			foundSecurity = true
		}
	}
	if !foundSecurity {
		t.Fatalf("expected security category in catalog: %#v", catalog.Categories)
	}
}

func TestRegistryValidateDetectsMetadataDrift(t *testing.T) {
	registry := NewRegistry()
	registry.Register("orphan_handler", func(context.Context, map[string]any) (any, string, string, error) {
		return nil, "", "", nil
	})

	if err := registry.Validate(); err == nil {
		t.Fatal("expected registry validation to reject a handler without metadata")
	}
}
