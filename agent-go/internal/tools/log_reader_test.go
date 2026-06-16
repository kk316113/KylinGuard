package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestLogReaderWhitelistAllowsKnownPaths(t *testing.T) {
	for _, path := range []string{
		"/var/log/secure",
		"/var/log/auth.log",
		"/var/log/messages",
		"/var/log/syslog",
		"/var/log/audit/audit.log",
	} {
		if !isAllowedLogReadPath(path) {
			t.Fatalf("expected %s to be allowed", path)
		}
	}
}

func TestLogReaderRejectsNonWhitelistPath(t *testing.T) {
	output, summary, _, err := LogReader(context.Background(), map[string]any{
		"path": "/etc/shadow",
	})
	if err == nil {
		t.Fatal("expected non-whitelisted path to return error")
	}
	if !strings.Contains(summary, "not allowed") {
		t.Fatalf("expected policy rejection summary, got %q", summary)
	}
	result := output.(LogReaderResult)
	if len(result.Errors) == 0 {
		t.Fatal("expected rejection details in result errors")
	}
}

func TestLogReaderCapsLinesAt500(t *testing.T) {
	output, _, _, _ := LogReader(context.Background(), map[string]any{
		"path":  "/var/log/secure",
		"lines": 999,
	})
	result := output.(LogReaderResult)
	if result.LinesRequested != maxLogLines {
		t.Fatalf("expected lines to be capped at %d, got %d", maxLogLines, result.LinesRequested)
	}
}

func TestLogReaderMissingPathReturnsError(t *testing.T) {
	path := "/var/log/secure"
	if _, err := os.Stat(path); err == nil {
		t.Skipf("%s exists on this host; missing-path behavior is environment-dependent", path)
	}

	output, summary, _, err := LogReader(context.Background(), map[string]any{
		"path": path,
	})
	if err == nil {
		t.Fatal("expected missing allowed path to return error")
	}
	if !strings.Contains(summary, "no allowed log path could be read") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	result := output.(LogReaderResult)
	if result.Path != "" {
		t.Fatalf("missing path should not be selected, got %q", result.Path)
	}
}
