package tools

import (
	"context"
	"strings"
	"testing"
)

func TestAuthLogCollectorDefaultSourceOrder(t *testing.T) {
	result, _ := collectSSHAuthLogs(context.Background(), map[string]any{
		"paths": []string{"/var/log/secure", "/var/log/auth.log"},
		"lines": 999,
	})

	if result.LinesRequested != maxLogLines {
		t.Fatalf("expected lines to be capped at %d, got %d", maxLogLines, result.LinesRequested)
	}
	if len(result.AttemptedPaths) != 2 {
		t.Fatalf("expected two attempted paths, got %#v", result.AttemptedPaths)
	}
	if result.AttemptedPaths[0] != "/var/log/secure" || result.AttemptedPaths[1] != "/var/log/auth.log" {
		t.Fatalf("unexpected auth log source order: %#v", result.AttemptedPaths)
	}
}

func TestAuthLogCollectorWhitelistLogic(t *testing.T) {
	if !isAllowedLogReadPath("/var/log/secure") {
		t.Fatal("/var/log/secure should be allowed")
	}
	if isAllowedLogReadPath("/etc/shadow") {
		t.Fatal("/etc/shadow should not be allowed")
	}
}

func TestAuthLogCollectorRejectsArbitraryPathWithoutPanic(t *testing.T) {
	result, _ := collectSSHAuthLogs(context.Background(), map[string]any{
		"paths": []string{"/etc/shadow"},
	})

	if result.SourceType == "file" {
		t.Fatalf("non-whitelisted path must not be read as file: %#v", result)
	}
	foundPolicyError := false
	for _, item := range result.Errors {
		if strings.Contains(item, "not allowed") {
			foundPolicyError = true
			break
		}
	}
	if !foundPolicyError {
		t.Fatalf("expected policy error for arbitrary path, got %#v", result.Errors)
	}
}

func TestAuthLogCollectorGracefulUnavailable(t *testing.T) {
	result, err := collectSSHAuthLogs(context.Background(), map[string]any{
		"paths": []string{"/var/log/secure-does-not-exist"},
	})

	if result.SourceType == "file" {
		t.Fatalf("unexpected file source for unavailable path: %#v", result)
	}
	if result.Status == "" {
		t.Fatal("expected collector status")
	}
	if err != nil && result.Message == "" {
		t.Fatal("expected clear error message when collection fails")
	}
}
