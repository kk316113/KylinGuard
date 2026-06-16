package tools

import (
	"context"
	"testing"
)

func TestSSHLoginAnalyzerUnavailableLogsDoNotPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	output, summary, riskHint, err := SSHLoginAnalyzer(ctx, map[string]any{
		"paths": []string{"/etc/shadow"},
		"lines": 200,
	})

	if err == nil {
		t.Fatal("expected unavailable logs to return an error")
	}
	if summary == "" {
		t.Fatal("expected output summary")
	}
	if riskHint == "" {
		t.Fatal("expected risk hint")
	}
	result := output.(SSHLoginAnalyzerResult)
	if result.Analysis.RiskLevel != "unknown" {
		t.Fatalf("expected unknown risk for unavailable logs, got %q", result.Analysis.RiskLevel)
	}
	if result.LogCollection.Status != "error" {
		t.Fatalf("expected log collection error status, got %q", result.LogCollection.Status)
	}
}

func TestSSHLoginAnalyzerTraceSemantics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	registry := NewDefaultRegistry()
	result, _ := registry.Invoke(ctx, "ssh_login_analyzer", map[string]any{
		"paths": []string{"/etc/shadow"},
		"lines": 200,
	})

	trace := result.Trace
	if trace.ToolName != "ssh_login_analyzer" {
		t.Fatalf("unexpected tool name: %q", trace.ToolName)
	}
	if trace.OperationType != "analyze" {
		t.Fatalf("expected analyze operation, got %q", trace.OperationType)
	}
	if trace.ResourceType != "ssh_auth_log" {
		t.Fatalf("expected ssh_auth_log resource type, got %q", trace.ResourceType)
	}
	if trace.ResourcePath != "ssh_auth:none" {
		t.Fatalf("expected ssh_auth:none resource path, got %q", trace.ResourcePath)
	}
	if trace.BoundaryLevel != "sensitive_system_resource" {
		t.Fatalf("expected sensitive boundary, got %q", trace.BoundaryLevel)
	}
	if !trace.RequiresPrivilege {
		t.Fatal("ssh auth analysis should require privilege")
	}
	if !trace.AllowedByPolicy {
		t.Fatal("ssh auth analysis should be allowed by policy for requested diagnosis")
	}
}
