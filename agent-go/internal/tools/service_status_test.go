package tools

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
)

func TestServiceStatusUsesServiceNameSemantic(t *testing.T) {
	semantic := SemanticForTool("service_status", map[string]any{
		"service_name": "sshd",
	})

	if semantic.ResourceType != "system_service" {
		t.Fatalf("expected system_service resource type, got %q", semantic.ResourceType)
	}
	if semantic.ResourcePath != "systemd:sshd" {
		t.Fatalf("expected systemd:sshd resource path, got %q", semantic.ResourcePath)
	}
	if !semantic.AllowedByPolicy {
		t.Fatal("service status reads should be allowed by policy")
	}
}

func TestServiceStatusGracefulUnsupported(t *testing.T) {
	output, summary, _, err := ServiceStatus(context.Background(), map[string]any{
		"service_name": "sshd",
	})
	result := output.(ServiceStatusResult)

	if runtime.GOOS != "linux" {
		if err != nil {
			t.Fatalf("non-Linux service_status should be graceful, got error: %v", err)
		}
		if result.Status != "unsupported" {
			t.Fatalf("expected unsupported on non-Linux, got %q", result.Status)
		}
		return
	}

	if _, lookErr := exec.LookPath("systemctl"); lookErr != nil {
		if err == nil {
			t.Fatal("Linux without systemctl should return an error trace")
		}
		if result.Status != "error" {
			t.Fatalf("expected status=error without systemctl, got %q", result.Status)
		}
		return
	}

	if err != nil {
		t.Fatalf("systemctl command failures should be summarized gracefully, got tool error: %v", err)
	}
	if result.Service != "sshd" {
		t.Fatalf("expected service sshd, got %q", result.Service)
	}
	if summary == "" {
		t.Fatal("expected nonempty output summary")
	}
}
