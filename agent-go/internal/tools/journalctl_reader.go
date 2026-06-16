package tools

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type JournalctlReaderResult struct {
	ServiceName      string                     `json:"service_name"`
	Lines            []string                   `json:"lines"`
	Source           string                     `json:"source"`
	Status           string                     `json:"status"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

var safeJournalServicePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

func SafeJournalServiceName(name string) bool {
	return name != "" && safeJournalServicePattern.MatchString(name)
}

func JournalctlReader(ctx context.Context, input map[string]any) (any, string, string, error) {
	serviceName := strings.TrimSpace(stringValue(input, "service_name", "sshd"))
	lines := intValue(input, "lines", 100)
	if lines < 1 {
		lines = 1
	}
	if lines > 500 {
		lines = 500
	}
	now := time.Now().UTC()

	if serviceName == "" || !safeJournalServicePattern.MatchString(serviceName) {
		ec := execproxy.DeniedContext("journalctl", fmt.Sprintf("invalid service_name %q", serviceName))
		result := JournalctlReaderResult{
			ServiceName:      serviceName,
			Lines:            []string{},
			Status:           "denied",
			Message:          fmt.Sprintf("invalid service_name %q", serviceName),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "journalctl_reader: invalid service_name rejected", "low", fmt.Errorf("invalid service_name %q", serviceName)
	}

	if runtime.GOOS != "linux" {
		nec := execproxy.NativeExecutionContext(execproxy.ProfileSensitiveRead, "unsupported_platform", "journalctl_reader unsupported on non-Linux")
		result := JournalctlReaderResult{
			ServiceName:      serviceName,
			Lines:            []string{},
			Status:           "unsupported",
			Message:          "journalctl is only available on Linux with systemd",
			Timestamp:        now,
			ExecutionContext: ecPtr(nec),
		}
		return result, "journalctl_reader unsupported on non-Linux host", "low", nil
	}

	exec := execproxy.NewExecutor()
	args := []string{"-u", serviceName, "-n", fmt.Sprintf("%d", lines), "--no-pager"}
	result, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName:         "journalctl_reader",
		Profile:          execproxy.ProfileSensitiveRead,
		Command:          "journalctl",
		Args:             args,
		Timeout:          5 * time.Second,
		SensitiveOutput:  true,
		AllowNonZeroExit: true,
		Reason:           fmt.Sprintf("read journal logs for service %s", serviceName),
	})

	status := "ok"
	message := "journalctl log read completed"
	summary := fmt.Sprintf("journalctl_reader read %d lines from %s", lines, serviceName)
	riskHint := "low"

	if execErr != nil || result.Status == "error" {
		if strings.Contains(execErr.Error(), "permission") || strings.Contains(execErr.Error(), "denied") {
			status = "error"
			message = fmt.Sprintf("journalctl permission denied: %v", execErr)
			riskHint = "review"
		} else {
			status = "warning"
			message = fmt.Sprintf("journalctl returned with error: %v", execErr)
		}
	}

	logLines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	if len(logLines) == 1 && logLines[0] == "" {
		logLines = []string{}
	}

	r := JournalctlReaderResult{
		ServiceName:      serviceName,
		Lines:            logLines,
		Source:           "journalctl",
		Status:           status,
		Message:          message,
		Timestamp:        now,
		ExecutionContext: ecPtr(result.Context),
	}
	return r, summary, riskHint, nil
}
